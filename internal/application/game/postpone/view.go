// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package postpone

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Tops is, per pawn, the postponed work a view shows (not expired,
// not about to clear, target still there), its first ViewLines entries.
// It reuses buf and only reads the store.
func (s *Store) Tops(buf []Top, now, rev uint64, targetGone func(Key) bool) []Top {
	buf = buf[:0]
	if s.Len() == 0 {
		return buf
	}
	stale := s.Rev != rev
	s.All(func(k Key, v Val) {
		if now >= v.Until || v.Unreach && stale || targetGone(k) || k.Kind == job.Equip && k.Target == 0 {
			return
		}
		var t *Top
		for i := range buf {
			if buf[i].Pawn == k.Pawn {
				t = &buf[i]
				break
			}
		}
		if t == nil {
			buf = append(buf, Top{Pawn: k.Pawn})
			t = &buf[len(buf)-1]
		}
		t.Add(Entry{Key: k, Until: v.Until, Unreach: v.Unreach})
	})
	return buf
}

// Lines formats one pawn's selected postponed work, preserving view order.
func Lines(tops []Top, id entity.EntityID, now uint64, label func(Key) string) []string {
	var t *Top
	for i := range tops {
		if tops[i].Pawn == id {
			t = &tops[i]
			break
		}
	}
	if t == nil || t.N == 0 {
		return nil
	}
	var out []string
	for _, e := range t.E[:t.N] {
		if len(out) == ViewLines {
			break
		}
		if e.Unreach {
			out = append(out, label(e.Key)+": can't reach")
			continue
		}
		secs := (e.Until - now + 59) / 60
		out = append(out, fmt.Sprintf("%s: blocked, retry in %d s", label(e.Key), secs))
	}
	return out
}
