package randx

import "testing"

func TestSameSeedProducesSameSequence(t *testing.T) {
	a := New(42)
	b := New(42)

	for i := 0; i < 20; i++ {
		x, y := a.IntN(1000), b.IntN(1000)
		if x != y {
			t.Fatalf("sequence diverged at index %d: %d != %d", i, x, y)
		}
	}
}

func TestDifferentSeedsCanDiffer(t *testing.T) {
	a := New(1)
	b := New(2)

	same := true
	for i := 0; i < 20; i++ {
		if a.IntN(1_000_000) != b.IntN(1_000_000) {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced an identical sequence across 20 draws")
	}
}

func TestIntNStaysInRange(t *testing.T) {
	r := New(7)
	for i := 0; i < 100; i++ {
		if v := r.IntN(5); v < 0 || v >= 5 {
			t.Fatalf("IntN(5) = %d, out of range", v)
		}
	}
}
