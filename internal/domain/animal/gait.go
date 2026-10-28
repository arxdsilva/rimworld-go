package animal

// Gait (PLAY-022 C4) is how fast an animal moves and how it looks doing
// it: it ambles when it has nowhere to be, walks on an errand and runs
// from or after something. The sim fixes a step's gait when the step
// starts (StepGait), so a mode change mid-step never changes that step's
// ticks or label.
type Gait uint8

const (
	// GaitNone: no step taken yet (a new animal, or a save from before
	// gaits); the mode alone says how fast it moves.
	GaitNone Gait = iota
	// GaitAmble: wandering, grazing, penned or herd milling, at twice
	// the walking step.
	GaitAmble
	// GaitWalk: an errand (to food, a bed, a pen or trough; led on a
	// rope; walking off the map), at the species' step.
	GaitWalk
	// GaitRun: fleeing, hunting, charging, attacking, manhunting.
	GaitRun
)

var gaitNames = [...]string{GaitNone: "", GaitAmble: "amble", GaitWalk: "walk", GaitRun: "run"}

func (g Gait) String() string {
	if int(g) < len(gaitNames) {
		return gaitNames[g]
	}
	return ""
}

// ambleMilli is the amble step over the walking step, permille: twice as
// many ticks a tile (our number; PLAY-022 asks for 1.8..2.2).
const ambleMilli = 2000

// GaitAt is the gait of the next step s takes at tick now. Fleeing,
// hunting, attacking and a manhunter's rage run; an errand (State.Errand),
// walking off the map, and going to food or a bed walk; the rest amble.
func (s State) GaitAt(now uint64) Gait {
	switch {
	case s.Mode == Flee || s.Mode == Attack || s.Mode == Hunt || s.Enraged(now):
		return GaitRun
	case s.Errand || s.Leaving || s.Mode == Feed || s.Mode == Rest || s.Mode == Leave:
		return GaitWalk
	}
	return GaitAmble
}

// StepAt is the ticks per tile at gait g, slowed by a hurt body as Step
// is. Walk is the species' step. Run is the flee step while the animal
// flees; hunting, charging and attacking report run but keep the walking
// step (they were never faster, so no fight changes). Amble is twice the
// walk. GaitNone is Step, the mode's own pace.
func (s State) StepAt(g Gait) int {
	d := DefOf(s.Species)
	st := d.StepTicks
	switch g {
	case GaitNone:
		return s.Step()
	case GaitAmble:
		st = (st*ambleMilli + 500) / 1000
	case GaitRun:
		if s.Mode == Flee && d.FleeStep > 0 {
			st = d.FleeStep
		}
	}
	return s.slowedByBody(st)
}

// StepCostAt is the ticks for one step at gait g: StepAt straight, times
// 1.414 (rounded) diagonally, as StepCost.
func (s State) StepCostAt(g Gait, diagonal bool) int {
	st := s.StepAt(g)
	if diagonal {
		return (st*1414 + 500) / 1000
	}
	return st
}
