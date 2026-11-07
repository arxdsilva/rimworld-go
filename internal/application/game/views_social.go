package game

import (
	"rimworld-go/internal/application/game/dose"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/sim/entity"
)

// Read models of the social and host systems (ENG-003 C3), in game/tabview.
// The UI reads them through the methods below; they are copies, never live
// state. Until a milestone fills one it reports nothing (ok false, or empty).
type (
	SocialView          = tabview.Social
	RelationRowView     = tabview.RelationRow
	OpinionTermView     = tabview.OpinionTerm
	SocialLogLineView   = tabview.SocialLogLine
	HealthView          = tabview.Health
	OperationOptionView = tabview.OperationOption
	CapacityView        = tabview.Capacity
	PartView            = tabview.Part
	OperationView       = tabview.Operation
	PrisonerView        = tabview.Prisoner
	WorldMapView        = tabview.WorldMap
	AreaView            = tabview.Area
	HistoryEntryView    = tabview.HistoryEntry
)

// ConditionView is one row of the Health tab (game/roster).
type ConditionView = roster.ConditionView

// DrugLineView is one drug effect, tolerance or addiction line.
type DrugLineView = dose.Line

// CaravanView is one caravan (game/travel).
type CaravanView = travel.CaravanView

// Social is colonist id's Social tab (ok false: nothing to show yet).
func (g *Game) Social(id entity.EntityID) (SocialView, bool) {
	return g.socialView(id)
}

// Health is colonist id's Health tab (ok false: nothing to show yet).
func (g *Game) Health(id entity.EntityID) (HealthView, bool) {
	hv, ok := g.healthView(id)
	if ok { // care/policy.go (MED-03): the care level and the self-tend toggle
		hv.Care, hv.SelfTend = uint8(g.care.Level(id)), !g.care.Policy.SelfTendOff(id)
	}
	return hv, ok
}

// Prisoner is prisoner id's tab (ok false: not a prisoner).
func (g *Game) Prisoner(id entity.EntityID) (PrisonerView, bool) {
	return g.prisonerView(id)
}

// WorldMap is the World tab.
func (g *Game) WorldMap() WorldMapView {
	return g.worldMapView()
}

// Areas lists the colony's allowed areas.
func (g *Game) Areas() []AreaView {
	return g.areaViews()
}

// History lists the colony's history, oldest first.
func (g *Game) History() []HistoryEntryView {
	return g.historyViews()
}
