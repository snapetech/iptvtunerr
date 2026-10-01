package sports

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const settingsVersion = 1

var ErrSettingsPath = errors.New("sports automation settings file is not configured")

type TeamRule struct {
	Dataset string `json:"dataset"`
	Team    string `json:"team"`
}

type AutomationSettings struct {
	Version             int        `json:"version"`
	Enabled             bool       `json:"enabled"`
	Datasets            []string   `json:"datasets"`
	TeamRules           []TeamRule `json:"team_rules"`
	PastHours           int        `json:"past_hours"`
	LookaheadDays       int        `json:"lookahead_days"`
	MaxChannelsPerEvent int        `json:"max_channels_per_event"`
	UpdatedAt           string     `json:"updated_at,omitempty"`
}

func DefaultAutomationSettings() AutomationSettings {
	return NormalizeAutomationSettings(AutomationSettings{
		Version:             settingsVersion,
		Datasets:            []string{},
		PastHours:           6,
		LookaheadDays:       3,
		MaxChannelsPerEvent: 4,
	})
}

func NormalizeAutomationSettings(settings AutomationSettings) AutomationSettings {
	settings.Version = settingsVersion
	settings.Datasets = normalizeDatasetIDs(settings.Datasets)
	if settings.PastHours < 0 {
		settings.PastHours = 0
	}
	if settings.PastHours > 12 {
		settings.PastHours = 12
	}
	if settings.LookaheadDays < 1 {
		settings.LookaheadDays = 1
	}
	if settings.LookaheadDays > 7 {
		settings.LookaheadDays = 7
	}
	if settings.MaxChannelsPerEvent < 1 {
		settings.MaxChannelsPerEvent = 1
	}
	if settings.MaxChannelsPerEvent > 12 {
		settings.MaxChannelsPerEvent = 12
	}
	seen := map[string]bool{}
	teams := make([]TeamRule, 0, len(settings.TeamRules))
	for _, rule := range settings.TeamRules {
		rule.Dataset = strings.ToLower(strings.TrimSpace(rule.Dataset))
		rule.Team = strings.TrimSpace(rule.Team)
		if _, ok := DatasetByID(rule.Dataset); !ok || rule.Team == "" || len(rule.Team) > 96 {
			continue
		}
		key := rule.Dataset + "\x00" + strings.ToLower(rule.Team)
		if seen[key] {
			continue
		}
		seen[key] = true
		teams = append(teams, rule)
	}
	sort.Slice(teams, func(i, j int) bool {
		if teams[i].Dataset == teams[j].Dataset {
			return strings.ToLower(teams[i].Team) < strings.ToLower(teams[j].Team)
		}
		return teams[i].Dataset < teams[j].Dataset
	})
	settings.TeamRules = teams
	return settings
}

func LoadAutomationSettings(path string) (AutomationSettings, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultAutomationSettings(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultAutomationSettings(), nil
		}
		return AutomationSettings{}, fmt.Errorf("read sports settings: %w", err)
	}
	var settings AutomationSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return AutomationSettings{}, fmt.Errorf("decode sports settings: %w", err)
	}
	if settings.Version != 0 && settings.Version != settingsVersion {
		return AutomationSettings{}, fmt.Errorf("unsupported sports settings version %d", settings.Version)
	}
	return NormalizeAutomationSettings(settings), nil
}

func SaveAutomationSettings(path string, settings AutomationSettings) (AutomationSettings, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return AutomationSettings{}, ErrSettingsPath
	}
	settings = NormalizeAutomationSettings(settings)
	settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return AutomationSettings{}, fmt.Errorf("encode sports settings: %w", err)
	}
	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return AutomationSettings{}, fmt.Errorf("create sports settings directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".sports-settings-*.tmp")
	if err != nil {
		return AutomationSettings{}, fmt.Errorf("create sports settings temp file: %w", err)
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpName)
		if writeErr != nil {
			return AutomationSettings{}, fmt.Errorf("write sports settings: %w", writeErr)
		}
		return AutomationSettings{}, fmt.Errorf("close sports settings: %w", closeErr)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return AutomationSettings{}, fmt.Errorf("secure sports settings: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return AutomationSettings{}, fmt.Errorf("save sports settings: %w", err)
	}
	return settings, nil
}
