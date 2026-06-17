# Opportunities

- 2026-05-28: Profile override precedence gap for Plex internal fetchers. Per-channel `IPTV_TUNERR_PROFILE_OVERRIDES_FILE` entries load correctly, but PMS `Lavf` requests can still be forced by `IPTV_TUNERR_PLEX_INTERNAL_FETCHER_PROFILE` after the per-channel profile is selected. Consider adding a code path for per-channel/internal-fetcher profile overrides or making adaptation preserve explicit channel profile overrides where safe. Confidence: high.
- No active opportunities filed after local fallback removal. File future items here if they fit binary, Docker, systemd/bare-metal, or k3s as a single-owner deployment path.
- Package channel follow-up: first live Snap, Launchpad/PPA, Chocolatey, and Winget runs may need workflow hardening based on remote service responses. AUR and COPR have already been pushed successfully.
- Public Actions log hygiene: repo-controlled COPR upload logs now avoid publishing home-directory SRPM paths, but third-party self-hosted runner setup actions still print runner checkout/tool paths in job logs. A broader fix would migrate public release jobs to a generic runner account/path or GitHub-hosted runners where compatible. Confidence: moderate.
- Windows channel follow-up: native Windows host proof is still recommended before broad Windows parity claims.
- Release package follow-up: first GitHub Actions run should confirm Ubuntu runner `.deb`/`.rpm` direct package asset generation and remote channel publish responses end to end.
