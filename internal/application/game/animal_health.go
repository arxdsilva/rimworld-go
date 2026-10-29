package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal medicine (MED-09). A wounded animal gets a body
// (animal.State.Body). Since MED-13 it has its species' anatomy
// (animal.AnatomyOf: quadruped, bird, snake, lizard or insect parts,
// their hit points scaled by the species' size, animalHealthScale): a hit
// lands on a part picked by coverage, may take off a leg, a wing or an
// eye, and a lost leg slows the animal (body.Capacity(Moving),
// animal.State.Step). It is made on the first wound and dropped once
// nothing is left to treat and no part is lost, so an unhurt animal
// carries none and saves stay as they were. A body saved before MED-13
// (every wound on one virtual part) migrates on load
// (migrateAnimalBodies).
//
// The body reuses the humanlike rules: open cuts, bites and scratches
// bleed (body.BleedRate) until tended, blood loss kills at 1000; each
// wound that can fester rolls an infection 6–18 hours later
// (infection.WoundChance), which races immunity (infection.StepRace) and
// kills at full severity; wounds heal at the wiki rates (body.HealHour,
// faster tended and asleep). Bleeding is scaled by the species' size:
// RimWorld divides blood loss by body size; ours have none, so hit points
// against a human's (health.MaxHP) stand in.
//
// Injury stays the hit point scalar everything else reads (death at
// MaxHP, fleeing, the Health bar). The wounds are the treatable part of
// it and never add up to more (reconcileAnimalBody): a wound adds to
// both, healing a wound takes it off Injury, and Injury healed another way
// (a tame animal's natural heal) trims the smallest open wounds. A vet's
// tend works on the wounds (tendAnimalBody). Blood loss and infections
// kill; a bad enough injury or blood loss downs the animal first
// (animal.State.DownsNow, below).
//
// Once an hour the medical phase (phaseMedical, bleed.go) walks the
// animal patients (sorted ids of animals with a body; derived, rebuilt
// on first use after a load), never every animal. Rolls use the medical
// stream (the part a hit lands on, the infection delay). Scars are left
// out, so healing draws no random numbers.

// Animal health scale bounds (permille of a human's hit points): the
// smallest species bleeds at most 10x a human's rate, the largest at
// least a tenth of it (original tuning).
const (
	animalScaleMin = 100
	animalScaleMax = 10000
)

// animalHealthScale is species s's body size against a human's, from its
// hit points (permille).
func animalHealthScale(s animal.Species) int32 {
	return min(max(int32(animal.DefOf(s).MaxHP)*1000/health.MaxHP, animalScaleMin), animalScaleMax)
}

// animalBleedPerDay is how fast an animal of species s with body b bleeds
// (permille of its blood per day): the body's bleeding scaled by its size.
func animalBleedPerDay(b *body.Body, s animal.Species) int32 {
	if b == nil {
		return 0
	}
	n := b.BleedPerDay()
	if n <= 0 {
		return 0
	}
	return max(n*1000/animalHealthScale(s), 1)
}

// hourShare is a daily permille rate's share of one hour, rounded to
// nearest, never below 1 for a positive rate (body.StepBlood's rounding).
func hourShare(perDay int32) int32 {
	if perDay <= 0 {
		return 0
	}
	return max((perDay+12)/24, 1)
}

// stepAnimalBlood advances an hour of blood loss on body b of species s:
// the scaled bleeding, or the regeneration while nothing bleeds; the
// stage follows with its hysteresis (body.BloodStage).
func stepAnimalBlood(b *body.Body, s animal.Species) {
	if bleed := animalBleedPerDay(b, s); bleed > 0 {
		b.BloodLoss = min(b.BloodLoss+hourShare(bleed), body.BloodLossDeath)
	} else if b.BloodLoss > 0 {
		b.BloodLoss = max(b.BloodLoss-hourShare(body.BloodRegenPerDay), 0)
	}
	b.BloodStage = body.BloodStage(b.BloodLoss, b.BloodStage)
}

// animalBleedDeathHours is how many hours an animal of species s with
// body b has before bleeding out at its current rate (-1: not bleeding).
func animalBleedDeathHours(b *body.Body, s animal.Species) int {
	bleed := animalBleedPerDay(b, s)
	if bleed <= 0 {
		return -1
	}
	left := max(body.BloodLossDeath-b.BloodLoss, 0)
	h := hourShare(bleed)
	return int((left + h - 1) / h)
}

// newAnimalBody is a whole body of species s: its anatomy, its parts
// scaled to its hit points (MED-13).
func newAnimalBody(s animal.Species) *body.Body {
	return &body.Body{Anatomy: animal.AnatomyOf(s), Scale: animalHealthScale(s)}
}

// woundAnimal gives animal id (state a, Injury already raised by dmg)
// dmg of kind, or the attacker by's weapon's kind, on parts picked by
// coverage (woundAnimalBody). It makes the body on the first wound,
// schedules the new wounds' infection roll and adds the animal to the
// patients. A killing blow leaves no wound.
func (g *Game) woundAnimal(id entity.EntityID, a *animal.State, dmg int, kind body.WoundKind, by entity.EntityID, now uint64) {
	if dmg <= 0 || a.Dead() {
		return
	}
	if kind == woundByAttacker {
		kind = g.woundKindBy(by) // body.go
	}
	if a.Body == nil {
		a.Body = newAnimalBody(a.Species)
	}
	b := a.Body
	g.woundAnimalBody(b, dmg, kind, now)
	if infection.WoundChance(kind, -1, infection.RoomOther) > 0 {
		h := infection.DelayMin + g.stream(streamMedical).IntN(infection.DelaySpan+1)
		for i := range b.Wounds { // the wounds this hit opened
			if w := &b.Wounds[i]; w.At == now && w.InfectAt == 0 && !w.Permanent && !w.Tended {
				w.InfectAt = now + uint64(h)*clock.TicksPerHour
			}
		}
	}
	reconcileAnimalBody(a)
	if a.Body != nil {
		g.markAnimalPatient(id)
	}
}

// woundAnimalBody deals dmg of kind to animal body b on parts picked by
// coverage with the medical stream, as woundBody does a colonist's: a part
// whose loss would kill (the head, brain, heart, torso, the last lung:
// body.LossKills) is never taken off by a hit, the rest spills to the
// next pick; other parts (a leg, a wing, an eye) may be lost. What no
// part takes lands on the root (the torso, a snake's body).
func (g *Game) woundAnimalBody(b *body.Body, dmg int, kind body.WoundKind, now uint64) {
	rng := g.stream(streamMedical)
	left := dmg
	for try := 0; left > 0 && try < maxSpill; try++ {
		drawn := -1
		roll := func(n int) int { // one draw, replayed so Hurt picks the same part
			if drawn < 0 {
				drawn = rng.IntN(n)
			}
			return drawn
		}
		p, ok := b.PickPart(roll)
		if !ok {
			break
		}
		hit := left
		if room := b.PartDef(p).MaxHP - partDmg(b, p) - 1; hit > room && b.LossKills(p) {
			if room <= 0 {
				continue
			}
			hit = room
		}
		before := b.Damage()
		b.Hurt(roll, hit, kind, now)
		left -= b.Damage() - before
	}
	if left > 0 {
		b.Wounds = append(b.Wounds, body.Wound{Part: b.Root(), Dmg: left, Kind: kind, At: now, Size: int32(left)})
	}
}

// woundDmg is the damage of b's open wounds (scars excluded).
func woundDmg(b *body.Body) int {
	n := 0
	for _, w := range b.Wounds {
		if !w.Permanent {
			n += w.Dmg
		}
	}
	return n
}

// reconcileAnimalBody keeps a's open wounds within its Injury (trimming
// the smallest when Injury healed another way) and drops the body once
// nothing on it needs care and no part is lost (a lost leg stays lost:
// MED-13). Lost parts are not wounds: they do not count against Injury.
func reconcileAnimalBody(a *animal.State) {
	b := a.Body
	if b == nil {
		return
	}
	if d := woundDmg(b); d > a.Injury {
		trimOpen(b, d-max(a.Injury, 0)) // body.go
	}
	if !needsCare(b) && b.Missing == 0 { // patients.go
		a.Body = nil
	}
}

// animalPatient reports an animal the hourly medical step must walk: it
// has a body that needs care (patients.go). A body kept only for its
// lost parts needs none.
func animalPatient(a *animal.State) bool { return a.Body != nil && needsCare(a.Body) }

// migrateAnimalBodies gives every animal body saved before MED-13 (the
// humanlike model with no Scale: every wound on one virtual part) its
// species' anatomy, moving its wounds and conditions to the root part
// (the torso, a snake's body, an insect's thorax). Run once on load.
func (g *Game) migrateAnimalBodies() {
	for _, id := range g.animalIDs {
		a := g.animals.Ref(id)
		if a == nil || a.Body == nil || a.Body.Scale != 0 || a.Body.Anatomy != body.Humanlike {
			continue
		}
		migrateAnimalBody(a)
	}
}

// migrateAnimalBody is migrateAnimalBodies on one animal.
func migrateAnimalBody(a *animal.State) {
	old := a.Body
	nb := newAnimalBody(a.Species)
	root := nb.Root()
	nb.Wounds, nb.Conds = old.Wounds, old.Conds
	nb.BloodLoss, nb.BloodStage = old.BloodLoss, old.BloodStage
	for i := range nb.Wounds {
		nb.Wounds[i].Part = root
	}
	for i := range nb.Conds {
		nb.Conds[i].Part = root
	}
	a.Body = nb
}

// animalTendDue reports animal id (state a) with something a tend would
// help at tick now: with a body, an untended wound or an infection due its
// next tend (body.TendDue: one tend per window); without one (an older
// save, starvation), any injury not tended within TendTicks.
func (g *Game) animalTendDue(id entity.EntityID, a *animal.State, now uint64) bool {
	if a.Body != nil {
		return a.Body.TendDue(now)
	}
	return a.Injury > 0 && !g.tended(id)
}

// tendAnimalBody applies a vet's tend of quality q to animal a at tick
// now. With a body, every open wound and due infection on it is tended:
// the bleeding stops, the wounds heal faster by the hour and fester less
// (as a colonist's). Without one (an older save, starvation) Injury mends
// at once by animalTendHeal (medical.go), the scalar model's tend.
func tendAnimalBody(a *animal.State, q int32, now uint64) {
	if a.Body == nil {
		a.Injury = max(0, a.Injury-animalTendHeal(q))
		return
	}
	a.Body.TendAll(q, now)
	reconcileAnimalBody(a)
}

// stepAnimalHealth runs an hour of animal medicine (from phaseMedical)
// on every animal patient in id order, then drops the healed and the gone.
func (g *Game) stepAnimalHealth(now uint64) {
	ids := g.animalPatientIDs()
	if len(ids) == 0 {
		return
	}
	for _, id := range slices.Clone(ids) { // a death touches the stores
		a, ok := g.animals.Get(id)
		if !ok || a.Body == nil || a.Dead() {
			continue
		}
		if g.stepAnimalBody(id, &a, now) {
			continue // died
		}
		g.animals.Set(id, a)
	}
	g.sweepAnimalPatients()
}

// stepAnimalBody is one hour on animal id's body: blood loss, due
// infection rolls, the infection races, then healing. It reports the
// animal died.
func (g *Game) stepAnimalBody(id entity.EntityID, a *animal.State, now uint64) bool {
	b := a.Body
	stepAnimalBlood(b, a.Species)
	if b.BloodLoss >= body.BloodLossDeath {
		g.animalMedicalDeath(id, a)
		return true
	}
	g.rollAnimalInfections(id, a, b, now)
	if g.stepAnimalConds(id, a, b, now) {
		return true
	}
	lying := int32(0)
	if lyingDown(*a) {
		lying = body.LieGround
	}
	before := woundDmg(b)
	b.HealHour(noScarRoll, lying)
	a.Injury = max(0, a.Injury-(before-woundDmg(b)))
	reconcileAnimalBody(a)
	g.checkAnimalDowned(id, a, now)
	return false
}

// ---- downed animals ----
//
// An animal goes down at animal.DownInjuryPct of its hit points lost or
// with severe blood loss, and stands again only at animal.UpInjuryPct with
// blood loss below moderate (the hysteresis lives in the domain:
// animal.State.DownsNow and StandsUp). It is checked when the animal is
// hurt (animalHurt), on the hourly medical step of an animal patient
// (stepAnimalBody) and on the animal's own decision cycle (downedThink:
// starvation, a tend without a body) — never by a scan of every animal.
//
// A downed animal lies where it fell (the rest pose, tinted): thinkAnimal
// returns at once, so it neither wanders, flees, fights, hunts nor feeds,
// and it takes no path step (moveAnimal). It still bleeds, heals, starves
// and can die. Predators take it as easy prey (huntable, findPrey),
// hunters finish a marked one first (assignHunt), a vet tends a tame one
// where it lies, and a downed wild animal is easier to tame (doTame).

// lyingDown reports an animal lying on the ground: asleep, or downed. It
// heals as a sleeper and its rest refills.
func lyingDown(a animal.State) bool { return a.Downed || sleeping(a) } // animal_needs.go

// checkAnimalDowned downs animal id (state a) when it is now too hurt or
// bled to stand, or stands a downed one well enough again. It reports a
// change.
func (g *Game) checkAnimalDowned(id entity.EntityID, a *animal.State, now uint64) bool {
	switch {
	case a.DownsNow():
		g.downAnimal(id, a, now)
		return true
	case a.StandsUp():
		a.Downed, a.DownedTick = false, 0
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		a.NextDecide = now // it thinks again at once
		return true
	}
	return false
}

// downAnimal puts animal id (state a) down at tick now: it drops whatever
// it was doing (a hunt, a flight, a meal, a charge, a manhunter's chase)
// and lies where it is.
func (g *Game) downAnimal(id entity.EntityID, a *animal.State, now uint64) {
	a.Downed, a.DownedTick = true, now
	a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	a.Target, a.HuntSince, a.FleeUntil, a.ChargeSince = 0, 0, 0, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	g.logEvent(id, logEntry{Kind: logDowned})
	if a.Tame && a.Name != "" {
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s the %s is down.", a.Name, a.Species.Label())
	}
}

// downedThink is a downed animal's tick in thinkAnimal: it lies still,
// and on its decision cycle a tame one heals naturally and it is checked
// for standing up. A standing animal is checked for going down on its
// decision cycle too (starvation hurts outside combat). It reports the
// animal is (still) down: thinkAnimal does nothing else.
func (g *Game) downedThink(id entity.EntityID, a *animal.State, now uint64) bool {
	if now >= a.NextDecide {
		if !a.Downed {
			if !a.DownsNow() {
				return false
			}
			g.downAnimal(id, a, now)
		}
		a.NextDecide = nextDecide(id, now) // animal_needs.go
		if a.Tame && (now+uint64(id))%animal.DecideEvery == 0 {
			g.naturalHeal(id, a, now) // vet_animals.go
		}
		if g.checkAnimalDowned(id, a, now) && !a.Downed {
			return false // up again: it decides at once
		}
	}
	if !a.Downed {
		return false
	}
	// whatever was set on it from outside (a pack's call, a rally), it
	// lies still
	a.Mode, a.Target, a.Path, a.PathI = animal.Wander, 0, nil, 0
	return true
}

// noScarRoll is the scar roll animal healing passes: never a scar
// (animals heal clean; it keeps healing free of random draws).
func noScarRoll(int) int { return 1 << 20 }

// animalMedicalDeath kills animal id (state a) of its wounds, blood loss
// or an infection, with no killer (killAnimal says a tame one has died).
func (g *Game) animalMedicalDeath(id entity.EntityID, a *animal.State) {
	g.animals.Set(id, *a)
	g.killAnimal(id, 0) // animals.go
}

// rollAnimalInfections rolls every wound of animal id whose infection
// roll is due: the wound's chance from its tend quality and the room,
// scaled by the difficulty (as rollInfections, infection.go).
func (g *Game) rollAnimalInfections(id entity.EntityID, a *animal.State, b *body.Body, now uint64) {
	for i := range b.Wounds {
		w := &b.Wounds[i]
		if w.InfectAt == 0 || now < w.InfectAt {
			continue
		}
		w.InfectAt = 0
		if w.Permanent || w.Dmg <= 0 {
			continue
		}
		tq := int32(-1)
		if w.Tended {
			tq = int32(w.TendQ)
		}
		p := g.infectionChance(infection.WoundChance(w.Kind, tq, g.roomInfectionFactor(id))) // story.go, infection.go
		if p <= 0 || int32(g.stream(streamMedical).IntN(1000)) >= p {
			continue
		}
		if b.AddCond(body.Condition{Kind: body.CondInfection, Part: w.Part, Sev: infection.Onset, Since: now}) {
			g.logEvent(id, logEntry{Kind: logInfected})
			if a.Tame {
				pos, _ := g.positions.Get(id)
				g.message(pos.X, pos.Y, "%s's wound is infected.", a.Name)
			}
		}
	}
}

// stepAnimalConds advances an hour of every condition race on animal id
// (normal immunity: no organs or monitors); it reports the animal died. A
// beaten condition is removed.
func (g *Game) stepAnimalConds(id entity.EntityID, a *animal.State, b *body.Body, now uint64) bool {
	if len(b.Conds) == 0 {
		return false
	}
	beaten := false
	for i := range b.Conds {
		c := &b.Conds[i]
		if body.IsDisease(c.Kind) {
			continue
		}
		d := body.DefOfCond(c.Kind)
		dead, cured := infection.StepRace(c, d, now < c.TendUntil, 1000, 1)
		switch {
		case dead:
			g.animalMedicalDeath(id, a)
			return true
		case cured:
			c.Sev, beaten = 0, true
			g.logEvent(id, logEntry{Kind: logInfectionCleared})
		}
	}
	if beaten {
		b.Conds = slices.DeleteFunc(b.Conds, func(c body.Condition) bool { return c.Sev <= 0 })
		if len(b.Conds) == 0 {
			b.Conds = nil
		}
	}
	return false
}

// ---- the animal patients index ----

// animalPatientIDs is the sorted ids of animals with a body that needs
// care (animalPatient), built from the animals on first use (a new game,
// or after a load reset the care state).
func (g *Game) animalPatientIDs() []entity.EntityID {
	c := &g.med
	if !c.animalIdxOK {
		c.animalPatients = c.animalPatients[:0]
		for _, id := range g.animalIDs {
			if a := g.animals.Ref(id); a != nil && animalPatient(a) {
				c.animalPatients = append(c.animalPatients, id)
			}
		}
		slices.Sort(c.animalPatients)
		c.animalIdxOK = true
	}
	return c.animalPatients
}

// markAnimalPatient adds animal id to the index (once).
func (g *Game) markAnimalPatient(id entity.EntityID) {
	ids := g.animalPatientIDs()
	if i, found := slices.BinarySearch(ids, id); !found {
		g.med.animalPatients = slices.Insert(ids, i, id)
	}
}

// sweepAnimalPatients drops every indexed animal that is gone or has no
// body needing care left.
func (g *Game) sweepAnimalPatients() {
	ids := g.animalPatientIDs()
	kept := ids[:0]
	for _, id := range ids {
		if a := g.animals.Ref(id); a != nil && animalPatient(a) {
			kept = append(kept, id)
		}
	}
	clear(ids[len(kept):])
	g.med.animalPatients = kept
}

// ---- the read model ----

// AnimalPartView is one hurt or lost part of an animal (MED-13): its
// label ("front left leg"), hit points left of MaxHP, lost, and bleeding
// (an open wound, or a fresh stump).
type AnimalPartView struct {
	Label     string
	HP, MaxHP int
	Missing   bool
	Bleeding  bool
}

// animalParts lists body b's hurt and lost parts in part order; a lost
// part's children go with it.
func animalParts(b *body.Body) []AnimalPartView {
	var out []AnimalPartView
	for _, ps := range b.Parts() {
		if ps.Missing {
			if par := b.PartDef(ps.Part).Parent; par != ps.Part && b.PartMissing(par) {
				continue
			}
		} else if ps.HP >= ps.MaxHP {
			continue
		}
		out = append(out, AnimalPartView{Label: ps.Label, HP: ps.HP, MaxHP: ps.MaxHP, Missing: ps.Missing,
			Bleeding: b.PartBleedPerDay(ps.Part) > 0})
	}
	return out
}

// fillAnimalHealth fills an animal view's health block (MED-09): the
// headline, bleeding, blood loss, the worst infection, whether a tend is
// due, and the care level.
func (g *Game) fillAnimalHealth(av *AnimalView, id entity.EntityID, a animal.State, now uint64) {
	av.CareLevel = uint8(g.care.Level(id)) // care package
	av.BleedDeathHours = -1
	av.HealthState = "Healthy"
	av.MovingPct = 100
	if a.Injury > 0 {
		av.HealthState = "Injured"
	}
	b := a.Body
	if b == nil {
		av.NeedsTend = a.Tame && a.Injury > 0 && !g.tended(id)
		return
	}
	av.Parts = animalParts(b)
	av.MovingPct = int(b.Capacity(body.Moving)) / 10
	av.NeedsTend = a.Tame && b.TendDue(now)
	av.BleedPerDay = int(animalBleedPerDay(b, a.Species))
	av.BleedDeathHours = animalBleedDeathHours(b, a.Species)
	av.BloodLossPct = int(b.BloodLoss) / 10
	av.BloodStage = body.BloodStageLabel(b.BloodStage)
	for _, c := range b.Conds {
		if c.Kind == body.CondInfection && (av.InfectionPct == 0 || int(c.Sev)/10 > av.InfectionPct) {
			av.InfectionPct = max(int(c.Sev)/10, 1)
			av.ImmunityPct = int(c.Immunity) / 10
			av.InfectionLosing = infection.Losing(c, body.DefOfCond(c.Kind), now < c.TendUntil, 1000)
		}
	}
	if av.BleedPerDay > 0 {
		av.HealthState = fmt.Sprintf("Bleeding %d%%/day · death in %dh", av.BleedPerDay/10, av.BleedDeathHours)
	}
}
