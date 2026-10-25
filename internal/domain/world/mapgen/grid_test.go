package mapgen

import (
	"math"
	"testing"
)

func mask(rows ...string) (int, int, []bool) {
	w, h := len(rows[0]), len(rows)
	m := make([]bool, w*h)
	for y, r := range rows {
		for x, c := range r {
			m[y*w+x] = c == '#'
		}
	}
	return w, h, m
}

func TestComponents(t *testing.T) {
	w, h, m := mask(
		"##..#",
		"#...#",
		"..#..",
		"...##",
	)
	lab, sizes := Components(w, h, m, false)
	if len(sizes) != 4 || sizes[0] != 3 || sizes[1] != 2 || sizes[2] != 1 || sizes[3] != 2 {
		t.Fatalf("4-conn sizes %v", sizes)
	}
	if lab[2] != -1 || lab[0] != lab[5] {
		t.Fatalf("labels %v", lab)
	}
	if _, s8 := Components(w, h, m, true); len(s8) != 3 {
		t.Fatalf("8-conn sizes %v", s8)
	}
}

func TestChebDepth(t *testing.T) {
	w, h, m := mask(
		"#####",
		"#####",
		"#####",
		"..###",
	)
	d := ChebDepth(w, h, m)
	if d[3*w+0] != 0 || d[2*w+0] != 1 || d[2*w+2] != 1 || d[0] != 3 || d[4] != 3 || d[3*w+4] != 3 {
		t.Fatalf("depths %v", d)
	}
	_, _, all := mask("##", "##")
	for _, v := range ChebDepth(2, 2, all) {
		if v != 255 {
			t.Fatal("no open ground: every rock is 255")
		}
	}
}

func TestBFSDist(t *testing.T) {
	w, h, src := mask(
		"#....",
		".....",
	)
	_, _, wall := mask(
		"..#..",
		"..#..",
	)
	pass := make([]bool, len(wall))
	for i := range pass {
		pass[i] = !wall[i]
	}
	d := BFSDist(w, h, src, pass, false)
	if d[0] != 0 || d[1] != 1 || d[w+1] != 2 || d[3] != -1 {
		t.Fatalf("4-conn %v", d)
	}
	if d8 := BFSDist(w, h, src, nil, true); d8[w+1] != 1 || d8[4] != 4 {
		t.Fatalf("8-conn %v", d8)
	}
}

func TestQuantileAndRank(t *testing.T) {
	v := []float32{5, 1, 3, 2, 4}
	if Quantile(v, 0) != 1 || Quantile(v, 1) != 5 || Quantile(v, 0.5) != 3 || Quantile(nil, 0.5) != 0 {
		t.Fatal("quantile")
	}
	if v[0] != 5 {
		t.Fatal("Quantile must not reorder its input")
	}
	RankNormalise(v)
	if v[1] != 0 || v[0] != 1 || v[2] != 0.5 {
		t.Fatalf("rank %v", v)
	}
	one := []float32{7}
	if RankNormalise(one); one[0] != 0 {
		t.Fatal("single value ranks 0")
	}
	neg := []float32{0.5, -1, float32(math.Copysign(0, -1)), 0, -3}
	if RankNormalise(neg); neg[4] != 0 || neg[1] != 0.25 || neg[2] != 0.5 || neg[3] != 0.75 || neg[0] != 1 {
		t.Fatalf("negatives and signed zero %v", neg)
	}
	ties := []float32{2, 2, 2}
	if RankNormalise(ties); ties[0] != 0 || ties[2] != 1 {
		t.Fatalf("ties by index %v", ties)
	}
}

func TestSmoothstep(t *testing.T) {
	if Smoothstep(0, 1, -1) != 0 || Smoothstep(0, 1, 2) != 1 || Smoothstep(0, 1, 0.5) != 0.5 {
		t.Fatal("smoothstep")
	}
	if Smoothstep(1, 1, 0.5) != 0 || Smoothstep(1, 1, 1) != 1 {
		t.Fatal("degenerate smoothstep")
	}
}
