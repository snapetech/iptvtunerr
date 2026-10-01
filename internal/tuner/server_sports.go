package tuner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/snapetech/iptvtunerr/internal/catalog"
	"github.com/snapetech/iptvtunerr/internal/sports"
)

type SportsAutomationView struct {
	GeneratedAt      string                    `json:"generated_at"`
	Configured       bool                      `json:"configured"`
	SettingsWritable bool                      `json:"settings_writable"`
	APIKeyConfigured bool                      `json:"api_key_configured"`
	Datasets         []sports.Dataset          `json:"datasets"`
	AvailableTeams   []SportsTeamOption        `json:"available_teams"`
	Settings         sports.AutomationSettings `json:"settings"`
	Status           sports.Status             `json:"status"`
	Report           SportsAutomationReport    `json:"report"`
}

type SportsTeamOption struct {
	Dataset string `json:"dataset"`
	Team    string `json:"team"`
}

type SportsAutomationReport struct {
	GeneratedAt         string               `json:"generated_at"`
	Enabled             bool                 `json:"enabled"`
	GuideReady          bool                 `json:"guide_ready"`
	ScheduleEventCount  int                  `json:"schedule_event_count"`
	MatchedEventCount   int                  `json:"matched_event_count"`
	UnmatchedEventCount int                  `json:"unmatched_event_count"`
	GeneratedCount      int                  `json:"generated_count"`
	Warnings            []string             `json:"warnings,omitempty"`
	Events              []SportsEventReport  `json:"events"`
	Channels            []SportsEventChannel `json:"channels"`
}

type SportsEventReport struct {
	Event    sports.Event         `json:"event"`
	Matched  bool                 `json:"matched"`
	Reason   string               `json:"reason,omitempty"`
	Channels []SportsEventChannel `json:"channels,omitempty"`
}

type SportsEventChannel struct {
	ID                string    `json:"id"` // stable event+feed identity; independent of reusable guide number
	EventID           string    `json:"event_id"`
	Dataset           string    `json:"dataset"`
	Name              string    `json:"name"`
	GuideID           string    `json:"guide_id"`
	GuideNumber       string    `json:"guide_number"`
	StartsAt          time.Time `json:"starts_at"`
	StopsAt           time.Time `json:"stops_at"`
	StreamChannelID   string    `json:"stream_channel_id"`
	SourceChannelName string    `json:"source_channel_name"`
	SourceGuideNumber string    `json:"source_guide_number,omitempty"`
	SourceProgramme   string    `json:"source_programme"`
	MatchConfidence   string    `json:"match_confidence"`
	PlayURL           string    `json:"play_url"`
}

type sportsGuideMatch struct {
	channel catalog.LiveChannel
	program xmlProgramme
	score   int
	start   time.Time
	stop    time.Time
}

const sportsGuideMatchWindow = 3 * time.Hour

func (s *Server) sportsSettings() sports.AutomationSettings {
	path := strings.TrimSpace(s.SportsAutomationFile)
	settings, err := sports.LoadAutomationSettings(path)
	if err != nil {
		return sports.DefaultAutomationSettings()
	}
	return settings
}

func (s *Server) sportsSnapshot(settings sports.AutomationSettings) sports.Snapshot {
	if s == nil || s.Sports == nil {
		return sports.Snapshot{Events: []sports.Event{}, Status: sports.Status{Provider: "API-Sports", Datasets: []sports.DatasetStatus{}}}
	}
	now := time.Now().UTC()
	start := now.Add(-time.Duration(settings.PastHours) * time.Hour)
	end := now.Add(time.Duration(settings.LookaheadDays) * 24 * time.Hour)
	return s.Sports.CachedWindow(settings.Datasets, start, end)
}

func (s *Server) buildSportsAutomationReport(settings sports.AutomationSettings, snapshot sports.Snapshot) SportsAutomationReport {
	now := time.Now().UTC()
	report := SportsAutomationReport{
		GeneratedAt: now.Format(time.RFC3339),
		Enabled:     settings.Enabled,
		Events:      []SportsEventReport{},
		Channels:    []SportsEventChannel{},
		Warnings:    append([]string(nil), snapshot.Warnings...),
	}
	if snapshot.Status.LastError != "" {
		report.Warnings = append(report.Warnings, snapshot.Status.LastError)
	}
	if !settings.Enabled {
		return report
	}
	var guideData []byte
	if s != nil && s.xmltv != nil {
		s.xmltv.mu.RLock()
		guideData = append([]byte(nil), s.xmltv.cachedXML...)
		s.xmltv.mu.RUnlock()
	}
	if len(guideData) == 0 {
		report.Warnings = append(report.Warnings, "Tunerr's merged XMLTV guide is not ready yet.")
		return report
	}
	var guide xmlTVRoot
	if err := xml.Unmarshal(guideData, &guide); err != nil {
		report.Warnings = append(report.Warnings, "Tunerr's merged XMLTV guide could not be parsed.")
		return report
	}
	report.GuideReady = true
	channels := s.sportsLineupSnapshot()
	channelByGuideID := map[string]catalog.LiveChannel{}
	for _, channel := range channels {
		if strings.TrimSpace(channel.ChannelID) == "" || !liveChannelHasStream(channel) {
			continue
		}
		plexSafe := s.xmltv != nil && s.xmltv.PlexSafeIDs
		for _, id := range sportsGuideIDs(channel, plexSafe) {
			channelByGuideID[id] = channel
		}
	}
	guideCandidates := make([]sportsGuideMatch, 0, len(guide.Programmes))
	for _, program := range guide.Programmes {
		channel, ok := channelByGuideID[strings.TrimSpace(program.Channel)]
		if !ok {
			continue
		}
		start, okStart := parseXMLTVTime(program.Start)
		stop, okStop := parseXMLTVTime(program.Stop)
		if !okStart || !okStop || !stop.After(start) {
			continue
		}
		guideCandidates = append(guideCandidates, sportsGuideMatch{channel: channel, program: program, start: start, stop: stop})
	}
	sort.Slice(guideCandidates, func(i, j int) bool { return guideCandidates[i].start.Before(guideCandidates[j].start) })

	allowedDatasets := map[string]bool{}
	for _, id := range settings.Datasets {
		allowedDatasets[id] = true
	}
	teamRules := map[string][]string{}
	for _, rule := range settings.TeamRules {
		teamRules[rule.Dataset] = append(teamRules[rule.Dataset], rule.Team)
	}
	now = now.UTC()
	for _, event := range snapshot.Events {
		if !allowedDatasets[event.Dataset] || eventIsCancelled(event) || eventIsFinished(event) {
			continue
		}
		if event.StartsAt.Before(now.Add(-time.Duration(settings.PastHours)*time.Hour)) || event.StartsAt.After(now.Add(time.Duration(settings.LookaheadDays)*24*time.Hour)) {
			continue
		}
		if rules := teamRules[event.Dataset]; len(rules) > 0 && !eventIncludesRuleTeam(event, rules) {
			continue
		}
		report.ScheduleEventCount++
		outcome := SportsEventReport{Event: event, Channels: []SportsEventChannel{}}
		matches := matchSportsEventToGuide(event, guideCandidates)
		if len(matches) == 0 {
			outcome.Reason = "No current lineup channel has an XMLTV programme containing both teams near the scheduled start."
			report.UnmatchedEventCount++
			report.Events = append(report.Events, outcome)
			continue
		}
		if len(matches) > settings.MaxChannelsPerEvent {
			matches = matches[:settings.MaxChannelsPerEvent]
		}
		outcome.Matched = true
		report.MatchedEventCount++
		for _, match := range matches {
			channel := generatedSportsChannel(event, match)
			outcome.Channels = append(outcome.Channels, channel)
			report.Channels = append(report.Channels, channel)
		}
		report.Events = append(report.Events, outcome)
	}
	sort.Slice(report.Events, func(i, j int) bool { return report.Events[i].Event.StartsAt.Before(report.Events[j].Event.StartsAt) })
	sort.Slice(report.Channels, func(i, j int) bool {
		if report.Channels[i].Dataset == report.Channels[j].Dataset {
			if report.Channels[i].StartsAt.Equal(report.Channels[j].StartsAt) {
				return report.Channels[i].ID < report.Channels[j].ID
			}
			return report.Channels[i].StartsAt.Before(report.Channels[j].StartsAt)
		}
		return report.Channels[i].Dataset < report.Channels[j].Dataset
	})
	numberedChannels, truncated := assignSportsGuideNumbers(report.Channels)
	report.Channels = numberedChannels
	if truncated > 0 {
		report.Warnings = append(report.Warnings, "A league feed reached its 1,000-number channel block limit; excess feeds were omitted.")
	}
	channelIDs := make(map[string]bool, len(report.Channels))
	for _, channel := range report.Channels {
		channelIDs[channel.ID] = true
	}
	report.MatchedEventCount = 0
	report.UnmatchedEventCount = 0
	for i := range report.Events {
		assigned := report.Events[i].Channels[:0]
		for _, channel := range report.Events[i].Channels {
			if channelIDs[channel.ID] {
				for _, numbered := range report.Channels {
					if numbered.ID == channel.ID {
						channel.GuideNumber = numbered.GuideNumber
						break
					}
				}
				assigned = append(assigned, channel)
			}
		}
		report.Events[i].Channels = assigned
		report.Events[i].Matched = len(assigned) > 0
		if report.Events[i].Matched {
			report.MatchedEventCount++
		} else if report.Events[i].Reason == "" {
			report.Events[i].Reason = "The generated sports channel number block is full."
			report.UnmatchedEventCount++
		} else {
			report.UnmatchedEventCount++
		}
	}
	report.GeneratedCount = len(report.Channels)
	return report
}

func matchSportsEventToGuide(event sports.Event, candidates []sportsGuideMatch) []sportsGuideMatch {
	var matches []sportsGuideMatch
	from, through := event.StartsAt.Add(-sportsGuideMatchWindow), event.StartsAt.Add(sportsGuideMatchWindow)
	first := sort.Search(len(candidates), func(i int) bool { return !candidates[i].start.Before(from) })
	for i := first; i < len(candidates) && !candidates[i].start.After(through); i++ {
		candidate := candidates[i]
		program, channel := candidate.program, candidate.channel
		text := strings.Join([]string{channel.GuideName, program.Title.Value, program.SubTitle.Value, program.Desc.Value}, " ")
		home, homeScore := teamTextMatch(text, event.HomeTeam, event.HomeTeamCode)
		away, awayScore := teamTextMatch(text, event.AwayTeam, event.AwayTeamCode)
		if !home || !away {
			continue
		}
		score := homeScore + awayScore
		score += 30
		if textHasTeam(program.Title.Value, event.HomeTeam, event.HomeTeamCode) && textHasTeam(program.Title.Value, event.AwayTeam, event.AwayTeamCode) {
			score += 35
		}
		if strings.TrimSpace(program.SubTitle.Value) != "" {
			score += 2
		}
		candidate.score = score
		matches = append(matches, candidate)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		left, right := absDuration(matches[i].start.Sub(event.StartsAt)), absDuration(matches[j].start.Sub(event.StartsAt))
		if left != right {
			return left < right
		}
		return matches[i].channel.ChannelID < matches[j].channel.ChannelID
	})
	seen := map[string]bool{}
	out := matches[:0]
	for _, match := range matches {
		if seen[match.channel.ChannelID] {
			continue
		}
		seen[match.channel.ChannelID] = true
		out = append(out, match)
	}
	return out
}

func teamTextMatch(text, name, code string) (bool, int) {
	if textHasPhrase(text, name) {
		return true, 40
	}
	if code != "" && len([]rune(strings.TrimSpace(code))) >= 3 && textHasToken(text, code) {
		return true, 25
	}
	return false, 0
}

func textHasTeam(text, name, code string) bool { ok, _ := teamTextMatch(text, name, code); return ok }

func textHasPhrase(text, phrase string) bool {
	needle := normalizeSportsText(phrase)
	if len([]rune(needle)) < 4 {
		return false
	}
	return strings.Contains(" "+normalizeSportsText(text)+" ", " "+needle+" ")
}

func textHasToken(text, token string) bool {
	needle := normalizeSportsText(token)
	if needle == "" {
		return false
	}
	for _, part := range strings.Fields(normalizeSportsText(text)) {
		if part == needle {
			return true
		}
	}
	return false
}

func normalizeSportsText(raw string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(raw) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func eventIncludesRuleTeam(event sports.Event, teams []string) bool {
	for _, team := range teams {
		if strings.EqualFold(strings.TrimSpace(team), strings.TrimSpace(event.HomeTeam)) || strings.EqualFold(strings.TrimSpace(team), strings.TrimSpace(event.AwayTeam)) {
			return true
		}
	}
	return false
}

func eventIsCancelled(event sports.Event) bool {
	code := strings.ToUpper(strings.TrimSpace(event.StatusCode))
	status := strings.ToLower(strings.TrimSpace(event.Status))
	return code == "CANC" || code == "CAN" || code == "PST" || code == "POST" || code == "ABD" || code == "SUSP" || strings.Contains(status, "cancel") || strings.Contains(status, "postpon") || strings.Contains(status, "abandon")
}

func eventIsFinished(event sports.Event) bool {
	code := strings.ToUpper(strings.TrimSpace(event.StatusCode))
	status := strings.ToLower(strings.TrimSpace(event.Status))
	return code == "FT" || code == "AOT" || code == "F" || code == "FINAL" || strings.Contains(status, "finished") || strings.Contains(status, "final")
}

func generatedSportsChannel(event sports.Event, match sportsGuideMatch) SportsEventChannel {
	dataset, _ := sports.DatasetByID(event.Dataset)
	teamTitle := event.AwayTeam + " at " + event.HomeTeam
	channelHash := sha256.Sum256([]byte(match.channel.ChannelID))
	id := fmt.Sprintf("sports.%s.%s.%s", dataset.ID, strings.TrimPrefix(event.ID, dataset.ID+":"), hex.EncodeToString(channelHash[:4]))
	channelName := strings.TrimSpace(match.channel.GuideName)
	if channelName == "" {
		channelName = "Live feed"
	}
	name := dataset.Name + " | " + teamTitle + " · " + channelName
	stop := event.StartsAt.Add(defaultSportsDuration(event.Dataset))
	if match.stop.After(event.StartsAt) && match.stop.Before(event.StartsAt.Add(8*time.Hour)) {
		stop = match.stop
	}
	if stop.Before(event.StartsAt.Add(30 * time.Minute)) {
		stop = event.StartsAt.Add(defaultSportsDuration(event.Dataset))
	}
	guideID := id
	playURL := "/stream/" + strings.TrimSpace(match.channel.ChannelID)
	confidence := "medium"
	if match.score >= 100 {
		confidence = "high"
	}
	return SportsEventChannel{
		ID: id, EventID: event.ID, Dataset: event.Dataset, Name: name, GuideID: guideID,
		StartsAt: event.StartsAt, StopsAt: stop, StreamChannelID: match.channel.ChannelID,
		SourceChannelName: strings.TrimSpace(match.channel.GuideName), SourceGuideNumber: strings.TrimSpace(match.channel.GuideNumber),
		SourceProgramme: strings.TrimSpace(match.program.Title.Value), MatchConfidence: confidence, PlayURL: playURL,
	}
}

func defaultSportsDuration(dataset string) time.Duration {
	if dataset == "mlb" {
		return 5 * time.Hour
	}
	return 4 * time.Hour
}

func assignSportsGuideNumbers(channels []SportsEventChannel) ([]SportsEventChannel, int) {
	counts := map[string]int{}
	out := make([]SportsEventChannel, 0, len(channels))
	truncated := 0
	for i := range channels {
		dataset, ok := sports.DatasetByID(channels[i].Dataset)
		if !ok {
			continue
		}
		slot := counts[dataset.ID]
		if slot >= 1000 {
			truncated++
			continue
		}
		counts[dataset.ID]++
		channels[i].GuideNumber = fmt.Sprintf("%d", dataset.GuideBlock+slot)
		out = append(out, channels[i])
	}
	return out, truncated
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func sportsGuideIDs(channel catalog.LiveChannel, plexSafe bool) []string {
	ids := []string{
		xmltvChannelIDForChannel(channel, plexSafe),
		xmltvChannelIDForChannel(channel, false),
		strings.TrimSpace(channel.GuideNumber),
		strings.TrimSpace(channel.ChannelID),
		strings.TrimSpace(channel.TVGID),
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func liveChannelHasStream(channel catalog.LiveChannel) bool {
	if strings.TrimSpace(channel.StreamURL) != "" {
		return true
	}
	for _, value := range channel.StreamURLs {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func (s *Server) sportsLineupSnapshot() []catalog.LiveChannel {
	if s == nil {
		return nil
	}
	s.channelsMu.RLock()
	out := cloneLiveChannels(s.Channels)
	s.channelsMu.RUnlock()
	return out
}

func (s *Server) sportsAutomationView() SportsAutomationView {
	settings, err := sports.LoadAutomationSettings(strings.TrimSpace(s.SportsAutomationFile))
	if err != nil {
		settings = sports.DefaultAutomationSettings()
	}
	snapshot := s.sportsSnapshot(settings)
	report := s.buildSportsAutomationReport(settings, snapshot)
	return SportsAutomationView{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), Configured: snapshot.Status.Configured,
		SettingsWritable: strings.TrimSpace(s.SportsAutomationFile) != "",
		APIKeyConfigured: snapshot.Status.Configured, Datasets: sports.Datasets(), AvailableTeams: sportsAvailableTeams(settings, snapshot), Settings: settings,
		Status: snapshot.Status, Report: report,
	}
}

func sportsAvailableTeams(settings sports.AutomationSettings, snapshot sports.Snapshot) []SportsTeamOption {
	allowed := make(map[string]bool, len(settings.Datasets))
	for _, dataset := range settings.Datasets {
		allowed[dataset] = true
	}
	teams := make(map[string]SportsTeamOption)
	add := func(dataset, team string) {
		dataset = strings.TrimSpace(dataset)
		team = strings.TrimSpace(team)
		if team == "" || !allowed[dataset] {
			return
		}
		key := dataset + "\x00" + strings.ToLower(team)
		teams[key] = SportsTeamOption{Dataset: dataset, Team: team}
	}
	for _, event := range snapshot.Events {
		add(event.Dataset, event.HomeTeam)
		add(event.Dataset, event.AwayTeam)
	}
	for _, rule := range settings.TeamRules {
		add(rule.Dataset, rule.Team)
	}
	out := make([]SportsTeamOption, 0, len(teams))
	for _, team := range teams {
		out = append(out, team)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dataset == out[j].Dataset {
			return strings.ToLower(out[i].Team) < strings.ToLower(out[j].Team)
		}
		return out[i].Dataset < out[j].Dataset
	})
	return out
}

func (s *Server) serveSportsAutomation() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method {
		case http.MethodGet:
			if !operatorUIAllowed(w, r) {
				return
			}
			writeSportsJSON(w, http.StatusOK, s.sportsAutomationView())
		case http.MethodPatch, http.MethodPut:
			if !operatorUIAllowed(w, r) {
				return
			}
			if strings.TrimSpace(s.SportsAutomationFile) == "" {
				writeServerJSONError(w, http.StatusServiceUnavailable, "sports settings file is not configured")
				return
			}
			limited := http.MaxBytesReader(w, r.Body, 1<<20)
			defer limited.Close()
			var settings sports.AutomationSettings
			dec := json.NewDecoder(limited)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&settings); err != nil {
				writeServerJSONError(w, http.StatusBadRequest, "invalid sports automation settings")
				return
			}
			saved, err := sports.SaveAutomationSettings(s.SportsAutomationFile, settings)
			if err != nil {
				writeServerJSONError(w, http.StatusBadRequest, "could not save sports automation settings")
				return
			}
			writeSportsJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": saved, "view": s.sportsAutomationView()})
		default:
			writeMethodNotAllowedJSON(w, http.MethodGet, http.MethodPatch, http.MethodPut)
		}
	})
}

func (s *Server) serveSportsEvents() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowedJSON(w, http.MethodGet)
			return
		}
		if !operatorUIAllowed(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeSportsJSON(w, http.StatusOK, s.sportsAutomationView().Report)
	})
}

func (s *Server) serveSportsRefresh() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowedJSON(w, http.MethodPost)
			return
		}
		if !operatorUIAllowed(w, r) {
			return
		}
		if s.Sports == nil {
			writeServerJSONError(w, http.StatusServiceUnavailable, "API-Sports service unavailable")
			return
		}
		settings := s.sportsSettings()
		if !settings.Enabled || len(settings.Datasets) == 0 {
			writeServerJSONError(w, http.StatusConflict, "enable at least one supported schedule dataset first")
			return
		}
		now := time.Now().UTC()
		snapshot, err := s.Sports.RefreshWindow(r.Context(), settings.Datasets, now.Add(-time.Duration(settings.PastHours)*time.Hour), now.Add(time.Duration(settings.LookaheadDays)*24*time.Hour), true)
		report := s.buildSportsAutomationReport(settings, snapshot)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err != nil && len(snapshot.Events) == 0 {
			writeSportsJSON(w, http.StatusBadGateway, map[string]any{"error": "schedule refresh failed", "detail": err.Error(), "report": report, "status": snapshot.Status})
			return
		}
		writeSportsJSON(w, http.StatusOK, map[string]any{"ok": err == nil, "warning": errorText(err), "report": report, "status": snapshot.Status})
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) serveSportsEventM3U() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodHead)
			return
		}
		view := s.sportsAutomationView()
		base := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
		if base == "" {
			base = "http://localhost:5004"
		}
		w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = fmt.Fprintf(w, "#EXTM3U url-tvg=\"%s/sports/guide.xml\"\n", base)
		for _, channel := range view.Report.Channels {
			_, _ = fmt.Fprintf(w, "#EXTINF:-1 tvg-id=\"%s\" tvg-name=\"%s\" tvg-chno=\"%s\" group-title=\"Sports | %s\",%s\n%s\n",
				sportsM3UAttr(channel.GuideID), sportsM3UAttr(channel.Name), sportsM3UAttr(channel.GuideNumber), sportsM3UAttr(datasetName(channel.Dataset)), sportsM3UAttr(channel.Name), base+channel.PlayURL)
		}
	})
}

func (s *Server) serveSportsEventGuide() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodHead)
			return
		}
		view := s.sportsAutomationView()
		root := xmlTVRoot{XMLName: xml.Name{Local: "tv"}, Source: "IPTV Tunerr Sports Automation"}
		for _, channel := range view.Report.Channels {
			root.Channels = append(root.Channels, xmlChannel{ID: channel.GuideID, DisplayNames: []xmlValue{{Value: channel.Name}}})
			root.Programmes = append(root.Programmes, xmlProgramme{
				Start: formatXMLTVTime(channel.StartsAt), Stop: formatXMLTVTime(channel.StopsAt), Channel: channel.GuideID,
				Title: xmlValue{Value: channel.Name}, SubTitle: xmlValue{Value: channel.SourceProgramme},
				Desc:       xmlValue{Value: "Matched from " + channel.SourceChannelName + " in Tunerr's merged XMLTV guide."},
				Categories: []xmlValue{{Value: "Sports"}},
			})
		}
		data, err := xml.MarshalIndent(root, "", "  ")
		if err != nil {
			writeServerJSONError(w, http.StatusInternalServerError, "encode sports guide")
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(append([]byte(xml.Header), data...))
		}
	})
}

func datasetName(id string) string {
	dataset, ok := sports.DatasetByID(id)
	if !ok {
		return "Sports"
	}
	return dataset.Name
}

func sportsM3UAttr(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", `"`, "'").Replace(strings.TrimSpace(value))
	return strings.ReplaceAll(value, ",", " ")
}

func writeSportsJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		writeServerJSONError(w, http.StatusInternalServerError, "encode sports response")
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *Server) startSportsScheduleRefresh(ctx context.Context) {
	if s == nil || s.Sports == nil {
		return
	}
	go func() {
		run := func() {
			settings := s.sportsSettings()
			if !settings.Enabled || len(settings.Datasets) == 0 || !s.Sports.Status().Configured {
				return
			}
			now := time.Now().UTC()
			_, err := s.Sports.RefreshWindow(ctx, settings.Datasets, now.Add(-time.Duration(settings.PastHours)*time.Hour), now.Add(time.Duration(settings.LookaheadDays)*24*time.Hour), false)
			if err != nil && ctx.Err() == nil {
				log.Printf("Sports Automation schedule refresh had unavailable dates: %v", err)
			}
		}
		run()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
