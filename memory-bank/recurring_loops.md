# Recurring loops and hard-to-solve problems

### Loop: Recreating the removed cluster fallback after it caused split-brain DVR churn

**Symptom**
- Agents look for or recreate the removed manifest tree, service-DNS URLs, cluster command workflows, or cluster deploy scripts when Plex registration breaks.
- Plex accumulates empty DVR rows or flips between conflicting tuner URLs.

**Why it's tricky**
- Older repo history had many examples for that path, so search results suggested it even when the active system is bare-metal/systemd.
- Multiple registrars using the same Plex device IDs cause Plex DVR split-brain and guide reload churn.

**What works**
- Do not recreate the removed path. Use binary, Docker, or systemd/bare-metal paths only.
- Keep exactly one owner for each Plex `IPTV_TUNERR_DEVICE_ID`.
- If Plex has empty DVR rows, delete only `0/0` IPTV DVR rows after verifying the live non-empty DVRs.

**Where it's documented**
- `memory-bank/known_issues.md`
- `docs/how-to/deployment.md`

## Loop protocol
- If you attempt the same approach twice and it still fails, stop and collect evidence before trying a new strategy.
- Do not silence failures; add a repro or focused test and fix the root cause.
- Do not revert unrelated user changes.

### Loop: Misreading slow Plex guide fill as slow Tunerr XMLTV serving

**Symptom**
- Plex Live TV rows fill in very slowly after guide reload even though Tunerr `/guide.xml` returns `200` quickly.

**Why it's tricky**
- Plex can fetch and index the XMLTV channelmap, but the later full channel activation PUT can take more than a minute for large lineups. If Tunerr times out early, the DVR can appear under-activated or empty in Plex UI while `/guide.xml` itself is healthy.

**What works**
- Measure Tunerr first: `/guide.xml` status, `X-Iptvtunerr-Guide-State`, byte size, channel/programme counts, and response time.
- Check per-DVR `ChannelMapping` counts, not only summary `<Channel>` counts from `/livetv/dvrs`.
- Allow a longer timeout for full channelmap activation; Plex treats activation as a full replacement, so do not split the mapping into batches.

**Where it's documented**
- `internal/plex/dvr.go`

### Loop: Sports lineup probe can collapse the live Plex DVR lineup

**Symptom**
- Sports Live TV channels disappear or click-to-play spins, while the sports tuner `/lineup.json` returns `[]`.

**Why it's tricky**
- A bounded runtime probe can decide no sports feeds are healthy and publish an empty lineup. Plex then reloads/activates the empty lineup, so the UI still has guide/provider state but no usable tuner rows.

**What works**
- Check the tuner directly before blaming Plex: `curl http://127.0.0.1:5005/lineup.json | jq length`.
- For production recovery, disable `IPTV_TUNERR_LINEUP_PROBE_ENABLED` on the sports service, restart `iptvtunerr-sports.service`, wait for lineup rebuild, then confirm Plex channel activation completes.
- Keep visual/probe cache changes out of emergency recovery unless the provider health issue is already understood.

**Where it's documented**
- `/etc/iptvtunerr/sports.env` on `kspls0`

### Loop: Event-only sports rows need real DVR-sized guide windows

**Symptom**
- Plex can tune an event sports channel, but recording from the guide fails with a vague client-side error such as "undefined".
- The tuner stream URL works, `/lineup.json` includes the event, and `/guide.xml` is `ready`, so the failure looks like Plex rather than Tunerr.

**Why it's tricky**
- Event rows from provider names like `NEXT | ... Sun 17 May 19:00 EDT ...` may not have upstream EPG programme data or a TVGID.
- The generic no-EPG fallback used to publish a week-long placeholder programme named after the channel. That keeps channels visible, but it is a poor DVR scheduling target for one-off sports events.

**What works**
- For live/next sports rows with parseable event times, publish a bounded programme window at the event time instead of the week-long placeholder.
- Use sport-aware default durations for Plex-facing guide metadata: basketball/hockey about 3.5h, soccer/rugby about 2.5h, baseball about 4.5h, and add extra padding for Game 7/finals/playoff wording.
- Treat timezone abbreviations explicitly; do not rely on Go's generic `MST` parser for `EDT`/`NDT` because unknown abbreviations can collapse to UTC.

**Where it's documented**
- `internal/tuner/epg_pipeline.go`
- `internal/tuner/xmltv_test.go`

### Loop: Generic live sports titles can become stale Plex recording markers

**Symptom**
- Plex shows a red recording marker on current guide rows that are not being recorded, often after recording a similarly named sports block days earlier.
- External/shared users may hit a Plex Web "Something went wrong" view when opening the row because Plex follows stale recording/subscription state instead of plain Live TV playback.

**Why it's tricky**
- Some provider XMLTV rows publish generic titles such as `Live: NBA Basketball` with no subtitle/date/episode identity. Plex can collapse those rows into a title-only XMLTV movie GUID such as `tv.plex.xmltv://movie/Live%3A%20NBA%20Basketball`.
- The tuner streams stay healthy, so the failure looks like playback or entitlement until `/media/subscriptions` reveals the stale title-only subscription.

**What works**
- Add deterministic programme identity to recurring event-like XMLTV rows without changing the visible title: date-specific `sub-title`, `date`, and `episode-num system="iptvtunerr"` generated from channel/start/stop/title metadata.
- Cover all generic `Live:` rows plus generic sports titles such as plain `NBA Basketball`, because providers are inconsistent about including the `Live:` prefix.
- Delete the stale Plex subscription keyed to the generic title, reload the Plex DVR guide, and verify `/media/subscriptions` no longer contains the title-only XMLTV GUID.
- 2026-05-28 recurrence: a tester saw Plex Web `s1002 (Network)` on `Live: NBA Basketball` even though the tuner stream and current guide identity were healthy. Plex still had a stale title-only `Live: NBA Basketball` media subscription; deleting only that subscription cleared `/media/subscriptions`.
- If the stale marker is gone but shared-user playback still returns `s1002`, verify the PMS advertised connection state before widening proxy classifiers again. The documented working state is static manual port `443`, relay disabled, and the public media HTTPS custom connection; automatic port drift back to `32400` can make clients miss the entitlement proxy.

**Where it's documented**
- `internal/tuner/xmltv.go`
- `internal/tuner/epg_pipeline.go`
- `internal/tuner/xmltv_test.go`

### Loop: Short recurring shows can save as title-only one-shot rules

**Symptom**
- Plex accepts Record for a short recurring programme such as `etalk`, but the guide row does not show the red scheduled-recording marker.
- `/media/subscriptions` shows a title-only XMLTV movie rule such as `tv.plex.xmltv://movie/etalk`, while `/media/subscriptions/scheduled` has no matching future grab.

**Why it's tricky**
- The proxy save/read flow can be healthy; the failure is Plex matching a metadata-poor programme title instead of a specific guide airing.
- Plex may create a one-shot subscription with `startTimeslot=Any`, so it looks saved but is not bound to the selected airing.

**What works**
- Metadata-poor short recurring rows need the same kind of deterministic XMLTV identity as generic sports rows: date-specific `sub-title`, `date`, and `episode-num system="iptvtunerr"`.
- Preserve rows that already carry episode metadata; do not overwrite upstream `sub-title`, `date`, or `episode-num`.
- Delete stale title-only subscriptions after deploying the fixed guide identity, reload Plex guides, and have the tester recreate the recording.

**Where it's documented**
- `internal/tuner/xmltv.go`
- `internal/tuner/xmltv_test.go`

### Loop: Plex Record Options uses subscription reads before save

**Symptom**
- Shared Plex users can open the guide and Live TV rows, but the Record Options dialog fails with "There was a problem saving your changes. Please try again."
- PMS logs show shared-user `403` responses for `/media/subscriptions` or `/media/subscriptions/scheduled`, sometimes before any tuner stream request reaches Tunerr.

**Why it's tricky**
- These read endpoints do not carry the XMLTV `guid`, `key`, or `uri` parameters that identify a specific Live TV programme, so a classifier that only recognizes XMLTV template/create requests misses them.
- The failure appears in the Plex client as a generic save error even though it is an entitlement failure during Record Options discovery.

**What works**
- Treat read-only `GET /media/subscriptions` and `GET /media/subscriptions/scheduled` as Live TV discovery inside the Live TV proxy.
- Keep mutating subscription requests, including `/media/subscriptions/{id}` rule edits and `POST /media/subscriptions` saves with Plex `hints[guid]`/`hints[ratingKey]` query parameters, scoped to XMLTV-backed bodies or query parameters so ordinary library subscription creation or id-only deletes are not elevated.

**Where it's documented**
- `internal/plexlabelproxy/entitlement.go`
- `internal/plexlabelproxy/proxy_test.go`

### Loop: Plex Web rolling Live TV playback deletes a subscription id

**Symptom**
- Plex Web reports `s1002 (Network)` after a shared user starts a Live TV stream.
- Tuner and PMS evidence can look healthy: the tune request succeeds, the tuner serves bytes, PMS creates a Live TV transcode, and the client pulls HLS segments.
- Proxy access logs then show `DELETE /media/subscriptions/{id}` denied as `403`. Plex Web may send playback evidence as query `X-Plex-Playback-Session-Id`, query `X-Plex-Session-Id`, or the corresponding request headers.

**Why it's tricky**
- The request is a mutating `/media/subscriptions/{id}` path, so broad elevation would risk ordinary library subscription changes.
- The rolling Live TV cleanup request does not carry XMLTV `guid`, `key`, `uri`, or `hints[...]` values; it carries playback-session evidence instead.

**What works**
- Elevate only `DELETE /media/subscriptions/<numeric-id>` when playback/session evidence is present and non-empty: `X-Plex-Playback-Session-Id` or `X-Plex-Session-Id` in either query parameters or headers.
- Keep plain id-only deletes, product-only deletes, nested scheduled paths, and library subscription edits non-elevated.
- Validate with both classifier tests and live proxy logs: the playback/session DELETE should log `live_tv=true`; the no-session neighbor should remain `live_tv=false`.

**Where it's documented**
- `internal/plexlabelproxy/entitlement.go`
- `internal/plexlabelproxy/proxy_test.go`

### Loop: Ingress encoded-slash deny blocks Plex Live TV transcode before the proxy

**Symptom**
- Plex Web reports `s1002 (Network)` immediately after a successful Live TV tune.
- PMS starts a Live TV session/transcode, but the client-facing `/video/:/transcode/universal/*` and `/:/timeline` calls return tiny `404` responses and do not show up in the Plex Live TV proxy journal.

**Why it's tricky**
- The tuner, PMS transcode worker, and proxy tune/subscription paths can all look healthy.
- Plex Web legitimately sends encoded slashes in query values such as `path=%2Flivetv%2Fsessions%2F...`; a generic public-ingress abnormal-URI rule that scans the full URI can block that before the proxy ever sees it.

**What works**
- Compare the same encoded transcode URL through direct PMS, direct Go proxy, and HTTPS ingress. If only HTTPS ingress returns the 9-byte `404`, fix ingress routing before changing tuner or proxy classifiers again.
- Keep the ingress exception narrow to Plex playback paths that need encoded Live TV session values, such as `/video/:/transcode/*`, `/:/timeline*`, and `/playQueues*`.
- After the ingress change, validate that the encoded `/video/:/transcode/universal/decision` request reaches `plex-live-tv-proxy.service` and logs `live_tv=true stream=true`.

**Where it's documented**
- `memory-bank/current_task.md`

### Loop: Proxy hotfix validated from `/tmp` but not active in systemd

**Symptom**
- A proxy classifier fix appears tested and "deployed", but the next tester retry shows no change.
- `journalctl -u plex-live-tv-proxy.service` shows zero expected `plexlabelproxy_access`/`plexlabelproxy_audit` lines for the retry, or the service process start time predates the hotfix.

**Why it's tricky**
- Building a corrected binary under `/tmp` and running synthetic checks can look like a live deploy if the active `/opt/iptvtunerr/iptv-tunerr-proxy` path and systemd process are not checked afterward.
- Caddy may still route correctly to the proxy, so this is not an ingress bypass; it is an inactive binary/process problem.

**What works**
- After every proxy hotfix, verify all three facts: `/opt/iptvtunerr/iptv-tunerr-proxy version` reports the intended build, `ps` shows a fresh `plex-label-proxy` process, and live journal lines from `127.0.0.1:33240` show the expected classification.
- Use the exact failing request shape for live validation, then also validate a neighboring non-elevated shape to prove the classifier stayed narrow.

**Where it's documented**
- `memory-bank/task_history.md`
- `memory-bank/current_task.md`

### Loop: Plex Record Save may need subscription detail reads after create

**Symptom**
- A shared user can click Record and the client appears to accept the action, but the guide row does not show a scheduled/recording marker afterward.
- The save path may be fixed already, so there is no obvious client-side "problem saving your changes" error.

**Why it's tricky**
- Plex can follow a successful Live TV subscription save with read-only detail/scheduled requests such as `/media/subscriptions/{id}` or `/media/subscriptions/scheduled/{id}` that carry only a numeric rule id.
- If those reads use the shared user's token, Plex may fail to render the DVR rule state even though the create request succeeded.

**What works**
- Treat read-only `/media/subscriptions*` paths as Live TV DVR discovery, except `/media/subscriptions/template`, which must still require Live TV/XMLTV evidence so ordinary library subscription templates are not elevated.
- Keep mutating subscription edits scoped to XMLTV evidence, and scan both query strings and form bodies for Plex `hints[...]` fields such as `hints[ratingKey]`.
- When validating a tester retry, check both the client marker and proxy access lines for all `/media/subscriptions*` requests around the click.

**Where it's documented**
- `internal/plexlabelproxy/entitlement.go`
- `internal/plexlabelproxy/proxy_test.go`

### Loop: Plex Web asks for JSON provider metadata, bypassing XML-only entitlement rewrites

**Symptom**
- External/shared users can see guide/provider rows, but clicking Live TV spins and the tuner never receives `/stream/...`.
- Proxy logs show elevated `/media/providers`, while PMS/Tunerr logs show no tune or active stream.

**Why it's tricky**
- The proxy originally rewrote `allowTuners` only in XML. Plex Web can negotiate JSON for `/media/providers`, so the request is elevated but the client-side entitlement hint can still say tuners are not allowed.

**What works**
- Rewrite `allowTuners` in both XML and JSON bodies, keeping the rewrite narrow to entitlement hints only.
- Verify with `Accept: application/json` against the proxy and confirm `allowTuners` is true.

**Where it's documented**
- `internal/plexlabelproxy/entitlement.go`
- `internal/plexlabelproxy/proxy.go`

### Loop: Self-hosted COPR runner reuses stale SRPMs

**Symptom**
- A COPR workflow rerun for a newer tag succeeds, but COPR receives an older `iptvtunerr-X.Y.Z-1.src.rpm`.
- GitHub Actions reports a green upload because `copr-cli build` accepted an SRPM, but the selected file was not built for the requested tag.

**Why it's tricky**
- Self-hosted runners can retain `~/rpmbuild/SRPMS` between jobs.
- A pattern like `find ~/rpmbuild/SRPMS -name '*.src.rpm' -print -quit` silently chooses the first leftover file, which may be from a prior release.

**What works**
- Build each release SRPM under an isolated temporary RPM topdir.
- Compute and test the exact expected path, `iptvtunerr-${version}-1.src.rpm`.
- Copy that file to a relative workspace path such as `dist/iptvtunerr-${version}-1.src.rpm` and pass that explicit path to `copr-cli build`.
- Verify the COPR upload log contains the requested release version, then watch the external COPR build to success.

**Where it's documented**
- `.github/workflows/release-copr.yml`
- `memory-bank/task_history.md`
