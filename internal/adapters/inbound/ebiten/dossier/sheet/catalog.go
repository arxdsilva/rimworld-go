package sheet

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

// Entry describes one element the layout editor can add.
type Entry struct {
	Key   string
	Group string // layout, values, bars, pickers, generic
	Label string
	Tip   string
	// Container is a column, strip, block or line.
	Container bool
	// NeedsDef: the node must name a definition from Defs(Key).
	NeedsDef bool
	// Bar: the node takes a bar colour style.
	Bar bool
	// Reserved: the key exists but shows nothing until its system does.
	Reserved bool
}

// Def is one choice for a generic widget: a skill, a stat, a need...
type Def struct{ Key, Label string }

// Catalog lists every element in the order the editor's menus show it.
func Catalog(tr lang.Tr) []Entry {
	return []Entry{
		{Key: KindColumn, Group: "layout", Label: tr.T("Stack, top to bottom"), Container: true,
			Tip: tr.T("Stacks what it holds from top to bottom. It can hold anything.")},
		{Key: KindStrip, Group: "layout", Label: tr.T("Stack, side by side"), Container: true,
			Tip: tr.T("Sets what it holds side by side in equal widths. It can hold anything.")},
		{Key: KindBlock, Group: "layout", Label: tr.T("Panel"), Container: true,
			Tip: tr.T("A panel of rows. It holds rows only.")},
		{Key: KindLine, Group: "layout", Label: tr.T("Row"), Container: true,
			Tip: tr.T("A row of values spread evenly across its width. It holds values only.")},
		{Key: "gap", Group: "layout", Label: tr.T("Blank space"), Tip: tr.T("Empty space the height of one small gap.")},
		{Key: "rule", Group: "layout", Label: tr.T("Divider"), Tip: tr.T("A thin line across the card.")},
		{Key: "title", Group: "values", Label: tr.T("Name"), Tip: tr.T("The name in large type, tinted by how the colony sees them. Hover for the life story; click for Social.")},
		{Key: "standing", Group: "values", Label: tr.T("Standing"), Tip: tr.T("Who this is to the colony: prisoner, ally, drifter, hostile or wild, and what kind.")},
		{Key: "identity", Group: "values", Label: tr.T("Sex and age"), Tip: tr.T("Sex, kind of creature and age, with the days since the last birthday.")},
		{Key: "body_state", Group: "values", Label: tr.T("Body state"), Tip: tr.T("The worst thing wrong with the body, in words and colour. Click for Health.")},
		{Key: "mind_state", Group: "values", Label: tr.T("Mind state"), Tip: tr.T("A stun, a break, or how near one is; otherwise how the mood reads. Click for Needs.")},
		{Key: "doing", Group: "values", Label: tr.T("What it is doing"), Tip: tr.T("The current activity, and which work and skills it draws on.")},
		{Key: "next_up", Group: "values", Label: tr.T("Next order"), Tip: tr.T("The first queued order and how many wait behind it.")},
		{Key: "in_hand", Group: "values", Label: tr.T("Weapon or restraints"), Tip: tr.T("The weapon in hand, or that the person is held captive.")},
		{Key: "hauling", Group: "values", Label: tr.T("Carrying"), Tip: tr.T("What is being carried.")},
		{Key: "traits", Group: "values", Label: tr.T("Traits"), Tip: tr.T("Traits as a list; hover for what each does.")},
		{Key: "notes", Group: "values", Label: tr.T("Notes"), Tip: tr.T("Short status notes: pregnancy, hunting, exposure.")},
		{Key: "captive", Group: "values", Label: tr.T("Captive details"), Tip: tr.T("A prisoner's resistance and where they came from. Click for the Prisoner tab.")},
		{Key: "keeper", Group: "values", Label: tr.T("Master"), Tip: tr.T("The colonist an animal follows, tinted when they are bonded.")},
		{Key: "vigour_bar", Group: "bars", Label: tr.T("Health bar"), Bar: true, Tip: tr.T("How much of the body's hit points is left.")},
		{Key: "mood_bar", Group: "bars", Label: tr.T("Mood bar"), Bar: true, Tip: tr.T("Mood, with a tick at each level where a break can begin.")},
		{Key: "hunger_bar", Group: "bars", Label: tr.T("Food bar"), Bar: true, Tip: tr.T("How well fed.")},
		{Key: "rest_bar", Group: "bars", Label: tr.T("Rest bar"), Bar: true, Tip: tr.T("How rested.")},
		{Key: "fun_bar", Group: "bars", Label: tr.T("Recreation bar"), Bar: true, Tip: tr.T("How much free-time need is met.")},
		{Key: "need_bar", Group: "bars", Label: tr.T("Other need bar"), Bar: true, NeedsDef: true, Tip: tr.T("A bar for comfort, beauty or outdoors.")},
		{Key: "diet_pick", Group: "pickers", Label: tr.T("Food policy"), Tip: tr.T("A menu to set what this one is allowed to eat.")},
		{Key: "area_pick", Group: "pickers", Label: tr.T("Allowed area"), Tip: tr.T("A menu to restrict where this one may go.")},
		{Key: "hour_pick", Group: "pickers", Label: tr.T("This hour"), Tip: tr.T("What the schedule has for the current hour; a menu changes it.")},
		{Key: "clothes_pick", Group: "pickers", Label: tr.T("Clothing policy"), Reserved: true, Tip: tr.T("Reserved: it shows once the game has clothing policies.")},
		{Key: "skill", Group: "generic", Label: tr.T("Skill"), NeedsDef: true, Tip: tr.T("One skill: its level, passion and learning state.")},
		{Key: "train", Group: "generic", Label: tr.T("Training"), NeedsDef: true, Tip: tr.T("One trainable behaviour of a tame animal and how far it has got.")},
		{Key: "stat", Group: "generic", Label: tr.T("Stat"), NeedsDef: true, Tip: tr.T("One number from the stat sheet.")},
		{Key: "tally", Group: "generic", Label: tr.T("Record"), NeedsDef: true, Tip: tr.T("A running count kept for the pawn, such as kills.")},
	}
}

// Lookup finds a catalog entry by key.
func Lookup(key string) (Entry, bool) {
	for _, e := range Catalog(lang.English{}) {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

// Groups is the editor's menu order.
func Groups() []string { return []string{"layout", "values", "bars", "pickers", "generic"} }

// StatKey is a stat's stable key: its label in snake case.
func StatKey(id stat.ID) string {
	return strings.ReplaceAll(strings.ToLower(id.Label()), " ", "_")
}

// StatByKey finds a stat by key.
func StatByKey(key string) (stat.ID, bool) {
	for _, id := range cardStats() {
		if StatKey(id) == key {
			return id, true
		}
	}
	return 0, false
}

// cardStats are the stats worth a line on a card: the furniture stats are not.
func cardStats() []stat.ID {
	var out []stat.ID
	for id := stat.ID(0); id < stat.NumStats; id++ {
		switch id {
		case stat.Comfort, stat.RestEffectiveness, stat.Beauty:
			continue
		}
		out = append(out, id)
	}
	return out
}

// TrainLabel is a trainable behaviour's name.
func TrainLabel(s training.Skill) string {
	switch s {
	case training.Obedience:
		return "Obedience"
	case training.Guard:
		return "Guard"
	case training.Release:
		return "Release"
	}
	return ""
}

// TrainKey is a trainable behaviour's stable key.
func TrainKey(s training.Skill) string { return strings.ToLower(TrainLabel(s)) }

// TrainByKey finds a trainable behaviour by key.
func TrainByKey(key string) (training.Skill, bool) {
	for s := training.Skill(0); s < training.NumSkills; s++ {
		if TrainKey(s) == key {
			return s, true
		}
	}
	return 0, false
}

// Defs lists the definitions a generic widget can show.
func Defs(tr lang.Tr, key string) []Def {
	var out []Def
	switch key {
	case "skill":
		for id := skill.ID(0); id < skill.NumSkills; id++ {
			out = append(out, Def{id.Key(), id.Label()})
		}
	case "train":
		for s := training.Skill(0); s < training.NumSkills; s++ {
			out = append(out, Def{TrainKey(s), TrainLabel(s)})
		}
	case "stat":
		for _, id := range cardStats() {
			out = append(out, Def{StatKey(id), id.Label()})
		}
	case "tally":
		out = []Def{{"kills", tr.T("Kills")}, {"damage", tr.T("Damage dealt")}}
	case "need_bar":
		out = []Def{{"comfort", tr.T("Comfort")}, {"beauty", tr.T("Beauty")}, {"outdoors", tr.T("Outdoors")}}
	}
	return out
}

// LookupDef finds a definition of a generic widget.
func LookupDef(tr lang.Tr, key, def string) (Def, bool) {
	for _, d := range Defs(tr, key) {
		if d.Key == def {
			return d, true
		}
	}
	return Def{}, false
}
