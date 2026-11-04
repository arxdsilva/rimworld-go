package dossierview

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/skill"
)

// NoneID is Card.ActiveSkill and Card.WorkType when there is none.
const NoneID = -1

// New is an empty card: nothing practised, no work type.
func New() Card { return Card{ActiveSkill: NoneID, WorkType: NoneID} }

// TicksLeft is what is left of a timer ending at until, 0 once it has run out.
func TicksLeft(until, now uint64) int {
	if until <= now {
		return 0
	}
	return int(until - now)
}

// RunsBill reports a job that works a bench's bill, whose work type is the
// recipe's rather than the job's.
func RunsBill(k job.Kind) bool {
	return k == job.Bill || k == job.HaulIngredients || k == job.PackIngredients
}

// WorkTypeOf is the work type whose priority starts a job of kind k (NoneID
// for free time, needs, drafted orders, and bills: see RunsBill). The kinds
// follow Game.assignWorkType, which has the only such mapping in the game.
func WorkTypeOf(k job.Kind) int {
	t := work.NumTypes
	switch k {
	case job.FightFire:
		t = work.Firefight
	case job.Rescue, job.Tend, job.EmergencyTend, job.Surgery, job.TendAnimal, job.FeedPatient:
		t = work.Doctor
	case job.Harvest, job.Sow:
		t = work.Grow
	case job.Cut:
		t = work.PlantCut
	case job.Construct, job.Deconstruct, job.Repair, job.Roof, job.Unroof, job.Smooth, job.Flick, job.Uninstall, job.Install, job.RemoveFloor:
		t = work.Construct
	case job.Hunt:
		t = work.Hunt
	case job.Haul, job.FillHopper, job.Refuel, job.Bury, job.Strip, job.OpenContainer, job.LoadPods, job.RearmTurret, job.UnloadPack:
		t = work.Haul
	case job.Clean, job.ClearSnow, job.CleanRoom:
		t = work.Clean
	case job.Research, job.ScanGround, job.ScanMinerals:
		t = work.Research
	case job.Mine, job.OperateDrill:
		t = work.Mine
	case job.Tame, job.Milk, job.Shear, job.Train, job.Rope, job.Slaughter, job.StockTank, job.NetTank:
		t = work.Handle
	case job.Warden, job.DeliverFood, job.CarryToPrisonBed:
		t = work.Warden
	}
	if t == work.NumTypes {
		return NoneID
	}
	return int(t)
}

// Facts are the sim's numbers behind the skill tooltip's extra lines.
type Facts struct {
	Animals, Cooking, Medicine int // skill levels
	Melee                      attack.Melee
	MineYieldPct, CropYieldPct int // the worker's skill and the difficulty
	ButcherDifficultyPct       int
}

// Surgery line's assumptions: a plain clean floor in full light, no hospital
// bed or monitor, herbal medicine, a plain install.
const surgeryLineLight = 100

// Lines are the skill tooltip's facts that need the sim. Each comes from the
// formula the work itself uses; StatLine holds whole percents, so a chance
// under one percent reads 0 or 1.
func Lines(f Facts) [skill.NumSkills][]StatLine {
	var out [skill.NumSkills][]StatLine
	surgery, _ := body.SurgeryChance(body.SurgeryInput{Skill: f.Medicine, Manip: int32(f.Melee.Manip), Sight: int32(f.Melee.Sight),
		LightPct: surgeryLineLight, Op: body.InstallImplant, Medicine: item.HerbalMedicine})
	out[skill.Animals] = []StatLine{{"Training success", int(training.StepChance(f.Animals, false)) / 10}}
	out[skill.Cooking] = []StatLine{
		{"Food poison chance from the cook", (diet.CookChance(f.Cooking) + 50) / 100},
		{"Butchery yield", animal.ButcheryEfficiency(f.Cooking) * f.ButcherDifficultyPct / 1000},
	}
	out[skill.Medicine] = []StatLine{{"Surgery success in a clean, lit room with herbal medicine", int(surgery) / 10}}
	out[skill.Melee] = []StatLine{
		{"Melee hit chance", int(attack.HitChanceOf(f.Melee)) / 10},
		{"Melee dodge chance", int(attack.DodgeChanceOf(f.Melee)) / 10},
	}
	out[skill.Mining] = []StatLine{{"Mining yield", f.MineYieldPct}}
	out[skill.Plants] = []StatLine{{"Harvest yield", f.CropYieldPct}}
	return out
}

// maxNameRunes is the wizard's limit on a name (roster.MaxNameRunes).
const maxNameRunes = 16

// CleanName is a rename's name, trimmed; "" asks for the generated name.
// false refuses it, by the wizard's rule: at most maxNameRunes runes of
// letters, spaces, apostrophes and hyphens.
func CleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxNameRunes {
		return "", false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && r != ' ' && r != '\'' && r != '-' {
			return "", false
		}
	}
	return s, true
}
