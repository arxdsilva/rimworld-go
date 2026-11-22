// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// chronicle records what the raid package logs, by kind (DEF 3.1).
type chronicle struct{ fights, alarms []string }

func (c *chronicle) Fight(text string) { c.fights = append(c.fights, text) }
func (c *chronicle) Alarm(text string) { c.alarms = append(c.alarms, text) }

// chronicleHost is the little of Host a dying raider, a thief getting
// away and a landing pod use; any other host call panics.
type chronicleHost struct {
	Host
	Arrive
	raiders map[entity.EntityID]Raider
	vital   map[entity.EntityID]health.Record
	pos     map[entity.EntityID][2]int
	groups  []Group
	removed []entity.EntityID
	landed  int
}

func (h *chronicleHost) Raider(id entity.EntityID) Raider       { return h.raiders[id] }
func (h *chronicleHost) SetRaider(id entity.EntityID, r Raider) { h.raiders[id] = r }
func (h *chronicleHost) Prisoner(entity.EntityID) bool          { return false }
func (h *chronicleHost) Vital(id entity.EntityID) health.Record { return h.vital[id] }
func (h *chronicleHost) DropGear(entity.EntityID)               {}
func (h *chronicleHost) Name(entity.EntityID) string            { return "Rook" }
func (h *chronicleHost) Job(entity.EntityID) (job.Record, bool) { return job.Record{}, false }
func (h *chronicleHost) Size() (int, int)                       { return 10, 10 }
func (h *chronicleHost) RemoveRaider(id entity.EntityID)        { h.removed = append(h.removed, id) }
func (h *chronicleHost) Groups() []Group                        { return h.groups }
func (h *chronicleHost) Land(int, Pod) entity.EntityID          { h.landed++; return 0 }
func (h *chronicleHost) Pos(id entity.EntityID) (x, y int, ok bool) {
	p, ok := h.pos[id]
	return p[0], p[1], ok
}

// TestChronicleKinds: a raider killed is a fight entry; a thief getting
// away and hostile pods landing are threat entries.
func TestChronicleKinds(t *testing.T) {
	t.Parallel()
	h := &chronicleHost{
		raiders: map[entity.EntityID]Raider{
			1: {Arrived: 1},
			2: {Arrived: 1, Mode: Fleeing, Loot: item.Silver, LootQty: 30},
		},
		vital:  map[entity.EntityID]health.Record{1: {Status: health.Dead}, 2: {Status: health.Alive}},
		pos:    map[entity.EntityID][2]int{1: {5, 5}, 2: {0, 4}},
		groups: []Group{{Pods: []Pod{{X: 3, Y: 3, Land: 50}, {X: 4, Y: 3, Land: 50}}}},
	}
	ch := &chronicle{}
	r := New(h, Roles{Arrive: h, Chronicle: ch})
	r.Raiders.Think(1, 100) // dead
	r.Raiders.Think(1, 101) // logged once
	r.Raiders.Think(2, 100) // on the edge with loot: gone home
	r.Pods.Step(49)
	r.Pods.Step(50)
	r.Pods.Step(51)
	if len(ch.fights) != 1 || ch.fights[0] != "Raider Rook was killed." {
		t.Fatalf("fights %q", ch.fights)
	}
	want := []string{"Rook got away with 30 " + item.Silver.String() + ".", "Hostile drop pods are landing."}
	if len(ch.alarms) != len(want) || ch.alarms[0] != want[0] || ch.alarms[1] != want[1] {
		t.Fatalf("alarms %q, want %q", ch.alarms, want)
	}
	if h.landed != 2 || len(h.removed) != 1 || h.removed[0] != 2 {
		t.Fatalf("landed %d, removed %v", h.landed, h.removed)
	}
}
