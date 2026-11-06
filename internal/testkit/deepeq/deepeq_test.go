package deepeq

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
)

type plainRow struct {
	X, Y  int
	Name  string
	F     float64
	On    bool
	Cells [3]uint8
}

type hidden struct {
	n    int
	rows []plainRow
	tags map[string]int
}

type tree struct {
	Rows   []plainRow
	Ptrs   []*plainRow
	Nested [][]int
	ByName map[string]plainRow
	Any    any
	Next   *tree
	Inner  hidden
	Fn     func()
	Arr    [2][]int
}

// TestEqualMatchesDeepEqual: Equal and reflect.DeepEqual agree on hand-made
// edge cases (nil versus empty, NaN, same backing array, unexported fields,
// interfaces of different types, funcs, cycles).
func TestEqualMatchesDeepEqual(t *testing.T) {
	t.Parallel()
	nan := math.NaN()
	rows := make([]plainRow, 200)
	for i := range rows {
		rows[i] = plainRow{X: i, Name: "r", Cells: [3]uint8{uint8(i)}}
	}
	other := append([]plainRow(nil), rows...)
	other[150].Cells[2] = 9
	cyc1, cyc2 := &tree{}, &tree{}
	cyc1.Next, cyc2.Next = cyc1, cyc2
	cyc3 := &tree{Rows: rows}
	cyc3.Next = cyc3
	f := func() {}
	cases := []struct {
		name string
		a, b any
	}{
		{"nil nil", nil, nil},
		{"nil value", nil, 1},
		{"types", int32(1), int64(1)},
		{"ints", 3, 3},
		{"nan", nan, nan},
		{"nan in struct", plainRow{F: nan}, plainRow{F: nan}},
		{"nan in slice", []float64{nan}, []float64{nan}},
		{"same nan slice", rows[:0], rows[:0]},
		{"nil vs empty slice", []int(nil), []int{}},
		{"nil vs empty map", map[int]int(nil), map[int]int{}},
		{"long plain slices", rows, append([]plainRow(nil), rows...)},
		{"long plain slices differ late", rows, other},
		{"prefix of one array", rows[:100], rows[:101]},
		{"plain struct", plainRow{X: 1, Name: "a"}, plainRow{X: 1, Name: "a"}},
		{"plain struct differs", plainRow{X: 1, Name: "a"}, plainRow{X: 1, Name: "b"}},
		{"arrays", [4]int{1, 2}, [4]int{1, 2}},
		{"unexported", hidden{n: 1, rows: rows}, hidden{n: 1, rows: other}},
		{"unexported equal", hidden{n: 1, rows: rows, tags: map[string]int{"a": 1}}, hidden{n: 1, rows: append([]plainRow(nil), rows...), tags: map[string]int{"a": 1}}},
		{"pointers deep", &plainRow{X: 2}, &plainRow{X: 2}},
		{"pointer slices", tree{Ptrs: []*plainRow{{X: 1}, nil}}, tree{Ptrs: []*plainRow{{X: 1}, nil}}},
		{"pointer nil vs set", tree{Ptrs: []*plainRow{nil}}, tree{Ptrs: []*plainRow{{}}}},
		{"nested slices", tree{Nested: [][]int{{1}, {2, 3}}}, tree{Nested: [][]int{{1}, {2, 3}}}},
		{"nested nil inner", tree{Nested: [][]int{nil}}, tree{Nested: [][]int{{}}}},
		{"maps", tree{ByName: map[string]plainRow{"a": {X: 1}}}, tree{ByName: map[string]plainRow{"a": {X: 1}}}},
		{"maps differ key", tree{ByName: map[string]plainRow{"a": {}}}, tree{ByName: map[string]plainRow{"b": {}}}},
		{"any types", tree{Any: 1}, tree{Any: int64(1)}},
		{"any equal", tree{Any: []int{1}}, tree{Any: []int{1}}},
		{"any nil", tree{Any: nil}, tree{Any: []int(nil)}},
		{"funcs nil", tree{}, tree{}},
		{"funcs set", tree{Fn: f}, tree{Fn: f}},
		{"array of slices", tree{Arr: [2][]int{{1}, nil}}, tree{Arr: [2][]int{{1}, {}}}},
		{"cycles", cyc1, cyc2},
		{"cycle vs value", cyc3, &tree{Rows: rows, Next: &tree{}}},
	}
	for _, c := range cases {
		if got, want := Equal(c.a, c.b), reflect.DeepEqual(c.a, c.b); got != want {
			t.Errorf("%s: Equal %v, DeepEqual %v", c.name, got, want)
		}
	}
}

// TestEqualMatchesDeepEqualRandom: random trees and single-field mutations
// of them get the same answer from both.
func TestEqualMatchesDeepEqualRandom(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	gen := func() *tree {
		tr := &tree{}
		n := r.IntN(150)
		if r.IntN(4) > 0 {
			tr.Rows = make([]plainRow, n)
		}
		for i := range tr.Rows {
			tr.Rows[i] = plainRow{X: r.IntN(3), Name: string(rune('a' + r.IntN(2))), Cells: [3]uint8{uint8(r.IntN(2))}}
		}
		for range r.IntN(3) {
			tr.Nested = append(tr.Nested, make([]int, r.IntN(3)))
		}
		if r.IntN(2) == 0 {
			tr.ByName = map[string]plainRow{"k": {X: r.IntN(2)}}
		}
		tr.Inner = hidden{n: r.IntN(2), rows: append([]plainRow(nil), tr.Rows...)}
		if r.IntN(3) == 0 {
			tr.Any = r.IntN(2)
		}
		return tr
	}
	clone := func(tr *tree) *tree {
		c := *tr
		if tr.Rows != nil {
			c.Rows = append([]plainRow{}, tr.Rows...)
		}
		c.Nested = nil
		for _, s := range tr.Nested {
			c.Nested = append(c.Nested, append([]int{}, s...))
		}
		if tr.ByName != nil {
			c.ByName = map[string]plainRow{}
			for k, v := range tr.ByName {
				c.ByName[k] = v
			}
		}
		c.Inner.rows = append([]plainRow(nil), tr.Inner.rows...)
		return &c
	}
	cmp := New() // one Comparer across all, as a test loop keeps it
	for i := range 3000 {
		a := gen()
		b := clone(a)
		switch r.IntN(6) {
		case 0:
			if len(b.Rows) > 0 {
				b.Rows[r.IntN(len(b.Rows))].X += r.IntN(2)
			}
		case 1:
			if len(b.Inner.rows) > 0 {
				b.Inner.rows[r.IntN(len(b.Inner.rows))].Cells[0] ^= uint8(r.IntN(2))
			}
		case 2:
			if b.ByName != nil {
				b.ByName["k"] = plainRow{X: r.IntN(2)}
			}
		case 3:
			if len(b.Nested) > 0 && r.IntN(2) == 0 {
				b.Nested[0] = nil
			}
		case 4:
			b = gen()
		}
		if got, want := cmp.Equal(a, b), reflect.DeepEqual(a, b); got != want || Equal(a, b) != want {
			t.Fatalf("case %d: Equal %v, DeepEqual %v\n%+v\n%+v", i, got, want, a, b)
		}
	}
}

// TestEqualChunks: plain slices of every length up to a few chunks, equal
// and differing at each position, the slices sharing no memory.
func TestEqualChunks(t *testing.T) {
	t.Parallel()
	cmp := New()
	for n := range 4200 {
		if n > 300 && n%97 != 0 {
			continue
		}
		a, b := make([]uint8, n), make([]uint8, n)
		x, y := make([]int, n), make([]int, n)
		if !cmp.Equal(a, b) || !cmp.Equal(x, y) {
			t.Fatalf("n %d: equal slices differ", n)
		}
		for i := 0; i < n; i += max(1, n/40) {
			b[i], y[i] = 1, 1
			if cmp.Equal(a, b) || cmp.Equal(x, y) {
				t.Fatalf("n %d: a difference at %d missed", n, i)
			}
			b[i], y[i] = 0, 0
		}
	}
}
