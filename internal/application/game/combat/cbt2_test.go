package combat

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/weather"
)

// CBT-2 unit tests: the melee curve and dodge, tools and the gun's stock,
// straight-line ranges, weather, downed targets, auto-engage, the
// auto-undraft timer, dropping and stowing, grenade walk-ins, experience.

// brawl strikes n times with a seeded roll and counts the blows that hit.
func brawl(f *fakeHost, s *Strikes, n int) int {
	r := rand.New(rand.NewPCG(7, 11))
	before := len(f.hurts)
	for i := 0; i < n; i++ {
		s.StrikeRoll(1, 2, weapon.Knife, r.IntN)
	}
	return len(f.hurts) - before
}

// TestSkill20MeleeHitsNinetyAndDodges: a skill-20 fighter hits about
// 90 % of the time; a skill-20 defender dodges 30 % of what would land.
func TestSkill20MeleeHitsNinetyAndDodges(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	f.melee[1] = attack.Fresh(20)
	const n = 4000
	if got := brawl(f, s, n); got < n*88/100 || got > n*92/100 {
		t.Fatalf("skill 20 hits %d of %d, want about 90 %%", got, n)
	}
	f.melee[2] = attack.Fresh(20)
	if got := brawl(f, s, n); got < n*60/100 || got > n*66/100 {
		t.Fatalf("against a skill-20 dodger %d of %d, want about 63 %%", got, n)
	}
	m := attack.Fresh(20)
	m.Down = true
	f.melee[2] = m
	if got := brawl(f, s, n); got < n*88/100 {
		t.Fatalf("a downed defender never dodges: %d of %d", got, n)
	}
	f.melee[1] = attack.Fresh(0)
	f.melee[2] = attack.Fresh(0)
	if got := brawl(f, s, n); got < n*47/100 || got > n*53/100 {
		t.Fatalf("skill 0 hits %d of %d, want about 50 %%", got, n)
	}
}

// TestMeleeDamageVaries: a blow lands for x0.8..1.2 of its tool.
func TestMeleeDamageVaries(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	for _, c := range []struct{ roll, want int }{{0, 10}, {200, 12}, {400, 14}} {
		f.rolls = []int{0, 0, c.roll} // the blade, a hit, the damage
		s.Strike(1, 2, weapon.Knife)
		if got := f.traces[len(f.traces)-1].Damage; got != c.want {
			t.Fatalf("damage roll %d: %d, want %d", c.roll, got, c.want)
		}
	}
}

// TestGunStockWhenAdjacent: a rifleman beside its target clubs it with
// the stock (no shot flies), and its next blow follows the stock's
// cooldown; an animal never does.
func TestGunStockWhenAdjacent(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	c := f.combat()
	f.pawn(1, 5, 5, weapon.Rifle)
	f.raider(2, 6, 6, weapon.Fists)
	f.rolls = []int{0, 0, 200} // the stock, a hit, x1.0
	c.Strikes.Strike(1, 2, weapon.Rifle)
	tr := f.traces[len(f.traces)-1]
	if c.Shots.Len() != 0 || !tr.Melee || tr.Weapon != "rifle" || tr.Damage != 9 || tr.Stage != projectile.Whole {
		t.Fatalf("the stock: shots %d trace %+v", c.Shots.Len(), tr)
	}
	if h := f.hurts[len(f.hurts)-1]; h != "2-9-1-"+damage.FormBlunt.String() {
		t.Fatalf("a blunt blow: %s", h)
	}
	if r, _ := c.Strikes.Report(1, 2); !r.Melee || r.Chance != attack.HitChanceOf(f.Melee(1)) {
		t.Fatalf("the tooltip reports the blow: %+v", r)
	}
	j := job.Record{Kind: job.Attack, Target: 2, State: job.Do, WorkLeft: 1}
	c.Strikes.Do(1, j, f.now)
	if j := f.jobs[1]; j.WorkLeft != weapon.ToolsOf(weapon.Rifle)[0].Cooldown || j.Count != 0 {
		t.Fatalf("the stock's cooldown: %+v", j)
	}
	f.pos[2] = [2]int{9, 5}
	c.Strikes.Strike(1, 2, weapon.Rifle)
	if c.Shots.Len() != 1 {
		t.Fatal("two tiles off it shoots")
	}
	// an animal with no tools of its own: one attack, its own profile
	f.beast(3, 1, 1, animal.State{Species: animal.Deer})
	f.pawn(4, 2, 1, weapon.Fists)
	f.rolls = nil
	c.Strikes.Strike(3, 4, weapon.Fists)
	if tr := f.traces[len(f.traces)-1]; !tr.Melee || !tr.Hit {
		t.Fatalf("a beast's blow: %+v", tr)
	}
}

// TestDiagonalRangeEqualsStraight: a rifle reaches 36 tiles straight and
// 26 on each axis (36.8 tiles), never 27 (38.2).
func TestDiagonalRangeEqualsStraight(t *testing.T) {
	t.Parallel()
	f := newFake(80, 80)
	c := f.combat()
	f.pawn(1, 0, 0, weapon.Rifle)
	f.raider(2, 36, 0, weapon.Fists)
	d := weapon.DefOf(weapon.Rifle)
	if !c.Sight.CanHit(0, 0, d, 36, 0) || c.Sight.CanHit(0, 0, d, 37, 0) {
		t.Fatal("straight")
	}
	if !c.Sight.CanHit(0, 0, d, 26, 26) || c.Sight.CanHit(0, 0, d, 27, 27) {
		t.Fatal("diagonal")
	}
	f.pos[2] = [2]int{26, 26}
	if r, _ := c.Strikes.Report(1, 2); !r.InRange || r.Dist != 37 {
		t.Fatalf("report on the diagonal %+v", r)
	}
	f.pos[2] = [2]int{27, 27}
	if r, _ := c.Strikes.Report(1, 2); r.InRange {
		t.Fatalf("out of range on the diagonal %+v", r)
	}
}

// TestWeatherLowersHitChance: rain takes a fifth, a blizzard 30 %, a roof
// over both shields the shot.
func TestWeatherLowersHitChance(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 2, 5, weapon.Rifle)
	f.raider(2, 12, 5, weapon.Fists)
	clear, _ := c.Strikes.Report(1, 2)
	f.sky = weather.Rain
	rain, _ := c.Strikes.Report(1, 2)
	f.sky = weather.Blizzard
	bliz, _ := c.Strikes.Report(1, 2)
	if rain.Weather != attack.RainFactor || bliz.Weather != attack.BlizzardFactor || !(bliz.Chance < rain.Chance && rain.Chance < clear.Chance) {
		t.Fatalf("clear %+v rain %+v blizzard %+v", clear, rain, bliz)
	}
	f.roofed[[2]int{2, 5}] = true
	if r, _ := c.Strikes.Report(1, 2); r.Weather != attack.BlizzardFactor {
		t.Fatal("one end under the open sky: the weather counts")
	}
	f.roofed[[2]int{12, 5}] = true
	if r, _ := c.Strikes.Report(1, 2); r.Weather != attack.ClearFactor || r.Chance != clear.Chance {
		t.Fatalf("both under a roof: %+v", r)
	}
}

// TestDownedOnlyByOrder: a downed raider is no target for an attack of
// the colonist's own, but an ordered one keeps at it.
func TestDownedOnlyByOrder(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	s := f.combat().Strikes
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	f.raiders[2], f.lying[2] = false, true
	if _, _, ok := s.Target(2); ok || !s.Ordered(2) {
		t.Fatal("downed: no target, but may be ordered")
	}
	s.Do(1, job.Record{Kind: job.Attack, Target: 2, State: job.Do, WorkLeft: 1}, f.now)
	if f.jobs[1].State != job.Complete {
		t.Fatal("an attack of its own stops at a downed raider")
	}
	f.rolls = nil
	s.Do(1, job.Record{Kind: job.Attack, Target: 2, State: job.Do, WorkLeft: 1, Forced: true}, f.now)
	if j := f.jobs[1]; j.State != job.Do || len(f.hurts) != 1 {
		t.Fatalf("an ordered attack strikes: %+v %v", j, f.hurts)
	}
	f.lying[2] = false
	if s.Ordered(2) {
		t.Fatal("gone: no order")
	}
}

// TestAutoEngageAnyHostile: a manhunter, a mechanoid and a hive insect
// are engaged without attacking anyone; a calm wild animal is not.
func TestAutoEngageAnyHostile(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		a    animal.State
		want bool
	}{
		{"calm", animal.State{Species: animal.Deer}, false},
		{"manhunter", animal.State{Species: animal.Deer, Manhunter: 5000}, true},
		{"tame", animal.State{Species: animal.Deer, Manhunter: 5000, Tame: true}, false},
		{"mechanoid", animal.State{Species: animal.Harrow}, true},
		{"insect", animal.State{Species: animal.HiveCrawler}, true},
		{"dormant", animal.State{Species: animal.Harrow, Dormant: true}, false},
	} {
		f := newFake(40, 10)
		s := f.combat().Strikes
		f.pawn(1, 2, 5, weapon.Rifle)
		f.drafted[1] = true
		f.beast(3, 12, 5, c.a)
		s.AutoEngage(1)
		if got := f.jobs[1].Kind == job.Attack && f.jobs[1].Target == 3; got != c.want {
			t.Errorf("%s: engaged %v, want %v", c.name, got, c.want)
		}
		if got := s.Threatened(f.now); got != c.want {
			t.Errorf("%s: threat %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAutoUndraftAfterQuiet: released after AutoUndraft idle ticks with
// no threat; an order or a threat starts the quiet over; once released
// it stays released (no flap).
func TestAutoUndraftAfterQuiet(t *testing.T) {
	t.Parallel()
	f := newFake(60, 60)
	c := f.combat()
	f.pawn(1, 5, 5, weapon.Fists)
	f.drafted[1] = true
	idle := func(n int) {
		for i := 0; i < n && f.drafted[1]; i++ {
			f.now++
			c.Strikes.AutoEngage(1)
		}
	}
	idle(AutoUndraft - 1)
	if !f.drafted[1] {
		t.Fatal("released early")
	}
	f.now += 5 // busy: an order ran for five ticks
	idle(AutoUndraft - 1)
	if !f.drafted[1] || len(f.undrafted) != 0 {
		t.Fatal("an order starts the quiet over")
	}
	// a raider on the map for a moment, out of reach: the quiet starts over
	f.raider(9, 59, 59, weapon.Fists)
	idle(threatEvery)
	f.raiders[9] = false
	idle(AutoUndraft - threatEvery)
	if !f.drafted[1] {
		t.Fatal("a threat starts the quiet over")
	}
	idle(AutoUndraft)
	if f.drafted[1] || len(f.undrafted) != 1 {
		t.Fatalf("released after the quiet: %v", f.undrafted)
	}
	for i := 0; i < 3*AutoUndraft; i++ { // undrafted, it is never idle-drafted again
		f.now++
		if f.drafted[1] {
			c.Strikes.AutoEngage(1)
		}
	}
	if f.drafted[1] || len(f.undrafted) != 1 {
		t.Fatal("no flap after the release")
	}
	if s := c.Drafts.set[1]; s.QuietFrom != 0 || s.IdleAt != 0 {
		t.Fatalf("the timer resets on release: %+v", s)
	}
}

// TestDropThenReEquip: a drop puts the weapon down and draws the spare;
// a stow swaps; with nothing wielded a drop puts the spare down; the
// spare survives a save and falls with its carrier.
func TestDropThenReEquip(t *testing.T) {
	t.Parallel()
	f := newFake(20, 10)
	c := f.combatArmed()
	d := c.Drafts
	f.pawn(1, 5, 5, weapon.Rifle)
	f.wield[1] = item.Rifle
	d.Apply(command.Command{Kind: command.DropWeapon, Pawn: 1})
	if _, armed := f.wield[1]; armed || len(f.putDown) != 1 || f.putDown[0] != "5,5:rifle:1" || f.weapons[1] != weapon.Fists {
		t.Fatalf("dropped: %v %v", f.wield, f.putDown)
	}
	fa := fakeArms{f}
	fa.Wield(1, item.Knife) // re-equipped (the game's equip job)
	d.Apply(command.Command{Kind: command.DropWeapon, Pawn: 1, Enabled: true})
	if k, n, ok := d.Spare(1); !ok || k != item.Knife || n != 1 || f.weapons[1] != weapon.Fists {
		t.Fatalf("stowed: %v %d %v", k, n, ok)
	}
	fa.Wield(1, item.Rifle)
	d.Apply(command.Command{Kind: command.DropWeapon, Pawn: 1})
	if f.wield[1] != item.Knife || f.weapons[1] != weapon.Knife || len(f.putDown) != 2 {
		t.Fatalf("drop draws the spare: %v %v", f.wield, f.putDown)
	}
	d.Apply(command.Command{Kind: command.DropWeapon, Pawn: 1, Enabled: true}) // stow the knife
	s, q := d.export()
	d2 := &Drafts{h: f, arms: fa}
	d2.importDrafts(s, q)
	if k, _, ok := d2.Spare(1); !ok || k != item.Knife {
		t.Fatal("the spare survives a save")
	}
	d2.Apply(command.Command{Kind: command.DropWeapon, Pawn: 1}) // unarmed: the spare goes down
	if _, _, ok := d2.Spare(1); ok || f.putDown[len(f.putDown)-1] != "5,5:knife:1" {
		t.Fatalf("unarmed drop: %v", f.putDown)
	}
	// a grenade belt stows whole
	fa.Wield(1, item.FragGrenades)
	d.Carry(1, item.FragGrenades, 3)
	d.DropWeapon(1, true)
	if k, n, ok := d.Spare(1); !ok || k != item.FragGrenades || n != 3 {
		t.Fatalf("belt stowed: %v %d", k, n)
	}
	d.DropWeapon(1, true)
	if f.wield[1] != item.FragGrenades || d.Grenades(1) != 3 {
		t.Fatal("belt drawn")
	}
	d.DropWeapon(1, true)
	d.DropSpare(1)
	if _, _, ok := d.Spare(1); ok || f.putDown[len(f.putDown)-1] != "5,5:frag grenades:3" {
		t.Fatalf("the dead drop their spare: %v", f.putDown)
	}
	f.raider(2, 7, 7, weapon.Club)
	f.wield[2] = item.Club
	d.DropWeapon(2, false)
	if f.wield[2] != item.Club {
		t.Fatal("raiders take no order")
	}
}

// TestGrenadeOrderWalksThenThrows: out of range it walks; once in range
// it stops, waits out the warmup and throws; another order drops it.
func TestGrenadeOrderWalksThenThrows(t *testing.T) {
	t.Parallel()
	f := newFake(60, 20)
	c := f.combatArmed()
	d := c.Drafts
	f.pawn(1, 5, 5, weapon.FragGrenade)
	d.Carry(1, item.FragGrenades, 2)
	if !d.Throw(1, 40, 5) {
		t.Fatal("taken: it walks")
	}
	walk := f.jobs[1]
	if walk.Kind != job.Move || !walk.Forced || walk.TX != 40 || len(d.queue) != 1 || !d.queue[0].Walk {
		t.Fatalf("walking: %+v %+v", walk, d.queue)
	}
	walk.State = job.Goto
	f.jobs[1] = walk
	d.phase(f.now)
	if len(d.queue) != 1 || !d.queue[0].Walk || c.Shots.Len() != 0 {
		t.Fatal("still too far")
	}
	f.pos[1] = [2]int{28, 5} // 12 tiles off
	interr := f.interr
	d.phase(f.now + 1)
	if len(d.queue) != 1 || d.queue[0].Walk || f.interr != interr+1 {
		t.Fatalf("in range it stops: %+v", d.queue)
	}
	warm := uint64(weapon.DefOf(weapon.FragGrenade).Warmup)
	d.phase(f.now + warm)
	if c.Shots.Len() != 0 {
		t.Fatal("not before the warmup")
	}
	d.phase(f.now + 1 + warm)
	if c.Shots.Len() != 1 || d.Grenades(1) != 1 || len(d.queue) != 0 {
		t.Fatalf("thrown: %d shots, %d left", c.Shots.Len(), d.Grenades(1))
	}
	// another order replaces the walk: the throw is dropped
	f.pos[1] = [2]int{5, 5}
	d.Throw(1, 40, 5)
	f.jobs[1] = job.Record{Kind: job.Move, TX: 3, TY: 3, Forced: true, State: job.Goto}
	d.phase(f.now + 2*warm)
	if len(d.queue) != 0 {
		t.Fatal("a new order drops the walk")
	}
}

// TestCombatXPPerAttack: a melee hit teaches more than a miss, by the
// tool's cycle; a shot learns at the muzzle; animals learn nothing.
func TestCombatXPPerAttack(t *testing.T) {
	t.Parallel()
	f := newFake(40, 10)
	c := f.combat()
	f.pawn(1, 5, 5, weapon.Knife)
	f.raider(2, 6, 5, weapon.Fists)
	blade := weapon.ToolsOf(weapon.Knife)[0]
	f.rolls = []int{0, 0, 0}
	c.Strikes.Strike(1, 2, weapon.Knife)
	f.rolls = []int{0, 999}
	c.Strikes.Strike(1, 2, weapon.Knife)
	if len(f.xp) != 2 || f.xp[0] != attack.CombatXP(true, blade.Cooldown, 1) || f.xp[1] != attack.CombatXP(false, blade.Cooldown, 1) || f.xp[0] <= f.xp[1] {
		t.Fatalf("melee xp %v", f.xp)
	}
	f.pawn(3, 2, 2, weapon.Rifle)
	f.pos[2] = [2]int{20, 2}
	f.rolls = []int{0}
	c.Strikes.Strike(3, 2, weapon.Rifle)
	d := weapon.DefOf(weapon.Rifle)
	if len(f.xp) != 3 || f.xp[2] != attack.CombatXP(true, CycleTicks(d), 1) {
		t.Fatalf("shooting xp %v", f.xp)
	}
	f.rolls = []int{999, 0, 0}
	c.Strikes.Strike(3, 2, weapon.Rifle)
	if f.xp[3] != attack.CombatXP(false, CycleTicks(d), 1) {
		t.Fatalf("a wild shot is a miss: %v", f.xp)
	}
	f.beast(4, 7, 5, animal.State{Species: animal.TimberWolf})
	c.Strikes.Strike(4, 2, weapon.Fists)
	if len(f.xp) != 4 {
		t.Fatal("animals learn nothing")
	}
}
