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
	// Social and host systems (ENG-003, append only, saved): looks,
	// temperament, orientation, voice, distrust, pace, appetite for
	// drugs and for implants. Generation offers them only outside Legacy
	// rules (Generated).
	Striking
	Fair
	Homely
	Grotesque
	Gracious
	ColdHearted
	Bloodthirsty
	Austere
	Asexual
	Gay
	Bisexual
	GratingVoice
	RaspingBreath
	DistrustsMen
	DistrustsWomen
	Idle
	Abstainer
	Indulgent
	Hedonist
	AugmentFan
	NaturalPurist
	NumTraits
)

// legacyTraits is how many traits (None included) generation knew before
// ENG-003: Legacy rules draw from these only, so old seeds keep their pawns.
const legacyTraits = Sturdy + 1

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
	// ENG-003.
	GroupBeauty
	GroupOrientation
	GroupChem
	GroupAugment
	GroupMisanthropy
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
	// Rarity (ENG-003): 0 is the normal generation weight; N > 0 makes the
	// trait N times rarer.
	Rarity uint8
	// Note is a plain summary of an effect no stat mod expresses ("Beauty
	// +2"), shown with Effects.
	Note string
	// Beauty is the trait's looks (-2..+2), read by social opinion.
	Beauty int8
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
		Desc: "{name} has slept in ditches and eaten worse. Hardship barely registers.",
		ThoughtScale: map[mood.Kind]int16{mood.SleptOutside: 25, mood.SleptOnGround: 25, mood.AteRaw: 25,
			mood.AtePaste: 0}}, // STO-2: the wiki's ascetic minds no paste
	Fussy: {Key: "fussy", Label: "Fussy", Group: GroupStandards,
		Desc: "{name} notices every draught, lump and stale crumb, and loves a well-kept room.",
		ThoughtScale: map[mood.Kind]int16{mood.SleptOutside: 200, mood.SleptOnGround: 200, mood.AteRaw: 200,
			mood.SleptInBedroom: 150}}, // BED-2: the bedroom ladder, both ways, replaces the nice-room bonus
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
	// ENG-003. Effects beyond stat mods are read by the social, drug and
	// body systems; Note says what they are.
	Striking: {Key: "striking", Label: "Striking", Group: GroupBeauty, Beauty: 2, Note: "Beauty +2",
		Desc: "Heads turn when {name} walks into a room, and {they} stopped noticing years ago."},
	Fair: {Key: "fair", Label: "Fair", Group: GroupBeauty, Beauty: 1, Note: "Beauty +1",
		Desc: "{name} has the kind of open, pleasant face that people warm to without knowing why."},
	Homely: {Key: "homely", Label: "Homely", Group: GroupBeauty, Beauty: -1, Note: "Beauty -1",
		Desc: "Nobody would call {name} handsome, and people sometimes forget to look at {them} at all."},
	Grotesque: {Key: "grotesque", Label: "Grotesque", Group: GroupBeauty, Beauty: -2, Note: "Beauty -2",
		Desc: "{name}'s face makes strangers flinch, which has taught {them} a great deal about strangers."},
	Gracious: {Key: "gracious", Label: "Gracious", Note: "Never insults or slights anyone",
		Desc: "{name} has a kind word for everyone and means most of them."},
	ColdHearted: {Key: "cold_hearted", Label: "Cold-hearted", Note: "Unmoved by death, grief and the suffering of others",
		Desc: "{name} weighs people like tools and feels nothing much when one breaks."},
	Bloodthirsty: {Key: "bloodthirsty", Label: "Bloodthirsty", Needs: backstory.Violence, Note: "Enjoys fighting and killing",
		Desc: "{name} comes alive in a fight, and the quiet days leave {them} restless."},
	Austere: {Key: "austere", Label: "Austere", Note: "Indifferent to fine rooms; content with little",
		Desc: "{name} wants a roof, a blanket and nothing more, and distrusts anyone who wants more."},
	Asexual: {Key: "asexual", Label: "Asexual", Group: GroupOrientation, Rarity: 20, Note: "Feels no romantic attraction",
		Desc: "{name} likes people well enough but has never felt the pull others talk about."},
	Gay: {Key: "gay", Label: "Gay", Group: GroupOrientation, Rarity: 10, Note: "Attracted to the same sex",
		Desc: "{name} is drawn to people of {their} own sex."},
	Bisexual: {Key: "bisexual", Label: "Bisexual", Group: GroupOrientation, Rarity: 10, Note: "Attracted to either sex",
		Desc: "{name} falls for a person, never for a man or a woman as such."},
	GratingVoice: {Key: "grating_voice", Label: "Grating voice", Note: "Others find the voice off-putting",
		Desc: "{name}'s voice sets teeth on edge, however pleasant the words."},
	RaspingBreath: {Key: "rasping_breath", Label: "Rasping breath", Note: "Others find the breathing a little off-putting",
		Desc: "Every breath {name} takes rattles like a loose shutter in the wind."},
	DistrustsMen: {Key: "distrusts_men", Label: "Distrusts men", Group: GroupMisanthropy, Note: "Thinks less of men",
		Desc: "Something long ago taught {name} to keep men at arm's length."},
	DistrustsWomen: {Key: "distrusts_women", Label: "Distrusts women", Group: GroupMisanthropy, Note: "Thinks less of women",
		Desc: "{name} expects the worst of women and is rarely talked out of it."},
	Idle: {Key: "idle", Label: "Idle", Group: GroupPace,
		Desc: "{name} treats every job as a suggestion and every chair as an invitation.",
		Mods: []stat.Mod{{Stat: stat.WorkSpeed, Mul: 600}}},
	Abstainer: {Key: "abstainer", Label: "Abstainer", Group: GroupChem, Note: "Refuses drugs; dislikes using them",
		Desc: "{name} has seen what a bottle does to people and wants no part of it."},
	Indulgent: {Key: "indulgent", Label: "Indulgent", Group: GroupChem, Note: "Craves drugs now and then",
		Desc: "{name} rarely says no to a drink, and sometimes goes looking for one."},
	Hedonist: {Key: "hedonist", Label: "Hedonist", Group: GroupChem, Note: "Craves drugs; needs a steady supply",
		Desc: "For {name}, a day without something to take the edge off is a day wasted."},
	AugmentFan: {Key: "augment_fan", Label: "Augment fan", Group: GroupAugment, Note: "Happier with artificial parts",
		Desc: "{name} sees the body as a first draft and dreams of upgrades."},
	NaturalPurist: {Key: "natural_purist", Label: "Natural purist", Group: GroupAugment, Note: "Unhappy with artificial parts",
		Desc: "{name} would rather limp than walk on something made in a workshop."},
}

// Generated lists the traits pawn generation may roll, in ID order: under
// legacy rules only the original set (so existing seeds keep their pawns),
// otherwise every trait.
func Generated(legacy bool) []ID {
	n := NumTraits
	if legacy {
		n = legacyTraits
	}
	out := make([]ID, 0, n-1)
	for t := ID(1); t < n; t++ {
		out = append(out, t)
	}
	return out
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
	if d.Note != "" {
		parts = append(parts, d.Note)
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
