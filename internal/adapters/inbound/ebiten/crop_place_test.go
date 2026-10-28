package ebiten

import (
	"fmt"
	"math"
	"slices"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-019: every crop unit stands on its own tile. The art puts a unit's
// root at the tile-centre point (plant_base.py CX, CY = 0.5, -0.5); a
// unit whose top would cross the tile's north edge moves south (its root
// no further than cropRootMax), then shrinks until it fits, and its foot
// never crosses the south edge.

// TestCropFitKeepsUnitInTile: cropFit keeps a unit inside its tile, leaves
// one that fits where it is, moves one that does not before shrinking it,
// and shrinks it only until its top meets the north edge.
func TestCropFitKeepsUnitInTile(t *testing.T) {
	t.Parallel()
	const eps = 1e-9
	for _, dy := range []float64{-0.1, 0, 0.22, 0.3, 0.5, 0.76, 0.83} {
		for _, above := range []float64{0, 0.05, 0.2, 0.45, 0.7, 0.9, 0.95, 1.2, 1.6} {
			for _, below := range []float64{0, 0.02, 0.1, 0.3, 0.6} {
				root, f := cropFit(dy, above, below)
				top, bot := root-above*f, root+below*f
				where := fmt.Sprintf("dy %.2f above %.2f below %.2f: root %.3f size %.3f", dy, above, below, root, f)
				if top < -eps || bot > 1+eps {
					t.Errorf("%s: spans %.3f..%.3f, off its tile", where, top, bot)
				}
				if f <= 0 || f > 1+eps {
					t.Errorf("%s: size factor out of (0, 1]", where)
				}
				if dy-above >= 0 && dy+below <= 1 && dy <= cropRootMax && (root != dy || f != 1) {
					t.Errorf("%s: a unit that fits was moved or shrunk", where)
				}
				if above <= cropRootMax && above+below <= 1 && f != 1 {
					t.Errorf("%s: shrunk although moving it was enough", where)
				}
				if root > dy+eps && root > cropRootMax+eps {
					t.Errorf("%s: moved south past %.2f", where, cropRootMax)
				}
				if f < 1-eps && math.Abs(top) > 1e-9 {
					t.Errorf("%s: shrunk more than needed (top at %.3f)", where, top)
				}
			}
		}
	}
}

// cropState is a crop's growth state: a growth percent, the sim's stage
// and the cold damage.
type cropState struct {
	name                  string
	growth, stage, damage int
}

func (s cropState) edit(p *game.PlantView) {
	p.Growth, p.Stage, p.Damage, p.DamageKind, p.Harvestable = s.growth, s.stage, s.damage, "cold", s.growth >= 100
}

// cropStates cover every drawn stage, each growth range from start to
// end, the ripe plant, the wilted one and a fading one.
var cropStates = []cropState{
	{"sown", 2, 0, 0}, {"sprout8", 8, 0, 0}, {"sprout14", 14, 0, 0}, {"sprout19", 19, 0, 0},
	{"young20", 20, 1, 0}, {"young37", 37, 1, 0}, {"young54", 54, 1, 0},
	{"mature55", 55, 2, 0}, {"mature77", 77, 2, 0}, {"mature99", 99, 2, 0},
	{"ripe", 100, 3, 0}, {"wilted", 100, 3, cropWiltDamage}, {"wilted-young", 40, 1, 60}, {"fading", 100, 3, 85},
}

// cropDrawEach draws every plant of r's stub world like cropDraw and hands
// each plant's queued unit quads to fn.
func cropDrawEach(r *Renderer, mw, mh int, tile float64, fn func(p *game.PlantView, quads []ebiten.Vertex)) {
	r.cam.zoom, r.cam.offsetX, r.cam.offsetY = tile/TileSize, -tile, -tile
	sw, sh := int(float64(mw+2)*tile), int(float64(mh+2)*tile)
	r.setView(sw, sh, tile)
	w := r.game.World()
	r.view.W = w
	r.stepEnvironment(w, nil)
	dst := ebiten.NewImage(sw, sh)
	defer dst.Deallocate()
	r.lod.crops.begin()
	for i := range w.Plants {
		n0, f0 := len(r.lod.crops.verts), r.lod.crops.flushes
		r.drawCropObj(dst, &w, &w.Plants[i], tile)
		if r.lod.crops.flushes != f0 { // the batch was drawn first: this plant's units start it
			n0 = 0
		}
		fn(&w.Plants[i], r.lod.crops.verts[n0:])
	}
}

// cropStateFields lays every state of cropStates out as a field of one
// crop: east-west fields (6×3, no zone) stacked down the map, or
// north-south ones (3×6, each its grow zone) side by side.
func cropStateFields(species string, vertical bool) (w game.WorldView, mw, mh int) {
	var ps []game.PlantView
	var zones []game.ZoneView
	for i, s := range cropStates {
		x0, y0, fw, fh := 1, 1+4*i, 6, 3
		if vertical {
			x0, y0, fw, fh = 1+4*i, 1, 3, 6
		}
		field := cropPlants(species, x0, y0, fw, fh, s.edit)
		ps = append(ps, field...)
		if vertical {
			z := game.ZoneView{ID: uint16(i + 1), Kind: game.ZoneGrow, Crop: species, MinX: x0, MinY: y0, MaxX: x0 + fw - 1, MaxY: y0 + fh - 1}
			for _, p := range field {
				z.Cells = append(z.Cells, [2]int{p.X, p.Y})
			}
			zones = append(zones, z)
		}
	}
	mw, mh = 8, 4*len(cropStates)+1
	if vertical {
		mw, mh = mh, mw
	}
	return cropWorld(mw, mh, ps, zones...), mw, mh
}

// cropNames is every crop species the sim grows.
func cropNames() []string {
	var out []string
	for _, c := range game.Crops() {
		out = append(out, c.Name)
	}
	return out
}

// TestCropUnitsStayOnTheirTile: with the real art, every unit of every
// crop, at every growth stage, on east-west and north-south fields (every
// unit pattern), live at zoom 1 and as the one representative unit of
// the chunk tier, calm or in a gale that stretches or squashes it, draws
// inside its own tile: its top on or below the tile's north edge, its
// bottom on or above the south edge.
func TestCropUnitsStayOnTheirTile(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	const eps = 0.5 // px
	winds := []struct {
		name   string
		wx, wy float64
	}{{"calm", 0, 0}, {"gale-north", 3, -6}, {"gale-south", -3, 6}}
	bad := 0
	for _, sp := range cropNames() {
		for _, vertical := range []bool{false, true} {
			w, mw, mh := cropStateFields(sp, vertical)
			for _, wind := range winds {
				w.WindX, w.WindY, w.Weather = wind.wx, wind.wy, "cloudy"
				for _, tile := range []float64{TileSize, 32} {
					for _, present := range []uint64{0, 37} {
						sim := &stubSim{w: mw, h: mh, world: w}
						r := New(sim)
						r.art, r.speed, r.present = art, speedPaused, present
						quads := 0
						cropDrawEach(r, mw, mh, tile, func(p *game.PlantView, vs []ebiten.Vertex) {
							_, ty := r.worldToScreenF(float64(p.X), float64(p.Y))
							for q := 0; q+3 < len(vs); q += 4 {
								quads++
								top := min(vs[q].DstY, vs[q+1].DstY)
								bot := max(vs[q+2].DstY, vs[q+3].DstY)
								if top >= ty-eps && bot <= ty+float32(tile)+eps {
									continue
								}
								if bad++; bad <= 30 {
									state := cropStates[(p.Y-1)/4].name
									if vertical {
										state = cropStates[(p.X-1)/4].name
									}
									t.Errorf("%s %s vertical=%v %s tile %.0f px t=%d at (%d,%d): unit spans %.3f..%.3f tiles of its tile",
										sp, state, vertical, wind.name, tile, present, p.X, p.Y, float64(top-ty)/tile, float64(bot-ty)/tile)
								}
							}
						})
						if quads == 0 {
							t.Fatalf("%s vertical=%v: no units drawn", sp, vertical)
						}
					}
				}
			}
		}
	}
	if bad > 30 {
		t.Errorf("... %d units off their tile in all", bad)
	}
}

// TestCropGrowthRisesStageToStage: with the real art a plant grows on
// continuously and never shrinks: the growth curve rises from stage to
// stage and the growth-scale clamp never binds, so nothing pops where the
// stage (and so the sprite) changes; through the draw path a field's
// units stand visibly taller stage by stage, sown to mature, and a ripe
// field stays as tall as a mature one (ripe sprites change the look, not
// the size).
func TestCropGrowthRisesStageToStage(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	names := cropNames()
	a := buildCropAtlas(art, names)
	steps := []cropState{{"sown", 2, 0, 0}, {"sprout", 14, 0, 0}, {"young", 37, 1, 0}, {"mature", 77, 2, 0}, {"ripe", 100, 3, 0}}
	for _, sp := range names {
		set := a.sets[sp]
		prev := 0.0
		for g := 8; g <= 100; g++ {
			st := cropGrowthStage(game.PlantView{Species: sp, Growth: g, Stage: stageOfGrowth(g)})
			if set.nominal[st] <= 0 {
				t.Fatalf("%s: no %s sprite", sp, cropStageNames[st])
			}
			h := set.height(float64(g))
			if got := float64(set.nominal[st]) * set.scale(st, g); math.Abs(got-h) > 1e-6 {
				t.Errorf("%s growth %d: the %s sprite draws %.3f tall, the growth curve says %.3f (the scale clamp binds)", sp, g, cropStageNames[st], got, h)
			}
			if h < prev-1e-9 {
				t.Errorf("%s growth %d: the plant shrinks to %.3f from %.3f", sp, g, h, prev)
			}
			prev = h
		}
		if !(set.height(14) < set.height(37.5) && set.height(37.5) < set.height(77.5)) {
			t.Errorf("%s: growth curve sprout %.3f, young %.3f, mature %.3f does not rise", sp, set.height(14), set.height(37.5), set.height(77.5))
		}
		var hs []float64
		for _, s := range steps {
			w := cropWorld(14, 14, cropPlants(sp, 1, 1, 12, 12, s.edit))
			r := New(&stubSim{w: 14, h: 14, world: w})
			r.art, r.speed = art, speedPaused
			var sum float64
			var units int
			cropDrawEach(r, 14, 14, TileSize, func(_ *game.PlantView, vs []ebiten.Vertex) {
				for q := 0; q+3 < len(vs); q += 4 {
					// the plant's height is its top's height above its root: the
					// quad's bottom less the sprite's reach below the root
					var spr *cropSprite
					for i := range r.lod.crops.atlas.sprites {
						if s := &r.lod.crops.atlas.sprites[i]; s.sx == min(vs[q].SrcX, vs[q+1].SrcX) && s.sy == vs[q].SrcY {
							spr = s
						}
					}
					if spr == nil {
						t.Fatalf("%s: a unit quad matches no atlas sprite", sp)
					}
					ks := (vs[q+1].DstX - vs[q].DstX) / spr.w
					root := max(vs[q+2].DstY, vs[q+3].DstY) - (spr.h-spr.fy)*ks
					sum += float64(root-min(vs[q].DstY, vs[q+1].DstY)) / TileSize
					units++
				}
			})
			hs = append(hs, sum/float64(units))
		}
		t.Logf("%-9s mean unit height above its root, sown..ripe %.3f", sp, hs)
		for i := 1; i < 4; i++ {
			if hs[i] < 1.25*hs[i-1] {
				t.Errorf("%s: %s units stand %.3f tall, not visibly taller than %s ones at %.3f", sp, steps[i].name, hs[i], steps[i-1].name, hs[i-1])
			}
		}
		if hs[4] < 0.95*hs[3] {
			t.Errorf("%s: ripe units stand %.3f tall, shorter than mature ones at %.3f", sp, hs[4], hs[3])
		}
	}
}

// TestCropSwayPivotsAtRoot: a unit leans about its root, the art's
// tile-centre point: under two different gusts its sheared left edge
// passes through the same point on the root's row, not above or below it.
func TestCropSwayPivotsAtRoot(t *testing.T) {
	t.Parallel()
	const up, down = 40, 12 // the block's stored px above and below the root
	draw := func(present uint64) []ebiten.Vertex {
		w := cropWorld(6, 6, cropPlants("wheat", 2, 2, 1, 1, nil))
		w.WindX, w.WindY, w.Weather = 2.4, 0.4, "cloudy"
		r := cropRenderer(w, 6, 6)
		for _, name := range cropStageNames {
			for v := 0; v < cropVariants; v++ {
				r.art.sprites[fmt.Sprintf("plant/wheat_%s_v%d", name, v)] = cropBlockSprite(up, down)
			}
		}
		r.present = present
		cropDraw(r, 6, 6, 64)
		return slices.Clone(r.lod.crops.verts)
	}
	a, b := draw(40), draw(95)
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("%d / %d vertices, want four units", len(a), len(b))
	}
	leaned := 0
	for q := 0; q < 16; q += 4 {
		ks := (a[q+1].DstX - a[q].DstX) / 20 // screen px per stored px: the block is 20 wide
		rootY := a[q+2].DstY - down*ks
		if d := b[q+2].DstY - a[q+2].DstY; math.Abs(float64(d)) > 1e-3 {
			t.Fatalf("unit %d: its foot moved %.3f px with the wind", q/4, d)
		}
		at := func(v []ebiten.Vertex) float32 { // the left edge's x on the root's row
			x0, y0, x1, y1 := v[q].DstX, v[q].DstY, v[q+2].DstX, v[q+2].DstY
			return x0 + (x1-x0)*(rootY-y0)/(y1-y0)
		}
		if d := math.Abs(float64(at(a) - at(b))); d > 0.05 {
			t.Errorf("unit %d: the left edge on the root's row moves %.2f px between gusts (the pivot is not the root)", q/4, d)
		}
		if math.Abs(float64(a[q].DstX-b[q].DstX)) > 0.5 {
			leaned++
		}
	}
	if leaned < 2 {
		t.Fatalf("only %d units lean differently between the gusts: the test proves nothing", leaned)
	}
}
