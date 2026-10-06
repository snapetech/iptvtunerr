package tuner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const recordingRulesVersion = 1

// RecordingRuleFeatures are the rule capabilities API clients can rely on.
// They are reported in rule responses and never persisted.
//   - title_equals: exact (case-insensitive) title matching
//   - start_window: start_after / start_before programme start bounds
//   - rules_only_recorder: catchup-daemon -rules-only records only rule matches
var RecordingRuleFeatures = []string{"rules_only_recorder", "start_window", "title_equals"}

type RecordingRule struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Enabled             bool     `json:"enabled"`
	IncludeLanes        []string `json:"include_lanes,omitempty"`
	IncludeChannelIDs   []string `json:"include_channel_ids,omitempty"`
	IncludeGuideNumbers []string `json:"include_guide_numbers,omitempty"`
	IncludeTVGIDs       []string `json:"include_tvg_ids,omitempty"`
	IncludeCategories   []string `json:"include_categories,omitempty"`
	States              []string `json:"states,omitempty"`
	TitleContains       []string `json:"title_contains,omitempty"`
	// TitleEquals matches the whole title, ignoring case and repeated spaces.
	TitleEquals []string `json:"title_equals,omitempty"`
	// StartAfter and StartBefore bound the programme start (RFC3339,
	// inclusive). An unparsable bound matches nothing.
	StartAfter  string `json:"start_after,omitempty"`
	StartBefore string `json:"start_before,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type RecordingRuleset struct {
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Rules     []RecordingRule `json:"rules"`
	// Features is set on API responses only; see RecordingRuleFeatures.
	Features []string `json:"features,omitempty"`
}

type RecordingRulePreviewMatch struct {
	RuleID       string           `json:"rule_id"`
	Name         string           `json:"name"`
	Enabled      bool             `json:"enabled"`
	MatchCount   int              `json:"match_count"`
	SampleTitles []string         `json:"sample_titles,omitempty"`
	Sample       []CatchupCapsule `json:"sample,omitempty"`
}

type RecordingRulePreviewReport struct {
	GeneratedAt string                      `json:"generated_at"`
	RuleCount   int                         `json:"rule_count"`
	SourceReady bool                        `json:"source_ready"`
	Matches     []RecordingRulePreviewMatch `json:"matches"`
}

type RecordingRuleHistoryMatch struct {
	RuleID           string `json:"rule_id"`
	Name             string `json:"name"`
	ActiveCount      int    `json:"active_count"`
	CompletedCount   int    `json:"completed_count"`
	FailedCount      int    `json:"failed_count"`
	PublishedCount   int    `json:"published_count"`
	InterruptedCount int    `json:"interrupted_count"`
}

type RecordingRuleHistoryReport struct {
	GeneratedAt string                      `json:"generated_at"`
	StateFile   string                      `json:"state_file,omitempty"`
	RuleCount   int                         `json:"rule_count"`
	Unmatched   CatchupRecorderLaneReport   `json:"unmatched"`
	Matches     []RecordingRuleHistoryMatch `json:"matches"`
}

func normalizeRecordingRule(rule RecordingRule) RecordingRule {
	rule.ID = strings.TrimSpace(rule.ID)
	if rule.ID == "" {
		rule.ID = slugRecordingRule(strings.ToLower(strings.TrimSpace(rule.Name)))
	}
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", time.Now().UTC().UnixNano())
	}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		rule.Name = rule.ID
	}
	rule.IncludeLanes = dedupeSortedStrings(rule.IncludeLanes)
	rule.IncludeChannelIDs = dedupeSortedStrings(rule.IncludeChannelIDs)
	rule.IncludeGuideNumbers = dedupeSortedStrings(rule.IncludeGuideNumbers)
	rule.IncludeTVGIDs = dedupeSortedStrings(rule.IncludeTVGIDs)
	rule.IncludeCategories = dedupeSortedStrings(rule.IncludeCategories)
	rule.States = dedupeSortedStrings(rule.States)
	rule.TitleContains = dedupeSortedStrings(rule.TitleContains)
	rule.TitleEquals = dedupeSortedStrings(rule.TitleEquals)
	rule.StartAfter = strings.TrimSpace(rule.StartAfter)
	rule.StartBefore = strings.TrimSpace(rule.StartBefore)
	rule.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return rule
}

func normalizeRecordingRuleset(set RecordingRuleset) RecordingRuleset {
	set.Version = recordingRulesVersion
	set.Features = nil
	seen := map[string]struct{}{}
	out := make([]RecordingRule, 0, len(set.Rules))
	for _, rule := range set.Rules {
		rule = normalizeRecordingRule(rule)
		if _, ok := seen[rule.ID]; ok {
			continue
		}
		seen[rule.ID] = struct{}{}
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	set.Rules = out
	set.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return set
}

func loadRecordingRulesFile(path string) (RecordingRuleset, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return normalizeRecordingRuleset(RecordingRuleset{}), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return normalizeRecordingRuleset(RecordingRuleset{}), nil
		}
		return RecordingRuleset{}, err
	}
	var set RecordingRuleset
	if err := json.Unmarshal(data, &set); err != nil {
		return RecordingRuleset{}, err
	}
	return normalizeRecordingRuleset(set), nil
}

// LoadRecordingRulesFile reads and normalizes a recording rules file. A
// missing file yields an empty ruleset.
func LoadRecordingRulesFile(path string) (RecordingRuleset, error) {
	return loadRecordingRulesFile(path)
}

func saveRecordingRulesFile(path string, set RecordingRuleset) (RecordingRuleset, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return RecordingRuleset{}, fmt.Errorf("recording rules file not configured")
	}
	set = normalizeRecordingRuleset(set)
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return RecordingRuleset{}, err
	}
	dir := filepath.Dir(filepath.Clean(path))
	tmp, err := os.CreateTemp(dir, ".recording-rules-*.json.tmp")
	if err != nil {
		return RecordingRuleset{}, err
	}
	name := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if writeErr != nil {
			return RecordingRuleset{}, writeErr
		}
		return RecordingRuleset{}, closeErr
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = os.Remove(name)
		return RecordingRuleset{}, err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return RecordingRuleset{}, err
	}
	return set, nil
}

func upsertRecordingRule(set RecordingRuleset, rule RecordingRule) RecordingRuleset {
	rule = normalizeRecordingRule(rule)
	replaced := false
	for i := range set.Rules {
		if strings.EqualFold(set.Rules[i].ID, rule.ID) {
			set.Rules[i] = rule
			replaced = true
			break
		}
	}
	if !replaced {
		set.Rules = append(set.Rules, rule)
	}
	return normalizeRecordingRuleset(set)
}

func deleteRecordingRule(set RecordingRuleset, ruleID string) RecordingRuleset {
	ruleID = strings.TrimSpace(ruleID)
	out := make([]RecordingRule, 0, len(set.Rules))
	for _, rule := range set.Rules {
		if strings.EqualFold(strings.TrimSpace(rule.ID), ruleID) {
			continue
		}
		out = append(out, rule)
	}
	set.Rules = out
	return normalizeRecordingRuleset(set)
}

func toggleRecordingRule(set RecordingRuleset, ruleID string, enabled bool) RecordingRuleset {
	for i := range set.Rules {
		if strings.EqualFold(strings.TrimSpace(set.Rules[i].ID), strings.TrimSpace(ruleID)) {
			set.Rules[i].Enabled = enabled
			set.Rules[i] = normalizeRecordingRule(set.Rules[i])
		}
	}
	return normalizeRecordingRuleset(set)
}

func matchRecordingRuleCapsule(rule RecordingRule, capsule CatchupCapsule) bool {
	if !rule.Enabled {
		return false
	}
	if !matchesStringFilter(rule.IncludeLanes, capsule.Lane) {
		return false
	}
	if !matchesStringFilter(rule.IncludeChannelIDs, capsule.ChannelID) {
		return false
	}
	if !matchesStringFilter(rule.IncludeGuideNumbers, capsule.GuideNumber) {
		return false
	}
	if !matchesStringFilter(rule.States, capsule.State) {
		return false
	}
	if len(rule.IncludeCategories) > 0 && !matchesAnyFold(rule.IncludeCategories, capsule.Categories) {
		return false
	}
	if !matchesTitleContains(rule.TitleContains, capsule.Title) {
		return false
	}
	if !matchesTitleEquals(rule.TitleEquals, capsule.Title) {
		return false
	}
	if !matchesStartWindow(rule.StartAfter, rule.StartBefore, capsule.Start) {
		return false
	}
	return true
}

func matchRecordingRuleItem(rule RecordingRule, item CatchupRecorderItem) bool {
	if !rule.Enabled {
		return false
	}
	if !matchesStringFilter(rule.IncludeLanes, item.Lane) {
		return false
	}
	if !matchesStringFilter(rule.IncludeChannelIDs, item.ChannelID) {
		return false
	}
	if !matchesStringFilter(rule.IncludeGuideNumbers, item.GuideNumber) {
		return false
	}
	if !matchesTitleContains(rule.TitleContains, item.Title) {
		return false
	}
	if !matchesTitleEquals(rule.TitleEquals, item.Title) {
		return false
	}
	if !matchesStartWindow(rule.StartAfter, rule.StartBefore, item.Start) {
		return false
	}
	return true
}

// MatchAnyRecordingRuleCapsule reports whether any enabled rule matches.
func MatchAnyRecordingRuleCapsule(set RecordingRuleset, capsule CatchupCapsule) bool {
	for _, rule := range set.Rules {
		if matchRecordingRuleCapsule(rule, capsule) {
			return true
		}
	}
	return false
}

// withRecordingRuleFeatures returns a copy of set for API responses.
func withRecordingRuleFeatures(set RecordingRuleset) RecordingRuleset {
	set.Features = append([]string(nil), RecordingRuleFeatures...)
	return set
}

func buildRecordingRulePreview(rules RecordingRuleset, preview CatchupCapsulePreview) RecordingRulePreviewReport {
	out := RecordingRulePreviewReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RuleCount:   len(rules.Rules),
		SourceReady: preview.SourceReady,
		Matches:     make([]RecordingRulePreviewMatch, 0, len(rules.Rules)),
	}
	for _, rule := range rules.Rules {
		row := RecordingRulePreviewMatch{
			RuleID:  rule.ID,
			Name:    rule.Name,
			Enabled: rule.Enabled,
		}
		titleSeen := map[string]struct{}{}
		for _, capsule := range preview.Capsules {
			if !matchRecordingRuleCapsule(rule, capsule) {
				continue
			}
			row.MatchCount++
			if len(row.Sample) < 5 {
				row.Sample = append(row.Sample, capsule)
			}
			title := strings.TrimSpace(capsule.Title)
			if title != "" {
				if _, ok := titleSeen[title]; !ok && len(row.SampleTitles) < 5 {
					titleSeen[title] = struct{}{}
					row.SampleTitles = append(row.SampleTitles, title)
				}
			}
		}
		out.Matches = append(out.Matches, row)
	}
	return out
}

func buildRecordingRuleHistory(rules RecordingRuleset, report CatchupRecorderReport) RecordingRuleHistoryReport {
	out := RecordingRuleHistoryReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		StateFile:   strings.TrimSpace(report.StateFile),
		RuleCount:   len(rules.Rules),
		Matches:     make([]RecordingRuleHistoryMatch, 0, len(rules.Rules)),
	}
	type bucket struct {
		active, completed, failed, published, interrupted int
	}
	byID := map[string]*bucket{}
	for _, rule := range rules.Rules {
		byID[rule.ID] = &bucket{}
	}
	visit := func(items []CatchupRecorderItem, mode string) {
		for _, item := range items {
			matched := false
			for _, rule := range rules.Rules {
				if !matchRecordingRuleItem(rule, item) {
					continue
				}
				matched = true
				cur := byID[rule.ID]
				switch mode {
				case "active":
					cur.active++
				case "completed":
					cur.completed++
					if strings.TrimSpace(item.PublishedPath) != "" {
						cur.published++
					}
				case "failed":
					cur.failed++
					if strings.EqualFold(strings.TrimSpace(item.Status), "interrupted") {
						cur.interrupted++
					}
				}
			}
			if matched {
				continue
			}
			switch mode {
			case "active":
				out.Unmatched.ActiveCount++
			case "completed":
				out.Unmatched.CompletedCount++
				if strings.TrimSpace(item.PublishedPath) != "" {
					out.Unmatched.PublishedCount++
				}
			case "failed":
				out.Unmatched.FailedCount++
				if strings.EqualFold(strings.TrimSpace(item.Status), "interrupted") {
					out.Unmatched.InterruptedCount++
				}
			}
		}
	}
	visit(report.Active, "active")
	visit(report.Completed, "completed")
	visit(report.Failed, "failed")
	out.Unmatched.Lane = "unmatched"
	for _, rule := range rules.Rules {
		cur := byID[rule.ID]
		out.Matches = append(out.Matches, RecordingRuleHistoryMatch{
			RuleID:           rule.ID,
			Name:             rule.Name,
			ActiveCount:      cur.active,
			CompletedCount:   cur.completed,
			FailedCount:      cur.failed,
			PublishedCount:   cur.published,
			InterruptedCount: cur.interrupted,
		})
	}
	return out
}

func matchesStringFilter(filters []string, value string) bool {
	if len(filters) == 0 {
		return true
	}
	value = strings.TrimSpace(value)
	for _, filter := range filters {
		if strings.EqualFold(strings.TrimSpace(filter), value) {
			return true
		}
	}
	return false
}

func matchesAnyFold(filters, values []string) bool {
	for _, value := range values {
		if matchesStringFilter(filters, value) {
			return true
		}
	}
	return false
}

func matchesTitleContains(filters []string, title string) bool {
	if len(filters) == 0 {
		return true
	}
	title = strings.ToLower(strings.TrimSpace(title))
	for _, filter := range filters {
		filter = strings.ToLower(strings.TrimSpace(filter))
		if filter != "" && strings.Contains(title, filter) {
			return true
		}
	}
	return false
}

func normalizeTitleForEquals(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

func matchesTitleEquals(filters []string, title string) bool {
	if len(filters) == 0 {
		return true
	}
	title = normalizeTitleForEquals(title)
	for _, filter := range filters {
		if filter = normalizeTitleForEquals(filter); filter != "" && filter == title {
			return true
		}
	}
	return false
}

func matchesStartWindow(after, before, start string) bool {
	after = strings.TrimSpace(after)
	before = strings.TrimSpace(before)
	if after == "" && before == "" {
		return true
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(start))
	if err != nil {
		return false
	}
	if after != "" {
		bound, err := time.Parse(time.RFC3339, after)
		if err != nil || at.Before(bound) {
			return false
		}
	}
	if before != "" {
		bound, err := time.Parse(time.RFC3339, before)
		if err != nil || at.After(bound) {
			return false
		}
	}
	return true
}

func dedupeSortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func slugRecordingRule(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}
