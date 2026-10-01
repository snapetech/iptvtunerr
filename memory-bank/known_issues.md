# Known issues

## Plex / Deployment

- **The old local split-brain Tunerr/Plex fallback is intentionally removed (2026-05-12).** Do not recreate local production jobs that register the same Plex DVR identity as the systemd-owned host. Active supported deployment paths are binary, Docker, systemd/bare-metal, and k3s when k3s is the single owner for its Plex DVR identity.

- **Plex can report a DVR device as `dead` even when enabled channel mappings are healthy.** The watchdog must not recreate a mapped DVR solely because of that flag; recreate only when mappings are missing or badly under-activated.

## Security

- **Credentials:** Secrets must live only in `.env`, environment variables, or host-local service environment files. `.env` is ignored. Never commit `.env` or log secrets.

- **Live TV abuse blocking must not override valid Plex authorization.** A source/IP block can be triggered by missing-token probes from Plex clients or shared networks. The proxy must allow an already-authorized Plex token to bypass the source block while continuing to deny missing or unauthorized tokens.

## Release / Packaging

- **Winget ZIP manifests must point at the executable inside the archive directory.** The Windows release ZIP contains `iptv-tunerr-vX.Y.Z-windows-amd64/iptv-tunerr.exe`, not a root-level `iptv-tunerr-vX.Y.Z-windows-amd64.exe`. A wrong `NestedInstallerFiles.RelativeFilePath` downloads and hashes fine but fails Microsoft install validation.
- **Docker Hub compatibility image needs separate repository write access.** For `v0.1.87`, the configured Docker Hub identity published `snapetech/iptvtunerr` but received `insufficient_scope` when pushing `keefshape/iptvtunerr`; GHCR and the primary Docker Hub tags are current while the compatibility tag is stale. Grant the configured identity write access to the compatibility repository, then rerun the Docker workflow and compare both tags' digests.
- **Snap stable is behind because the Store credential is rejected.** The public stable channel is `0.1.78`; the `v0.1.87` publisher run failed authentication and no local Snapcraft login is available. Refresh the repository's Store credential through the owner account and rerun the Snap publisher.
- **Launchpad binary publication is asynchronous.** The `v0.1.87` Noble and Jammy amd64 source builds succeeded, while both exact source publications remain `Pending` and `getPublishedBinaries` returns no binaries. Wait for archive publication before describing the PPA as updated.
- **Winget package availability waits for Microsoft review.** PR [#444749](https://github.com/microsoft/winget-pkgs/pull/444749) submits `0.1.87` and remains open; the workflow's successful submission does not mean the package is merged or available through WinGet.

## Repository remotes

- **Pre-release Local Identity Leak Check matched existing dependency-update co-author trailers.** Five commit-message lines matched the private identity denylist before `v0.1.86` was tagged. Post-release main checks pass because the latest-tag scan baseline advanced; the historical lines remain in their original commits. Preserve history as directed and do not weaken the scanner.
- **GitLab mirror ref listing currently fails, and its cached `main` contains an extra donation-links commit.** The GitLab SSH service reports a multi-pack-index signature error during ref listing and rejected a normal push of verified merge candidate `4b5d293` with an index-pack error. The candidate retains the GitLab-only commit; no remote ref changed. Repair the server-side repository index, read current refs, and retry only as a normal fast-forward. Never force-push this branch.
