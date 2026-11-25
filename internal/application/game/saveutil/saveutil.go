// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package saveutil holds pure compatibility transforms used by game saves.
package saveutil

import (
	"encoding/binary"
	"hash/fnv"
	"maps"
	"math/bits"
	"reflect"
	"slices"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// BitIndexes lists set bits in ascending order.
func BitIndexes(words []uint64) []int {
	n := 0
	for _, w := range words {
		n += bits.OnesCount64(w)
	}
	out := make([]int, 0, n)
	for wi, w := range words {
		for w != 0 {
			out = append(out, wi*64+bits.TrailingZeros64(w))
			w &= w - 1
		}
	}
	return out
}

// TerrainHash returns the deterministic seed for an older save's geology.
func TerrainHash(t terrain.Map) uint64 {
	h := fnv.New64a()
	var hdr [16]byte
	binary.LittleEndian.PutUint64(hdr[:8], uint64(t.W))
	binary.LittleEndian.PutUint64(hdr[8:], uint64(t.H))
	_, _ = h.Write(hdr[:])
	cells := make([]byte, len(t.Cells))
	for i, c := range t.Cells {
		cells[i] = byte(c)
	}
	_, _ = h.Write(cells)
	return h.Sum64()
}

// NilIfEmpty normalizes a mutable empty map for Gob round-trip stability:
// decoding an empty map does not preserve whether it was allocated.
func NilIfEmpty[M ~map[K]V, K comparable, V any](m M) M {
	if len(m) == 0 {
		return nil
	}
	return m
}

// ExportPrios writes work priorities as slices, preserving appended columns.
func ExportPrios(m map[entity.EntityID]work.Priorities) map[entity.EntityID][]uint8 {
	out := make(map[entity.EntityID][]uint8, len(m))
	for id, p := range m {
		out[id] = slices.Clone(p[:])
	}
	return out
}

// ImportPrios restores older, shorter lists without shifting work indices.
func ImportPrios(m map[entity.EntityID][]uint8) map[entity.EntityID]work.Priorities {
	out := make(map[entity.EntityID]work.Priorities, len(m))
	for id, l := range m {
		var p work.Priorities
		copy(p[:], l)
		if len(l) <= int(work.PlantCut) {
			p[work.PlantCut] = p[work.Grow]
		}
		if len(l) <= int(work.Art) {
			p[work.Art] = p[work.Craft]
		}
		out[id] = p
	}
	return out
}

// CanonicalHashFields keeps replay hashes independent of save-only encoding
// choices: research width, legacy field aliases, priority shape and RNG seed.
func CanonicalHashFields(f map[string]any, seedKey string) {
	research := intSlice(f["ResearchAll"])
	if research == nil {
		research = intSlice(f["Research"])
	}
	for len(research) > 0 && research[len(research)-1] == 0 {
		research = research[:len(research)-1]
	}
	f["Research"] = research
	for _, key := range [...]string{
		"ResearchAll", "BlockedBits", "SkillLive", "StainDrip", "PostSweep",
		"DoorsOriented", "ContentKeys", "View",
	} {
		delete(f, key)
	}
	if ms, ok := f["Memories"].(map[entity.EntityID]mood.Memories); ok {
		copy := maps.Clone(ms)
		for id, memory := range copy {
			if len(memory) == 0 {
				copy[id] = nil
			}
		}
		f["Memories"] = copy
	}
	if l, ok := f["PrioList"].(map[entity.EntityID][]uint8); ok && l != nil {
		delete(f, "PrioList")
		prios := ImportPrios(l)
		var art map[entity.EntityID]uint8
		for id, p := range prios {
			if p[work.Art] != p[work.Craft] {
				if art == nil {
					art = make(map[entity.EntityID]uint8)
				}
				art[id] = p[work.Art]
			}
			p[work.Art] = 0
			prios[id] = p
		}
		f["Prios"], f["ArtPriorityOverrides"] = prios, art
	}
	if streams, ok := f["RNGStreams"].(map[string][]byte); ok && streams != nil {
		withoutSeed := maps.Clone(streams)
		delete(withoutSeed, seedKey)
		f["RNGStreams"] = withoutSeed
	}
}

func intSlice(x any) []int {
	v := reflect.ValueOf(x)
	if !v.IsValid() || (v.Kind() != reflect.Array && v.Kind() != reflect.Slice) || v.Type().Elem().Kind() != reflect.Int {
		return nil
	}
	out := make([]int, v.Len())
	for i := range out {
		out[i] = int(v.Index(i).Int())
	}
	return slices.Clip(out)
}

// NilEmptyMapValues normalizes map payloads because Gob decodes zero-length slice values in maps as nil. Preserve map keys
// and nonempty values without changing the live map or allocating in the
// normal case.
func NilEmptyMapValues[K comparable, V any](m map[K][]V) map[K][]V {
	var out map[K][]V
	for k, v := range m {
		if v != nil && len(v) == 0 {
			if out == nil {
				out = make(map[K][]V, len(m))
				for key, value := range m {
					out[key] = value
				}
			}
			out[k] = nil
		}
	}
	if out != nil {
		return out
	}
	return m
}

// ExportedFields names the authoritative Gob fields without encoding the
// wire stream. Callers normalize representation-specific fields separately.
func ExportedFields(state any, info any) map[string]any {
	f := map[string]any{"Info": info}
	v := reflect.ValueOf(state).Elem()
	t := v.Type()
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			f[t.Field(i).Name] = v.Field(i).Interface()
		}
	}
	return f
}

// InheritArtMaskOnly migrates old mask-only records. A complete current
// row is always explicit; its Art preference must never be overwritten.
func InheritArtMaskOnly[E EntryShape[work.Mask]](store *component.Store[work.Mask], wide []E, rows map[entity.EntityID][]uint8) {
	for _, e := range wide {
		id, m := entryParts[work.Mask](e)
		if _, has := rows[id]; !has {
			store.Set(id, m.Set(work.Art, m.Enabled(work.Craft)))
		}
	}
}

// SyncWorkFlags preserves the legacy PlantCut migration and widens the
// historical priority rows, without altering independent current Art bits.
func SyncWorkFlags(store *component.Store[work.Mask], prios map[entity.EntityID]work.Priorities, rows map[entity.EntityID][]uint8) {
	for id, p := range prios {
		if m, ok := store.Get(id); ok {
			if len(rows[id]) <= int(work.Art) {
				m = m.Set(work.Art, p[work.Art] > 0)
			}
			store.Set(id, m.Set(work.PlantCut, p[work.PlantCut] > 0))
		}
	}
}

// HashLabor retains the historical low/high bit projection. When Art
// inherits Craft, the old value is sufficient to reconstruct the new flag.
// An independent Art choice (including off) gets its own logical field.
func HashLabor[L ~uint16, E EntryShape[L], W EntryShape[work.Mask]](f map[string]any, wide []W, rows map[entity.EntityID][]uint8) {
	if wide == nil {
		return
	}
	low := make([]E, len(wide))
	var high []W
	var overrides map[entity.EntityID]bool
	for i, e := range wide {
		id, m := entryParts[work.Mask](e)
		if len(rows[id]) > int(work.Art) && m.Enabled(work.Art) != m.Enabled(work.Craft) {
			if overrides == nil {
				overrides = make(map[entity.EntityID]bool)
			}
			overrides[id] = m.Enabled(work.Art)
		}
		m = m.Set(work.Art, false)
		low[i] = makeEntry[L, E](id, L(m))
		if hi := m &^ work.Mask(0xffff); hi != 0 {
			high = append(high, makeEntry[work.Mask, W](id, hi))
		}
	}
	f["Labor"], f["LaborWide"], f["ArtLaborOverrides"] = low, high, overrides
}
