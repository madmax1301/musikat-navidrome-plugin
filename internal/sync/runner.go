// Package sync runs one cycle of the musikat-discovery pipeline.
//
// In the per-track-loop variant of v1 the plugin called /api/download once
// per item — but the upstream /api/plugin/library/missing already takes
// ~22 s on real LB-histories and Navidrome kills plugin-callbacks after
// ~30 s. So we moved the heavy lifting into the backend:
//
//   plugin → POST /api/plugin/sync (returns 202 immediately)
//   backend runs discovery + queue in a BackgroundTask
//   plugin reads /api/plugin/sync-status next time the user opens the
//     settings page (no live polling — Navidrome plugins can't render
//     live UI anyway).
//
// The runner therefore just fires the trigger and writes a "we asked"
// status into KVStore. Real progress is tracked server-side and surfaced
// via /api/plugin/sync-status.
package sync

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"musikat-navidrome-plugin/internal/client"
)

// KVKeyStatus is read by the settings-page status-block (Sprint 4).
const KVKeyStatus = "musikat_status"

const defaultLocation = "navidrome"

// Config is the runtime input to a sync run. The cron-callback assembles
// it from the plugin's user-config (see main.go OnCallback).
type Config struct {
	URL             string
	Token           string
	LBUser          string
	TopArtists      int
	TracksPerArtist int
	MaxQueuePerRun  int
}

// Status mirrors what we write into KVStore. The settings-page reads this
// on view and displays the relevant fields read-only.
type Status struct {
	LastTriggerMs    int64  `json:"last_trigger_ms"`
	LastTriggerStatus string `json:"last_trigger_status"` // "ok" | "error"
	LastTriggerError  string `json:"last_trigger_error,omitempty"`
	LastTriggerInfo   string `json:"last_trigger_info,omitempty"`
}

func writeStatus(s Status) {
	blob, err := json.Marshal(s)
	if err != nil {
		pdk.Log(pdk.LogWarn, "marshal status: "+err.Error())
		return
	}
	if err := host.KVStoreSet(KVKeyStatus, blob); err != nil {
		pdk.Log(pdk.LogWarn, "KVStoreSet("+KVKeyStatus+"): "+err.Error())
	}
}

// Run does one trigger cycle: health-check + POST /api/plugin/sync. Total
// wall-clock cost is two HTTP roundtrips (~200 ms each) — well below the
// plugin-callback timeout. The actual discovery + queue runs server-side.
func Run(cfg Config) error {
	st := Status{LastTriggerMs: time.Now().UnixMilli()}

	cl := client.New(cfg.URL, cfg.Token)

	// 1. Quick health-check so we surface obvious config errors (URL typo,
	//    backend down, missing token) right at the trigger time, not 5
	//    minutes later when the user wonders why nothing happened.
	h, err := cl.Health()
	if err != nil {
		st.LastTriggerStatus = "error"
		st.LastTriggerError = "health: " + err.Error()
		writeStatus(st)
		return fmt.Errorf("musikat health: %w", err)
	}
	if h.AuthRequired && cfg.Token == "" {
		st.LastTriggerStatus = "error"
		st.LastTriggerError = "musikat verlangt einen Bearer-Token, Plugin hat keinen konfiguriert"
		writeStatus(st)
		return fmt.Errorf("backend requires token but plugin has none")
	}

	// 2. Fire the trigger.
	pdk.Log(pdk.LogInfo, fmt.Sprintf(
		"musikat-sync: triggering backend (lb=%s top=%d perArtist=%d max=%d)",
		cfg.LBUser, cfg.TopArtists, cfg.TracksPerArtist, cfg.MaxQueuePerRun,
	))
	resp, err := cl.TriggerSync(client.TriggerSyncRequest{
		ListenBrainzUser: cfg.LBUser,
		TopArtists:       cfg.TopArtists,
		TracksPerArtist:  cfg.TracksPerArtist,
		MaxTotal:         cfg.MaxQueuePerRun,
		Location:         defaultLocation,
	})
	if err != nil {
		st.LastTriggerStatus = "error"
		st.LastTriggerError = "trigger: " + err.Error()
		writeStatus(st)
		return fmt.Errorf("trigger sync: %w", err)
	}

	st.LastTriggerStatus = "ok"
	st.LastTriggerInfo = fmt.Sprintf("backend ack: started=%v %s", resp.Started, resp.Message)
	writeStatus(st)
	pdk.Log(pdk.LogInfo, "musikat-sync: backend accepted trigger; pipeline running server-side")
	return nil
}
