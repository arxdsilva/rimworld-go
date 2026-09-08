package schedule

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestDefaultColonyBlocks(t *testing.T) {
	t.Parallel()
	g := DefaultColony()
	cases := []struct {
		hour int
		want Activity
	}{
		{0, Sleep},
		{5, Sleep},
		{6, Anything},
		{7, Anything},
		{8, Work},
		{16, Work},
		{17, Recreation},
		{19, Recreation},
		{20, Sleep},
		{23, Sleep},
	}
	for _, tc := range cases {
		if got := g.At(tc.hour); got != tc.want {
			t.Fatalf("hour %d: %s, want %s", tc.hour, got, tc.want)
		}
	}
}

func TestAtOutOfRangeIsSleep(t *testing.T) {
	t.Parallel()
	g := DefaultColony()
	if g.At(-1) != Sleep || g.At(clock.HoursPerDay) != Sleep {
		t.Fatal("out-of-range hours should be sleep")
	}
}
