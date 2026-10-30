package combat

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
)

// TestBestCoverPicksSandbags: of the tiles in reach, the one behind the
// sandbags (facing the shooter) wins; a tile someone stands on never.
func TestBestCoverPicksSandbags(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.build(50, construction.Sandbags, 15, 10)
	d := weapon.DefOf(weapon.Rifle)
	cx, cy, best, here, ok := c.Tactics.BestCover(1, 10, 10, 8, d, 25, 10)
	if !ok || cx != 14 || cy != 10 || best <= here || here != 0 {
		t.Fatalf("behind the sandbags: (%d,%d) cover %d here %d ok %v", cx, cy, best, here, ok)
	}
	f.pawn(9, 14, 10, weapon.Fists)
	if cx, cy, _, _, _ := c.Tactics.BestCover(1, 10, 10, 8, d, 25, 10); cx == 14 && cy == 10 {
		t.Fatal("a taken tile is no cover spot")
	}
	if _, _, _, _, ok := c.Tactics.BestCover(1, 10, 10, 1, weapon.DefOf(weapon.Fists), 25, 10); ok {
		t.Fatal("fists reach no target 15 off")
	}
}

// TestTakeCoverDoesNotFlap: a drafted rifleman steps once into adjacent
// cover, looks again only after CoverHold, and never steps back out to an
// equal tile; not against an unarmed target, nor holding position.
func TestTakeCoverDoesNotFlap(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.build(50, construction.Sandbags, 12, 10)
	f.build(51, construction.Sandbags, 12, 12)
	f.pawn(1, 10, 11, weapon.Rifle)
	f.raider(2, 25, 11, weapon.Rifle)
	d := weapon.DefOf(weapon.Rifle)
	moves := 0
	for i := 0; i < 3*CoverHold; i++ {
		p := f.pos[1]
		if c.Tactics.TakeCover(1, 2, p[0], p[1], d, 25, 11) {
			moves++
			j := f.jobs[1]
			if j.State = job.Goto; !c.Tactics.Covering(1, j) {
				t.Fatal("walking into the chosen cover")
			}
			f.pos[1] = [2]int{j.TX, j.TY} // arrives at once
		}
		f.now++
	}
	if moves != 1 {
		t.Fatalf("stepped into cover %d times", moves)
	}
	if p := f.pos[1]; p[0] != 11 {
		t.Fatalf("beside the sandbags: %v", p)
	}
	f2 := newFake(40, 20)
	c2 := f2.combat()
	f2.build(50, construction.Sandbags, 12, 11)
	f2.pawn(1, 10, 11, weapon.Rifle)
	f2.raider(2, 25, 11, weapon.Fists)
	if c2.Tactics.TakeCover(1, 2, 10, 11, d, 25, 11) {
		t.Fatal("an unarmed target: no cover needed")
	}
	f2.weapons[2] = weapon.Rifle
	f2.now += CoverHold
	c2.Drafts.SetHold(1, true)
	if c2.Tactics.TakeCover(1, 2, 10, 11, d, 25, 11) {
		t.Fatal("holding position: no step")
	}
}

// TestDangerFromGrenade: a grenade in flight endangers the tiles its blast
// reaches, not those beyond.
func TestDangerFromGrenade(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	if _, _, _, ok := c.Tactics.Danger(20, 10); ok {
		t.Fatal("nothing flies")
	}
	c.Shots.Lob(1, weapon.FragGrenade, 10, 10, 20, 10, 20, 10)
	f.now++ // the danger list is rebuilt once a tick
	lx, ly, reach, ok := c.Tactics.Danger(21, 10)
	if !ok || lx != 20 || ly != 10 || reach < 2 {
		t.Fatalf("beside the landing: %d %d %d %v", lx, ly, reach, ok)
	}
	if _, _, _, ok := c.Tactics.Danger(30, 10); ok {
		t.Fatal("far off: safe")
	}
	c.Shots.Lob(1, weapon.Rifle, 10, 10, 30, 10, 30, 10) // not explosive
	f.now++
	if _, _, _, ok := c.Tactics.Danger(30, 10); ok {
		t.Fatal("a bullet is no blast")
	}
}

// TestBlockAndBash: the first shut door on a breach route blocks a
// mechanoid; blows harm it beside it, not from afar.
func TestBlockAndBash(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.beast(1, 5, 10, animal.State{Species: animal.Harrow})
	f.build(50, construction.Door, 8, 10)
	f.route = []pathing.Point{{X: 6, Y: 10}, {X: 7, Y: 10}, {X: 8, Y: 10}, {X: 9, Y: 10}}
	b, sx, sy, ok := c.Tactics.Block(1, 9, 10)
	if !ok || b != 50 || sx != 7 || sy != 10 {
		t.Fatalf("blocked by the door from (7,10): %d (%d,%d) %v", b, sx, sy, ok)
	}
	d := weapon.DefOf(weapon.MechBlades)
	if c.Tactics.Bash(1, 50, d) {
		t.Fatal("out of touch")
	}
	f.pos[1] = [2]int{7, 10}
	if !c.Tactics.Bash(1, 50, d) || f.damage[50] != d.Damage || len(f.traces) != 1 {
		t.Fatalf("a blow on the door: %v", f.damage)
	}
	if !c.Tactics.Bash(1, 50, weapon.DefOf(weapon.Rifle)) || f.damage[50] != d.Damage+buttDamage {
		t.Fatal("a gun is swung")
	}
	f.noRoute = true
	if _, _, _, ok := c.Tactics.Block(1, 9, 10); ok {
		t.Fatal("no route: nothing to bash")
	}
	if c.Tactics.Bash(1, 77, d) {
		t.Fatal("a building that is gone")
	}
}

// TestColonistsInReach: only standing colonists near.
func TestColonistsInReach(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Rifle)
	f.pawn(2, 30, 10, weapon.Rifle)
	f.beast(3, 11, 10, animal.State{Species: animal.TimberWolf})
	got := c.Tactics.Colonists(10, 10, 5)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("colonists %v", got)
	}
}

// TestThreatResponseSetting: the setting, its default, its effect on
// fighting back and the engage reach (wiki: Threat response).
func TestThreatResponseSetting(t *testing.T) {
	t.Parallel()
	f := newFake(40, 20)
	d := f.combat().Drafts
	if d.Response(1) != RespondDefault || !d.Fights(1, true) || d.Fights(1, false) {
		t.Fatal("default: an armed, able colonist fights; others run")
	}
	d.SetResponse(1, RespondFlee)
	if d.Fights(1, true) || d.Response(1).String() != "flee" {
		t.Fatal("set to flee, it runs")
	}
	d.SetResponse(1, RespondAttack)
	if !d.Fights(1, true) || d.Fights(1, false) || RespondAttack.String() != "attack" {
		t.Fatal("set to attack: only armed and able")
	}
	d.SetResponse(1, RespondIgnore+1) // unknown: ignored
	if d.Response(1) != RespondAttack || RespondIgnore.String() != "ignore" || RespondDefault.String() != "default" {
		t.Fatal("an unknown response is ignored")
	}
	d.SetResponse(1, RespondDefault)
	if _, ok := d.set[1]; ok {
		t.Fatal("the default keeps no record")
	}
	if EngageReach(1) != EngageMelee || EngageReach(10) != 7 || EngageReach(100) != EngageRanged {
		t.Fatal("engage reach: 8 melee, 70 % of a gun's range up to 20")
	}
}

// TestDefendResponds: set to attack, an armed colonist goes for a
// raider in reach that is not shooting at it; set to ignore it does
// nothing; unarmed (flee) it runs from that raider.
func TestDefendResponds(t *testing.T) {
	t.Parallel()
	f := newFake(60, 30)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Rifle)
	f.raider(2, 20, 10, weapon.Rifle)
	for (f.now+1)%foeEvery != 0 { // the foe scan's tick for id 1
		f.now++
	}
	if !c.Guard.Defend(1, job.Record{}) {
		t.Fatal("armed: it acts on the raider in reach")
	}
	if j := f.jobs[1]; !(j.Kind == job.Attack && j.Target == 2 || j.Kind == job.Move) {
		t.Fatalf("attack (or a step into cover): %+v", j)
	}
	c.Drafts.SetResponse(1, RespondIgnore)
	if c.Guard.Defend(1, job.Record{}) {
		t.Fatal("set to ignore: nothing")
	}
	c.Drafts.SetResponse(1, RespondDefault)
	f.weapons[1] = weapon.Fists
	f.started = nil
	if !c.Guard.Defend(1, job.Record{}) || len(f.started) != 1 || f.started[0].Kind != job.Move || f.dropped == 0 {
		t.Fatalf("unarmed: it runs from the raider, %+v", f.started)
	}
	if f.started[0].TX >= 10 {
		t.Fatalf("away from the raider: %+v", f.started[0])
	}
	if _, ok := f.calm[1]; !ok {
		t.Fatal("it calms down only after")
	}
}

// TestHuntWithAnyGun: a hunter without a gun fetches any ranged weapon
// but an explosive one (CBT-4, wiki: Hunting).
func TestHuntWithAnyGun(t *testing.T) {
	t.Parallel()
	if !Hunter(weapon.Rifle) || Hunter(weapon.FragGrenade) || Hunter(weapon.Fists) || Hunter(weapon.Knife) {
		t.Fatal("hunting weapons: guns and bows, no explosives, no blades")
	}
	if !huntArm(item.ShortBow) || !huntArm(item.Revolver) || huntArm(item.Knife) || huntArm(item.Wood) {
		t.Fatal("a hunter fetches a bow or a revolver, not a knife or wood")
	}
	f := newFake(40, 20)
	c := f.combat()
	f.pawn(1, 10, 10, weapon.Fists)
	f.beast(2, 20, 10, animal.State{Species: animal.TimberWolf, Designated: true})
	f.marked = true
	f.free[item.Revolver] = [3]int{77, 3, 3}
	if !c.Hunting.Assign(1) || f.jobs[1].Kind != job.Equip || f.jobs[1].Target != 77 {
		t.Fatalf("fetches the revolver: %+v", f.jobs[1])
	}
}
