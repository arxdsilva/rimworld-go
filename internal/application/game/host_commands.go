package game

import (
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// applyHostCommand routes the ENG-003 commands to their owners' handlers
// (lead-owned: each handler lives in its milestone's file).
func (g *Game) applyHostCommand(c command.Command) {
	pawn, target := entity.EntityID(c.Pawn), entity.EntityID(c.Target)
	switch c.Kind {
	case command.AddOperation:
		g.cmdAddOperation(pawn, c.Index, c.Build, c.Stuff) // surgery.go
	case command.RemoveOperation:
		g.cmdRemoveOperation(pawn, c.Index) // surgery.go
	case command.SetDrugPolicy:
		g.cmdSetDrugPolicy(pawn, c.Mode) // drugs.go
	case command.AssignBedOwner:
		g.rest.Beds.AssignOwner(target, pawn, c.Index) // rest/beds.go
	case command.SetAnimalMaster:
		g.cmdSetAnimalMaster(target, pawn) // bonds.go
	case command.SetAnimalFollow:
		g.cmdSetAnimalFollow(target, c.Enabled) // train_animals.go
	case command.SetTraining:
		g.cmdSetTraining(target, c.Index, c.Enabled) // bonds.go
	case command.GiftAnimal:
		g.cmdGiftAnimal(target, pawn) // bonds.go
	case command.Capture:
		g.care.StartCapture(pawn, target, false) // care/capture.go
	case command.Arrest:
		g.care.StartArrest(pawn, target, false) // care/capture.go
	case command.SetPrisonerMode:
		g.prison.SetMode(target, c.Mode) // prison/prison.go
	case command.SetBedForPrisoners:
		g.cmdSetBedForPrisoners(target, c.Enabled) // prisoners.go
	case command.ReleasePrisoner:
		g.prison.Release(target, 0) // prison/prison.go: walks off the map
	case command.FormCaravan:
		g.cmdFormCaravan(c.Name, c.Count) // world_state.go
	case command.CaravanReturn:
		g.cmdCaravanReturn(target) // world_state.go
	case command.PaintArea:
		g.cmdPaintArea(c.Index, c.X, c.Y, c.X2, c.Y2, c.Enabled) // areas.go
	case command.RenameArea:
		g.cmdRenameArea(c.Index, c.Name) // areas.go
	case command.AssignArea:
		g.cmdAssignArea(pawn, c.Index) // areas.go
	case command.CreateArea:
		g.cmdCreateArea(c.Name) // areas.go
	case command.DeleteArea:
		g.cmdDeleteArea(c.Index) // areas.go
	case command.InvertArea:
		g.cmdInvertArea(c.Index) // areas.go
	case command.SetBedMedical:
		g.care.Rest.SetBedMedical(target, c.Enabled) // care/rest.go (PLAY-020)
	case command.SetPawnBedRest:
		g.care.Rest.SetPawnBedRest(target, c.Enabled) // care/rest.go
	case command.SetZoneSow:
		g.cmdSetZoneSow(uint16(c.Target), c.Enabled) // zone_cmds.go (PLAY-020)
	case command.SetZoneCut:
		g.cmdSetZoneCut(uint16(c.Target), c.Enabled) // zone_cmds.go
	case command.SetZoneHidden:
		g.cmdSetZoneHidden(uint16(c.Target), c.Enabled) // zone_cmds.go
	case command.RenameZone:
		g.cmdRenameZone(uint16(c.Target), c.Name) // zone_cmds.go
	case command.SetZoneCrop:
		g.cmdSetZoneCrop(uint16(c.Target), c.Build) // zone_cmds.go
	case command.ExpandZone:
		g.cmdExpandZone(uint16(c.Target), c.X, c.Y, c.X2, c.Y2) // zone_cmds.go
	case command.RemoveZoneCells:
		g.cmdRemoveZoneCells(uint16(c.Target), c.X, c.Y, c.X2, c.Y2) // zone_cmds.go
	case command.DeleteZone:
		g.cmdDeleteZone(uint16(c.Target)) // zone_cmds.go
	case command.SetZonePriority:
		g.cmdSetZonePriority(uint16(c.Target), c.Index) // haul_priority.go (PLAY-023)
	case command.SetPowerWanted:
		g.cmdSetPowerWanted(target, c.Enabled) // power_flick.go (PLAY-021)
	case command.SetThermostat:
		g.cmdSetThermostat(target, c.Count) // power_heat.go (PLAY-021)
	case command.SetMedicalCare:
		g.care.SetMedicalCare(pawn, c.Mode) // care package (MED-03)
	case command.SetDefaultCare:
		g.care.Policy.SetDefault(c.Index, c.Mode) // care package
	case command.SetSelfTend:
		g.care.SetSelfTend(pawn, c.Enabled) // care package
	case command.EmergencyTend:
		g.care.Doctor.EmergencyTend(pawn, target) // care package
	case command.ReorderOperation:
		g.cmdReorderOperation(pawn, c.Index, c.Count) // surgery.go
	case command.ProposeTrade:
		g.cmdProposeTrade(c.Name, c.Enabled) // trade_session.go (PLAY-024)
	case command.CloseTrade:
		g.cmdCloseTrade() // trade_session.go
	case command.CommsCallTrader:
		g.cmdCommsCallTrader() // comms.go
	case command.CommsGift:
		g.cmdCommsGift(c.Index, c.Count) // comms.go
	case command.CommsRequestCaravan:
		g.cmdCommsRequestCaravan(c.Index) // comms.go
	case command.CommsRequestAid:
		g.cmdCommsRequestAid(c.Index) // comms.go
	case command.CloseComms:
		g.cmdCloseComms() // comms.go
	}
}
