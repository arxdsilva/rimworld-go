package game

import (
	"log"
	"os"

	cligame "rimworld-go/internal/adapters/inbound/cli/game"
	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/display"
	"rimworld-go/internal/application/devscene"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/platform/logfile"
	"rimworld-go/internal/platform/profiling"
)

// Run wires the client from the command line and runs it until the window
// closes. version is the build's (internal/platform/buildinfo). A fatal
// error ends the process with log.Fatal.
func Run(opts cligame.Options, version string, d Display) {
	defer logfile.Start()() // OPT-003: Options › Open log folder shows it
	probe := fpsProbeOf(opts.FPSProbe, opts.CaptureZoom, opts.CaptureSpeed)
	profilePath := opts.CPUProfile
	var probeProfile func() // with -fpsprobe the CPU profile covers only the measured span
	if probe != nil && profilePath != "" {
		path := profilePath
		probe.Measuring = func() { probeProfile = profiling.Start(path, "") }
		profilePath = ""
	}
	stopProfiles := profiling.Start(profilePath, opts.Trace)

	var shot *uiadapter.Capture
	if opts.Capture != "" {
		shot = &uiadapter.Capture{Path: opts.Capture, Ticks: opts.CaptureTicks, Zoom: opts.CaptureZoom, UI: opts.CaptureUI,
			Frames: opts.CaptureFrames, Speed: opts.CaptureSpeed, Timelapse: opts.CaptureTimelapse, UIScale: opts.CaptureUIScale,
			Bare: devscene.Bare(opts.Scenario)} // set pieces: no scripted starter orders
	}
	biomeID, ok := biome.Parse(opts.Biome)
	if !ok {
		log.Fatalf("unknown -biome %q (one of: %s)", opts.Biome, cligame.BiomeKeys())
	}
	defaultSeed := uint64(0) // the wizard rolls a random seed
	if opts.SeedSet || shot != nil {
		defaultSeed = opts.Seed
	}

	gfx := devGraphics{hidpi: opts.HiDPI, vsync: opts.VSync, renderScale: opts.RenderScale, tier: opts.Graphics, windowed: probe != nil}
	devRun := shot != nil || probe != nil || opts.Scenario != "" || gfx.any()
	usage := usageTracker(version, devRun, d)
	scenes := devScenes{devscene.Config{Scenario: opts.Scenario, Width: opts.Width, Height: opts.Height, Hour: opts.Hour, Day: opts.Day, Capturing: shot != nil}}

	cfg := uiadapter.AppConfig{
		DefaultSeed:   defaultSeed,
		DefaultBiome:  biomeID,
		Version:       version,
		Capture:       shot,
		Saves:         saveHooks(opts.SaveDir),
		Settings:      settingsHooks(shot != nil, gfx, d),
		CardFiles:     cardFiles(shot != nil),
		Scenarios:     scenarioStore(shot != nil),
		FPSProbe:      probe,
		NewSimBuilder: newSimBuilder,
		DeviceScale:   d.DeviceScale, // the App never reads the monitor itself
		Window:        d.Window,
		Display:       d.Apply,               // OPT-029
		Cursor:        d.Cursor,              // OPT-028
		Hardware:      hardware(shot != nil), // PLAY-019: graphics tier and the low-memory notice
		NewSim:        scenes.newSim,
	}
	// only a live engine: a nil *Engine in the interface would not be nil
	if snd := openSound(shot != nil || opts.NoAudio || os.Getenv("RIMWORLD_GO_NOAUDIO") == "1"); snd != nil {
		cfg.Sound = snd
	}
	if usage != nil { // a nil *Tracker in the interface would not read as "none"
		cfg.Usage = usage
	}
	if updates := updateChecker(version, devRun); updates != nil {
		cfg.Updates = updates
	}
	app := uiadapter.NewApp(cfg)

	runErr := d.Run(app, display.Window{TPS: clock.TicksPerSecond, Width: uiadapter.DefaultWindowWidth,
		Height: uiadapter.DefaultWindowHeight, Title: "rimworld-go", Probe: probe != nil})
	if usage != nil {
		usage.Close() // before a log.Fatal would skip it
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
	stopProfiles()
	if probeProfile != nil {
		probeProfile()
	}
	profiling.WriteHeap(opts.MemProfile)
	if err := app.CaptureErr(); err != nil {
		log.Fatal(err)
	}
}

// fpsProbeOf is the dev frame-rate probe config (nil when off).
func fpsProbeOf(seconds int, zoom float64, speed int) *uiadapter.FPSProbe {
	if seconds <= 0 {
		return nil
	}
	if speed <= 1 {
		speed = 2 // 3 ticks per frame: the NG-1 budget speed
	}
	return &uiadapter.FPSProbe{Seconds: seconds, Zoom: zoom, Speed: speed}
}
