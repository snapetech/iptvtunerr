# Known issues

## Plex / Deployment

- **The old local split-brain Tunerr/Plex fallback is intentionally removed (2026-05-12).** Do not recreate local production jobs that register the same Plex DVR identity as the systemd-owned host. Active supported deployment paths are binary, Docker, systemd/bare-metal, and k3s when k3s is the single owner for its Plex DVR identity.

- **Plex can report a DVR device as `dead` even when enabled channel mappings are healthy.** The watchdog must not recreate a mapped DVR solely because of that flag; recreate only when mappings are missing or badly under-activated.

## Security

- **Credentials:** Secrets must live only in `.env`, environment variables, or host-local service environment files. `.env` is ignored. Never commit `.env` or log secrets.

- **Live TV abuse blocking must not override valid Plex authorization.** A source/IP block can be triggered by missing-token probes from Plex clients or shared networks. The proxy must allow an already-authorized Plex token to bypass the source block while continuing to deny missing or unauthorized tokens.

## Release / Packaging

- **COPR publishing is blocked by expired/invalid credentials (2026-06-16).** `v0.1.85` Release, Docker, AUR, PPA, CI, CodeQL, Gitleaks, and local-identity checks are green, but COPR failed before upload because COPR rejected `COPR_LOGIN`/`COPR_TOKEN` as invalid or expired. A follow-up workflow patch retried with the configured Fedora OTP/GSSAPI fallback; COPR still returned a non-JSON API response for write operations, and browser GSSAPI login returned `401 Unauthorized`. No readable local/OpenBao replacement secret was available. Fix requires rotating `COPR_LOGIN`/`COPR_TOKEN` from the COPR API page or adding valid `COPR_KERBEROS_PRINCIPAL`/`COPR_KERBEROS_KEYTAB_B64` secrets, then rerunning `release-copr.yml` for the affected tag.

- **Winget ZIP manifests must point at the executable inside the archive directory.** The Windows release ZIP contains `iptv-tunerr-vX.Y.Z-windows-amd64/iptv-tunerr.exe`, not a root-level `iptv-tunerr-vX.Y.Z-windows-amd64.exe`. A wrong `NestedInstallerFiles.RelativeFilePath` downloads and hashes fine but fails Microsoft install validation.
