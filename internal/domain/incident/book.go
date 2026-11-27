// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"slices"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// Class is the incident class name in plugin files, patch targets and a
// save's key table.
const Class = "incident"

// MaxBook is the most incidents a book holds: an ID is a byte.
const MaxBook = 256

// Book is a run's frozen set of incidents: core's first, then the
// plugins' in load order (PLG-026). An incident's ID is its position, so
// core's keep theirs. The zero Book is core.
type Book struct {
	defs []Def
	keys []string // keys[i] is defs[i]'s namespaced key
}

// Core is the game's own incidents, keyed "core.<Key>".
func Core() Book {
	keys := make([]string, len(defs))
	for i, d := range defs {
		keys[i] = "core." + d.Key
	}
	return Book{defs: defs[:], keys: keys}
}

// NewBook freezes list (core's first, in catalog order) with its
// namespaced keys, one per def. Each def's ID becomes its position. A list
// longer than MaxBook is cut there (the decoder refuses the rest first).
func NewBook(list []Def, keys []string) Book {
	n := min(len(list), len(keys), MaxBook)
	out := make([]Def, n)
	for i := range n {
		out[i] = list[i]
		out[i].ID = ID(i)
	}
	return Book{defs: out, keys: append([]string(nil), keys[:n]...)}
}

func (b Book) self() Book {
	if b.defs == nil {
		return Core()
	}
	return b
}

// list is the incidents Pick and Total draw among (core's package array
// for the zero Book: no copy).
func (b Book) list() []Def {
	if b.defs == nil {
		return defs[:]
	}
	return b.defs
}

// DefOf looks an incident up; an unknown ID gives the zero Def.
func (b Book) DefOf(id ID) Def {
	l := b.defs
	if l == nil {
		return DefOf(id)
	}
	if int(id) >= len(l) {
		return Def{}
	}
	return l[id]
}

// Like is the core event whose behaviour id runs (id itself for a core
// event or an unknown one).
func (b Book) Like(id ID) ID {
	if d := b.DefOf(id); d.Key != "" {
		return d.Like
	}
	return id
}

// All lists every incident in ID order (a copy).
func (b Book) All() []Def { return append([]Def(nil), b.self().defs...) }

// Len is the number of incidents.
func (b Book) Len() int {
	if b.defs == nil {
		return len(defs)
	}
	return len(b.defs)
}

// Key is id's namespaced key ("" for an unknown ID).
func (b Book) Key(id ID) string {
	s := b.self()
	if int(id) >= len(s.keys) {
		return ""
	}
	return s.keys[id]
}

// Keys is every key in ID order (index i is ID i's).
func (b Book) Keys() []string { return append([]string(nil), b.self().keys...) }

// IDOf finds an incident by namespaced key.
func (b Book) IDOf(key string) (ID, bool) {
	i := slices.Index(b.self().keys, key)
	return ID(i), i >= 0
}

// ByName finds an incident by its Def.Key or its namespaced key.
func (b Book) ByName(name string) (Def, bool) {
	s := b.self()
	for i, d := range s.defs {
		if d.Key == name || s.keys[i] == name {
			return d, true
		}
	}
	return Def{}, false
}

// Pick is the package's PickOdds over this book's incidents.
func (b Book) Pick(c Category, st State, now uint64, bi biome.ID, o Odds, roll int, ok func(ID) bool) (ID, bool) {
	return pickOdds(b.list(), c, st, now, bi, o, roll, ok)
}

// Total is the package's TotalOdds over this book's incidents.
func (b Book) Total(c Category, st State, now uint64, bi biome.ID, o Odds, ok func(ID) bool) int {
	return totalOdds(b.list(), c, st, now, bi, o, ok)
}

// PickThreat maps roll, a face of the threat die (0..ThreatRolls-1), to
// the threat event it shows at tick now in biome bi at odds o. Walked in
// ID order, each threat event in season (Season; UnderRaid, WorldObject and
// filler events never lay out a face) claims Weight faces, laid out from
// face ThreatRolls-2 round the die; the filler takes the faces left. An
// event that is not open (its day, biome and point minimum) hands its face
// to the filler, and a face whose event fails ok, the caller's situational
// check, falls back round the ring of pack, manhunters and hive from
// roll%3 (never to a WorldObject event; the hive only from its own first
// day). Cooldowns are the caller's to check at fire (Def.Cooled). It is
// pure and deterministic; ok is false when the face shows nothing.
func (b Book) PickThreat(now uint64, bi biome.ID, o Odds, roll int, ok func(ID) bool) (ID, bool) {
	if roll < 0 || roll >= ThreatRolls {
		return 0, false
	}
	list := b.list()
	day := int(now / clock.TicksPerDay)
	pos := (roll + ThreatRolls - (ThreatRolls - 2)) % ThreatRolls
	cand, found, hasFiller := threatFace(list, pos, day, bi, o)
	if !found && !hasFiller {
		return 0, false // no filler: a face no open event claims shows nothing
	}
	if found && (ok == nil || ok(cand)) {
		return cand, true
	}
	ring := [...]ID{PredatorPack, ManhunterPack, HiveSwarm}
	for i := range ring {
		alt := ring[(roll+i)%len(ring)]
		if int(alt) >= len(list) || list[alt].Target != HomeMap || alt == HiveSwarm && day < list[alt].MinDay {
			continue
		}
		if ok == nil || ok(alt) {
			return alt, true
		}
	}
	return 0, false
}

// threatFace is the event showing at position pos of the die laid out over
// list: the one owning it if it is open, else the filler (if it is open).
// hasFiller reports that the book has a filler.
func threatFace(list []Def, pos, day int, bi biome.ID, o Odds) (id ID, found, hasFiller bool) {
	at, filler := 0, -1
	owner := -1
	for i, d := range list {
		if d.Category != Threat {
			continue
		}
		if d.Fill > 0 {
			if filler < 0 && d.Target == HomeMap {
				filler = i
			}
			continue
		}
		if d.Weight <= 0 || d.UnderRaid || d.Target != HomeMap || !d.Season.Allows(o.Winter) {
			continue
		}
		if owner < 0 && pos >= at && pos < at+d.Weight {
			owner = i
		}
		at += d.Weight
	}
	if owner >= 0 && list[owner].open(day, bi, o) {
		return list[owner].ID, true, filler >= 0
	}
	if filler >= 0 && list[filler].open(day, bi, o) {
		return list[filler].ID, true, true
	}
	return 0, false, filler >= 0
}

// LastFired is the event of category c that fired most recently in st,
// with the tick it fired; ok is false when none has. A tie goes to an
// UnderRaid event, then to the higher ID.
func (b Book) LastFired(st State, c Category) (Def, uint64, bool) {
	var best Def
	var at uint64
	found := false
	for _, d := range b.list() {
		t, fired := st.Last[d.ID]
		if d.Category != c || !fired {
			continue
		}
		if !found || t > at || t == at && (d.UnderRaid || !best.UnderRaid) {
			best, at, found = d, t, true
		}
	}
	return best, at, found
}
