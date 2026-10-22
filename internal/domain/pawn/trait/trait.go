// Package trait is a colonist's personality traits: original set, conflict
// groups, stat mods and thought scaling (NG-1 §3.6, §4.6).
package trait

import (
	"sort"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// ID is a trait (append only, saved). 0 is "none".
type ID uint8

const (
	None ID = iota
	Tireless
	Unhurried
	Sunny
	Brooding
	Steady
	Brittle
	FleetFooted
	Plodding
	HeartyEater
	LightEater
	Roughneck
	Fussy
	DeliberateAim
	QuickDraw
	GreenThumb
	Bookish
	Gentle
	ColdHardy
	Sturdy
	NumTraits
)

// Conflict groups: traits sharing a non-zero group exclude each other.
const (
	NoGroup uint8 = iota
	GroupPace
	GroupOutlook
	GroupNerve
	GroupGait
	GroupAppetite
	GroupStandards
	GroupTrigger
)

// Def is one trait's definition.
type Def struct {
	ID           ID
	Key, Label   string
	Desc         string
	Group        uint8 // traits sharing a non-zero group conflict
	Mods         []stat.Mod
	ThoughtScale map[mood.Kind]int16 // percent; 100 = unchanged
	AddThought   map[mood.Kind]int8
	WorkSpeed    map[skill.ID]int16 // percent bonus to work using that skill
	LearnMul     map[skill.ID]int16 // percent learning multiplier
	SkillBonus   map[skill.ID]int8  // levels added at generation
	Incapable    backstory.Bits
	// Needs is what the trait is pointless without: it never pairs with a
	// backstory incapable of any of these (Green thumb needs Plants).
	Needs backstory.Bits
}

var defs = [NumTraits]Def{
	None: {},
	Tireless: {Key: "tireless", Label: "Tireless", Group: GroupPace,
		Desc: "{name} finds a rhythm and keeps it long after everyone else has stopped for breath.",
		Mods: []stat.Mod{{Stat: stat.WorkSpeed, Mul: 1200}}},
	Unhurried: {Key: "unhurried", Label: "Unhurried", Group: GroupPace,
		Desc: "{name} refuses to rush anything, which is slow, but does leave {them} in no hurry to be entertained either.",
		Mods: []stat.Mod{{Stat: stat.WorkSpeed, Mul: 800}, {Stat: stat.RecreationRate, Mul: 850}}},
	Sunny: {Key: "sunny", Label: "Sunny", Group: GroupOutlook,
		Desc: "{name} wakes up expecting a good day and, oddly often, gets one.",
		Mods: []stat.Mod{{Stat: stat.MoodOffset, Add: 8}}},
	Brooding: {Key: "brooding", Label: "Brooding", Group: GroupOutlook,
		Desc: "{name} carries a private storm cloud and checks on it regularly.",
		Mods: []stat.Mod{{Stat: stat.MoodOffset, Add: -8}}},
	Steady: {Key: "steady", Label: "Steady", Group: GroupNerve,
		Desc: "It takes a great deal to shake {name}, who bends where others snap.",
		Mods: []stat.Mod{{Stat: stat.BreakThreshold, Add: -6}}},
	Brittle: {Key: "brittle", Label: "Brittle", Group: GroupNerve,
		Desc: "{name} holds together well enough, right up until the moment it all gives way.",
		Mods: []stat.Mod{{Stat: stat.BreakThreshold, Add: 6}}},
	FleetFooted: {Key: "fleet_footed", Label: "Fleet-footed", Group: GroupGait,
		Desc: "{name} goes everywhere at a lope and arrives before anyone expected.",
		Mods: []stat.Mod{{Stat: stat.MoveSpeed, Mul: 1150}}},
	Plodding: {Key: "plodding", Label: "Plodding", Group: GroupGait,
		Desc: "{name} walks as if every step needed planning permission.",
		Mods: []stat.Mod{{Stat: stat.MoveSpeed, Mul: 880}}},
	HeartyEater: {Key: "hearty_eater", Label: "Hearty eater", Group: GroupAppetite,
		Desc:       "{name} eats like two people and enjoys a proper meal more than most.",
		Mods:       []stat.Mod{{Stat: stat.HungerRate, Mul: 1250}},
		AddThought: map[mood.Kind]int8{mood.AteMeal: 2}},
	LightEater: {Key: "light_eater", Label: "Light eater", Group: GroupAppetite,
		Desc: "{name} gets by on a few mouthfuls and forgets meals entirely when busy.",
		Mods: []stat.Mod{{Stat: stat.HungerRate, Mul: 800}}},
	Roughneck: {Key: "roughneck", Label: "Roughneck", Group: GroupStandards,
		Desc:         "{name} has slept in ditches and eaten worse. Hardship barely registers.",
		ThoughtScale: map[mood.Kind]int16{mood.SleptOutside: 25, mood.SleptOnGround: 25, mood.AteRaw: 25}},
	Fussy: {Key: "fussy", Label: "Fussy", Group: GroupStandards,
		Desc:         "{name} notices every draught, lump and stale crumb, and loves a well-kept room.",
		ThoughtScale: map[mood.Kind]int16{mood.SleptOutside: 200, mood.SleptOnGround: 200, mood.AteRaw: 200},
		AddThought:   map[mood.Kind]int8{mood.SleptInNiceRoom: 3}},
	DeliberateAim: {Key: "deliberate_aim", Label: "Deliberate aim", Group: GroupTrigger, Needs: backstory.Violence,
		Desc: "{name} lines up every shot as if it were the last one {they} had.",
		Mods: []stat.Mod{{Stat: stat.ShootAcc, Add: 12}, {Stat: stat.AimCooldown, Mul: 1200}}},
	QuickDraw: {Key: "quick_draw", Label: "Quick draw", Group: GroupTrigger, Needs: backstory.Violence,
		Desc: "{name} shoots first and aims somewhere around the second shot.",
		Mods: []stat.Mod{{Stat: stat.AimCooldown, Mul: 750}, {Stat: stat.ShootAcc, Add: -10}}},
	GreenThumb: {Key: "green_thumb", Label: "Green thumb", Needs: backstory.Plants,
		Desc:      "Seedlings seem to grow faster just because {name} is watching.",
		WorkSpeed: map[skill.ID]int16{skill.Plants: 25}},
	Bookish: {Key: "bookish", Label: "Bookish", Needs: backstory.Intellectual,
		Desc:      "{name} would rather read about a thing than do it, and learns accordingly.",
		WorkSpeed: map[skill.ID]int16{skill.Intellectual: 25},
		LearnMul:  map[skill.ID]int16{skill.Intellectual: 150}},
	Gentle: {Key: "gentle", Label: "Gentle", Incapable: backstory.Violence,
		Desc:       "{name} will not raise a hand against anyone, and people open up to {them} because of it.",
		SkillBonus: map[skill.ID]int8{skill.Social: 3}},
	ColdHardy: {Key: "cold_hardy", Label: "Cold-hardy",
		Desc: "{name} shrugs off a frost that sends everyone else for blankets.",
		Mods: []stat.Mod{{Stat: stat.ColdComfort, Add: -60}}},
	Sturdy: {Key: "sturdy", Label: "Sturdy",
		Desc: "{name} keeps going through injuries that would floor most people.",
		Mods: []stat.Mod{{Stat: stat.PainTolerance, Add: 15}}},
}

func init() {
	for i := range defs {
		defs[i].ID = ID(i)
	}
}

// DefOf looks a trait up; unknown IDs give the zero Def.
func DefOf(id ID) Def {
	if id >= NumTraits {
		return Def{}
	}
	return defs[id]
}

// All lists every trait except None, in ID order.
func All() []Def { return append([]Def(nil), defs[1:]...) }

// Parse finds a trait by key.
func Parse(key string) (ID, bool) {
	for i := ID(1); i < NumTraits; i++ {
		if defs[i].Key == key {
			return i, true
		}
	}
	return None, false
}

// Conflicts reports whether a and b can't be held together: the same
// trait twice, two traits of one group, or a pacifist with a trigger
// style.
func Conflicts(a, b ID) bool {
	if a == None || b == None || a >= NumTraits || b >= NumTraits {
		return false
	}
	if a == b {
		return true
	}
	da, db := defs[a], defs[b]
	if da.Group != NoGroup && da.Group == db.Group {
		return true
	}
	return da.Incapable&db.Needs != 0 || db.Incapable&da.Needs != 0
}

// Clashes reports whether trait t can't go with backstory b: t needs
// something b can't do (Green thumb with a plants-incapable past), or t
// takes away something b is built on (Gentle with a backstory that taught
// shooting or melee).
func Clashes(t ID, b backstory.Def) bool {
	if t == None || t >= NumTraits {
		return false
	}
	d := defs[t]
	if d.Needs&b.Incapable != 0 {
		return true
	}
	for id, v := range b.Skills {
		if v > 0 && d.Incapable.DisablesSkill(id) {
			return true
		}
	}
	return false
}

// Effects is a one-line summary of what the trait does:
// "Work speed +20%", "Ate raw food thoughts x0.25", "Incapable: Violence".
func (d Def) Effects() string {
	var parts []string
	for _, m := range d.Mods {
		parts = append(parts, stat.Format(m))
	}
	for _, k := range sortedKinds(d.ThoughtScale) {
		parts = append(parts, "\""+mood.DefOf(k).Label+"\" x"+pct(int(d.ThoughtScale[k])))
	}
	for _, k := range sortedKinds(d.AddThought) {
		parts = append(parts, "\""+mood.DefOf(k).Label+"\" "+sign(int(d.AddThought[k])))
	}
	for _, s := range sortedSkills(d.WorkSpeed) {
		parts = append(parts, s.Label()+" speed "+sign(int(d.WorkSpeed[s]))+"%")
	}
	for _, s := range sortedSkills(d.LearnMul) {
		parts = append(parts, s.Label()+" learning x"+pct(int(d.LearnMul[s])))
	}
	for _, s := range sortedSkills(d.SkillBonus) {
		parts = append(parts, s.Label()+" "+sign(int(d.SkillBonus[s]))+" levels")
	}
	if l := d.Incapable.Labels(); len(l) > 0 {
		parts = append(parts, "Incapable: "+strings.Join(l, ", "))
	}
	return strings.Join(parts, ", ")
}

func sign(v int) string {
	if v < 0 {
		return strconv.Itoa(v)
	}
	return "+" + strconv.Itoa(v)
}

// pct reads a percent as a factor: 25 -> "0.25", 150 -> "1.5", 200 -> "2".
func pct(v int) string {
	s := strconv.Itoa(v / 100)
	if r := v % 100; r != 0 {
		f := strconv.Itoa(100 + r)[1:] // two digits
		s += "." + strings.TrimRight(f, "0")
	}
	return s
}

func sortedKinds[V any](m map[mood.Kind]V) []mood.Kind {
	out := make([]mood.Kind, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedSkills[V any](m map[skill.ID]V) []skill.ID {
	out := make([]skill.ID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
