package randx

import "testing"

// TestKeyedNeverChanges pins the stream: it is part of the simulation.
func TestKeyedNeverChanges(t *testing.T) {
	k := NewKeyed(1, 2, 3, 4, 5)
	got := [3]uint64{k.Uint64(), k.Uint64(), k.Uint64()}
	t.Logf("%#x", got)
	if got != keyedPin {
		t.Fatalf("Keyed(1,2,3,4,5) = %#x, want %#x", got, keyedPin)
	}
}

var keyedPin = [3]uint64{0xbe60973550b0a242, 0xe3b8641321729384, 0xe25e44702f2f86d6}

func TestKeyedDependsOnKeyAndCountOnly(t *testing.T) {
	a, b := NewKeyed(7, 100, 1, 42, 0), NewKeyed(7, 100, 1, 42, 0)
	other := NewKeyed(7, 100, 1, 43, 0)
	for range 10 {
		other.Uint64() // another key's draws never move this one
	}
	for i := range 100 {
		if x, y := a.Uint64(), b.Uint64(); x != y {
			t.Fatalf("draw %d: %#x vs %#x for one key", i, x, y)
		}
	}
	fork := a // a copy goes on from the same place
	if fork.Uint64() != a.Uint64() {
		t.Fatal("a copied stream diverged")
	}
	seen := map[uint64]bool{}
	for _, k := range []Keyed{NewKeyed(0, 0, 0, 0, 0), NewKeyed(1, 0, 0, 0, 0), NewKeyed(0, 1, 0, 0, 0),
		NewKeyed(0, 0, 1, 0, 0), NewKeyed(0, 0, 0, 1, 0), NewKeyed(0, 0, 0, 0, 1), NewKeyed(0, 0, 0, 1, 1)} {
		v := k.Uint64()
		if seen[v] {
			t.Fatalf("two keys share a first draw %#x", v)
		}
		seen[v] = true
	}
}

func TestKeyedIntNUniformAndInRange(t *testing.T) {
	k := NewKeyed(3, 0, 0, 0, 0)
	var counts [6]int
	for range 60000 {
		v := k.IntN(6)
		if v < 0 || v >= 6 {
			t.Fatalf("IntN(6) = %d", v)
		}
		counts[v]++
	}
	for i, c := range counts {
		if c < 9500 || c > 10500 {
			t.Fatalf("value %d drawn %d times in 60000, want about 10000", i, c)
		}
	}
	for range 1000 {
		if f := k.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 = %v", f)
		}
	}
	if n := testing.AllocsPerRun(100, func() { k.IntN(1000); k.Float64() }); n != 0 {
		t.Fatalf("%v allocations a draw", n)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("IntN(0) did not panic")
		}
	}()
	k.IntN(0)
}
