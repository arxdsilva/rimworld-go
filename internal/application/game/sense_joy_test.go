package game

import (
	"testing"

	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// toppedOff is a colonist at full recreation in a Recreation block, with rock to
// mine and nothing else to do.
func toppedOff(t *testing.T, on bool) (*Game, entity.EntityID) {
	t.Helper()
	g, id := drugColony(t)
	onlyWork(g, id, work.Mine)
	for x := 8; x <= 14; x++ {
		g.setTerrain(x, 2, terrain.Rock)
	}
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 8, Y: 2, X2: 14, Y2: 2})
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.JoyTopOff, on)
	g.joy[id] = recreation.Full
	return g, id
}

func mining(g *Game, id entity.EntityID) bool {
	j, _ := g.jobs.Get(id)
	return j.Kind == job.Mine && j.Active()
}

// TestTopOffWorksItsFreeHours: at full recreation the rule sends it to work in a
// Recreation block; with the rule off, never; under the low mark, back to fun.
func TestTopOffWorksItsFreeHours(t *testing.T) {
	t.Parallel()
	g, id := toppedOff(t, false)
	drugStepFed(g, id, 300)
	if mining(g, id) {
		t.Fatal("rule off: a Recreation block never works")
	}
	g, id = toppedOff(t, true)
	drugStepFed(g, id, 300)
	if !mining(g, id) {
		j, _ := g.jobs.Get(id)
		t.Fatalf("topped off: looks for work, job %v", j.Kind)
	}
	g.joy[id] = recreation.Full*80/100 - 1
	for i := 0; i < 2000 && mining(g, id); i++ {
		drugStepFed(g, id, 1)
	}
	drugStepFed(g, id, 100)
	if mining(g, id) {
		t.Fatal("under the low mark it goes back to its free time once the job is done")
	}
}

// TestTopOffKeepsBedtime: mid-work in a free hour, bedtime still takes it.
func TestTopOffKeepsBedtime(t *testing.T) {
	t.Parallel()
	g, id := toppedOff(t, true)
	drugStepFed(g, id, 300)
	if !mining(g, id) {
		t.Fatal("setup: topped off, it mines")
	}
	scheduleAll(g, schedule.Sleep)
	g.joy[id] = recreation.Full
	g.SetNeeds(id, needs.Full, 5000)
	g.Step(2)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep {
		t.Fatalf("bedtime: job %v, want sleep", j.Kind)
	}
}

// TestTopOffStillEats: being kept at work doesn't starve it.
func TestTopOffStillEats(t *testing.T) {
	t.Parallel()
	g, id := toppedOff(t, true)
	stack(t, g, item.SurvivalMeal, 5, 2, 3)
	drugStepFed(g, id, 300)
	if !mining(g, id) {
		t.Fatal("setup: topped off, it mines")
	}
	g.joy[id] = recreation.Full
	g.SetNeeds(id, 500, needs.Full)
	g.Step(2)
	if j, _ := g.jobs.Get(id); !j.Kind.Eating() {
		t.Fatalf("hungry: job %v, want eating", j.Kind)
	}
}

// TestTopOffSeatWanted: low comfort is what a free hour is also for, and
// a failed look for a seat doesn't hold it from work.
func TestTopOffSeatWanted(t *testing.T) {
	t.Parallel()
	g, id := toppedOff(t, true)
	h := senseJoy{g}
	set := func(comfort int32, seatAt uint64) {
		r := g.rest.Needs.Of(id)
		r.Comfort, r.SeatAt = comfort, seatAt
		g.rest.Needs.Import(map[entity.EntityID]rest.NeedRecord{id: r})
	}
	set(rest.SeekComfort-1, 0)
	if !h.SeatWanted(id) {
		t.Fatal("low comfort, no retry pending: seat first")
	}
	set(rest.SeekComfort-1, uint64(g.clock.Now())+100)
	if h.SeatWanted(id) {
		t.Fatal("waiting out the seat retry: to work")
	}
	set(rest.SeekComfort, 0)
	if h.SeatWanted(id) {
		t.Fatal("comfort fine")
	}
}

// TestTopOffJobsDoNotFlap: the job history of a topped-off colonist stays on
// the work side, however long, while it stays in the band.
func TestTopOffJobsDoNotFlap(t *testing.T) {
	t.Parallel()
	g, id := toppedOff(t, true)
	turns, was := 0, false
	for i := 0; i < 1500; i++ {
		g.joy[id] = recreation.Full*80/100 + i%2000 // anywhere in the band
		drugStepFed(g, id, 1)
		j, _ := g.jobs.Get(id)
		if idle := j.Kind.IsIdle(); idle != was && i > 200 {
			turns++
			was = idle
		} else if i <= 200 {
			was = idle
		}
	}
	if turns > 2 {
		t.Fatalf("%d turns between idling and work inside the band", turns)
	}
}

// workedInRecreation runs a colony with the rule as given, its colonists entering
// their first Recreation hours just topped off (the case the rule is for: a normal
// day reaches that only some evenings). It counts, over colonists and ticks, the
// ticks in Recreation hours, those spent on real work, those asleep in a Sleep hour
// (a nap in a free hour gives way to the work, by design), those hungry and not at or
// on the way to a meal, and the longest hungry spell of any colonist.
func workedInRecreation(t *testing.T, on bool, ticks int) (hours, worked, slept, unfed, longest int) {
	t.Helper()
	g := NewColony(Config{Seed: 1, Width: 48, Height: 40})
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.JoyTopOff, on)
	applyNow(g, command.Command{Kind: command.DesignateCut, X: 0, Y: 0, X2: 47, Y2: 39})
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 0, Y: 0, X2: 47, Y2: 39})
	seen := map[entity.EntityID]bool{}
	spell := map[entity.EntityID]int{}
	for i := 0; i < ticks; i++ {
		g.Step(1)
		for _, id := range g.pawnIDs {
			if !g.colonist(id) {
				continue
			}
			j, _ := g.jobs.Get(id)
			if j.Kind == job.Sleep && g.activityOf(id) == schedule.Sleep {
				slept++
			}
			if n, _ := g.need.Get(id); n.Hungry() {
				spell[id]++
				longest = max(longest, spell[id])
				if !j.Kind.Eating() {
					unfed++
				}
			} else {
				spell[id] = 0
			}
			if g.activityOf(id) != schedule.Recreation {
				continue
			}
			if !seen[id] { // the block's first tick: it has just had its fill
				seen[id] = true
				g.joy[id] = recreation.Full * 97 / 100
			}
			hours++
			if j.Active() && j.Kind != job.Sleep && !j.Kind.IsIdle() && !j.Kind.Eating() {
				worked++
			}
		}
	}
	return hours, worked, slept, unfed, longest
}

// TestRecreationBlockWorksWhenFull is the balance change whole: a default colony
// over a day, with the rule off and on. It logs the working hours CSE-002 adds.
func TestRecreationBlockWorksWhenFull(t *testing.T) {
	t.Parallel()
	ticks := 40000 // past the evening's Recreation hours from 08:00
	hoursOff, workedOff, sleptOff, unfedOff, longestOff := workedInRecreation(t, false, ticks)
	hoursOn, workedOn, sleptOn, unfedOn, longestOn := workedInRecreation(t, true, ticks)
	t.Logf("recreation hours: off %d of %d colonist-ticks at work, on %d of %d (%.1f%%); sleep %d to %d, hungry away from a meal %d to %d, longest hunger %d to %d",
		workedOff, hoursOff, workedOn, hoursOn, 100*float64(workedOn)/float64(max(hoursOn, 1)), sleptOff, sleptOn, unfedOff, unfedOn, longestOff, longestOn)
	if hoursOn == 0 || workedOn <= workedOff {
		t.Fatalf("rule on: %d worked of %d, rule off %d of %d", workedOn, hoursOn, workedOff, hoursOff)
	}
	if sleptOn < sleptOff*9/10 {
		t.Fatalf("slept %d, was %d: a topped-off colonist must keep its bedtime", sleptOn, sleptOff)
	}
	// A hungry colonist drops the work at once. It may have further to walk: a worker
	// is out in the field when the meal comes due, an idler is next to the food.
	if unfedOn > unfedOff*5/4+200 {
		t.Fatalf("hungry and not going to eat %d ticks, was %d: a topped-off colonist must keep eating", unfedOn, unfedOff)
	}
	if longestOn > longestOff+600 { // a walk across this 48x40 map
		t.Fatalf("longest hunger %d ticks, was %d: a topped-off colonist must keep eating", longestOn, longestOff)
	}
}

// relaxScene: a colonist with nothing to do, beer near by, not bored.
func relaxScene(t *testing.T, joy int) (*Game, entity.EntityID, job.Record) {
	t.Helper()
	g, id := drugColony(t)
	g.joy[id] = joy
	return g, id, job.Record{Kind: job.SocialRelax, TX: 10, TY: 2, DestX: 11, DestY: 2, WorkLeft: 3000}
}

func TestRelaxDrinkFinishesBeforeFull(t *testing.T) {
	t.Parallel()
	g, id, seat := relaxScene(t, 7000)
	stack(t, g, item.Beer, 5, 4, 2)
	if !senseOf(g).Relax.Assign(id, seat) {
		t.Fatal("beer near by: takes one along")
	}
	carried := false
	for i := 0; i < 3000; i++ {
		drugStepFed(g, id, 1)
		j, _ := g.jobs.Get(id)
		if j.Kind != job.SocialRelax {
			break
		}
		if j.Carry != 0 {
			carried = true
			if pos, _ := g.positions.Get(id); inPlay(g, item.Beer) != 5 || (pos == Position{}) {
				t.Fatal("the drink in hand still counts")
			}
		}
	}
	if !carried {
		t.Fatal("the drink was never in hand")
	}
	if n := inPlay(g, item.Beer); n != 4 {
		t.Fatalf("%d beers left of 5", n)
	}
	mark := recreation.Full*99/100 - 2000
	if g.joy[id] < mark || g.joy[id] >= recreation.Full {
		t.Fatalf("joy %d: the drink is finished at %d and gives 2000, nothing wasted at full", g.joy[id], mark)
	}
	if s := g.drugs[id]; s == nil || s.LastDoseOf(item.Beer) == 0 {
		t.Fatal("no dose on record")
	}
}

func TestRelaxDrinkRespectsPolicy(t *testing.T) {
	t.Parallel()
	g, id, seat := relaxScene(t, 7000)
	stack(t, g, item.Beer, 5, 4, 2)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.NoDrugs)})
	if senseOf(g).Relax.Assign(id, seat) {
		t.Fatal("no drugs: a bare relax")
	}
	g, id, seat = relaxScene(t, 7000)
	stack(t, g, item.Beer, 5, 4, 2)
	doseOf(g).Ingest(id, item.Beer)
	if senseOf(g).Relax.Assign(id, seat) {
		t.Fatal("inside the dose gap: a bare relax")
	}
	g, id, seat = relaxScene(t, 7000)
	stack(t, g, item.SparkTablet, 5, 4, 2)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.Unrestricted)})
	if senseOf(g).Relax.Assign(id, seat) {
		t.Fatal("no hard drug makes a sociable drink")
	}
	g, id, seat = relaxScene(t, 7000)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.RelaxDrink, false)
	stack(t, g, item.Beer, 5, 4, 2)
	if s.Relax.Assign(id, seat) {
		t.Fatal("rule off: a bare relax")
	}
}

// TestRelaxDoesNotFlap keeps a colonist relaxing in company over more than a dose
// gap: it has one drink per gap, however many relaxes that is, and each sits.
func TestRelaxDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, id := drugColony(t)
	other, _ := g.SpawnNamedPawn(12, 4, "Com")
	buildNow(t, g, construction.Table, item.Wood, construction.South, 11, 2)
	stack(t, g, item.Beer, 50, 3, 3)
	relaxes, drinks := 0, 0
	var was bool
	var lastDose uint64
	for i := 0; i < 34000; i++ {
		g.jobs.Set(other, job.Record{Kind: job.Relax, State: job.Do, TX: 12, TY: 4, WorkLeft: 1000})
		g.joy[id] = 5000
		for _, p := range g.pawnIDs {
			g.SetNeeds(p, needs.Full, needs.Full)
		}
		if j, _ := g.jobs.Get(id); !j.Active() || j.Kind != job.SocialRelax {
			g.idleSocialRelax(id, 600)
		}
		g.Step(1)
		j, _ := g.jobs.Get(id)
		on := j.Kind == job.SocialRelax && j.Active() && j.Count > 0
		if on && !was {
			relaxes++
		}
		was = on
		if s := g.drugs[id]; s != nil && s.LastDoseOf(item.Beer) != lastDose {
			if now := s.LastDoseOf(item.Beer); lastDose != 0 && now-lastDose < 12*2500 {
				t.Fatalf("second drink %d ticks after the first", now-lastDose)
			}
			lastDose = s.LastDoseOf(item.Beer)
			drinks++
		}
	}
	if drinks != 2 || relaxes != 2 {
		t.Fatalf("%d drinks in %d relaxes over 34000 ticks, want one per 30000-tick gap", drinks, relaxes)
	}
	if n := inPlay(g, item.Beer); n != 48 {
		t.Fatalf("%d beers left of 50 after %d drinks", n, drinks)
	}
}

// TestRelaxDrinkBalance: whatever stops the relax, the drink is somewhere.
func TestRelaxDrinkBalance(t *testing.T) {
	t.Parallel()
	events := []struct {
		name string
		do   func(t *testing.T, g *Game, id entity.EntityID)
	}{
		{"interrupted", func(_ *testing.T, g *Game, id entity.EntityID) { g.interrupt(id, job.Interrupted) }},
		{"drafted", func(_ *testing.T, g *Game, id entity.EntityID) {
			applyNow(g, command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
		}},
		{"replaced by work", func(t *testing.T, g *Game, id entity.EntityID) {
			if !g.startJob(id, job.Record{Kind: job.Move, TX: 2, TY: 6}) {
				t.Fatal("setup: the new job starts")
			}
		}},
		{"work giver fails", func(t *testing.T, g *Game, id entity.EntityID) {
			held := stack(t, g, item.Steel, 1, 3, 6)
			g.reserve.TryEntity(id+1000, held)
			if g.startJob(id, job.Record{Kind: job.Haul, TX: 3, TY: 6, Target: held}) {
				t.Fatal("setup: the new job fails")
			}
		}},
		{"dies", func(_ *testing.T, g *Game, id entity.EntityID) { g.dieBy(id, health.CauseNone, 0) }},
	}
	for _, ev := range events {
		for _, fromPack := range []bool{false, true} {
			name := ev.name + "/stack"
			if fromPack {
				name = ev.name + "/pack"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				g, id, seat := relaxScene(t, 7000)
				pile := stack(t, g, item.Beer, 3, 4, 2)
				if fromPack {
					packed(t, g, id, pile, 3)
				}
				if !senseOf(g).Relax.Assign(id, seat) {
					t.Fatal("takes one along")
				}
				if !stepUntil(g, 200, func() bool { j, _ := g.jobs.Get(id); return j.Carry != 0 }) {
					t.Fatal("setup: the drink never came to hand")
				}
				ev.do(t, g, id)
				for i := 0; i < 80; i++ {
					if v, _ := g.vital.Get(id); v.Status != health.Dead {
						g.SetNeeds(id, needs.Full, needs.Full)
						g.joy[id] = 7000
					}
					g.Step(1)
				}
				if s := g.drugs[id]; s != nil && s.LastDoseOf(item.Beer) != 0 {
					t.Skip("it found time for the drink after all")
				}
				if n := inPlay(g, item.Beer); n != 3 {
					t.Fatalf("%d of 3 beers in play", n)
				}
			})
		}
	}
}

// moodLifted is a bored colonist on the mood lift, beer in reach, its mood pinned.
func moodLifted(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g, id := drugColony(t)
	stack(t, g, item.Beer, 3, 4, 2)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.MoodLift)})
	g.joy[id] = 7000                                                            // not bored: the lift doesn't ask
	g.memories[id] = g.memories[id].Add(mood.GotMarried, uint64(g.clock.Now())) // on the way up
	g.Step(1)
	return g, id
}

// TestMoodGateUsesPotential: a mood below the gate on its way up doesn't dose.
func TestMoodGateUsesPotential(t *testing.T) {
	t.Parallel()
	g, id := moodLifted(t)
	if m := g.thoughtLevel(id); m < 50 {
		t.Fatalf("setup: the target mood %d is below the gate", m)
	}
	for i := 0; i < 400; i++ {
		g.soc.moodM[id] = 30 * 1000
		drugStepFed(g, id, 1)
	}
	if s := g.drugs[id]; s != nil && s.LastDoseOf(item.Beer) != 0 {
		t.Fatal("rule on: a recovering mood takes no dose")
	}
}

// TestMoodGateCurrentOnlyWhenOff: the rule off, current mood alone decides.
func TestMoodGateCurrentOnlyWhenOff(t *testing.T) {
	t.Parallel()
	g, id := moodLifted(t)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.DrugPotentialMood, false)
	for i := 0; i < 400; i++ {
		g.soc.moodM[id] = 30 * 1000
		drugStepFed(g, id, 1)
	}
	if st := g.drugs[id]; st == nil || st.LastDoseOf(item.Beer) == 0 {
		t.Fatal("rule off: mood below the gate takes the dose")
	}
	g, id = moodLifted(t)
	for i := 0; i < 400; i++ {
		g.soc.moodM[id] = 60 * 1000
		drugStepFed(g, id, 1)
	}
	if st := g.drugs[id]; st != nil && st.LastDoseOf(item.Beer) != 0 {
		t.Fatal("mood above the gate: no dose")
	}
}

// packLook is long enough for a bored colonist's look for a dose.
const packLook = 3000

// TestDoseFromPack: the pack comes before any stack, and the dose stays counted.
func TestDoseFromPack(t *testing.T) {
	t.Parallel()
	g, id := drugColony(t)
	pile := stack(t, g, item.Beer, 2, 2, 3)
	packed(t, g, id, pile, 2)
	far := stack(t, g, item.Beer, 4, 12, 2) // the pack is nearer than anything
	drugStepFed(g, id, packLook)
	st := g.drugs[id]
	if st == nil || st.LastDoseOf(item.Beer) == 0 {
		t.Fatal("bored: takes a dose from its pack")
	}
	if s, _ := g.stacks.Get(far); s.Qty != 4 {
		t.Fatalf("the pile was touched: %d of 4", s.Qty)
	}
	if n := inPlay(g, item.Beer); n != 5 {
		t.Fatalf("%d of 6-1 beers in play", n)
	}
	if e := senseOf(g).Gear.Pack(id).Items; len(e) != 1 || e[0].Stack.Qty != 1 {
		t.Fatalf("pack %+v, want one left", e)
	}
}

// TestPackDoseInterrupted: a dose stopped on its way down goes back to the pack.
func TestPackDoseInterrupted(t *testing.T) {
	t.Parallel()
	g, id := drugColony(t)
	pile := stack(t, g, item.Beer, 2, 2, 3)
	packed(t, g, id, pile, 2)
	dosing := func() bool { j, _ := g.jobs.Get(id); return j.Kind == job.Ingest && j.Carry != 0 && j.Active() }
	if !stepUntil(g, packLook, dosing) {
		t.Fatal("setup: never started the dose")
	}
	g.interrupt(id, job.Interrupted)
	g.SetNeeds(id, needs.Full, needs.Full)
	g.joy[id] = recreation.Full
	g.Step(1)
	if e := senseOf(g).Gear.Pack(id).Items; len(e) != 1 || e[0].Stack.Qty != 2 {
		t.Fatalf("pack %+v, want both back", e)
	}
	if n := inPlay(g, item.Beer); n != 2 {
		t.Fatalf("%d of 2 beers in play", n)
	}
}

// TestPackDoseOffWithTheRule: the rule off, the pack is not looked into.
func TestPackDoseOffWithTheRule(t *testing.T) {
	t.Parallel()
	g, id := drugColony(t)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.AnyDrugFromPack, false)
	pile := stack(t, g, item.Beer, 2, 2, 3)
	packed(t, g, id, pile, 2)
	drugStepFed(g, id, packLook)
	if st := g.drugs[id]; st != nil && st.LastDoseOf(item.Beer) != 0 {
		t.Fatal("rule off: the pack stays shut")
	}
}

func TestPackDoseKeepsPolicy(t *testing.T) {
	t.Parallel()
	g, id := drugColony(t)
	pile := stack(t, g, item.Beer, 2, 2, 3)
	packed(t, g, id, pile, 2)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.NoDrugs)})
	drugStepFed(g, id, packLook)
	if st := g.drugs[id]; st != nil && st.LastDoseOf(item.Beer) != 0 {
		t.Fatal("no drugs: not from the pack either")
	}
	g, id = drugColony(t)
	pile = stack(t, g, item.SparkTablet, 2, 2, 3)
	packed(t, g, id, pile, 2)
	drugStepFed(g, id, packLook)
	if st := g.drugs[id]; st != nil && st.LastDoseOf(item.SparkTablet) != 0 {
		t.Fatal("social drinking: no hard drug out of the pack")
	}
}

// giftScene: a sick colonist awake in bed, not entertained, a friend with beer in
// reach and recreation to find.
func giftScene(t *testing.T) (g *Game, pat, fri entity.EntityID) {
	t.Helper()
	g, pat, _ = sickPatient(t)
	fri = friendAt(t, g, pat, 8, 4)
	stack(t, g, item.Beer, 6, 8, 6)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(fri), Mode: uint8(drug.NoDrugs)}) // the beer is for the patient
	return g, pat, fri
}

// giftsOver steps n ticks and counts the doses patient had.
func giftsOver(g *Game, pat, fri entity.EntityID, n int) (doses int) {
	var last uint64
	for i := 0; i < n; i++ {
		g.joy[fri] = min(g.joy[fri], recreation.Bored)
		g.joy[pat] = min(g.joy[pat], recreation.Bored)
		restFed(g, 1)
		if s := g.drugs[pat]; s != nil && s.LastDoseOf(item.Beer) != last {
			last = s.LastDoseOf(item.Beer)
			doses++
		}
	}
	return doses
}

func TestGiftOnlyWithinPolicy(t *testing.T) {
	t.Parallel()
	g, pat, fri := giftScene(t)
	if doses := giftsOver(g, pat, fri, 20000); doses == 0 {
		t.Fatal("a friend brings a sick colonist a drink in time")
	}
	if n := inPlay(g, item.Beer); n >= 6 {
		t.Fatalf("%d beers of 6 after the gift", n)
	}
	g, pat, fri = giftScene(t)
	applyNow(g, command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(pat), Mode: uint8(drug.NoDrugs)})
	if doses := giftsOver(g, pat, fri, 12000); doses != 0 {
		t.Fatalf("%d doses to a patient on no drugs", doses)
	}
	g, pat, fri = giftScene(t)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.SickGifts, false)
	if doses := giftsOver(g, pat, fri, 12000); doses != 0 {
		t.Fatalf("rule off: %d doses", doses)
	}
}

// TestGiftDoesNotFlap: the dose gap holds however often the friend comes.
func TestGiftDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, pat, fri := giftScene(t)
	doses := giftsOver(g, pat, fri, 20000)
	if doses != 1 {
		t.Fatalf("%d doses in 20000 ticks, want one inside the 30000-tick gap", doses)
	}
	if n := inPlay(g, item.Beer); n != 5 {
		t.Fatalf("%d of 6 beers after one dose", n)
	}
}

// TestGiftFallsThrough: a patient that fell asleep leaves the dose with the
// visitor, which puts it down; the count balances.
func TestGiftFallsThrough(t *testing.T) {
	t.Parallel()
	for _, fromPack := range []bool{false, true} {
		g, pat, fri := giftScene(t)
		if fromPack {
			for _, sid := range g.itemIDs {
				if st, ok := g.stacks.Get(sid); ok && st.Kind == item.Beer {
					packed(t, g, fri, sid, st.Qty)
				}
			}
		}
		rec := job.Record{Kind: job.FeedPatient, TX: 9, TY: 4, Dest: pat}
		if fromPack {
			carry, ok := doseHabits{g}.Unpack(fri, item.Beer)
			if !ok {
				t.Fatal("setup: unpack")
			}
			rec.Target, rec.Carry = carry, carry
		} else {
			rec.Target, rec.TX, rec.TY = g.itemIDs[len(g.itemIDs)-1], 8, 6
		}
		if !g.startJob(fri, rec) {
			t.Fatal("setup: the gift starts")
		}
		g.Step(5)
		g.interrupt(fri, job.Interrupted)
		restFed(g, 40)
		if n := inPlay(g, item.Beer); n != 6 {
			t.Fatalf("from pack %v: %d of 6 beers", fromPack, n)
		}
	}
}

// joyOffHash is a whole colony's hash after a few thousand ticks with every habit
// off: the state of the game before the habits had any behaviour (pinned below).
func joyOffHash(seed uint64, ticks int) (uint64, *Game) {
	g := NewColony(Config{Seed: seed, Width: 48, Height: 40})
	s := senseOf(g)
	for tg := gameplay.Toggle(0); tg < gameplay.NumToggles; tg++ {
		s.Rules = s.Rules.With(tg, false)
	}
	g.Step(ticks)
	s.Rules = gameplay.Settings{}
	return g.StateHash(), g
}

// TestHabitsOffKeepTheHash: with the rules off, a colony's state is the one it was
// before they were written: pinned at f7631b72a, where they had no behaviour. A
// change outside the habits moves it, as it moves the scale goldens: re-pin it
// then, with the reason.
//
// Re-pinned (CI-002): seed 1 was taken on arm64, where the weather fused x*y+z
// and rounded differently. That commit on amd64 gives the value below, and with
// the fusion gone from the per-tick code both architectures do.
func TestHabitsOffKeepTheHash(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		seed uint64
		want uint64
	}{{1, 0xee2b9d9d5399d8ac}, {2, 0xf54cac3f8e33e07f}} {
		h, g := joyOffHash(c.seed, 30000)
		if h != c.want {
			t.Errorf("seed %d: hash %#x, want %#x", c.seed, h, c.want)
		}
		if s := senseOf(g); len(s.TopOffOn) != 0 {
			t.Errorf("seed %d: recorded topped off: %v", c.seed, s.TopOffOn)
		}
		if rs := g.soc.rec; rs != nil {
			if e := rs.Export(); e != nil && len(e.Craving) != 0 {
				t.Errorf("seed %d: recorded cravings: %v", c.seed, e.Craving)
			}
		}
	}
}

// BenchmarkFreeTimeTopOff: the free-time read as phaseAI makes it, flag set, each
// with the seat look behind it.
func BenchmarkFreeTimeTopOff(b *testing.B) {
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	s := senseOf(g)
	s.TopOff.Wants(id, recreation.Full)
	b.ReportAllocs()
	for b.Loop() {
		s.TopOff.Wants(id, 9000)
	}
}

// BenchmarkFreeTimeStep is a colony's tick in Recreation hours with nothing to do
// with the rule on (work looked for at the idle recheck) and off.
func BenchmarkFreeTimeStep(b *testing.B) {
	for _, on := range []bool{false, true} {
		name := "off"
		if on {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			g := NewColony(Config{Seed: 1, Width: 48, Height: 40})
			scheduleAll(g, schedule.Recreation)
			s := senseOf(g)
			s.Rules = s.Rules.With(gameplay.JoyTopOff, on)
			g.Step(2000)
			for _, id := range g.pawnIDs {
				g.joy[id] = recreation.Full
			}
			b.ResetTimer()
			for b.Loop() {
				g.Step(1)
			}
		})
	}
}
