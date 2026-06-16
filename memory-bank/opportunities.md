# Opportunities

- 2026-05-28: Profile override precedence gap for Plex internal fetchers. Per-channel `IPTV_TUNERR_PROFILE_OVERRIDES_FILE` entries load correctly, but PMS `Lavf` requests can still be forced by `IPTV_TUNERR_PLEX_INTERNAL_FETCHER_PROFILE` after the per-channel profile is selected. Consider adding a code path for per-channel/internal-fetcher profile overrides or making adaptation preserve explicit channel profile overrides where safe. Confidence: high.
- No active opportunities filed after local fallback removal. File future items here if they fit binary, Docker, systemd/bare-metal, or k3s as a single-owner deployment path.
- Package channel follow-up: first live Snap, Launchpad/PPA, COPR, Chocolatey, and Winget runs may need workflow hardening based on remote service responses. AUR has already been pushed successfully.
- Windows channel follow-up: native Windows host proof is still recommended before broad Windows parity claims.
- Release package follow-up: first GitHub Actions run should confirm Ubuntu runner `.deb`/`.rpm` direct package asset generation and remote channel publish responses end to end.
