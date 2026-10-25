package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestTrainingDefaultsAndToggles(t *testing.T) {
	t.Parallel()
	g, _, dog := bondYard(t, 20)
	a, _ := g.animals.Get(dog)
	if !trainOn(a, training.Obedience) || trainOn(a, training.Guard) || trainOn(a, training.Release) {
		t.Fatal("Obedience is on by default, the rest off")
	}
	set := func(id entity.EntityID, s training.Skill, on bool) {
		applyNow(g, command.Command{Kind: command.SetTraining, Target: uint64(id), Index: int(s), Enabled: on})
	}
	set(dog, training.Guard, true)
	set(dog, training.Obedience, false)
	set(dog, training.Release, true) // a labrador is no hunter: ignored
	a, _ = g.animals.Get(dog)
	if trainOn(a, training.Obedience) || !trainOn(a, training.Guard) || trainOn(a, training.Release) {
		t.Fatalf("toggled %v", a.Trained)
	}
	if trainSteps(a, training.Guard, 0) != 0 || trainSteps(a, training.Obedience, 0) != 0 {
		t.Fatal("the toggle bit is not a step")
	}
	set(dog, training.Obedience, true)
	if a, _ = g.animals.Get(dog); !trainOn(a, training.Obedience) || a.Trained[training.Obedience] != 0 {
		t.Fatal("back to default clears the bit")
	}
	wolf, _ := g.SpawnAnimal("timber_wolf", 10, 10)
	set(wolf, training.Release, true)
	if w, _ := g.animals.Get(wolf); trainOn(w, training.Release) {
		t.Fatal("wild animals are not trained")
	}
	g.TameAnimal(wolf, "Burr")
	set(wolf, training.Release, true)
	if w, _ := g.animals.Get(wolf); !trainOn(w, training.Release) {
		t.Fatal("a tame wolf can learn Release")
	}
}

func TestTrainingStepsOrderAndDecay(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 21)
	(*g.sk.skills[ada])[skill.Animals].Level = 20
	a, _ := g.animals.Get(dog)
	a.Bond = uint64(ada) // a bonded trainer of 20 never fails
	a.Trained[training.Guard] |= trainToggled
	now := uint64(1000)
	if s, _ := nextTrainSkill(a, now); s != training.Obedience {
		t.Fatal("Obedience first")
	}
	for i := 0; i < 3; i++ {
		s, _ := nextTrainSkill(a, now)
		g.trainSession(ada, dog, &a, s, now)
	}
	if !trainLearned(a, training.Obedience, now) {
		t.Fatalf("three steps learn Obedience: %v", a.Trained)
	}
	if s, ok := nextTrainSkill(a, now); !ok || s != training.Guard {
		t.Fatal("then Guard")
	}
	for i := 0; i < 3; i++ {
		g.trainSession(ada, dog, &a, training.Guard, now)
	}
	if _, ok := nextTrainSkill(a, now); ok {
		t.Fatal("everything on is learned")
	}
	later := now + training.DecayTicks
	if trainSteps(a, training.Guard, later) != 2 || trainLearned(a, training.Guard, later) {
		t.Fatal("ten days unpractised costs a step")
	}
	if s, ok := nextTrainSkill(a, later); !ok || s != training.Obedience {
		t.Fatal("a decayed skill is trained again")
	}
	if trainSteps(a, training.Obedience, now+99*training.DecayTicks) != 1 {
		t.Fatal("learned Obedience keeps its floor")
	}
	if !trainDue(dog, a, now+trainCooldown+trainJitter) || trainDue(dog, a, now+trainCooldown-1) {
		t.Fatal("the next session waits for the cooldown")
	}
}

// A handler trains a tame animal through Handle work until it learns
// Obedience, and never starts a session before the cooldown (no flap).
func TestHandlerTrainsAnimalWithCooldown(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 22)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	(*g.sk.skills[ada])[skill.Animals].Level = 20
	a, _ := g.animals.Get(dog)
	a.Bond = uint64(ada)
	g.animals.Set(dog, a)
	var starts []uint64
	last := job.None
	for i := 0; i < 4*int(trainCooldown) && len(starts) < 4; i++ {
		g.Step(1)
		if i%50 == 0 {
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.Train && last != job.Train {
			starts = append(starts, uint64(g.clock.Now()))
		}
		last = j.Kind
	}
	if len(starts) < 3 {
		t.Fatalf("handler started %d sessions", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i]-starts[i-1] < trainCooldown {
			t.Fatalf("sessions %d ticks apart: %v", starts[i]-starts[i-1], starts)
		}
	}
	a, _ = g.animals.Get(dog)
	if !trainLearned(a, training.Obedience, uint64(g.clock.Now())) {
		t.Fatalf("Obedience learnt after %d sessions: %v", len(starts), a.Trained)
	}
}

// An obedient animal follows its drafted master; undrafted, it does not.
func TestObedientAnimalFollowsDraftedMaster(t *testing.T) {
	t.Parallel()
	run := func(drafted bool) int {
		g := New(Config{Seed: 23, Width: 44, Height: 20})
		ada, _ := g.SpawnNamedPawn(36, 10, "Ada")
		dog, _ := g.SpawnAnimal("labrador", 6, 10)
		g.TameAnimal(dog, "Rook")
		a, _ := g.animals.Get(dog)
		a.Master, a.TrainDecay = uint64(ada), 1
		a.Trained[training.Obedience] = 3
		g.animals.Set(dog, a)
		g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: drafted})
		for i := 0; i < 40; i++ {
			g.Step(20)
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		dp, mp := g.posOf(dog), g.posOf(ada)
		return weapon.Distance(dp.X, dp.Y, mp.X, mp.Y)
	}
	if d := run(true); d > followFar+1 {
		t.Fatalf("drafted master: the dog stays close (%d tiles)", d)
	}
	if d := run(false); d <= followFar+1 {
		t.Fatalf("undrafted: the dog keeps to the colony (%d tiles)", d)
	}
}

func TestUntrainedAnimalDoesNotFollow(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 24)
	a, _ := g.animals.Get(dog)
	a.Master = uint64(ada)
	g.animals.Set(dog, a)
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true})
	a, _ = g.animals.Get(dog)
	if g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) {
		t.Fatal("no Obedience, no following")
	}
	a.Trained[training.Obedience], a.TrainDecay = 3, uint64(g.clock.Now())
	if !g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) {
		t.Fatal("obedient: follows")
	}
}

// dutyYard is Ada at (20, 10) with Rook, her tame labrador at (6, 10),
// trained in skills (learned, switched on), and a raider at (20+dx, 10).
func dutyYard(t *testing.T, dx int, skills ...training.Skill) (g *Game, ada, dog, rid entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 31, Width: 48, Height: 24})
	ada, _ = g.SpawnNamedPawn(20, 10, "Ada")
	dog, _ = g.SpawnAnimal("labrador", 6, 10)
	g.TameAnimal(dog, "Rook")
	a, _ := g.animals.Get(dog)
	a.Master, a.TrainDecay = uint64(ada), 0
	for _, s := range skills {
		a.Trained[s] = training.Needed(s) | trainToggled
	}
	g.animals.Set(dog, a)
	rid = spawnRaiderAt(t, g, 20+dx, 10, 1)
	g.invalidateCreatures()
	return g, ada, dog, rid
}

// dutyTick is the first tick at or after now on which id looks for a duty
// target (its staggered animal.DecideEvery slot).
func dutyTick(id entity.EntityID, now uint64) uint64 {
	for (now+uint64(id))%animal.DecideEvery != 0 {
		now++
	}
	return now
}

func TestGuardAnimalDefendsMasterFromNearbyHostile(t *testing.T) {
	t.Parallel()
	g, _, dog, rid := dutyYard(t, 3, training.Guard)
	now := dutyTick(dog, uint64(g.clock.Now()))
	a, _ := g.animals.Get(dog)
	if g.masterDuty(dog, &a, now+1) {
		t.Fatal("looked for a foe off its staggered slot")
	}
	if !g.masterDuty(dog, &a, now) || a.Target != rid || a.Mode != animal.Attack {
		t.Fatalf("guard did not take on the raider by its master: target %d mode %v", a.Target, a.Mode)
	}
	// untrained: no duty
	g2, _, dog2, _ := dutyYard(t, 3)
	b, _ := g2.animals.Get(dog2)
	if g2.masterDuty(dog2, &b, dutyTick(dog2, uint64(g2.clock.Now()))) {
		t.Fatal("an untrained animal guards")
	}
	// a hostile beyond guardMasterRadius is no business of the guard
	g3, _, dog3, _ := dutyYard(t, guardMasterRadius+1, training.Guard)
	c, _ := g3.animals.Get(dog3)
	if g3.masterDuty(dog3, &c, dutyTick(dog3, uint64(g3.clock.Now()))) {
		t.Fatal("guard went after a raider beyond its radius")
	}
}

func TestGuardAnimalAnswersHitOnMaster(t *testing.T) {
	t.Parallel()
	g, ada, dog, rid := dutyYard(t, 14, training.Guard) // a shooter far off
	now := dutyTick(dog, uint64(g.clock.Now()))
	g.lastHit[ada] = hitRec{Tick: now - 10, FromX: 34, FromY: 10, Damage: 5}
	a, _ := g.animals.Get(dog)
	if !g.masterDuty(dog, &a, now) || a.Target != rid {
		t.Fatalf("guard ignored the one shooting its master: target %d", a.Target)
	}
	g.lastHit[ada] = hitRec{Tick: now - guardHitWindow - 1, FromX: 34, FromY: 10}
	a, _ = g.animals.Get(dog)
	if g.masterDuty(dog, &a, now) {
		t.Fatal("an old hit still calls the guard")
	}
}

func TestReleaseAnimalJoinsDraftedMasterAttack(t *testing.T) {
	t.Parallel()
	g, ada, dog, rid := dutyYard(t, 15, training.Release)
	now := dutyTick(dog, uint64(g.clock.Now()))
	a, _ := g.animals.Get(dog)
	if g.masterDuty(dog, &a, now) {
		t.Fatal("released with no attack order")
	}
	g.setDraft(ada, true)
	g.jobs.Set(ada, job.Record{Kind: job.Attack, Target: rid, State: job.Do})
	if !g.masterDuty(dog, &a, now) || a.Target != rid {
		t.Fatalf("release did not join the master's attack: target %d", a.Target)
	}
	// it keeps at it on every tick while the order stands, and lets go when
	// the master stops attacking
	if !g.masterDuty(dog, &a, now+1) {
		t.Fatal("release dropped the master's target")
	}
	g.jobs.Set(ada, job.Record{})
	if g.masterDuty(dog, &a, now+2) {
		t.Fatal("release kept fighting after the attack order ended")
	}
	// guard alone does not join a far attack
	g2, ada2, dog2, rid2 := dutyYard(t, 15, training.Guard)
	g2.setDraft(ada2, true)
	g2.jobs.Set(ada2, job.Record{Kind: job.Attack, Target: rid2, State: job.Do})
	b, _ := g2.animals.Get(dog2)
	if g2.masterDuty(dog2, &b, dutyTick(dog2, uint64(g2.clock.Now()))) {
		t.Fatal("a guard (not release) joined a far attack")
	}
}

// No flap: a raider pacing between 6 and 7 tiles from the master is never
// taken on (enter at 5), and once engaged at 3 it is kept while it paces
// there (leave at 8); the guard lets go only beyond guardKeepRadius.
func TestGuardDutyHysteresisNoFlap(t *testing.T) {
	t.Parallel()
	pace := func(g *Game, rid entity.EntityID, i int) {
		g.positions.Set(rid, Position{X: 26 + i%2, Y: 10})
		g.invalidateCreatures()
	}
	g, _, dog, rid := dutyYard(t, 6, training.Guard)
	a, _ := g.animals.Get(dog)
	now := uint64(g.clock.Now())
	for i := 0; i < 300; i++ {
		pace(g, rid, i)
		if g.masterDuty(dog, &a, now+uint64(i)) {
			t.Fatalf("tick %d: took on a raider beyond the enter radius", i)
		}
	}
	g, _, dog, rid = dutyYard(t, 3, training.Guard)
	a, _ = g.animals.Get(dog)
	now = dutyTick(dog, uint64(g.clock.Now()))
	if !g.masterDuty(dog, &a, now) {
		t.Fatal("set-up: guard did not engage")
	}
	changes, last := 0, true
	for i := 1; i <= 300; i++ {
		pace(g, rid, i)
		on := g.masterDuty(dog, &a, now+uint64(i)) && a.Target == rid
		if on != last {
			changes++
		}
		last = on
	}
	if changes != 0 {
		t.Fatalf("guard duty flapped %d times inside the keep band", changes)
	}
	g.positions.Set(rid, Position{X: 20 + guardKeepRadius + 1, Y: 10})
	if g.masterDuty(dog, &a, now+301) && a.Target == rid {
		t.Fatal("guard kept a foe beyond the keep radius")
	}
}

// Stepped: a manhunter goes for Ada; her guard dog leaves its spot and
// closes on it.
func TestGuardAnimalClosesOnThreatStepped(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 31, Width: 48, Height: 24})
	ada, _ := g.SpawnNamedPawn(20, 10, "Ada")
	dog, _ := g.SpawnAnimal("labrador", 6, 10)
	g.TameAnimal(dog, "Rook")
	a, _ := g.animals.Get(dog)
	a.Master = uint64(ada)
	a.Trained[training.Guard] = training.Needed(training.Guard) | trainToggled
	g.animals.Set(dog, a)
	wolf, _ := g.SpawnAnimal("ridgewolf", 24, 10)
	w, _ := g.animals.Get(wolf)
	now := uint64(g.clock.Now())
	g.enrage(wolf, &w, ada, now)
	g.animals.Set(wolf, w)
	engaged := false
	for i := 0; i < animal.DecideEvery+1 && !engaged; i++ { // the dog's first duty slot
		g.Step(1)
		d, _ := g.animals.Get(dog)
		engaged = d.Target == wolf && d.Mode == animal.Attack
	}
	if !engaged {
		t.Fatal("the guard did not take on the manhunter by its master")
	}
	closest := 1 << 10
	for i := 0; i < 600; i++ {
		g.Step(1)
		if _, ok := g.animals.Get(wolf); !ok {
			break
		}
		dp, wp := g.posOf(dog), g.posOf(wolf)
		closest = min(closest, weapon.Distance(dp.X, dp.Y, wp.X, wp.Y))
		if closest <= 1 {
			break
		}
	}
	if closest > 1 {
		t.Fatalf("the guard never reached the manhunter (closest %d tiles)", closest)
	}
	// an untrained dog stays out of it
	g2 := New(Config{Seed: 31, Width: 48, Height: 24})
	ada2, _ := g2.SpawnNamedPawn(20, 10, "Ada")
	dog2, _ := g2.SpawnAnimal("labrador", 6, 10)
	g2.TameAnimal(dog2, "Rook")
	b, _ := g2.animals.Get(dog2)
	b.Master = uint64(ada2)
	g2.animals.Set(dog2, b)
	wolf2, _ := g2.SpawnAnimal("ridgewolf", 24, 10)
	w2, _ := g2.animals.Get(wolf2)
	g2.enrage(wolf2, &w2, ada2, uint64(g2.clock.Now()))
	g2.animals.Set(wolf2, w2)
	for i := 0; i < 120; i++ {
		g2.Step(1)
		if b, _ := g2.animals.Get(dog2); b.Target == wolf2 {
			t.Fatal("an untrained dog joined the fight")
		}
	}
}

// PLAY-015: the player can stop a trained animal following its drafted
// master; the view says whether it can and does.
func TestAnimalFollowSetting(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 24)
	a, _ := g.animals.Get(dog)
	a.Master = uint64(ada)
	a.Trained[training.Obedience], a.TrainDecay = 3, uint64(g.clock.Now())
	g.animals.Set(dog, a)
	applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true})
	view := func() AnimalView {
		for _, av := range g.World().Animals {
			if av.ID == dog {
				return av
			}
		}
		t.Fatal("dog not in view")
		return AnimalView{}
	}
	if v := view(); !v.CanFollow || !v.Follows {
		t.Fatalf("trained dog: can %v follows %v", v.CanFollow, v.Follows)
	}
	applyNow(g, command.Command{Kind: command.SetAnimalFollow, Target: uint64(dog), Enabled: false})
	a, _ = g.animals.Get(dog)
	if g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) || view().Follows {
		t.Fatal("following turned off: stays")
	}
	applyNow(g, command.Command{Kind: command.SetAnimalFollow, Target: uint64(dog), Enabled: true})
	a, _ = g.animals.Get(dog)
	if !g.followMaster(dog, &a, g.posOf(dog), uint64(g.clock.Now())) {
		t.Fatal("turned back on: follows")
	}
}
