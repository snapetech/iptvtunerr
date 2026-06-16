package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/snapetech/iptvtunerr/internal/emby"
	"github.com/snapetech/iptvtunerr/internal/plex"
	"github.com/snapetech/iptvtunerr/internal/tuner"
)

func applyPlexVODLibraryPreset(plexBaseURL, plexToken string, sec *plex.LibrarySection) error {
	if sec == nil {
		return fmt.Errorf("nil library section")
	}
	prefs, err := plex.GetLibrarySectionPrefs(plexBaseURL, plexToken, sec.Key)
	if err != nil {
		return err
	}
	desired := map[string]string{
		"enableBIFGeneration":           "0",
		"enableChapterThumbGeneration":  "0",
		"enableIntroMarkerGeneration":   "0",
		"enableCreditsMarkerGeneration": "0",
		"enableAdMarkerGeneration":      "0",
		"enableVoiceActivityGeneration": "0",
	}
	updates := map[string]string{}
	for k, v := range desired {
		if got, ok := prefs[k]; ok && got != v {
			updates[k] = v
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return plex.UpdateLibrarySectionPrefs(plexBaseURL, plexToken, sec.Key, updates)
}

func resolvePlexAccess(flagURL, flagToken string) (string, string) {
	baseURL := strings.TrimSpace(flagURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("IPTV_TUNERR_PMS_URL"))
	}
	if baseURL == "" {
		baseURL = plexBaseURLFromHostAlias(os.Getenv("PLEX_HOST"))
	}
	token := strings.TrimSpace(flagToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("IPTV_TUNERR_PMS_TOKEN"))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv("PLEX_TOKEN"))
	}
	return baseURL, token
}

func plexBaseURLFromHostAlias(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}
	if strings.Contains(host, ":") {
		return "http://" + host
	}
	return "http://" + host + ":32400"
}

func registerCatchupPlexLibraries(baseURL, token string, manifest tuner.CatchupPublishManifest, refresh bool) error {
	for _, lib := range manifest.Libraries {
		sec, created, err := plex.EnsureLibrarySection(baseURL, token, plex.LibraryCreateSpec{
			Name:     lib.Name,
			Type:     "movie",
			Path:     lib.Path,
			Language: "en-US",
		})
		if err != nil {
			return err
		}
		if created {
			log.Printf("Created Plex catch-up library %q (key=%s path=%s)", sec.Title, sec.Key, lib.Path)
		} else {
			log.Printf("Reusing Plex catch-up library %q (key=%s path=%s)", sec.Title, sec.Key, lib.Path)
		}
		if err := applyPlexVODLibraryPreset(baseURL, token, sec); err != nil {
			return err
		}
		if refresh {
			if err := plex.RefreshLibrarySection(baseURL, token, sec.Key); err != nil {
				return err
			}
			log.Printf("Refresh started for Plex catch-up library %q", sec.Title)
		}
	}
	return nil
}

func registerCatchupMediaServerLibraries(serverType, host, token string, manifest tuner.CatchupPublishManifest, refresh bool) error {
	cfg := emby.Config{
		Host:       strings.TrimSpace(host),
		Token:      strings.TrimSpace(token),
		ServerType: serverType,
	}
	for _, lib := range manifest.Libraries {
		got, created, err := emby.EnsureLibrary(cfg, emby.LibraryCreateSpec{
			Name:           lib.Name,
			CollectionType: "movies",
			Path:           lib.Path,
			Refresh:        false,
		})
		if err != nil {
			return err
		}
		if created {
			log.Printf("Created %s catch-up library %q (id=%s path=%s)", serverType, lib.Name, got.ID, lib.Path)
		} else {
			log.Printf("Reusing %s catch-up library %q (id=%s path=%s)", serverType, got.Name, got.ID, lib.Path)
		}
	}
	if refresh {
		if err := emby.RefreshLibraryScan(cfg); err != nil {
			return err
		}
		log.Printf("Triggered %s library refresh", serverType)
	}
	return nil
}
