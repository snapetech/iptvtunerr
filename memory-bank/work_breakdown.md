# Work breakdown

## 2026-10-01 - API-Sports Sports Automation

Objective: implement the supplied M3U Web Picker Sports Automation model in Tunerr: canonical API-Sports schedule anchors, provider/XMLTV reconciliation, temporary event channels, and operator API/WebUI controls.

| Story ID | Scope | Status |
| --- | --- | --- |
| SPH-001 | Replace the scoreboard client with fixed MLB/NFL/NCAA schedule adapters, normalized events, bounded persistent date cache, stale fallback, request/quota status, and server-only credentials. | Completed |
| SPH-002 | Add automation settings and matching rules, then reconcile canonical event anchors against current Tunerr channels and merged XMLTV with conservative team/time matching. | Completed |
| SPH-003 | Generate event-scoped stable identities and separate temporary sports M3U/XMLTV outputs that route only through Tunerr's existing stream gateway. | Completed |
| SPH-004 | Add operator-gated configuration/status/events/refresh APIs and typed WebUI functions. | Completed |
| SPH-005 | Replace the scoreboard page with Sports Automation controls, dataset/cache health, matched/unmatched events, and generated feed links. | Completed |
| SPH-006 | Update feature/how-to/reference docs and memory; run prescribed non-test verification without committing or publishing. | Completed |
| SPH-007 | Add README TOC and feature coverage, prepare a user-facing release note, keep Snap and Docker Hub paused while retaining GHCR `latest`, run release verification, then commit, push, and publish v0.1.88 by fast-forward. | Completed; release and applicable package channels published |
| SPH-008 | Retry transient Launchpad API polling errors, allow delayed exact source discovery for up to 20 minutes, preserve the separate amd64 binary publication wait, and audit the existing `v0.1.88` PPA builds. | Poller fixes are on `main`; exact Jammy/Noble amd64 binaries remain unpublished in Launchpad |

Guardrail: preserve manual channels and standard lineup state; API-Sports is canonical schedule data, never a stream provider. Use fixed upstream hosts, conservative date windows, cached schedule fallback, and only current Tunerr catalog channels for generated event playback. Do not expose the API key or provider stream URLs. Preserve published Git history and tags; push only by normal fast-forward. Skip Snap and Docker Hub for this release.

## 2026-09-30 - Opportunity backlog implementation sweep

Objective: implement all repo-owned open opportunities, reconcile the items that require package-service or live-host state, and push the result without rewriting history.

| Story ID | Scope | Status |
| --- | --- | --- |
| OPP-001 | Preserve explicit per-channel profile overrides for Plex internal fetchers when a global internal-fetcher profile is configured; document precedence. | Completed |
| OPP-002 | Replace private self-hosted runner paths in public Actions jobs with compatible GitHub-hosted Ubuntu/Windows runners; update release-channel docs. | Completed |
| OPP-003 | Send the same curated release notes to Matrix as to GitHub/Discord, with bounded chunks and safe HTML rendering. | Implemented; first delivery awaits the next release |
| OPP-004 | Reconcile first-run package and direct package asset results for v0.1.86; retain only absent Snap or new-version external gates. | Completed for configured channels and direct assets; Snap publisher is not configured |
| OPP-005 | Check whether the single CI retry failure recurred; fix only if source evidence identifies a repeatable defect. | Closed for now; no recurrence found and latest main CI is green |
| OPP-006 | Run the existing Windows native smoke on a GitHub-hosted Windows runner when Go code changes reach main. | Completed; run 36793770645 succeeded |
| OPP-007 | Reconcile the Plex Live TV watchdog schedule and snapshots; separate repo-owned implementation from host-managed state. | Pending deployment-host timer/service inspection and a fresh snapshot |

Guardrail: do not publish package versions, change a live host, or rewrite history. Preserve the original dirty local checkout; let normal post-push CI provide code and Windows smoke evidence without running tests locally.

## 2026-09-30 - Structured release notes and Discord announcements

Objective: bring Tunerr's release-note curation, changelog preparation, and Discord announcement quality in line with Seerrng while retaining all published history.

| Story ID | Scope | Status |
| --- | --- | --- |
| RELNOTE-001 | Add validated append-only release-note fragments and preview/validation tooling; require a fragment or explicit internal-only opt-out in CI. | Completed |
| CHANGELOG-001 | Prepare versioned changelog sections from fragments since the previous tag and audit that each existing release tag has one preserved changelog section. | Completed |
| CHANGELOG-002 | Backfill link-only sections for historical published releases missing from the changelog and document tags with no published GitHub Release. | Completed; 17 links added, two exceptions documented, and CI passed on retry. |
| DISCORD-001 | Send the generated GitHub release body through bounded, verified Discord embeds after release publication. | Completed |
| DOCS-001 | Document fragment authorship and the release preparation/announcement flow; preserve private-identity restrictions. | Completed |

Guardrail: do not rewrite or move existing release tags or changelog history, publish a new release, or change Matrix announcement behavior in this work.

## 2026-09-30 - Release-channel version parity

Objective: ensure every configured release channel receives the same tagged build, publish `v0.1.87` to lagging channels, and fail visibly when publisher jobs do not make the release available.

| Story | Acceptance criteria | Status |
| --- | --- | --- |
| RELCH-001 | Chocolatey and Winget receive `v0.1.87`; future release dispatch includes both alongside AUR, PPA, COPR, and Snap. | Completed: Chocolatey `0.1.87` is in the public feed and Winget PR #444749 merged |
| RELCH-002 | GHCR and both Docker Hub names publish matching `latest` and `v0.1.87` tags from one image build; verify registry digests. | Partial: GHCR and `snapetech/iptvtunerr` tags match digest `sha256:baab1bd9ff3a640ee951c2e994f626f2187e9a1469b204c132433ca6a4831a6f`; `keefshape/iptvtunerr` push was denied `insufficient_scope`; further Docker Hub work is deferred per user request |
| RELCH-003 | Existing registered Snap package has a least-privilege Go CLI Snap, publisher workflow, and `v0.1.87` stable submission. | Deferred per user request; Store credential rejected and public stable remains `0.1.78` |
| RELCH-004 | Launchpad publisher waits for published amd64 binaries, and `v0.1.87` reaches supported PPA series. | In progress: Noble and Jammy builds succeeded; exact source records remain `Pending`; publisher run `36809148695` is still waiting |
| RELCH-005 | CI enforces the exact publisher map, latest stable tag guard, no duplicate release triggers, and docs record external review gates. | Completed; wiring checks, full CI, and security checks pass on `793c665` |

Guardrail: use the existing immutable `v0.1.87` release tag and preserve package history. Do not bypass Chocolatey, Microsoft, Launchpad, COPR, or Snap Store validation. Snap and Docker Hub follow-up is deferred as requested.

## 2026-09-30 - PR/security sweep, tester report, and v0.1.86

Objective: incorporate all currently open PRs, resolve repository CodeQL/dependency findings, fix the tester-reported WebUI/provider state mismatch, then cut the next patch release.

| Story ID | Scope | Status |
| --- | --- | --- |
| SWEEP-001 | Review and land open GitHub PRs #30-#40, resolving changelog/check failures without bypassing substantive CI. | Completed; #40 closed as superseded by the already-landed secure upgrade. |
| SEC-001 | Fix CodeQL `go/path-injection` alert #61 by rejecting request-supplied alias paths in the three tuner guide diagnostic endpoints; verify the remote alert closes after push. | Completed; current-main analysis passed and the alert is closed. |
| TESTER-001 | Fix WebUI XMLTV route mismatches, surface the current tuner lineup, and explain the separate runtime and database-backed provider/channel views. | Completed |
| PRIV-001 | Resolve the Local Identity Leak Check finding while preserving already-published history; do not weaken the scanner. | Current-main check passes after `v0.1.86` became the latest-tag baseline. The pre-release check matched five historical co-author trailer lines; they remain unchanged under the user's no-rewrite instruction. |
| REL-001 | Update changelog/docs and memory closeout, run prescribed verification/release readiness, push to GitHub by fast-forward, publish `v0.1.86`, and check release workflows/security queues. | Completed for GitHub: release published and verification/assets passed. Local Identity Leak Check remains unresolved under the no-history-rewrite instruction; GitLab sync is tracked separately below. |
| GITLAB-001 | Reconcile GitLab `main` with the verified GitHub release by a normal merge that retains the donation-links commit; never force-push. | Blocked pending GitLab server-side multi-pack-index repair. Candidate `4b5d293` was rejected by normal push; no remote ref changed. Rebuild from then-current GitHub `main` after repair. |

Guardrail: keep code changes scoped to the reported WebUI state mismatch, the CodeQL path finding, and required release notes. Do not change Jellyfin, Plex, deployment hosts, or unrelated packaging behavior.

## 2026-06-16 - PR and security sweep

Objective: resolve the full open Dependabot PR queue and current repository security alerts with one verified consolidated update.

| Story ID | Scope | Status |
| --- | --- | --- |
| SEC-PR-001 | Resolve web Dependabot/security alerts for `esbuild` and `react-router` by updating Vite/React Router tooling and validating `npm audit`. | Completed |
| SEC-PR-002 | Resolve Go module PRs for `golang.org/x/crypto` and `golang.org/x/net`, refresh vendor, and validate with Go vulnerability checks. | Completed |
| SEC-PR-003 | Resolve GitHub Actions PRs for `actions/checkout` and `actions/upload-artifact`, including the changelog gate failure. | Completed |
| SEC-PR-004 | Reconcile GitHub PR state after landing: verify alerts/checks, then close or merge every superseded PR with explicit comments. | Completed |

Guardrail: keep this sweep limited to dependency, workflow, vulnerability, and required memory-bank/changelog updates. Do not change unrelated Plex/tuner behavior.
