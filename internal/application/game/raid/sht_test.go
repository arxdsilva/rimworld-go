// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// aimHost is the little of Host a raider starting an attack uses: one
// raider (1) and one colonist (2) beside it; AimTicks answers the
// fighter's aim at half the warmup (a quick draw), as the game's
// combat.AimTicks does for a gun. Any other host call panics.
type aimHost struct {
	Host
	r     Raider
	group Group
	job   job.Record
	set   []job.Record
	aimed int
}

func (h *aimHost) Raider(entity.EntityID) Raider              { return h.r }
func (h *aimHost) SetRaider(_ entity.EntityID, r Raider)      { h.r = r }
func (h *aimHost) RaiderIDs() []entity.EntityID               { return []entity.EntityID{1} }
func (h *aimHost) StandingRaider(entity.EntityID) bool        { return true }
func (h *aimHost) Prisoner(entity.EntityID) bool              { return false }
func (h *aimHost) Vital(entity.EntityID) health.Record        { return health.Record{Status: health.Alive} }
func (h *aimHost) Job(entity.EntityID) (job.Record, bool)     { return h.job, h.job.Kind != 0 }
func (h *aimHost) SetJob(_ entity.EntityID, j job.Record)     { h.set = append(h.set, j); h.job = j }
func (h *aimHost) ReleaseOwner(entity.EntityID)               {}
func (h *aimHost) Groups() []Group                            { return []Group{h.group} }
func (h *aimHost) Pos(entity.EntityID) (int, int, bool)       { return 5, 5, true }
func (h *aimHost) Size() (int, int)                           { return 40, 40 }
func (h *aimHost) Weapon(entity.EntityID) weapon.Def          { return weapon.DefOf(weapon.Rifle) }
func (h *aimHost) CanHit(int, int, weapon.Def, int, int) bool { return true }
func (h *aimHost) NearestColonist(int, int) (entity.EntityID, int, int, bool) {
	return 2, 6, 5, true
}
func (h *aimHost) RaidTarget(int, int) (entity.EntityID, int, int, bool) { return 2, 6, 5, true }
func (h *aimHost) TurretFor(int, int, int, int) (entity.EntityID, int, int, bool) {
	return 0, 0, 0, false
}
func (h *aimHost) AimTicks(_ entity.EntityID, d weapon.Def) int {
	h.aimed++
	return d.Warmup / 2
}

// TestFirstAimScales (SHT-102): every raid site that starts an attack
// (the assault, the siege camp, a prisoner breaking out) takes its first
// aim from Fight.AimTicks, which the game scales by the aiming time.
func TestFirstAimScales(t *testing.T) {
	t.Parallel()
	half := weapon.DefOf(weapon.Rifle).Warmup / 2
	for _, tc := range []struct {
		name  string
		r     Raider
		group Group
	}{
		{"assault", Raider{Mode: Assaulting, Arrived: 1}, Group{Plan: Assault}},
		{"siege camp", Raider{Mode: Assaulting, Arrived: 1}, Group{Plan: Siege, Camped: true, Until: 1 << 40}},
		{"escape", Raider{Mode: Fleeing, Escaping: true, Arrived: 1}, Group{Plan: Assault}},
	} {
		h := &aimHost{r: tc.r, group: tc.group}
		New(h, Roles{}).Raiders.Think(1, 1000)
		if h.aimed != 1 || len(h.set) != 1 || h.set[0].Kind != job.Attack || h.set[0].WorkLeft != half {
			t.Fatalf("%s: aimed %d, jobs %+v; want one attack aiming %d", tc.name, h.aimed, h.set, half)
		}
	}
}
