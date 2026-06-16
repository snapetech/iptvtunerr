## 2026-05-29 - Fix Plex Web Live TV transcode ingress 404

- Found the remaining tester `s1002 (Network)` failure was not stale DVR state or tuner output: Plex Web's `/video/:/transcode/universal/*` and `/:/timeline` calls were returning Caddy's 9-byte `404` immediately after a successful Live TV tune.
- Direct PMS and direct Go proxy probes reached Plex/proxy, while the same encoded Live TV transcode URL through HTTPS ingress failed before the proxy. Root cause was the public-ingress abnormal-URI matcher blocking `%2f` in Plex Web's legitimate `path=%2Flivetv%2Fsessions%2F...` query value.
- Installed the patched active proxy binary (`dev-3739f33-rolling-delete-session`), backed up the previous binary, and restarted only `plex-live-tv-proxy.service`.
- Backed up the active ingress Caddyfile, narrowed the abnormal-URI deny rule so Plex playback paths `/video/:/transcode/*`, `/:/timeline*`, and `/playQueues*` can carry encoded Live TV session values, validated the Caddyfile, and restarted only the ingress service.
- Live validation: the encoded HTTPS transcode probe no longer returns Caddy's 9-byte `404`; it reaches `plex-live-tv-proxy.service`, logs `live_tv=true stream=true`, and returns a Plex/proxy response for the synthetic fake session. Timeline requests also reach the proxy instead of being denied at ingress.
- Verification: previous focused `go test -count=1 ./internal/plexlabelproxy` and full `./scripts/verify` passed for the proxy code change; the Caddyfile change was validated live with `caddy validate` and synthetic ingress probes.

## 2026-05-28 - Patch Plex rolling Live TV subscription cleanup

- Investigated the tester's post-refresh Plex Web `s1002 (Network)` retry after the stale subscription was removed and the sports bridge was switched to AAC for PMS internal fetches.
- Found the fresh retry reached the tuner and PMS successfully: the Live TV tune request was elevated, the sports bridge served the AAC profile, PMS confirmed AAC audio, and Plex Web pulled many HLS segments.
- Found the remaining proxy denial: Plex Web sent `DELETE /media/subscriptions/{id}` for the rolling Live TV playback subscription with `X-Plex-Playback-Session-Id`, and the proxy classified it as non-Live-TV because it did not carry XMLTV guide evidence.
- Patched the proxy classifier so only `DELETE /media/subscriptions/<numeric-id>` with non-empty `X-Plex-Playback-Session-Id` is elevated. Plain id-only deletes, nested scheduled paths, and library subscription edits remain non-elevated.
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed and full `./scripts/verify` passed.
- Live deploy: installed the verified proxy hotfix binary on the live validation host, backed up the previous binary, restarted only `plex-live-tv-proxy.service`, and confirmed the service is active.
- Live validation: synthetic missing-token playback-session DELETE is now classified as `live_tv=true` and denied for missing token; synthetic id-only DELETE remains `live_tv=false`.
- Follow-up correction: after the tester still saw `s1002`, found the active live proxy process was still the old binary and had logged zero Live TV elevations in the retry window. Rebuilt and installed `dev-3739f33-rolling-delete`, restarted only `plex-live-tv-proxy.service`, and validated the active service now classifies playback-session DELETE as `live_tv=true` while id-only DELETE remains `live_tv=false`.
- Follow-up session-shape fix: after the tester still saw `s1002`, broadened only `DELETE /media/subscriptions/<numeric-id>` rolling Live TV cleanup classification to accept non-empty `X-Plex-Playback-Session-Id` or `X-Plex-Session-Id` from query parameters or headers. Plain id-only deletes, product-only deletes, nested scheduled paths, and library subscription edits remain non-elevated.
- Live deploy: built `dev-3739f33-rolling-delete-session`, backed up the prior live proxy binary, installed the new binary to the active proxy path, and restarted only `plex-live-tv-proxy.service`.
- Live validation: synthetic header playback-session DELETE and query session-id DELETE now log `live_tv=true` and deny missing-token; neighboring product-only DELETE logs `live_tv=false`.
- Verification after actual deploys: focused proxy tests and full `./scripts/verify` passed.
- Post-deploy correction after another tester failure: no real tester Live TV requests reached the patched proxy journal after the hotfix. Restored PMS remote advertisement to the documented static proxy state: manual mapping enabled, manual port `443`, relay disabled, public media HTTPS custom connection retained, and automatic mapped port cleared to `0`.
- Validation after PMS correction: active public-media HTTPS path reaches the Go proxy and logs `/media/providers` as `elevated_live_tv`; plex.tv resources advertise the public remote HTTPS custom connection.

## 2026-05-28 - Clear stale Plex NBA Live TV subscription after tester playback error

- Investigated tester screenshot showing Plex Web playback error `s1002 (Network)` for `Live: NBA Basketball` on the Sportsnet 360 raw row.
- Verified the sports tuner service and guide were healthy: `/readyz`, `/lineup.json`, and `/guide.xml` returned `200`; the current `Live: NBA Basketball` guide rows include stable `sub-title`, `date`, and `episode-num system="iptvtunerr"` identity.
- Verified the reported channel stream itself was usable: direct `/stream/10014` returned HTTP `200` and MPEG-TS bytes, and live tuner logs showed Plex had recently received first bytes and tens of MB for the same raw Sportsnet 360 stream.
- Verified recent Live TV proxy discovery requests were elevated successfully with no missing-token, unauthorized, or blocked-source denials.
- Found Plex still had a stale title-only `Live: NBA Basketball` media subscription, matching the known generic-live-sports stale identity loop.
- Backed up the relevant Plex subscription XML on the live validation host, then deleted only the stale title-only `Live: NBA Basketball` media subscription. Plex returned `200`, and `/media/subscriptions` no longer contains that rule.
- Verification: `go test -count=1 ./internal/tuner -run 'TestXMLTV_(externalSourceRemap_StabilizesRecurringEventIdentity|buildMergedEPG_stabilizesRecurringEventIdentity)'` passed; `go test -count=1 ./internal/plexlabelproxy -run 'TestIsLiveTVRequest_(MediaSubscriptionTemplateForXMLTVElevated|PostMediaSubscriptionWithPlexHintQueryElevated|MediaSubscriptionReadsElevated)|TestProxy_RewritesJSONAllowTunersForMediaProviders'` passed.

## 2026-05-18 - Fix shared-user Plex DVR subscription list failure

- Investigated the external tester's Plex Record Options save error and found PMS was returning shared-user `403` responses for read-only DVR subscription list endpoints.
- Patched the Live TV proxy so `GET /media/subscriptions` and `GET /media/subscriptions/scheduled` are classified as Live TV discovery and can borrow owner tuner entitlement.
- Kept subscription mutation scoping intact: XMLTV template/create paths and XMLTV-backed `/media/subscriptions/{id}` rule edits are elevated, while ordinary library subscription requests and id-only deletes are not widened.
- Deployed the patched proxy binary to the live instance and restarted only the Live TV proxy service.
- Verification: focused proxy tests passed; live validation showed missing-token requests are still denied, authorized subscription list requests return `200`, and an XMLTV-style rule edit is classified as Live TV without elevating a library-style edit.
- Repo/deploy follow-up: committed and pushed the full requested dirty tree as `e9bc7e2`, then installed a commit-stamped internal build to the primary/sports Tunerr binary and Live TV proxy binary paths. All three services restarted active; primary and sports ready/guide endpoints returned `200`, proxy identity returned `200`, and Plex DVR activation completed for 426 primary channels and 125 sports channels.
- Follow-up hotfix: the actual save request used `POST /media/subscriptions` with XMLTV identity in `hints[guid]` and `hints[ratingKey]` query parameters; patched and deployed the Live TV proxy classifier so bracketed subscription hint keys are elevated only when their values are Live TV/XMLTV.

## 2026-05-18 - Fix package-channel publication gaps

- Package-smoke found real drift after `v0.1.77`: Docker Hub did not have `snapetech/iptvtunerr:v0.1.77`, Jammy PPA had `v0.1.75`, COPR Fedora 43 had `v0.1.76`, Noble PPA was missing, and the direct RPM had unresolved `%systemd_*` macro scriptlet output when built outside Fedora macros.
- Changed Docker Hub publishing to target `snapetech/iptvtunerr` when Docker Hub credentials are present, and added GitLab Docker Hub promotion from the internal registry image.
- Changed GitLab release promotion to dispatch the GitHub AUR, Snap, PPA, and COPR package publisher workflows after creating the GitHub release.
- Changed PPA publishing to build both Jammy and Noble source packages and explicitly include the staged binary in `debian/source/include-binaries`.
- Changed COPR publishing to modify existing projects with Fedora 43 and Rawhide chroots before building for those chroots.
- Replaced direct RPM `%systemd_*` macro scriptlets with portable shell commands; a local package rebuild showed plain shell scriptlets.
- Verification: YAML parse, shell syntax, `git diff --check`, and targeted RPM rebuild/scriptlet inspection passed.

## 2026-05-18 - Cut v0.1.77 release

- Committed and pushed the full dirty tree requested by the operator, including Plex DVR event-window fixes, shared-user Record-button entitlement fixes, generated council metadata, and release workflow hardening.
- Configured the Discord release webhook as the `DISCORD_RELEASE_WEBHOOK` repository secret without committing the webhook URL; the tracked tree was checked for the literal webhook value.
- Published GitHub Release `v0.1.77` with 18 assets, including raw binaries, archives, `.deb`, `.rpm`, checksums, and release manifest.
- Release workflow run `26008882309` passed verification, release smoke, asset build/verification, GitHub Release publish, Discord announcement, Matrix announcement, and package-channel dispatch.
- Latest `main` checks for the final release commit passed: CI, CodeQL, Gitleaks, Docker, and Local Identity Leak Check.
- Verification: `bash ./scripts/ci-smoke.sh`, `./scripts/release-readiness.sh`, and final GitHub Release workflow passed. Downstream AUR/Snap/PPA/COPR workflows were dispatched and were still running or queued at closeout.

## 2026-05-17 - Fix Plex recording failure on event-only sports guide rows

- Investigated a Plex "undefined" recording error on a basketball Game 7 attempt; local standby Live TV proxy had no traffic, sports lineup/guide endpoints were healthy, and the suspected NBA Pass stream returned MPEG-TS data.
- Root cause: no-EPG event sports channels were exposed with a week-long placeholder programme, which kept the row visible but gave Plex a bad DVR scheduling target.
- Patched XMLTV fallback generation so parseable live/next sports event names get bounded 3-hour programme windows from explicit timestamps or named North American timezone strings.
- Live deploy: installed the patched binary on the active tuner host, restarted the primary and sports Tunerr services, and verified the sports DVR reactivated all 160 mappings.
- Validation: DET/CLE event row now publishes `20260517230000 +0000` to `20260518020000 +0000`; direct stream pull returned MPEG-TS data; `./scripts/verify` passed.
- Follow-up: added sport-aware fallback durations so Plex users can record from Plex without Tunerr prompts: basketball/hockey 3.5h, soccer/rugby 2.5h, baseball 4.5h, plus extra padding for Game 7/finals/playoff wording.
- Live redeploy: installed the refined duration build, restarted the primary and sports services, and verified the DET/CLE row now publishes `20260517230000 +0000` to `20260518023000 +0000`; Plex activated all 158 sports mappings.
- Incident follow-up: after the user's retry still showed a Plex undefined error, Tunerr had no new Plex tune request. The matching event row was stream `1634335`, guide channel `10129`, `NEXT | DET - PISTONS VS CLE - CAVALIERS | Sun 17 May 19:00 EDT (US) | 8K EXCLUSIVE | US: NBA PASS PPV 1`.
- Emergency capture: started an operator-side ffmpeg recording of stream `1634335` to `/var/lib/iptvtunerr/emergency-recordings/det-cle-game7-20260518T001150Z.ts` and confirmed the file was growing.
- Record-button fix: PMS logs showed the shared user's `GET /media/subscriptions/template?guid=tv.plex.xmltv...` returned `403`; patched and deployed the Live TV proxy so XMLTV-backed media subscription template/create requests are classified as Live TV and borrow owner tuner entitlement. Authorized validation returned `200` for the formerly failing path.
- Verification: focused XMLTV tests passed; `./scripts/verify` passed on rerun. The first full run hit the existing `TestServer_reapplyDeferredGuidePolicyDoesNotCumulativelyShrink` timing flake, then rerun passed without code changes.

## 2026-05-16 - Cut v0.1.76 release

- Promoted the Unreleased security/dependency/CI notes into `v0.1.76`.
- Patched local-runner workflow failures by replacing Debian-only package installs with `scripts/install-ci-tools.sh` and making CodeQL use `actions/setup-go` plus a manual Go build.
- Committed and pushed release prep as `80004d4`, pushed annotated tag `v0.1.76`, and the GitHub release job uploaded binary, archive, `.deb`, `.rpm`, checksum, and manifest assets.
- Post-release CI follow-up: replaced the Gitleaks third-party action with direct CLI installation/execution because the action cache extraction failed on the local self-hosted runner.
- Post-release channel follow-up: patched PPA, COPR, and Snap workflows for local-runner portability; successful reruns published PPA, COPR, and Snap.
- Verification: `./scripts/release-readiness.sh` passed; macOS bare-metal and Windows package smoke were skipped by default; latest `main` CI, CodeQL, Gitleaks, Local Identity Leak Check, PPA, COPR, and Snap runs completed successfully.

## 2026-05-16 - GitHub PR and security maintenance

- Merged Dependabot PRs `#19` (`github.com/hanwen/go-fuse/v2` to `v2.10.1`) and `#20` (`golang.org/x/net` to `v0.54.0`).
- Applied the stale/conflicting Dependabot PR `#15` directly by upgrading `github.com/andybalholm/brotli` to `v1.2.1` and refreshing `vendor/`.
- Hardened CodeQL findings by removing raw apparent-source and trusted proxy header values from Plex label proxy logs, constraining Plex provider lineup URLs to the configured provider origin, and rejecting invalid HDHomeRun lineup URL schemes/credentials.
- Switched GitHub Actions Linux jobs to the local self-hosted Linux runner labels and Windows package jobs to self-hosted Windows labels.
- Updated the Plex Live TV entitlement proxy runbook for the new redacted audit log fields.
- Verification: focused package tests passed; `./scripts/verify` passed.

## 2026-05-15 - Prepare v0.1.75 release

- Promoted the populated Unreleased changelog notes for Plex Live TV proxy stability and release-process fixes into `v0.1.75`.
- Confirmed the repo was already clean on `main` before release-prep edits; no unrelated dirty files were present to include.
- Verification: `./scripts/verify` passed; `./scripts/release-readiness.sh` passed.

## 2026-05-14 - Fix Winget install validation path

- Inspected `microsoft/winget-pkgs#374269` after the Microsoft package-manager bot reported a general installation failure.
- Root cause: the submitted ZIP portable manifest pointed `NestedInstallerFiles.RelativeFilePath` at root-level `iptv-tunerr-v0.1.68-windows-amd64.exe`, but the release ZIP contains `iptv-tunerr-v0.1.68-windows-amd64/iptv-tunerr.exe`.
- Patched `packaging/scripts/update-winget-manifests.sh` to generate the correct nested executable path and added a Windows ZIP nested-path assertion to `scripts/verify-release-assets.sh`.
- Pushed winget-pkgs PR update commit `740f80f081e` with only the manifest path correction.
- Verification: downloaded the v0.1.68 Windows ZIP, confirmed the corrected manifest path exists in the archive, then ran `./scripts/build-release-assets.sh v0.0.0 dist && ./scripts/verify-release-assets.sh v0.0.0 dist`.

## 2026-05-13 - Pause noisy Windows package publishing

- Closed duplicate automated Winget PRs `microsoft/winget-pkgs#374279`, `#374280`, and `#374285`; kept the original `#374269` open for the first validation gate.
- Confirmed `#374269` has a green `license/cla` check after the agreement comment, but is labeled `Internal-Error`, `Needs-Attention`, and `Validation-Guide` by Microsoft's validation system.
- Confirmed Chocolatey package packing succeeds and the public feed has no existing `iptvtunerr` package entry, but `choco push` returns `403 Forbidden` from Chocolatey's push endpoint.
- Removed Chocolatey and Winget from the automatic post-release channel dispatch list; the workflows remain available for manual dispatch after their external gates are resolved.

## 2026-05-13 - Patch Snap release payload layout

- `v0.1.72` confirmed release assets, Docker, AUR, COPR, PPA, and Winget paths were working, while Snap reached the pack step and failed because the command path referenced a versioned directory that was absent from Snapcraft's dump-plugin prime output.
- Changed the Snap workflow to build a root-level `iptv-tunerr` tarball inside `packaging/snap/` and keep the Snap app command as `iptv-tunerr`.
- Chocolatey remains blocked by a remote `403 Forbidden` during `choco push`, which points at package/account/API-key permission rather than local packaging.

## 2026-05-13 - Add AUR release-channel scaffolding

- Added AUR package metadata for `iptvtunerr` (source build from tagged GitHub archive) and `iptvtunerr-bin` (Linux amd64 release asset).
- Added systemd, sysusers, tmpfiles, env, and install metadata under `packaging/aur/`.
- Added AUR helper scripts for SSH setup, clone/push, `.SRCINFO` generation, and checksum validation.
- Added `.github/workflows/release-aur.yml` to publish both AUR packages on GitHub release publication or manual tag dispatch when `AUR_SSH_KEY` is configured.
- Added `AUR_SSH_KEY` to `snapetech/iptvtunerr` GitHub secrets from the local AUR SSH key, then created and pushed the initial `iptvtunerr` and `iptvtunerr-bin` AUR repos.
- Unsealed local OpenBao and checked for release-channel credentials; COPR/Snapcraft credentials were not present there. Added available PPA credentials (`GPG_PRIVATE_KEY`, `LAUNCHPAD_SFTP_KEY`, `LAUNCHPAD_SFTP_USER`) to `snapetech/iptvtunerr`.
- Created Launchpad PPA `ppa:keefshape/iptvtunerr`; Launchpad account `keefshape` has display name `slskdn`.
- Added provided COPR credentials as `COPR_LOGIN` and `COPR_TOKEN` GitHub secrets for `snapetech/iptvtunerr`.
- Installed `snapd` from AUR and `snapcraft` via snap locally; attempted constrained Snapcraft credential export, but it requires Ubuntu One interactive email/password/2FA and no local Snapcraft session exists.
- Confirmed user completed Snapcraft credential export and `SNAPCRAFT_STORE_CREDENTIALS` is present in `snapetech/iptvtunerr`.
- Documented the AUR setup and remaining credential sources in `docs/how-to/release-channels.md`.
- Verification: `bash -n packaging/scripts/*.sh`, AUR checksum validation, `.SRCINFO` generation for both packages, and a Go build/version smoke passed.
- Verification: `./scripts/verify` passed after the packaging changes; AUR `list-repos` shows `iptvtunerr` and `iptvtunerr-bin`; fresh HTTPS clones of both AUR repos include `.SRCINFO`, `PKGBUILD`, and support files.
- Opportunity filed: Snap, Launchpad/PPA, and COPR packaging still need adapted package metadata/workflows.

## 2026-05-13 - Add Windows channel scaffolding

- Added Chocolatey package metadata and install/uninstall scripts for the Windows amd64 GitHub Release ZIP.
- Added Winget manifest generator for `snapetech.iptvtunerr`.
- Added manual GitHub Actions workflows for Chocolatey publishing and Winget PR submission from a release tag.
- Added `WINGETCREATE_GITHUB_TOKEN` from the authenticated GitHub CLI token.
- Documented Windows release-channel status and required secrets in `docs/how-to/release-channels.md`.
- Verification: `bash -n packaging/scripts/*.sh`, Winget manifest generation smoke, and `./scripts/windows-baremetal-package.sh` passed.
- Remaining: native Windows host proof is still recommended.

## 2026-05-13 - Add Snap, PPA, COPR, and Docker release-channel wiring

- Added Snap metadata and `release-snap.yml` to build/publish the strict `iptvtunerr` snap from the release binary.
- Added Debian package metadata and `release-ppa.yml` to build/sign/upload a source package to `ppa:keefshape/iptvtunerr`.
- Added RPM spec and `release-copr.yml` to build/upload an SRPM to the `slskdn/iptvtunerr` COPR project.
- Set repo variable `DOCKERHUB_USERNAME=keefshape`; existing Docker workflow now has the Docker Hub username needed to publish alongside GHCR.
- Added `CHOCO_API_KEY` GitHub secret from the `slskdn` Chocolatey account.
- Documented container, Linux package, and Windows package-channel status in `docs/how-to/release-channels.md`.
- Verification: packaging script syntax passed; `./scripts/windows-baremetal-package.sh` passed. Debian/RPM package CLIs were not installed locally, so PPA/COPR workflows still need first-run validation in Actions.

## 2026-05-13 - Harden tag-only release asset pipeline

- Added `scripts/build-release-assets.sh` to produce raw executables, Linux/macOS tarballs, Windows ZIPs, `SHA256SUMS.txt`, and `release-manifest.json` for Linux amd64/arm64/armv7, macOS amd64/arm64, and Windows amd64/arm64.
- Added `scripts/build-linux-package-assets.sh` to add direct `.deb` and `.rpm` assets to GitHub Releases.
- Added `scripts/verify-release-assets.sh` and wired CI to build and verify the full release asset set with a dummy version.
- Added `scripts/ensure-release-tag-on-main.sh` and wired release, Docker, AUR, Snap, PPA, COPR, Chocolatey, and Winget workflows to reject tags that do not point at current `main`.
- Changed Docker publishing to tag-only; `latest` now moves only on release tags.
- Expanded generated release notes with release asset/checksum details and install notes.
- Verification: installed local `dpkg-deb` and `rpmbuild` tooling, then the full release asset path passed locally for raw binaries, archives, `.deb`, `.rpm`, manifest, and checksum verification.

## 2026-05-13 - Enforce changelog-backed release notes

- Added `scripts/verify-changelog-entry.sh` with staged, range, and release-tag modes.
- Added `.githooks/pre-commit` plus `scripts/install-git-hooks.sh` so local commits reject release-relevant changes that omit `docs/CHANGELOG.md`.
- Wired CI to enforce changelog updates on release-relevant code, workflow, script, packaging, and docs changes.
- Wired the Release workflow to require a populated changelog section for the exact release tag before release notes are generated.
- Documented the hook installation and release changelog rule in `docs/how-to/release-channels.md`.
- Follow-up while testing releases: `v0.1.66` failed direct RPM asset generation on Ubuntu because `rpmbuild` enforced `systemd-rpm-macros`; patched the direct GitHub Release RPM build to use `--nodeps` while leaving COPR metadata intact.
- Follow-up while testing hooks: patched `.githooks/pre-push` to avoid `pipefail` broken-pipe warnings during regex scans.
- Follow-up while testing `v0.1.68`: GitHub Release and Docker succeeded, but release-event package workflows did not fan out from a release created by Actions. Manually dispatched AUR, Snap, PPA, COPR, Chocolatey, and Winget for `v0.1.68`, then patched the Release workflow to explicitly dispatch those channel workflows after asset upload.
- Follow-up while inspecting manual `v0.1.68` channel tests: PPA, Chocolatey, and Winget passed. AUR failed only its tarball layout check, Snap failed because `core24` requires `platforms`, and COPR failed because `slskdn/iptvtunerr` did not exist yet. Patched all three for `v0.1.70`.
- Follow-up while watching `v0.1.69` package-channel auto-dispatch: some queued channel jobs failed the strict tag-equals-current-main guard after a newer release-fix commit advanced `main`. Patched channel workflows to use an ancestor-in-main guard while keeping the primary Release workflow exact.
- Follow-up while watching `v0.1.71` package-channel runs: AUR, COPR, PPA, Winget, Release, Docker, and CI passed. Snap failed because the managed LXD build could not see a source tarball written outside `packaging/snap`; patched the Snap workflow to stage the tarball inside the snap project.

## 2026-05-12 - Expand sports DVR and restore slow-but-working playback

- Expanded `iptvtunerr-sports` from the strict `sports_na` recipe to `sports_now`, set `IPTV_TUNERR_LINEUP_MAX_CHANNELS=480`, disabled runtime lineup probing, and turned sports runtime guide pruning off so non-EPG sports rows remain visible.
- Verified `/lineup.json` and `/guide.xml` expose 480 sports channels and Plex activated all 480 channel mappings.
- Confirmed the guide/EPG path was not the playback blocker: direct Tunerr stream test on `CA| SPORTSNET WORLD FHD` returned about 24 MB over 20 seconds with HTTP 200.
- Patched `plex-label-proxy` to rewrite JSON `allowTuners` hints in addition to XML, because Plex Web was negotiating JSON `/media/providers` and the proxy previously logged that rewrite was skipped.
- Verification: `go test -count=1 ./internal/plexlabelproxy ./cmd/iptv-tunerr` passed; patched proxy binary installed on `kspls0`; `plex-live-tv-proxy.service` restarted; JSON `/media/providers` now reports `allowTuners: true`.
- User confirmed streams do play, but startup remains slow.

## 2026-05-12 - Repair Plex Live TV spinning after Plex Pass switch

- Found the immediate external failure was the Live TV entitlement proxy's persisted abuse block: `/media/providers` from the user's apparent external source was returning `blocked_bad_actor` even when later requests carried a valid Plex token.
- Backed up and removed `/var/lib/iptvtunerr/plex-live-tv-proxy-blocks.json`, restarted `plex-live-tv-proxy.service`, and verified the same source now gets `outcome=elevated_live_tv` with HTTP 200.
- Found Plex was still advertising a dead automatic remote route (`plex.direct:55556`) after NAT-PMP. Cleared `LastAutomaticMappedPort`, enforced `ManualPortMappingMode=1`, `ManualPortMappingPort=443`, `RelayEnabled=0`, and `customConnections=https://media.snape.tech:443`, then restarted Plex.
- Verified PMS stayed on the Plex Pass build `1.43.2.10687-563d026ea`; plex.tv resources now advertise `https://media.snape.tech:443` plus the static `plex.direct:443`, with the stale `:55556` route gone.
- Found the sports tuner lineup had dropped to 0 channels after runtime probing; disabled `IPTV_TUNERR_LINEUP_PROBE_ENABLED` in `/etc/iptvtunerr/sports.env`, restarted `iptvtunerr-sports.service`, and verified `lineup.json` recovered to 100 channels.
- Plex sports DVR activation completed successfully for all 100 channels.

## 2026-05-12 - Restore external Live TV after Plex Pass switch

- External users reported Live TV disappeared after the Plex container was recreated for the Plex Pass update channel.
- Confirmed Plex, Tunerr, and proxy services were active, but PMS returned `503 Maintenance` for several minutes while database migrations completed.
- Verified the public Cloudflare path `https://media.snape.tech` returns `200` for owner-token `/media/providers` and `/livetv/dvrs`; no-token Live TV remains denied.
- Found Plex was advertising both the intended custom remote URL and a direct `plex.direct` public-port URL. Later repair showed the working production state is static manual mapping (`ManualPortMappingMode=1`) with port 443, `RelayEnabled=0`, and `customConnections=https://media.snape.tech:443`.
- Proxy audit showed an external `/media/providers` request from `204.83.235.92` was elevated after PMS recovered.
- Updated the Live TV entitlement proxy runbook so future Plex updates keep static port mode and do not reintroduce automatic NAT-PMP routes.

## 2026-05-12 - Enable Plex Pass PMS update channel on kspls0

- Found `plex-host` was running `linuxserver/plex:latest` without `VERSION`; LSIO logs said the update routine was skipped because `VERSION` was unset.
- Recreated `plex-host` with the same host network, mounts, `/dev/dri`, and group settings plus `VERSION=latest`, which lets the signed-in Plex Pass account receive the newest entitled PMS build.
- Preserved rollback container as `plex-host.pre-version-latest-20260512-165623` and left it stopped.
- PMS upgraded from `1.43.1.10611-1e34174b1` to `1.43.2.10687-563d026ea`; `/identity` returned 200 after startup migrations.
- Validation: `plex-host`, `iptvtunerr-primary`, `iptvtunerr-sports`, and `plex-live-tv-proxy` are active; Live TV channel mappings remain enabled (`385/385` primary, `77/77` sports after guide-policy filtering).
- Updated deployment docs to require/check `VERSION=latest` for the live Plex host container.

## 2026-05-12 - Point local Tunerr env/docs at kspls0

- Updated local `.env` `IPTV_TUNERR_BASE_URL` from `http://kspld0:5004` to `http://kspls0:5004`.
- Updated the stale shared-lease deployment note in `docs/reference/cli-and-env-reference.md` to name `kspls0`.
- Verified `http://kspls0:5004/discover.json` returns 200 and advertises `http://192.168.50.84:5004`.
- Verified `http://kspls0:5004/guide.xml` returns 200 with `X-Iptvtunerr-Guide-State: ready`; response size was 6,937,320 bytes.
- Found Plex channel activation, not Tunerr guide serving, was the slow step: primary `/guide.xml` served in milliseconds, but Plex's full 385-channel activation PUT exceeded the old 60s client timeout.
- Live repair: sent full primary and sports channelmaps with a longer timeout; Plex now reports enabled `ChannelMapping` rows: primary `385/385`, sports `79/79`.
- Code fix: raised `ActivateChannelsAPI` timeout to 5 minutes and removed token-bearing activation URLs from timeout errors.
- Verification: `go test -count=1 ./internal/plex ./cmd/iptv-tunerr` passed; patched binary installed on `kspls0`; both Tunerr services restarted and `/discover.json` checks passed.

## 2026-05-12 - Fix external Plex Live TV abuse-block false positive

- External user reported the Plex Live TV provider unavailable again.
- Found the live proxy had temporarily blocked the user's apparent source after repeated missing-token Live TV probes, and the source-level block was also rejecting later `/media/providers` requests that carried valid Plex tokens.
- Immediate live fix: cleared the persisted abuse-block state and restarted `plex-live-tv-proxy.service`; the affected source resumed `elevated_live_tv` requests immediately.
- Code fix: source blocks now apply only after checking for an authorized inbound Plex token. Owner tokens and tokens already authorized for the Plex server bypass the source block; missing/unauthorized tokens remain blocked or cooled down.
- Verification: public `/identity` returned `200`, public no-token `/livetv/dvrs` returned `403`, all media services remained active, and `go test -count=1 ./cmd/iptv-tunerr ./internal/plexlabelproxy` passed.

## 2026-05-18 - Prepare v0.1.79 Plex DVR token-audit release

- Promoted the repeated URL-decoding DVR classifier hardening, safe subscription-save diagnostics, and Web UI smoke retry fix into the `v0.1.79` changelog section.
- Confirmed the prior `v0.1.78` release workflow eventually completed successfully, including Discord, Matrix, and package-channel dispatch.
- Verification before release prep: focused proxy tests, direct binary smoke, and full `./scripts/verify` passed.

## 2026-05-18 - Audit Plex Live TV token elevation gaps

- Relaxed Live TV/XMLTV text detection for repeated URL-encoding so Plex DVR save hints that arrive only as double-encoded `ratingKey` values still borrow owner tuner entitlement.
- Added safe access logging for `/media/subscriptions*` requests even when they are intentionally not elevated; logs include redacted query-key names, status, token fingerprint, and no raw values.
- Checked recent live proxy logs for fresh post-release subscription denials; the sampled window showed no new failing DVR traffic.
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed.
- Follow-up: fixed a binary-smoke Web UI sidecar port-collision retry bug discovered during full verification; the tuner process can stay alive after only the sidecar listener fails, so the smoke script now kills/retries based on bind-failure log evidence.
- Verification: direct `bash scripts/ci-smoke.sh` passed; full `./scripts/verify` passed after the smoke retry fix.
- Live deploy: installed the patched proxy binary and restarted only the Live TV proxy service.
- Live validation: a no-token double-encoded XMLTV `hints[ratingKey]` subscription save probe logged `live_tv=true` with redacted query keys and was denied before elevation; a no-token library subscription probe stayed `live_tv=false` and logged redacted query keys/status.

## 2026-05-18 - Prepare v0.1.78 Plex DVR release

- Promoted the shared-user Plex DVR subscription-save classifier fix into the `v0.1.78` changelog section.
- Included release-readiness generated council metadata in the release-prep dirty tree as requested.
- Verification: `./scripts/release-readiness.sh` passed; optional macOS and Windows package lanes were skipped by default.

## 2026-05-12 - Harden Plex Live TV proxy abuse controls

- Added env/CLI knobs for Live TV bad-source blocking threshold/window/duration, denied source+token authorization cooldown, optional persisted block state, and audit summary interval.
- Changed source identity handling to prefer trusted `CF-Connecting-IP`, then trusted `X-Forwarded-For`, then socket remote address; forwarded source headers are trusted only from loopback/private frontend peers.
- Added optional persisted JSON block/cooldown state so bad actors remain blocked across proxy restarts without storing Plex tokens.
- Added aggregate `plexlabelproxy_audit_summary` counters for elevated, denied, cooldown, cache-hit/cache-miss, and blocked requests.
- Updated `docs/scripts/validate-plex-live-tv-proxy.sh` to verify owner success, random/no-token denial, optional shared-user success, and repeated bad-attempt blocking.
- Verification: `bash -n docs/scripts/validate-plex-live-tv-proxy.sh`, `go test -count=1 ./internal/plexlabelproxy ./cmd/iptv-tunerr`, and `./scripts/verify` passed.

## 2026-05-12 - Validate v0.1.63 live Plex/Tunerr/proxy stack

- Confirmed GitHub `v0.1.63` release, CI, Docker, CodeQL, and Gitleaks completed successfully.
- Reinstalled the live proxy binary with `-X main.Version=v0.1.63`; `/opt/iptvtunerr/iptv-tunerr-proxy --version` now reports `v0.1.63`.
- Live services checked: Plex host, proxy, primary Tunerr bridge, sports Tunerr bridge, frontend tunnel, and VODFS services are running.
- Tunerr endpoints checked: primary and sports `/discover.json`, `/lineup.json`, `/lineup_status.json`, and `/guide.xml` all return `200`.
- Plex DVR state checked: two intended Tunerr DVRs, both enabled, with 479 and 92 enabled channel mappings.
- Entitlement checks: local and public Live TV owner-token paths return `200`; public no-token and fake-token Live TV paths return `403`; local non-Live-TV no-token path returns Plex `401` without entitlement audit noise.
- Added automated proxy coverage for an external shared user coming through forwarded frontend headers: shared token is authorized, Live TV request is elevated to owner token, source headers are retained for audit context, and raw tokens are not logged.
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed; `./scripts/verify` passed.

## 2026-05-20 - Fix title-only Plex recording identity for short recurring shows

- Followed tester feedback that Record appeared to work for `etalk` but the guide did not show a red recording marker.
- Found Plex had created a title-only subscription `tv.plex.xmltv://movie/etalk` at `2026-05-20 18:00:08 CST`, while `/media/subscriptions/scheduled` had no `etalk` grab. Proxy logs showed no tester subscription POST, so the remaining issue was guide identity, not proxy save classification.
- Patched XMLTV output so metadata-poor short recurring rows such as `etalk` get date-specific `sub-title`, `date`, and deterministic `episode-num system="iptvtunerr"` fields; rows with existing episode metadata are preserved.
- Deployed the patched binary to the live tuner host, restarted primary/sports/proxy services, verified live `/guide.xml` now includes stable identity on `etalk` rows, forced Plex guide reloads for both DVRs, and deleted stale title-only subscription `783`.
- Verification: focused XMLTV tests and `go test -count=1 ./internal/tuner` passed.

## 2026-05-20 - Harden shared-user Plex DVR scheduled-recording visibility

- Patched the Live TV entitlement proxy so read-only `/media/subscriptions*` detail and scheduled-recording paths borrow owner tuner entitlement as DVR discovery, except `/media/subscriptions/template`, which still requires Live TV/XMLTV evidence.
- Kept mutating subscription edits scoped to XMLTV evidence and broadened form-body scanning to recognize Plex `hints[...]` fields such as `hints[ratingKey]`.
- Added regression coverage for subscription detail reads, scheduled detail reads, and body-carried XMLTV hint fields.
- Documented the new recurring Plex post-save detail-read loop in `memory-bank/recurring_loops.md`.
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed; `./scripts/verify` passed.
- Live deploy: installed the patched proxy binary to the standalone Live TV proxy service and restarted `plex-live-tv-proxy.service`.
- Live validation: proxy `/identity` returned `200`; owner-token `/media/subscriptions` and `/media/subscriptions/scheduled` returned `200`; no-token `/media/subscriptions/42` returned `403`.

- 2026-05-18: Added reusable public package-channel smoke validation. New `packaging/smoke/package-smoke` installs from release/package channels, records JSON/JUnit/log evidence, checks requested-version reporting, and supports cleanup hooks. GitLab now has a tag-only `post_release_validate` stage for container, Ubuntu, Fedora, AUR, and Snap channels; GitHub has equivalent disabled `workflow_dispatch` scaffolding. Verification: shell syntax, YAML parsing, evidence success/failure checks, and `git diff --check` passed.

## 2026-05-12 - Block repeated bad Plex Live TV elevation attempts

- Added an in-process temporary block for repeated missing-token or unauthorized-token Live TV elevation attempts from the same apparent source.
- Default policy: 5 failed Live TV elevation attempts within 5 minutes blocks that source from Live TV entitlement paths for 30 minutes; ordinary non-Live-TV paths are not blocked by this guard.
- Added redacted audit outcomes for block creation (`bad_actor_blocked`) and blocked requests (`blocked_bad_actor`).
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed; `./scripts/verify` passed.
- Live deploy: installed the patched proxy binary and restarted `plex-live-tv-proxy.service`.
- Live validation: `/library/sections` without a token returned `401` without an audit denial; six bad Live TV requests from a synthetic source returned `403` for the first five and `429` for the sixth, with redacted block audit logs.
- Monitoring: watched the live audit journal for 3 minutes after deploy; observed authorized Live TV elevation only, plus the synthetic block validation.

## 2026-05-12 - Add Plex Live TV proxy security audit logging

- Added redacted `plexlabelproxy_audit` log lines for Live TV owner-token elevation, missing-token denial, and unauthorized-token denial decisions.
- Audit fields include method, path, Live TV classifier state, remote address, proxy source headers, and a SHA-256 token fingerprint; raw Plex tokens are not logged.
- Updated the Live TV entitlement proxy runbook with audit queries for operator checks.
- Verification: `go test -count=1 ./internal/plexlabelproxy` passed; `./scripts/verify` passed.
- Live deploy: rebuilt and installed the proxy binary, restarted `plex-live-tv-proxy.service`, and confirmed live missing-token/fake-token probes return `403` while emitting audit lines.

## 2026-05-20 - Fix stale Plex recording marker for generic live sports rows

- Found Plex had a stale DVR subscription keyed only to `tv.plex.xmltv://movie/Live%3A%20NBA%20Basketball`, so every current `Live: NBA Basketball` XMLTV row could inherit the old recording marker.
- Patched XMLTV output so recurring event-like rows keep their visible title but gain date-specific `sub-title`, `date`, and deterministic `episode-num system="iptvtunerr"` fields, preventing Plex from collapsing future events into the same title-only movie identity.
- Broadened coverage beyond the observed NBA row to all `Live:` rows and generic sports event titles such as plain `NBA Basketball`.
- Added regression tests for both the external XMLTV remap path and the real merged EPG provider-import path.
- Deleted the stale Plex subscription, deployed the patched live Tunerr binary, restarted both Tunerr bridge services, and forced Plex guide reloads for both DVRs.
- Verification: `./scripts/verify` passed; live guide rows now include date/subtitle/episode identity fields; Plex subscriptions list is empty; primary/sports/proxy services are active; direct affected TSN stream returned MPEG-TS data.

## 2026-05-21 - Fix short recurring show recording identity and harden release channels

- Found Plex saved the tester's `etalk` attempt as title-only `tv.plex.xmltv://movie/etalk` with no scheduled grab, while proxy subscription reads were healthy.
- Patched XMLTV output so metadata-poor short recurring shows get date-specific `sub-title`, `date`, and deterministic `episode-num system="iptvtunerr"` fields without overwriting existing upstream episode metadata.
- Deployed the patched binary to the live tuner host, restarted Tunerr/proxy services, verified `etalk` guide rows now carry stable identity, reloaded both Plex DVR guides, and deleted the stale title-only subscription.
- Investigated `v0.1.81` package-channel failures: Docker hit a transient Docker Hub DNS timeout, COPR lacked `requests-gssapi` for configured GSSAPI auth, and Launchpad rejected the PPA upload after Actions reported success because the `.dsc` was not readable during incoming processing.
- Hardened release workflows: Docker build/push retries once, COPR installs `requests-gssapi`, and PPA upload validates the `.changes` file manifest then uploads payloads, `.dsc`, and `.changes` with retries in a safer order.
- Follow-up: `v0.1.82` Docker still failed because the BuildKit container could not resolve `registry-1.docker.io` through `8.8.8.8`; patched BuildKit setup to use explicit DNS servers and the Docker Hub mirror before cutting `v0.1.83`.
- Follow-up: `v0.1.83` Docker still used `8.8.8.8` despite inline DNS config, so the workflow now generates BuildKit DNS config from the runner resolver; COPR also now prefers configured API token credentials before attempting Fedora OTP/Kerberos fallback auth.
- Release: tagged and published `v0.1.84`; GitHub Release, Docker, AUR, PPA, COPR, CI, CodeQL, Gitleaks, and local-identity checks completed successfully. Launchpad PPA acceptance still requires watching asynchronous Launchpad email after the green upload job.
- Verification: focused XMLTV tests, `go test -count=1 ./internal/tuner`, YAML parsing for edited workflows, `git diff --check`, and `./scripts/release-readiness.sh` passed.

## 2026-05-20 - Release and deploy v0.1.80

- Committed the full requested dirty tree, including unrelated files, as `984b24e` and pushed `main`.
- Tagged and pushed annotated release tag `v0.1.80`.
- Published GitHub release `v0.1.80` with binary archives, raw binaries, package assets, `SHA256SUMS.txt`, and the release manifest.
- Deployed the built `v0.1.80` binary to the live network primary/sports Tunerr services and Live TV proxy service, restarted all three services, and confirmed both Tunerr `/readyz` endpoints report ready.
- Verification: `./scripts/verify`, `./scripts/release-readiness.sh`, local service health checks, live guide identity spot checks, and GitHub release asset publication passed.
- Follow-up: Docker tag workflow initially failed on a transient Docker Hub DNS lookup and was rerun; GitHub Actions status was still queued/in cleanup at handoff while the GitHub release and live deploy were complete.

## 2026-05-12 - Harden Plex Live TV entitlement proxy

- Found the live `plex-live-tv-proxy.service` was running `-elevate-all`; unauthenticated public requests to the media frontend returned `200`, so the proxy was effectively acting as an owner-token deputy for anyone who could reach it.
- Added proxy-side elevation gating: missing inbound Plex tokens are never elevated, and production CLI wiring validates inbound tokens against PMS `/library/sections` before borrowing the owner token.
- Narrowed Live TV elevation classification to known Live TV paths/helpers and safe methods, with `POST /playQueues` allowed only for Live TV stream starts.
- Updated runbook/reference/systemd docs to describe the no-friction model: Plex clients keep sending their own Plex tokens; only already-shared users can borrow owner tuner entitlement for Live TV.
- Verification: `./scripts/verify` passed; `./scripts/release-readiness.sh` passed.
- Live deploy: installed patched `/opt/iptvtunerr/iptv-tunerr-proxy`, backed up the previous binary/service under `/opt/iptvtunerr/backups/`, switched `plex-live-tv-proxy.service` from `-elevate-all` to `-elevate-live-tv -neutralize-owner-history`, and restarted it.
- Live validation: owner token returns `200`; fake token returns `401` for `/library/sections` and `403` for `/livetv/dvrs`; missing token returns `401` for libraries and `403` for Live TV; public direct `media.example.com:32400` timed out.

## 2026-05-12 - Clarify k3s remains supported

- User clarified the deployment boundary after `v0.1.59`: k3s is still a supported user/lab deployment mode; the removed fallback was only the local production split-brain path.
- Added `docs/how-to/k3s-deployment.md` with Secret, Deployment, Service, probe, Plex reachability, HDHomeRun discovery, and multi-DVR ownership guidance.
- Updated deployment docs, README, docs index, and changelog wording to include k3s while preserving the rule that the local production host is systemd-owned.
- Verification: `./scripts/verify` passed; `./scripts/release-readiness.sh` passed.

## 2026-05-12 - Prepare v0.1.59 deployment-contract release

- Added release notes for DVR safety, token redaction, retired orchestration cleanup, and supported deployment contract.
- Documented supported deployment as binary, systemd, Docker/container-on-host, or k3s with one active Tunerr owner per Plex DVR identity.
- Documented duplicate/empty Plex DVR recovery order in Plex ops patterns.
- Verification: targeted retired-path searches were clean; `./scripts/verify` passed; `./scripts/release-readiness.sh` passed.

## 2026-05-12 - Remove old cluster Tunerr/Plex fallback

- Deleted live Tunerr/Plex remnants from the orchestration namespace, including stale Tunerr deployments and matching proxy/config/secret leftovers.
- Removed repo deployment artifacts for that path: manifest tree, cluster deploy workflows, deploy scripts, Plex runbooks, and cluster-specific helper scripts.
- Updated docs/scripts/code fixtures to stop pointing operators or agents at service-DNS DVR URLs, cluster commands, or cluster recovery paths.
- Active local production direction is systemd/bare-metal ownership; k3s remains supported for users/labs when it is the single Plex DVR owner for its identity.

## 2026-05-12 - Stop the Plex host Plex DVR zombie spam

- Root cause was duplicate registrars. Bare-metal systemd Tunerr services and the removed cluster Tunerr path were registering the same Plex device IDs/friendly names with different guide URLs. Plex accumulated empty `0/0` DVR rows and entered repeated `/livetv/dvrs` timeout / maintenance windows.
- Live fix kept the host/systemd Tunerr services as the single owner, installed the patched binary, and deleted twelve empty IPTV DVR rows.
- Code fix: the watchdog no longer re-registers solely because Plex marks a device `dead` while mappings remain healthy; activation request timeout errors redact token-bearing URLs.
- Verification at that point: `./scripts/verify` passed.

## 2026-06-16 - Resolve open PR and security queues

- Inspected the full open GitHub queue: Dependabot PRs #22, #23, #26, #27, #28, and #29; Dependabot security alerts for `esbuild` and `react-router`; code scanning; secret scanning; and open issues.
- Landed one consolidated update on `main`: Vite/React tooling plus Node 22 pinning, React Router 6.30.4, Go `x/crypto` 0.53.0, Go `x/net` 0.56.0, vendored Go module refresh including `x/sys` 0.46.0, and `actions/checkout@v6` / `actions/upload-artifact@v7` in the disabled package-smoke workflow.
- Rebuilt the shipped Web UI static bundle and updated the changelog so the dependency/workflow changes pass the repo changelog gate.
- Closed superseded Dependabot PRs #22, #23, #26, #27, #28, and #29 with explicit resolution comments after the consolidated commit landed.
- Final GitHub state: open PRs, open Dependabot alerts, open code-scanning alerts, open secret-scanning alerts, and open issues are all empty.
- Verification: `npm --prefix web audit` reported zero vulnerabilities; `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` reported no called Go vulnerabilities; `npm --prefix web run build`, `git diff --check`, `./scripts/check-local-identity-leaks.sh`, and full `./scripts/verify` passed.
- GitHub verification: CI, CodeQL, Gitleaks, Dependency Graph, and Local Identity checks passed for the consolidated dependency/security commit.

## 2026-06-16 - Commit dirty tree and publish v0.1.85

- Fast-forwarded local `main`, reapplied the requested dirty and unrelated tree, resolved generated-file conflicts, regenerated council outputs, promoted the changelog, and committed the full requested tree as `1c7fcd7`.
- Published `v0.1.85`: pushed `main`, pushed the annotated `v0.1.85` tag, and confirmed the GitHub Release workflow published release assets and manifest/checksum assets.
- Release content: Plex/PMS canonical env preference and startup drift warnings, Live TV proxy rolling cleanup classifier tightening, PR/security dependency upgrades from the consolidated sweep, docs/changelog/memory updates, and generated council state.
- Verification before release: focused PMS/runtime/proxy/setupdoctor tests, changelog gate, local identity scan, staged diff check, and full `./scripts/release-readiness.sh` passed.
- GitHub verification for the release commit: Release, Docker, AUR, PPA, main CI, CodeQL, Gitleaks, and Local Identity checks completed successfully.
- COPR follow-up: initial COPR publish failed because the configured COPR API token was invalid or expired. Pushed `f5c79db` to retry configured Fedora GSSAPI fallback after token-auth expiry and reran COPR for `v0.1.85`.
- COPR final state: rerun `27634610662` still failed before upload. The workflow attempted fallback, but COPR returned a non-JSON response for write auth; browser GSSAPI login returned `401 Unauthorized`; OIDC required Fedora username/password/OTP; no readable local/OpenBao replacement secret was available.
- Filed known issue: COPR package publishing requires rotating `COPR_LOGIN`/`COPR_TOKEN` or adding valid COPR Kerberos keytab secrets, then rerunning `release-copr.yml` for `v0.1.85`.
