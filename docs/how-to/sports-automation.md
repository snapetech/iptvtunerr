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
- You can sign in to the authenticated operator WebUI.

Without an API-Sports key, Tunerr does not make schedule requests. The normal lineup, guide, and streams keep working.

## Connect API-Sports

1. Get an API key from your API-Sports account.
2. Open **Sports** in Tunerr's authenticated WebUI and paste the key into **API-Sports connection**.
3. Select **Save API-Sports key**. Tunerr stores it beside the Sports Automation state file with owner-only file permissions and uses it immediately; no environment-file edits or restart are needed.

The saved key is write-only in the WebUI. The status API reports whether it is configured but never returns its value. You can replace or remove it from the same section.

For deployments that manage secrets through environment variables, set `IPTV_TUNERR_API_SPORTS_KEY` in the Tunerr server environment and restart the process. This value takes precedence over the key saved in the WebUI, and the page will explain that the environment owns the key. Tunerr sends the key only in the request header to fixed API-Sports hosts.

The implemented schedule datasets are MLB, NFL, NCAA Football, and NBA. NFL and NCAA Football share the American Football API endpoint, so Tunerr fetches one response per date and partitions it by league ID. NBA uses the separate API-NBA v2 product with the `standard` league and season year; its season year is the starting year (for example, `2026` for the 2026-27 season). API-Basketball is a separate product; its broader leagues and cups are not included yet. Other sports continue to use ordinary provider and XMLTV data and do not cause API-Sports calls. See the [API-NBA documentation](https://api-sports.io/documentation/nba/v2) for the provider's current request and response contract.

Optional state paths:

| Variable | Default | Purpose |
| --- | --- | --- |
| `IPTV_TUNERR_SPORTS_CACHE_FILE` | `<cache-dir>/sports-schedules.json` | Persistent schedule, refresh, and quota cache. |
| `IPTV_TUNERR_SPORTS_AUTOMATION_FILE` | Beside the catalog as `sports-automation.json` | Persisted automation settings and team rules. |
| `IPTV_TUNERR_API_SPORTS_KEY_FILE` | Beside the automation file as `sports-api-key` | Optional path for the key saved through the WebUI. Tunerr writes this file with owner-only permissions. |

Use writable persistent directories for these state files when running in a container. Do not mount the key file into a public web directory or commit it.

## Enable event channels

1. Open **Sports** in the Tunerr WebUI.
2. Enable Sports Automation and select one or more datasets.
3. Optionally choose followed teams. An empty team list includes all teams in the selected datasets.
4. Set how long to keep games after their scheduled start, schedule lookahead, and the maximum Tunerr feeds to publish for each game.
5. Save the settings, then select **Refresh schedules**. Later refreshes run automatically every five minutes while automation is enabled and a key is configured.

Team choices come from the currently cached schedule. If the list is empty, save the selected dataset and refresh first. The API-Sports request cache limits routine refreshes; the status table reports cached dates, stale dates, the last request result, and daily quota for each API product when headers are available. Tunerr tracks each product's daily quota separately, so exhausting NBA requests does not block Baseball or American Football requests. Cached schedules remain usable after upstream errors. See [API-Sports](https://api-sports.io/) for current product plans and request quotas.

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
| `PUT` or `PATCH /v1/sports/automation` | Persist automation settings. `POST` is not supported for this route. |
| `POST /v1/sports/credentials` | Save the API-Sports key without returning it. |
| `DELETE /v1/sports/credentials` | Remove the saved API-Sports key. |
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

- **Key not configured:** enter the key in the Sports page. If the page says the key is managed by the server environment, update `IPTV_TUNERR_API_SPORTS_KEY` in the container or service configuration and restart Tunerr.
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
