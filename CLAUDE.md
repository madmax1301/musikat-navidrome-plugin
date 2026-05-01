# CLAUDE.md — `musikat-navidrome-plugin`

**Was:** Navidrome-Plugin (WASM, kompiliert mit TinyGo) das die musikat-FastAPI-API per HTTP triggert. Im Navidrome-UI als nativer Plugin sichtbar mit Settings-Page (musikat-URL, Token, Cron-Hour, Discovery-Knöpfe). Cron-Schedule registriert sich plugin-seitig.

**Hauptzweck:** Discovery-Sync (LB-Top-Artists → Deezer-Artist-Radio → musikat-Queue) auf Schedule, damit der User das musikat-UI nicht mehr direkt aufrufen muss.

## Architektur

- **Plugin-Type:** Lifecycle (`OnInit`) + Scheduler (`OnCallback`).
- **Externe API:** Eine — die musikat-API auf `<musikat_url>/api/plugin/*` (3 Endpoints: health, library/missing, sync-status) plus die etablierten /api/import/csv* Routen.
- **Auth:** Wenn musikat einen `MUSIKAT_API_TOKEN` erzwingt, schickt das Plugin `Authorization: Bearer <token>`.
- **Storage:** KVStore für Run-Historie + zuletzt-gesehenen-Status (für Read-Only-Block in der Settings-Page).
- **Sprache:** Go 1.23+, kompiliert via TinyGo zu `plugin.wasm`. Distribution: `.ndp` (= zip aus manifest.json + plugin.wasm).

## Tech-Stack

- Extism Go-PDK via `github.com/navidrome/navidrome/plugins/pdk/go/{lifecycle,scheduler,host,pdk}`
- TinyGo `>= 0.40` mit Target `wasip1`
- Reines Go-Modul, keine externen Bibliotheken außer der Plugin-PDK

## Build / Test / Install

```bash
# Voraussetzung: TinyGo + Go installiert
make build        # baut plugin.wasm
make package      # baut + zipt zu musikat-navidrome-plugin.ndp
make install-local # kopiert .ndp in $NAVIDROME_PLUGINS_DIR (Default: ~/.navidrome/plugins)
make clean
```

## Entwicklung

- **Struktur:**
  - `main.go` — Plugin-Struct + Hooks (klein halten, Logik in internal/)
  - `internal/sync/runner.go` — Sync-Pipeline (HTTP → musikat → KVStore)
  - `internal/client/musikat.go` — HTTP-Client für die musikat-API (kommt in Sprint 3)
- **Pattern für Capabilities:** Plugin als Struct, dann via `lifecycle.Register(&plugin{})` etc. registrieren — siehe LB-Plugin als Vorlage.
- **HTTP-Calls:** Über `host.HTTPSend(host.HTTPRequest{...})`. KEINE direkten `net/http`-Calls (TinyGo/wasip1 kann das nicht).
- **Logging:** `pdk.Log(pdk.LogInfo, "msg")` — Levels: Trace/Debug/Info/Warn/Error.

## Schwester-Repo

`~/SecondBrain/10-Projects/private-music-server/code/backend/` — die musikat-FastAPI-App, deren API dieses Plugin triggert. Siehe dort `services/discovery.py` und die `/api/plugin/*`-Endpoints.

## Versionierung

Tags `v0.X.Y` triggern GitHub-Actions die `.ndp` als Release-Asset bauen (Sprint 5).
