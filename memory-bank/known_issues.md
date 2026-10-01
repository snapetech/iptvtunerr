# Known issues

## Plex / Deployment

- **The old local split-brain Tunerr/Plex fallback is intentionally removed (2026-05-12).** Do not recreate local production jobs that register the same Plex DVR identity as the systemd-owned host. Active supported deployment paths are binary, Docker, systemd/bare-metal, and k3s when k3s is the single owner for its Plex DVR identity.

- **Plex can report a DVR device as `dead` even when enabled channel mappings are healthy.** The watchdog must not recreate a mapped DVR solely because of that flag; recreate only when mappings are missing or badly under-activated.

## Security

- **Credentials:** Secrets must live only in `.env`, environment variables, or host-local service environment files. `.env` is ignored. Never commit `.env` or log secrets.

- **Live TV abuse blocking must not override valid Plex authorization.** A source/IP block can be triggered by missing-token probes from Plex clients or shared networks. The proxy must allow an already-authorized Plex token to bypass the source block while continuing to deny missing or unauthorized tokens.

## Sports Automation

- **Event matching depends on provider and guide naming.** API-Sports does not provide channel carriage or stream URLs. Tunerr matches canonical team names/codes in current lineup channel names and merged XMLTV programme text, with programme starts limited to three hours from the canonical start. Provider-specific abbreviations and generic guide labels can leave a real game unmatched. Only current lineup channels with streams can be generated.
- **Canonical schedule coverage is limited to MLB, NFL, and NCAA Football.** NFL and NCAA Football share one API-Sports product endpoint and are partitioned by league ID. Other competitions continue to rely on provider/EPG behavior without canonical schedule matching.

## Release / Packaging

- **Winget ZIP manifests must point at the executable inside the archive directory.** The Windows release ZIP contains `iptv-tunerr-vX.Y.Z-windows-amd64/iptv-tunerr.exe`, not a root-level `iptv-tunerr-vX.Y.Z-windows-amd64.exe`. A wrong `NestedInstallerFiles.RelativeFilePath` downloads and hashes fine but fails Microsoft install validation.
- **Docker Hub compatibility image is stale; follow-up deferred.** For `v0.1.87`, the configured Docker Hub identity published `snapetech/iptvtunerr` but received `insufficient_scope` when pushing `keefshape/iptvtunerr`; GHCR and the primary Docker Hub tags are current. Per the user's latest direction, do not rerun Docker Hub publishing or seek additional access for now.
- **Snap stable is behind; follow-up deferred.** The public stable channel is `0.1.78`; the `v0.1.87` publisher run failed authentication. Per the user's latest direction, do not rerun Snap publishing or seek replacement credentials for now.
- **Launchpad binary publication is asynchronous.** For `v0.1.88`, the exact Jammy and Noble source packages are `Published`; both amd64 builds report `Successfully built` on the latest check, though Noble's state varied between processing states across earlier API reads. Neither exact amd64 binary is published yet. The first poll stopped on HTTP 502; later polling exposed delayed source visibility and a 30-second timeout that exhausted an equally short retry budget. The publisher now allows 20 minutes for source discovery and 180 seconds of curl retries per request. Report the channel as updated only after both exact amd64 binaries reach `Published`.

## Repository remotes

- **Pre-release Local Identity Leak Check matched existing dependency-update co-author trailers.** Five commit-message lines matched the private identity denylist before `v0.1.86` was tagged. Post-release main checks pass because the latest-tag scan baseline advanced; the historical lines remain in their original commits. Preserve history as directed and do not weaken the scanner.
- **GitLab mirror ref listing currently fails, and its cached `main` contains an extra donation-links commit.** The GitLab SSH service reports a multi-pack-index signature error during ref listing and rejected a normal push of verified merge candidate `4b5d293` with an index-pack error. The candidate retains the GitLab-only commit; no remote ref changed. Repair the server-side repository index, read current refs, and retry only as a normal fast-forward. Never force-push this branch.
