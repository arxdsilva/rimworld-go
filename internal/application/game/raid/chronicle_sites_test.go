// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// siteHost adds what the threat sites (theft, arson, grab, resupply,
// retreat, siege, storm) call to chronicleHost; any other call panics.
type siteHost struct {
	*chronicleHost
	ids    []entity.EntityID
	jobs   map[entity.EntityID]job.Record
	threat []string
}

func (h *siteHost) Interrupt(entity.EntityID, job.FailReason)     {}
func (h *siteHost) Message(int, int, string, ...any)              {}
func (h *siteHost) Threat(t string, _, _ int, _ string, _ ...any) { h.threat = append(h.threat, t) }
func (h *siteHost) Roll(int) int                                  { return 0 }
func (h *siteHost) Home() (int, int)                              { return 5, 5 }
func (h *siteHost) InBounds(int, int) bool                        { return true }
func (h *siteHost) RaiderIDs() []entity.EntityID                  { return h.ids }
func (h *siteHost) StandingRaider(entity.EntityID) bool           { return false }
func (h *siteHost) WalkableNear(x, y, _ int) (int, int, bool)     { return x, y, true }
func (h *siteHost) Crash(x, y int) (int, int, bool)               { return x, y, true }
func (h *siteHost) Job(id entity.EntityID) (job.Record, bool)     { return h.jobs[id], true }

// siteMischief has a stack underfoot to steal and a wooden wall beside.
type siteMischief struct{ Mischief }

func (siteMischief) Loot(x, y, _ int) (entity.EntityID, int, int, bool) { return 90, x, y, true }
func (siteMischief) Steal(entity.EntityID, entity.EntityID) (item.Kind, int, bool) {
	return item.Silver, 5, true
}
func (siteMischief) Flammable(x, y, _ int) (int, int, bool) { return x + 1, y, true }
func (siteMischief) Ignite(int, int) bool                   { return true }

// siteTactics has no crops; it takes supply drops and mortar refills.
type siteTactics struct {
	Tactics
	drops, restocks int
}

func (t *siteTactics) Crop(int, int) bool            { return false }
func (t *siteTactics) Drop(item.Kind, int, int, int) { t.drops++ }
func (t *siteTactics) Restock(entity.EntityID)       { t.restocks++ }

func siteRig() (*Raiders, *siteHost, *chronicle, *siteTactics) {
	h := &siteHost{chronicleHost: &chronicleHost{
		raiders: map[entity.EntityID]Raider{},
		groups:  []Group{{Plan: Siege, Size: 4, CampX: 5, CampY: 9, Camped: true, Supply: 1, Mortars: []entity.EntityID{70}}},
	}, jobs: map[entity.EntityID]job.Record{}}
	ch, tk := &chronicle{}, &siteTactics{}
	r := New(h, Roles{Arrive: h, Chronicle: ch, Mischief: siteMischief{}, Tactics: tk})
	return r.Raiders, h, ch, tk
}

// TestChronicleThreatSites: every threat site in the raid package writes
// an Alarm (a History Threats entry) and never a Fight (DEF 3.1).
func TestChronicleThreatSites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		run        func(rs *Raiders, h *siteHost)
	}{
		{"theft", "Rook stole 5 " + item.Silver.String() + " and is running off with it.", func(rs *Raiders, _ *siteHost) {
			rs.steal(1, Raider{}, job.Record{}, 3, 3, 10)
		}},
		{"arson", "Rook is setting the colony on fire!", func(rs *Raiders, _ *siteHost) {
			rs.burn(1, Raider{}, job.Record{}, 3, 3, 10)
		}},
		{"grab", "Rook grabbed 5 " + item.Silver.String() + " on the way out.", func(rs *Raiders, _ *siteHost) {
			rs.grab(1, Raider{}, job.Record{}, 3, 3, 10)
		}},
		{"resupply", "A drop pod brought the besiegers supplies.", func(rs *Raiders, h *siteHost) {
			rs.resupply(&h.groups[0], 10)
		}},
		{"retreat", "The raiders have lost heart and are retreating.", func(rs *Raiders, _ *siteHost) {
			rs.broken(0, 10)
			rs.broken(0, 11) // logged once
		}},
		{"siege", "The raiders have dug in for a siege and are setting up mortars. They will storm the colony within a few days.", func(rs *Raiders, _ *siteHost) {
			rs.digIn(0, 10)
		}},
		{"storm", "They storm.", func(rs *Raiders, h *siteHost) {
			h.ids, h.jobs[2] = []entity.EntityID{2}, job.Record{Kind: job.ManMortar}
			rs.storm(0, 5, 9, 10, "They storm.")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs, h, ch, _ := siteRig()
			tc.run(rs, h)
			if len(ch.fights) != 0 || len(ch.alarms) != 1 || ch.alarms[0] != tc.want {
				t.Fatalf("fights %q, alarms %q, want one alarm %q", ch.fights, ch.alarms, tc.want)
			}
		})
	}
}

// TestChronicleResupplyAndStormSideEffects: the logged resupply drops the
// meals and refills the mortars; the logged storm posts a siege threat.
func TestChronicleResupplyAndStormSideEffects(t *testing.T) {
	t.Parallel()
	rs, h, _, tk := siteRig()
	rs.resupply(&h.groups[0], 10)
	rs.storm(0, 5, 9, 10, "x")
	if tk.drops != 1 || tk.restocks != 1 || len(h.threat) != 1 || h.threat[0] != "Siege assault" || !h.groups[0].Stormed {
		t.Fatalf("drops %d restocks %d threats %q group %+v", tk.drops, tk.restocks, h.threat, h.groups[0])
	}
}
