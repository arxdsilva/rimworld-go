// Package scenario is how a colony starts: who arrives, how, with what,
// and the special rules that last the whole game. Built-in scenarios plus
// custom ones from the editor, encoded as versioned JSON (NG-1 §3.1, §4.4).
package scenario

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
	"rimworld-go/pkg/randx"
)

// Arrival is how the colonists reach the map.
type Arrival uint8

const (
	DropPods Arrival = iota
	Standing
	WalkIn
)

// PartKind is an optional scenario part (append only, saved in files).
type PartKind uint8

const (
	PartNone PartKind = iota
	PartForcedTrait
	PartAgeRange
	PartForcedCondition
	PartNoClothes
	PartStartItem
	PartScatterItem
	PartStartAnimal
	PartCondition
	PartScheduledIncident
	PartDisabledIncident
	PartDisallowBuilding
	PartResearch
	PartResearchCost
	PartIntro
	NumPartKinds
)

// Who a forced part applies to.
type Who uint8

const (
	WhoAll Who = iota
	WhoOneRandom
	WhoNRandom
)

// Condition is a permanent map/world condition.
type Condition uint8

const (
	NoCondition Condition = iota
	AshfallSky
	EndlessChill
	LongSummer
	StillAir
)

// Part is one optional rule (a tagged struct; unused fields are zero).
type Part struct {
	Kind     PartKind
	Trait    trait.ID
	Chance   uint8
	Who      Who
	N        uint8
	MinAge   uint8
	MaxAge   uint8
	PawnCond pawngen.Condition
	Item     item.Kind
	Qty      int32
	Near     bool
	Species  animal.Species
	// Random makes a StartAnimal part pick N animals from the pet set
	// (cat, labrador, husky, guinea pig) with the scenario's seed instead
	// of using Species.
	Random    bool
	Cond      Condition
	Incident  string
	FirstDay  uint16
	EveryDays uint16
	Building  construction.Kind
	Project   research.Project
	Percent   uint16
	Text      string
}

// Def is a scenario.
type Def struct {
	Key, Name, Summary, Desc string
	Builtin                  bool
	Seed                     uint64
	Faction                  faction.Type
	People, ChooseFrom       int
	Arrival                  Arrival
	Tech                     faction.Tech
	Hardness                 uint8
	Parts                    []Part
}

// Severity of a validation problem.
type Severity uint8

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Problem is a validation finding; Part is the index into Parts, -1 for the
// mandatory fields.
type Problem struct {
	Part     int
	Severity Severity
	Msg      string
}

// Stack is a starting or scattered item stack. Piles is how many separate
// heaps a scattered stack is split into (1 for starting items).
type Stack struct {
	Kind  item.Kind
	Qty   int
	Near  bool
	Piles int
}

// Sched is a scheduled incident.
type Sched struct {
	Incident            string
	FirstDay, EveryDays int
}

// Effects is a scenario flattened for the game.
type Effects struct {
	People, ChooseFrom int
	Arrival            Arrival
	Tech               faction.Tech
	Faction            faction.Type
	StartItems         []Stack
	Scatter            []Stack
	Animals            []animal.Species
	Research           []research.Project
	ResearchCostPct    int
	Conditions         []Condition
	Scheduled          []Sched
	Disabled           map[string]bool
	Disallowed         map[construction.Kind]bool
	Rules              pawngen.Rules
	Intro              string
}

// Limits of the editor's fields (NG-1 §2.3).
const (
	MaxName      = 40
	MaxSummary   = 90
	MaxDesc      = 1200
	MaxIntro     = 1500
	MinPeople    = 1
	MaxPeople    = 10
	MaxExtraPool = 8 // "choose from" is at most People+8
	MinChance    = 5
	MaxChance    = 100
	MinAge       = 14
	MaxAge       = 80
	MinQty       = 1
	MaxQty       = 5000
	MaxWeaponQty = 10
	MaxAnimals   = 5
	MinFirstDay  = 1
	MaxFirstDay  = 120
	MaxEveryDays = 60
	MinCostPct   = 50
	MaxCostPct   = 300
	MaxHardness  = 4
)

// Pet set for a random starting pet.
var petSet = []animal.Species{animal.Cat, animal.Labrador, animal.Husky, animal.GuineaPig}

// PetSet lists the species a random starting pet is drawn from.
func PetSet() []animal.Species { return append([]animal.Species(nil), petSet...) }

// TechCostPct is the research cost percent a tech level implies.
func TechCostPct(t faction.Tech) int {
	switch t {
	case faction.Neolithic:
		return 150
	case faction.Spacer:
		return 80
	}
	return 100
}

// backstoryTag maps a tech level to the backstory fit tag.
func backstoryTag(t faction.Tech) backstory.Tag {
	switch t {
	case faction.Neolithic:
		return backstory.TagNeolithic
	case faction.Spacer:
		return backstory.TagSpacer
	}
	return backstory.TagIndustrial
}

// salt keeps the scenario's own random draws apart from other streams.
const salt = 0x7363656e // "scen"

// Apply flattens d for the game. It never fails: out-of-range values are
// passed through for Validate to flag, and unusable parts are skipped.
func (d Def) Apply() Effects {
	e := Effects{
		People: d.People, ChooseFrom: d.ChooseFrom, Arrival: d.Arrival, Tech: d.Tech, Faction: d.Faction,
		ResearchCostPct: TechCostPct(d.Tech),
		Disabled:        map[string]bool{},
		Disallowed:      map[construction.Kind]bool{},
		Rules:           pawngen.Rules{Tech: backstoryTag(d.Tech)},
	}
	rng := randx.New(d.Seed ^ salt)
	haveCond, haveIntro := false, false
	for _, p := range d.Parts {
		switch p.Kind {
		case PartForcedTrait:
			e.Rules.Forced = append(e.Rules.Forced, pawngen.ForcedTrait{Trait: p.Trait, Chance: forcedChance(p, d.People)})
		case PartAgeRange:
			if e.Rules.MinAge == 0 || int(p.MinAge) > e.Rules.MinAge {
				e.Rules.MinAge = int(p.MinAge)
			}
			if e.Rules.MaxAge == 0 || int(p.MaxAge) < e.Rules.MaxAge {
				e.Rules.MaxAge = int(p.MaxAge)
			}
		case PartForcedCondition:
			if !haveCond {
				e.Rules.ForcedCond, e.Rules.CondChance = p.PawnCond, p.Chance
				haveCond = true
			}
		case PartNoClothes:
			e.Rules.NoClothes = true
		case PartStartItem:
			e.StartItems = append(e.StartItems, Stack{Kind: p.Item, Qty: int(p.Qty), Near: true, Piles: 1})
		case PartScatterItem:
			e.Scatter = append(e.Scatter, Stack{Kind: p.Item, Qty: int(p.Qty), Near: p.Near, Piles: max(int(p.N), 1)})
		case PartStartAnimal:
			for i := 0; i < max(int(p.N), 1); i++ {
				if p.Random {
					e.Animals = append(e.Animals, petSet[rng.IntN(len(petSet))])
				} else {
					e.Animals = append(e.Animals, p.Species)
				}
			}
		case PartCondition:
			if !hasCondition(e.Conditions, p.Cond) {
				e.Conditions = append(e.Conditions, p.Cond)
			}
		case PartScheduledIncident:
			e.Scheduled = append(e.Scheduled, Sched{Incident: p.Incident, FirstDay: int(p.FirstDay), EveryDays: int(p.EveryDays)})
		case PartDisabledIncident:
			e.Disabled[p.Incident] = true
		case PartDisallowBuilding:
			e.Disallowed[p.Building] = true
		case PartResearch:
			if !hasProject(e.Research, p.Project) {
				e.Research = append(e.Research, p.Project)
			}
		case PartResearchCost:
			e.ResearchCostPct = e.ResearchCostPct * int(p.Percent) / 100
		case PartIntro:
			if !haveIntro {
				e.Intro, haveIntro = p.Text, true
			}
		}
	}
	return e
}

// forcedChance turns a forced-trait part into the per-colonist chance
// pawngen takes: "all" is the part's chance; "one random" and "N random"
// spread it over the starting people (N/People of the chance), at least 1%.
func forcedChance(p Part, people int) uint8 {
	if p.Who == WhoAll || people <= 0 {
		return p.Chance
	}
	n := 1
	if p.Who == WhoNRandom {
		n = int(p.N)
	}
	n = min(max(n, 1), people)
	return uint8(max(1, (int(p.Chance)*n+people/2)/people))
}

func hasCondition(l []Condition, c Condition) bool {
	for _, v := range l {
		if v == c {
			return true
		}
	}
	return false
}

func hasProject(l []research.Project, p research.Project) bool {
	for _, v := range l {
		if v == p {
			return true
		}
	}
	return false
}

// ---- validation ----

// Validate checks d (errors block, warnings don't).
func (d Def) Validate() []Problem {
	var out []Problem
	add := func(part int, sev Severity, format string, a ...any) {
		out = append(out, Problem{Part: part, Severity: sev, Msg: fmt.Sprintf(format, a...)})
	}
	if strings.TrimSpace(d.Name) == "" {
		add(-1, Error, "The scenario needs a name.")
	}
	tooLong := func(field, s string, limit int) {
		if n := utf8.RuneCountInString(s); n > limit {
			add(-1, Error, "%s is %d characters; the limit is %d.", field, n, limit)
		}
	}
	tooLong("The name", d.Name, MaxName)
	tooLong("The summary", d.Summary, MaxSummary)
	tooLong("The description", d.Desc, MaxDesc)
	if d.People < MinPeople || d.People > MaxPeople {
		add(-1, Error, "Starting people must be %d to %d (it is %d).", MinPeople, MaxPeople, d.People)
	}
	if d.ChooseFrom < d.People {
		add(-1, Error, "Choose from (%d) cannot be fewer than the starting people (%d).", d.ChooseFrom, d.People)
	} else if d.ChooseFrom > d.People+MaxExtraPool {
		add(-1, Error, "Choose from can be at most %d more than the starting people.", MaxExtraPool)
	}
	if d.Faction != faction.Settlers && d.Faction != faction.Clans {
		add(-1, Error, "The player faction must be Settlers or Clan.")
	}
	if d.Arrival > WalkIn {
		add(-1, Error, "Unknown arrival.")
	}
	if d.Tech > faction.Spacer {
		add(-1, Error, "Unknown tech level.")
	}
	if d.Hardness > MaxHardness {
		add(-1, Error, "Hardness must be 0 to %d.", MaxHardness)
	}

	var forced []int // indexes of ForcedTrait parts seen
	scheduled := map[string]int{}
	disabled := map[string]int{}
	condSeen := map[Condition]bool{}
	foodInKit, firstForcedCond := false, -1
	for i, p := range d.Parts {
		if p.Kind == PartNone || p.Kind >= NumPartKinds {
			add(i, Error, "Unknown part kind %d.", p.Kind)
			continue
		}
		label := p.Kind.Label()
		switch p.Kind {
		case PartForcedTrait:
			if p.Trait == trait.None || trait.DefOf(p.Trait).Key == "" {
				add(i, Error, "%s: the trait does not exist.", label)
			} else {
				for _, j := range forced {
					if trait.Conflicts(p.Trait, d.Parts[j].Trait) {
						add(i, Error, "%s: %s conflicts with %s (part %d).", label,
							trait.DefOf(p.Trait).Label, trait.DefOf(d.Parts[j].Trait).Label, j+1)
					}
				}
				forced = append(forced, i)
			}
			checkChance(add, i, label, p.Chance)
			if p.Who > WhoNRandom {
				add(i, Error, "%s: unknown 'who'.", label)
			}
			if p.Who == WhoNRandom && (p.N < 1 || int(p.N) > d.People) {
				add(i, Error, "%s: N must be between 1 and the number of starting people.", label)
			}
		case PartAgeRange:
			if p.MinAge < MinAge || p.MaxAge > MaxAge {
				add(i, Error, "%s: ages must be between %d and %d.", label, MinAge, MaxAge)
			}
			if p.MinAge > p.MaxAge {
				add(i, Error, "%s: the minimum age (%d) is above the maximum (%d).", label, p.MinAge, p.MaxAge)
			}
		case PartForcedCondition:
			if p.PawnCond == pawngen.NoCondition || p.PawnCond > pawngen.Exhausted {
				add(i, Error, "%s: unknown condition.", label)
			}
			checkChance(add, i, label, p.Chance)
			if firstForcedCond >= 0 {
				add(i, Warning, "%s: only the first forced condition (part %d) is used.", label, firstForcedCond+1)
			} else {
				firstForcedCond = i
			}
		case PartStartItem, PartScatterItem:
			if !playerItem(p.Item) {
				add(i, Error, "%s: unknown item.", label)
				break
			}
			limit := MaxQty
			if p.Item.IsWeapon() {
				limit = MaxWeaponQty
			}
			if int(p.Qty) < MinQty || int(p.Qty) > limit {
				add(i, Error, "%s: quantity of %s must be %d to %d.", label, p.Item, MinQty, limit)
			}
			if p.Kind == PartStartItem && p.Item.IsFood() && p.Qty > 0 {
				foodInKit = true
			}
			if p.Kind == PartScatterItem && p.N > 20 {
				add(i, Error, "%s: at most 20 piles.", label)
			}
		case PartStartAnimal:
			if !p.Random && !startAnimal(p.Species) {
				add(i, Error, "%s: pick a domestic or pet species.", label)
			}
			if p.N < 1 || p.N > MaxAnimals {
				add(i, Error, "%s: the count must be 1 to %d.", label, MaxAnimals)
			}
		case PartCondition:
			if p.Cond == NoCondition || p.Cond > StillAir {
				add(i, Error, "%s: unknown condition.", label)
			} else if condSeen[p.Cond] {
				add(i, Error, "%s: %s is already in the scenario (one of each).", label, p.Cond.Label())
			}
			condSeen[p.Cond] = true
		case PartScheduledIncident:
			if !validIncident(p.Incident) {
				add(i, Error, "%s: unknown incident %q.", label, p.Incident)
			}
			if p.FirstDay < MinFirstDay || p.FirstDay > MaxFirstDay {
				add(i, Error, "%s: the first day must be %d to %d.", label, MinFirstDay, MaxFirstDay)
			}
			if p.EveryDays > MaxEveryDays {
				add(i, Error, "%s: repeating every %d days is over the limit of %d.", label, p.EveryDays, MaxEveryDays)
			}
			if _, ok := scheduled[p.Incident]; !ok {
				scheduled[p.Incident] = i
			}
		case PartDisabledIncident:
			if !validIncident(p.Incident) {
				add(i, Error, "%s: unknown incident %q.", label, p.Incident)
			}
			if _, ok := disabled[p.Incident]; !ok {
				disabled[p.Incident] = i
			}
		case PartDisallowBuilding:
			if !p.Building.Valid() {
				add(i, Error, "%s: unknown building.", label)
				break
			}
			if essentialBuilding(p.Building) {
				add(i, Warning, "%s: disallowing %s makes the early game much harder.", label, p.Building)
			}
		case PartResearch:
			if p.Project >= research.NumProjects {
				add(i, Error, "%s: unknown research project.", label)
			}
		case PartResearchCost:
			if p.Percent < MinCostPct || p.Percent > MaxCostPct {
				add(i, Error, "%s: the percent must be %d to %d.", label, MinCostPct, MaxCostPct)
			}
		case PartIntro:
			tooLong("The intro text", p.Text, MaxIntro)
		}
	}
	for inc, di := range disabled {
		if _, ok := scheduled[inc]; ok {
			add(di, Error, "%s: %s is both disabled and scheduled.", PartDisabledIncident.Label(), incidentLabel(inc))
		}
	}
	if !foodInKit && d.Arrival != WalkIn {
		add(-1, Warning, "The kit has no food and the colonists do not walk in: they will start hungry.")
	}
	sortProblems(out)
	return out
}

func checkChance(add func(int, Severity, string, ...any), i int, label string, c uint8) {
	if c < MinChance || c > MaxChance {
		add(i, Error, "%s: the chance must be %d to %d percent.", label, MinChance, MaxChance)
	}
}

// sortProblems orders findings by part (mandatory fields first), stable.
func sortProblems(p []Problem) {
	sort.SliceStable(p, func(i, j int) bool { return p[i].Part < p[j].Part })
}

// HasErrors reports any error-severity problem.
func HasErrors(ps []Problem) bool {
	for _, p := range ps {
		if p.Severity == Error {
			return true
		}
	}
	return false
}

// essentialBuilding is a building the early game leans on; disallowing it
// is allowed but warned about.
func essentialBuilding(k construction.Kind) bool {
	switch k {
	case construction.Wall, construction.Bed, construction.DoubleBed, construction.Campfire:
		return true
	}
	return false
}

// playerItem is an item a scenario may give: anything but carcasses, bones,
// fertile eggs and the legacy "stone". The STO/CBT kinds (paste meals,
// belts, the new weapons) have art and use since STO-2 and CBT-3.
func playerItem(k item.Kind) bool {
	if k.String() == "unknown" || k.IsCorpse() {
		return false
	}
	switch k {
	case item.Stone, item.Bones, item.FertileEgg:
		return false
	}
	return true
}

// ItemChoices lists the items the editor's item dropdown offers.
func ItemChoices() []item.Kind {
	var out []item.Kind
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		if playerItem(k) {
			out = append(out, k)
		}
	}
	return out
}

// startAnimal is a species a scenario may start with: tameable domestic
// and pet roster only.
func startAnimal(s animal.Species) bool {
	if s.String() == "animal" {
		return false
	}
	b := animal.RosterOf(s).Behaviour
	return b == animal.Domestic || b == animal.Pet
}

// AnimalChoices lists the species the editor's animal dropdown offers.
func AnimalChoices() []animal.Species {
	var out []animal.Species
	for _, s := range animal.All() {
		if startAnimal(s) {
			out = append(out, s)
		}
	}
	return out
}

// HardnessPips is the display hardness: the stored value, or an estimate
// for custom scenarios (1 gentle ... 4 brutal).
func (d Def) HardnessPips() int {
	if d.Hardness > 0 {
		return int(d.Hardness)
	}
	h := 1
	if d.People <= 1 {
		h++
	}
	if d.Tech == faction.Neolithic {
		h++
	}
	for _, p := range d.Parts {
		if p.Kind == PartNoClothes {
			h++
			break
		}
	}
	return min(h, MaxHardness)
}
