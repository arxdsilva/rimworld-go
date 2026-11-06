// Package gearview is the gear and outfit contract's read-model fields
// (GEAR_GAP_ANALYSIS.md, GEAR-1 and GEAR-3): one struct per game view,
// carried as its Gear field (game.PawnView.Gear and so on), so the owners
// fill and read their own fields without editing game.go. Every field is
// zero until its owner fills it; each names its owner. Pack stays
// comparable.
package gearview

import "rimworld-go/internal/domain/combat/armor"

// Pawn is game.PawnView.Gear.
type Pawn struct {
	// GEAR-001 (game/gearfacts): the weapons, worn apparel and pack, in
	// grams, against the carrying capacity, 75 kg × Manipulation × body
	// size (wiki: Carrying capacity). The pack's own limit stays
	// PawnView.PackLimitGrams.
	GearGrams, CapacityGrams int
	// GEAR-002 (game/gearfacts): each body part's armour after every layer
	// covering it, weighted by the part's coverage, capped at 200 %.
	OverallArmor armor.Set
	// GEAR-009 (game/gearfacts): what worn gear adds to move speed, in
	// percent of the base speed (negative slows).
	MovePct int
	// OUT-003 (game/outfit): the apparel policy the pawn dresses by.
	ApparelPolicy     uint32
	ApparelPolicyName string
	// GEAR-1 (game/sense): why nothing on this pawn can be dropped (a guest,
	// hostile, downed or dead); "" when drops are allowed.
	NoDrop string
}

// Use is what the Gear tab's Eat button does with a pack entry (GEAR-005).
type Use uint8

const (
	UseNone Use = iota // not consumable from the tab
	UseEat             // food: eaten through the ingest path
	UseDrug            // a drug: taken, subject to the drug policy
)

// Pack is game.PackItemView.Gear (game/sense, GEAR-1).
type Pack struct {
	Grams int // the stack's mass
	Use   Use
	Why   string // why Eat is refused for this entry ("" allowed)
}

// Policy is one apparel policy as the Assign window lists it (game/outfit,
// OUT-001). Allowed holds the allowed apparel kind names; the filter's
// other parts (materials, HP and quality ranges, tainted) are its own
// fields so the editor can show them.
type Policy struct {
	ID                     uint32
	Name                   string
	Allowed, Stuffs        []string
	HPMin, HPMax           int // percent, 0..100
	QualityMin, QualityMax uint8
	NoTainted              bool
	Default                bool // given to new colonists (SetDefaultApparelPolicy)
}

// World is game.WorldView.Gear (game/outfit).
type World struct {
	Policies []Policy
}
