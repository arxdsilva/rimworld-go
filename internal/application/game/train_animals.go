package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal training (ENG-003, M10; Handle work). A handler trains a tame
// animal's first enabled, unlearned skill (Obedience before Guard and
// Release) once its session is due: trainCooldown (plus a per-animal
// stagger) after the last one, success or not, so neither handler nor
// animal flaps between sessions. A session succeeds with
// training.StepChance (x5 for the bonded trainer) and a success may bond
// them (training.BondTrain). Skills decay from the last session
// (training.Decay, read on demand: the saved steps are those of the last
// session, so a learned Obedience keeps its floor). An animal that has
// learned Obedience follows its drafted master.
//
// Which skills are on lives in the high bit of animal.State.Trained:
// trainToggled marks a skill the player flipped away from its default
// (Obedience on, the others off), so older saves and new animals need no
// extra field.

const (
	// trainWork: the ticks one session takes (like a taming).
	trainWork = animal.TameWork
	// trainCooldown and trainJitter: a session is due this long (plus up
	// to trainJitter, by id) after the last.
	trainCooldown = 3 * clock.TicksPerHour
	trainJitter   = 600
	// trainToggled: Trained[s] bit set when s is not at its default.
	trainToggled uint8 = 0x80
	// followFar: an obedient animal walks to its drafted master once it
	// is farther than this; it then goes right up to them (followNear).
	followFar  = 3
	followNear = 1
)

var trainLabels = [training.NumSkills]string{training.Obedience: "Obedience", training.Guard: "Guard", training.Release: "Release"}

// trainLabel is a skill's display name.
func trainLabel(s training.Skill) string {
	if s >= training.NumSkills {
		return ""
	}
	return trainLabels[s]
}

// canLearn reports whether species sp can learn s at all: Obedience any
// tame animal but hive insects, Guard fighters and pets, Release
// predators.
func canLearn(sp animal.Species, s training.Skill) bool {
	d := animal.DefOf(sp)
	if d.Hive {
		return false
	}
	switch s {
	case training.Obedience:
		return true
	case training.Guard:
		return animal.Fighter(sp) || animal.RosterOf(sp).Behaviour == animal.Pet
	case training.Release:
		return d.Predator
	}
	return false
}

// trainDefault is whether s is on for a new animal.
func trainDefault(s training.Skill) bool { return s == training.Obedience }

// trainOn reports whether skill s is on for a.
func trainOn(a animal.State, s training.Skill) bool {
	if s >= training.NumSkills || !canLearn(a.Species, s) {
		return false
	}
	return trainDefault(s) != (a.Trained[s]&trainToggled != 0)
}

// trainSteps is a's steps of s as of now (decayed since the last
// session).
func trainSteps(a animal.State, s training.Skill, now uint64) uint8 {
	if s >= training.NumSkills {
		return 0
	}
	steps := a.Trained[s] &^ trainToggled
	if a.TrainDecay == 0 || now <= a.TrainDecay {
		return steps
	}
	return training.Decay(s, steps, now-a.TrainDecay)
}

// trainLearned reports whether a knows s now.
func trainLearned(a animal.State, s training.Skill, now uint64) bool {
	return training.Learned(s, trainSteps(a, s, now))
}

// nextTrainSkill is the skill a's next session works on: the first on and
// unlearned one, Guard and Release only once Obedience is learned.
func nextTrainSkill(a animal.State, now uint64) (training.Skill, bool) {
	for s := training.Skill(0); s < training.NumSkills; s++ {
		if !trainOn(a, s) || trainLearned(a, s, now) {
			continue
		}
		if s != training.Obedience && !trainLearned(a, training.Obedience, now) {
			continue
		}
		return s, true
	}
	return 0, false
}

// trainDue reports whether animal id's next session may start now.
func trainDue(id entity.EntityID, a animal.State, now uint64) bool {
	return a.TrainDecay == 0 || now >= a.TrainDecay+trainCooldown+uint64(id)%trainJitter
}

// trainable reports whether a handler may train animal id now.
func (g *Game) trainable(id entity.EntityID, a animal.State, now uint64) bool {
	if !a.Tame || a.Dead() || a.Enraged(now) || sleeping(a) || a.Leaving ||
		a.Mode == animal.Flee || a.Mode == animal.Attack || a.Mode == animal.Confused || !trainDue(id, a, now) {
		return false
	}
	_, ok := nextTrainSkill(a, now)
	return ok
}

// cmdSetTraining turns training skill on or off for animal.
func (g *Game) cmdSetTraining(id entity.EntityID, s int, on bool) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame || s < 0 || s >= int(training.NumSkills) || !canLearn(a.Species, training.Skill(s)) {
		return
	}
	sk := training.Skill(s)
	if on == trainDefault(sk) {
		a.Trained[sk] &^= trainToggled
	} else {
		a.Trained[sk] |= trainToggled
	}
	g.animals.Set(id, a)
}

// assignTrain sends handler id to train the nearest tame animal whose
// session is due.
func (g *Game) assignTrain(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	if !g.anyAnimal(func(aid entity.EntityID, a animal.State) bool { return g.trainable(aid, a, now) }) {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || !g.trainable(aid, a, now) {
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
	s, ok := nextTrainSkill(a, now)
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
		a.Trained[k] = a.Trained[k]&trainToggled | trainSteps(*a, k, now)
	}
	a.TrainDecay = now
	chance := training.StepChance(int(g.skillOf(trainer, skill.Animals).Level), a.Bond == uint64(trainer))
	if int32(g.stream(streamBond).IntN(1000)) >= chance {
		return
	}
	a.Trained[s]++
	if training.Learned(s, a.Trained[s]&^trainToggled) {
		p, _ := g.positions.Get(id)
		g.message(p.X, p.Y, "%s trained %s the %s in %s.", g.name(trainer), a.Name, a.Species.Label(), trainLabel(s))
	}
	g.tryBond(id, a, trainer, training.BondTrain)
}

// followMaster: an animal that has learned Obedience stays by its drafted
// master, walking up to them once they are more than followFar away. It
// reports true while it follows.
func (g *Game) followMaster(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	if a.Master == 0 || !a.Tame || !trainLearned(*a, training.Obedience, now) {
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

// trainViews is a's training rows for the view: every skill, with whether
// it can be learned, is on, and its steps.
func trainViews(a animal.State, now uint64) ([]bool, []AnimalTrainingView) {
	learned := make([]bool, training.NumSkills)
	rows := make([]AnimalTrainingView, training.NumSkills)
	for s := training.Skill(0); s < training.NumSkills; s++ {
		learned[s] = trainLearned(a, s, now)
		rows[s] = AnimalTrainingView{Skill: int(s), Label: trainLabel(s), Allowed: canLearn(a.Species, s),
			On: trainOn(a, s), Learned: learned[s], Steps: int(trainSteps(a, s, now)), Needed: int(training.Needed(s))}
	}
	return learned, rows
}
