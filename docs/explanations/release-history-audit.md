---
id: release-history-audit
type: explanation
status: stable
tags: [release, changelog, history]
---

# Release history audit

When the changelog coverage gate was added, these published GitHub releases had
Git tags but no matching section in `docs/CHANGELOG.md`. Each now has a dated,
link-only section that points to its original GitHub Release. The original
release bodies, tags, and existing changelog sections remain unchanged.

| Version | Published |
| --- | --- |
| `v0.1.7` | 2026-03-18 |
| `v0.1.11` | 2026-03-19 |
| `v0.1.13` | 2026-03-19 |
| `v0.1.19` | 2026-03-20 |
| `v0.1.25` | 2026-03-21 |
| `v0.1.30` | 2026-03-22 |
| `v0.1.31` | 2026-03-22 |
| `v0.1.32` | 2026-03-22 |
| `v0.1.33` | 2026-03-22 |
| `v0.1.34` | 2026-03-22 |
| `v0.1.35` | 2026-03-23 |
| `v0.1.36` | 2026-03-23 |
| `v0.1.38` | 2026-03-23 |
| `v0.1.39` | 2026-03-23 |
| `v0.1.47` | 2026-04-18 |
| `v0.1.48` | 2026-04-18 |
| `v0.1.57` | 2026-05-08 |

The tags `v0.1.37` and `v0.1.49` exist without a published GitHub Release as
of 2026-09-30. Their reasons are recorded in
[`scripts/changelog-tag-exceptions.json`](../../scripts/changelog-tag-exceptions.json).
The coverage checker requires every exception to name an existing tag, include
a reason, and remain absent from the changelog. If a GitHub Release is later
published for either tag, add a changelog section from its release notes and
remove the exception.

## See also

- [Release notes and changelog workflow](../how-to/release-channels.md#release-notes-and-changelog).
- [Changelog](../CHANGELOG.md).
