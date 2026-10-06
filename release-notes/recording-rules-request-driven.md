---
category: added
audience: operators
area: recordings
action: To record only requested programmes, run catchup-daemon with -rules-only and IPTV_TUNERR_RECORDING_RULES_FILE.
breaking: false
---
Recording rules can now target one airing exactly with title_equals and a start window, and catchup-daemon -rules-only records only what rules request. The rules API reports these features so SeerrNG recording requests can rely on them.
