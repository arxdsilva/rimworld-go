package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF-4 unit tests: drafted switches, grenade belts and throws, the
// combat log, the raiders' mortars, IEDs set off by bullets and blasts.

// Disarm is the fake's Arms role.
func (f *fakeHost) Disarm(id entity.EntityID) { delete(f.weapons, id) }

func (f *fakeHost) combatArmed() *Combat {
	return New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Arms: f})
}

// TestDraftSwitchesDoNotFlap: fire at will off stops the auto attack and
// keeps it off over many ticks; hold position takes the step out of an
// auto attack; the switches change only on a command.
func TestDraftSwitchesDoNotFlap(t *testing.T) {
	t.Parallel()
	f := newFake(60, 60)
	c := f.combatArmed()
	d := c.Drafts
	f.pawn(1, 5, 5, weapon.Rifle)
	f.drafted[1] = true
	f.raider(9, 12, 5, weapon.Club)
	f.raiderIDs = []entity.EntityID{9}
	if !d.FireAtWill(1) || d.Holding(1) {
		t.Fatal("fire at will on, hold off by default")
	}
	c.Strikes.AutoEngage(1)
	if f.jobs[1].Kind != job.Attack || f.jobs[1].State == job.Do {
		t.Fatalf("an auto attack starts walking: %+v", f.jobs[1])
	}
	j := f.jobs[1]
	j.State = job.Goto
	f.jobs[1] = j
	d.Apply(command.Command{Kind: command.SetFireAtWill, Pawn: 1, Enabled: false})
	if d.FireAtWill(1) || f.interr != 1 {
		t.Fatal("off: the auto attack is cancelled")
	}
	f.jobs[1] = job.Record{}
	for i := 0; i < 300; i++ {
		c.Strikes.AutoEngage(1)
		if f.jobs[1].Kind == job.Attack {
			t.Fatalf("tick %d: an attack with fire at will off", i)
		}
	}
	d.Apply(command.Command{Kind: command.SetFireAtWill, Pawn: 1, Enabled: true})
	d.Apply(command.Command{Kind: command.SetHoldPosition, Pawn: 1, Enabled: true})
	c.Strikes.AutoEngage(1)
	if j := f.jobs[1]; j.Kind != job.Attack || j.State != job.Do {
		t.Fatalf("holding: it shoots from its tile: %+v", j)
	}
	f.pos[9] = [2]int{59, 59} // out of reach
	c.Strikes.Do(1, f.jobs[1], f.now)
	if j := f.jobs[1]; j.State != job.Complete || len(j.Path) != 0 {
		t.Fatalf("holding: it never chases: %+v", j)
	}
	forced := job.Record{Kind: job.Attack, Target: 9, Forced: true}
	c.Strikes.Do(1, forced, f.now)
	if f.jobs[1].State != job.Goto {
		t.Fatal("an order still walks")
	}
	f.raider(8, 3, 3, weapon.Club)
	d.Apply(command.Command{Kind: command.SetHoldPosition, Pawn: 8, Enabled: true})
	d.Apply(command.Command{Kind: command.SetHoldPosition, Pawn: 77, Enabled: true})
	if d.Holding(8) || d.Holding(77) {
		t.Fatal("raiders and the unknown take no orders")
	}
	d.SetHold(1, false)
	d.SetFireAtWill(1, true)
	if len(d.set) != 0 {
		t.Fatal("default switches keep no state")
	}
}

// TestGrenadeBeltAndThrow: a stack whole on the belt, a throw too far
// refused, a throw after the warmup lobs and spends one, the last one
// disarms; a raider's never run out.
func TestGrenadeBeltAndThrow(t *testing.T) {
	t.Parallel()
	f := newFake(40, 40)
	c := f.combatArmed()
	d := c.Drafts
	f.pawn(1, 5, 5, weapon.FragGrenade)
	if d.Grenades(1) != 1 {
		t.Fatal("a grenade wielded from before belts counts one")
	}
	if n := d.Carry(1, item.FragGrenades, 2); n != 2 || d.Grenades(1) != 2 {
		t.Fatalf("carry %d, belt %d", n, d.Grenades(1))
	}
	if d.Throw(1, 30, 5) || len(f.msgs) != 1 {
		t.Fatal("too far is refused with a message")
	}
	if !d.Throw(1, 10, 5) {
		t.Fatal("a throw in range")
	}
	warm := uint64(weapon.DefOf(weapon.FragGrenade).Warmup)
	d.phase(f.now + warm - 1)
	if c.Shots.Len() != 0 {
		t.Fatal("not before the warmup")
	}
	d.phase(f.now + warm)
	if c.Shots.Len() != 1 || d.Grenades(1) != 1 {
		t.Fatalf("thrown: shots %d, left %d", c.Shots.Len(), d.Grenades(1))
	}
	s, q := d.export()
	d2 := &Drafts{h: f}
	d2.importDrafts(s, q)
	if d2.Grenades(1) != 1 {
		t.Fatal("the belt survives a save")
	}
	d.Throw(1, 10, 6)
	d.phase(f.now + 2*warm)
	if _, armed := f.weapons[1]; armed || d.Grenades(1) != 0 {
		t.Fatal("the last grenade disarms")
	}
	f.raider(9, 20, 20, weapon.FragGrenade)
	d.spend(9, weapon.FragGrenade)
	if f.weapons[9] != weapon.FragGrenade {
		t.Fatal("a raider's kit never runs out")
	}
	if d.Carry(1, item.Rifle, 1) != 1 || d.Drop(1, item.Rifle) != 1 {
		t.Fatal("any other weapon: one")
	}
	d.Carry(1, item.MolotovCocktails, 4)
	if d.Drop(1, item.MolotovCocktails) != 4 || d.Grenades(1) != 0 {
		t.Fatal("the belt is put down whole")
	}
	f.weapons[2] = weapon.Molotov
	f.pos[2] = [2]int{6, 6}
	if d.Throw(2, 8, 6) {
		t.Fatal("only a standing colonist throws")
	}
	f.pawns[2] = true
	d.Throw(2, 8, 6)
	d.Forget(2)
	if len(d.queue) != 0 {
		t.Fatal("forgotten")
	}
}

func TestCombatLogRing(t *testing.T) {
	t.Parallel()
	f := newFake(10, 10)
	c := f.combat()
	l := c.Log
	l.Add("")
	if len(l.Entries()) != 0 || l.export() != nil {
		t.Fatal("empty lines are dropped")
	}
	for i := 0; i < LogSize+10; i++ {
		f.now++
		l.Add("x")
	}
	l.Add("last")
	e := l.Entries()
	if len(e) != LogSize || e[len(e)-1].Text != "last" || e[0].Tick != f.now-LogSize+2 {
		t.Fatalf("bounded ring: %d, %+v", len(e), e[0])
	}
	if &l.Entries()[0] != &e[0] {
		t.Fatal("the read model's slice is shared until the log changes")
	}
	f.pawn(5, 1, 1, weapon.Club)
	l.Down(5, true)
	delete(f.pawns, 5)
	l.Down(5, false)
	l.Down(5, true)
	if e := l.Entries(); e[len(e)-1].Text != "p5 was struck down." || e[len(e)-2].Text != "last" {
		t.Fatalf("a colonist struck down is logged once: %+v", e[len(e)-2:])
	}
	c.Drafts.SetHold(3, true)
	s := c.Turrets.Export()
	c2 := newFake(10, 10).combat()
	c2.Turrets.Import(s)
	if len(c2.Log.Entries()) != LogSize || !c2.Drafts.Holding(3) {
		t.Fatal("log and switches ride in the defence save")
	}
	c2.Log.importLog(make([]LogEntry, LogSize+5))
	if len(c2.Log.Entries()) != LogSize {
		t.Fatal("an oversized saved log is cut")
	}
}

// TestRaidersMortar: a besieging raid's mortar is crewed only by the raider
// sent to it, aims at colonists, takes no orders and no rearm.
func TestRaidersMortar(t *testing.T) {
	t.Parallel()
	f := newFake(80, 80)
	f.build(70, construction.Mortar, 5, 5)
	c := f.combat()
	m := c.Mortars
	m.Raise(70)
	if crew, ok := m.Crew(70); !ok || crew != 0 || m.Shells(70) != SiegeHEShells+SiegeIncendiaryShells {
		t.Fatal("raised: uncrewed, loaded")
	}
	if _, ok := m.Crew(71); ok {
		t.Fatal("no such mortar")
	}
	f.pawn(1, 7, 9, weapon.Rifle)
	f.drafted[1] = true
	f.pawn(2, 5, 45, weapon.Rifle) // the mark, 40 tiles off
	f.now = scan70(f.now)
	m.phase(f.now)
	if m.State(70).Crew != 0 {
		t.Fatal("colonists never man the raiders' mortar")
	}
	if b, room := m.wants(70); b || room != 0 {
		t.Fatal("nor rearm it")
	}
	m.SetTarget(70, 9, 9, 1)
	if m.State(70).Forced {
		t.Fatal("it takes no orders")
	}
	f.raider(9, 6, 7, weapon.Rifle)
	if !m.Man(9, 70) || m.Man(8, 70) {
		t.Fatal("one raider crews it")
	}
	arrive(f, 9)
	start := f.now
	for len(f.traces) == 0 && f.now < start+2000 {
		f.now++
		m.phase(f.now)
	}
	if len(f.traces) != 1 || f.traces[0].ToY != 45 {
		t.Fatalf("it shells the colonist: %+v", f.traces)
	}
	delete(f.raiders, 9) // the crew falls
	f.now++
	m.phase(f.now)
	if m.State(70).Crew != 0 {
		t.Fatal("a fallen crew lets go")
	}
}

// TestIEDStruckAndChained: a bullet landing on a charge lights it; a
// blast lights the charges it reaches instead of damaging them.
func TestIEDStruckAndChained(t *testing.T) {
	t.Parallel()
	f := newFake(30, 30)
	f.build(60, construction.IEDHighExplosive, 5, 5)
	f.build(61, construction.IEDHighExplosive, 7, 5)
	f.build(62, construction.Wall, 20, 20)
	c := f.combat()
	c.Traps.Struck(20, 20)
	c.Traps.Struck(1, 1)
	if len(c.Traps.fuses) != 0 {
		t.Fatal("only a charge is lit")
	}
	c.Traps.Struck(5, 5)
	if !c.Traps.Burning(60) || c.Traps.Burning(61) {
		t.Fatal("the bullet lit the first")
	}
	c.Traps.phase(f.now + IEDFuse)
	if !c.Traps.Burning(61) || f.damage[61] != 0 {
		t.Fatal("the blast lit the second instead of breaking it")
	}
}
