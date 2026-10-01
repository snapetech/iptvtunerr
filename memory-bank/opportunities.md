# Opportunities

## Implemented or reconciled on 2026-09-30

- **Plex internal-fetcher profile precedence — implemented.** Explicit per-channel profile overrides now take precedence over `IPTV_TUNERR_PLEX_INTERNAL_FETCHER_PROFILE`; the environment setting remains the fallback. See [CLI and environment reference](../docs/reference/cli-and-env-reference.md) and WBS story OPP-001.
- **Public Actions log hygiene — implemented.** Public GitHub Actions workflows use GitHub-hosted Ubuntu or Windows runners, so checkout/tool logs no longer expose paths from the private self-hosted runner. See WBS story OPP-002.
- **Release announcement parity — implemented.** Matrix now sends the curated release body used by GitHub and Discord in bounded messages, escapes note text for HTML, and replaces same-day retry chunks. The first Matrix delivery using this path will be confirmed on the next release. See WBS story OPP-003.
- **First-run package follow-up — reconciled.** The `v0.1.86` [AUR](https://github.com/snapetech/iptvtunerr/actions/runs/36778705022), [PPA](https://github.com/snapetech/iptvtunerr/actions/runs/36778708581), and [COPR](https://github.com/snapetech/iptvtunerr/actions/runs/36778712646) publisher workflows succeeded; GitHub Release includes `.deb` and `.rpm` assets. Chocolatey [`0.1.68`](https://community.chocolatey.org/packages/iptvtunerr) is approved, and Winget PR [`microsoft/winget-pkgs#374269`](https://github.com/microsoft/winget-pkgs/pull/374269) for `0.1.68` merged. No workflow hardening issue was found in these first live results. Snap publication is not configured; see the external follow-up below. See WBS story OPP-004.
- **Release package generation — confirmed.** The [`v0.1.86` GitHub Release](https://github.com/snapetech/iptvtunerr/releases/tag/v0.1.86) contains direct `.deb` and `.rpm` assets. See WBS story OPP-004.
- **Single-run CI failure — closed for now.** The unchanged retry passed full verification, and [CI for current `main`](https://github.com/snapetech/iptvtunerr/actions/runs/36787502831) is green. No source-level defect or recurrence was found; reopen only if this failure repeats. See WBS story OPP-005.
- **Windows native proof — completed.** Windows Smoke passed on the GitHub-hosted runner after the runner migration and push trigger: [run 36793770645](https://github.com/snapetech/iptvtunerr/actions/runs/36793770645).

## External follow-up

- **Snap publisher.** The repository has Snap smoke-test support but no Snapcraft manifest or publisher workflow. Adding this channel requires a package definition and Snap Store credentials; no release can exercise it before those prerequisites exist.
- **Current Windows package versions.** Chocolatey and Winget still list `0.1.68`; the manual publisher workflows have not submitted the current `v0.1.86` release. See [Chocolatey](https://community.chocolatey.org/packages/iptvtunerr) and [Winget PR 374269](https://github.com/microsoft/winget-pkgs/pull/374269).
- **Plex Live TV watchdog schedule.** The prior read-only audit found the watchdog service inactive and its newest snapshot dated 2026-06-18. No watchdog timer/service definition is owned by this repository. Verify the deployment's timer/service wiring and collect a fresh snapshot through its host-managed configuration before relying on it during a Live TV failure. See WBS story OPP-007.
