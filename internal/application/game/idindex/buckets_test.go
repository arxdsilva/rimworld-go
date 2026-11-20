// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package idindex

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/domain/sim/entity"
)

// TestPawnBucketsDueIsTheCadenceFilter: over two periods and several
// offsets, Due is exactly the ids cadence.Due picks, ascending.
func TestPawnBucketsDueIsTheCadenceFilter(t *testing.T) {
	t.Parallel()
	for _, every := range []uint64{1, 7, 60} {
		var b Buckets
		b.Reset(every, "src")
		for id := entity.EntityID(500); id >= 1; id-- { // out of order on purpose
			b.Add(id)
		}
		for _, off := range []uint64{0, 1, 41, 83, 59} {
			for now := uint64(0); now < 2*every; now++ {
				var want []entity.EntityID
				for id := entity.EntityID(1); id <= 500; id++ {
					if cadence.Due(now, every, off, id) {
						want = append(want, id)
					}
				}
				if got := b.Due(now, off); !slices.Equal(got, want) {
					t.Fatalf("every %d off %d now %d: %v, want %v", every, off, now, got, want)
				}
			}
		}
	}
}

// TestPawnBucketsSortedNoDuplicates: an out-of-order Add sorts in, and a
// second Add of an id changes nothing.
func TestPawnBucketsSortedNoDuplicates(t *testing.T) {
	t.Parallel()
	var b Buckets
	b.Reset(10, 1)
	for _, id := range []entity.EntityID{33, 3, 23, 13, 23, 3, 43} {
		b.Add(id)
	}
	if got := b.Due(7, 0); !slices.Equal(got, []entity.EntityID{3, 13, 23, 33, 43}) {
		t.Fatalf("bucket 3: %v", got)
	}
}

// TestPawnBucketsFromAndReset: as Set, From holds only for the source of
// the last Reset; Reset empties every bucket; the zero value is empty.
func TestPawnBucketsFromAndReset(t *testing.T) {
	t.Parallel()
	var b Buckets
	a, c := new(int), new(int)
	if b.From(nil) || b.From(a) {
		t.Fatal("a zero Buckets is built from nothing")
	}
	b.Add(5) // ignored before a Reset
	if got := b.Due(0, 0); got != nil {
		t.Fatalf("zero value Due: %v", got)
	}
	b.Reset(4, a)
	b.Add(5)
	if !b.From(a) || b.From(c) {
		t.Fatal("From after Reset(a)")
	}
	b.Reset(4, c)
	if b.From(a) || !b.From(c) {
		t.Fatal("From after Reset(c)")
	}
	for now := range uint64(4) {
		if got := b.Due(now, 0); len(got) != 0 {
			t.Fatalf("after Reset, tick %d: %v", now, got)
		}
	}
	b.Reset(0, a) // a period of 0 is one bucket
	b.Add(9)
	if got := b.Due(3, 0); !slices.Equal(got, []entity.EntityID{9}) {
		t.Fatalf("every 0: %v", got)
	}
}
