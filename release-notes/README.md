# Release-note fragments

Add one Markdown fragment for every user-facing pull request. The release
process uses these fragments to prepare `docs/CHANGELOG.md`, the GitHub Release
body, and the Discord announcement.

The preview, validation, and local pre-commit tools require Node.js 22. CI sets
up this version automatically.

```md
---
category: fixed
audience: users, operators
area: plex-live-tv
action: none
breaking: false
---
Plex DVR recording saves now keep working for shared users when the client sends the playback session identifier in a request header.
```

The frontmatter records:

- `category`: `added`, `changed`, `fixed`, `security`, `removed`, or `deprecated`.
- `audience`: `users`, `operators`, or `users, operators`.
- `area`: a 2-32 character lowercase slug describing the product area.
- `action`: the upgrade or operating step required (5-200 characters), or
  `none` when no action is needed.
- `breaking`: `true` or `false`; breaking changes must describe the required action.

Keep the body between 30 and 400 characters. Write the user impact in a
capitalized sentence ending with punctuation; leave implementation details in
the commit history. Preview a pull request's release text with:

```bash
node scripts/preview-release-notes.mjs --base origin/main --head HEAD
```

Fragments are append-only after their version is tagged. Before creating the
next tag, preview all notes since the previous release and prepare the checked-in
changelog section:

```bash
previous_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*')"
node scripts/preview-release-notes.mjs --base "$previous_tag" --head HEAD
node scripts/prepare-release-changelog.mjs --version vX.Y.Z
node scripts/check-changelog-tags.mjs
git diff -- docs/CHANGELOG.md
```

Commit the generated changelog section to `main` before creating the matching
release tag. If committing it directly, include `release-note: none` in the
commit message because the user-facing changes were already captured by their
fragments. The tag workflow checks that the section still matches the
fragments. Never rewrite or move a published tag or its changelog section.

For internal-only pull requests, check the internal-only option in the pull
request template or add `release-note: none` to the description. Do not use the
opt-out for user-visible behavior, security, operations, or documentation
changes.
