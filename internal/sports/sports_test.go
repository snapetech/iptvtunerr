package sports

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestFetchDateNBAUsesSeasonQueryAndNormalizesSchedule(t *testing.T) {
	const date = "2026-10-03"
	const payload = `{"errors":[],"response":[{"id":"nba-game-1","league":"standard","season":2026,"date":{"start":"2026-10-03T23:30:00.000Z","end":null,"duration":null},"status":{"short":"1","long":"Not Started"},"teams":{"visitors":{"id":14,"name":"Los Angeles Lakers"},"home":{"id":2,"name":"Dallas Mavericks"}}}]}`

	service := NewService("test-api-key", "")
	var requestSeen *http.Request
	service.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestSeen = request.Clone(request.Context())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
			Request:    request,
		}, nil
	})}

	if err := service.fetchDate(context.Background(), "nba", date); err != nil {
		t.Fatalf("fetch NBA schedule: %v", err)
	}
	if requestSeen == nil {
		t.Fatal("NBA request was not sent")
	}
	if requestSeen.Method != http.MethodGet || requestSeen.URL.Host != "v2.nba.api-sports.io" || requestSeen.URL.Path != "/games" {
		t.Fatalf("request target = %s %s, want GET https://v2.nba.api-sports.io/games", requestSeen.Method, requestSeen.URL)
	}
	query := requestSeen.URL.Query()
	if query.Get("date") != date || query.Get("league") != "standard" || query.Get("season") != "2026" {
		t.Fatalf("NBA request query = %v, want date=%s league=standard season=2026", query, date)
	}
	if query.Has("timezone") {
		t.Fatalf("NBA request unexpectedly set timezone: %v", query)
	}
	if got := requestSeen.Header.Get("x-apisports-key"); got != "test-api-key" {
		t.Fatalf("API key header = %q, want test key", got)
	}

	entry, ok := service.entries[cacheKey("nba", date)]
	if !ok || len(entry.Events) != 1 {
		t.Fatalf("NBA cache entry = %+v, exists=%v; want one normalized event", entry, ok)
	}
	event := entry.Events[0]
	if event.ID != "nba:nba-game-1" || event.Dataset != "nba" || event.League != "standard" {
		t.Fatalf("normalized NBA identity = %+v", event)
	}
	if event.HomeTeam != "Dallas Mavericks" || event.HomeTeamID != "2" || event.AwayTeam != "Los Angeles Lakers" || event.AwayTeamID != "14" {
		t.Fatalf("normalized NBA teams = %+v", event)
	}
	wantStart := time.Date(2026, time.October, 3, 23, 30, 0, 0, time.UTC)
	if !event.StartsAt.Equal(wantStart) || event.Status != "Not Started" || event.StatusCode != "1" {
		t.Fatalf("normalized NBA date/status = %+v", event)
	}
}

func TestSeasonStartYear(t *testing.T) {
	tests := []struct {
		date  string
		month time.Month
		want  int
	}{
		{date: "2026-10-03", month: time.October, want: 2026},
		{date: "2026-03-02", month: time.October, want: 2025},
		{date: "bad-date", month: time.October, want: 0},
		{date: "2026-10-03", month: 0, want: 0},
	}
	for _, test := range tests {
		if got := seasonStartYear(test.date, test.month); got != test.want {
			t.Errorf("seasonStartYear(%q, %d) = %d, want %d", test.date, test.month, got, test.want)
		}
	}
}

func TestRequestTracksAndPersistsQuotaPerAPIProduct(t *testing.T) {
	service := NewService("test-api-key", filepath.Join(t.TempDir(), "sports-cache.json"))
	requests := 0
	service.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		remaining := "0"
		if request.URL.Host == "v1.american-football.api-sports.io" {
			remaining = "17"
		}
		header := make(http.Header)
		header.Set("x-ratelimit-requests-limit", "100")
		header.Set("x-ratelimit-requests-remaining", remaining)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[],"response":[]}`)),
			Request:    request,
		}, nil
	})}

	nba, _ := DatasetByID("nba")
	football, _ := DatasetByID("nfl")
	if _, _, err := service.request(context.Background(), nba, "2026-10-03"); err != nil {
		t.Fatalf("first NBA request: %v", err)
	}
	if _, _, err := service.request(context.Background(), nba, "2026-10-04"); err == nil || !strings.Contains(err.Error(), "quota is exhausted") {
		t.Fatalf("second NBA request error = %v, want per-product quota exhausted", err)
	}
	if requests != 1 {
		t.Fatalf("made %d requests after NBA quota was exhausted, want 1", requests)
	}

	service.nextRequest = time.Now().Add(-requestSpacing)
	if _, _, err := service.request(context.Background(), football, "2026-10-03"); err != nil {
		t.Fatalf("football request after NBA quota exhaustion: %v", err)
	}
	if requests != 2 {
		t.Fatalf("made %d requests after the football request, want 2", requests)
	}

	reloaded := NewService("test-api-key", service.cachePath)
	status := reloaded.Status()
	for _, dataset := range status.Datasets {
		switch dataset.ID {
		case "nba":
			if dataset.Quota == nil || dataset.Quota.DailyRemaining == nil || *dataset.Quota.DailyRemaining != 0 {
				t.Errorf("NBA quota after reload = %+v, want 0 remaining", dataset.Quota)
			}
		case "nfl", "ncaa":
			if dataset.Quota == nil || dataset.Quota.DailyRemaining == nil || *dataset.Quota.DailyRemaining != 17 {
				t.Errorf("%s quota after reload = %+v, want 17 remaining", dataset.ID, dataset.Quota)
			}
		}
	}
	if _, _, err := reloaded.request(context.Background(), nba, "2026-10-05"); err == nil || !strings.Contains(err.Error(), "quota is exhausted") {
		t.Errorf("NBA request after service reload error = %v, want persisted quota exhaustion", err)
	}
}
