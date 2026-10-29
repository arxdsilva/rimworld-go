package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
)

// Action read model (ART §27.2). Everything here is a read-only view of
// state the simulation already owns, plus two transient presentation
// stamps (strikeRec, hitRec) written when an attack resolves. The
// simulation never reads the stamps back, so they cannot change an
// outcome; they are keyed by entity and never iterated. They are saved
// additively (saveState.Strikes/Hits, absent in older saves) so a loaded
// game's views match the original exactly.

// strikeRec is an attacker's most recent resolved attack.
type strikeRec struct {
	Target entity.EntityID // 0 for a blow on a building
	Tick   uint64
}

// hitRec is the last hit a creature took.
type hitRec struct {
	Tick         uint64
	FromX, FromY int
	Damage       int
}

// actionOf maps a job to its canonical action (actions.go); "" when the
// pawn is idle, walking, or not yet working.
func (g *Game) actionOf(id entity.EntityID, j job.Record) string {
	if !g.alive(id) {
		return ""
	}
	active := j.State == job.Goto || j.State == job.Do
	working := j.State == job.Do
	switch j.Kind {
	case job.Attack, job.Hunt:
		if !active {
			return ""
		}
		if g.hasRanged(id) {
			return ActionAim
		}
		return ActionMelee
	case job.Haul:
		if active && j.Carry != 0 {
			return ActionHaul
		}
	case job.Bury:
		switch {
		case working && j.Carry != 0:
			return ActionBury
		case active && j.Carry != 0:
			return ActionHaul // carrying the body to the grave
		}
		return ""
	case job.Rescue:
		if working || active && g.care.Carry.Patient(id) != 0 {
			return ActionRescue
		}
	case job.Bill:
		if working && j.Carry != 0 {
			return g.billAction(j)
		}
	case job.Tend, job.EmergencyTend: // EmergencyTend: care/tend.go (MED-03)
		if working && !(j.Dest != 0 && j.Carry == 0) {
			return ActionTend
		}
	case job.Refuel:
		if working && j.Carry != 0 {
			return ActionRefuel
		}
	}
	if !working {
		return ""
	}
	switch j.Kind {
	case job.Construct:
		if fr, ok := g.frames.Get(j.Target); ok && fr.Kind == construction.Grave {
			return ActionDig
		}
		return ActionBuild
	case job.Deconstruct:
		return ActionDeconstruct
	case job.Cut:
		return ActionChop
	case job.Mine:
		return ActionMine
	case job.Smooth:
		return ActionSmooth
	case job.Sow:
		return ActionSow
	case job.Harvest:
		return ActionHarvest
	case job.Eat:
		return ActionEat
	case job.Horseshoes:
		return ActionHorseshoes
	case job.Research:
		return ActionResearch
	case job.Tame:
		return ActionTame
	case job.Milk:
		return ActionMilk
	case job.Shear:
		return ActionShear
	case job.Repair:
		return ActionRepair
	case job.Roof, job.Unroof:
		return ActionBuild
	case job.Breach:
		return ActionBreach
	case job.Clean, job.ClearSnow: // snow_clear.go: shovelling reads as sweeping
		return ActionClean
	}
	return ""
}

// billAction names the bench work by the bench the bill runs at.
func (g *Game) billAction(j job.Record) string {
	if b, ok := g.bldgs.Get(j.Dest); ok {
		switch construction.RecipeBench(b.Kind) { // an electric bench works its base bench's recipes
		case construction.Stove, construction.Campfire, construction.DryingRack:
			return ActionCook
		case construction.TailoringBench:
			return ActionTailor
		case construction.ButcherTable:
			return ActionButcher
		case construction.Pyre:
			return ActionCremate
		}
	}
	return ActionCraft
}

// workTotal is the full work of the job as its content def defines it, 0
// when unknown.
func (g *Game) workTotal(id entity.EntityID, j job.Record) int {
	switch j.Kind {
	case job.Construct:
		if fr, ok := g.frames.Get(j.Target); ok {
			return construction.WorkFor(fr.Kind, fr.Stuff)
		}
	case job.Deconstruct:
		if b, ok := g.bldgs.Get(j.Target); ok {
			return construction.DeconstructTicks(b.Kind)
		}
	case job.Cut:
		if p, ok := g.plants.Get(j.Target); ok {
			return plant.DefOf(p.Species).CutWork
		}
	case job.Harvest:
		if p, ok := g.plants.Get(j.Target); ok {
			return plant.DefOf(p.Species).HarvestWork
		}
	case job.Mine:
		return g.mineWorkAt(j.DestX, j.DestY)
	case job.Smooth:
		return g.smoothWork(j.DestX, j.DestY)
	case job.Sow:
		if z, ok := g.zoneAt(j.TX, j.TY); ok && plant.DefOf(plant.Species(z.Crop)).SowWork > 0 {
			return plant.DefOf(plant.Species(z.Crop)).SowWork
		}
		return SowWork
	case job.Research:
		return researchSession
	case job.Tend, job.EmergencyTend:
		if j.Target == id {
			return TendWork * 3 / 2
		}
		return TendWork
	case job.Tame:
		return animal.TameWork
	case job.Milk, job.Shear:
		return animal.CollectWork
	case job.Eat:
		return needs.EatWorkTicks
	case job.Bury:
		return buryWork
	case job.Roof:
		return roof.BuildWork
	case job.Unroof:
		return roof.RemoveWork
	case job.Refuel:
		return refuelWork
	case job.Clean, job.ClearSnow:
		return j.Count // set from the tile's filth (or snow) when the job starts
	case job.Bill:
		if bills := g.bills[j.Dest]; j.BillIndex < len(bills) {
			return recipe.DefOf(bills[j.BillIndex].Recipe).Work
		}
	}
	return 0
}

// progressOf is the job's progress 0..1, -1 when the total is unknown.
func (g *Game) progressOf(id entity.EntityID, j job.Record) float64 {
	total := g.workTotal(id, j)
	if total <= 0 {
		return -1
	}
	return clamp01(1 - float64(j.WorkLeft)/float64(total))
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

// cycle is progress through a countdown, kept below 1: only a resolved
// strike reads exactly 1.
func cycle(left, total int) float64 {
	if total <= 0 {
		return 0
	}
	return min(clamp01(1-float64(left)/float64(total)), 0.99)
}

// justStruck reports that the strike resolved in the tick just completed:
// the clock advances at the end of Step, so the first snapshot after a
// strike reads now == Tick+1 (the same snapshot where its trace first
// shows, at Age 1).
func justStruck(rec strikeRec, now uint64) bool { return rec.Tick+1 == now }

// attackCycleOf is how far a pawn is through its current aim or cooldown;
// exactly 1 on the tick its attack resolves.
func (g *Game) attackCycleOf(id entity.EntityID, j job.Record) float64 {
	now := uint64(g.clock.Now())
	rec, struck := g.lastStrike[id]
	if struck && justStruck(rec, now) {
		return 1
	}
	switch j.Kind {
	case job.Attack, job.Hunt:
		if j.State != job.Goto && j.State != job.Do {
			return 0
		}
		d := weaponDefOf(g, id)
		total := aimTicks(d)
		// Past the first aim the countdown is the weapon's cooldown.
		if j.WorkLeft > total || struck && rec.Target == j.Target && now-rec.Tick <= uint64(d.Cooldown) {
			total = d.Cooldown
		}
		return cycle(j.WorkLeft, total)
	case job.Breach:
		if j.State == job.Do {
			return cycle(j.WorkLeft, breachCooldown)
		}
	}
	return 0
}

// carryingOf is the item kind held in the arms ("pawn" while carrying a
// patient), "" when empty-handed.
func (g *Game) carryingOf(id entity.EntityID, j job.Record) string {
	if g.care.Carry.Patient(id) != 0 {
		return "pawn"
	}
	if j.Carry != 0 {
		if st, ok := g.stacks.Get(j.Carry); ok && st.Carrier == id {
			return st.Kind.String()
		}
	}
	return ""
}

// fillPawnAction fills the C1 action fields of a pawn or raider view.
func (g *Game) fillPawnAction(pv *PawnView, id entity.EntityID, j job.Record) {
	pv.Action = g.actionOf(id, j)
	pv.ActionProgress = -1
	if pv.Action != "" {
		pv.ActionProgress = g.progressOf(id, j)
		switch j.Kind {
		case job.Attack, job.Hunt, job.Tend, job.EmergencyTend, job.Tame, job.Rescue, job.Milk, job.Shear:
			pv.ActTarget = j.Target
		}
	}
	pv.AttackCycle = g.attackCycleOf(id, j)
	pv.Carrying = g.carryingOf(id, j)
	if j.Carry != 0 {
		if st, ok := g.stacks.Get(j.Carry); ok && st.Carrier == id && st.Kind == item.CorpseAnimal {
			pv.CarryingAnimal = animal.Species(st.Animal).String()
		}
	}
	if h, ok := g.lastHit[id]; ok {
		pv.HitTick, pv.HitFromX, pv.HitFromY, pv.HitDamage = h.Tick, h.FromX, h.FromY, h.Damage
	}
}

// chargeLine is a charging predator's state line, "charging Ada" (the
// inspector capitalises it); "" when it is not charging.
func (g *Game) chargeLine(a animal.State) string {
	if a.ChargeSince == 0 || a.Mode != animal.Attack || a.Target == 0 {
		return ""
	}
	return "charging " + g.creatureName(a.Target)
}

// fillAnimalAction fills the C1 action fields of an animal view from real,
// the animal as it is. The action switch reads a feeding, leaving or
// resting animal as a wandering one (actionState); the graze rule reads
// the real state (what it is eating, where).
func (g *Game) fillAnimalAction(av *AnimalView, id entity.EntityID, real animal.State) {
	now := uint64(g.clock.Now())
	def := animal.DefOf(real.Species)
	a := actionState(real)
	struck := false
	if rec, ok := g.lastStrike[id]; ok && justStruck(rec, now) {
		struck = true
	}
	switch a.Mode {
	case animal.Flee:
		av.Action = ActionFlee
	case animal.Attack:
		pos, _ := g.positions.Get(id)
		if tp, ok := g.positions.Get(a.Target); struck || ok && weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) <= 1 {
			av.Action = animal.RosterOf(a.Species).Attack.String() // bite, claw, kick, gore, headbutt, peck
			av.AttackCycle = cycle(a.Cooldown, def.Cooldown)
		}
	default:
		if g.grazing(id, real) {
			av.Action = ActionGraze
		}
	}
	if struck {
		av.AttackCycle = 1
	}
	if h, ok := g.lastHit[id]; ok {
		av.HitTick, av.HitFromX, av.HitFromY, av.HitDamage = h.Tick, h.FromX, h.FromY, h.Damage
	}
}

// grazing reports an animal with its head down to eat (PLAY-022 C5): an
// idle non-predator that stands on something it eats (grass, or a plant
// it browses: Diet.Plants), or that is eating its feed target within
// reach (the bush or bowl beside it). Anywhere else, bare ground
// included, it does not graze. Reads two cell lookups, never a scan.
func (g *Game) grazing(id entity.EntityID, a animal.State) bool {
	if animal.DefOf(a.Species).Predator || a.PathI < len(a.Path) {
		return false
	}
	pos, ok := g.positions.Get(id)
	if !ok {
		return false
	}
	if t := a.Feed; a.Mode == animal.Feed && t.Kind != animal.FeedNone &&
		weapon.Distance(pos.X, pos.Y, t.X, t.Y) <= feedReach(t) && g.feedThere(&a, t) { // animal_needs.go
		return true
	}
	if !animal.EcologyOf(a.Species).Diet.Plants() {
		return false
	}
	return g.hasGrass(pos.X, pos.Y) || g.browsedPlantAt(&a, pos)
}

// browsedPlantAt reports a plant on pos that animal a eats from: a wild
// plant that is browse food (30% grown, as findFeed reads it), or a crop
// a wild animal raids.
func (g *Game) browsedPlantAt(a *animal.State, pos Position) bool {
	pid := g.plantAt(pos.X, pos.Y)
	if pid == 0 {
		return false
	}
	p, ok := g.plants.Get(pid)
	if !ok {
		return false
	}
	pd := plant.DefOf(p.Species)
	switch {
	case p.Zone && pd.Crop:
		return !a.Tame && p.Percent() >= animal.CropDies+5
	case pd.Browse:
		return p.Percent() >= 30
	}
	return false
}
