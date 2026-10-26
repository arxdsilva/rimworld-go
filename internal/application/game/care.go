package game

import "rimworld-go/internal/domain/sim/entity"

// Medical care commands (MED-00 contract): routed from host_commands.go,
// they change nothing yet; each is implemented in MED-03.

// cmdSetMedicalCare sets patient's medical care policy to mode
// (implemented in MED-03).
func (g *Game) cmdSetMedicalCare(patient entity.EntityID, mode uint8) {}

// cmdSetDefaultCare sets the default care policy of group (0 colonists,
// 1 prisoners, 2 guests, 3 animals) to mode (implemented in MED-03).
func (g *Game) cmdSetDefaultCare(group int, mode uint8) {}

// cmdSetSelfTend lets colonist id tend itself or not (implemented in
// MED-03).
func (g *Game) cmdSetSelfTend(id entity.EntityID, on bool) {}

// cmdEmergencyTend sends drafted doctor to tend patient now (implemented
// in MED-03).
func (g *Game) cmdEmergencyTend(doctor, patient entity.EntityID) {}
