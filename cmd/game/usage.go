package main

import (
	"log"
	"os"
	"runtime"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/outbound/settingsfile"
	"rimworld-go/internal/adapters/outbound/usagehttp"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/sysinfo"
)

// usageEndpointEnv points a build at another server and enables dev builds,
// for testing usage data locally.
const usageEndpointEnv = "RIMWORLD_USAGE_ENDPOINT"

// settingsStore is the part of the settings file the install ID needs.
type settingsStore interface {
	Load() (config.Settings, error)
	Save(config.Settings) error
}

// usageEndpoint is where this build sends usage data, or "" for nowhere.
// Developer runs stay out of the numbers: dev builds, captures, the frame-rate
// probe, dev scenarios and graphics overrides.
func usageEndpoint(buildVersion, override string, devRun bool) string {
	if override != "" {
		return override
	}
	if devRun || buildVersion == "dev" {
		return ""
	}
	return usagehttp.Endpoint
}

// usageTracker builds the tracker, or returns nil when this run sends
// nothing. It runs before the App loads the settings, so the App sees the
// install ID and later saves keep it.
func usageTracker(devRun bool) *usagehttp.Tracker {
	endpoint := usageEndpoint(version, os.Getenv(usageEndpointEnv), devRun)
	if endpoint == "" {
		return nil
	}
	store, err := settingsfile.Default()
	if err != nil {
		return nil
	}
	settings, ok := ensureInstallID(store)
	if !ok {
		return nil
	}
	w, h := ebiten.Monitor().Size()
	return usagehttp.New(usagehttp.Config{
		Endpoint:  endpoint,
		InstallID: settings.Telemetry.InstallID,
		Version:   version,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Advanced:  settings.Telemetry.Advanced,
		RAMBytes:  sysinfo.Read().Total,
		ScreenW:   w,
		ScreenH:   h,
	})
}

// ensureInstallID loads the settings, making and saving the random install ID
// on the first run. An unreadable or unwritable file means no usage data this
// run: the App reports the file itself, and a new ID on every launch would
// count one player many times.
func ensureInstallID(store settingsStore) (config.Settings, bool) {
	settings, err := store.Load()
	if err != nil {
		return settings, false
	}
	if usagehttp.ValidID(settings.Telemetry.InstallID) {
		return settings, true
	}
	settings.Telemetry.InstallID = usagehttp.NewID()
	if err := store.Save(settings); err != nil {
		log.Printf("usage data: off this run: %v", err)
		return settings, false
	}
	return settings, true
}
