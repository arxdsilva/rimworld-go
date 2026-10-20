package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Burial and cremation (SIM-017): bodies become corpse items, haulers
// carry them to graves, pyres burn them. Contract stubs; the burial work
// fills them in.

// graveRec is who lies in a filled grave.
type graveRec struct {
	Occupant  entity.EntityID
	Name      string
	Cause     string
	DiedDay   int
	BuriedDay int
	Headstone int
}

// spawnCorpse turns a pawn that just died into a human corpse item.
func (g *Game) spawnCorpse(id entity.EntityID) {}

// assignBury gives hauler id a body to carry to an empty grave.
func (g *Game) assignBury(id entity.EntityID) bool { return false }

// doBury runs a burial job: pick the body up, carry it, fill the grave.
func (g *Game) doBury(id entity.EntityID, j job.Record) {}

// afterLoadBurial gives dead colonists from older saves their corpses.
func (g *Game) afterLoadBurial() {}

// fillPawnDeath sets PawnView.Embodied.
func (g *Game) fillPawnDeath(pv *PawnView) {}

// fillCorpseView sets ItemView.Of and OfName for a human corpse or bones.
func (g *Game) fillCorpseView(iv *ItemView, st item.Stack) {}

// fillGraveView sets a grave's occupant, epitaph and headstone, and a
// pyre's Burning.
func (g *Game) fillGraveView(bv *BuildingView, id entity.EntityID, b construction.Building) {}
