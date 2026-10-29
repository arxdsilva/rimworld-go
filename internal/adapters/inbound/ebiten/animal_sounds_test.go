package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	cuepkg "rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// voiceRenderer is a renderer looking at the map's top-left corner at the
// given zoom, on a 1280x720 screen, with the animal voice's viewport set.
func voiceRenderer(zoom float64) *Renderer {
	r := New(&stubSim{w: 64, h: 64})
	r.cam.Zoom = zoom
	r.cueBus().Voice.View = camerapkg.ColonyViewport(1280, 720)
	return r
}

func wanderer(id uint64, species string, x, y int) game.AnimalView {
	return game.AnimalView{ID: entity.EntityID(id), Species: species, X: x, Y: y, Mode: "wandering"}
}

// heardCue is a cue with the frame it came out on.
type heardCue struct {
	frame int
	cuepkg.Cue
}

// listen runs listenAnimals over frames frames of the same world and
// returns every cue, with its frame.
func listen(r *Renderer, w game.WorldView, frames int) []heardCue {
	var out []heardCue
	for f := 0; f < frames; f++ {
		r.mo.Stepped = r.speed.ticksPerFrame() // what advance stepped this frame
		r.cueBus().ListenAnimals(r.liveView(), w, r.mo.Stepped)
		for _, c := range r.cueBus().Take() {
			out = append(out, heardCue{f, c})
		}
	}
	return out
}

func idsOf(cs []heardCue) []string {
	var ids []string
	for _, c := range cs {
		ids = append(ids, c.ID)
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
	cues := listen(r, w, cuepkg.FirstCallMin+cuepkg.FirstCallSpan+1)
	if len(cues) == 0 {
		t.Fatal("a wandering deer on screen never called within its first-call window")
	}
	c := cues[0]
	if c.ID != "animal.deer.call" || c.Vol != cuepkg.VolCall || c.X != 5.5 || c.Y != 5.5 || c.Global {
		t.Fatalf("first call = %+v, want animal.deer.call at 5.5,5.5 volume %v", c.Cue, cuepkg.VolCall)
	}
	if c.frame < cuepkg.FirstCallMin {
		t.Fatalf("first call at frame %d, earlier than %d", c.frame, cuepkg.FirstCallMin)
	}
	// and then it calls again, on a longer beat
	more := listen(r, w, cuepkg.CallEveryMin+cuepkg.CallEverySpan+cuepkg.DeferMin+cuepkg.DeferSpan+1)
	if len(more) == 0 {
		t.Fatal("the deer never called a second time")
	}
}

func TestNoCallsWhilePaused(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	r.speed = speedPaused
	w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
	if cues := listen(r, w, 2*(cuepkg.FirstCallMin+cuepkg.FirstCallSpan)); len(cues) != 0 {
		t.Fatalf("paused game called: %v", idsOf(cues))
	}
	// unpaused, the timer starts from where it was: nothing is owed
	r.speed = speedNormal
	if cues := listen(r, w, cuepkg.FirstCallMin-1); len(cues) != 0 {
		t.Fatalf("a call came before its time after unpausing: %v", idsOf(cues))
	}
}

func TestCallTimerFollowsGameSpeed(t *testing.T) {
	t.Parallel()
	first := func(s speed) int {
		r := voiceRenderer(1)
		r.speed = s
		w := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
		cues := listen(r, w, cuepkg.FirstCallMin+cuepkg.FirstCallSpan+1)
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
	if cues := listen(r, far, 2*(cuepkg.FirstCallMin+cuepkg.FirstCallSpan)); len(cues) != 0 {
		t.Fatalf("an off-screen deer called: %v", idsOf(cues))
	}
	// it walks into view: the missed calls are not made up
	near := game.WorldView{Animals: []game.AnimalView{wanderer(7, "deer", 5, 5)}}
	if cues := listen(r, near, 5); len(cues) != 0 {
		t.Fatalf("catch-up call as the deer came on screen: %v", idsOf(cues))
	}
	if r.cueBus().Voice.NextCall[7] <= r.cueBus().Voice.Clock {
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
	if len(cues) != 1 || cues[0].ID != "animal.deer.call" || cues[0].frame != 0 {
		t.Fatalf("a fleeing deer should cry once, right away; got %v", cues)
	}
	if r.cueBus().Voice.NextCall[7] <= r.cueBus().Voice.Clock {
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
	r.cueBus().ListenAnimals(r.liveView(), game.WorldView{Animals: animals}, r.mo.Stepped)
	r.cueBus().Take()
	return r
}

func TestAttackCueAtAttacker(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "timber_wolf", 10, 10))
	r.cueBus().TraceSound(game.TraceView{Attacker: 9, Target: 3, FromX: 10, FromY: 10, ToX: 11, ToY: 10, Hit: true, Melee: true})
	cues := r.cueBus().Take()
	if len(cues) != 2 {
		t.Fatalf("a wolf bite should cue the snarl and the impact, got %+v", cues)
	}
	if a := cues[0]; a.ID != "animal.timber_wolf.attack" || a.Vol != cuepkg.VolAttack || a.X != 10.5 || a.Y != 10.5 {
		t.Fatalf("attack cue = %+v, want animal.timber_wolf.attack at the wolf, volume %v", a, cuepkg.VolAttack)
	}
	if m := cues[1]; m.ID != "sfx.melee_hit" || m.Vol != cuepkg.VolMeleeUnder || m.X != 11.5 {
		t.Fatalf("impact = %+v, want sfx.melee_hit at the target, volume %v under the voice", m, cuepkg.VolMeleeUnder)
	}
	// a pawn's blow keeps the full-volume impact and no voice
	r.cueBus().TraceSound(game.TraceView{Attacker: 3, Target: 4, FromX: 1, FromY: 1, ToX: 2, ToY: 1, Hit: true, Melee: true})
	cues = r.cueBus().Take()
	if len(cues) != 1 || cues[0].ID != "sfx.melee_hit" || cues[0].Vol != 1 {
		t.Fatalf("pawn blow = %+v, want one full-volume sfx.melee_hit", cues)
	}
}

func TestHurtCueOnHit(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "deer", 10, 10))
	r.cueBus().TraceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 9, FromY: 10, ToX: 10, ToY: 10, Hit: true, Melee: true})
	cues := r.cueBus().Take()
	if len(cues) != 2 || cues[0].ID != "animal.deer.hurt" || cues[0].Vol != cuepkg.VolHurt || cues[0].X != 10.5 {
		t.Fatalf("a hit deer should yelp at the deer; got %+v", cues)
	}
	if cues[1].ID != "sfx.melee_hit" || cues[1].Vol != cuepkg.VolMeleeUnder {
		t.Fatalf("impact = %+v, want sfx.melee_hit at volume %v", cues[1], cuepkg.VolMeleeUnder)
	}
	// a shot is heard from the muzzle and the deer still yelps
	r.cueBus().TraceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 2, FromY: 2, ToX: 10, ToY: 10, Hit: true})
	ids := idsOf(heardOf(r.cueBus().Take()))
	if len(ids) != 2 || ids[0] != "animal.deer.hurt" || ids[1] != "sfx.gunshot" {
		t.Fatalf("a shot deer: got %v", ids)
	}
}

func heardOf(cs []cuepkg.Cue) []heardCue {
	out := make([]heardCue, len(cs))
	for i, c := range cs {
		out[i] = heardCue{Cue: c}
	}
	return out
}

func TestNoHurtOnMiss(t *testing.T) {
	t.Parallel()
	r := primed(wanderer(9, "deer", 10, 10))
	r.cueBus().TraceSound(game.TraceView{Attacker: 3, Target: 9, FromX: 9, FromY: 10, ToX: 10, ToY: 10, Melee: true})
	if cues := r.cueBus().Take(); len(cues) != 0 {
		t.Fatalf("a missed blow on a deer made sound: %+v", cues)
	}
	// a blow on a building the animal is gnawing is the wood's sound, not a yelp
	r.cueBus().TraceSound(game.TraceView{Attacker: 3, Target: 9, Hit: true, Building: true, Melee: true, ToX: 4, ToY: 4})
	if ids := idsOf(heardOf(r.cueBus().Take())); len(ids) != 1 || ids[0] != "sfx.wood_hit" {
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
	r.cueBus().ListenAnimals(r.liveView(), w, r.mo.Stepped)
	cues := r.cueBus().Take()
	if len(cues) != 1 || cues[0].ID != "animal.deer.die" || cues[0].Vol != cuepkg.VolDie || cues[0].X != 10.5 || cues[0].Y != 10.5 {
		t.Fatalf("death cue = %+v, want animal.deer.die at 10.5,10.5 volume %v", cues, cuepkg.VolDie)
	}
	// once only
	r.cueBus().ListenAnimals(r.liveView(), w, r.mo.Stepped)
	if cues := r.cueBus().Take(); len(cues) != 0 {
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
		r.cueBus().ListenAnimals(r.liveView(), game.WorldView{Items: items}, r.mo.Stepped)
		if cues := r.cueBus().Take(); len(cues) != 0 {
			t.Errorf("%s: died aloud: %+v", name, cues)
		}
	}
}

func TestBurstCue(t *testing.T) {
	t.Parallel()
	r := voiceRenderer(1)
	w := game.WorldView{Bursts: []game.BurstView{{X: 4, Y: 5, Tick: 3, Species: "emberbuck"}}}
	r.pipe().Creatures.EmitWorldBursts(r.liveView(), w)
	r.pipe().Creatures.EmitWorldBursts(r.liveView(), w) // the same stamp again: still one bang
	cues := r.cueBus().Take()
	if len(cues) != 1 || cues[0].ID != cuepkg.VoiceBurst || cues[0].Vol != cuepkg.VolBurst ||
		int(cues[0].X) != 4 || int(cues[0].Y) != 5 {
		t.Fatalf("burst cues = %+v, want one animal.burst in tile 4,5", cues)
	}
	w.Bursts = append(w.Bursts, game.BurstView{X: 8, Y: 2, Tick: 9})
	r.pipe().Creatures.EmitWorldBursts(r.liveView(), w)
	if cues := r.cueBus().Take(); len(cues) != 1 || int(cues[0].X) != 8 {
		t.Fatalf("a new stamp should add exactly one bang, got %+v", cues)
	}
}
