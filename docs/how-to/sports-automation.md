---
id: how-to-sports-automation
type: how-to
status: current
tags: [sports, api-sports, webui, epg]
---

# Configure Sports Automation

Tunerr adapts the Sports Automation design in [M3U Web Picker](https://github.com/zschmook/m3u-web-picker#sports-automation): canonical API-Sports fixtures identify scheduled games, Tunerr matches those games to its own merged XMLTV and playable lineup, and publishes matches as temporary event channels. The generated feeds are separate from the regular Tunerr lineup.

## Preconditions

- Tunerr is running with a current live-channel catalog and a merged XMLTV guide.
- The operator WebUI can reach the tuner API.
- For canonical schedules, an API-Sports key is available to the server.

Without an API-Sports key, Tunerr does not make schedule requests. The normal lineup, guide, and streams keep working.

## Configure the server key

Set `IPTV_TUNERR_API_SPORTS_KEY` in Tunerr's private environment and restart the process. Tunerr sends the key only in the request header to fixed API-Sports hosts; the API and WebUI return only whether a key is configured.

The implemented schedule datasets are MLB, NFL, and NCAA Football. NFL and NCAA Football share the American Football API endpoint, so Tunerr fetches one response per date and partitions it by league ID. Other sports continue to use ordinary provider and XMLTV data and do not cause API-Sports calls.

Optional state paths:

| Variable | Default | Purpose |
| --- | --- | --- |
| `IPTV_TUNERR_SPORTS_CACHE_FILE` | `<cache-dir>/sports-schedules.json` | Persistent schedule, refresh, and quota cache. |
| `IPTV_TUNERR_SPORTS_AUTOMATION_FILE` | Beside the catalog as `sports-automation.json` | Persisted automation settings and team rules. |

Both files are written with owner-only permissions. Use a writable persistent directory for each when running in a container.

## Enable event channels

1. Open **Sports** in the Tunerr WebUI.
2. Enable Sports Automation and select one or more datasets.
3. Optionally choose followed teams. An empty team list includes all teams in the selected datasets.
4. Set how long to keep games after their scheduled start, schedule lookahead, and the maximum Tunerr feeds to publish for each game.
5. Save the settings, then select **Refresh schedules**. Later refreshes run automatically every five minutes while automation is enabled and a key is configured.

Team choices come from the currently cached schedule. If the list is empty, save the selected dataset and refresh first. The API-Sports request cache limits routine refreshes; the status panel reports cached dates, stale dates, the last request result, and quota headers when API-Sports returns them. Manual refresh uses fresh requests but will not issue more requests after Tunerr observes that the daily quota is exhausted. Cached schedules remain usable after upstream errors.

## Matching and generated feeds

An event is published only when all these conditions hold:

- It is inside the configured time window and is not cancelled, postponed, suspended, or finished.
- A current Tunerr lineup channel has a stream.
- The channel name or a programme on that channel in Tunerr's merged XMLTV contains both canonical team names (or API team codes when supplied).
- The programme starts within three hours of the canonical game start.

Tunerr ranks qualifying feeds by team-name and schedule proximity, then limits them to the configured per-game maximum. Missing aliases, generic programme titles, or inaccurate guide times can leave a valid game unmatched; the report gives a reason when no feed qualifies. Tunerr does not create a provider stream URL from API-Sports data.

The generated outputs are:

| Output | Route | Use |
| --- | --- | --- |
| Event playlist | `/sports/live.m3u` | Import as a separate playlist source. |
| Event guide | `/sports/guide.xml` | Configure as the XMLTV guide for that separate playlist. |

The playlist contains generated event identities and links back to Tunerr's `/stream/{channel_id}` gateway. The guide contains one event programme per generated channel. Event identities are stable for an event/feed pair, while channel numbers are reusable within dedicated sports ranges. Neither output modifies the regular lineup or saved manual channels.

## Operator API

| Method and route | Purpose |
| --- | --- |
| `GET /v1/sports/automation` | Read settings, supported datasets, cache status, and the current match report. |
| `PATCH /v1/sports/automation` | Persist automation settings. |
| `GET /v1/sports/status` | Compatibility alias for the automation view. |
| `GET /v1/sports/events` | Read matched and unmatched scheduled events. |
| `POST /v1/sports/refresh` | Force a schedule refresh for enabled datasets and the configured time window. |
| `GET /sports/live.m3u` | Read the generated event playlist. |
| `GET /sports/guide.xml` | Read the generated event guide. |

The configuration, status, event, and refresh routes use Tunerr's existing operator-access check (localhost by default; LAN access follows `IPTV_TUNERR_UI_ALLOW_LAN`). The WebUI reaches them through its authenticated session proxy. Feed routes are intended for the IPTV client configured with the generated playlist; protect access through the same private network or ingress policy used for Tunerr streams.

## Verify

In the Sports page, confirm the selected datasets appear in the cache table and the schedule status has no refresh error. A matched game should show one or more Tunerr feed choices and increase **Generated event channels**. Open the M3U and XMLTV links to confirm the event channel and its programme are present.

For an operator API check from the tuner host:

```sh
curl -fsS http://127.0.0.1:5004/v1/sports/automation
curl -fsS http://127.0.0.1:5004/sports/live.m3u
curl -fsS http://127.0.0.1:5004/sports/guide.xml
```

An empty generated feed is expected until a current XMLTV programme matches both teams and the schedule window.

## Troubleshooting

- **Key not configured:** confirm `IPTV_TUNERR_API_SPORTS_KEY` is set in the tuner process environment, then restart Tunerr.
- **No team choices:** enable a dataset, save settings, and refresh schedules.
- **Schedule cached but no feeds:** check the merged XMLTV programme title/subtitle/description, both team names, and its start time. Tunerr requires a close match to avoid publishing unrelated channels.
- **Stale schedule or quota warning:** Tunerr keeps the last cached schedule on upstream failures. Review cache status and wait for API-Sports quota to recover.
- **Playlist opens but playback fails:** check the source channel directly through Tunerr's stream gateway; schedule data identifies events but does not supply a playable stream.

See also
--------
- [Sports Automation feature](../features.md)
- [Environment reference](../reference/cli-and-env-reference.md)
- [EPG doctor](fix-guide-data-with-epg-doctor.md)
- [Docs index](../index.md)

Related ADRs
------------
- None.
