package temperature

import "testing"

// TestWallLossMatchesWikiTable: one interval at a -9555 C difference loses
// what the wiki measured for square rooms, single and double walls.
func TestWallLossMatchesWikiTable(t *testing.T) {
	const diff = Milli(-9555000)
	for _, c := range []struct {
		n              int
		single, double float64 // wiki: Temperature, degrees per 120 ticks
	}{{1, -161.47, -81.23}, {2, -81.24, -40.62}, {3, -54.16, -27.08}, {5, -32.49, -16.25}, {7, -23.21, -11.61}} {
		one := Exposure{Cells: c.n * c.n, Single: 4 * c.n}.Loss(diff)
		two := Exposure{Cells: c.n * c.n, Double: 4 * c.n}.Loss(diff)
		for _, k := range []struct {
			got  Milli
			want float64
		}{{one, c.single}, {two, c.double}} {
			if g := float64(k.got) / 1000; g > k.want*0.99 || g < k.want*1.01 {
				t.Errorf("%dx%d: lost %.2f, wiki %.2f", c.n, c.n, g, k.want)
			}
		}
	}
}

// TestThinRoofLossIsSizeFree: a thin roof loses -57.331 at -9555 C for any
// room size; overhead mountain exchanges nothing.
func TestThinRoofLossIsSizeFree(t *testing.T) {
	for _, n := range []int{1, 4, 9, 30} {
		got := Exposure{Cells: n * n, Thin: n * n}.Equalise(0, -9555000)
		if g := float64(got) / 1000; g > -57.0 || g < -57.7 {
			t.Errorf("%dx%d thin roof: %.3f, wiki -57.331", n, n, g)
		}
		if got := (Exposure{Cells: n * n, Thick: n * n}).Equalise(5000, -9555000); got != 5000 {
			t.Errorf("%dx%d mountain room moved to %d with the outside", n, n, got)
		}
	}
}

// TestMountainCoolsOnlyWhenWarm: overhead rock cools a room above 15 C a
// little and leaves a colder one alone.
func TestMountainCoolsOnlyWhenWarm(t *testing.T) {
	e := Exposure{Cells: 25, Thick: 25}
	if got := e.Equalise(30000, 30000); got >= 30000 || got < 29990 {
		t.Fatalf("a 30 C mountain room went to %d, want a tiny drop", got)
	}
	if got := e.Equalise(10000, 30000); got != 10000 {
		t.Fatalf("a 10 C mountain room moved to %d", got)
	}
}

// TestQuarterUnroofedTakesTheOutside: at 25 % unroofed a room is the
// outdoor air; below it the open cells only speed the exchange.
func TestQuarterUnroofedTakesTheOutside(t *testing.T) {
	if got := (Exposure{Cells: 100, Single: 40, Thin: 75, Open: 25}).Equalise(20000, -5000); got != -5000 {
		t.Fatalf("25 %% unroofed: %d, want the outside", got)
	}
	closed := Exposure{Cells: 100, Single: 40, Thin: 100}.Equalise(20000, -5000)
	leaky := Exposure{Cells: 100, Single: 40, Thin: 76, Open: 24}.Equalise(20000, -5000)
	if !(leaky < closed && leaky > -5000) {
		t.Fatalf("24 %% unroofed %d, roofed %d", leaky, closed)
	}
}

func TestMilliRounds(t *testing.T) {
	for in, want := range map[Milli]Temp{0: 0, 49: 0, 50: 1, -50: -1, -49: 0, 21549: 215, -21551: -216} {
		if got := in.Temp(); got != want {
			t.Errorf("%d -> %d, want %d", in, got, want)
		}
	}
	if Temp(-35).Milli() != -3500 {
		t.Error("Temp.Milli")
	}
}
