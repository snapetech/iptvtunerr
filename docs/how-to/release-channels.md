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

Public GitHub Actions workflows use GitHub-hosted Ubuntu or Windows runners so
their logs do not expose paths from a private self-hosted runner.

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

## Release version parity

The `vX.Y.Z` Git tag is the version source for every release. After the GitHub
Release and its assets are ready, `.github/workflows/release.yml` dispatches
Docker plus the AUR, PPA, COPR, Chocolatey, Winget, and Snap publishers through
[`scripts/dispatch-release-channels.sh`](../../scripts/dispatch-release-channels.sh)
with that same tag. Each publisher checks that the tag is the newest stable
`vX.Y.Z` reachable from `main` immediately before submission. Publisher runs
are serialized per channel so a newer dispatch cancels an older in-progress
run. AUR, PPA, and COPR are dispatched once by the release workflow; they do
not also subscribe to the GitHub Release event.

The dispatcher starts Docker from the current `main` workflow definition. The
image uses the exact release tag for application source and version, with the
current `main` Dockerfile packaging recipe, then publishes both `latest` and
the matching version tag to GHCR and both configured Docker Hub image names.
The runtime image uses Debian Bookworm package archives for FFmpeg and its
supporting tools. The Docker publisher runs BuildKit with host networking so
its build sandbox can resolve package mirrors through the GitHub runner's DNS;
the hosted builder failed to resolve Debian mirrors on its isolated network.
The `v0.1.87` run built all three target architectures and published matching
`latest` and version tags to GHCR and `snapetech/iptvtunerr`. Docker Hub returned
`insufficient_scope` for the compatibility image `keefshape/iptvtunerr`; the
Docker Hub identity used by Actions needs write permission to that repository
before those tags can be updated.

CI checks the publisher list, latest-tag guard, Docker image names, and tag
contract. The GitHub Release workflow waits for every dispatched publisher and
fails if a workflow fails or times out. Launchpad
publishing waits for the target series' amd64 binary package to reach
`Published`; a successful source upload alone does not count as availability.
Chocolatey moderation, Winget's Microsoft review, and Snap Store review remain
upstream gates, so those packages may take longer to become installable after
their matching submissions.

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
job. Matrix receives the same notes as bounded, HTML-escaped messages; same-day
retry chunks are redacted before replacements are posted. Matrix announcement
failures do not fail the release.

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

`.github/workflows/docker.yml` publishes multi-arch container images when the
release workflow dispatches it or on a manual rerun. It rejects any tag other
than the latest stable release reachable from `main` and publishes `latest`
and the release tag from the same build. It uses the Dockerfile from current
`main` with application source checked out at the exact release tag. Debian
package installation retries transient failures; GitHub Actions BuildKit could
not reach the Alpine package CDN or its official mirrors.

Configured registries:

- GHCR: `ghcr.io/snapetech/iptvtunerr`
- Docker Hub: `snapetech/iptvtunerr` and `keefshape/iptvtunerr` (compatibility image)

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

The AUR, PPA, and COPR publisher workflows completed successfully for
`v0.1.86`; the closeout below records their registry-visible state. The
`v0.1.86` GitHub Release also contains direct `.deb` and `.rpm` assets.

GitHub Actions secrets from another repository cannot be read back out, so
future rotations have to re-enter, regenerate, or source values from a local
secret store.

The GitHub Release also carries direct `.deb` and `.rpm` assets for users who do
not want to use PPA/COPR.

## Windows Channels

Windows release assets are portable ZIP files from GitHub Releases. The
Windows Smoke workflow uses `windows-latest`, and native smoke validation
passed in run
[`36793770645`](https://github.com/snapetech/iptvtunerr/actions/runs/36793770645).

Configured packaging:

- Chocolatey metadata lives in `packaging/chocolatey/`.
- Winget manifest generation lives in `packaging/scripts/update-winget-manifests.sh`.
- `.github/workflows/publish-chocolatey.yml` publishes a Chocolatey package
  from a release tag. It runs on a GitHub-hosted Ubuntu runner, rewrites the
  nuspec/install script for the requested tag, packs the Chocolatey `.nupkg`
  with .NET/NuGet, replaces the packed root nuspec with the exact Chocolatey
  nuspec so Chocolatey-specific metadata is preserved, and pushes it to
  `https://push.chocolatey.org/`. Package preparation and push run as separate
  Bash steps with a unique runner-temp directory and GitHub step timeouts so
  package preparation or publishing cannot hang indefinitely. The workflow
  loads the current-main release guard, so even a manual rerun against an old
  tag is rejected instead of replacing the latest package version.
- `.github/workflows/publish-winget.yml` submits a Winget PR from a release tag.
- The release dispatcher starts both Windows publishers for each new release.
  Their manual inputs remain available for catch-up and retries.

Required GitHub secrets:

- `CHOCO_API_KEY` - Chocolatey API key for the `iptvtunerr` package.
- `WINGETCREATE_GITHUB_TOKEN` - GitHub token that can open PRs against
  `microsoft/winget-pkgs`.
- `SNAPCRAFT_STORE_CREDENTIALS` - restricted Snap Store credentials for the
  `iptvtunerr` snap.

Current status:

- `CHOCO_API_KEY` is configured for the `slskdn` Chocolatey account.
- `WINGETCREATE_GITHUB_TOKEN` is configured.
- Chocolatey `0.1.86` was accepted by the publisher workflow; the package's
  moderation state is checked separately from upload success.
- Winget PR
  [`microsoft/winget-pkgs#444713`](https://github.com/microsoft/winget-pkgs/pull/444713)
  submits version `0.1.86` and is awaiting Microsoft review.
- Chocolatey and Winget publisher runs for `v0.1.86`:
  [Chocolatey](https://github.com/snapetech/iptvtunerr/actions/runs/36795573443),
  [Winget](https://github.com/snapetech/iptvtunerr/actions/runs/36795573436).

## Snap

The registered `iptvtunerr` Snap is built from the release source as a strictly
confined amd64 CLI. Install it with `snap install iptvtunerr`, then run
`snap run iptvtunerr serve` or another Tunerr subcommand. It uses the user's
home directory and network interfaces; connect the `removable-media` plug if
provider files are stored on removable media. The release dispatcher publishes
the same version to the Snap Store's stable channel.

Snap Store review controls when a new stable revision becomes available. The
publisher uses the configured Store credential and records upload failures in
GitHub Actions.

NuGet is not currently a fit for IPTV Tunerr. The project ships a Go CLI/server
binary, not a .NET library or .NET global tool.

See also
--------
- [package-test-builds](package-test-builds.md)
- [release-readiness-matrix](../explanations/release-readiness-matrix.md)
