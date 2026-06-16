# Work breakdown

## 2026-06-16 - PR and security sweep

Objective: resolve the full open Dependabot PR queue and current repository security alerts with one verified consolidated update.

| Story ID | Scope | Status |
| --- | --- | --- |
| SEC-PR-001 | Resolve web Dependabot/security alerts for `esbuild` and `react-router` by updating Vite/React Router tooling and validating `npm audit`. | Completed |
| SEC-PR-002 | Resolve Go module PRs for `golang.org/x/crypto` and `golang.org/x/net`, refresh vendor, and validate with Go vulnerability checks. | Completed |
| SEC-PR-003 | Resolve GitHub Actions PRs for `actions/checkout` and `actions/upload-artifact`, including the changelog gate failure. | Completed |
| SEC-PR-004 | Reconcile GitHub PR state after landing: verify alerts/checks, then close or merge every superseded PR with explicit comments. | Pending |

Guardrail: keep this sweep limited to dependency, workflow, vulnerability, and required memory-bank/changelog updates. Do not change unrelated Plex/tuner behavior.
