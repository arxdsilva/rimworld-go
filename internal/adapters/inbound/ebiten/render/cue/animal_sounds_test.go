package cue

import (
	"math"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// stubHost is a Host over hand-built values: animals are drawn on their
// tile, nothing else exists.
type stubHost struct{}

func (stubHost) Terrain(game.WorldView, int, int) (uint8, bool) { return 0, false }
func (stubHost) IsWater(uint8) bool                             { return false }
func (stubHost) PlantsOn([]game.PlantView, int, int, int, int, []int32) []int32 {
	return nil
}
func (stubHost) Chopping(game.PawnView) bool { return false }
func (stubHost) AnimalAt(an game.AnimalView) (float64, float64) {
	return float64(an.X), float64(an.Y)
}
func (stubHost) ListenCaskets(game.WorldView) {}

// rig is a bus looking at the map's top-left corner at the given zoom, on
// a 1280x720 screen, with the animal voice's viewport set.
type rig struct {
	b *Bus
	v *render.View
}

func newBus(zoom float64) rig {
	b := New()
	b.Host = stubHost{}
	b.Voice.View = camera.ColonyViewport(1280, 720)
	return rig{b, &render.View{Cam: camera.Camera{Zoom: zoom}, MapW: 64, MapH: 64}}
}

func wanderer(id uint64, species string, x, y int) game.AnimalView {
	return game.AnimalView{ID: entity.EntityID(id), Species: species, X: x, Y: y, Mode: "wandering"}
}

// heardCue is a cue with the frame it came out on.
type heardCue struct {
	frame int
	Cue
}

// listen runs ListenAnimals over frames frames of the same world and
// returns every cue, with its frame.
func listen(r rig, w game.WorldView, frames int) []heardCue {
	var out []heardCue
	for f := 0; f < frames; f++ {
		r.b.ListenAnimals(r.v, w, 1)
		for _, c := range r.b.Take() {
			out = append(out, heardCue{f, c})
		}
	}
	return out
}

func TestFallbackChain(t *testing.T) {
	t.Parallel()
	have := map[string]bool{}
	r := newBus(1)
	r.b.SoundHas = func(id string) bool { return have[id] }
	if got := r.b.AnimalSoundID("tapir", VoiceCall); got != "" {
		t.Fatalf("nothing exists, got %q", got)
	}
	have["animal.generic.call"] = true
	have["animal.canid.hurt"] = true
	have["animal.red_fox.die"] = true
	for _, tc := range []struct{ key, event, want string }{
		{"red_fox", "call", "animal.generic.call"}, // only the generic voice
		{"red_fox", "hurt", "animal.canid.hurt"},   // the group voice
		{"red_fox", "die", "animal.red_fox.die"},   // its own voice
		{"mystery_beast", "call", "animal.generic.call"},
		{"red_fox", "attack", ""},
	} {
		if got := r.b.AnimalSoundID(tc.key, tc.event); got != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.key, tc.event, got, tc.want)
		}
	}
	// resolved once and cached
	have["animal.red_fox.call"] = true
	if got := r.b.AnimalSoundID("red_fox", "call"); got != "animal.generic.call" {
		t.Fatalf("the answer should be cached, got %q", got)
	}
	// no soundHas: every sound exists, the species' own is chosen
	r2 := newBus(1)
	if got := r2.b.AnimalSoundID("red_fox", "call"); got != "animal.red_fox.call" {
		t.Fatalf("nil soundHas = %q", got)
	}
	// a missing sound is silent: no cue, no panic
	r.b.CueAnimal("red_fox", "attack", 1, 1, 1)
	if len(r.b.Take()) != 0 {
		t.Fatal("a missing sound still cued")
	}
}

func TestEveryRosterVoiceHasAGroup(t *testing.T) {
	t.Parallel()
	groups := map[string]bool{"small_mammal": true, "large_mammal": true, "canid": true, "felid": true,
		"bovid": true, "bird_small": true, "bird_large": true, "reptile": true, "insect": true}
	if len(VoiceGroups) != 65 {
		t.Fatalf("%d voice groups listed, want 65 species", len(VoiceGroups))
	}
	for k, g := range VoiceGroups {
		if !groups[g] {
			t.Errorf("%s: unknown group %q", k, g)
		}
	}
	if VoiceGroup("nobody") != "generic" {
		t.Fatal("an unlisted species should fall to generic")
	}
}

func TestCallIntervalScales(t *testing.T) {
	t.Parallel()
	base := CallInterval(game.AnimalView{ID: 5, Species: "deer"}, 2, false)
	if base < CallEveryMin || base >= CallEveryMin+CallEverySpan {
		t.Fatalf("interval %d outside %d..%d", base, CallEveryMin, CallEveryMin+CallEverySpan)
	}
	approx := func(got uint64, scale float64) bool {
		return math.Abs(float64(got)-float64(base)*scale) <= 2
	}
	for _, tc := range []struct {
		an    game.AnimalView
		scale float64
	}{
		{game.AnimalView{ID: 5, Species: "cat"}, 0.7},
		{game.AnimalView{ID: 5, Species: "chicken"}, 0.7},
		{game.AnimalView{ID: 5, Species: "ostrich"}, 0.7},
		{game.AnimalView{ID: 5, Species: "deer", Size: LargeSize}, 1.3},
		{game.AnimalView{ID: 5, Species: "hive_beetle"}, 0.5},
	} {
		if got := CallInterval(tc.an, 2, false); !approx(got, tc.scale) {
			t.Errorf("%s size %v: %d, want about %d x %v", tc.an.Species, tc.an.Size, got, base, tc.scale)
		}
	}
	first := CallInterval(game.AnimalView{ID: 5, Species: "deer"}, 0, true)
	if first < FirstCallMin || first >= FirstCallMin+FirstCallSpan {
		t.Fatalf("first call %d outside %d..%d", first, FirstCallMin, FirstCallMin+FirstCallSpan)
	}
	if CallInterval(game.AnimalView{ID: 6, Species: "deer"}, 0, true) == first {
		t.Fatal("two animals share a first-call time")
	}
}

func TestCallRateCaps(t *testing.T) {
	t.Parallel()
	r := newBus(0.5)
	species := []string{"deer", "cow", "horse", "goat", "sheep", "cat", "husky", "chicken", "duck", "pig", "elk", "lynx"}
	var herd []game.AnimalView
	for i := 0; i < 40; i++ {
		an := wanderer(uint64(100+i), species[i%len(species)], 2+i%8, 3+i/8)
		herd = append(herd, an)
		if !r.b.Audible(r.v, float64(an.X)+0.5, float64(an.Y)+0.5) {
			t.Fatalf("test setup: animal %d must be within earshot", i)
		}
	}
	const frames = 3600
	cues := listen(r, game.WorldView{Animals: herd}, frames)
	if len(cues) < 3 {
		t.Fatalf("a 40-animal herd made only %d calls in %d frames", len(cues), frames)
	}
	for _, c := range cues {
		if !strings.HasSuffix(c.ID, ".call") {
			t.Fatalf("unexpected cue %q", c.ID)
		}
	}
	for start := 0; start < frames; start++ {
		calls, seen := 0, map[string]bool{}
		for _, c := range cues {
			if c.frame >= start && c.frame < start+CapCallsWindow {
				calls++
			}
			if c.frame >= start && c.frame < start+CapSpeciesSpan {
				seen[c.ID] = true
			}
		}
		if calls > CapCalls {
			t.Fatalf("%d calls in the %d frames from %d", calls, CapCallsWindow, start)
		}
		if len(seen) > CapSpecies {
			t.Fatalf("%d species called in the %d frames from %d: %v", len(seen), CapSpeciesSpan, start, seen)
		}
	}
}

func TestVoicePrunedWithAnimals(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	fleer := wanderer(3, "deer", 7, 7)
	fleer.Mode = "fleeing"
	three := game.WorldView{Animals: []game.AnimalView{wanderer(1, "deer", 5, 5), wanderer(2, "cow", 6, 6), fleer}}
	listen(r, three, 3)
	v := &r.b.Voice
	if len(v.NextCall) != 2 || len(v.Last) != 3 || len(v.ByID) != 3 {
		t.Fatalf("tracking %d timers (the fleeing deer has none yet), %d seen, %d indexed; want 2/3/3", len(v.NextCall), len(v.Last), len(v.ByID))
	}
	listen(r, game.WorldView{Animals: []game.AnimalView{wanderer(1, "deer", 5, 5)}}, 2)
	if len(v.NextCall) != 1 || len(v.Last) != 1 || len(v.ByID) != 1 || len(v.Fleeing) != 1 || len(v.Calls) > 1 {
		t.Fatalf("after two left: nextCall %d last %d byID %d fleeing %d calls %d, want 1 each (calls at most 1)",
			len(v.NextCall), len(v.Last), len(v.ByID), len(v.Fleeing), len(v.Calls))
	}
	if _, ok := v.NextCall[1]; !ok {
		t.Fatal("the animal that stayed lost its timer")
	}
	listen(r, game.WorldView{}, 1)
	if len(v.NextCall)+len(v.Last)+len(v.ByID)+len(v.Fleeing)+len(v.Calls) != 0 {
		t.Fatal("an empty world left voice state behind")
	}
}

func TestListenAnimalsWithoutViewportCallsAnywhere(t *testing.T) {
	t.Parallel()
	r := newBus(1) // no viewport set: the director culls
	r.b.Voice.View = camera.Viewport{}
	w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 60, 60)}}
	if len(listen(r, w, FirstCallMin+FirstCallSpan+1)) == 0 {
		t.Fatal("with no viewport the deer should still call (the director culls)")
	}
}
func TestZoomEar(t *testing.T) {
	t.Parallel()
	const out = 0.3
	cases := []struct {
		zoom, gain, pan float64
		perID           int
	}{
		{3.0, 1, 1, 3}, {1.5, 1, 1, 3}, {1.0, 0.748, 0.874, 3},
		{0.5, 0.317, 0.658, 1}, {0.3, 0, 0.5, 1}, {0.2, 0, 0.5, 1},
	}
	for _, c := range cases {
		g, p, n := ZoomEar(c.zoom, out)
		if math.Abs(g-c.gain) > 0.01 || math.Abs(p-c.pan) > 0.01 || n != c.perID {
			t.Fatalf("ZoomEar(%v) = %.3f %.3f %d, want %.3f %.3f %d", c.zoom, g, p, n, c.gain, c.pan, c.perID)
		}
	}
	prev := 2.0
	for z := 3.0; z >= out; z *= 0.95 {
		g, _, _ := ZoomEar(z, out)
		if g > prev {
			t.Fatalf("gain rises zooming out at %v", z)
		}
		prev = g
	}
	if g, p, n := ZoomEar(0.5, 1.5); g != 1 || p != 1 || n != MaxPerID {
		t.Fatal("a map that fits at zoomNear never fades")
	}
}
