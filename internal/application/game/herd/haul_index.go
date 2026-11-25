// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/region"
	"rimworld-go/pkg/mathx"
)

// haulFinalists bounds the expensive destination searches and actual
// path attempts per admission. Source order breaks equal-distance ties.
const haulFinalists = 16

type HaulOffer struct {
	Source            entity.EntityID
	SX, SY, X, Y, Qty int
}

func (o HaulOffer) distance(x, y int) int { return mathx.Abs(o.SX-x) + mathx.Abs(o.SY-y) }

// HaulRevision lists the authoritative owners that invalidate admission.
// Sources only depend on the first three revisions. Claims, topology,
// structures, areas and fire additionally invalidate destination offers.
type HaulRevision struct {
	Stacks, Zones, Fog                                          uint64
	Grid, Structures, Fences, Frames, Claims, Areas, Home, Fire uint64
}

type HaulScope struct {
	Regions *region.Map
	Layer   region.Layer
	Area    int
	Flee    bool
	Safe    func(int, int) bool
}

type haulGroup struct {
	layer      region.Layer
	component  int32
	area, x, y int
	flee       bool
}

// A negative result with no physically compatible source/destination pair
// cannot be changed by reservations alone, even if the claim journal wraps.
type haulAdmission struct {
	offers         []HaulOffer
	staticNegative bool
}

type haulSource struct {
	id   entity.EntityID
	load item.Stack
}

// HaulIndex is a map-local derived availability index. In stable no-work
// conditions, all animals with the same area and reachable component share
// the negative result; neither the global item list nor storage is rescanned.
type HaulIndex struct {
	rev                     HaulRevision
	valid                   bool
	sources                 []haulSource
	sourceIDs               map[entity.EntityID]bool
	claims                  *reservation.Table
	groups                  map[haulGroup]haulAdmission
	scans, checks, searches uint64 // source rebuild, admission and destination work counters
}

func (i *HaulIndex) Counts() (scans, checks, searches uint64) { return i.scans, i.checks, i.searches }

func (i *HaulIndex) Offers(h AnimalHauler, claims *reservation.Table, id entity.EntityID, s HaulScope, rev HaulRevision) []HaulOffer {
	// Only claims on an indexed source or a ground storage cell can
	// change these offers. Human movement elsewhere never scans sources.
	prev := i.rev
	prev.Claims = rev.Claims
	if !i.valid || prev != rev || i.claims != claims {
		clear(i.groups)
		if !i.valid || rev.Stacks != i.rev.Stacks || rev.Zones != i.rev.Zones || rev.Fog != i.rev.Fog {
			i.rebuild(h)
		}
	} else if claims.ChangedSince(i.rev.Claims, func(target entity.EntityID, x, y int) bool {
		if target != 0 {
			return i.sourceIDs[target]
		}
		z, ok := stockZone(h, x, y)
		return ok && (z.Kind == zone.KindStockpile || z.Kind == zone.KindDump)
	}) {
		for key, admission := range i.groups {
			if !admission.staticNegative {
				delete(i.groups, key)
			}
		}
	}
	i.rev, i.valid, i.claims = rev, true, claims
	px, py := h.Position(id)
	key := haulGroup{layer: s.Layer, component: s.Regions.Label(s.Layer, px, py), area: s.Area, flee: s.Flee}
	if key.component < 0 {
		key.x, key.y = px, py // a blocked origin may border several components
	}
	if admission, ok := i.groups[key]; ok {
		return admission.offers
	}
	if i.groups == nil {
		i.groups = make(map[haulGroup]haulAdmission)
	}
	var offers []HaulOffer
	physical := availableStorage(h, id, s, px, py, true)
	if len(physical) == 0 {
		i.groups[key] = haulAdmission{staticNegative: true}
		return nil
	}
	type unavailable struct {
		load      item.Stack
		component int32
	}
	noDestination := make(map[unavailable]bool)
	physicalMiss := make(map[unavailable]bool)
	var dests map[int32]storageOptions
	possible := false
	for _, src := range i.sources {
		i.checks++
		st := src.load
		if !h.Allowed(id, st.X, st.Y) || !s.Safe(st.X, st.Y) || !s.Regions.Reach(s.Layer, px, py, st.X, st.Y) {
			continue
		}
		// The physical load is comparable; position does not affect storage
		// filters, so identical loads share a negative destination result.
		normalized := st
		normalized.X, normalized.Y = 0, 0
		normalized.Rot, normalized.DiedAt = 0, 0
		normalized.Unclaimed, normalized.HaulMarked = false, false
		k := unavailable{normalized, s.Regions.Label(s.Layer, st.X, st.Y)}
		if physicalMiss[k] {
			continue
		}
		limit := h.Items().StackLimit(st.Kind)
		if !physical[k.component].takes(st, limit) {
			physicalMiss[k] = true
			continue
		}
		if !possible {
			possible = true
			dests = availableStorage(h, id, s, px, py, false)
			if len(dests) == 0 {
				break // compatible storage exists, but it is reserved
			}
		}
		actual, ok := h.Stack(src.id)
		if !ok || claims.Free(src.id, actual.Qty) != actual.Qty || noDestination[k] {
			continue
		}
		if !dests[k.component].takes(st, limit) {
			noDestination[k] = true
			continue
		}
		i.searches++
		x, y, ok := store.Search(h, st, st.X, st.Y, func(x, y int) bool {
			return s.Safe(x, y) && s.Regions.Reach(s.Layer, st.X, st.Y, x, y) && animalDestination(h, id, st, x, y)
		}, zone.PriorityLow-1, false)
		if !ok {
			noDestination[k] = true
			continue
		}
		offers = append(offers, HaulOffer{src.id, st.X, st.Y, x, y, st.Qty})
		if len(offers) == haulFinalists {
			break
		}
	}
	i.groups[key] = haulAdmission{offers: offers, staticNegative: !possible}
	return offers
}

func (i *HaulIndex) rebuild(h AnimalHauler) {
	i.sources = i.sources[:0]
	if i.sourceIDs == nil {
		i.sourceIDs = make(map[entity.EntityID]bool)
	}
	clear(i.sourceIDs)
	for _, sid := range h.ItemIDs() {
		i.scans++
		st, ok := h.Stack(sid)
		if !ok || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden || st.Rotten() || !h.Visible(st.X, st.Y) {
			continue
		}
		if z, ok := stockZone(h, st.X, st.Y); ok && z.AllowsStack(st) {
			continue
		}
		// Original local tuning: 20 kg, capped by the ordinary haul limit.
		st.Qty = min(st.Qty, item.CarryLimit, 20000/max(1, h.Items().MassGrams(st.Kind)))
		if st.Qty > 0 {
			i.sources = append(i.sources, haulSource{sid, st})
			i.sourceIDs[sid] = true
		}
	}
}

// A component's storage cells are inspected once for the group, not once
// for each source. Only final placement invokes the existing exact search.
type storageSummary struct {
	zone  zone.Zone
	empty bool
	top   map[item.Kind][]item.Stack
}
type storageOptions []*storageSummary

func (o storageOptions) takes(st item.Stack, limit int) bool {
	for _, z := range o {
		if !z.zone.AllowsStack(st) {
			continue
		}
		if z.empty {
			return true
		}
		for _, top := range z.top[st.Kind] {
			if item.SameLoad(top, st) && top.Qty+st.Qty <= limit {
				return true
			}
		}
	}
	return false
}

func availableStorage(h AnimalHauler, id entity.EntityID, s HaulScope, px, py int, ignoreClaims bool) map[int32]storageOptions {
	out := make(map[int32]storageOptions)
	for _, priority := range zone.Priorities {
		for _, store := range h.Stores(priority) {
			z, ok := h.Zones().Zone(store.ID)
			if !ok || z.Kind != zone.KindStockpile && z.Kind != zone.KindDump {
				continue
			}
			perComponent := make(map[int32]*storageSummary)
			h.Zones().EachCell(z.ID, func(x, y int) bool {
				if !h.Allowed(id, x, y) || !s.Safe(x, y) || !s.Regions.Reach(s.Layer, px, py, x, y) || !ignoreClaims && h.TileTaken(x, y) || h.Blocked(x, y, false) {
					return true
				}
				top, occupied := h.First(x, y)
				if occupied && top.Qty >= h.Items().StackLimit(top.Kind) {
					return true
				}
				component := s.Regions.Label(s.Layer, x, y)
				p := perComponent[component]
				if p == nil {
					p = &storageSummary{zone: z}
					perComponent[component] = p
					out[component] = append(out[component], p)
				}
				if !occupied {
					p.empty = true
					return true
				}
				if p.top == nil {
					p.top = make(map[item.Kind][]item.Stack)
				}
				p.top[top.Kind] = append(p.top[top.Kind], top)
				return true
			})
		}
	}
	return out
}
