// Package backstory is a colonist's childhood and adulthood: original
// titles and text, skill changes and work they can't do (NG-1 §3.7, §4.6).
package backstory

import (
	"rimworld-go/internal/domain/ai/work"
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
)

// Tag fits a backstory to a scenario's tech level.
type Tag uint8

const (
	TagNeolithic Tag = 1 << iota
	TagIndustrial
	TagSpacer
)

// Def is one backstory. Desc may use {name} {they} {them} {their}.
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

// WorkMask is the work types b disables.
func (b Bits) WorkMask() work.Mask { return 0 } // NG-1 stub (PAWN-D)

// Skills lists the skills b disables (shown as "—").
func (b Bits) Skills() []skill.ID { return nil } // NG-1 stub (PAWN-D)

// Labels names the bits ("Violence", "Cooking").
func (b Bits) Labels() []string { return nil } // NG-1 stub (PAWN-D)

// All lists every backstory; DefOf looks one up.
func All() []Def      { return nil }   // NG-1 stub (PAWN-D)
func DefOf(id ID) Def { return Def{} } // NG-1 stub (PAWN-D)
