package item

import "testing"

func TestMergePoisonWeightsByQuantity(t *testing.T) {
	for _, c := range []struct {
		qa   int
		pa   int16
		qb   int
		pb   int16
		want int16
	}{{1, 40, 3, 20, 25}, {5, 0, 5, 0, 0}, {0, 0, 0, 0, 0}, {2, 10, 0, 99, 10}, {1, 1, 1, 2, 2}} {
		if got := MergePoison(c.qa, c.pa, c.qb, c.pb); got != c.want {
			t.Fatalf("MergePoison(%d×%d, %d×%d) = %d, want %d", c.qa, c.pa, c.qb, c.pb, got, c.want)
		}
	}
}
