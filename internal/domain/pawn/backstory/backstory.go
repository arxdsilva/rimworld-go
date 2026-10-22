// Package backstory is a colonist's childhood and adulthood: original
// titles and text, skill changes and work they can't do (NG-1 §3.7, §4.6).
package backstory

import (
	"strings"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
)

// ID is a backstory (append only, saved). 0 is "none".
type ID uint16

// Slot is childhood or adulthood.
type Slot uint8

const (
	Childhood Slot = iota
	Adulthood
)

// Bits is a set of things a colonist is incapable of.
type Bits uint16

const (
	Violence Bits = 1 << iota
	Caring
	DumbLabor
	Intellectual
	Animals
	Cooking
	Plants
	Mining
	Crafting
	Construction
	numBits = iota
)

// AllBits is every valid incapability bit.
const AllBits Bits = 1<<numBits - 1

// Tag fits a backstory to a scenario's tech level.
type Tag uint8

const (
	TagNeolithic Tag = 1 << iota
	TagIndustrial
	TagSpacer
)

// Def is one backstory. Desc may use {name} {they} {them} {their} (and
// capitalised {They} {Them} {Their}); see Fill.
type Def struct {
	ID        ID
	Slot      Slot
	Title     string
	Short     string
	Desc      string
	Skills    map[skill.ID]int8
	Incapable Bits
	Weight    int
	Tags      Tag
}

// bitInfo is each bit's label, disabled work and disabled skills.
var bitInfo = [numBits]struct {
	label  string
	work   []work.Type
	skills []skill.ID
}{
	{"Violence", []work.Type{work.Hunt}, []skill.ID{skill.Shooting, skill.Melee}},
	{"Caring", []work.Type{work.Doctor}, []skill.ID{skill.Medicine}},
	{"Menial work", []work.Type{work.Haul, work.Clean}, nil},
	{"Intellectual", []work.Type{work.Research}, []skill.ID{skill.Intellectual}},
	{"Animals", []work.Type{work.Handle}, []skill.ID{skill.Animals}},
	{"Cooking", []work.Type{work.Cook}, []skill.ID{skill.Cooking}},
	{"Plants", []work.Type{work.Grow}, []skill.ID{skill.Plants}},
	{"Mining", []work.Type{work.Mine}, []skill.ID{skill.Mining}},
	{"Crafting", []work.Type{work.Craft}, []skill.ID{skill.Crafting}},
	{"Construction", []work.Type{work.Construct}, []skill.ID{skill.Construction}},
}

// WorkMask is the work types b disables.
func (b Bits) WorkMask() work.Mask {
	var m work.Mask
	for i := range numBits {
		if b&(1<<i) != 0 {
			for _, t := range bitInfo[i].work {
				m = m.Set(t, true)
			}
		}
	}
	return m
}

// Skills lists the skills b disables (shown as "—"), in skill order.
func (b Bits) Skills() []skill.ID {
	var out []skill.ID
	for id := skill.ID(0); id < skill.NumSkills; id++ {
		if b.DisablesSkill(id) {
			out = append(out, id)
		}
	}
	return out
}

// DisablesSkill reports whether b makes skill id unusable.
func (b Bits) DisablesSkill(id skill.ID) bool {
	for i := range numBits {
		if b&(1<<i) == 0 {
			continue
		}
		for _, s := range bitInfo[i].skills {
			if s == id {
				return true
			}
		}
	}
	return false
}

// Labels names the bits ("Violence", "Cooking").
func (b Bits) Labels() []string {
	var out []string
	for i := range numBits {
		if b&(1<<i) != 0 {
			out = append(out, bitInfo[i].label)
		}
	}
	return out
}

// Fits reports whether d carries any of the tags in t.
func (d Def) Fits(t Tag) bool { return d.Tags&t != 0 }

// defs is indexed by ID; defs[0] is "none".
var defs = func() []Def {
	out := make([]Def, 1, len(catalog)+1)
	for i, d := range catalog {
		d.ID = ID(i + 1)
		out = append(out, d)
	}
	return out
}()

// All lists every backstory (a copy; the maps inside are shared, read only).
func All() []Def { return append([]Def(nil), defs[1:]...) }

// DefOf looks a backstory up; unknown IDs give the zero Def.
func DefOf(id ID) Def {
	if int(id) >= len(defs) {
		return Def{}
	}
	return defs[id]
}

// Pronouns for a sex: subject, object, possessive. Any value other than
// look.Male and look.Female reads as they/them/their.
func Pronouns(s look.Sex) (they, them, their string) {
	switch s {
	case look.Male:
		return "he", "him", "his"
	case look.Female:
		return "she", "her", "her"
	}
	return "they", "them", "their"
}

// Fill replaces {name}, {they}, {them}, {their} (and the capitalised
// {They}, {Them}, {Their}) in desc.
func Fill(desc string, s look.Sex, name string) string {
	they, them, their := Pronouns(s)
	return strings.NewReplacer(
		"{name}", name,
		"{they}", they, "{them}", them, "{their}", their,
		"{They}", capital(they), "{Them}", capital(them), "{Their}", capital(their),
	).Replace(desc)
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
