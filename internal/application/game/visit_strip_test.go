package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// SIM-065: visiting the sick, bedridden partners and stripping.

// friendAt adds colonist Fri at (x, y), a friend of pat both ways.
func friendAt(t *testing.T, g *Game, pat entity.EntityID, x, y int) entity.EntityID {
	t.Helper()
	fri, _ := g.SpawnNamedPawn(x, y, "Fri")
	g.ensurePerson(fri)
	now := socNow(g)
	g.addOpinionMemory(fri, pat, social.MemRescuedMe, now)
	g.addOpinionMemory(pat, fri, social.MemRescuedMe, now)
	return fri
}

func TestFriendVisitsBedriddenColonistAndBothRelax(t *testing.T) {
	g, pat, _ := sickPatient(t)
	fri := friendAt(t, g, pat, 8, 4)
	if !g.lyingAwake(pat) {
		g.SetNeeds(pat, 9000, 9000)
		restFed(g, 1)
	}
	joyP := g.joy[pat]
	visiting := func() bool { j, _ := g.jobs.Get(fri); return j.Kind == job.VisitSick }
	start := g.joy[fri]
	ok := false
	for i := 0; i < 8000 && !ok; i++ {
		g.joy[fri] = min(g.joy[fri], recreation.Bored) // bored again
		restFed(g, 1)
		ok = visiting()
	}
	if !ok {
		t.Fatalf("a bored friend of a bedridden colonist visits it (joy %d, start %d)", g.joy[fri], start)
	}
	g.joy[pat], joyP = 3000, 3000
	g.joy[fri] = recreation.Bored
	if !stepUntilFed(g, 3000, func() bool { return !visiting() }) {
		t.Fatal("the visit ends")
	}
	if g.joy[fri] <= recreation.Bored+care.VisitGain/2 || g.joy[pat] <= joyP+care.VisitGain/2 {
		t.Fatalf("both gain recreation: friend %d, patient %d", g.joy[fri], g.joy[pat])
	}
	found := false
	for _, e := range g.logs[pat] {
		found = found || e.Kind == logVisited
	}
	if !found {
		t.Fatal("the patient's log says it was visited")
	}
}

// stepUntilFed steps with everyone fed and rested, not touching joy.
func stepUntilFed(g *Game, n int, cond func() bool) bool { return restFedUntil(g, n, cond) }

func TestVisitPairDoesNotFlap(t *testing.T) {
	g, pat, _ := sickPatient(t)
	fri := friendAt(t, g, pat, 8, 4)
	starts, prev := 0, false
	var last, lastEnd uint64
	for i := 0; i < 12000; i++ {
		g.joy[fri] = min(g.joy[fri], recreation.Bored)
		restFed(g, 1)
		j, _ := g.jobs.Get(fri)
		on := j.Kind == job.VisitSick
		now := socNow(g)
		if on && !prev {
			starts++
			if last != 0 && now-lastEnd < 1000 {
				t.Fatalf("visit restarted %d ticks after the last ended", now-lastEnd)
			}
			last = now
		}
		if !on && prev {
			lastEnd = now
		}
		prev = on
	}
	if starts == 0 {
		t.Fatal("no visit happened")
	}
}

func TestBedriddenAwakePatientIsAChatPartner(t *testing.T) {
	g, pat, _ := sickPatient(t)
	if !g.lyingAwake(pat) {
		g.SetNeeds(pat, 9000, 9000)
		restFed(g, 1)
	}
	g.ensurePerson(pat)
	if !g.lyingAwake(pat) || !g.canSocialize(pat, false) {
		t.Fatal("a rested patient lying in bed can listen")
	}
	g.SetNeeds(pat, 9000, 1000)
	if g.lyingAwake(pat) {
		t.Fatal("an exhausted one sleeps")
	}
}

func TestStripDropsDownedColonistsGear(t *testing.T) {
	g, doc, patient := careScene(t)
	w := g.apparel[patient]
	w.On[item.Body].Kind = item.Parka
	if g.apparel == nil {
		g.apparel = map[entity.EntityID]worn{}
	}
	g.apparel[patient] = w
	g.equip[patient] = item.Rifle
	pos, _ := g.positions.Get(patient)
	g.Enqueue(command.Command{Kind: command.Prioritize, Order: command.OrderStrip, Pawn: uint64(doc),
		Target: uint64(patient), X: pos.X, Y: pos.Y})
	have := func(k item.Kind) bool {
		found := false
		g.stacks.All(func(_ entity.EntityID, st item.Stack) { found = found || st.Kind == k && st.Carrier == 0 })
		return found
	}
	if !runCare(g, 200, func() bool { return have(item.Rifle) }, doc, patient) {
		t.Fatal("the weapon drops as an item")
	}
	// the parka dropped too (a colonist may already have picked it up)
	if g.apparel[patient].Kind(item.Body) != 0 {
		t.Fatal("nothing left worn")
	}
	took := false
	for _, e := range g.logs[patient] {
		took = took || e.Kind == logTookOff
	}
	if !took {
		t.Fatal("the patient's log records the garment taken off")
	}
	if _, armed := g.equip[patient]; armed {
		t.Fatal("no weapon left")
	}
	if v := g.VitalOf(patient); v.Status != health.Downed {
		t.Fatalf("still downed: %+v", v)
	}
}
