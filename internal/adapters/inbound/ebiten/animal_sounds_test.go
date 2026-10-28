package ebiten

import (
	"math"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// voiceRenderer is a renderer looking at the map's top-left corner at the
// given zoom, on a 1280x720 screen, with the animal voice's viewport set.
func voiceRenderer(zoom float64) *Renderer {
	r := New(&stubSim{w: 64, h: 64})
	r.cam.Zoom = zoom
	r.voice.view = colonyViewport(1280, 720)
	return r
}

func wanderer(id uint64, species string, x, y int) game.AnimalView {
	return game.AnimalView{ID: entity.EntityID(id), Species: species, X: x, Y: y, Mode: "wandering"}
}

// heardCue is a cue with the frame it came out on.
type heardCue struct {
	frame int
	soundCue
}

// listen runs listenAnimals over frames frames of the same world and
// returns every cue, with its frame.
func listen(r *Renderer, w game.WorldView, frames int) []heardCue {
	var out []heardCue
	for f := 0; f < frames; f++ {
		r.mo.stepped = r.speed.ticksPerFrame() // what advance stepped this frame
		r.listenAnimals(w)
		for _, c := range r.takeCues() {
			out = append(out, heardCue{f, c})
		}
	}
	return out
}

func idsOf(cs []heardCue) []string {
	var ids []string
	for _, c := range cs {
		ids = append(ids, c.id)
	}
	return ids
}

func TestCallsWhileWanderingOnScreen(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
	if !r.audible(5.5, 5.5) {
		t.Fatal("test setup: the deer must be within earshot")
	}
	cues := listen(r, w, firstCallMin+firstCallSpan+1)
	if len(cues) == 0 {
		t.Fatal("a wandering deer on screen never called within its first-call window")
	}
	c := cues[0]
	if c.id != "animal.deer.call" || c.vol != volCall || c.x != 5.5 || c.y != 5.5 || c.global {
		t.Fatalf("first call = %+v, want animal.deer.call at 5.5,5.5 volume %v", c.soundCue, volCall)
	}
	if c.frame < firstCallMin {
		t.Fatalf("first call at frame %d, earlier than %d", c.frame, firstCallMin)
	}
	// and then it calls again, on a longer beat
	more := listen(r, w, callEveryMin+callEverySpan+deferMin+deferSpan+1)
	if len(more) == 0 {
		t.Fatal("the deer never called a second time")
	}
}

func TestNoCallsWhilePaused(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	r.speed = speedPaused
	w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
	if cues := listen(r, w, 2*(firstCallMin+firstCallSpan)); len(cues) != 0 {
		t.Fatalf("paused game called: %v", idsOf(cues))
	}
	// unpaused, the timer starts from where it was: nothing is owed
	r.speed = speedNormal
	if cues := listen(r, w, firstCallMin-1); len(cues) != 0 {
		t.Fatalf("a call came before its time after unpausing: %v", idsOf(cues))
	}
}

func TestCallTimerFollowsGameSpeed(t *testing.T) {
	t.Parallel()
	first := func(s speed) int {
		r := voiceRenderer(1)
		r.speed = s
		w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
		cues := listen(r, w, firstCallMin+firstCallSpan+1)
		if len(cues) == 0 {
			t.Fatalf("%s: no call", s.label())
		}
		return cues[0].frame
	}
	if n, f := first(speedNormal), first(speedFast); f*2 >= n {
		t.Fatalf("fast speed first call at frame %d, normal at %d: the timer should run about 3x faster", f, n)
	}
}

func TestNoCallsOffscreen(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	far := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 60, 60)}}
	if r.audible(60.5, 60.5) {
		t.Fatal("test setup: tile 60,60 must be out of earshot")
	}
	if cues := listen(r, far, 2*(firstCallMin+firstCallSpan)); len(cues) != 0 {
		t.Fatalf("an off-screen deer called: %v", idsOf(cues))
	}
	// it walks into view: the missed calls are not made up
	near := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
	if cues := listen(r, near, 5); len(cues) != 0 {
		t.Fatalf("catch-up call as the deer came on screen: %v", idsOf(cues))
	}
	if r.voice.nextCall[7] <= r.voice.clock {
		t.Fatal("the skipped call was not rescheduled")
	}
}

func TestFleeCriesOnce(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	an := wanderer(7, "deer", 5, 5)
	listen(r, game.WorldView{Animals: []game.AnimalView{an}}, 3)
	an.Mode = "fleeing"
	cues := listen(r, game.WorldView{Animals: []game.AnimalView{an}}, 120)
	if len(cues) != 1 || cues[0].id != "animal.deer.call" || cues[0].frame != 0 {
		t.Fatalf("a fleeing deer should cry once, right away; got %v", cues)
	}
	if r.voice.nextCall[7] <= r.voice.clock {
		t.Fatal("the cry did not reset the call timer")
	}
	// a new episode cries again
	an.Mode = "wandering"
	listen(r, game.WorldView{Animals: []game.AnimalView{an}}, 3)
	an.Mode = "fleeing"
	if cues := listen(r, game.WorldView{Animals: []game.AnimalView{an}}, 10); len(cues) != 1 {
		t.Fatalf("a second flight should cry once, got %v", idsOf(cues))
	}
}

func TestFleeingAnimalThatAppearsFleeingIsSilent(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	an := wanderer(7, "deer", 5, 5)
	an.Mode = "fleeing"
	if cues := listen(r, game.WorldView{Animals: []game.AnimalView{an}}, 30); len(cues) != 0 {
		t.Fatalf("a deer first seen already fleeing cried: %v", idsOf(cues))
	}
}

// primed is a renderer that has seen the animals once, so traces can look
// them up.
func primed(animals ...game.AnimalView) *Renderer {
	r := voiceRenderer(1)
	r.listenAnimals(game.WorldView{Animals: animals})
	r.takeCues()
	return r
}

func TestAttackCueAtAttacker(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "timber_wolf", 10, 10))
	r.traceSound(game.TraceView{Attacker: 9, Target: 3, FromX: 10, FromY: 10, ToX: 11, ToY: 10, Hit: true, Melee: true})
	cues := r.takeCues()
	if len(cues) != 2 {
		t.Fatalf("a wolf bite should cue the snarl and the impact, got %+v", cues)
	}
	if a := cues[0]; a.id != "animal.timber_wolf.attack" || a.vol != volAttack || a.x != 10.5 || a.y != 10.5 {
		t.Fatalf("attack cue = %+v, want animal.timber_wolf.attack at the wolf, volume %v", a, volAttack)
	}
	if m := cues[1]; m.id != "sfx.melee_hit" || m.vol != volMeleeUnder || m.x != 11.5 {
		t.Fatalf("impact = %+v, want sfx.melee_hit at the target, volume %v under the voice", m, volMeleeUnder)
	}
	// a pawn's blow keeps the full-volume impact and no voice
	r.traceSound(game.TraceView{Attacker: 3, Target: 4, FromX: 1, FromY: 1, ToX: 2, ToY: 1, Hit: true, Melee: true})
	cues = r.takeCues()
	if len(cues) != 1 || cues[0].id != "sfx.melee_hit" || cues[0].vol != 1 {
		t.Fatalf("pawn blow = %+v, want one full-volume sfx.melee_hit", cues)
	}
}

func TestHurtCueOnHit(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "deer", 10, 10))
	r.traceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 9, FromY: 10, ToX: 10, ToY: 10, Hit: true, Melee: true})
	cues := r.takeCues()
	if len(cues) != 2 || cues[0].id != "animal.deer.hurt" || cues[0].vol != volHurt || cues[0].x != 10.5 {
		t.Fatalf("a hit deer should yelp at the deer; got %+v", cues)
	}
	if cues[1].id != "sfx.melee_hit" || cues[1].vol != volMeleeUnder {
		t.Fatalf("impact = %+v, want sfx.melee_hit at volume %v", cues[1], volMeleeUnder)
	}
	// a shot is heard from the muzzle and the deer still yelps
	r.traceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 2, FromY: 2, ToX: 10, ToY: 10, Hit: true})
	ids := idsOf(heardOf(r.takeCues()))
	if len(ids) != 2 || ids[0] != "animal.deer.hurt" || ids[1] != "sfx.gunshot" {
		t.Fatalf("a shot deer: got %v", ids)
	}
}

func heardOf(cs []soundCue) []heardCue {
	out := make([]heardCue, len(cs))
	for i, c := range cs {
		out[i] = heardCue{soundCue: c}
	}
	return out
}

func TestNoHurtOnMiss(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "deer", 10, 10))
	r.traceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 9, FromY: 10, ToX: 10, ToY: 10, Melee: true})
	if cues := r.takeCues(); len(cues) != 0 {
		t.Fatalf("a missed blow on a deer made sound: %+v", cues)
	}
	// a blow on a building the animal is gnawing is the wood's sound, not a yelp
	r.traceSound(game.TraceView{Attacker: 3, Target: 9, Hit: true, Building: true, Melee: true, ToX: 4, ToY: 4})
	if ids := idsOf(heardOf(r.takeCues())); len(ids) != 1 || ids[0] != "sfx.wood_hit" {
		t.Fatalf("building hit: got %v", ids)
	}
}

func TestDeathCueWithCorpse(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "deer", 10, 10), wanderer(8, "deer", 3, 3))
	// 9 dies and leaves a corpse on the next tile; 8 is still here
	w := game.WorldView{
		Animals: []game.AnimalView{wanderer(8, "deer", 3, 3)},
		Items:   []game.ItemView{{ID: 50, Kind: "corpse", Animal: "deer", X: 10, Y: 11}},
	}
	r.listenAnimals(w)
	cues := r.takeCues()
	if len(cues) != 1 || cues[0].id != "animal.deer.die" || cues[0].vol != volDie || cues[0].x != 10.5 || cues[0].y != 10.5 {
		t.Fatalf("death cue = %+v, want animal.deer.die at 10.5,10.5 volume %v", cues, volDie)
	}
	// once only
	r.listenAnimals(w)
	if cues := r.takeCues(); len(cues) != 0 {
		t.Fatalf("the death sounded again: %+v", cues)
	}
}

func TestNoDeathCueWithoutCorpse(t *testing.T) {
	t.Parallel()
	cases := map[string][]game.ItemView{
		"released or left (no corpse)":   nil,
		"a corpse of another species":    {{Animal: "cow", X: 10, Y: 10}},
		"a corpse far from where it was": {{Animal: "deer", X: 14, Y: 10}},
		"a corpse being carried":         {{Animal: "deer", X: 10, Y: 10, Carried: true}},
	}
	for name, items := range cases {
		r := primed(wanderer(9, "deer", 10, 10))
		r.listenAnimals(game.WorldView{Items: items})
		if cues := r.takeCues(); len(cues) != 0 {
			t.Errorf("%s: died aloud: %+v", name, cues)
		}
	}
}

func TestBurstCue(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	w := game.WorldView{Bursts: []game.BurstView{{X: 4, Y: 5, Tick: 3, Species: "emberbuck"}}}
	r.emitWorldBursts(w)
	r.emitWorldBursts(w) // the same stamp again: still one bang
	cues := r.takeCues()
	if len(cues) != 1 || cues[0].id != voiceBurst || cues[0].vol != volBurst ||
		int(cues[0].x) != 4 || int(cues[0].y) != 5 {
		t.Fatalf("burst cues = %+v, want one animal.burst in tile 4,5", cues)
	}
	w.Bursts = append(w.Bursts, game.BurstView{X: 8, Y: 2, Tick: 9})
	r.emitWorldBursts(w)
	if cues := r.takeCues(); len(cues) != 1 || int(cues[0].x) != 8 {
		t.Fatalf("a new stamp should add exactly one bang, got %+v", cues)
	}
}

func TestFallbackChain(t *testing.T) {
	t.Parallel()
	have := map[string]bool{}
	r := voiceRenderer(1)
	r.soundHas = func(id string) bool { return have[id] }
	if got := r.animalSoundID("tapir", voiceCall); got != "" {
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
		if got := r.animalSoundID(tc.key, tc.event); got != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.key, tc.event, got, tc.want)
		}
	}
	// resolved once and cached
	have["animal.red_fox.call"] = true
	if got := r.animalSoundID("red_fox", "call"); got != "animal.generic.call" {
		t.Fatalf("the answer should be cached, got %q", got)
	}
	// no soundHas: every sound exists, the species' own is chosen
	r2 := voiceRenderer(1)
	if got := r2.animalSoundID("red_fox", "call"); got != "animal.red_fox.call" {
		t.Fatalf("nil soundHas = %q", got)
	}
	// a missing sound is silent: no cue, no panic
	r.cueAnimal("red_fox", "attack", 1, 1, 1)
	if len(r.takeCues()) != 0 {
		t.Fatal("a missing sound still cued")
	}
}

func TestEveryRosterVoiceHasAGroup(t *testing.T) {
	t.Parallel()
	groups := map[string]bool{"small_mammal": true, "large_mammal": true, "canid": true, "felid": true,
		"bovid": true, "bird_small": true, "bird_large": true, "reptile": true, "insect": true}
	if len(voiceGroups) != 61 {
		t.Fatalf("%d voice groups listed, want 61 species", len(voiceGroups))
	}
	for k, g := range voiceGroups {
		if !groups[g] {
			t.Errorf("%s: unknown group %q", k, g)
		}
	}
	if voiceGroup("nobody") != "generic" {
		t.Fatal("an unlisted species should fall to generic")
	}
}

func TestCallIntervalScales(t *testing.T) {
	t.Parallel()
	base := callInterval(game.AnimalView{ID: 5, Species: "deer"}, 2, false)
	if base < callEveryMin || base >= callEveryMin+callEverySpan {
		t.Fatalf("interval %d outside %d..%d", base, callEveryMin, callEveryMin+callEverySpan)
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
		{game.AnimalView{ID: 5, Species: "deer", Size: largeSize}, 1.3},
		{game.AnimalView{ID: 5, Species: "hive_beetle"}, 0.5},
	} {
		if got := callInterval(tc.an, 2, false); !approx(got, tc.scale) {
			t.Errorf("%s size %v: %d, want about %d x %v", tc.an.Species, tc.an.Size, got, base, tc.scale)
		}
	}
	first := callInterval(game.AnimalView{ID: 5, Species: "deer"}, 0, true)
	if first < firstCallMin || first >= firstCallMin+firstCallSpan {
		t.Fatalf("first call %d outside %d..%d", first, firstCallMin, firstCallMin+firstCallSpan)
	}
	if callInterval(game.AnimalView{ID: 6, Species: "deer"}, 0, true) == first {
		t.Fatal("two animals share a first-call time")
	}
}

func TestCallRateCaps(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(0.5)
	species := []string{"deer", "cow", "horse", "goat", "sheep", "cat", "husky", "chicken", "duck", "pig", "elk", "lynx"}
	var herd []game.AnimalView
	for i := 0; i < 40; i++ {
		an := wanderer(uint64(100+i), species[i%len(species)], 2+i%8, 3+i/8)
		herd = append(herd, an)
		if !r.audible(float64(an.X)+0.5, float64(an.Y)+0.5) {
			t.Fatalf("test setup: animal %d must be within earshot", i)
		}
	}
	const frames = 3600
	cues := listen(r, game.WorldView{Animals: herd}, frames)
	if len(cues) < 3 {
		t.Fatalf("a 40-animal herd made only %d calls in %d frames", len(cues), frames)
	}
	for _, c := range cues {
		if !strings.HasSuffix(c.id, ".call") {
			t.Fatalf("unexpected cue %q", c.id)
		}
	}
	for start := 0; start < frames; start++ {
		calls, seen := 0, map[string]bool{}
		for _, c := range cues {
			if c.frame >= start && c.frame < start+capCallsWindow {
				calls++
			}
			if c.frame >= start && c.frame < start+capSpeciesSpan {
				seen[c.id] = true
			}
		}
		if calls > capCalls {
			t.Fatalf("%d calls in the %d frames from %d", calls, capCallsWindow, start)
		}
		if len(seen) > capSpecies {
			t.Fatalf("%d species called in the %d frames from %d: %v", len(seen), capSpeciesSpan, start, seen)
		}
	}
}

func TestVoicePrunedWithAnimals(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	fleer := wanderer(3, "deer", 7, 7)
	fleer.Mode = "fleeing"
	three := game.WorldView{Animals: []game.AnimalView{wanderer(1, "deer", 5, 5), wanderer(2, "cow", 6, 6), fleer}}
	listen(r, three, 3)
	v := &r.voice
	if len(v.nextCall) != 2 || len(v.last) != 3 || len(v.byID) != 3 {
		t.Fatalf("tracking %d timers (the fleeing deer has none yet), %d seen, %d indexed; want 2/3/3", len(v.nextCall), len(v.last), len(v.byID))
	}
	listen(r, game.WorldView{Animals: []game.AnimalView{wanderer(1, "deer", 5, 5)}}, 2)
	if len(v.nextCall) != 1 || len(v.last) != 1 || len(v.byID) != 1 || len(v.fleeing) != 1 || len(v.calls) > 1 {
		t.Fatalf("after two left: nextCall %d last %d byID %d fleeing %d calls %d, want 1 each (calls at most 1)",
			len(v.nextCall), len(v.last), len(v.byID), len(v.fleeing), len(v.calls))
	}
	if _, ok := v.nextCall[1]; !ok {
		t.Fatal("the animal that stayed lost its timer")
	}
	listen(r, game.WorldView{}, 1)
	if len(v.nextCall)+len(v.last)+len(v.byID)+len(v.fleeing)+len(v.calls) != 0 {
		t.Fatal("an empty world left voice state behind")
	}
}

func TestListenAnimalsWithoutViewportCallsAnywhere(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64}) // no viewport set: the director culls
	w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 60, 60)}}
	if len(listen(r, w, firstCallMin+firstCallSpan+1)) == 0 {
		t.Fatal("with no viewport the deer should still call (the director culls)")
	}
}
