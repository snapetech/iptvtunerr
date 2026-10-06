package tuner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/snapetech/iptvtunerr/internal/catalog"
)

func TestRecordingRuleTitleEqualsIsExact(t *testing.T) {
	rule := RecordingRule{ID: "r", Enabled: true, TitleEquals: []string{"The  News"}}
	if !matchRecordingRuleCapsule(rule, CatchupCapsule{Title: "the news"}) {
		t.Fatal("expected case- and space-insensitive exact match")
	}
	if matchRecordingRuleCapsule(rule, CatchupCapsule{Title: "The News Tonight"}) {
		t.Fatal("title_equals must not match a longer title")
	}
	if !matchRecordingRuleItem(rule, CatchupRecorderItem{Title: "THE NEWS"}) {
		t.Fatal("history matching must use title_equals too")
	}
}

func TestRecordingRuleStartWindow(t *testing.T) {
	rule := RecordingRule{
		ID:          "r",
		Enabled:     true,
		StartAfter:  "2026-10-06T17:58:00Z",
		StartBefore: "2026-10-06T18:02:00Z",
	}
	cases := map[string]bool{
		"2026-10-06T17:58:00Z": true, // inclusive lower bound
		"2026-10-06T18:00:00Z": true,
		"2026-10-06T18:02:00Z": true, // inclusive upper bound
		"2026-10-06T18:03:00Z": false,
		"2026-10-07T18:00:00Z": false, // same slot next day
		"not-a-time":           false,
	}
	for start, want := range cases {
		if got := matchRecordingRuleCapsule(rule, CatchupCapsule{Start: start}); got != want {
			t.Errorf("start %s: got %v want %v", start, got, want)
		}
	}
	if !matchRecordingRuleItem(rule, CatchupRecorderItem{Start: "2026-10-06T18:01:00Z"}) {
		t.Error("history matching must apply the start window")
	}
	bad := RecordingRule{ID: "b", Enabled: true, StartAfter: "yesterday"}
	if matchRecordingRuleCapsule(bad, CatchupCapsule{Start: "2026-10-06T18:00:00Z"}) {
		t.Error("an unparsable bound must match nothing")
	}
}

func TestRecordingRulesEndpointReportsFeaturesWithoutPersistingThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	s := &Server{RecordingRulesFile: path}
	body := bytes.NewBufferString(`{"action":"upsert","rule":{"id":"seerrng-1","name":"SeerrNG #1","enabled":true,"title_equals":["News"],"start_after":"2026-10-06T17:58:00Z","start_before":"2026-10-06T18:02:00Z","features":["x"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/recordings/rules.json", body)
	req.RemoteAddr = "127.0.0.1:1"
	w := httptest.NewRecorder()
	s.serveRecordingRules().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var set RecordingRuleset
	if err := json.Unmarshal(w.Body.Bytes(), &set); err != nil {
		t.Fatal(err)
	}
	if strings.Join(set.Features, ",") != "rules_only_recorder,start_window,title_equals" {
		t.Fatalf("features=%v", set.Features)
	}
	if len(set.Rules) != 1 || set.Rules[0].StartAfter == "" || len(set.Rules[0].TitleEquals) != 1 {
		t.Fatalf("rule fields not kept: %+v", set.Rules)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "features") {
		t.Fatalf("features must not be persisted: %s", raw)
	}

	get := httptest.NewRequest(http.MethodGet, "/recordings/rules.json", nil)
	get.RemoteAddr = "127.0.0.1:1"
	w = httptest.NewRecorder()
	s.serveRecordingRules().ServeHTTP(w, get)
	if !strings.Contains(w.Body.String(), `"title_equals"`) || !strings.Contains(w.Body.String(), `"features"`) {
		t.Fatalf("GET body=%s", w.Body.String())
	}
}

func rulesOnlyManager(t *testing.T, rulesFile string) *catchupRecorderManager {
	t.Helper()
	dir := t.TempDir()
	m, err := newCatchupRecorderManager(CatchupRecorderDaemonConfig{
		OutDir:             dir,
		MaxConcurrency:     1,
		LeadTime:           5 * time.Minute,
		RulesOnly:          true,
		RecordingRulesFile: rulesFile,
		Now:                time.Now,
	}, filepath.Join(dir, "state.json"), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCatchupRecorderRulesOnlyRecordsOnlyRequestedProgrammes(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-time.Minute).Truncate(time.Second)
	requested := CatchupCapsule{CapsuleID: "a", Title: "Evening News", GuideNumber: "101", State: "in_progress", Start: start.Format(time.RFC3339)}
	other := CatchupCapsule{CapsuleID: "b", Title: "Cooking Live", GuideNumber: "101", State: "in_progress", Start: start.Format(time.RFC3339)}

	path := filepath.Join(t.TempDir(), "rules.json")
	if _, err := saveRecordingRulesFile(path, RecordingRuleset{Rules: []RecordingRule{{
		ID: "seerrng-1", Name: "SeerrNG #1", Enabled: true,
		TitleEquals:         []string{"Evening News"},
		IncludeGuideNumbers: []string{"101"},
	}}}); err != nil {
		t.Fatal(err)
	}

	m := rulesOnlyManager(t, path)
	if !m.eligibleCapsule(requested, now) {
		t.Fatal("requested programme should be eligible")
	}
	if m.eligibleCapsule(other, now) {
		t.Fatal("unrequested programme must not be recorded in rules-only mode")
	}

	// Rule changes are picked up without restarting the daemon.
	time.Sleep(10 * time.Millisecond)
	if _, err := saveRecordingRulesFile(path, RecordingRuleset{}); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if m.eligibleCapsule(requested, now) {
		t.Fatal("deleted rule must stop recording")
	}
}

func TestCatchupRecorderRulesOnlyFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	capsule := CatchupCapsule{CapsuleID: "a", Title: "News", State: "in_progress", Start: now.Format(time.RFC3339)}

	missing := rulesOnlyManager(t, filepath.Join(t.TempDir(), "absent.json"))
	if missing.eligibleCapsule(capsule, now) {
		t.Fatal("a missing rules file means no rules")
	}

	broken := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(broken, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rulesOnlyManager(t, broken).eligibleCapsule(capsule, now) {
		t.Fatal("an unreadable rules file must record nothing")
	}

	// Default mode is unchanged.
	dir := t.TempDir()
	plain, err := newCatchupRecorderManager(CatchupRecorderDaemonConfig{OutDir: dir, MaxConcurrency: 1, LeadTime: time.Minute, Now: time.Now}, filepath.Join(dir, "s.json"), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if !plain.eligibleCapsule(capsule, now) {
		t.Fatal("without -rules-only the recorder keeps its existing behavior")
	}
}

func TestBuildCatchupCapsulePreviewFilteredAppliesKeepBeforeLimit(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	var b strings.Builder
	b.WriteString(`<tv>`)
	for i := 0; i < 30; i++ {
		start := now.Add(time.Duration(i) * time.Minute)
		title := "Filler"
		if i == 29 {
			title = "Wanted Show"
		}
		b.WriteString(`<programme start="` + start.Format("20060102150405 -0700") + `" stop="` + start.Add(30*time.Minute).Format("20060102150405 -0700") + `" channel="` + string(rune('A'+i%26)) + `"><title>` + title + `</title></programme>`)
	}
	b.WriteString(`</tv>`)
	channels := []catalog.LiveChannel{}
	keep := func(c CatchupCapsule) bool { return c.Title == "Wanted Show" }

	unfiltered, err := BuildCatchupCapsulePreview(channels, []byte(b.String()), now, 3*time.Hour, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range unfiltered.Capsules {
		if c.Title == "Wanted Show" {
			t.Fatal("fixture should push the wanted show past the unfiltered limit")
		}
	}
	filtered, err := BuildCatchupCapsulePreviewFiltered(channels, []byte(b.String()), now, 3*time.Hour, 5, keep)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Capsules) != 1 || filtered.Capsules[0].Title != "Wanted Show" {
		t.Fatalf("filtered=%+v", filtered.Capsules)
	}
}
