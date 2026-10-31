package combat

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// fakeDebris is the Debris role over plain maps (CBT-5): loose stacks by
// id, the fuel per cell, the cells lit and the plant damage asked.
type fakeDebris struct {
	stacks map[entity.EntityID]item.Stack
	order  []entity.EntityID // item order
	fuel   map[[2]int]int
	lit    map[[2]int]bool
	litSeq [][2]int
	plants map[[2]int]int
	gone   []entity.EntityID
	loose  int // Loose calls
}

func newFakeDebris() *fakeDebris {
	return &fakeDebris{stacks: map[entity.EntityID]item.Stack{}, fuel: map[[2]int]int{}, lit: map[[2]int]bool{}, plants: map[[2]int]int{}}
}

func newDebrisCombat(f *fakeHost, db *fakeDebris) *Combat {
	return New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Draft: f, Debris: db})
}

func (d *fakeDebris) put(id entity.EntityID, st item.Stack) {
	if _, ok := d.stacks[id]; !ok {
		d.order = append(d.order, id)
	}
	d.stacks[id] = st
}

func (d *fakeDebris) Loose(x, y int, buf []entity.EntityID) []entity.EntityID {
	d.loose++
	for _, id := range d.order {
		if st, ok := d.stacks[id]; ok && st.Carrier == 0 && st.X == x && st.Y == y {
			buf = append(buf, id)
		}
	}
	return buf
}
func (d *fakeDebris) Stack(id entity.EntityID) (item.Stack, bool) {
	st, ok := d.stacks[id]
	return st, ok
}
func (d *fakeDebris) PutStack(id entity.EntityID, st item.Stack) { d.stacks[id] = st }
func (d *fakeDebris) DropStack(id entity.EntityID) {
	delete(d.stacks, id)
	d.gone = append(d.gone, id)
}
func (d *fakeDebris) Fuel(x, y int) int { return d.fuel[[2]int{x, y}] }
func (d *fakeDebris) Kindle(x, y int) {
	d.lit[[2]int{x, y}] = true
	d.litSeq = append(d.litSeq, [2]int{x, y})
}
func (d *fakeDebris) HarmPlant(x, y, pct int) { d.plants[[2]int{x, y}] += pct }

// TestFragGrenadeWearsAndDestroysItems: a frag grenade wears the made
// things it reaches by its distance-scaled damage, destroys burnable raw
// stacks hit hard enough, leaves stone and metal whole, and a second one
// finishes the rifle off.
func TestFragGrenadeWearsAndDestroysItems(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	db := newFakeDebris()
	c := newDebrisCombat(f, db)
	db.put(1, item.Stack{Kind: item.Rifle, Qty: 1, X: 10, Y: 10})             // centre: 50 of its 100
	db.put(2, item.Stack{Kind: item.Meal, Qty: 5, X: 10, Y: 10})              // centre: 50, all of a meal's 50 HP (FOOD-4)
	db.put(3, item.Stack{Kind: item.Steel, Qty: 75, X: 10, Y: 10})            // metal shrugs it off
	db.put(4, item.Stack{Kind: item.Meal, Qty: 5, X: 11, Y: 11})              // 1.41 tiles: 31 of its 50 HP
	db.put(5, item.Stack{Kind: item.Parka, Qty: 1, X: 11, Y: 11})             // worn by 31
	db.put(6, item.Stack{Kind: item.Rifle, Qty: 1, X: 12, Y: 10})             // 2 tiles: out of 1.9
	db.put(7, item.Stack{Kind: item.Rifle, Qty: 1, X: 10, Y: 9, Carrier: 99}) // carried: not loose
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, fragBlast)
	if st := db.stacks[1]; st.Wear != 50 || st.HP() != 50 {
		t.Fatalf("centre rifle wear %d", st.Wear)
	}
	if _, ok := db.stacks[2]; ok {
		t.Fatal("the meals at the centre survived")
	}
	if db.stacks[3].Wear != 0 || db.stacks[3].Qty != 75 {
		t.Fatal("the steel took damage")
	}
	if st, ok := db.stacks[4]; !ok || st.Wear != fragBlast.At(1414) {
		t.Fatalf("the meals at 1.41 tiles: %+v (a glancing blow wears, never destroys)", st)
	}
	if w := db.stacks[5].Wear; w != fragBlast.At(1414) {
		t.Fatalf("parka wear %d, want %d", w, fragBlast.At(1414))
	}
	if db.stacks[6].Wear != 0 || db.stacks[7].Wear != 0 {
		t.Fatal("a stack out of range or carried was worn")
	}
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, fragBlast)
	if _, ok := db.stacks[1]; ok {
		t.Fatal("two grenades did not destroy the rifle")
	}
	// FOOD-4: food has hit points (wiki: Simple meal, 50), so the second
	// glancing 31 takes the meals at 1.41 tiles past their 50 as it does
	// the rifle past its 100: damage adds up on every item with HP.
	if !slices.Equal(db.gone, []entity.EntityID{2, 1, 4}) {
		t.Fatalf("destroyed %v", db.gone)
	}
	if len(db.lit) != 0 {
		t.Fatal("a frag grenade lit a fire")
	}
}

// TestBombBuildingMultipliers: wiki: Damage types: x2 on a passable
// building, x4 on an impassable one, x8 on a wall; x4 on plants (a tree
// holds three times as long). EMP and smoke harm none of them.
func TestBombBuildingMultipliers(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	db := newFakeDebris()
	c := newDebrisCombat(f, db)
	big := explosion.Def{Kind: explosion.Bomb, Radius: 3900, Damage: 50, Pen: 100}
	f.build(30, construction.Wall, 11, 10)
	f.build(31, construction.Workbench, 10, 12) // blocks movement
	f.build(32, construction.Door, 8, 10)       // passable
	f.plants[[2]int{10, 8}] = plant.Potato
	f.plants[[2]int{12, 12}] = plant.Tree
	c.Blasts.Explode(9, weapon.FragGrenade, 10, 10, big)
	if f.damage[30] != big.At(1000)*8 || f.damage[31] != big.At(2000)*4 || f.damage[32] != big.At(2000)*2 {
		t.Fatalf("wall %d bench %d door %d", f.damage[30], f.damage[31], f.damage[32])
	}
	if p := db.plants[[2]int{10, 8}]; p != big.At(2000)*4*100/explosion.PlantHP {
		t.Fatalf("crop damage %d", p)
	}
	if p := db.plants[[2]int{12, 12}]; p != big.At(explosion.Dist(2, 2))*4*100/explosion.TreeHP {
		t.Fatalf("tree damage %d", p)
	}
	f2 := newFake(20, 20)
	db2 := newFakeDebris()
	c2 := newDebrisCombat(f2, db2)
	f2.build(30, construction.Wall, 11, 10)
	f2.plants[[2]int{10, 9}] = plant.Potato
	db2.put(1, item.Stack{Kind: item.Rifle, Qty: 1, X: 10, Y: 10})
	c2.Blasts.Explode(9, weapon.EMPGrenade, 10, 10, weapon.DefOf(weapon.EMPGrenade).Blast)
	c2.Blasts.Explode(9, weapon.MortarSmoke, 10, 10, weapon.DefOf(weapon.MortarSmoke).Blast)
	if len(f2.damage) != 0 || len(db2.plants) != 0 || db2.stacks[1].Wear != 0 || db2.loose != 0 {
		t.Fatalf("EMP or smoke harmed: %v %v %+v (loose calls %d)", f2.damage, db2.plants, db2.stacks[1], db2.loose)
	}
}

// cookRig is a map with stacks of mortar shells 3 tiles apart (the HE
// shell's 2.9 radius from one reaches the next only through a frag).
func cookRig(t *testing.T) (*fakeHost, *fakeDebris, *Combat) {
	t.Helper()
	f := newFake(30, 30)
	db := newFakeDebris()
	return f, db, newDebrisCombat(f, db)
}

// runFuses steps the fuse queue from f.now for n ticks.
func runFuses(f *fakeHost, c *Combat, n int) {
	for i := 0; i < n; i++ {
		f.now++
		c.Traps.phase(f.now)
	}
}

// TestShellsCookOffInFireAndChainOnce: a stack of shells in a fire goes
// off CookOffFuse ticks later as one shell; its blast reaches a second
// stack, which goes off in turn; a stack goes off once, and the chain
// ends with the queue empty.
func TestShellsCookOffInFireAndChainOnce(t *testing.T) {
	t.Parallel()
	f, db, c := cookRig(t)
	db.put(1, item.Stack{Kind: item.MortarShells, Qty: 10, X: 10, Y: 10})
	db.put(2, item.Stack{Kind: item.MortarShells, Qty: 4, X: 12, Y: 10}) // 2 tiles: inside 2.9
	db.put(3, item.Stack{Kind: item.Steel, Qty: 10, X: 10, Y: 10})
	c.Blasts.InFire(10, 10)
	c.Blasts.InFire(10, 10) // a fire burning on: the fuse is lit once
	if len(c.Traps.fuses) != 1 || !c.Traps.fuses[0].Stack {
		t.Fatalf("fuses %+v", c.Traps.fuses)
	}
	runFuses(f, c, explosion.CookOffFuse-1)
	if f.bursts != 0 {
		t.Fatal("went off before its fuse ran out")
	}
	runFuses(f, c, 1)
	he := weapon.DefOf(weapon.MortarHE).Blast
	if f.bursts != 1 || f.blasts[0] != he || len(db.gone) != 1 || db.gone[0] != 1 {
		t.Fatalf("first: bursts %d %v gone %v", f.bursts, f.blasts, db.gone)
	}
	if len(f.msgs) != 1 || !strings.Contains(f.msgs[0], "mortar shells cooked off") {
		t.Fatalf("message %v", f.msgs)
	}
	if !c.Traps.Burning(2) {
		t.Fatal("the blast did not light the second stack")
	}
	runFuses(f, c, explosion.CookOffFuse)
	if f.bursts != 2 || !slices.Equal(db.gone, []entity.EntityID{1, 2}) {
		t.Fatalf("second: bursts %d gone %v", f.bursts, db.gone)
	}
	runFuses(f, c, 10*explosion.CookOffFuse)
	if f.bursts != 2 || len(c.Traps.fuses) != 0 {
		t.Fatalf("the chain did not end: bursts %d fuses %v", f.bursts, c.Traps.fuses)
	}
	if _, ok := db.stacks[3]; !ok {
		t.Fatal("steel blown away")
	}
}

// TestCookOffSkipsGoneAndCarried: a stack used, merged or picked up
// before its fuse runs out does not go off.
func TestCookOffSkipsGoneAndCarried(t *testing.T) {
	t.Parallel()
	f, db, c := cookRig(t)
	db.put(1, item.Stack{Kind: item.FragGrenades, Qty: 3, X: 5, Y: 5})
	db.put(2, item.Stack{Kind: item.EMPShells, Qty: 3, X: 20, Y: 20})
	c.Blasts.InFire(5, 5)
	c.Blasts.InFire(20, 20)
	db.DropStack(1)
	st := db.stacks[2]
	st.Carrier = 7
	db.stacks[2] = st
	runFuses(f, c, explosion.CookOffFuse)
	if f.bursts != 0 || len(c.Traps.fuses) != 0 {
		t.Fatalf("bursts %d fuses %v", f.bursts, c.Traps.fuses)
	}
}

// TestCookOffBoundedPerTick: a packed stockpile goes off at most
// CookOffsPerTick stacks a tick, the rest on the next.
func TestCookOffBoundedPerTick(t *testing.T) {
	t.Parallel()
	f, db, c := cookRig(t)
	n := explosion.CookOffsPerTick + 3
	for i := 0; i < n; i++ {
		id := entity.EntityID(100 + i)
		db.put(id, item.Stack{Kind: item.SmokeShells, Qty: 1, X: 1 + 2*i, Y: 1})
		c.Blasts.InFire(1+2*i, 1)
	}
	runFuses(f, c, explosion.CookOffFuse)
	if f.bursts != explosion.CookOffsPerTick {
		t.Fatalf("first tick: %d bursts, want %d", f.bursts, explosion.CookOffsPerTick)
	}
	runFuses(f, c, 1)
	if f.bursts != n || len(c.Blasts.Views()) != n {
		t.Fatalf("next tick: %d bursts, %d clouds; want %d", f.bursts, len(c.Blasts.Views()), n)
	}
}

// TestCookOffFuseSaved: a cooking stack's fuse survives a save (the gob
// round trip of the turrets block).
func TestCookOffFuseSaved(t *testing.T) {
	t.Parallel()
	f, db, c := cookRig(t)
	db.put(1, item.Stack{Kind: item.IncendiaryShells, Qty: 2, X: 5, Y: 5})
	c.Blasts.InFire(5, 5)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c.Turrets.Export()); err != nil {
		t.Fatal(err)
	}
	var s *TurretSave
	if err := gob.NewDecoder(&buf).Decode(&s); err != nil {
		t.Fatal(err)
	}
	c2 := newDebrisCombat(f, db)
	c2.Turrets.Import(s)
	if len(c2.Traps.fuses) != 1 || c2.Traps.fuses[0] != c.Traps.fuses[0] {
		t.Fatalf("fuses %+v, want %+v", c2.Traps.fuses, c.Traps.fuses)
	}
	runFuses(f, c2, explosion.CookOffFuse)
	if f.bursts != 1 || f.blasts[0] != weapon.DefOf(weapon.MortarIncendiary).Blast {
		t.Fatalf("restored fuse: bursts %d %v", f.bursts, f.blasts)
	}
}

// TestFlameIgnitesWholeRadius: an incendiary shell lights every cell it
// reaches that has something to burn (by flammability: a sure thing at
// 1000), the creatures' cells by CreatureFlammability, and the centre
// always; bare ground and cells behind a wall stay dark.
func TestFlameIgnitesWholeRadius(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	db := newFakeDebris()
	c := newDebrisCombat(f, db)
	d := weapon.DefOf(weapon.MortarIncendiary).Blast // 2.9
	for y := 7; y <= 13; y++ {
		for x := 7; x <= 13; x++ {
			db.fuel[[2]int{x, y}] = 1000
		}
	}
	db.fuel[[2]int{10, 10}] = 0 // bare centre: lit anyway
	db.fuel[[2]int{8, 10}] = 0  // bare ground in range
	f.build(30, construction.Wall, 11, 12)
	f.pawn(1, 10, 8, weapon.Fists) // a creature on fuel 1000 anyway
	db.fuel[[2]int{9, 11}] = 0
	f.pawn(2, 9, 11, weapon.Fists) // a creature on bare ground
	f.rolls = []int{0}             // its roll (the wounds roll after none: 0)
	c.Blasts.Explode(9, weapon.MortarIncendiary, 10, 10, d)
	want := 0
	for y := 7; y <= 13; y++ {
		for x := 7; x <= 13; x++ {
			in := d.Within(x-10, y-10) && (x == 10 && y == 10 || c.Sight.Line(10, 10, x, y))
			bare := x == 8 && y == 10
			if got := db.lit[[2]int{x, y}]; got != (in && !bare) {
				t.Errorf("cell (%d,%d) lit %v (in %v, bare %v)", x, y, got, in, bare)
			}
			if in && !bare {
				want++
			}
		}
	}
	if len(db.lit) != want || want < 20 {
		t.Fatalf("lit %d cells, want %d", len(db.lit), want)
	}
	if db.lit[[2]int{11, 13}] {
		t.Fatal("the cell behind the wall caught")
	}
}

// TestFlameCreatureMayNotCatch: a creature on bare ground catches only
// on a roll under CreatureFlammability.
func TestFlameCreatureMayNotCatch(t *testing.T) {
	t.Parallel()
	f := newFake(20, 20)
	db := newFakeDebris()
	c := newDebrisCombat(f, db)
	f.pawn(1, 11, 10, weapon.Fists)
	// every roll (the burn's, then the cell's ignition) misses the 70 %
	f.rolls = []int{700, 700, 700, 700, 700, 700}
	c.Blasts.Explode(9, weapon.Molotov, 10, 10, weapon.DefOf(weapon.Molotov).Blast)
	if db.lit[[2]int{11, 10}] || !db.lit[[2]int{10, 10}] || len(db.lit) != 1 {
		t.Fatalf("lit %v", db.lit)
	}
}

// TestEMPTurretAdapts: a turret that recovered from an EMP shrugs off the
// next pulse for EMPAdaptTicks, then can be stunned again.
func TestEMPTurretAdapts(t *testing.T) {
	t.Parallel()
	f, _ := turretRig(t)
	c := New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f})
	emp := weapon.DefOf(weapon.EMPGrenade).Blast
	c.Blasts.Explode(9, weapon.EMPGrenade, 10, 6, emp)
	until := c.Turrets.Aim(50).StunnedUntil
	f.now = until + 10
	c.Blasts.Explode(9, weapon.EMPGrenade, 10, 6, emp)
	if c.Turrets.Aim(50).StunnedUntil != until {
		t.Fatal("an adapted turret was stunned again")
	}
	f.now = until + explosion.EMPAdaptTicks
	c.Blasts.Explode(9, weapon.EMPGrenade, 10, 6, emp)
	if c.Turrets.Aim(50).StunnedUntil <= until {
		t.Fatal("the adaptation never wore off")
	}
}

// TestBlastChainDeterministic: the same cook-off chain with fire, wear
// and plants twice gives the same record.
func TestBlastChainDeterministic(t *testing.T) {
	t.Parallel()
	run := func() string {
		f, db, c := cookRig(t)
		for i := 0; i < 6; i++ {
			db.put(entity.EntityID(10+i), item.Stack{Kind: item.MortarShells, Qty: 2, X: 5 + 2*i, Y: 5})
			db.put(entity.EntityID(20+i), item.Stack{Kind: item.Parka, Qty: 1, X: 5 + 2*i, Y: 6})
			db.fuel[[2]int{5 + 2*i, 7}] = 500
			f.plants[[2]int{6 + 2*i, 6}] = plant.Tree
		}
		db.put(30, item.Stack{Kind: item.IncendiaryShells, Qty: 1, X: 4, Y: 5})
		f.pawn(1, 7, 7, weapon.Fists)
		f.rolls = []int{3, 900, 10, 400, 999, 0, 1, 700, 650, 20}
		c.Blasts.InFire(5, 5)
		c.Blasts.Explode(0, weapon.Molotov, 4, 6, weapon.DefOf(weapon.Molotov).Blast)
		runFuses(f, c, 20*explosion.CookOffFuse)
		var b strings.Builder
		fmt.Fprintf(&b, "bursts %d gone %v lit %v plants %v hurts %v", f.bursts, db.gone, db.litSeq, db.plants, f.hurts)
		for _, id := range db.order {
			if st, ok := db.stacks[id]; ok {
				fmt.Fprintf(&b, " %d:%d", id, st.Wear)
			}
		}
		return b.String()
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("two runs differ:\n%s\n%s", a, b)
	}
	if !strings.Contains(a, "bursts 8") {
		t.Fatalf("expected the molotov and seven cook-offs: %s", a)
	}
}
