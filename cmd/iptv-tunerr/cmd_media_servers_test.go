package main

import "testing"

func TestResolvePlexAccessPrefersCanonicalPMSEnv(t *testing.T) {
	t.Setenv("IPTV_TUNERR_PMS_URL", "http://canonical.example:32400")
	t.Setenv("IPTV_TUNERR_PMS_TOKEN", "canonical-token")
	t.Setenv("PLEX_HOST", "legacy.example:32400")
	t.Setenv("PLEX_TOKEN", "legacy-token")

	gotURL, gotToken := resolvePlexAccess("", "")
	if gotURL != "http://canonical.example:32400" {
		t.Fatalf("url=%q want canonical PMS URL", gotURL)
	}
	if gotToken != "canonical-token" {
		t.Fatalf("token=%q want canonical PMS token", gotToken)
	}
}

func TestResolvePlexAccessLegacyHostAliasWithScheme(t *testing.T) {
	t.Setenv("IPTV_TUNERR_PMS_URL", "")
	t.Setenv("IPTV_TUNERR_PMS_TOKEN", "")
	t.Setenv("PLEX_HOST", "http://legacy.example:32400")
	t.Setenv("PLEX_TOKEN", "legacy-token")

	gotURL, gotToken := resolvePlexAccess("", "")
	if gotURL != "http://legacy.example:32400" {
		t.Fatalf("url=%q want legacy URL preserved", gotURL)
	}
	if gotToken != "legacy-token" {
		t.Fatalf("token=%q want legacy token", gotToken)
	}
}

func TestResolvePlexAccessLegacyHostAliasWithoutPort(t *testing.T) {
	t.Setenv("IPTV_TUNERR_PMS_URL", "")
	t.Setenv("IPTV_TUNERR_PMS_TOKEN", "")
	t.Setenv("PLEX_HOST", "legacy.example")
	t.Setenv("PLEX_TOKEN", "legacy-token")

	gotURL, _ := resolvePlexAccess("", "")
	if gotURL != "http://legacy.example:32400" {
		t.Fatalf("url=%q want default Plex port", gotURL)
	}
}
