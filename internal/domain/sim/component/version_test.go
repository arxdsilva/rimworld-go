// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package component

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestStoreVersionTracksValueWritesOnly(t *testing.T) {
	s := NewStore[int]()
	if s.Ver() != 0 {
		t.Fatalf("empty store version = %d", s.Ver())
	}
	s.Set(1, 10)
	s.Set(1, 10) // replacement, even with the same value
	s.Set(2, 20)
	if s.Ver() != 3 {
		t.Fatalf("three Sets: version = %d", s.Ver())
	}
	if v, ok := s.Get(1); !ok || v != 10 || s.Ref(2) == nil || !s.Has(1) {
		t.Fatal("ordinary reads lost values")
	}
	s.All(func(_ entity.EntityID, _ int) {})
	if s.Ver() != 3 {
		t.Fatalf("reads moved version: %d", s.Ver())
	}
	s.Delete(99)
	if s.Ver() != 3 {
		t.Fatalf("missing Delete moved version: %d", s.Ver())
	}
	s.Delete(1)
	if s.Ver() != 4 || s.Has(1) || !s.Has(2) {
		t.Fatalf("existing Delete: version=%d len=%d", s.Ver(), s.Len())
	}
}
