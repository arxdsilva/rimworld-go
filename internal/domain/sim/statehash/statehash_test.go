package statehash

import "testing"

type inner struct {
	A int
	b int // unexported: ignored
}

type state struct {
	M  map[int]string
	S  []inner
	P  *inner
	F  float64
	B  []byte
	N  []int
	Ok bool
}

func TestOfIsStableAcrossMapOrder(t *testing.T) {
	mk := func() state {
		m := map[int]string{}
		for i := 0; i < 200; i++ {
			m[i*7919%1000] = string(rune('a' + i%26))
		}
		return state{M: m, S: []inner{{1, 2}}, P: &inner{A: 3}, F: 1.5, B: []byte("xy"), Ok: true}
	}
	want := Of(mk())
	for i := 0; i < 20; i++ {
		if got := Of(mk()); got != want {
			t.Fatalf("hash %d = %x, want %x", i, got, want)
		}
	}
}

func TestOfDistinguishesState(t *testing.T) {
	base := state{S: []inner{{1, 0}}}
	cases := map[string]state{
		"value":     {S: []inner{{2, 0}}},
		"nil slice": {S: nil},
		"empty":     {S: []inner{}},
		"ptr":       {S: []inner{{1, 0}}, P: &inner{}},
		"bool":      {S: []inner{{1, 0}}, Ok: true},
		"map":       {S: []inner{{1, 0}}, M: map[int]string{}},
	}
	h := Of(base)
	for name, c := range cases {
		if Of(c) == h {
			t.Errorf("%s: same hash as base", name)
		}
	}
	if Of(state{S: []inner{{1, 9}}}) != h {
		t.Error("unexported field changed the hash")
	}
}

func TestBytes(t *testing.T) {
	// FNV-64a of the empty input is the offset basis.
	if got := Bytes(nil); got != 0xcbf29ce484222325 {
		t.Fatalf("Bytes(nil) = %x", got)
	}
	if Bytes([]byte("a")) == Bytes([]byte("b")) {
		t.Fatal("collision")
	}
}

func TestOfNilAndKinds(t *testing.T) {
	var p *inner
	var i any
	_ = Of(nil)
	if Of(p) == Of(&inner{}) {
		t.Fatal("nil pointer equals zero struct pointer")
	}
	_ = Of(i)
	_ = Of(complex(1, 2))
	_ = Of([2]uint8{1, 2})
	_ = Of(func() {})
	_ = Of(struct{ X any }{X: 3})
}

func TestOfFieldsSkipsZeroAndOrders(t *testing.T) {
	a := OfFields(map[string]any{"A": 1, "B": []int{2}})
	b := OfFields(map[string]any{"B": []int{2}, "A": 1, "C": 0, "D": []int{}, "E": map[int]int(nil), "F": nil})
	if a != b {
		t.Fatal("zero fields changed the hash")
	}
	if OfFields(map[string]any{"A": 1, "B": []int{3}}) == a || OfFields(map[string]any{"X": 1, "B": []int{2}}) == a {
		t.Fatal("value or name change not seen")
	}
}

// TestOfFieldsAppendedZeroFields: OfFields encodes values sparsely, so a
// struct field or array slot appended at its zero value (an additive save
// field deep inside a record) keeps the hash, while any non-zero value,
// even in the new field, still changes it.
func TestOfFieldsAppendedZeroFields(t *testing.T) {
	type v1 struct {
		A int
		P [2]uint8
	}
	type v2 struct {
		A int
		P [3]uint8
		B bool
		S []int
	}
	old := OfFields(map[string]any{"M": map[v1]v1{{A: 1}: {A: 2}}})
	if OfFields(map[string]any{"M": map[v1]v1{{A: 1}: {A: 2, P: [2]uint8{0, 0}}}}) != old {
		t.Fatal("zero map value slot moved the hash")
	}
	if a, b := OfFields(map[string]any{"X": []v1{{1, [2]uint8{4, 0}}}}), OfFields(map[string]any{"X": []v2{{A: 1, P: [3]uint8{4}}}}); a != b {
		t.Fatal("appended zero field or array slot moved the hash")
	}
	base := OfFields(map[string]any{"X": []v2{{A: 1}}})
	for name, c := range map[string]v2{"new field": {A: 1, B: true}, "new slot": {A: 1, P: [3]uint8{0, 0, 1}},
		"moved value": {P: [3]uint8{1}}, "slice": {A: 1, S: []int{0}}} {
		if OfFields(map[string]any{"X": []v2{c}}) == base {
			t.Errorf("%s: same hash", name)
		}
	}
}
