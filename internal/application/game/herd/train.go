package herd

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal training's pure rules (ENG-003, M10), moved from the game
// package's train_animals.go (AX-7 paid its game lines with them); the
// sessions, duties and views stay there. Which skills are on lives in the
// high bit of animal.State.Trained: TrainToggled marks a skill the player
// flipped away from its default (Obedience on, the others off).

const (
	// TrainCooldown and TrainJitter: a session is due this long (plus up
	// to TrainJitter, by id) after the last.
	TrainCooldown = 3 * clock.TicksPerHour
	TrainJitter   = 600
	// TrainToggled: Trained[s] bit set when s is not at its default.
	TrainToggled uint8 = 0x80
)

var trainLabels = [training.NumSkills]string{training.Obedience: "Obedience", training.Guard: "Guard", training.Release: "Release"}

// TrainLabel is a skill's display name.
func TrainLabel(s training.Skill) string {
	if s >= training.NumSkills {
		return ""
	}
	return trainLabels[s]
}

// CanLearn reports whether species sp can learn s at all: Obedience any
// tame animal but hive insects, Guard fighters and pets, Release
// predators.
func CanLearn(sp animal.Species, s training.Skill) bool {
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

// TrainDefault is whether s is on for a new animal.
func TrainDefault(s training.Skill) bool { return s == training.Obedience }

// TrainOn reports whether skill s is on for a.
func TrainOn(a animal.State, s training.Skill) bool {
	if s >= training.NumSkills || !CanLearn(a.Species, s) {
		return false
	}
	return TrainDefault(s) != (a.Trained[s]&TrainToggled != 0)
}

// TrainSteps is a's steps of s as of now (decayed since the last
// session).
func TrainSteps(a animal.State, s training.Skill, now uint64) uint8 {
	if s >= training.NumSkills {
		return 0
	}
	steps := a.Trained[s] &^ TrainToggled
	if a.TrainDecay == 0 || now <= a.TrainDecay {
		return steps
	}
	return training.Decay(s, steps, now-a.TrainDecay)
}

// TrainLearned reports whether a knows s now.
func TrainLearned(a animal.State, s training.Skill, now uint64) bool {
	return training.Learned(s, TrainSteps(a, s, now))
}

// NextTrainSkill is the skill a's next session works on: the first on and
// unlearned one, Guard and Release only once Obedience is learned.
func NextTrainSkill(a animal.State, now uint64) (training.Skill, bool) {
	for s := training.Skill(0); s < training.NumSkills; s++ {
		if !TrainOn(a, s) || TrainLearned(a, s, now) {
			continue
		}
		if s != training.Obedience && !TrainLearned(a, training.Obedience, now) {
			continue
		}
		return s, true
	}
	return 0, false
}

// TrainDue reports whether animal id's next session may start now.
func TrainDue(id entity.EntityID, a animal.State, now uint64) bool {
	return a.TrainDecay == 0 || now >= a.TrainDecay+TrainCooldown+uint64(id)%TrainJitter
}
