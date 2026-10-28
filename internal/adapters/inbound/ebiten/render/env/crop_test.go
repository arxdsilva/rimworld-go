package env

import (
	"fmt"
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// TestCropUnitsDeterministic: a tile's units are a pure function of the
// tile; tiles differ.
func TestCropUnitsDeterministic(t *testing.T) {
	t.Parallel()
	same := 0
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			a := CropUnits(x, y, 4, false, nil)
			b := CropUnits(x, y, 4, false, nil)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("tile (%d,%d) unit %d differs between calls: %+v vs %+v", x, y, i, a[i], b[i])
				}
			}
			if o := CropUnits(x+1, y, 4, false, nil); o[0] == a[0] {
				same++
			}
		}
	}
	if same > 4 {
		t.Fatalf("%d of 144 neighbouring tiles share their first unit exactly", same)
	}
}

// TestCropUnitsRowAlignment: on an east-west field every unit of a
// species sits on one of the pattern's rows (across jitter), so a field
// reads in rows; a north-south field turns the rows on their side.
func TestCropUnitsRowAlignment(t *testing.T) {
	t.Parallel()
	check := func(n int, vertical bool, rows []float64) {
		t.Helper()
		var buf []CropUnit
		hit := make([]int, len(rows))
		for y := 0; y < 30; y++ {
			for x := 0; x < 30; x++ {
				buf = CropUnits(x, y, n, vertical, buf[:0])
				for _, u := range buf {
					across, along := u.Dy, u.Dx
					if vertical {
						across, along = u.Dx, u.Dy
					}
					found := false
					for i, r := range rows {
						if math.Abs(across-r) <= CropJitterAcross+1e-9 {
							found = true
							hit[i]++
						}
					}
					if !found {
						t.Fatalf("n=%d vertical=%v tile (%d,%d): unit across=%.3f is off every row %v", n, vertical, x, y, across, rows)
					}
					if along < -0.2 || along > 1.2 {
						t.Fatalf("n=%d: unit along=%.3f strays from its tile", n, along)
					}
				}
			}
		}
		for i, h := range hit {
			if h == 0 {
				t.Errorf("n=%d vertical=%v: row %d (v=%.2f) never used", n, vertical, i, rows[i])
			}
		}
	}
	check(2, false, []float64{0.5})
	check(2, true, []float64{0.5})
	check(4, false, []float64{0.3, 0.76})
	check(4, true, []float64{0.3, 0.76})
	check(3, false, []float64{0.3, 0.78})
	check(1, false, []float64{0.55})
	// the along-row spread: wheat's four units never pile on one spot
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			us := CropUnits(x, y, 4, false, nil)
			for i := range us {
				for j := i + 1; j < len(us); j++ {
					if math.Hypot(us[i].Dx-us[j].Dx, us[i].Dy-us[j].Dy) < 0.12 {
						t.Fatalf("tile (%d,%d): units %d and %d overlap", x, y, i, j)
					}
				}
			}
		}
	}
}

// TestCropUnitsMirrorVariantTrim: units mirror at random, pick all four
// variants and trim their size within 0.9–1.1; sway share and phase vary.
func TestCropUnitsMirrorVariantTrim(t *testing.T) {
	t.Parallel()
	mirrored, plain := 0, 0
	variants := map[uint32]bool{}
	phases := map[float64]bool{}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			for _, u := range CropUnits(x, y, 4, false, nil) {
				if u.Mirror {
					mirrored++
				} else {
					plain++
				}
				variants[u.Variant%CropVariants] = true
				phases[u.Phase] = true
				if u.Trim < 0.9-1e-9 || u.Trim > 1.1+1e-9 {
					t.Fatalf("trim %.3f outside 0.9–1.1", u.Trim)
				}
				if u.Sway < 0.8-1e-9 || u.Sway > 1.2+1e-9 {
					t.Fatalf("sway share %.3f outside 0.8–1.2", u.Sway)
				}
			}
		}
	}
	if mirrored < 500 || plain < 500 {
		t.Fatalf("mirroring not mixed: %d mirrored, %d plain", mirrored, plain)
	}
	if len(variants) != CropVariants {
		t.Fatalf("variants used %v, want all %d", variants, CropVariants)
	}
	if len(phases) < 1000 {
		t.Fatalf("only %d distinct sway phases in 1600 units", len(phases))
	}
}

// TestCropUnitRanks: a tile's drop-out ranks are a permutation of 0..n-1.
func TestCropUnitRanks(t *testing.T) {
	t.Parallel()
	first := map[int]int{}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			for n := 1; n <= 4; n++ {
				seen := make([]bool, n)
				for _, u := range CropUnits(x, y, n, false, nil) {
					if u.Rank < 0 || u.Rank >= n || seen[u.Rank] {
						t.Fatalf("tile (%d,%d) n=%d: bad rank %d", x, y, n, u.Rank)
					}
					seen[u.Rank] = true
				}
			}
			for i, u := range CropUnits(x, y, 4, false, nil) {
				if u.Rank == 0 {
					first[i]++
				}
			}
		}
	}
	if len(first) < 4 {
		t.Fatalf("the first unit to go is always the same slot: %v", first)
	}
}

func TestCropStageFollowsDamage(t *testing.T) {
	t.Parallel()
	base := game.PlantView{Species: "corn", Growth: 100, Stage: 3}
	for _, c := range []struct {
		damage int
		want   int
	}{{0, StHarvestable}, {25, StHarvestable}, {49, StHarvestable}, {50, StWilted}, {80, StWilted}, {100, StWilted}} {
		p := base
		p.Damage = c.damage
		if got := CropStage(p); got != c.want {
			t.Errorf("damage %d: stage %d, want %d", c.damage, got, c.want)
		}
	}
	// a crop with the plain wilt flag but no damage keeps its normal look
	if got := CropStage(game.PlantView{Species: "corn", Growth: 60, Stage: 2, Wilted: true}); got != StMature {
		t.Errorf("Wilted flag alone: stage %d, want mature", got)
	}
	if got := CropStage(game.PlantView{Species: "corn", Growth: 3, Stage: 0}); got != StSown {
		t.Errorf("fresh sowing: stage %d", got)
	}
	if got := CropStage(game.PlantView{Species: "corn", Growth: 9, Stage: 0}); got != StSprout {
		t.Errorf("sprouting: stage %d", got)
	}
}

// TestCropDamageTint: below 50% damage the sprite is the normal one with
// a rime (cold: cooler, lighter) or scorch (heat: browner, darker blue)
// tint that grows with the damage.
func TestCropDamageTint(t *testing.T) {
	t.Parallel()
	if cs := CropDamageTint(0, "cold"); cs != sprite.NoCS {
		t.Errorf("no damage tints %v", cs)
	}
	prevB := float32(1)
	prevScorchB := float32(1)
	for d := 5; d < CropWiltDamage; d += 5 {
		cold, heat := CropDamageTint(d, "cold"), CropDamageTint(d, "heat")
		if cold[2] <= prevB {
			t.Errorf("cold tint's blue %.3f at %d%% does not grow (was %.3f)", cold[2], d, prevB)
		}
		if heat[2] >= prevScorchB {
			t.Errorf("scorch blue %.3f at %d%% does not fall (was %.3f)", heat[2], d, prevScorchB)
		}
		if cold[2] <= cold[0] {
			t.Errorf("rime %v is not blue-white at %d%%", cold, d)
		}
		if heat[0] <= heat[2] || heat[1] <= heat[2] {
			t.Errorf("scorch %v is not brown-yellow at %d%%", heat, d)
		}
		prevB, prevScorchB = cold[2], heat[2]
	}
	if CropDamageTint(40, "cold") == CropDamageTint(40, "heat") {
		t.Error("cold and heat tint alike")
	}
	if a := CropDamageTint(40, "cold")[3]; a != 1 {
		t.Errorf("tint alpha %v", a)
	}
}

// TestCropUnitLook: from 50% the units shrink towards 60% and, over the
// last 30%, fade out one by one, each in its own band, so a tile thins
// out; at 100% nothing is left.
func TestCropUnitLook(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 4} {
		for rank := 0; rank < n; rank++ {
			if s, a := CropUnitLook(0, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d undamaged: scale %v alpha %v", n, rank, s, a)
			}
			if s, a := CropUnitLook(49, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d at 49%%: scale %v alpha %v", n, rank, s, a)
			}
			if s, a := CropUnitLook(50, rank, n); s != 1 || a != 1 {
				t.Errorf("n=%d rank %d at 50%%: scale %v alpha %v", n, rank, s, a)
			}
			if s, _ := CropUnitLook(100, rank, n); math.Abs(s-CropMinShrink) > 1e-9 {
				t.Errorf("n=%d rank %d at 100%%: scale %v, want %v", n, rank, s, CropMinShrink)
			}
			if _, a := CropUnitLook(100, rank, n); a != 0 {
				t.Errorf("n=%d rank %d still drawn at 100%%: alpha %v", n, rank, a)
			}
			if _, a := CropUnitLook(70, rank, n); a < 1-1e-9 && rank > 0 {
				t.Errorf("n=%d rank %d fades before its band: %v", n, rank, a)
			}
		}
		// shrink is monotone and continuous
		prev := 1.0
		for d := 50; d <= 100; d++ {
			s, _ := CropUnitLook(d, 0, n)
			if s > prev+1e-9 || prev-s > 0.01 {
				t.Errorf("n=%d damage %d: shrink %.4f after %.4f", n, d, s, prev)
			}
			prev = s
		}
		// units go one by one, rank 0 first, each fully gone before the next
		// starts to fade, and the tile is empty at 100
		goneAt := make([]int, n)
		for rank := 0; rank < n; rank++ {
			goneAt[rank] = -1
			for d := 50; d <= 100; d++ {
				if _, a := CropUnitLook(d, rank, n); a == 0 {
					goneAt[rank] = d
					break
				}
			}
			if goneAt[rank] < 0 {
				t.Errorf("n=%d rank %d never disappears", n, rank)
			}
			if rank > 0 && goneAt[rank] <= goneAt[rank-1] && n > 1 {
				t.Errorf("n=%d: rank %d gone at %d, not after rank %d at %d", n, rank, goneAt[rank], rank-1, goneAt[rank-1])
			}
		}
		if goneAt[n-1] != 100 {
			t.Errorf("n=%d: last unit gone at %d, want 100", n, goneAt[n-1])
		}
	}
}

// TestPlantDamageLookOnOtherPlants: trees and bushes shrink, tint and
// fade with damage too; undamaged plants draw as before.
func TestPlantDamageLookOnOtherPlants(t *testing.T) {
	t.Parallel()
	cs, k := PlantDamageLook(game.PlantView{Species: "tree"})
	if cs != sprite.NoCS || k != 1 {
		t.Fatalf("undamaged: %v %v", cs, k)
	}
	cs, k = PlantDamageLook(game.PlantView{Species: "tree", Damage: 30, DamageKind: "cold"})
	if cs == sprite.NoCS || k != 1 {
		t.Fatalf("30%%: tint %v scale %v", cs, k)
	}
	cs, k = PlantDamageLook(game.PlantView{Species: "berry_bush", Damage: 85, DamageKind: "heat"})
	if k >= 1 || cs[3] >= 1 {
		t.Fatalf("85%%: scale %v alpha %v", k, cs[3])
	}
}

func TestCropIsCropPlant(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p    game.PlantView
		want bool
	}{
		{game.PlantView{Species: "corn"}, true},
		{game.PlantView{Species: "healroot", Zone: "healroot"}, true},
		{game.PlantView{Species: "healroot"}, false}, // wild
		{game.PlantView{Species: "tree"}, false},
		{game.PlantView{Species: "berry_bush"}, false},
	} {
		if got := IsCropPlant(&c.p); got != c.want {
			t.Errorf("%+v: %v, want %v", c.p, got, c.want)
		}
	}
}

// PLAY-016: a dying plant's shadow shrinks with its units and is gone when
// they are.
func TestDamageShadowFactor(t *testing.T) {
	t.Parallel()
	corn := game.PlantView{Species: "corn", Stage: 2}
	if DamageShadowFactor(corn) != 1 {
		t.Fatal("healthy: full shadow")
	}
	prev := 1.0
	for _, d := range []int{30, 60, 80, 90, 99} {
		corn.Damage = d
		f := DamageShadowFactor(corn)
		if f > prev || f < 0 {
			t.Fatalf("damage %d: factor %v after %v", d, f, prev)
		}
		prev = f
	}
	corn.Damage = 100
	if DamageShadowFactor(corn) != 0 {
		t.Fatal("dead: no shadow")
	}
	if f := DamageShadowFactor(game.PlantView{Species: "tree", Damage: 85}); f <= 0 || f >= 1 {
		t.Fatalf("a dying tree's shadow: %v", f)
	}
}

// TestCropFitKeepsUnitInTile: CropFit keeps a unit inside its tile, leaves
// one that fits where it is, moves one that does not before shrinking it,
// and shrinks it only until its top meets the north edge.
func TestCropFitKeepsUnitInTile(t *testing.T) {
	t.Parallel()
	const eps = 1e-9
	for _, dy := range []float64{-0.1, 0, 0.22, 0.3, 0.5, 0.76, 0.83} {
		for _, above := range []float64{0, 0.05, 0.2, 0.45, 0.7, 0.9, 0.95, 1.2, 1.6} {
			for _, below := range []float64{0, 0.02, 0.1, 0.3, 0.6} {
				root, f := CropFit(dy, above, below)
				top, bot := root-above*f, root+below*f
				where := fmt.Sprintf("dy %.2f above %.2f below %.2f: root %.3f size %.3f", dy, above, below, root, f)
				if top < -eps || bot > 1+eps {
					t.Errorf("%s: spans %.3f..%.3f, off its tile", where, top, bot)
				}
				if f <= 0 || f > 1+eps {
					t.Errorf("%s: size factor out of (0, 1]", where)
				}
				if dy-above >= 0 && dy+below <= 1 && dy <= CropRootMax && (root != dy || f != 1) {
					t.Errorf("%s: a unit that fits was moved or shrunk", where)
				}
				if above <= CropRootMax && above+below <= 1 && f != 1 {
					t.Errorf("%s: shrunk although moving it was enough", where)
				}
				if root > dy+eps && root > CropRootMax+eps {
					t.Errorf("%s: moved south past %.2f", where, CropRootMax)
				}
				if f < 1-eps && math.Abs(top) > 1e-9 {
					t.Errorf("%s: shrunk more than needed (top at %.3f)", where, top)
				}
			}
		}
	}
}
