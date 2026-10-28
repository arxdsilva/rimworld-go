package game

import "rimworld-go/internal/domain/animal"

// viewGait is AnimalView.Gait (PLAY-022 C4): the gait of the step the
// animal is taking. It is the gait the step started with (stepAnimal
// writes it to State.StepGait), not what the animal's mode says now. It
// reads "" while the animal is not moving: before its first step, lying
// down, or once its last step has ended with no cell left on its path.
func viewGait(a animal.State, m motionRec, now uint64, hasNext bool) string {
	if a.StepGait == animal.GaitNone || lyingDown(a) {
		return ""
	}
	if !hasNext && now >= m.StepTick+uint64(m.StepTicks) {
		return ""
	}
	switch a.StepGait {
	case animal.GaitAmble:
		return GaitAmble
	case animal.GaitWalk:
		return GaitWalk
	case animal.GaitRun:
		return GaitRun
	}
	return ""
}
