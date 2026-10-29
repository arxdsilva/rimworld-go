package game

import (
	"bytes"
	"math"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

var pawnActions = map[string]bool{
	ActionAim: true, ActionMelee: true, ActionChop: true, ActionMine: true, ActionBuild: true,
	ActionDeconstruct: true, ActionRepair: true, ActionBreach: true, ActionSow: true, ActionHarvest: true,
	ActionCraft: true, ActionCook: true, ActionTailor: true, ActionButcher: true, ActionResearch: true,
	ActionTend: true, ActionRescue: true, ActionTame: true, ActionEat: true, ActionRefuel: true,
	ActionHaul: true, ActionHorseshoes: true, ActionClean: true, ActionDig: true, ActionBury: true, ActionCremate: true,
}

func TestActionVocabularyCoversJobs(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 8, 2)
	bench := g.layerBldgAt(8, 2, construction.LayerObject)
	do := func(k job.Kind) job.Record { return job.Record{Kind: k, State: job.Do, Carry: 1} }

	want := map[job.Kind]string{
		job.Construct: ActionBuild, job.Deconstruct: ActionDeconstruct, job.Cut: ActionChop,
		job.Mine: ActionMine, job.Sow: ActionSow, job.Harvest: ActionHarvest, job.Eat: ActionEat,
		job.Tend: ActionTend, job.Refuel: ActionRefuel, job.Horseshoes: ActionHorseshoes,
		job.Research: ActionResearch, job.Tame: ActionTame, job.Repair: ActionRepair,
		job.Breach: ActionBreach, job.Haul: ActionHaul, job.Clean: ActionClean,
	}
	for k, a := range want {
		if got := g.actionOf(id, do(k)); got != a {
			t.Errorf("%v: action %q, want %q", k, got, a)
		}
	}
	// Kinds without a hands-on action stay "".
	for _, k := range []job.Kind{job.None, job.Move, job.Sleep, job.Equip, job.Warm, job.Wander, job.Chat, job.Relax, job.FireSide, job.Hide} {
		if got := g.actionOf(id, do(k)); got != "" {
			t.Errorf("%v: action %q, want none", k, got)
		}
	}
	// Every job kind maps to "" or a C2 action.
	for k := job.None; k <= job.Clean; k++ {
		if got := g.actionOf(id, do(k)); got != "" && !pawnActions[got] {
			t.Errorf("%v: %q is not in the vocabulary", k, got)
		}
	}
	// Not yet working: walking to the job, or picking up the ingredient.
	if got := g.actionOf(id, job.Record{Kind: job.Construct, State: job.Goto}); got != "" {
		t.Errorf("walking to a build site acts %q", got)
	}
	if got := g.actionOf(id, job.Record{Kind: job.Bill, State: job.Do, Dest: bench}); got != "" {
		t.Errorf("fetching the ingredient acts %q", got)
	}
	if got := g.actionOf(id, job.Record{Kind: job.Haul, State: job.Goto}); got != "" {
		t.Errorf("walking empty-handed to a haul acts %q", got)
	}
	if got := g.actionOf(id, job.Record{Kind: job.Haul, State: job.Goto, Carry: 1}); got != ActionHaul {
		t.Errorf("hauling a load acts %q", got)
	}
	// Downed pawns do nothing.
	g.Injure(id, 200)
	if got := g.actionOf(id, do(job.Cut)); got != "" {
		t.Errorf("a dead pawn acts %q", got)
	}
}

func TestActionAttackFollowsWeapon(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for _, st := range []job.State{job.Goto, job.Do} {
		j := job.Record{Kind: job.Attack, State: st}
		for _, k := range []job.Kind{job.Attack, job.Hunt} {
			j.Kind = k
			if got := g.actionOf(id, j); got != ActionMelee {
				t.Errorf("%v/%v unarmed = %q, want melee", k, st, got)
			}
		}
	}
	for _, w := range []item.Kind{item.Club, item.Knife, item.Spear} {
		g.Arm(id, w)
		if got := g.actionOf(id, job.Record{Kind: job.Hunt, State: job.Goto}); got != ActionMelee {
			t.Errorf("%v = %q, want melee", w, got)
		}
	}
	g.Arm(id, item.Rifle)
	for _, k := range []job.Kind{job.Attack, job.Hunt} {
		for _, st := range []job.State{job.Goto, job.Do} {
			if got := g.actionOf(id, job.Record{Kind: k, State: st}); got != ActionAim {
				t.Errorf("rifle %v/%v = %q, want aim", k, st, got)
			}
		}
	}
	if got := g.actionOf(id, job.Record{Kind: job.Attack, State: job.Complete}); got != "" {
		t.Errorf("a finished attack acts %q", got)
	}
}

func TestActionRescueAndBenches(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	patient, _ := g.SpawnNamedPawn(6, 5, "Bo")
	// (benches are placed along y=1, x=2..16)
	if got := g.actionOf(id, job.Record{Kind: job.Rescue, State: job.Do}); got != ActionRescue {
		t.Errorf("lifting a patient = %q", got)
	}
	g.care.Carry.Pick(id, patient)
	if got := g.actionOf(id, job.Record{Kind: job.Rescue, State: job.Goto}); got != ActionRescue {
		t.Errorf("carrying a patient to bed = %q", got)
	}
	g.care.Carry.Drop(id)
	if got := g.actionOf(id, job.Record{Kind: job.Rescue, State: job.Goto}); got != "" {
		t.Errorf("walking to the patient = %q", got)
	}

	benches := []struct {
		kind construction.Kind
		x    int
		want string
	}{
		{construction.Workbench, 2, ActionCraft},
		{construction.Stove, 4, ActionCook},
		{construction.Campfire, 6, ActionCook},
		{construction.DryingRack, 8, ActionCook},
		{construction.TailoringBench, 10, ActionTailor},
		{construction.ButcherTable, 12, ActionButcher},
		// PLAY-021: an electric bench works its base bench's recipes
		{construction.ElectricStove, 14, ActionCook},
		{construction.ElectricTailoringBench, 16, ActionTailor},
	}
	for _, b := range benches {
		placeBuilt(t, g, b.kind, construction.South, b.x, 1)
		bid := g.layerBldgAt(b.x, 1, construction.LayerObject)
		if bid == 0 {
			t.Fatalf("%v not placed", b.kind)
		}
		j := job.Record{Kind: job.Bill, State: job.Do, Carry: 1, Dest: bid}
		if got := g.actionOf(id, j); got != b.want {
			t.Errorf("%v bill = %q, want %q", b.kind, got, b.want)
		}
	}
}

func TestActionProgress(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	plantID, err := g.SpawnPlant(9, 9)
	if err != nil {
		t.Fatal(err)
	}
	g.placeFrame(construction.Wall, 12, 4)
	frameID := g.frameIDs[0]
	fr, _ := g.frames.Get(frameID)
	placeBuilt(t, g, construction.Workbench, construction.South, 15, 2)
	bench := g.layerBldgAt(15, 2, construction.LayerObject)
	g.Enqueue(commandAddBill(15, 2, recipe.Club))
	g.Step(1)

	cases := []struct {
		name string
		j    job.Record
	}{
		{"cut", job.Record{Kind: job.Cut, Target: plantID, TX: 9, TY: 9}},
		{"harvest", job.Record{Kind: job.Harvest, Target: plantID, TX: 9, TY: 9}},
		{"mine", job.Record{Kind: job.Mine, TX: 3, TY: 3, DestX: 4, DestY: 3}},
		{"sow", job.Record{Kind: job.Sow, TX: 3, TY: 3}},
		{"research", job.Record{Kind: job.Research}},
		{"tend", job.Record{Kind: job.Tend, Target: 9999}},
		{"tame", job.Record{Kind: job.Tame}},
		{"construct", job.Record{Kind: job.Construct, Target: frameID, TX: 12, TY: 4}},
		{"bill", job.Record{Kind: job.Bill, Dest: bench, BillIndex: 0, Carry: 1}},
		{"eat", job.Record{Kind: job.Eat}},
	}
	totals := map[string]int{
		"cut": plant.DefOf(plant.BerryBush).CutWork, "harvest": plant.DefOf(plant.BerryBush).HarvestWork, "mine": MineWork, "sow": SowWork, "research": researchSession,
		"tend": TendWork, "tame": animal.TameWork, "construct": fr.WorkLeft, "bill": recipe.DefOf(recipe.Club).Work, "eat": 20,
	}
	for _, c := range cases {
		c.j.State = job.Do
		tot := totals[c.name]
		if tot <= 1 {
			t.Fatalf("%s: unknown total %d", c.name, tot)
		}
		if got := g.progressOf(id, withLeft(c.j, tot)); got != 0 {
			t.Errorf("%s at start = %v, want 0", c.name, got)
		}
		if got := g.progressOf(id, withLeft(c.j, tot/2)); math.Abs(got-0.5) > 0.02 {
			t.Errorf("%s halfway = %v, want ~0.5", c.name, got)
		}
		if got := g.progressOf(id, withLeft(c.j, 0)); got != 1 {
			t.Errorf("%s done = %v, want 1", c.name, got)
		}
		if got := g.progressOf(id, withLeft(c.j, tot*3)); got != 0 {
			t.Errorf("%s clamps below 0: %v", c.name, got)
		}
	}
	// Unknown totals read -1.
	for _, k := range []job.Kind{job.Haul, job.Rescue, job.Attack, job.Breach, job.Repair, job.Move} {
		if got := g.progressOf(id, job.Record{Kind: k, State: job.Do, WorkLeft: 10}); got != -1 {
			t.Errorf("%v progress = %v, want -1", k, got)
		}
	}
	// A self-tend takes half again as long.
	self := job.Record{Kind: job.Tend, State: job.Do, Target: id, WorkLeft: TendWork * 3 / 2}
	if got := g.progressOf(id, self); got != 0 {
		t.Errorf("self tend start = %v", got)
	}
}

func withLeft(j job.Record, n int) job.Record { j.WorkLeft = n; return j }

func TestActionProgressRisesDuringRealWork(t *testing.T) {
	g := New(Config{Seed: 2, Width: 24, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	patient, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.Injure(patient, 20)
	g.jobs.Set(id, job.Record{Kind: job.Tend, State: job.Do, Target: patient, TX: 6, TY: 5, WorkLeft: TendWork})
	last := -1.0
	for i := 0; i < 100; i++ {
		g.Step(1)
		pv := g.Snapshot()[0]
		if pv.Action != ActionTend {
			break
		}
		if pv.ActionProgress < last {
			t.Fatalf("progress fell: %v -> %v", last, pv.ActionProgress)
		}
		last = pv.ActionProgress
	}
	if last <= 0 {
		t.Fatalf("tending never progressed: %v", last)
	}
	if pv := g.Snapshot()[0]; pv.Action == ActionTend && pv.ActTarget != patient {
		t.Fatalf("ActTarget = %d, want %d", pv.ActTarget, patient)
	}
}

// attackSetup returns a game with an armed attacker and a defenceless
// colonist target the given distance away.
func attackSetup(t *testing.T, w item.Kind, dist int) (g *Game, att, tgt entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 4, Width: 30, Height: 12})
	att, _ = g.SpawnNamedPawn(5, 5, "Ada")
	tgt, _ = g.SpawnNamedPawn(5+dist, 5, "Bo")
	g.Arm(att, w)
	g.jobs.Set(tgt, job.Record{Kind: job.Sleep, State: job.Do, WorkLeft: 1 << 20, TX: 5 + dist, TY: 5})
	d := weapon.DefOf(weaponKindOf(w))
	g.startJob(att, job.Record{Kind: job.Attack, Target: tgt, TX: 5 + dist, TY: 5, WorkLeft: aimTicks(d)})
	return
}

func pawnView(g *Game, id entity.EntityID) PawnView {
	for _, p := range g.Snapshot() {
		if p.ID == id {
			return p
		}
	}
	return PawnView{}
}

func TestAttackCycle(t *testing.T) {
	for _, tc := range []struct {
		w    item.Kind
		dist int
	}{{item.Rifle, 5}, {item.Club, 1}, {item.Knife, 1}, {item.Spear, 1}} {
		g, att, tgt := attackSetup(t, tc.w, tc.dist)
		d := weapon.DefOf(weaponKindOf(tc.w))
		strikes, prev := 0, -1.0
		for i := 0; i < 3*d.Cooldown && strikes < 3; i++ {
			g.Step(1)
			pv := pawnView(g, att)
			struck := false
			for _, tr := range g.World().Traces {
				if tr.Attacker == att && tr.Target == tgt && tr.Age == 1 {
					struck = true
				}
			}
			switch {
			case struck:
				strikes++
				if pv.AttackCycle != 1 {
					t.Fatalf("%v: cycle on strike tick = %v, want exactly 1", tc.w, pv.AttackCycle)
				}
				prev = 1
			case pv.AttackCycle >= 1:
				t.Fatalf("%v: cycle %v reached 1 without a strike", tc.w, pv.AttackCycle)
			case pv.AttackCycle < 0:
				t.Fatalf("%v: cycle %v below 0", tc.w, pv.AttackCycle)
			default:
				if prev == 1 && pv.AttackCycle > 0.1 {
					t.Fatalf("%v: cycle after the strike = %v, want it reset", tc.w, pv.AttackCycle)
				}
				if prev >= 0 && prev < 1 && pv.AttackCycle < prev-1e-9 {
					t.Fatalf("%v: cycle fell %v -> %v before the strike", tc.w, prev, pv.AttackCycle)
				}
				prev = pv.AttackCycle
			}
		}
		if strikes < 2 {
			t.Fatalf("%v: only %d strikes", tc.w, strikes)
		}
	}
}

func TestAttackCycleIdleIsZero(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	if pv := pawnView(g, id); pv.AttackCycle != 0 || pv.Action != "" {
		t.Fatalf("idle pawn = %+v", pv)
	}
}

func TestTraceCarriesAttackerTargetWeapon(t *testing.T) {
	// Pawn vs pawn.
	g, att, tgt := attackSetup(t, item.Rifle, 4)
	var tr TraceView
	for i := 0; i < 200 && tr.Attacker == 0; i++ {
		g.Step(1)
		for _, x := range g.World().Traces {
			if x.Age == 1 {
				tr = x
			}
		}
	}
	if tr.Attacker != att || tr.Target != tgt || tr.Weapon != "rifle" {
		t.Fatalf("pawn trace = %+v (att %d tgt %d)", tr, att, tgt)
	}

	// Animal bite.
	g = New(Config{Seed: 5, Width: 30, Height: 12})
	victim, _ := g.SpawnNamedPawn(10, 6, "Bait")
	wolf, _ := g.SpawnAnimal("ridgewolf", 11, 6)
	a, _ := g.animals.Get(wolf)
	a.Mode, a.Target = animal.Attack, victim
	g.animals.Set(wolf, a)
	tr = TraceView{}
	for i := 0; i < 200 && tr.Attacker == 0; i++ {
		g.Step(1)
		for _, x := range g.World().Traces {
			if x.Age == 1 {
				tr = x
			}
		}
	}
	if tr.Attacker != wolf || tr.Target != victim || tr.Weapon != "bite" {
		t.Fatalf("bite trace = %+v (wolf %d victim %d)", tr, wolf, victim)
	}

	// A breach blow on a building has no creature target.
	g = New(Config{Seed: 6, Width: 20, Height: 12})
	g.SpawnNamedPawn(2, 2, "Ada")
	placeBuilt(t, g, construction.Wall, construction.South, 8, 5)
	wall := g.layerBldgAt(8, 5, construction.LayerObject)
	g.RaidArmed(item.Club)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 7, Y: 5})
	g.doBreach(rid, job.Record{Kind: job.Breach, State: job.Do, Target: wall, DestX: 8, DestY: 5, TX: 7, TY: 5})
	ts := g.World().Traces
	if len(ts) == 0 {
		t.Fatal("no breach trace")
	}
	bt := ts[len(ts)-1]
	if !bt.Building || bt.Attacker != rid || bt.Target != 0 || bt.Weapon != "club" {
		t.Fatalf("breach trace = %+v", bt)
	}
}

func TestHitStampsVictim(t *testing.T) {
	// Pawn victim.
	g, att, tgt := attackSetup(t, item.Rifle, 4)
	var hitTick uint64
	for i := 0; i < 600 && hitTick == 0; i++ {
		g.Step(1)
		for _, x := range g.World().Traces {
			if x.Age == 1 && x.Hit && x.Target == tgt {
				hitTick = uint64(g.clock.Now()) - 1
			}
		}
	}
	if hitTick == 0 {
		t.Fatal("never hit")
	}
	pv := pawnView(g, tgt)
	ap, _ := g.positions.Get(att)
	if pv.HitTick != hitTick || pv.HitFromX != ap.X || pv.HitFromY != ap.Y || pv.HitDamage != weapon.DefOf(weapon.Rifle).Damage {
		t.Fatalf("victim = %+v, want tick %d from (%d,%d) dmg 14", pv, hitTick, ap.X, ap.Y)
	}
	if pawnView(g, att).HitTick != 0 {
		t.Fatal("the attacker was not hit")
	}

	// Animal victim.
	g = New(Config{Seed: 7, Width: 30, Height: 12})
	shooter, _ := g.SpawnNamedPawn(5, 5, "Ada")
	buck, _ := g.SpawnAnimal("dustbuck", 9, 5)
	g.Arm(shooter, item.Rifle)
	g.strikeUntilHit(t, shooter, buck, weapon.Rifle)
	var av AnimalView
	for _, x := range g.World().Animals {
		if x.ID == buck {
			av = x
		}
	}
	sp, _ := g.positions.Get(shooter)
	if av.HitTick == 0 || av.HitFromX != sp.X || av.HitFromY != sp.Y || av.HitDamage != 14 {
		t.Fatalf("animal = %+v", av)
	}
}

// strikeUntilHit calls strike until the target is hit (rng-driven).
func (g *Game) strikeUntilHit(t *testing.T, att, tgt entity.EntityID, wk weapon.Kind) {
	t.Helper()
	for i := 0; i < 200; i++ {
		before := len(g.traces)
		g.strike(att, tgt, wk)
		if len(g.traces) > before && g.traces[len(g.traces)-1].Hit {
			return
		}
	}
	t.Fatal("never hit")
}

func TestMissDoesNotStampVictim(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 12})
	shooter, _ := g.SpawnNamedPawn(5, 5, "Ada")
	target, _ := g.SpawnNamedPawn(6, 5, "Bo")
	for i := 0; i < 400; i++ {
		before := g.lastHit[target]
		g.strike(shooter, target, weapon.Fists)
		if !g.traces[len(g.traces)-1].Hit {
			if g.lastHit[target] != before {
				t.Fatal("a miss stamped the victim")
			}
			return
		}
	}
	t.Fatal("never missed")
}

func TestRaiderViewsCarryActions(t *testing.T) {
	g := New(Config{Seed: 6, Width: 20, Height: 12})
	g.SpawnNamedPawn(2, 2, "Ada")
	g.RaidArmed(item.Rifle)
	rid := g.raiderIDs[0]
	g.jobs.Set(rid, job.Record{Kind: job.Attack, State: job.Goto, Target: 1})
	var rv PawnView
	for _, p := range g.Snapshot() {
		if p.ID == rid {
			rv = p
		}
	}
	if rv.Action != ActionAim || rv.ActTarget != 1 || rv.ActionProgress != -1 {
		t.Fatalf("raider view = %+v", rv)
	}
}

func TestSaveKeepsStrikeAndHitStamps(t *testing.T) {
	g, att, tgt := attackSetup(t, item.Rifle, 4)
	for i := 0; i < 600 && g.lastHit[tgt].Tick == 0; i++ {
		g.Step(1)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "stamps"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.lastHit[tgt] != g.lastHit[tgt] || h.lastStrike[att] != g.lastStrike[att] || h.lastHit[tgt].Tick == 0 {
		t.Fatalf("stamps lost: %+v %+v", h.lastHit, h.lastStrike)
	}
}

func TestCarryingExposesItemKind(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	patient, _ := g.SpawnNamedPawn(6, 5, "Bo")
	if pawnView(g, id).Carrying != "" {
		t.Fatal("empty-handed")
	}
	sid, err := g.SpawnStack(item.Wood, 10, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(sid)
	st.Carrier = id
	g.stacks.Set(sid, st)
	g.jobs.Set(id, job.Record{Kind: job.Haul, State: job.Goto, Carry: sid, DestX: 9, DestY: 5, TX: 9, TY: 5})
	if got := pawnView(g, id).Carrying; got != "wood" {
		t.Fatalf("hauling wood: Carrying = %q", got)
	}
	if pv := pawnView(g, id); pv.Action != ActionHaul {
		t.Fatalf("Action = %q", pv.Action)
	}
	g.jobs.Set(id, job.Record{})
	g.care.Carry.Pick(id, patient)
	if got := pawnView(g, id).Carrying; got != "pawn" {
		t.Fatalf("rescuing: Carrying = %q", got)
	}
}

func TestAnimalActionAndCycle(t *testing.T) {
	g := New(Config{Seed: 5, Width: 30, Height: 12})
	victim, _ := g.SpawnNamedPawn(10, 6, "Bait")
	wolf, _ := g.SpawnAnimal("ridgewolf", 11, 6)
	buck, _ := g.SpawnAnimal("dustbuck", 25, 2)
	a, _ := g.animals.Get(wolf)
	a.Mode, a.Target = animal.Attack, victim
	g.animals.Set(wolf, a)
	view := func(id entity.EntityID) AnimalView {
		for _, x := range g.World().Animals {
			if x.ID == id {
				return x
			}
		}
		return AnimalView{}
	}
	if got := view(buck).Action; got != "" && got != ActionGraze {
		t.Fatalf("calm dustbuck = %q", got)
	}
	struck := 0
	for i := 0; i < 400 && struck < 2; i++ {
		g.Step(1)
		av := view(wolf)
		fresh := false
		for _, tr := range g.World().Traces {
			fresh = fresh || tr.Attacker == wolf && tr.Age == 1
		}
		if fresh {
			struck++
			if av.AttackCycle != 1 || av.Action != ActionBite {
				t.Fatalf("on the bite: %+v", av)
			}
		} else if av.AttackCycle >= 1 {
			t.Fatalf("cycle %v without a bite", av.AttackCycle)
		}
	}
	if struck < 2 {
		t.Fatal("the wolf never bit twice")
	}
	// A fleeing dustbuck flees; a hooved attacker kicks.
	b, _ := g.animals.Get(buck)
	b.Mode = animal.Flee
	g.animals.Set(buck, b)
	if got := view(buck).Action; got != ActionFlee {
		t.Fatalf("fleeing dustbuck = %q", got)
	}
	b.Mode, b.Target = animal.Attack, victim
	g.animals.Set(buck, b)
	bp, _ := g.positions.Get(victim)
	g.positions.Set(buck, Position{X: bp.X + 1, Y: bp.Y})
	if got := view(buck).Action; got != ActionKick {
		t.Fatalf("attacking dustbuck = %q", got)
	}
}

func TestKnifeSpearWeaponMapping(t *testing.T) {
	if weaponKindOf(item.Knife) != weapon.Knife || weaponKindOf(item.Spear) != weapon.Spear {
		t.Fatal("knife and spear map to their weapons")
	}
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.Arm(id, item.Spear)
	if pawnView(g, id).Weapon != "spear" {
		t.Fatalf("weapon = %q", pawnView(g, id).Weapon)
	}
}

func TestRaidKitCanRollKnifeSpear(t *testing.T) {
	seen := map[item.Kind]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		g := New(Config{Seed: seed, Width: 10, Height: 8})
		for _, k := range g.raidKit(1 << 20) {
			seen[k]++
		}
	}
	for _, k := range []item.Kind{item.Club, item.Knife, item.Spear, item.Rifle} {
		if seen[k] == 0 {
			t.Errorf("raid kits never include %v: %v", k, seen)
		}
	}
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	if k := g.raidKit(knifeCost); len(k) != 1 {
		t.Fatalf("knife points buy one raider: %v", k)
	}
	if knifeCost != 50 || spearCost != 70 || clubCost != 60 || rifleCost != 90 {
		t.Fatal("kit costs: knife 50, spear 70, club 60, rifle 90")
	}
	for _, k := range g.kitOf(200) {
		seen[k]++
	}
	if seen[item.Knife] == 0 {
		t.Fatal("kitOf rolls knives")
	}
}

func commandAddBill(x, y int, r recipe.ID) command.Command {
	return command.Command{Kind: command.AddBill, X: x, Y: y, Recipe: uint8(r), Mode: uint8(recipe.DoCount), Count: 1}
}
