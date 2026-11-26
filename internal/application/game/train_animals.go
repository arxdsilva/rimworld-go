// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

// Animal training uses the Handle job for Obedience, Guard and Release.
// Each session attempts a step, may form a bond and resets saved maintenance.
// Periodic maintenance lowers the outermost trained skill at a wildness-based
// interval, and eventually removes tameness from neglected wild animals.
// Guard and Release follow the drafted master and defend its current target.
// The high bit of Trained stores each player-selected training toggle.

const (
	// trainWork: the ticks one session takes (like a taming).
	trainWork = animal.TameWork
	// Follow its drafted master once more than followFar away.
	followFar  = 3
	followNear = 1
	// Guard engagement and disengagement radii.
	guardMasterRadius = 5
	guardKeepRadius   = 8
	// Guard reacts to a recent hit on its master.
	guardHitWindow = 2 * animal.DecideEvery
)

// trainable reports whether a handler may train animal id now.
func (g *Game) trainable(id entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Tame || a.Dead() || a.Enraged(now) || sleeping(*a) || a.Leaving ||
		a.Mode == animal.Flee || a.Mode == animal.Attack || a.Mode == animal.Confused || !herd.TrainDue(id, *a, now) {
		return false
	}
	_, ok := herd.NextTrainSkill(*a, now, trainingAt(g, id))
	return ok || maintenanceDue(g, id, *a, now)
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
	m := trainingAt(g, id)
	if on == herd.TrainDefault(sk) {
		*herd.TrainingSlot(&a, &m, sk) &^= herd.TrainToggled
	} else {
		*herd.TrainingSlot(&a, &m, sk) |= herd.TrainToggled
	}
	g.animals.Set(id, a)
	saveTraining(g, id, m)
}

// assignTrain sends handler id to train the nearest tame animal whose
// session is due.
func (g *Game) assignTrain(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	if !g.animals.AnyTame(func(aid entity.EntityID, a *animal.State) bool { return g.trainable(aid, a, now) }) {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		if a := g.animals.Ref(aid); a == nil || !g.trainable(aid, a.SnapshotRef(), now) {
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
	s, ok := herd.NextTrainSkill(a, now, trainingAt(g, j.Target))
	if ok {
		g.trainSession(id, j.Target, &a, s, now)
	} else if maintenanceDue(g, j.Target, a, now) {
		g.resetAnimalTameness(j.Target, &a, now)
	} else {
		return
	}
	g.animals.Set(j.Target, a)
}

// trainSession resets maintenance, rolls a training step and may bond.
func (g *Game) trainSession(trainer, id entity.EntityID, a *animal.State, s training.Skill, now uint64) {
	g.resetAnimalTameness(id, a, now)
	chance := training.StepChance(int(g.skillOf(trainer, skill.Animals).Level), a.Bond == uint64(trainer))
	if int32(g.stream(streamBond).IntN(1000)) >= chance {
		return
	}
	m := trainingAt(g, id)
	*herd.TrainingSlot(a, &m, s)++
	saveTraining(g, id, m)
	if training.Learned(s, *herd.TrainingSlot(a, &m, s)&^herd.TrainToggled) {
		p, _ := g.positions.Get(id)
		g.message(p.X, p.Y, "%s trained %s the %s in %s.", g.name(trainer), a.Name, a.Species.Label(), herd.TrainLabel(s))
	}
	g.tryBond(id, a, trainer, training.BondTrain)
}

// followMaster keeps an obedient animal near its drafted master.
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
	if j, ok := g.jobs.Get(m); ok && g.drafted(m) && j.Kind == job.Attack && j.Active() && rageOf(g).CombatAlive(j.Target) {
		return j.Target
	}
	return 0
}

// guardFoe recognizes raiders and hostile animals, including manhunters.
func (g *Game) guardFoe(id, o entity.EntityID, now uint64) bool {
	if g.isRaider(o) {
		return g.standingRaider(o)
	}
	a, ok := g.animals.Get(o)
	return o != id && ok && (!a.Tame || a.Enraged(now)) && !a.Dead() && (g.hostile(a) || a.Enraged(now))
}

// dutyKeeps retains the engaged release target or a nearby guard target.
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

// dutyTarget picks the master's attack target, a nearby foe or recent attacker.
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

// decayAnimalTraining advances the persistent global maintenance of a tame animal.
func decayAnimalTraining(g *Game, id entity.EntityID, a *animal.State, now uint64) {
	m := trainingAt(g, id)
	before, haulBefore := a.Trained, m.HaulProgress
	feral := herd.AdvanceTraining(a, &m, now)
	saveTraining(g, id, m)
	if a.Trained != before || m.HaulProgress != haulBefore {
		g.viewGen++
	}
	if !feral {
		return
	}
	g.onAnimalLost(id)
	a.Trained, a.TrainDecay = [training.LegacySkills]uint8{}, 0
	herd.CancelHaul(animalHaulHost{haulHost{g}}, g.reserve, id, a, &m)
	saveTraining(g, id, training.Maintenance{})
	a.Master, a.Bond = 0, 0
	g.endMating(id, a)
	a.Tame, a.NoFollow = false, false
	a.Mode, a.Target, a.Path, a.PathI = animal.Wander, 0, nil, 0
	g.reserve.ReleaseOwner(id)
	g.reserve.ReleaseTarget(id)
	g.viewGen++
	if p, ok := g.positions.Get(id); ok {
		g.message(p.X, p.Y, "%s the %s became wild again.", a.Name, a.Species.Label())
	}
}

// resetAnimalTameness starts a new deadline without changing learned skills.
func (g *Game) resetAnimalTameness(id entity.EntityID, a *animal.State, now uint64) {
	a.TrainDecay = now
	m := trainingAt(g, id)
	m.TameLoss, m.DecayAt = 0, 0
	saveTraining(g, id, herd.StartDecay(*a, m, now))
}

// seedLegacyTrainingDecay: a pre-v21 save's tame animals, on a map or off it, start decay at load.
func seedLegacyTrainingDecay(g *Game) {
	seed := func(id entity.EntityID, a *animal.State) bool {
		saveTraining(g, id, herd.StartDecay(*a, trainingAt(g, id), uint64(g.clock.Now())))
		return false // every tame animal
	}
	eachMap(g, func() { g.animals.AnyTame(seed) })
	for _, r := range g.worldS().stashed {
		if r.Beast != nil && r.Beast.Tame {
			seed(r.ID, r.Beast)
		}
	}
}

func trainingAt(g *Game, id entity.EntityID) training.Maintenance {
	if g.worldSt == nil {
		return training.Maintenance{}
	}
	return g.worldSt.trainingMaintenance[id]
}
func saveTraining(g *Game, id entity.EntityID, m training.Maintenance) {
	if m == (training.Maintenance{}) {
		if g.worldSt != nil {
			delete(g.worldSt.trainingMaintenance, id)
		}
		return
	}
	w := g.worldS()
	if w.trainingMaintenance == nil {
		w.trainingMaintenance = make(map[entity.EntityID]training.Maintenance)
	}
	w.trainingMaintenance[id] = m
}

func maintenanceDue(g *Game, id entity.EntityID, a animal.State, now uint64) bool {
	return g.worldSt != nil && herd.MaintenanceDue(a, g.worldSt.trainingMaintenance[id], now)
}
