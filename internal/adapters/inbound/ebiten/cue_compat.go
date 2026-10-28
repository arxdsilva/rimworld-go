package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/application/game"
)

// The renderer's sound cues live in render/cue (ARCH-004 wave 4.3). These
// aliases and one-line wrappers keep the root's call sites unchanged; the
// last mover deletes this file (SOLID_GO.md §11). The App's sound director
// (sound_director.go) stays here: it decides what is heard from the cues
// the bus hands it.

type (
	soundCue    = cue.Cue
	soundMemo   = cue.Memo
	animalVoice = cue.Voice
	loopWant    = cue.Loop
)

const (
	voiceCall     = cue.VoiceCall
	voiceAttack   = cue.VoiceAttack
	voiceHurt     = cue.VoiceHurt
	voiceDie      = cue.VoiceDie
	voiceBurst    = cue.VoiceBurst
	volCall       = cue.VolCall
	volAttack     = cue.VolAttack
	volHurt       = cue.VolHurt
	volDie        = cue.VolDie
	volBurst      = cue.VolBurst
	volMeleeUnder = cue.VolMeleeUnder

	firstCallMin   = cue.FirstCallMin
	firstCallSpan  = cue.FirstCallSpan
	callEveryMin   = cue.CallEveryMin
	callEverySpan  = cue.CallEverySpan
	capCallsWindow = cue.CapCallsWindow
	capCalls       = cue.CapCalls
	deferMin       = cue.DeferMin
	deferSpan      = cue.DeferSpan
	capSpeciesSpan = cue.CapSpeciesSpan
	capSpecies     = cue.CapSpecies
	largeSize      = cue.LargeSize

	podFallLead     = cue.PodFallLead
	maxMachineLoops = cue.MaxMachineLoops
	cueMaxPerID     = cue.MaxPerID
	panWidth        = cue.PanWidth
	zoomNear        = cue.ZoomNear
	waterRiverGain  = cue.WaterRiverGain
	waterShoreGain  = cue.WaterShoreGain
)

var (
	actionSounds     = cue.ActionSounds
	voiceGroups      = cue.VoiceGroups
	sndWoodGenerator = cue.WoodGeneratorKind
)

func zoomEar(zoom, zoomOut float64) (gain, panScale float64, perID int) {
	return cue.ZoomEar(zoom, zoomOut)
}

func thunderDelay(bucket uint64) uint64 { return cue.ThunderDelay(bucket) }

func callInterval(an game.AnimalView, n uint64, first bool) uint64 {
	return cue.CallInterval(an, n, first)
}

func voiceGroup(key string) string { return cue.VoiceGroup(key) }

// spatial places a world point (tiles) for the ear (cue.Spatial).
func spatial(r *Renderer, x, y float64, v viewport) (vol, pan float64, ok bool) {
	return cue.Spatial(r.cueView(), x, y, v)
}

// cueBus is the renderer's sound cues, with the renderer's lookups wired
// in on first use.
func (r *Renderer) cueBus() *cue.Bus {
	if r.sound == nil {
		r.sound = cue.New()
	}
	if r.sound.Host == nil {
		r.sound.Host = cueHost{r}
	}
	return r.sound
}

// cueView is the frame view the cue layer reads, with the renderer's
// camera, presentation frame, tick and map size copied in: the listeners
// run in advance, before Draw fills the view.
func (r *Renderer) cueView() *render.View {
	v := &r.view
	v.Present = r.present
	if !r.lod.baking {
		v.Cam = r.cam
	}
	if r.game != nil {
		v.Tick, v.MapW, v.MapH = r.game.Tick(), r.game.Width(), r.game.Height()
	}
	return v
}

// cueHost is what the cue layer reads of the renderer (cue.Host).
type cueHost struct{ r *Renderer }

func (h cueHost) Terrain(w game.WorldView, x, y int) (uint8, bool) { return terrainCode(w, x, y) }

func (h cueHost) IsWater(code uint8) bool { return isWaterCode[code] }

func (h cueHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.plantsOn(plants, x0, y0, x1, y1, buf)
}

func (h cueHost) Chopping(p game.PawnView) bool {
	act, _, ok := actionOf(p)
	return ok && act == game.ActionChop
}

func (h cueHost) AnimalAt(an game.AnimalView) (float64, float64) { return h.r.interpAnimal(an) }

func (h cueHost) ListenCaskets(w game.WorldView) { h.r.listenCaskets(w) }

func (r *Renderer) cue(id string, x, y, vol float64) { r.cueBus().Cue(id, x, y, vol) }

func (r *Renderer) globalCue(id string) { r.cueBus().Global(id) }

// takeCues hands this frame's cues to the director.
func (r *Renderer) takeCues() []soundCue { return r.cueBus().Take() }

func (r *Renderer) listenWorld(w game.WorldView, pawns []game.PawnView) {
	r.cueBus().ListenWorld(r.cueView(), w, pawns)
}

func (r *Renderer) listenAnimals(w game.WorldView) {
	r.cueBus().ListenAnimals(r.cueView(), w, r.mo.Stepped)
}

func (r *Renderer) listenThunder() { r.cueBus().ListenThunder(r.cueView()) }

func (r *Renderer) listenArrivals(w game.WorldView) { r.cueBus().ListenArrivals(r.cueView(), w) }

func (r *Renderer) traceSound(t game.TraceView) { r.cueBus().TraceSound(t) }

func (r *Renderer) audible(x, y float64) bool { return r.cueBus().Audible(r.cueView(), x, y) }

func (r *Renderer) animalSoundID(key, event string) string {
	return r.cueBus().AnimalSoundID(key, event)
}

func (r *Renderer) cueAnimal(key, event string, x, y, vol float64) bool {
	return r.cueBus().CueAnimal(key, event, x, y, vol)
}

func (r *Renderer) machineAmbience(want map[string]loopWant, w game.WorldView, v viewport) {
	r.cueBus().Machines(r.cueView(), want, w, v)
}

func (r *Renderer) waterOnScreen(w game.WorldView, v viewport) (water, shore float64) {
	return r.cueBus().WaterOnScreen(r.cueView(), w, v)
}
