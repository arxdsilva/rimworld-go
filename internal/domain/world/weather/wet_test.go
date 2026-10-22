package weather

import "testing"

func TestWetnessScalesRainOdds(t *testing.T) {
	for _, k := range []Kind{Clear, Cloudy, Rain, Storm, Snow, Blizzard} {
		for _, season := range []string{"Spring", "Summer", "Fall", "Winter"} {
			if RowWet(k, season, 100) != Row(k, season) {
				t.Fatalf("wet 100 must be the temperate row (%s, %s)", k, season)
			}
			dry, wet := RowWet(k, season, 20), RowWet(k, season, 160)
			base := Row(k, season)
			if dry[Rain] >= base[Rain] || wet[Rain] <= base[Rain] {
				t.Fatalf("%s %s: rain %v (dry) %v (base) %v (wet)", k, season, dry[Rain], base[Rain], wet[Rain])
			}
			sum := 0.0
			for _, p := range wet {
				sum += p
			}
			if sum < 0.999 || sum > 1.001 {
				t.Fatalf("row sums to %v", sum)
			}
		}
	}
	if r := RowWet(Rain, "Spring", 0); r[Rain] != 0 || r[Storm] != 0 {
		t.Fatalf("a rainless climate still rains: %v", r)
	}
}

func TestNextWetIsNextAtTemperate(t *testing.T) {
	var a, b State
	rainy, dry := 0, 0
	var w, d State
	for blk := uint64(0); blk < 400; blk++ {
		a, b = Next(5, blk, a, 8, "Spring"), NextWet(5, blk, b, 8, "Spring", 100)
		if a != b {
			t.Fatalf("block %d: NextWet(100) differs from Next", blk)
		}
		w, d = NextWet(5, blk, w, 8, "Spring", 160), NextWet(5, blk, d, 8, "Spring", 10)
		if w.Target.Falling() {
			rainy++
		}
		if d.Target.Falling() {
			dry++
		}
	}
	if rainy <= dry {
		t.Fatalf("wet climate rained %d blocks, dry one %d", rainy, dry)
	}
}
