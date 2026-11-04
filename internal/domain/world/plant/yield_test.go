package plant

import "testing"

// TestHarvestYieldTable pins the wiki's table and its capacity factors.
func TestHarvestYieldTable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ level, manip, sight, want int }{
		{0, 100, 100, 600},
		{7, 100, 100, 975},
		{8, 100, 100, 1000},
		{9, 100, 100, 1010},
		{18, 100, 100, 1100},
		{19, 100, 100, 1120},
		{20, 100, 100, 1130},
		{-1, 100, 100, 600},
		{25, 100, 100, 1130},
		{8, 50, 100, 850},   // Manipulation at 30 % weight: 100 - 15
		{8, 150, 100, 1150}, // it raises the yield as well
		{8, 100, 50, 900},   // Sight at 20 % weight: 100 - 10
		{8, 100, 150, 1000}, // good sight never raises it
		{20, 300, 100, 1500},
		{0, 0, 0, 336}, // 60 % x 70 % x 80 %
	} {
		if got := HarvestYieldPermil(c.level, c.manip, c.sight); got != c.want {
			t.Errorf("level %d manip %d sight %d: %d, want %d", c.level, c.manip, c.sight, got, c.want)
		}
	}
}

func TestPlantWorkSpeed(t *testing.T) {
	t.Parallel()
	for level, want := range map[int]int{0: 100, 1: 195, 7: 885, 8: 1000, 20: 2380} {
		if got := WorkSpeedPermil(level); got != want {
			t.Errorf("level %d: %d, want %d", level, got, want)
		}
	}
}

// TestYieldAmount: a bonus above 100 %, no smaller amount below it.
func TestYieldAmount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ qty, yield, want int }{
		{20, 600, 20}, {20, 1000, 20}, {20, 1130, 22}, {100, 1500, 150}, {1, 1130, 1}, {0, 1130, 0}, {-1, 900, 0},
	} {
		if got := YieldAmount(c.qty, c.yield); got != c.want {
			t.Errorf("YieldAmount(%d, %d) = %d, want %d", c.qty, c.yield, got, c.want)
		}
	}
}

func TestWasteChance(t *testing.T) {
	t.Parallel()
	for yield, want := range map[int]int{600: 400, 975: 25, 1000: 0, 1130: 0} {
		if got := WasteChancePermil(yield); got != want {
			t.Errorf("yield %d: %d, want %d", yield, got, want)
		}
	}
}
