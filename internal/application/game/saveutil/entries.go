// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saveutil

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

// EntryShape accepts the game package's existing wire entry type without
// changing the concrete type name seen by Gob.
type EntryShape[T any] interface {
	~struct {
		ID entity.EntityID
		V  T
	}
}

func entryParts[T any, E EntryShape[T]](e E) (entity.EntityID, T) {
	v := struct {
		ID entity.EntityID
		V  T
	}(e)
	return v.ID, v.V
}

func makeEntry[T any, E EntryShape[T]](id entity.EntityID, v T) E {
	return E(struct {
		ID entity.EntityID
		V  T
	}{id, v})
}

// ExportStore preserves the store's established iteration order.
func ExportStore[E EntryShape[T], T any](s *component.Store[T]) []E {
	out := make([]E, 0, s.Len())
	s.All(func(id entity.EntityID, v T) { out = append(out, makeEntry[T, E](id, v)) })
	return out
}

func ImportStore[E EntryShape[T], T any](es []E) *component.Store[T] {
	s := component.NewStore[T]()
	for _, e := range es {
		id, v := entryParts[T](e)
		s.Set(id, v)
	}
	return s
}

// ExportMap lists a pointer map's entries in id order (nil when empty).
func ExportMap[E EntryShape[T], T any](m map[entity.EntityID]*T) []E {
	if len(m) == 0 {
		return nil
	}
	out := make([]E, 0, len(m))
	for id, v := range m {
		out = append(out, makeEntry[T, E](id, *v))
	}
	sortEntries[T](out)
	return out
}

func sortEntries[T any, E EntryShape[T]](out []E) {
	slices.SortFunc(out, func(a, b E) int { aid, _ := entryParts[T](a); bid, _ := entryParts[T](b); return cmp.Compare(aid, bid) })
}

func ImportMap[E EntryShape[T], T any](es []E) map[entity.EntityID]*T {
	if len(es) == 0 {
		return nil
	}
	m := make(map[entity.EntityID]*T, len(es))
	for _, e := range es {
		id, v := entryParts[T](e)
		m[id] = &v
	}
	return m
}

// ExportValues lists a value map's entries directly, avoiding a temporary
// pointer map; the returned wire order is unchanged.
func ExportValues[E EntryShape[T], T any](m map[entity.EntityID]T) []E {
	if len(m) == 0 {
		return nil
	}
	out := make([]E, 0, len(m))
	for id, v := range m {
		out = append(out, makeEntry[T, E](id, v))
	}
	sortEntries[T](out)
	return out
}

func ImportValues[E EntryShape[T], T any](es []E) map[entity.EntityID]T {
	m := make(map[entity.EntityID]T, len(es))
	for _, e := range es {
		id, v := entryParts[T](e)
		m[id] = v
	}
	return m
}

// SetMap installs a decoded map, keeping the empty one Gob leaves nil.
func SetMap[K comparable, V any](dst *map[K]V, src map[K]V) {
	if src != nil {
		*dst = src
	}
}
