---
id: howto-release-channels
type: how-to
status: draft
tags: [release, packaging, aur]
---

# Release channels

IPTV Tunerr release automation publishes GitHub release assets, package-channel
updates, and container images from release tags only. A release tag must be a
`v*` tag that points at the current `main` commit; release workflows reject tags
that point anywhere else.

The primary release workflow builds and uploads:

- raw executable assets for Linux, macOS, and Windows;
- `.tar.gz` bundles for Linux and macOS;
- `.zip` bundles for Windows package managers;
- direct `.deb` and `.rpm` Linux package assets;
- `SHA256SUMS.txt`;
- `release-manifest.json`;
- populated release notes generated from the versioned `docs/CHANGELOG.md`
  section and tagged commit range.

CI runs the same release asset builder and checksum verifier with a dummy
version so asset naming, archive layout, and checksum coverage stay tested
before tags are cut.

## Release notes and changelog

Each user-facing pull request adds one validated fragment under
[`release-notes/`](../../release-notes/). Fragments describe the impact for
users or operators and record their area, required action, and breaking-change
status. Internal-only pull requests select the explicit no-note option in the
pull request template. CI rejects missing, malformed, or already-shipped
fragments. The local tools require Node.js 22; CI installs it automatically.

Preview a pull request's note with:

```bash
node scripts/preview-release-notes.mjs --base origin/main --head HEAD
```

Before tagging a release, preview all notes since the previous tag and generate
the versioned changelog section from those fragments:

```bash
previous_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*')"
node scripts/preview-release-notes.mjs --base "$previous_tag" --head HEAD
node scripts/prepare-release-changelog.mjs --version vX.Y.Z
node scripts/check-changelog-tags.mjs
git diff -- docs/CHANGELOG.md
```

Commit the prepared changelog section to `main` before creating the matching
tag. For a direct preparation commit, add `release-note: none` to its commit
message because the release notes are already in the fragments. The release
workflow verifies that the tag's changelog section matches the fragments and
still contains one section for every published release. Previous sections and
tags are append-only.

The historical coverage audit and the two existing tags without published
GitHub Releases are documented in the
[release history audit](../explanations/release-history-audit.md). When adding a
no-release exception, first verify that the tag has no published GitHub Release
and record the reason in `scripts/changelog-tag-exceptions.json`. Remove the
exception when a changelog section is added.

The GitHub Release body contains the curated notes, technical commit list,
checksums, and install notes. Discord receives that same body as bounded embeds;
the workflow checks Discord's response for every chunk so no later notes are
silently dropped. A missing webhook or rejected message fails the announcement
job.

Install local hooks with:

```bash
./scripts/install-git-hooks.sh
```

The local pre-commit hook validates staged note fragments. CI enforces the
fragment-or-opt-out choice for pull requests and direct pushes.

## AUR

The repo includes AUR packaging for:

- `iptvtunerr` - builds from the tagged GitHub source archive.
- `iptvtunerr-bin` - installs the Linux `amd64` release asset.

The `.github/workflows/release-aur.yml` workflow runs on published GitHub
releases and can also be started manually with a tag. It waits for the
`iptv-tunerr-<tag>-linux-amd64.tar.gz` asset, updates `pkgver`, generates
`.SRCINFO`, and pushes both AUR repositories.

Required GitHub secret:

- `AUR_SSH_KEY` - private key for an SSH public key registered on the AUR
  account that maintains `iptvtunerr` and `iptvtunerr-bin`.

Current status:

- `AUR_SSH_KEY` is configured for `snapetech/iptvtunerr`.
- `iptvtunerr` and `iptvtunerr-bin` have been created on AUR.

## Manual AUR validation

From the repo root:

```bash
bash packaging/scripts/validate-aur-pkgbuild-hashes.sh packaging/aur/PKGBUILD packaging/aur/PKGBUILD-bin
bash packaging/scripts/generate-aur-srcinfo.sh packaging/aur/PKGBUILD
bash packaging/scripts/generate-aur-srcinfo.sh packaging/aur/PKGBUILD-bin
```

On an Arch host, run `makepkg` inside `packaging/aur/` after copying either
`PKGBUILD` or `PKGBUILD-bin` to `PKGBUILD`.

## Containers

`.github/workflows/docker.yml` publishes multi-arch container images on `v*`
tags only, after checking that the tag points at current `main`.

Configured registries:

- GHCR: `ghcr.io/snapetech/iptvtunerr`
- Docker Hub: `keefshape/iptvtunerr`

Credential status:

- GHCR: `GHCR_TOKEN` is configured, with `GITHUB_TOKEN` fallback.
- Docker Hub: `DOCKERHUB_TOKEN` is configured and repo variable
  `DOCKERHUB_USERNAME=keefshape` is set.

## Linux Package Channels

Launchpad/PPA and COPR have channel-specific metadata and workflows
for the Go binary:

- `.github/workflows/release-ppa.yml`
- `.github/workflows/release-copr.yml`
- `packaging/debian/`
- `packaging/rpm/iptvtunerr.spec`

Credential status:

- Launchpad/PPA: `GPG_PRIVATE_KEY`, `LAUNCHPAD_SFTP_KEY`, and
  `LAUNCHPAD_SFTP_USER` are configured for `snapetech/iptvtunerr`. The PPA is
  `ppa:keefshape/iptvtunerr`; Launchpad account `keefshape` has display name
  `slskdn`.
- COPR: `COPR_LOGIN` and `COPR_TOKEN` are configured for
  `snapetech/iptvtunerr`.

GitHub Actions secrets from another repository cannot be read back out, so
future rotations have to re-enter, regenerate, or source values from a local
secret store.

The GitHub Release also carries direct `.deb` and `.rpm` assets for users who do
not want to use PPA/COPR.

## Windows Channels

Windows release assets are portable ZIP files from GitHub Releases. Current
Windows status: the binary cross-builds and package prep passes; native Windows
host validation is still recommended before making broad Windows parity claims.

Configured packaging:

- Chocolatey metadata lives in `packaging/chocolatey/`.
- Winget manifest generation lives in `packaging/scripts/update-winget-manifests.sh`.
- `.github/workflows/publish-chocolatey.yml` publishes a Chocolatey package
  from a release tag. It runs on the repo Linux runner pool, rewrites the
  nuspec/install script for the requested tag, packs the Chocolatey `.nupkg`
  with .NET/NuGet, replaces the packed root nuspec with the exact Chocolatey
  nuspec so Chocolatey-specific metadata is preserved, and pushes it to
  `https://push.chocolatey.org/`. Package preparation and push run as separate
  Bash steps with a unique runner-temp directory plus GitHub step timeouts so
  self-hosted runner state cannot hang the publisher indefinitely. The workflow
  performs its tag-on-main check inline because older tags may not contain the
  current helper script used by other release workflows.
- `.github/workflows/publish-winget.yml` submits a Winget PR from a release tag.

These Windows package workflows are manual-only until their external gates are
clean. The main release workflow intentionally does not auto-dispatch
Chocolatey or Winget so release tags do not spam Chocolatey push attempts or
duplicate Winget PRs while validation/account issues are pending.

Required GitHub secrets:

- `CHOCO_API_KEY` - Chocolatey API key for the `iptvtunerr` package.
- `WINGETCREATE_GITHUB_TOKEN` - GitHub token that can open PRs against
  `microsoft/winget-pkgs`.

Current status:

- `CHOCO_API_KEY` is configured for the `slskdn` Chocolatey account.
- `WINGETCREATE_GITHUB_TOKEN` is configured.
- Chocolatey package `0.1.68` passed automated validation, verification, and
  scanning, then entered `Waiting for Maintainer` moderation on 2026-07-06.
  The reviewer requested the automated review guideline/suggestion metadata,
  plus a pass over `authors` and `owners`. The publish workflow now rewrites the
  Chocolatey nuspec at pack time so existing release tags can be republished
  with owner, icon, package source, project source, docs, bug tracker, and
  release-notes metadata without moving old release tags. The `v0.1.85`
  republish attempt built and downloaded assets successfully, but Chocolatey
  returned `403 Forbidden` because `0.1.68` is still the only package version and
  is waiting for maintainer action. Resubmit `v0.1.68` with the corrected
  metadata first, or add a Chocolatey review comment asking the moderator to
  reject `0.1.68` so the current release tag can be pushed.
- Winget PR `microsoft/winget-pkgs#374269` cleared the Microsoft CLA after the
  GitHub agreement comment, but currently has Microsoft validation labels
  `Internal-Error`, `Needs-Attention`, and `Validation-Guide`. Duplicate
  automated PRs for later release tags were closed.

NuGet is not currently a fit for IPTV Tunerr. The project ships a Go CLI/server
binary, not a .NET library or .NET global tool.

See also
--------
- [package-test-builds](package-test-builds.md)
- [release-readiness-matrix](../explanations/release-readiness-matrix.md)
