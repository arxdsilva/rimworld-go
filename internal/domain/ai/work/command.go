package work

import "rimworld-go/internal/domain/sim/command"

// FromCommand is the work type of a work column id (command.WorkType, the
// SetWorkEnabled column); ok false for none or an unknown one. Moved from
// the game package (AX-2 budget).
func FromCommand(c command.WorkType) (Type, bool) {
	switch c {
	case command.WorkHaul:
		return Haul, true
	case command.WorkConstruct:
		return Construct, true
	case command.WorkGrow:
		return Grow, true
	case command.WorkMine:
		return Mine, true
	case command.WorkCook:
		return Cook, true
	case command.WorkHunt:
		return Hunt, true
	case command.WorkCraft:
		return Craft, true
	case command.WorkDoctor:
		return Doctor, true
	case command.WorkResearch:
		return Research, true
	case command.WorkClean:
		return Clean, true
	case command.WorkHandle:
		return Handle, true
	case command.WorkWarden:
		return Warden, true
	case command.WorkFirefight:
		return Firefight, true
	case command.WorkPlantCut:
		return PlantCut, true
	}
	return 0, false
}
