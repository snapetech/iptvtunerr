# Work breakdown

## 2026-09-30 - PR/security sweep, tester report, and v0.1.86

Objective: incorporate all currently open PRs, resolve repository CodeQL/dependency findings, fix the tester-reported WebUI/provider state mismatch, then cut the next patch release.

| Story ID | Scope | Status |
| --- | --- | --- |
| SWEEP-001 | Review and land open GitHub PRs #30-#39, resolving changelog/check failures without bypassing substantive CI. | Completed |
| SEC-001 | Fix CodeQL `go/path-injection` alert #61 by rejecting request-supplied alias paths in the three tuner guide diagnostic endpoints; verify the remote alert closes after push. | In progress |
| TESTER-001 | Reproduce and fix the WebUI showing no provider/channel/guide data when the configured tuner is serving Live TV successfully to Jellyfin. | Open |
| REL-001 | Update changelog/docs and memory closeout, run prescribed verification/release readiness, push to configured GitHub and GitLab destinations, publish `v0.1.86`, and check release workflows/security queues. | Open |

Guardrail: keep the code change scoped to the reported WebUI state mismatch and the CodeQL path finding. Do not change Jellyfin, Plex, deployment hosts, or unrelated packaging behavior.

## 2026-06-16 - PR and security sweep

Objective: resolve the full open Dependabot PR queue and current repository security alerts with one verified consolidated update.

| Story ID | Scope | Status |
| --- | --- | --- |
| SEC-PR-001 | Resolve web Dependabot/security alerts for `esbuild` and `react-router` by updating Vite/React Router tooling and validating `npm audit`. | Completed |
| SEC-PR-002 | Resolve Go module PRs for `golang.org/x/crypto` and `golang.org/x/net`, refresh vendor, and validate with Go vulnerability checks. | Completed |
| SEC-PR-003 | Resolve GitHub Actions PRs for `actions/checkout` and `actions/upload-artifact`, including the changelog gate failure. | Completed |
| SEC-PR-004 | Reconcile GitHub PR state after landing: verify alerts/checks, then close or merge every superseded PR with explicit comments. | Completed |

Guardrail: keep this sweep limited to dependency, workflow, vulnerability, and required memory-bank/changelog updates. Do not change unrelated Plex/tuner behavior.
