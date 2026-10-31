package scenario

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// Part kind names: the stable keys of the file format and the card titles
// of the editor.
var partKeys = [NumPartKinds]string{
	PartForcedTrait:       "forced_trait",
	PartAgeRange:          "age_range",
	PartForcedCondition:   "forced_condition",
	PartNoClothes:         "no_clothes",
	PartStartItem:         "start_item",
	PartScatterItem:       "scatter_item",
	PartStartAnimal:       "start_animal",
	PartCondition:         "condition",
	PartScheduledIncident: "scheduled_incident",
	PartDisabledIncident:  "disabled_incident",
	PartDisallowBuilding:  "disallow_building",
	PartResearch:          "research",
	PartResearchCost:      "research_cost",
	PartIntro:             "intro",
}

var partLabels = [NumPartKinds]string{
	PartForcedTrait:       "Forced trait",
	PartAgeRange:          "Age range",
	PartForcedCondition:   "Forced condition",
	PartNoClothes:         "No clothes",
	PartStartItem:         "Starting item",
	PartScatterItem:       "Scattered item",
	PartStartAnimal:       "Starting animal",
	PartCondition:         "Permanent condition",
	PartScheduledIncident: "Scheduled incident",
	PartDisabledIncident:  "Disabled incident",
	PartDisallowBuilding:  "Disallowed building",
	PartResearch:          "Research granted",
	PartResearchCost:      "Research cost",
	PartIntro:             "Intro text",
}

var partDescs = [NumPartKinds]string{
	PartForcedTrait:       "Gives starting colonists a chosen trait, to all of them or a random few, with a chance you set.",
	PartAgeRange:          "Limits how young or old the starting colonists can be.",
	PartForcedCondition:   "Starts colonists with a lasting health problem: an old fracture, poor eyesight, a hand tremor or exhaustion.",
	PartNoClothes:         "Colonists arrive with nothing to wear.",
	PartStartItem:         "Adds a stack of an item to the starting kit.",
	PartScatterItem:       "Scatters an item in piles around the landing site, near the start or anywhere on the map.",
	PartStartAnimal:       "Starts with tame animals: a domestic or pet species, or a random pet.",
	PartCondition:         "A condition that lasts the whole game: ashfall sky, endless chill, long summer or still air.",
	PartScheduledIncident: "Makes an incident happen on a chosen day, and optionally every few days after.",
	PartDisabledIncident:  "Stops an incident from ever happening.",
	PartDisallowBuilding:  "Forbids building one kind of structure.",
	PartResearch:          "Starts with a research project already finished.",
	PartResearchCost:      "Makes every research project cheaper or dearer, on top of the tech level.",
	PartIntro:             "A message shown once after landing.",
}

// Key is the part kind's stable file-format name.
func (k PartKind) Key() string {
	if k == PartNone || k >= NumPartKinds {
		return ""
	}
	return partKeys[k]
}

// Label names a part kind for the editor.
func (k PartKind) Label() string {
	if k == PartNone || k >= NumPartKinds {
		return ""
	}
	return partLabels[k]
}

// Desc is a one-sentence description of the part kind.
func (k PartKind) Desc() string {
	if k == PartNone || k >= NumPartKinds {
		return ""
	}
	return partDescs[k]
}

// ParsePartKind finds a part kind by its file-format name.
func ParsePartKind(key string) (PartKind, bool) {
	for k := PartKind(1); k < NumPartKinds; k++ {
		if partKeys[k] == key {
			return k, true
		}
	}
	return PartNone, false
}

// KindInfo describes an addable part kind.
type KindInfo struct {
	Kind        PartKind
	Label, Desc string
}

// Category is a group of addable part kinds in the Add part dialog.
type Category struct {
	Name  string
	Kinds []KindInfo
}

var catalogLayout = []struct {
	name  string
	kinds []PartKind
}{
	{"Starting people", []PartKind{PartForcedTrait, PartAgeRange, PartForcedCondition, PartNoClothes}},
	{"Items", []PartKind{PartStartItem, PartScatterItem, PartStartAnimal, PartResearch}},
	{"Conditions", []PartKind{PartCondition}},
	{"Incidents", []PartKind{PartScheduledIncident, PartDisabledIncident}},
	{"Restrictions", []PartKind{PartDisallowBuilding, PartResearchCost}},
	{"Presentation", []PartKind{PartIntro}},
}

// Catalog lists every addable part kind by category, in dialog order.
func Catalog() []Category {
	out := make([]Category, 0, len(catalogLayout))
	for _, c := range catalogLayout {
		cat := Category{Name: c.name}
		for _, k := range c.kinds {
			cat.Kinds = append(cat.Kinds, KindInfo{Kind: k, Label: k.Label(), Desc: k.Desc()})
		}
		out = append(out, cat)
	}
	return out
}

// NewPart is a valid part of kind k with sensible defaults for the editor
// to append.
func NewPart(k PartKind) Part {
	p := Part{Kind: k}
	switch k {
	case PartForcedTrait:
		p.Trait, p.Chance, p.Who = trait.Sunny, 100, WhoAll
	case PartAgeRange:
		p.MinAge, p.MaxAge = 18, 60
	case PartForcedCondition:
		p.PawnCond, p.Chance = pawngen.OldFracture, 50
	case PartStartItem:
		p.Item, p.Qty = item.SurvivalMeal, 20
	case PartScatterItem:
		p.Item, p.Qty, p.N, p.Near = item.Steel, 100, 1, true
	case PartStartAnimal:
		p.Species, p.N = animal.Goat, 1
	case PartCondition:
		p.Cond = AshfallSky
	case PartScheduledIncident:
		p.Incident, p.FirstDay = "herd_crossing", 10
	case PartDisabledIncident:
		p.Incident = "hive_swarm"
	case PartDisallowBuilding:
		p.Building = construction.HorseshoePin
	case PartResearch:
		p.Project = research.Traps
	case PartResearchCost:
		p.Percent = 100
	}
	return p
}

// Label names a permanent condition.
func (c Condition) Label() string {
	switch c {
	case AshfallSky:
		return "Ashfall sky"
	case EndlessChill:
		return "Endless chill"
	case LongSummer:
		return "Long summer"
	case StillAir:
		return "Still air"
	}
	return ""
}

// Desc describes a permanent condition's effect.
func (c Condition) Desc() string {
	switch c {
	case AshfallSky:
		return "-20% light, crops -15%"
	case EndlessChill:
		return "-8 °C all year"
	case LongSummer:
		return "+6 °C, temperature swing -30%"
	case StillAir:
		return "no rain or snow"
	}
	return ""
}

var conditionKeys = [...]string{AshfallSky: "ashfall_sky", EndlessChill: "endless_chill", LongSummer: "long_summer", StillAir: "still_air"}

// Key is the condition's stable file-format name.
func (c Condition) Key() string {
	if c == NoCondition || int(c) >= len(conditionKeys) {
		return ""
	}
	return conditionKeys[c]
}

// Conditions lists the permanent conditions in dropdown order.
func Conditions() []Condition { return []Condition{AshfallSky, EndlessChill, LongSummer, StillAir} }

// IncidentInfo is a scenario-schedulable incident.
type IncidentInfo struct{ Key, Label string }

var incidents = []IncidentInfo{
	{"raid", "Raid"},
	{"predator_pack", "Predator pack"},
	{"manhunter_pack", "Manhunter pack"},
	{"hive_swarm", "Hive swarm"},
	{"herd_crossing", "Herd crossing"},
	{"stray_joins", "Stray joins"},
	{"farm_animals", "Farm animals"},
}

// Incidents lists the incidents a scenario can schedule or disable.
func Incidents() []IncidentInfo { return append([]IncidentInfo(nil), incidents...) }

func validIncident(key string) bool {
	for _, i := range incidents {
		if i.Key == key {
			return true
		}
	}
	return false
}

func incidentLabel(key string) string {
	for _, i := range incidents {
		if i.Key == key {
			return i.Label
		}
	}
	return key
}

// Summary is the one-line text of a collapsed part card.
func (p Part) Summary() string {
	who := func() string {
		switch p.Who {
		case WhoOneRandom:
			return "one random colonist"
		case WhoNRandom:
			return fmt.Sprintf("%d random colonists", p.N)
		}
		return "every colonist"
	}
	switch p.Kind {
	case PartForcedTrait:
		return fmt.Sprintf("%s for %s, %d%%", trait.DefOf(p.Trait).Label, who(), p.Chance)
	case PartAgeRange:
		return fmt.Sprintf("Ages %d to %d", p.MinAge, p.MaxAge)
	case PartForcedCondition:
		return fmt.Sprintf("%s, %d%%", pawnConditionLabel(p.PawnCond), p.Chance)
	case PartNoClothes:
		return "Colonists start unclothed"
	case PartStartItem:
		return fmt.Sprintf("%d x %s", p.Qty, p.Item)
	case PartScatterItem:
		where := "anywhere"
		if p.Near {
			where = "near the start"
		}
		return fmt.Sprintf("%d x %s in %d piles, %s", p.Qty, p.Item, max(int(p.N), 1), where)
	case PartStartAnimal:
		if p.Random {
			return fmt.Sprintf("%d random pet", max(int(p.N), 1))
		}
		return fmt.Sprintf("%d x %s", max(int(p.N), 1), p.Species.Label())
	case PartCondition:
		return p.Cond.Label()
	case PartScheduledIncident:
		s := fmt.Sprintf("%s on day %d", incidentLabel(p.Incident), p.FirstDay)
		if p.EveryDays > 0 {
			s += fmt.Sprintf(", every %d days", p.EveryDays)
		}
		return s
	case PartDisabledIncident:
		return incidentLabel(p.Incident) + " never happens"
	case PartDisallowBuilding:
		return p.Building.String() + " cannot be built"
	case PartResearch:
		return research.DefOf(p.Project).Label
	case PartResearchCost:
		return fmt.Sprintf("%d%% of normal", p.Percent)
	case PartIntro:
		t := strings.Join(strings.Fields(p.Text), " ")
		if r := []rune(t); len(r) > 40 {
			t = string(r[:40]) + "..."
		}
		return t
	}
	return ""
}

var pawnConditionKeys = [...]string{
	pawngen.OldFracture: "old_fracture", pawngen.Nearsighted: "nearsighted",
	pawngen.HandTremor: "hand_tremor", pawngen.Exhausted: "exhausted",
}

var pawnConditionLabels = [...]string{
	pawngen.OldFracture: "Old fracture", pawngen.Nearsighted: "Nearsighted",
	pawngen.HandTremor: "Hand tremor", pawngen.Exhausted: "Exhausted",
}

func pawnConditionLabel(c pawngen.Condition) string {
	if c == pawngen.NoCondition || int(c) >= len(pawnConditionLabels) {
		return ""
	}
	return pawnConditionLabels[c]
}

// PawnConditions lists the forced-condition choices in dropdown order.
func PawnConditions() []pawngen.Condition {
	return []pawngen.Condition{pawngen.OldFracture, pawngen.Nearsighted, pawngen.HandTremor, pawngen.Exhausted}
}

// PawnConditionLabel names a forced pawn condition for the editor.
func PawnConditionLabel(c pawngen.Condition) string { return pawnConditionLabel(c) }
