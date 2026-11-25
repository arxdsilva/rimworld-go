// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saveutil

import (
	"maps"
	"slices"

	"rimworld-go/internal/application/game/animalstate"
	"rimworld-go/internal/application/game/forage"
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// OwnSlice and OwnMap copy borrowed containers only for asynchronous
// capture. Synchronous encode/hash may read them on the simulation owner.
func OwnSlice[S ~[]E, E any](own bool, in S) S {
	if own {
		return slices.Clone(in)
	}
	return in
}

func OwnMap[M ~map[K]V, K comparable, V any](own bool, in M) M {
	if own {
		return maps.Clone(in)
	}
	return in
}

// OwnValues copies both the map and its borrowed value payloads.
func OwnValues[M ~map[K]V, K comparable, V any](own bool, in M, clone func(V) V) M {
	if !own || in == nil {
		return in
	}
	out := make(M, len(in))
	for k, v := range in {
		out[k] = clone(v)
	}
	return out
}

// OwnElements copies a borrowed slice and its value payloads.
func OwnElements[S ~[]E, E any](own bool, in S, clone func(E) E) S {
	return DetachElements(own, OwnSlice(own, in), clone)
}

// DetachElements detaches nested payloads in an already owned slice.
func DetachElements[S ~[]E, E any](own bool, in S, clone func(E) E) S {
	if own {
		for i := range in {
			in[i] = clone(in[i])
		}
	}
	return in
}

// DetachValues does not copy a map already constructed by export.
func DetachValues[M ~map[K]V, K comparable, V any](own bool, in M, clone func(V) V) M {
	if own {
		for k, v := range in {
			in[k] = clone(v)
		}
	}
	return in
}

// OwnSliceValues copies a map of borrowed scalar slices.
func OwnSliceValues[M ~map[K]S, K comparable, S ~[]E, E any](own bool, in M) M {
	return OwnValues(own, in, slices.Clone[S])
}

// DetachEntries owns payloads in the root's already exported entry list.
// The structural constraint preserves the original Gob type and fields.
func DetachEntries[T any, E ~struct {
	ID entity.EntityID
	V  T
}](own bool, in []E, clone func(T) T) []E {
	if own {
		for i, e := range in {
			v := struct {
				ID entity.EntityID
				V  T
			}(e)
			v.V = clone(v.V)
			in[i] = E(v)
		}
	}
	return in
}

func CloneJob(j job.Record) job.Record {
	j.Path, j.Route, j.Pickups = slices.Clone(j.Path), slices.Clone(j.Route), slices.Clone(j.Pickups)
	return j
}

func ClonePawn(p pawngen.Pawn) pawngen.Pawn {
	p.Traits, p.Conditions = slices.Clone(p.Traits), slices.Clone(p.Conditions)
	return p
}

func CloneBody(b *body.Body) *body.Body {
	if b == nil {
		return nil
	}
	out := *b
	out.Wounds, out.Replacements = slices.Clone(b.Wounds), slices.Clone(b.Replacements)
	out.Implants, out.Fitted, out.Conds = slices.Clone(b.Implants), slices.Clone(b.Fitted), slices.Clone(b.Conds)
	return &out
}

func CloneAnimalRecord(r animalstate.Record) animalstate.Record {
	r.State.Motion.Path = slices.Clone(r.State.Motion.Path)
	r.State.Medical.Body = CloneBody(r.State.Medical.Body)
	return r
}

func CloneBills(in []recipe.Bill) []recipe.Bill {
	return OwnElements(true, in, func(b recipe.Bill) recipe.Bill {
		b.Forbid = slices.Clone(b.Forbid)
		return b
	})
}

func CloneLetter(l letters.Letter) letters.Letter {
	l.Fields, l.Choices = slices.Clone(l.Fields), slices.Clone(l.Choices)
	return l
}

func OwnTerrain(own bool, t terrain.Map) terrain.Map {
	t.Cells = OwnSlice(own, t.Cells)
	return t
}

func OwnGeology(own bool, g geology.Map) geology.Map {
	g.Stone, g.Ore = OwnSlice(own, g.Stone), OwnSlice(own, g.Ore)
	return g
}

func CloneRaidGroup(g raid.Group) raid.Group {
	g.Pods, g.Mortars = slices.Clone(g.Pods), slices.Clone(g.Mortars)
	return g
}

func OwnIncidents(own bool, s incident.State) incident.State {
	s.Last, s.Queue = OwnMap(own, s.Last), OwnSlice(own, s.Queue)
	return s
}

func OwnForage(own bool, m forage.Memory) forage.Memory {
	m.Skip, m.Far = OwnSliceValues(own, m.Skip), OwnMap(own, m.Far)
	return m
}

func OwnHerd(own bool, s herd.Settings) herd.Settings {
	s.Caps = OwnMap(own, s.Caps)
	return s
}

func OwnRage(own bool, m rage.Memory) rage.Memory {
	m.Bash, m.Lock, m.Call = OwnMap(own, m.Bash), OwnMap(own, m.Lock), OwnMap(own, m.Call)
	return m
}

// OwnSense keeps the already allocated export and detaches the two
// nested payloads that are views of the live tidy/pack stores.
func OwnSense(own bool, s *sense.Save) *sense.Save {
	if own && s != nil {
		for i := range s.Resume {
			s.Resume[i].Job = CloneJob(s.Resume[i].Job)
		}
		for i := range s.Packs {
			p := &s.Packs[i].Pack
			p.Items = slices.Clone(p.Items)
		}
	}
	return s
}

func ClonePenSettings(s mapview.PenSettings) mapview.PenSettings {
	s.AutoCut, s.Species = slices.Clone(s.AutoCut), slices.Clone(s.Species)
	return s
}

// OwnBlobs detaches accepted provider bytes; Check already owns the list.
func OwnBlobs(own bool, b []modstate.Blob) []modstate.Blob {
	return DetachElements(own, b, func(v modstate.Blob) modstate.Blob {
		v.Data = slices.Clone(v.Data)
		return v
	})
}

// OwnPointerSlice copies a slice referenced through a comparable DTO field.
func OwnPointerSlice[S ~[]E, E any](own bool, p *S) *S {
	if !own || p == nil {
		return p
	}
	out := slices.Clone(*p)
	return &out
}

func OwnPawn(own bool, p pawngen.Pawn) pawngen.Pawn {
	if own {
		return ClonePawn(p)
	}
	return p
}

func OwnBody(own bool, b body.Body) body.Body {
	if own {
		return *CloneBody(&b)
	}
	return b
}

func OwnDrug(own bool, s drug.State) drug.State {
	s.Highs, s.Addictions = OwnSlice(own, s.Highs), OwnSlice(own, s.Addictions)
	s.LastDose, s.Tolerance = OwnSlice(own, s.LastDose), OwnSlice(own, s.Tolerance)
	return s
}
