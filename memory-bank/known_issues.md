# Known issues

## Plex / Deployment

- **The old local split-brain Tunerr/Plex fallback is intentionally removed (2026-05-12).** Do not recreate local production jobs that register the same Plex DVR identity as the systemd-owned host. Active supported deployment paths are binary, Docker, systemd/bare-metal, and k3s when k3s is the single owner for its Plex DVR identity.

- **Plex can report a DVR device as `dead` even when enabled channel mappings are healthy.** The watchdog must not recreate a mapped DVR solely because of that flag; recreate only when mappings are missing or badly under-activated.

## Security

- **Credentials:** Secrets must live only in `.env`, environment variables, or host-local service environment files. `.env` is ignored. Never commit `.env` or log secrets.

- **Live TV abuse blocking must not override valid Plex authorization.** A source/IP block can be triggered by missing-token probes from Plex clients or shared networks. The proxy must allow an already-authorized Plex token to bypass the source block while continuing to deny missing or unauthorized tokens.

## Release / Packaging

- **Winget ZIP manifests must point at the executable inside the archive directory.** The Windows release ZIP contains `iptv-tunerr-vX.Y.Z-windows-amd64/iptv-tunerr.exe`, not a root-level `iptv-tunerr-vX.Y.Z-windows-amd64.exe`. A wrong `NestedInstallerFiles.RelativeFilePath` downloads and hashes fine but fails Microsoft install validation.

## Repository remotes

- **Local Identity Leak Check matches existing dependency-update co-author trailers.** Five recent commit-message lines match the private identity denylist. The user directed that published history remain intact, so this check cannot be cleared by rewriting those commits; preserve the scanner and disclose the failing workflow status.
- **GitLab mirror ref listing currently fails, and its cached `main` contains an extra donation-links commit.** The configured GitLab SSH remote reports `multi-pack-index signature 0x00000000 does not match signature 0x4d494458` during ref listing; normal pushes from GitHub `main` are rejected as non-fast-forward. Preserve the GitLab-only commit, repair/read the GitLab ref before synchronizing, and never force-push this branch.
