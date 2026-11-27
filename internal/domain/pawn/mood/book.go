// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import "strings"

// Class is the moddable definition class of thoughts.
const Class = "thought"

// corePrefix namespaces every thought key.
const corePrefix = "core."

// Tuning is a thought's patchable numbers and line.
type Tuning struct {
	Label       string
	Value       int
	Duration    uint64
	StackLimit  uint8
	StackFactor int32
}

// Book is the frozen thought definitions of one run: core's, with the
// patches plugins applied. The zero value is core.
//
// A thought whose value or line the game computes (see CodeValued) cannot
// be patched: a new call site that computes a value must add its kind
// there.
type Book struct {
	defs []Def
	keys []string
}

// Core is the build's own thought book.
func Core() Book { return Book{defs: defs[:], keys: keys[:]} }

// NewBook is the book of the given tunings in core order; it is
// (Core(), false) unless there is one tuning per core thought.
func NewBook(list []Tuning) (Book, bool) {
	if len(list) != int(numKinds) {
		return Core(), false
	}
	patched := make([]Def, numKinds)
	core := Core()
	copy(patched, core.defs)
	for k, t := range list {
		patched[k].Label = t.Label
		patched[k].Value = t.Value
		patched[k].Duration = t.Duration
		patched[k].StackLimit = t.StackLimit
		patched[k].StackFactor = t.StackFactor
	}
	return Book{defs: patched, keys: core.keys}, true
}

// CodeValued reports whether the game computes k's value or line, so a
// plugin cannot patch it.
func CodeValued(k Kind) bool {
	switch k {
	case Malnourished, PartnerOpinion, Grief, AugmentFan, NaturalPurist,
		Difficulty, LovesWork, TaintedApparel, SharedBed,
		ComfortMood, OutdoorsMood, BeautyMood,
		SleptInBedroom, SleptInBarracks, SleptInHospital, AteInDiningRoom, EnjoyedRecRoom,
		ExpectationsExtremelyLow, ExpectationsVeryLow, ExpectationsLow,
		ExpectationsModerate, ExpectationsHigh, ExpectationsSkyHigh:
		return true
	}
	return false
}

// table is the definitions, core's when the book is the zero value.
func (b Book) table() []Def {
	if b.defs == nil {
		return Core().defs
	}
	return b.defs
}

// Len is how many thoughts the book defines.
func (b Book) Len() int { return len(b.table()) }

// DefOf returns k's definition, Def{} out of range; StackLimit 0 reads
// as 1 and StackFactor 0 as 1000.
func (b Book) DefOf(k Kind) Def { return defIn(b.table(), k) }

// defIn is k's definition in table t with the stacking defaults applied.
func defIn(t []Def, k Kind) Def {
	if int(k) >= len(t) {
		return Def{}
	}
	d := t[k]
	if d.StackLimit == 0 {
		d.StackLimit = 1
	}
	if d.StackFactor == 0 {
		d.StackFactor = 1000
	}
	return d
}

func (b Book) keyTable() []string {
	if b.keys == nil {
		return Core().keys
	}
	return b.keys
}

// Key is k's namespaced key ("core.<key>"), "" out of range.
func (b Book) Key(k Kind) string {
	t := b.keyTable()
	if int(k) >= len(t) {
		return ""
	}
	return corePrefix + t[k]
}

// Keys is every thought's namespaced key in kind order.
func (b Book) Keys() []string {
	t := b.keyTable()
	out := make([]string, len(t))
	for i, key := range t {
		out[i] = corePrefix + key
	}
	return out
}

// KindOf is the kind of a namespaced key.
func (b Book) KindOf(key string) (Kind, bool) {
	name, ok := strings.CutPrefix(key, corePrefix)
	if !ok {
		return 0, false
	}
	for k, n := range b.keyTable() {
		if n == name {
			return Kind(k), true
		}
	}
	return 0, false
}

// Add records a thought of kind k at now, replacing an existing one of the
// same kind and any other thought of the same group.
func (b Book) Add(m Memories, k Kind, now uint64) Memories {
	d := b.DefOf(k)
	if d.StackLimit > 1 {
		return b.AddLabeledUntil(m, k, 0, 0, "", now, now+d.Duration)
	}
	m = b.compactFor(m, k, d.Group, now)
	for i := range m {
		if m[i].Kind == k {
			m[i].Expires = now + d.Duration
			return m
		}
	}
	return append(m, Memory{Kind: k, Expires: now + d.Duration})
}

// AddFor records a thought of kind k about subject at now. Memories of the
// same kind and subject stack up to Def.StackLimit; beyond it the oldest
// is refreshed. Group replacement works as in Add.
func (b Book) AddFor(m Memories, k Kind, subject, now uint64) Memories {
	return b.AddLabeled(m, k, subject, 0, "", now)
}

// AddValue records a thought of kind k about subject with a computed value.
func (b Book) AddValue(m Memories, k Kind, subject uint64, value int, now uint64) Memories {
	return b.AddLabeled(m, k, subject, value, "", now)
}

// AddLabeled records a thought of kind k with a computed value and line
// ("": the def's).
func (b Book) AddLabeled(m Memories, k Kind, subject uint64, value int, label string, now uint64) Memories {
	return b.AddLabeledUntil(m, k, subject, value, label, now, now+b.DefOf(k).Duration)
}

// AddLabeledUntil is AddLabeled with an explicit expiry tick.
func (b Book) AddLabeledUntil(m Memories, k Kind, subject uint64, value int, label string, now, expires uint64) Memories {
	d := b.DefOf(k)
	m = b.compactFor(m, k, d.Group, now)
	n, oldest := 0, -1
	for i := range m {
		if m[i].Kind != k || d.StackLimit == 1 && m[i].Subject != subject {
			continue
		}
		n++
		if oldest < 0 || m[i].Expires < m[oldest].Expires {
			oldest = i
		}
	}
	fresh := Memory{Kind: k, Expires: expires, Subject: subject, Value: value, Label: label}
	if n >= int(d.StackLimit) && oldest >= 0 {
		if d.StackLimit == 1 {
			m[oldest] = fresh
			return m
		}
		// The refreshed event is newest; stacking weights follow event order.
		copy(m[oldest:], m[oldest+1:])
		m[len(m)-1] = fresh
		return m
	}
	return append(m, fresh)
}

// Active is the thoughts m and the current states contribute.
func (b Book) Active(m Memories, states ...Kind) []Thought {
	return b.AppendActive(nil, m, states...)
}

// AppendActive appends Active(m, states...) to out.
func (b Book) AppendActive(out []Thought, m Memories, states ...Kind) []Thought {
	return b.appendActive(out, m, 0, false, states...)
}

// AppendActiveAt appends the thoughts active at now to out, skipping
// expired memories. It leaves the backing memory slice untouched.
func (b Book) AppendActiveAt(out []Thought, m Memories, now uint64, states ...Kind) []Thought {
	return b.appendActive(out, m, now, true, states...)
}

func (b Book) appendActive(out []Thought, m Memories, now uint64, checkExpiry bool, states ...Kind) []Thought {
	if len(m) == 0 { // states only: no stacking tables needed
		for _, k := range states {
			d := b.DefOf(k)
			out = append(out, Thought{Kind: k, Label: d.Label, Value: d.Value})
		}
		return out
	}
	var counts [numKinds]uint8
	var weights, totals [numKinds]int64
	var shown [numKinds]int
	t := b.table()
	for _, x := range m {
		if checkExpiry && x.Expires <= now {
			continue
		}
		d := defIn(t, x.Kind)
		v := d.Value
		if x.Value != 0 {
			v = x.Value
		}
		if x.Kind < numKinds && d.StackLimit > 1 {
			k := x.Kind
			if counts[k] >= d.StackLimit {
				v = 0 // tolerate old saves with more memories than today's cap
			} else {
				w := weights[k]
				if w == 0 {
					w = stackPrecision
				} else {
					w = w * int64(d.StackFactor) / 1000
				}
				weights[k] = w
				totals[k] += int64(v) * w
				n := stackRounded(totals[k])
				v, shown[k] = n-shown[k], n
				counts[k]++
			}
		}
		label := d.Label
		if x.Label != "" {
			label = x.Label
		}
		out = append(out, Thought{Kind: x.Kind, Label: label, Value: v, Memory: true})
	}
	for _, k := range states {
		d := b.DefOf(k)
		out = append(out, Thought{Kind: k, Label: d.Label, Value: d.Value})
	}
	return out
}
