// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package idindex

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// Buckets files ids by cadence bucket (id % every), each bucket sorted,
// for a site that visits id on the ticks where (now+off+id)%every == 0
// (game/cadence.Due): a tick reads one bucket, not every id. It is built
// from one store, as Set is; the zero value is empty and ignores Add.
type Buckets struct {
	every uint64
	ids   [][]entity.EntityID // ids[k]: the ids with id%every == k, ascending
	src   any
	built bool
}

// From reports whether b was built from src (and not reset since).
func (b *Buckets) From(src any) bool { return b.built && b.src == src }

// Reset empties b for a build from src with period every (0 is 1).
func (b *Buckets) Reset(every uint64, src any) {
	every = max(every, 1)
	b.ids = slices.Grow(b.ids[:0], int(every))[:every]
	for k := range b.ids {
		b.ids[k] = b.ids[k][:0]
	}
	b.every, b.src, b.built = every, src, true
}

// Add files id in its bucket, in order, unless b holds it.
func (b *Buckets) Add(id entity.EntityID) {
	if b.every == 0 {
		return
	}
	k := uint64(id) % b.every
	if i, found := slices.BinarySearch(b.ids[k], id); !found {
		b.ids[k] = slices.Insert(b.ids[k], i, id)
	}
}

// Due is the bucket of the ids due at now with offset off, ascending. It
// is b's own slice: the caller must not change or keep it.
func (b *Buckets) Due(now, off uint64) []entity.EntityID {
	if b.every == 0 {
		return nil
	}
	return b.ids[(b.every-(now+off)%b.every)%b.every]
}
