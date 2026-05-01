# musikat-navidrome-plugin

A [Navidrome](https://www.navidrome.org/) plugin that triggers the [musikat](https://github.com/your-org/musikat) FastAPI backend on a schedule — so you can do discovery + auto-download from inside the Navidrome UI without ever opening the standalone musikat web interface.

**Status:** v0.x — work in progress. v1 ships sync-only (cron + discovery). Multi-schedule + listen-tracking come later.

## What it does (v1)

- Registers a recurring cron in Navidrome (default `0 7 * * *` — every day 07:00 local time).
- On trigger, calls the musikat backend at `<musikat_url>/api/plugin/library/missing` to get a curated list of "tracks you'd probably like, but don't have yet" (your ListenBrainz top-artists → Deezer artist-radio, filtered against your listening history).
- Pushes that list as a CSV to musikat's `/api/import/csv`, then triggers `/api/import/csv/queue-all/<job>` to bulk-download.
- Reads `/api/plugin/sync-status` and stores the result in the plugin's KVStore — surfaced as a read-only block on the plugin settings page so you can see "last run, queue health, last error" without leaving Navidrome.

## What it does NOT do

- It does **not** run downloads itself — that's musikat's job. The plugin is a thin trigger.
- It does **not** show a live queue in the Navidrome UI — Navidrome plugins can't inject custom UI components, only settings pages. Status is read-only and refreshes when you re-open the settings page.
- v1 has a single schedule. Multi-schedule (separate jobs for sync / playlist / cleanup) is v2.

## Requirements

- Navidrome `>= 0.60` (plugin system, Extism-based).
- A reachable musikat backend on the same LAN (or via DNS / reverse-proxy).
- musikat backend should expose `/api/plugin/health`, `/api/plugin/library/missing`, `/api/plugin/sync-status` (added on the musikat side; if missing, plugin will report errors at runtime).
- For dev/build: [TinyGo](https://tinygo.org/getting-started/install/) `>= 0.40` and [Go](https://golang.org/dl/) `>= 1.23`. `tinygo` falls back to `go` for builds without TinyGo, but the resulting WASM is **much** larger.

## Build

```bash
# Native TinyGo build (recommended; produces small .wasm)
make build

# Package as installable .ndp
make package
# → produces musikat-navidrome-plugin.ndp

# Optional: drop directly into your Navidrome plugins dir
NAVIDROME_PLUGINS_DIR=~/.navidrome/plugins make install-local
```

## Install

### Option 1 — Pre-built release (recommended)

Grab the latest `musikat-navidrome-plugin.ndp` from the [GitHub Releases](https://github.com/maxhartmann/musikat-navidrome-plugin/releases) page. Each release is built by the `release.yml` GitHub Action whenever a `v*.*.*` tag is pushed.

### Option 2 — Build from source

```bash
git clone https://github.com/maxhartmann/musikat-navidrome-plugin.git
cd musikat-navidrome-plugin
make package    # → musikat-navidrome-plugin.ndp
```

### Install into Navidrome

1. Upload `musikat-navidrome-plugin.ndp`.
2. In Navidrome admin → **Plugins** → upload the `.ndp` file.
3. Enable the plugin. It needs these permissions when prompted:
   - **HTTP** to `<your musikat host>` (declared in `manifest.json`; if your host name differs, you have to either rebuild with the right hostname or use `*` — see "Open question" below).
   - **Scheduler** for the cron registration.
   - **KVStore** for status persistence.
4. Open the plugin's settings page and fill in:
   - `musikat_url` — e.g. `http://musikat.lan:8000` (no trailing slash).
   - `musikat_token` — your `MUSIKAT_API_TOKEN` from `backend/.env`. Leave empty if musikat runs without a token.
   - `listenbrainz_user` — your LB username; the discovery pipeline pulls from there.
   - `cron_expression` — `0 7 * * *` is the default (daily 07:00).
   - `top_artists`, `tracks_per_artist`, `max_queue_per_run` — tune if you want a more or less aggressive discovery.

## How it works under the hood

1. On Navidrome startup or plugin install, `OnInit()` reads the config, validates it, and registers the cron via `host.SchedulerScheduleRecurring`.
2. When the cron fires, Navidrome calls `OnCallback()` in the plugin.
3. The callback runs the sync pipeline (`internal/sync.Run()` — coming in Sprint 3): health check → fetch missing list → push CSV → trigger queue-all → store status.
4. The settings page shows the status block by reading the plugin's KVStore on view.

## Open question

The plugin manifest declares `permissions.http.requiredHosts`. Navidrome's host allowlist may require this to be a concrete hostname rather than a wildcard. If your musikat host name differs from the default in `manifest.json`, you currently have two options:

- **Rebuild** the plugin with your hostname patched into `manifest.json` (`make build` after editing).
- **Try the wildcard** (`"*"`) and check whether the current Navidrome version accepts it.

Will be revisited once we test against real Navidrome installs.

## License

MIT — see [LICENSE](./LICENSE).
