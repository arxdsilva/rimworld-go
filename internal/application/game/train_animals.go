package game

import (
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal training (ENG-003, M10; Handle work). A handler trains a tame
// animal's first enabled, unlearned skill (Obedience before Guard and
// Release) once its session is due: herd.TrainCooldown (plus a per-animal
// stagger) after the last one, success or not, so neither handler nor
// animal flaps between sessions. A session succeeds with
// training.StepChance (x5 for the bonded trainer) and a success may bond
// them (training.BondTrain). Skills decay from the last session
// (training.Decay, read on demand: the saved steps are those of the last
// session, so a learned Obedience keeps its floor). An animal that has
// learned Obedience (or Guard, or Release) follows its drafted master.
// Guard: it defends its master from a hostile within guardMasterRadius of
// them, or from whoever just hit them. Release: it joins its drafted
// master's attack. A duty target is looked for once per
// animal.DecideEvery ticks (staggered by id, through the creature index);
// one it has engaged is kept, with a cheap check each tick, until the foe
// falls, the master's attack moves on, or a guard's foe is farther than
// guardKeepRadius from the master (enter 5, leave 8: no flapping).
//
// Which skills are on lives in the high bit of animal.State.Trained:
// herd.TrainToggled marks a skill the player flipped away from its default
// (Obedience on, the others off), so older saves and new animals need no
// extra field.

const (
	// trainWork: the ticks one session takes (like a taming).
	trainWork = animal.TameWork
	// followFar: an obedient animal walks to its drafted master once it
	// is farther than this; it then goes right up to them (followNear).
	followFar  = 3
	followNear = 1
	// guardMasterRadius: a guard animal takes on a hostile this close to
	// its master; guardKeepRadius: it keeps at it until the hostile is
	// farther than this from the master.
	guardMasterRadius = 5
	guardKeepRadius   = 8
	// guardHitWindow: a hit on the master this recent calls its guard to
	// whoever struck from there.
	guardHitWindow = 2 * animal.DecideEvery
)

// trainable reports whether a handler may train animal id now.
func (g *Game) trainable(id entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Tame || a.Dead() || a.Enraged(now) || sleeping(*a) || a.Leaving ||
		a.Mode == animal.Flee || a.Mode == animal.Attack || a.Mode == animal.Confused || !herd.TrainDue(id, *a, now) {
		return false
	}
	_, ok := herd.NextTrainSkill(*a, now)
	return ok
}

// cmdSetTraining turns training skill on or off for animal.
// cmdSetAnimalFollow sets whether tame animal id follows its drafted
// master (PLAY-015).
func (g *Game) cmdSetAnimalFollow(id entity.EntityID, on bool) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame {
		return
	}
	a.NoFollow = !on
	g.animals.Set(id, a)
}

func (g *Game) cmdSetTraining(id entity.EntityID, s int, on bool) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame || s < 0 || s >= int(training.NumSkills) || !herd.CanLearn(a.Species, training.Skill(s)) {
		return
	}
	sk := training.Skill(s)
	if on == herd.TrainDefault(sk) {
		a.Trained[sk] &^= herd.TrainToggled
	} else {
		a.Trained[sk] |= herd.TrainToggled
	}
	g.animals.Set(id, a)
}

// assignTrain sends handler id to train the nearest tame animal whose
// session is due.
func (g *Game) assignTrain(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	if !g.anyAnimal(func(aid entity.EntityID, a *animal.State) bool { return g.trainable(aid, a, now) }) {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		if a := g.animals.Ref(aid); a == nil || !g.trainable(aid, a, now) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(aid); taken {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := pathing.Dist(pos.X, pos.Y, p.X, p.Y); d < bestD || (d == bestD && aid < best) {
			best, bestD = aid, d
		}
	}
	if best == 0 {
		return false
	}
	p, _ := g.positions.Get(best)
	tx, ty, ok := g.besideSpot(id, p.X, p.Y)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Train, Target: best, TX: tx, TY: ty, WorkLeft: trainWork})
}

// doTrain runs a training session beside the animal.
func (g *Game) doTrain(id entity.EntityID, j job.Record) {
	a, ok := g.animals.Get(j.Target)
	pos, _ := g.positions.Get(id)
	ap, _ := g.positions.Get(j.Target)
	now := uint64(g.clock.Now())
	switch {
	case !ok || !a.Tame:
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	case weapon.Distance(pos.X, pos.Y, ap.X, ap.Y) > 1:
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skAnimals, true) // skills.go
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
	s, ok := herd.NextTrainSkill(a, now)
	if !ok {
		return
	}
	g.trainSession(id, j.Target, &a, s, now)
	g.animals.Set(j.Target, a)
}

// trainSession is one finished session of trainer on skill s: the decayed
// steps are settled, the clock restarts, and a success adds a step and
// may bond them.
func (g *Game) trainSession(trainer, id entity.EntityID, a *animal.State, s training.Skill, now uint64) {
	for k := training.Skill(0); k < training.NumSkills; k++ {
		a.Trained[k] = a.Trained[k]&herd.TrainToggled | herd.TrainSteps(*a, k, now)
	}
	a.TrainDecay = now
	chance := training.StepChance(int(g.skillOf(trainer, skill.Animals).Level), a.Bond == uint64(trainer))
	if int32(g.stream(streamBond).IntN(1000)) >= chance {
		return
	}
	a.Trained[s]++
	if training.Learned(s, a.Trained[s]&^herd.TrainToggled) {
		p, _ := g.positions.Get(id)
		g.message(p.X, p.Y, "%s trained %s the %s in %s.", g.name(trainer), a.Name, a.Species.Label(), herd.TrainLabel(s))
	}
	g.tryBond(id, a, trainer, training.BondTrain)
}

// followMaster: an animal that has learned Obedience stays by its drafted
// master, walking up to them once they are more than followFar away. It
// reports true while it follows.
func (g *Game) followMaster(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	if a.Master == 0 || !a.Tame || a.NoFollow || !herd.TrainLearned(*a, training.Obedience, now) &&
		!herd.TrainLearned(*a, training.Guard, now) && !herd.TrainLearned(*a, training.Release, now) {
		return false
	}
	m := entity.EntityID(a.Master)
	if !g.drafted(m) || !g.standingPawn(m) {
		return false
	}
	mp, _ := g.positions.Get(m)
	if a.PathI < len(a.Path) {
		if weapon.Distance(a.GoalX, a.GoalY, mp.X, mp.Y) > followNear && (now+uint64(id))%30 == 0 {
			g.animalGoTo(id, a, mp.X, mp.Y) // the master moved on
		}
		return true
	}
	if weapon.Distance(pos.X, pos.Y, mp.X, mp.Y) > followFar && now >= a.RetryAt {
		g.animalGoTo(id, a, mp.X, mp.Y)
	}
	return true
}

// masterDuty runs a guard- or release-trained animal's fight for its
// master (see the file comment). It reports true while it fights.
func (g *Game) masterDuty(id entity.EntityID, a *animal.State, now uint64) bool {
	if a.Master == 0 || !a.Tame {
		return false
	}
	guard, release := herd.TrainLearned(*a, training.Guard, now), herd.TrainLearned(*a, training.Release, now)
	if !guard && !release {
		return false
	}
	m := entity.EntityID(a.Master)
	mp, ok := g.positions.Get(m)
	if !ok || !g.alive(m) || !g.isPawn(m) {
		return false
	}
	if a.Target != 0 && a.Mode == animal.Attack && g.dutyKeeps(m, mp, a.Target, guard, release) {
		rageOf(g).Chase(id, a, now)
		return true
	}
	if (now+uint64(id))%animal.DecideEvery != 0 {
		return false
	}
	t := g.dutyTarget(id, m, mp, guard, release, now)
	if t == 0 {
		return false
	}
	a.Target, a.HuntSince = t, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0 // a resting guard gets up
	rageOf(g).Chase(id, a, now)
	return true
}

// masterAttack is drafted master m's attack target (0: none, or one no
// longer worth fighting).
func (g *Game) masterAttack(m entity.EntityID) entity.EntityID {
	if !g.drafted(m) {
		return 0
	}
	if j, ok := g.jobs.Get(m); ok && j.Kind == job.Attack && j.Active() && rageOf(g).CombatAlive(j.Target) {
		return j.Target
	}
	return 0
}

// guardFoe reports o is a threat a guard animal fights: a standing raider,
// or a wild animal attacking the colony, or a tame one gone manhunter
// (EAI-006: a grieving pet).
func (g *Game) guardFoe(id, o entity.EntityID, now uint64) bool {
	if g.isRaider(o) {
		return g.standingRaider(o)
	}
	a, ok := g.animals.Get(o)
	return o != id && ok && (!a.Tame || a.Enraged(now)) && !a.Dead() && (g.hostile(a) || a.Enraged(now))
}

// dutyKeeps reports engaged foe t is still the animal's business: its
// master's attack target (release), or a threat still near the master
// (guard, with the wider keep radius).
func (g *Game) dutyKeeps(m entity.EntityID, mp Position, t entity.EntityID, guard, release bool) bool {
	if !rageOf(g).CombatAlive(t) {
		return false
	}
	if release && g.masterAttack(m) == t {
		return true
	}
	if !guard || !g.guardFoe(0, t, uint64(g.clock.Now())) {
		return false
	}
	tp, _ := g.positions.Get(t)
	return weapon.Distance(mp.X, mp.Y, tp.X, tp.Y) <= guardKeepRadius
}

// dutyTarget picks the foe animal id should fight for master m at mp:
// the master's own attack target (release); else a threat within
// guardMasterRadius of them, or whoever just hit them (guard).
func (g *Game) dutyTarget(id, m entity.EntityID, mp Position, guard, release bool, now uint64) entity.EntityID {
	if release {
		if t := g.masterAttack(m); t != 0 && t != id {
			return t
		}
	}
	if !guard {
		return 0
	}
	foe := func(o entity.EntityID, _ Position) bool { return g.guardFoe(id, o, now) }
	if t, _, ok := g.nearestWhere(mp.X, mp.Y, guardMasterRadius, ckRaider|ckAnimal, foe); ok {
		return t
	}
	if h, ok := g.lastHit[m]; ok && h.Tick+guardHitWindow >= now {
		if t, _, ok := g.nearestWhere(h.FromX, h.FromY, 1, ckRaider|ckAnimal, foe); ok {
			return t // a shooter beyond the radius
		}
	}
	return 0
}

// trainViews is a's training rows for the view: every skill, with whether
// it can be learned, is on, and its steps.
func trainViews(a animal.State, now uint64) ([]bool, []AnimalTrainingView) {
	learned := make([]bool, training.NumSkills)
	rows := make([]AnimalTrainingView, training.NumSkills)
	for s := training.Skill(0); s < training.NumSkills; s++ {
		learned[s] = herd.TrainLearned(a, s, now)
		rows[s] = AnimalTrainingView{Skill: int(s), Label: herd.TrainLabel(s), Allowed: herd.CanLearn(a.Species, s),
			On: herd.TrainOn(a, s), Learned: learned[s], Steps: int(herd.TrainSteps(a, s, now)), Needed: int(training.Needed(s))}
	}
	return learned, rows
}
