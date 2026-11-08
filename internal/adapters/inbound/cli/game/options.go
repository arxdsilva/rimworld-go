package game

import (
	"errors"
	"flag"
	"io"
	"strings"

	"rimworld-go/internal/domain/world/biome"
)

// Options are cmd/game's parsed command line. Every flag but -seed, -hidpi
// and -vsync is a developer one (dev:).
type Options struct {
	Seed    uint64
	SeedSet bool // -seed was given (the wizard rolls a random seed otherwise)
	Width   int
	Height  int

	Capture          string
	CaptureTicks     int
	CaptureZoom      float64
	CaptureUI        string
	CaptureUIScale   float64
	CaptureFrames    int
	CaptureSpeed     int
	CaptureTimelapse int

	Scenario string
	Biome    string // a biome key; see BiomeKeys
	Hour     int

	NoAudio    bool
	CPUProfile string
	MemProfile string
	Trace      string
	FPSProbe   int
	// SaveDir replaces the per-user saves folder for this run (probes,
	// benchmarks: their autosaves never touch the player's saves).
	SaveDir string

	// HiDPI and VSync are nil unless their flag was given: only a given
	// flag overrides the Graphics setting.
	HiDPI, VSync *bool
	Graphics     string
	RenderScale  float64
}

// Parse reads args (os.Args: the program name, then the flags). On a bad
// flag it prints the error and the usage to stderr and returns the error;
// -h and -help print the usage and return flag.ErrHelp. ExitCode turns
// either into the exit status.
func Parse(args []string) (Options, error) {
	return parse(args, nil)
}

// parse is Parse writing its messages to out (nil: stderr).
func parse(args []string, out io.Writer) (Options, error) {
	var o Options
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	if out != nil {
		fs.SetOutput(out)
	}
	var hidpi, vsync bool
	fs.Uint64Var(&o.Seed, "seed", 1, "deterministic RNG seed")
	fs.IntVar(&o.Width, "width", 48, "map width in tiles")
	fs.IntVar(&o.Height, "height", 40, "map height in tiles")
	fs.StringVar(&o.Capture, "capture", "", "dev: save one colony frame to this PNG and exit")
	fs.IntVar(&o.CaptureTicks, "capture-ticks", 5400, "dev: simulation ticks to run before -capture")
	fs.Float64Var(&o.CaptureZoom, "capture-zoom", 1, "dev: camera zoom for -capture")
	fs.StringVar(&o.Scenario, "scenario", "", "dev: start scenario: pen (a pen with a feeding trough, tame animals and wild ones about) | rescue (a colonist starts badly hurt) | beds (sleeping spots in the other facings) | double (a double bed by the start) | corridor (two colonists meet head-on in a 1-wide corridor) | crowd (every colonist sent into a room with one door) | armory (armed, drafted colonists against a mixed raid) | chores (cutting, mining and tending by the start) | lategame (a grown colony in autumn rain, raid incoming) | rain (a fall storm) | snow (winter snowfall) | burial (a graveyard, a pyre and two bodies) | roofs (winter: a roofed room, an unroofed one, a roofed porch, a yard too big to roof) | sunny (a clear summer day from before dawn) | fence (a fenced field with a gate, deer outside) | forbid (loose items, some forbidden) | grown (the benchmark grown colony at -width x -height; set RIMWORLD_FIXTURE_DIR to cache it) | zoo (all 61 animal species, tamed and named, in a pen; use -width 80 -height 60) | social (a walled colony: a couple's double bed, a marriage spot, a brewing bench, a prisoner and warden, a bonded husky; needs -width/-height >= 60) | walls (linked walls and rock: every connection case, five stones, smoothed rock, ore) | power (a small powered base: generators, a battery, a switch, lamps, a heater, a hidden run, a lamp off the grid) | bleeding (a colonist bleeding from cuts, a hospital bed and medicine) | surgery (a peg leg install queued, a hospital bed beside a powered vitals monitor) | outbreak (flu sweeps the colony) | turret (two powered sentry turrets, then a raid) | trade (a powered radio console, a trade beacon with goods and silver, a trader caravan and a ship in orbit)")
	fs.StringVar(&o.Biome, "biome", "temperate_forest", "dev: biome of the map: "+BiomeKeys()+" (also the landing picker's starting choice)")
	fs.IntVar(&o.Hour, "hour", -1, "dev: jump to this hour (0-23) of the scenario's day, or the next day's if past")
	fs.IntVar(&o.CaptureFrames, "capture-frames", 0, "dev: record this many consecutive frames (PATH_NNN.png) instead of one still")
	fs.IntVar(&o.CaptureSpeed, "capture-speed", 1, "dev: game speed (1-3) while recording -capture-frames")
	fs.StringVar(&o.CaptureUI, "capture-ui", "", "dev: UI state for -capture: picker | wild | trough | inspector | architect | orders | bills | pawn | combat | melee | follow | grow | corridor | crowd | lategame | weather | grass | burial | roofs | roofs-overlay | home | carpet | plant | item | pawnactions | paused-stockpile | move | copy | menu | credits | options | options-graphics | options-gameplay | options-access | zones | zone | zone-shrink | power | power-overlay | paths | log | frame | fence | forbid | zoo | controls | search | rooms | multiselect | drawstyle | hideui | scenario | scenario-editor | scenario-editor-addpart | storyteller | difficulty-custom | worldgen | worldgen-progress | landing | landing-advanced | landing-factions | pawns | pawns-reroll | factions-colony | bio | work-incapable | arrival | options-story | options-habits | dossier | dossier-float | dossier-log | dossier-animal | dossier-setup | dossier-setup-sections | dossier-setup-presets | dossier-setup-colours | letter-intro | arch-hospital | arch-cabinet | arch-locked | chips | social | health | prisoner | history | world | quests | assign | turret | trade | perf")
	fs.Float64Var(&o.CaptureUIScale, "capture-ui-scale", 0, "dev: with -capture, shoot at this UI scale (1280s x 720s frame) to review scaled layouts; 0 = scale 1")
	fs.IntVar(&o.CaptureTimelapse, "capture-timelapse", 0, "dev: with -capture-frames, step this many sim ticks per recorded frame (a time-lapse)")
	fs.BoolVar(&o.NoAudio, "noaudio", false, "dev: run without sound")
	fs.StringVar(&o.CPUProfile, "cpuprofile", "", "dev: write a CPU profile to this file")
	fs.StringVar(&o.MemProfile, "memprofile", "", "dev: write a heap profile to this file on exit")
	fs.StringVar(&o.Trace, "trace", "", "dev: write an execution trace to this file")
	fs.StringVar(&o.SaveDir, "save-dir", "", "dev: save and autosave into this folder instead of the per-user saves folder")
	fs.IntVar(&o.FPSProbe, "fpsprobe", 0, "dev: run the colony for this many seconds at -capture-speed (2 = 3 ticks/frame) and -capture-zoom, log frame stats and quit")
	fs.BoolVar(&hidpi, "hidpi", true, "dev: render at the screen's true resolution; overrides the Graphics setting for this run without saving it")
	fs.StringVar(&o.Graphics, "graphics", "", "dev: apply this graphics tier (high, medium, low) for this run without saving it, captures and -fpsprobe included")
	fs.BoolVar(&vsync, "vsync", true, "dev: wait for the display's refresh; -vsync=false uncaps the frame rate for -fpsprobe (overrides the Graphics setting for this run without saving it)")
	fs.Float64Var(&o.RenderScale, "render-scale", 0, "dev: draw the map at this share of the physical pixels (1, 0.75 or 0.5; PERF-002) for this run without saving it; 0 = the Graphics setting")
	if err := fs.Parse(args[1:]); err != nil {
		return o, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "hidpi":
			o.HiDPI = &hidpi
		case "vsync":
			o.VSync = &vsync
		case "seed":
			o.SeedSet = true
		}
	})
	return o, nil
}

// ExitCode is the exit status after Parse returned err, as the flag
// package's ExitOnError has it: 0 for -h, 2 for a bad flag.
func ExitCode(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

// BiomeKeys lists the -biome keys, comma separated. The "biomes" capture
// is a script-level loop: for b in temperate_forest boreal_forest ...; do
// go run ./cmd/game -noaudio -biome $b -capture out_$b.png; done
func BiomeKeys() string {
	keys := make([]string, 0, biome.NumBiomes)
	for _, d := range biome.All() {
		keys = append(keys, d.Key)
	}
	return strings.Join(keys, ", ")
}
