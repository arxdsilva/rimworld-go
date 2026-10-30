package combat

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeBelts is what the fake's creatures wear at the belt (CBT-3).
type fakeBelts map[entity.EntityID]item.Kind

func (b fakeBelts) Belt(id entity.EntityID) (item.Kind, uint8) { return b[id], 0 }

// gearRig is a fake map whose colonists have hands and belts.
func gearRig() (*fakeHost, *Combat, fakeBelts) {
	f := newFake(60, 30)
	b := fakeBelts{}
	c := New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f, Field: f, Supply: f, Arms: fakeArms{f}, Draft: f, Belts: b})
	return f, c, b
}

// arm puts item k in colonist id's hands.
func arm(f *fakeHost, id entity.EntityID, k item.Kind) {
	f.wield[id], f.weapons[id] = k, weapon.ForItem(k)
}

// TestRocketLauncherSingleUse: a rocket launcher is used up by its shot
// and the attack ends; a triple launcher by its whole burst of three.
// wiki: Doomsday rocket launcher, Triple rocket launcher
func TestRocketLauncherSingleUse(t *testing.T) {
	t.Parallel()
	f, c, _ := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.RocketLauncher)
	f.raider(2, 25, 5, weapon.Fists)
	j := job.Record{Kind: job.Attack, Target: 2, Forced: true, State: job.Do}
	c.Strikes.Do(1, j, f.now)
	got := f.jobs[1]
	if c.Shots.Len() != 1 || f.weapons[1] != weapon.Fists || f.wield[1] != 0 || got.State != job.Complete {
		t.Fatalf("one rocket, then empty hands: shots %d weapon %v job %+v", c.Shots.Len(), f.weapons[1], got)
	}
	f.pawn(3, 5, 8, weapon.Fists)
	arm(f, 3, item.TripleRocketLauncher)
	j = job.Record{Kind: job.Attack, Target: 2, Forced: true, State: job.Do}
	for i := 0; i < 3; i++ {
		if f.weapons[3] != weapon.TripleRocketLauncher {
			t.Fatalf("used up after %d rockets", i)
		}
		c.Strikes.Do(3, j, f.now)
		j = f.jobs[3]
		j.WorkLeft = 0 // the burst's gap over
	}
	if c.Shots.Len() != 4 || f.weapons[3] != weapon.Fists || f.jobs[3].State != job.Complete {
		t.Fatalf("three rockets, then empty hands: shots %d weapon %v", c.Shots.Len(), f.weapons[3])
	}
	if c.Gear.Spend(3, weapon.Rifle) {
		t.Fatal("a rifle is never used up")
	}
}

// TestShieldBeltStopsBulletsAndBreaks: a charged belt stops bullets and
// blasts at 3.3 energy a point, never a blow; drained it breaks, lets hits
// through until it comes back at 20 after 3,200 ticks, and recharges 13 a
// second. wiki: Shield belt
func TestShieldBeltStopsBulletsAndBreaks(t *testing.T) {
	t.Parallel()
	f, c, b := gearRig()
	g := c.Gear
	f.pawn(1, 5, 5, weapon.Fists)
	bullet := damage.Hit{Form: damage.FormBullet, Amount: 10}
	if g.Shield(1, bullet) || g.ShieldPct(1) != 0 {
		t.Fatal("no belt, no shield")
	}
	b[1] = item.ShieldBelt
	if g.ShieldPct(1) != 100 || g.Shield(1, damage.Hit{Form: damage.FormCut, Amount: 10}) {
		t.Fatal("a fresh belt is full and lets blows through")
	}
	if !g.Shield(1, bullet) {
		t.Fatal("a charged belt stops a bullet")
	}
	if e, most := g.charge(1); most != ShieldMax || e != ShieldMax-10*shieldPerDamage {
		t.Fatalf("charge %d of %d", e, most)
	}
	if !g.Shield(1, damage.Hit{Form: damage.FormBomb, Amount: 20}) || !g.Shield(1, bullet) {
		t.Fatal("it stops a blast too, and the hit that drains it")
	}
	if g.ShieldPct(1) != 0 || g.Shield(1, bullet) {
		t.Fatal("broken: the next bullet goes through")
	}
	f.now += ShieldReset - 1
	if g.ShieldPct(1) != 0 {
		t.Fatal("still down before the reset")
	}
	f.now++
	if e, _ := g.charge(1); e != shieldOnReset {
		t.Fatalf("back at 20: %d", e)
	}
	f.now += 60
	if e, _ := g.charge(1); e != shieldOnReset+shieldRecharge {
		t.Fatalf("13 a second: %d", e)
	}
	f.now += 60 * 60
	if g.ShieldPct(1) != 100 {
		t.Fatal("full again")
	}
	// an EMP breaks it at once
	if !g.Shield(1, damage.Hit{Form: damage.FormEMP, Amount: 50}) || g.ShieldPct(1) != 0 {
		t.Fatal("an EMP breaks the belt")
	}
	f.now += ShieldReset + 60*60
	f.pawn(2, 7, 5, weapon.Fists)
	b[2] = item.ShieldBelt
	f.rolls = []int{0, 0, 0}
	c.Shots.Lob(9, weapon.EMPGrenade, 20, 5, 6, 5, 6, 5)
	fly(c)
	if g.ShieldPct(1) != 0 || g.ShieldPct(2) != 0 {
		t.Fatalf("an EMP grenade breaks the belts it reaches: %d %d", g.ShieldPct(1), g.ShieldPct(2))
	}
}

// TestShieldBeltStopsShotsAndBlocksTheWearer: a shot at a belted raider
// lands on the belt, not the body; the wearer's own gun stays silent at
// range and its attack closes in to club with the stock.
func TestShieldBeltStopsShotsAndBlocksTheWearer(t *testing.T) {
	t.Parallel()
	f, c, b := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.AssaultRifle)
	f.raider(2, 15, 5, weapon.Fists)
	b[2] = item.ShieldBelt
	if !c.Gear.Shield(2, damage.Hit{Form: damage.FormBullet, Amount: 11}) {
		t.Fatal("the belt takes a bullet")
	}
	b[1] = item.ShieldBelt
	c.Strikes.Strike(1, 2, weapon.AssaultRifle)
	if c.Shots.Len() != 0 || len(f.traces) != 0 {
		t.Fatal("a belted colonist cannot shoot out")
	}
	if d := c.Gear.Reach(1, weapon.AssaultRifle); d.Ranged || d.Range != 1 {
		t.Fatalf("its gun reaches a target beside it only: %+v", d)
	}
	c.Strikes.Do(1, job.Record{Kind: job.Attack, Target: 2, Forced: true}, f.now)
	if j := f.jobs[1]; j.State != job.Goto || c.Shots.Len() != 0 {
		t.Fatalf("it walks in to club: %+v", j)
	}
	arm(f, 1, item.FragGrenades)
	if c.Drafts.Throw(1, 10, 5) {
		t.Fatal("nor does it throw a grenade")
	}
}

// TestMeleeByStuffAndQuality: a steel mace hits harder than a wooden
// one; a masterwork harder still; a gun's stock by its quality. wiki:
// Mace, Quality
func TestMeleeByStuffAndQuality(t *testing.T) {
	t.Parallel()
	blow := func(st item.Stack) string {
		f, c, _ := gearRig()
		f.pawn(1, 5, 5, weapon.Fists)
		arm(f, 1, st.Kind)
		c.Gear.Hold(1, st)
		f.raider(2, 6, 5, weapon.Fists)
		f.rolls = []int{0, 0, 200} // the head, a hit, x1.0
		c.Strikes.Strike(1, 2, weapon.ForItem(st.Kind))
		return f.hurts[0]
	}
	steel, wood := blow(item.Stack{Kind: item.Mace, Stuff: item.Steel}), blow(item.Stack{Kind: item.Mace, Stuff: item.Wood})
	master := blow(item.Stack{Kind: item.Mace, Stuff: item.Steel, Quality: quality.Stored(quality.Masterwork)})
	if steel != "2-16-1-blunt" || wood != "2-14-1-blunt" || master != "2-23-1-blunt" {
		t.Fatalf("steel %s, wood %s, masterwork %s", steel, wood, master)
	}
	f, c, _ := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.Mace)
	c.Gear.Hold(1, item.Stack{Kind: item.Club, Stuff: item.Wood}) // a stale make: another weapon's
	if c.Gear.Make(1) != (weapon.Held{}) {
		t.Fatal("a make of another kind is ignored")
	}
}

// TestTurretShotHitsAFriendly: a mini-turret's round is a projectile; a
// colonist on its line beyond the safe distance can take it, and the hurt
// names no attacker.
func TestTurretShotHitsAFriendly(t *testing.T) {
	t.Parallel()
	f, c, _ := gearRig()
	f.build(50, construction.SentryTurret, 5, 5)
	f.raider(2, 20, 5, weapon.Fists)
	f.pawn(3, 12, 5, weapon.Fists) // on the line, 7 tiles out
	f.droll = []int{0}             // the aim holds
	c.Turrets.shot(50, weapon.SentryGun, 5, 5, 2, 20, 5)
	if c.Shots.Len() != 1 || c.Shots.Views()[0].Owner != 50 || len(f.hurts) != 0 {
		t.Fatalf("the round flies: shots %d hurts %v", c.Shots.Len(), f.hurts)
	}
	fly(c)
	if len(f.hurts) != 1 || !strings.HasPrefix(f.hurts[0], "3-") || !strings.Contains(f.hurts[0], "-0-") {
		t.Fatalf("the colonist in the way took it, from no attacker: %v", f.hurts)
	}
	if !slices.Contains(f.logs, "shot 3 true "+TurretLogName) {
		t.Fatalf("its log names the turret: %v", f.logs)
	}
	if tr := f.traces[len(f.traces)-1]; !tr.FromBuilding || tr.Attacker != 50 || tr.Target != 3 {
		t.Fatalf("landed trace %+v", tr)
	}
}

// TestRecordsAndDodges: hits that land count toward the attacker's damage
// dealt and the killing blow toward its kills; a dodged blow is marked
// apart from a miss.
func TestRecordsAndDodges(t *testing.T) {
	t.Parallel()
	f, c, _ := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.Mace)
	f.raider(2, 6, 5, weapon.Fists)
	f.rolls = []int{0, 0, 200}
	c.Strikes.Strike(1, 2, weapon.Mace)
	if v := c.Gear.View(1); v.Damage != 16 || v.Kills != 0 {
		t.Fatalf("records %+v", v)
	}
	c.Gear.Count(1, 9, 5, true)
	c.Gear.Count(0, 9, 50, true)
	c.Gear.Count(1, 9, 0, false)
	if v := c.Gear.View(1); v.Damage != 21 || v.Kills != 1 {
		t.Fatalf("a kill %+v", v)
	}
	c.Gear.Died(2, 0) // the raider bleeds out: its last attacker's kill
	c.Gear.Died(2, 0)
	if v := c.Gear.View(1); v.Kills != 2 {
		t.Fatalf("a death of wounds %+v", v)
	}
	c.Gear.Count(1, 8, 3, false)
	c.Gear.Died(8, 1) // a killing blow was counted already
	c.Gear.Died(77, 0)
	if v := c.Gear.View(1); v.Kills != 2 {
		t.Fatalf("no double count %+v", v)
	}
	f.melee[2] = attack.Fresh(20)
	f.rolls = []int{0, 0, 0}
	c.Strikes.Strike(1, 2, weapon.Mace)
	if tr := f.traces[len(f.traces)-1]; tr.Hit || !tr.Dodged || TraceViews(f.traces, f.now)[len(f.traces)-1].Dodged != true {
		t.Fatalf("dodged %+v", tr)
	}
}

// TestGearSaveRoundTrip: the wielded weapon's make, shield charge,
// smokepop uses and records ride the turret save block; a stale make is
// not saved.
func TestGearSaveRoundTrip(t *testing.T) {
	t.Parallel()
	f, c, b := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.Mace)
	c.Gear.Hold(1, item.Stack{Kind: item.Mace, Stuff: item.Silver, Quality: quality.Stored(quality.Excellent), Wear: 4})
	b[1] = item.SmokepopPack
	c.Gear.Count(1, 9, 30, true)
	if !c.Gear.Pop(1) || c.Gear.Smokepops(1) != SmokepopUses-1 || f.blasts[0].Kind != explosion.Smoke || f.blasts[0].Radius != 4900 {
		t.Fatalf("a smokepop: %d %+v", c.Gear.Smokepops(1), f.blasts)
	}
	f.pawn(2, 9, 9, weapon.Fists)
	arm(f, 2, item.Knife)
	c.Gear.Hold(2, item.Stack{Kind: item.Club})
	s := c.Turrets.Export()
	if s == nil || len(s.Gear) != 1 || s.Gear[0].ID != 1 { // the knife wielder's stale club make is dropped
		t.Fatalf("export %+v", s)
	}
	f2, c2, b2 := gearRig()
	f2.pawn(1, 5, 5, weapon.Fists)
	arm(f2, 1, item.Mace)
	b2[1] = item.SmokepopPack
	c2.Turrets.Import(s)
	v := c2.Gear.View(1)
	if v.Quality != quality.Stored(quality.Excellent) || v.Stuff != item.Silver.String() || v.Smokepops != 2 || v.Kills != 1 || v.Damage != 30 {
		t.Fatalf("import %+v", v)
	}
	if st := c2.Gear.Release(1, item.Mace, 1); st.Wear != 4 || st.Stuff != item.Silver {
		t.Fatalf("released %+v", st)
	}
	for c2.Gear.Pop(1) {
	}
	if c2.Gear.Smokepops(1) != 0 || c2.Gear.Pop(1) {
		t.Fatal("three uses")
	}
}

// TestDropWeaponKeepsTheMake: a weapon stowed and drawn again, or put
// down, keeps its quality and stuff.
func TestDropWeaponKeepsTheMake(t *testing.T) {
	t.Parallel()
	f, c, _ := gearRig()
	f.pawn(1, 5, 5, weapon.Fists)
	arm(f, 1, item.Mace)
	good := item.Stack{Kind: item.Mace, Stuff: item.Gold, Quality: quality.Stored(quality.Good)}
	c.Gear.Hold(1, good)
	c.Drafts.DropWeapon(1, true) // stow
	if f.weapons[1] != weapon.Fists || c.Gear.View(1).Spare != "mace" {
		t.Fatalf("stowed: %v %+v", f.weapons[1], c.Gear.View(1))
	}
	c.Drafts.DropWeapon(1, true) // draw it back
	if c.Gear.Make(1) != (weapon.Held{Quality: good.Quality, Stuff: item.Gold}) {
		t.Fatalf("drawn: %+v", c.Gear.Make(1))
	}
	c.Drafts.DropWeapon(1, false)
	if len(f.putDown) != 1 || f.putDown[0] != "5,5:mace:1" {
		t.Fatalf("put down %v", f.putDown)
	}
}

// TestKitOutfit: a raider's kit goes on slot by slot.
func TestKitOutfit(t *testing.T) {
	t.Parallel()
	o := KitOutfit([]item.Kind{item.FlakVest, item.SimpleHelmet, item.ShieldBelt, item.Steel})
	if o.Kind(item.Body) != item.FlakVest || o.Kind(item.Head) != item.SimpleHelmet || o.Kind(item.Belt) != item.ShieldBelt {
		t.Fatalf("kit outfit %+v", o)
	}
}

// TestRearmMaterialsAndRaiderSmokepop: the slug turret rearms with
// uranium, the mortar with a reinforced barrel, the others with steel; a
// raider with a smokepop pops it when a bullet strikes it, a colonist
// does not.
func TestRearmMaterialsAndRaiderSmokepop(t *testing.T) {
	t.Parallel()
	for k, want := range map[construction.Kind][2]int{
		construction.SentryTurret: {int(item.Steel), 9}, construction.Autocannon: {int(item.Steel), 20},
		construction.UraniumSlugTurret: {int(item.Uranium), 7}, construction.Mortar: {int(item.ReinforcedBarrel), 1},
	} {
		if m, n := BarrelCost(k); int(m) != want[0] || n != want[1] {
			t.Errorf("%v: %v x%d", k, m, n)
		}
	}
	f, c, b := gearRig()
	f.build(52, construction.UraniumSlugTurret, 3, 3)
	f.pawn(7, 4, 4, weapon.Fists)
	f.stock[600] = item.Stack{Kind: item.Steel, Qty: 30, X: 2, Y: 2}
	c.Rearms.barrel(52, f.bldgs[52]) // no uranium in stock: steel
	if len(c.Rearms.jobs) != 1 || c.Rearms.jobs[0].Kind != item.Steel {
		t.Fatalf("steel stands in: %+v", c.Rearms.jobs)
	}
	f.stock[601] = item.Stack{Kind: item.Uranium, Qty: 10, X: 2, Y: 3}
	c.Rearms.jobs = nil
	f.jobs = map[entity.EntityID]job.Record{}
	c.Rearms.barrel(52, f.bldgs[52])
	if len(c.Rearms.jobs) != 1 || c.Rearms.jobs[0].Kind != item.Uranium || f.jobs[7].Count != 7 {
		t.Fatalf("uranium when there is some: %+v %+v", c.Rearms.jobs, f.jobs[7])
	}
	f.raider(1, 5, 5, weapon.Fists)
	f.pawn(2, 9, 9, weapon.Fists)
	b[1], b[2] = item.SmokepopPack, item.SmokepopPack
	bullet := damage.Hit{Form: damage.FormBullet, Amount: 10}
	if c.Gear.Shield(1, bullet) || c.Gear.Smokepops(1) != SmokepopUses-1 || len(f.blasts) != 1 {
		t.Fatalf("a raider pops smoke and still takes the bullet: %d %v", c.Gear.Smokepops(1), f.blasts)
	}
	if c.Gear.Shield(2, bullet) || c.Gear.Smokepops(2) != SmokepopUses || c.Gear.Shield(1, damage.Hit{Form: damage.FormCut, Amount: 5}) {
		t.Fatal("a colonist pops on order only; a blow pops nothing")
	}
}
