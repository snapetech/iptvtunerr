// Package sports implements server-side API-Sports canonical schedule caching.
// The API key remains process-local, and requests use a fixed upstream registry.
package sports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxBodyBytes        = 8 << 20
	maxWindowPast       = 24 * time.Hour
	maxWindowFuture     = 7 * 24 * time.Hour
	requestSpacing      = time.Second
	baseballFreshFor    = 30 * time.Minute
	footballFreshFor    = 30 * time.Minute
	futureFreshFor      = 12 * time.Hour
	oldScheduleFreshFor = 6 * time.Hour
)

var (
	ErrNotConfigured = errors.New("API-Sports key is not configured")
	ErrInvalidWindow = errors.New("sports schedule window is outside the allowed range")
	ErrNoDatasets    = errors.New("no API-Sports datasets are enabled")
)

// Dataset is a supported canonical schedule feed. Hosts are fixed in code.
type Dataset struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Product        string `json:"product"`
	Host           string `json:"host"`
	RemoteLeagueID int    `json:"remote_league_id"`
	GuideBlock     int    `json:"guide_block"`
}

var datasetCatalog = []Dataset{
	{ID: "mlb", Name: "MLB", Product: "Baseball", Host: "v1.baseball.api-sports.io", RemoteLeagueID: 1, GuideBlock: 1000},
	{ID: "nfl", Name: "NFL", Product: "American Football", Host: "v1.american-football.api-sports.io", RemoteLeagueID: 1, GuideBlock: 4000},
	{ID: "ncaa", Name: "NCAA Football", Product: "American Football", Host: "v1.american-football.api-sports.io", RemoteLeagueID: 2, GuideBlock: 5000},
}

func Datasets() []Dataset { return append([]Dataset(nil), datasetCatalog...) }

func DatasetByID(id string) (Dataset, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, item := range datasetCatalog {
		if item.ID == id {
			return item, true
		}
	}
	return Dataset{}, false
}

// Event is an API-Sports schedule row normalized across Baseball and NFL/NCAA.
type Event struct {
	ID           string    `json:"id"`
	Dataset      string    `json:"dataset"`
	League       string    `json:"league,omitempty"`
	HomeTeam     string    `json:"home_team"`
	HomeTeamCode string    `json:"home_team_code,omitempty"`
	HomeTeamID   string    `json:"home_team_id,omitempty"`
	AwayTeam     string    `json:"away_team"`
	AwayTeamCode string    `json:"away_team_code,omitempty"`
	AwayTeamID   string    `json:"away_team_id,omitempty"`
	StartsAt     time.Time `json:"starts_at"`
	Status       string    `json:"status,omitempty"`
	StatusCode   string    `json:"status_code,omitempty"`
}

type Quota struct {
	DailyLimit      *int `json:"daily_limit,omitempty"`
	DailyRemaining  *int `json:"daily_remaining,omitempty"`
	MinuteLimit     *int `json:"minute_limit,omitempty"`
	MinuteRemaining *int `json:"minute_remaining,omitempty"`
}

type DatasetStatus struct {
	ID                  string   `json:"id"`
	CachedDates         []string `json:"cached_dates"`
	CachedEventCount    int      `json:"cached_event_count"`
	LastFetchAt         string   `json:"last_fetch_at,omitempty"`
	LastFetchEventCount int      `json:"last_fetch_event_count"`
	StaleDateCount      int      `json:"stale_date_count"`
}

type Status struct {
	Provider      string          `json:"provider"`
	Configured    bool            `json:"configured"`
	LastRequestAt string          `json:"last_request_at,omitempty"`
	LastSuccessAt string          `json:"last_success_at,omitempty"`
	LastError     string          `json:"last_error,omitempty"`
	Quota         Quota           `json:"quota"`
	Datasets      []DatasetStatus `json:"datasets"`
}

type Snapshot struct {
	Events   []Event  `json:"events"`
	Warnings []string `json:"warnings,omitempty"`
	Status   Status   `json:"status"`
}

type cacheEntry struct {
	Dataset   string    `json:"dataset"`
	Date      string    `json:"date"`
	FetchedAt time.Time `json:"fetched_at"`
	Events    []Event   `json:"events"`
}

type diskState struct {
	Version       int          `json:"version"`
	Entries       []cacheEntry `json:"entries"`
	LastRequestAt time.Time    `json:"last_request_at,omitempty"`
	LastSuccessAt time.Time    `json:"last_success_at,omitempty"`
	LastError     string       `json:"last_error,omitempty"`
	Quota         Quota        `json:"quota"`
	QuotaAt       time.Time    `json:"quota_at,omitempty"`
}

type Service struct {
	key       string
	cachePath string
	client    *http.Client

	refreshGate chan struct{}
	mu          sync.RWMutex
	entries     map[string]cacheEntry
	lastRequest time.Time
	lastSuccess time.Time
	lastError   string
	quota       Quota
	quotaAt     time.Time
	nextRequest time.Time
}

func NewService(key, cachePath string) *Service {
	s := &Service{
		key:       strings.TrimSpace(key),
		cachePath: strings.TrimSpace(cachePath),
		client: &http.Client{
			Timeout:       25 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		refreshGate: make(chan struct{}, 1),
		entries:     make(map[string]cacheEntry),
	}
	s.load()
	return s
}

func (s *Service) SetAPIKey(key string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.key = strings.TrimSpace(key)
	s.mu.Unlock()
}

func (s *Service) apiKey() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key
}

func (s *Service) APIKeyConfigured() bool {
	return s.apiKey() != ""
}

func cacheKey(dataset, date string) string { return dataset + "\x00" + date }

// RefreshWindow fetches only missing/expired selected schedule dates. The American
// Football endpoint is called once per date and its response is partitioned into NFL
// and NCAA caches, which avoids duplicate quota use when both are enabled.
func (s *Service) RefreshWindow(ctx context.Context, datasetIDs []string, start, end time.Time, force bool) (Snapshot, error) {
	if s == nil || !s.APIKeyConfigured() {
		return Snapshot{Status: s.Status()}, ErrNotConfigured
	}
	if start.IsZero() || end.IsZero() || !end.After(start) || start.Before(time.Now().Add(-maxWindowPast)) || end.After(time.Now().Add(maxWindowFuture)) {
		return Snapshot{Status: s.Status()}, ErrInvalidWindow
	}
	selected := normalizeDatasetIDs(datasetIDs)
	if len(selected) == 0 {
		return Snapshot{Status: s.Status()}, ErrNoDatasets
	}
	selectSet := make(map[string]bool, len(selected))
	for _, id := range selected {
		selectSet[id] = true
	}

	select {
	case s.refreshGate <- struct{}{}:
		defer func() { <-s.refreshGate }()
	case <-ctx.Done():
		return Snapshot{Status: s.Status()}, ctx.Err()
	}
	s.recordError(nil)

	dates := dateRange(start, end)
	var warnings []string
	for _, date := range dates {
		needBaseball := selectSet["mlb"] && (force || s.needsRefresh("mlb", date, time.Now()))
		needFootball := (selectSet["nfl"] && (force || s.needsRefresh("nfl", date, time.Now()))) ||
			(selectSet["ncaa"] && (force || s.needsRefresh("ncaa", date, time.Now())))
		if needBaseball {
			if err := s.fetchDate(ctx, "mlb", date); err != nil {
				warnings = append(warnings, fmt.Sprintf("MLB %s: %s", date, err))
			}
		}
		if needFootball {
			if err := s.fetchFootballDate(ctx, date); err != nil {
				warnings = append(warnings, fmt.Sprintf("NFL/NCAA %s: %s", date, err))
			}
		}
	}

	snapshot := s.CachedWindow(selected, start, end)
	snapshot.Warnings = append(snapshot.Warnings, warnings...)
	if len(warnings) > 0 {
		s.recordError(errors.New(strings.Join(warnings, "; ")))
		return snapshot, fmt.Errorf("one or more API-Sports schedule requests failed")
	}
	if len(snapshot.Events) == 0 && len(snapshot.Status.Datasets) > 0 && !s.hasCachedDates(selected, dates) {
		return snapshot, fmt.Errorf("API-Sports returned no schedule cache for the requested window")
	}
	return snapshot, nil
}

// CachedWindow reads persisted cache only; it never makes an upstream request.
func (s *Service) CachedWindow(datasetIDs []string, start, end time.Time) Snapshot {
	selected := normalizeDatasetIDs(datasetIDs)
	wanted := map[string]bool{}
	for _, id := range selected {
		wanted[id] = true
	}
	from := start.UTC().Add(-12 * time.Hour)
	through := end.UTC().Add(12 * time.Hour)
	out := Snapshot{Events: []Event{}}
	s.mu.RLock()
	for _, entry := range s.entries {
		if !wanted[entry.Dataset] {
			continue
		}
		for _, event := range entry.Events {
			if event.StartsAt.Before(from) || event.StartsAt.After(through) {
				continue
			}
			out.Events = append(out.Events, event)
		}
	}
	s.mu.RUnlock()
	sort.Slice(out.Events, func(i, j int) bool {
		if out.Events[i].StartsAt.Equal(out.Events[j].StartsAt) {
			return out.Events[i].ID < out.Events[j].ID
		}
		return out.Events[i].StartsAt.Before(out.Events[j].StartsAt)
	})
	out.Events = dedupeEvents(out.Events)
	out.Status = s.Status()
	return out
}

func (s *Service) Status() Status {
	out := Status{Provider: "API-Sports", Datasets: []DatasetStatus{}}
	if s == nil {
		return out
	}
	s.mu.RLock()
	out.Configured = strings.TrimSpace(s.key) != ""
	out.LastRequestAt = formatTime(s.lastRequest)
	out.LastSuccessAt = formatTime(s.lastSuccess)
	out.LastError = s.lastError
	if sameUTCDay(s.quotaAt, time.Now()) {
		out.Quota = cloneQuota(s.quota)
	}
	byDataset := map[string]*DatasetStatus{}
	for _, d := range datasetCatalog {
		byDataset[d.ID] = &DatasetStatus{ID: d.ID, CachedDates: []string{}}
	}
	for _, entry := range s.entries {
		d := byDataset[entry.Dataset]
		if d == nil {
			continue
		}
		d.CachedDates = append(d.CachedDates, entry.Date)
		d.CachedEventCount += len(entry.Events)
		if entry.FetchedAt.After(parseTime(d.LastFetchAt)) {
			d.LastFetchAt = entry.FetchedAt.UTC().Format(time.RFC3339)
			d.LastFetchEventCount = len(entry.Events)
		}
		freshFor := freshness(entry.Date, time.Now())
		if time.Since(entry.FetchedAt) > freshFor {
			d.StaleDateCount++
		}
	}
	for _, d := range datasetCatalog {
		row := byDataset[d.ID]
		sort.Strings(row.CachedDates)
		out.Datasets = append(out.Datasets, *row)
	}
	s.mu.RUnlock()
	return out
}

func (s *Service) needsRefresh(dataset, date string, now time.Time) bool {
	s.mu.RLock()
	entry, ok := s.entries[cacheKey(dataset, date)]
	s.mu.RUnlock()
	return !ok || now.Sub(entry.FetchedAt) > freshness(date, now)
}

func freshness(date string, now time.Time) time.Duration {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return oldScheduleFreshFor
	}
	today := now.UTC().Format("2006-01-02")
	if date == today {
		return footballFreshFor
	}
	if date > today {
		return futureFreshFor
	}
	return oldScheduleFreshFor
}

func (s *Service) fetchDate(ctx context.Context, datasetID, date string) error {
	dataset, _ := DatasetByID(datasetID)
	payload, quota, err := s.request(ctx, dataset.Host, date)
	if err != nil {
		s.recordError(err)
		return err
	}
	events := normalizePayload(dataset, payload)
	s.storeEntry(cacheEntry{Dataset: datasetID, Date: date, FetchedAt: time.Now().UTC(), Events: events}, quota)
	return nil
}

func (s *Service) fetchFootballDate(ctx context.Context, date string) error {
	var host string
	for _, dataset := range datasetCatalog {
		if dataset.ID == "nfl" {
			host = dataset.Host
		}
	}
	payload, quota, err := s.request(ctx, host, date)
	if err != nil {
		s.recordError(err)
		return err
	}
	now := time.Now().UTC()
	for _, id := range []string{"nfl", "ncaa"} {
		dataset, _ := DatasetByID(id)
		events := normalizePayload(dataset, payload)
		s.storeEntry(cacheEntry{Dataset: id, Date: date, FetchedAt: now, Events: events}, quota)
	}
	return nil
}

func (s *Service) request(ctx context.Context, host, date string) (map[string]json.RawMessage, Quota, error) {
	if host == "" {
		return nil, Quota{}, errors.New("unknown API-Sports host")
	}
	key := s.apiKey()
	if key == "" {
		return nil, Quota{}, ErrNotConfigured
	}
	s.mu.RLock()
	quotaExhausted := sameUTCDay(s.quotaAt, time.Now()) && s.quota.DailyRemaining != nil && *s.quota.DailyRemaining <= 0
	s.mu.RUnlock()
	if quotaExhausted {
		return nil, Quota{}, errors.New("API-Sports daily request quota is exhausted")
	}
	select {
	case <-ctx.Done():
		return nil, Quota{}, ctx.Err()
	case <-time.After(s.requestWait()):
	}
	endpoint := url.URL{Scheme: "https", Host: host, Path: "/games"}
	query := endpoint.Query()
	query.Set("date", date)
	query.Set("timezone", "UTC")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, Quota{}, errors.New("could not build API-Sports request")
	}
	// The American Football API can reject automatically supplied User-Agent headers.
	req.Header["User-Agent"] = []string{""}
	req.Header.Set("x-apisports-key", key)
	s.mu.Lock()
	s.lastRequest = time.Now().UTC()
	s.mu.Unlock()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, Quota{}, errors.New("API-Sports request failed")
	}
	defer resp.Body.Close()
	quota := quotaFromHeaders(resp.Header)
	s.mu.Lock()
	s.quota, s.quotaAt = cloneQuota(quota), time.Now().UTC()
	_ = s.saveLocked()
	s.mu.Unlock()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, quota, errors.New("could not read API-Sports response")
	}
	if len(raw) > maxBodyBytes {
		return nil, quota, errors.New("API-Sports response exceeded the 8 MB limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, quota, fmt.Errorf("API-Sports returned HTTP %d", resp.StatusCode)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, quota, errors.New("API-Sports returned invalid JSON")
	}
	if apiErrors := strings.TrimSpace(string(envelope["errors"])); apiErrors != "" && apiErrors != "null" && apiErrors != "{}" && apiErrors != "[]" && apiErrors != `""` {
		return nil, quota, errors.New("API-Sports reported an upstream error")
	}
	return envelope, quota, nil
}

func normalizePayload(dataset Dataset, payload map[string]json.RawMessage) []Event {
	var rows []map[string]any
	if err := json.Unmarshal(payload["response"], &rows); err != nil {
		return []Event{}
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		event, ok := normalizeEvent(dataset, row)
		if ok {
			out = append(out, event)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}

func normalizeEvent(dataset Dataset, root map[string]any) (Event, bool) {
	game := object(root["game"])
	league := object(root["league"])
	if intValue(league["id"]) != dataset.RemoteLeagueID {
		return Event{}, false
	}
	gameID := firstText(root["id"], game["id"])
	if gameID == "" {
		return Event{}, false
	}
	start := eventStart(root, game)
	if start.IsZero() {
		return Event{}, false
	}
	teams := object(root["teams"])
	home := object(teams["home"])
	away := object(teams["away"])
	homeName := text(home["name"])
	awayName := text(away["name"])
	if homeName == "" || awayName == "" {
		return Event{}, false
	}
	status := object(root["status"])
	if len(status) == 0 {
		status = object(game["status"])
	}
	leagueName := text(league["name"])
	return Event{
		ID:           dataset.ID + ":" + gameID,
		Dataset:      dataset.ID,
		League:       leagueName,
		HomeTeam:     homeName,
		HomeTeamCode: text(home["code"]),
		HomeTeamID:   firstText(home["id"]),
		AwayTeam:     awayName,
		AwayTeamCode: text(away["code"]),
		AwayTeamID:   firstText(away["id"]),
		StartsAt:     start.UTC(),
		Status:       text(status["long"]),
		StatusCode:   strings.ToUpper(text(status["short"])),
	}, true
}

func eventStart(root, game map[string]any) time.Time {
	candidates := []map[string]any{root, object(root["date"]), game, object(game["date"])}
	for _, item := range candidates {
		dateOnly := text(item["date"])
		clock := text(item["time"])
		if dateOnly != "" && clock != "" {
			if parsed, err := time.ParseInLocation("2006-01-02 15:04", dateOnly+" "+clock, time.UTC); err == nil {
				return parsed
			}
		}
		for _, key := range []string{"timestamp", "date", "datetime", "time", "starts_at"} {
			value := item[key]
			if key == "timestamp" {
				if n, err := strconv.ParseInt(text(value), 10, 64); err == nil && n > 0 {
					return time.Unix(n, 0).UTC()
				}
				continue
			}
			if raw := text(value); raw != "" {
				for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
					if parsed, err := time.Parse(layout, raw); err == nil {
						return parsed.UTC()
					}
				}
			}
		}
	}
	return time.Time{}
}

func object(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
func text(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}
func firstText(values ...any) string {
	for _, value := range values {
		if got := text(value); got != "" {
			return got
		}
	}
	return ""
}
func intValue(value any) int { n, _ := strconv.Atoi(text(value)); return n }

func (s *Service) storeEntry(entry cacheEntry, quota Quota) {
	s.mu.Lock()
	s.entries[cacheKey(entry.Dataset, entry.Date)] = entry
	s.lastSuccess = entry.FetchedAt
	s.quota = cloneQuota(quota)
	s.quotaAt = entry.FetchedAt
	for key, old := range s.entries {
		if time.Since(old.FetchedAt) > 15*24*time.Hour {
			delete(s.entries, key)
		}
	}
	err := s.saveLocked()
	if err != nil {
		s.lastError = "schedule cache could not be saved"
	}
	s.mu.Unlock()
}

func (s *Service) recordError(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if err == nil {
		s.lastError = ""
		s.mu.Unlock()
		return
	}
	s.lastError = strings.TrimSpace(err.Error())
	if len(s.lastError) > 240 {
		s.lastError = s.lastError[:240]
	}
	s.mu.Unlock()
}

func (s *Service) requestWait() time.Duration {
	s.mu.Lock()
	now := time.Now()
	waitUntil := s.nextRequest
	if waitUntil.Before(now) {
		waitUntil = now
	}
	s.nextRequest = waitUntil.Add(requestSpacing)
	s.mu.Unlock()
	wait := time.Until(waitUntil)
	if wait < 0 {
		return 0
	}
	return wait
}

func (s *Service) hasCachedDates(datasets []string, dates []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, dataset := range datasets {
		for _, date := range dates {
			if _, ok := s.entries[cacheKey(dataset, date)]; ok {
				return true
			}
		}
	}
	return false
}

func normalizeDatasetIDs(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range ids {
		id := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := DatasetByID(id); !ok || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func dateRange(start, end time.Time) []string {
	first := time.Date(start.UTC().Year(), start.UTC().Month(), start.UTC().Day(), 0, 0, 0, 0, time.UTC)
	last := time.Date(end.UTC().Year(), end.UTC().Month(), end.UTC().Day(), 0, 0, 0, 0, time.UTC)
	var out []string
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		out = append(out, day.Format("2006-01-02"))
	}
	return out
}

func dedupeEvents(events []Event) []Event {
	seen := map[string]bool{}
	out := events[:0]
	for _, event := range events {
		if !seen[event.ID] {
			seen[event.ID] = true
			out = append(out, event)
		}
	}
	return out
}

func quotaFromHeaders(header http.Header) Quota {
	return Quota{
		DailyLimit:      headerInt(header, "x-ratelimit-requests-limit"),
		DailyRemaining:  headerInt(header, "x-ratelimit-requests-remaining"),
		MinuteLimit:     headerInt(header, "x-ratelimit-limit"),
		MinuteRemaining: headerInt(header, "x-ratelimit-remaining"),
	}
}
func headerInt(header http.Header, name string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(header.Get(name)))
	if err != nil {
		return nil
	}
	return &n
}
func cloneQuota(q Quota) Quota {
	clone := func(v *int) *int {
		if v == nil {
			return nil
		}
		n := *v
		return &n
	}
	return Quota{DailyLimit: clone(q.DailyLimit), DailyRemaining: clone(q.DailyRemaining), MinuteLimit: clone(q.MinuteLimit), MinuteRemaining: clone(q.MinuteRemaining)}
}
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func sameUTCDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
func parseTime(value string) time.Time { parsed, _ := time.Parse(time.RFC3339, value); return parsed }

func (s *Service) load() {
	if s == nil || s.cachePath == "" {
		return
	}
	raw, err := os.ReadFile(s.cachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.lastError = "schedule cache could not be read"
		}
		return
	}
	var state diskState
	if json.Unmarshal(raw, &state) != nil || state.Version != 1 {
		s.lastError = "schedule cache is invalid"
		return
	}
	for _, entry := range state.Entries {
		if _, ok := DatasetByID(entry.Dataset); !ok || entry.FetchedAt.IsZero() {
			continue
		}
		if _, err := time.Parse("2006-01-02", entry.Date); err != nil {
			continue
		}
		s.entries[cacheKey(entry.Dataset, entry.Date)] = entry
	}
	s.lastRequest, s.lastSuccess, s.lastError, s.quota, s.quotaAt = state.LastRequestAt, state.LastSuccessAt, state.LastError, cloneQuota(state.Quota), state.QuotaAt
}

func (s *Service) saveLocked() error {
	if s.cachePath == "" {
		return nil
	}
	state := diskState{Version: 1, LastRequestAt: s.lastRequest, LastSuccessAt: s.lastSuccess, LastError: s.lastError, Quota: cloneQuota(s.quota), QuotaAt: s.quotaAt}
	for _, entry := range s.entries {
		state.Entries = append(state.Entries, entry)
	}
	sort.Slice(state.Entries, func(i, j int) bool {
		if state.Entries[i].Dataset == state.Entries[j].Dataset {
			return state.Entries[i].Date < state.Entries[j].Date
		}
		return state.Entries[i].Dataset < state.Entries[j].Dataset
	})
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Clean(s.cachePath))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sports-schedule-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.cachePath); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
