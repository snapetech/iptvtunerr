# Work breakdown

## 2026-09-30 - Structured release notes and Discord announcements

Objective: bring Tunerr's release-note curation, changelog preparation, and Discord announcement quality in line with Seerrng while retaining all published history.

| Story ID | Scope | Status |
| --- | --- | --- |
| RELNOTE-001 | Add validated append-only release-note fragments and preview/validation tooling; require a fragment or explicit internal-only opt-out in CI. | Completed |
| CHANGELOG-001 | Prepare versioned changelog sections from fragments since the previous tag and audit that each existing release tag has one preserved changelog section. | Completed |
| DISCORD-001 | Send the generated GitHub release body through bounded, verified Discord embeds after release publication. | Completed |
| DOCS-001 | Document fragment authorship and the release preparation/announcement flow; preserve private-identity restrictions. | Completed |

Guardrail: do not rewrite or move existing release tags or changelog history, publish a new release, or change Matrix announcement behavior in this work.

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
