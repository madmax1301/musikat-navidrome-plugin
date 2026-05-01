// musikat-navidrome-plugin: a Navidrome plugin that triggers the musikat
// FastAPI backend on a schedule. v1 is sync-only — one cron registers in
// OnInit, OnCallback runs the discovery pipeline (sync.Run).
//
// Plugin pattern: struct with methods, registered per-capability via
// `lifecycle.Register` and `scheduler.Register`. The Extism-PDK builds the
// exported WASM functions from those registrations.
package main

import (
	"fmt"
	"strconv"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/lifecycle"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scheduler"

	"musikat-navidrome-plugin/internal/sync"
)

// scheduleName is the cron-job key used both at registration and inside
// OnCallback to dispatch. Keep it stable — Navidrome stores it together with
// the cron-expression and won't double-register the same name.
const scheduleName = "musikat-sync"

// Defaults match the manifest.json `default` values; used when a config key
// is set but empty, so we don't need to duplicate the JSON-Schema defaults.
const (
	defaultCron            = "0 7 * * *"
	defaultTopArtists      = 10
	defaultTracksPerArtist = 5
	defaultMaxQueuePerRun  = 30
)

type plugin struct{}

// getInt reads an integer config value with a fallback. pdk.GetConfig returns
// strings, so we parse manually. Negative or zero values fall back too —
// the manifest validates upper bounds, but a user can still wipe a field.
func getInt(key string, fallback int) int {
	raw, ok := pdk.GetConfig(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

// OnInit reads the user config, validates the bare minimum (URL + LB user)
// and registers the recurring cron. We deliberately do NOT do a network
// reachability check here — Navidrome startup must not block on a flaky
// musikat backend. The first cron-trigger does the health-check.
func (p *plugin) OnInit() error {
	musikatURL, _ := pdk.GetConfig("musikat_url")
	lbUser, _ := pdk.GetConfig("listenbrainz_user")
	cronExpr, ok := pdk.GetConfig("cron_expression")
	if !ok || cronExpr == "" {
		cronExpr = defaultCron
	}

	if musikatURL == "" {
		return fmt.Errorf("musikat_url is required (set it on the plugin settings page)")
	}
	if lbUser == "" {
		return fmt.Errorf("listenbrainz_user is required (set it on the plugin settings page)")
	}

	if _, err := host.SchedulerScheduleRecurring(cronExpr, scheduleName, scheduleName); err != nil {
		return fmt.Errorf("failed to schedule '%s' (%s): %w", scheduleName, cronExpr, err)
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf(
		"musikat-sync initialized: url=%s, lb_user=%s, cron=%q",
		musikatURL, lbUser, cronExpr,
	))
	return nil
}

// OnCallback fires on every cron-trigger. It assembles a Config from the
// current plugin settings (so config-changes take effect immediately
// without a restart) and runs the sync pipeline.
//
// Errors from sync.Run are logged but NOT returned to the host — returning
// an error would mark the schedule as failed in Navidrome's plugin log.
// We prefer the user reads the detailed status from the KVStore-backed
// settings-page block.
func (p *plugin) OnCallback(req scheduler.SchedulerCallbackRequest) error {
	if req.ScheduleID != scheduleName && req.Payload != scheduleName {
		pdk.Log(pdk.LogWarn, "OnCallback got unexpected schedule: id="+req.ScheduleID+" payload="+req.Payload)
		return nil
	}

	url, _ := pdk.GetConfig("musikat_url")
	token, _ := pdk.GetConfig("musikat_token")
	lbUser, _ := pdk.GetConfig("listenbrainz_user")

	cfg := sync.Config{
		URL:             url,
		Token:           token,
		LBUser:          lbUser,
		TopArtists:      getInt("top_artists", defaultTopArtists),
		TracksPerArtist: getInt("tracks_per_artist", defaultTracksPerArtist),
		MaxQueuePerRun:  getInt("max_queue_per_run", defaultMaxQueuePerRun),
	}

	pdk.Log(pdk.LogInfo, "musikat-sync triggered — running pipeline")
	if err := sync.Run(cfg); err != nil {
		pdk.Log(pdk.LogError, "musikat-sync failed: "+err.Error())
		// Don't return the error — KVStore already has the details for the UI.
	}
	return nil
}

func main() {}

func init() {
	p := &plugin{}
	lifecycle.Register(p)
	scheduler.Register(p)
}
