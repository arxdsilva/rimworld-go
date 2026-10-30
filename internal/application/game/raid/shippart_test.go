package raid

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// crashed brings a part down at the world's edge range and returns it.
func crashed(t *testing.T, w *mechWorld, sp *ShipParts, points int) *ShipPart {
	t.Helper()
	w.home = [2]int{0, 0}
	if !sp.Crash(points, 0) || sp.Parts() != 1 || w.parts != 1 {
		t.Fatalf("no part came down: %d", sp.Parts())
	}
	return &sp.parts[0]
}

// TestShipPartGuardsSleep (DEF-4): a part comes down shipRangeMin..
// shipRangeMax from the colony with its guards dormant round it, posted
// where they lie, and a threat letter.
func TestShipPartGuardsSleep(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	ms, sp := w.mechs()
	p := crashed(t, w, sp, 600)
	if d := max(p.X, p.Y); d < shipRangeMin || d > shipRangeMax {
		t.Errorf("landed %d from home", d)
	}
	if len(p.Guards) == 0 || len(w.threats) != 1 || p.Awake {
		t.Fatalf("guards %v letters %v awake %v", p.Guards, w.threats, p.Awake)
	}
	for _, g := range p.Guards {
		gp, r := w.pos[g], ms.recs[g]
		if a := w.animals[g]; !a.Dormant || !animal.IsMech(a.Species) || !r.Guard || r.PostX != gp[0] || r.PostY != gp[1] {
			t.Errorf("guard %d: %+v rec %+v at %v", g, a, r, gp)
		}
	}
	if sp.Crash(600, 0) && sp.Parts() != 2 {
		t.Error("a second part is its own")
	}
}

// TestShipPartWakeNoFlap: a colonist just outside a guard's wake range
// leaves the group asleep check after check; one step closer wakes every
// guard at once, and walking away again never puts them back to sleep.
func TestShipPartWakeNoFlap(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	_, sp := w.mechs()
	p := crashed(t, w, sp, 600)
	g := w.pos[p.Guards[0]]
	// beside the first guard, on the side away from the hull
	c := w.colonist(g[0]-guardWake-1, g[1]+guardWake+1)
	for now := uint64(shipCheckEvery); now <= 20*shipCheckEvery; now += shipCheckEvery {
		sp.Tick(now)
	}
	if p.Awake {
		t.Fatal("a colonist out of reach woke the guards")
	}
	w.pos[c] = [2]int{g[0] - guardWake, g[1] + guardWake}
	sp.Tick(21 * shipCheckEvery)
	if !p.Awake || len(w.threats) != 2 {
		t.Fatalf("one step closer the guards wake: awake %v letters %v", p.Awake, w.threats)
	}
	flips := 0
	for now := uint64(22 * shipCheckEvery); now <= 200*shipCheckEvery; now += shipCheckEvery {
		w.pos[c] = [2]int{0, 0}
		if now%(2*shipCheckEvery) == 0 {
			w.pos[c] = [2]int{g[0] - guardWake, g[1] + guardWake}
		}
		sp.Tick(now)
		for _, id := range p.Guards {
			if w.animals[id].Dormant {
				flips++
			}
		}
	}
	if flips != 0 || !p.Awake || len(w.threats) != 2 {
		t.Fatalf("woken guards went back to sleep %d times (letters %v)", flips, w.threats)
	}
	for _, id := range p.Guards {
		if a := w.animals[id]; !a.Enraged(1<<40) || a.Mode != animal.Attack {
			t.Errorf("guard %d woke calm: %+v", id, a)
		}
	}
}

// TestShipPartWakesWhenHurt: a guard woken by a hurt (the game's) wakes
// its group on the next check; a hit on the hull wakes them at once.
func TestShipPartWakesWhenHurt(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	_, sp := w.mechs()
	p := crashed(t, w, sp, 600)
	a := w.animals[p.Guards[0]]
	a.Dormant = false
	w.animals[p.Guards[0]] = a
	sp.Tick(shipCheckEvery)
	for _, id := range p.Guards {
		if w.animals[id].Dormant {
			t.Fatal("one guard hurt wakes the group")
		}
	}
	w2 := newMechWorld()
	_, sp2 := w2.mechs()
	p2 := crashed(t, w2, sp2, 600)
	sp2.Hit(entity.EntityID(1), false, 3) // some other building
	if p2.Awake {
		t.Fatal("another building's damage is none of its business")
	}
	sp2.Hit(p2.ID, false, 3)
	if !p2.Awake || w2.animals[p2.Guards[0]].Dormant {
		t.Fatal("a hit on the hull wakes its guards")
	}
}

// TestShipPartBreaksIntoSalvage: destroyed, a part leaves the wiki's
// salvage, frees its guards from their posts and is gone.
func TestShipPartBreaksIntoSalvage(t *testing.T) {
	t.Parallel()
	w := newMechWorld()
	ms, sp := w.mechs()
	p := crashed(t, w, sp, 600)
	guard := p.Guards[0]
	sp.Hit(p.ID, true, 9)
	want := map[item.Kind]int{item.Steel: 60, item.Plasteel: 10, item.Components: 1, item.AdvancedComponents: 1}
	for k, n := range want {
		if w.drops[k] != n {
			t.Errorf("%v: %d, want %d", k, w.drops[k], n)
		}
	}
	if sp.Parts() != 0 || ms.recs[guard].Guard || w.animals[guard].Dormant {
		t.Fatalf("parts %d, guard %+v", sp.Parts(), ms.recs[guard])
	}
}

// TestShipPartBlight: the blight's radius grows from 5 to 100 tiles over
// 20 days and stops there; a day of hourly sweeps withers every plant in
// its disc and none beyond.
func TestShipPartBlight(t *testing.T) {
	t.Parallel()
	if BlightRadius(0) != blightStart || BlightRadius(10*clock.TicksPerDay) != 52 ||
		BlightRadius(20*clock.TicksPerDay) != blightMax || BlightRadius(90*clock.TicksPerDay) != blightMax {
		t.Fatalf("radius: %d %d %d %d", BlightRadius(0), BlightRadius(10*clock.TicksPerDay),
			BlightRadius(20*clock.TicksPerDay), BlightRadius(90*clock.TicksPerDay))
	}
	w := newMechWorld()
	_, sp := w.mechs()
	p := crashed(t, w, sp, 110)
	cx, cy := p.middle()
	const span = 20
	for y := cy - span; y <= cy+span; y++ {
		for x := cx - span; x <= cx+span; x++ {
			w.plants[[2]int{x, y}] = true
		}
	}
	for now := uint64(clock.TicksPerHour); now <= 2*clock.TicksPerDay; now += clock.TicksPerHour {
		sp.Tick(now)
	}
	// every row was swept on day two with at least the radius it had at
	// its start; nothing beyond the radius it has now was touched
	in, out := BlightRadius(clock.TicksPerDay), BlightRadius(2*clock.TicksPerDay)
	for y := cy - span; y <= cy+span; y++ {
		for x := cx - span; x <= cx+span; x++ {
			d2, alive := (x-cx)*(x-cx)+(y-cy)*(y-cy), w.plants[[2]int{x, y}]
			if d2 <= in*in && alive || d2 > out*out && !alive {
				t.Fatalf("plant at (%d, %d), %d² from the middle: alive %v (radius %d..%d)", x, y, d2, alive, in, out)
			}
		}
	}
}
