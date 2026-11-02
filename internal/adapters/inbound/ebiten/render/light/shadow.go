package light

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

// Colonist drop shadows (user decision, 2026-11-01): only colonists cast a
// shadow, and only while they stand on an unroofed tile. Nothing else
// casts one: no buildings, walls, rock, plants, items, roofs or bridges,
// and no animals, mechanoids, raiders, visitors or prisoners.
//
// A colonist's shadow is a soft ellipse under the feet, stretched and
// offset along the sun's (or, at night, the moon's) shadow direction and
// faded with the light's height: long and faint at dawn and dusk, short
// and dark at noon, a faint blob under the moon. Every blob of a frame
// goes to the screen in one batched draw, after the floors and zones and
// before every object; the blob texture is made once.

// Colonist blob sizes in tiles: colonistW across the shadow, colonistD
// along it at a high light; colonistH is the height that sets the
// stretch, clamped to colonistMaxLen.
const (
	colonistW      = 0.62
	colonistD      = 0.34
	colonistH      = 1.0
	colonistMaxLen = 1.2
	// colonistGain scales the light's shadow alpha: the blob is drawn
	// straight onto the screen, so it is a touch lighter than a cast
	// layer would be.
	colonistGain = 0.8
)

// ShadowColor is the shadow's dark cool tint at noon (ART §6.1); it
// tilts bluer at low sun (ShadowTint).
var ShadowColor = [3]float32{18, 20, 30}

// Foot is one colonist's shadow this frame: its ground pivot in world
// tiles and its size scale (creatures keep a minimum size when zoomed
// far out).
type Foot struct {
	X, Y, Scale float64
}

// Shadows is the colonist shadow pass's reusable state (kept on the
// renderer's environment).
type Shadows struct {
	// Feet is the pipeline's reused list of this frame's casters.
	Feet    []Foot
	batch   gfx.TriBatch
	blobImg *ebiten.Image
	// Drawn is how many blobs the last Draw queued; Flushes counts the
	// batched draws (tests).
	Drawn, Flushes int
	// Off skips the pass (Options > Graphics, PLAY-019).
	Off bool
}

// Cast, while set on a sprite pass (render/sprite Pass.Cast), marks draws
// that are not the frame's lit sprites: the pipeline sets it during a
// bake, and the overlays skip a figure's stains then.
type Cast struct{}

// SetOff turns the shadow pass off or on.
func (st *Shadows) SetOff(off bool) { st.Off = off }

// Colonist reports pw is one of the colony's own living people: not a
// raider, prisoner, visitor, ally or another faction's guest, not a body
// and not being carried.
func Colonist(pw *game.PawnView) bool {
	return !pw.Hostile && !pw.Prisoner && !pw.Visitor && !pw.Ally && pw.GuestFaction == "" &&
		!pw.Embodied && pw.CarriedBy == 0
}

// Casts reports pw casts a shadow on w: a colonist standing on a tile
// with no built or natural roof (a planned roof is no roof yet).
func Casts(pw *game.PawnView, w *game.WorldView) bool {
	return Colonist(pw) && !roofed(roofCellAt(w, pw.X, pw.Y))
}

// roofCellAt is the roof flags at (x, y) of w; 0 out of bounds.
func roofCellAt(w *game.WorldView, x, y int) uint8 {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoofCells) {
		return 0
	}
	return w.RoofCells[i]
}

// roofed reports a built or natural roof.
func roofed(c uint8) bool { return c&uint8(roof.Built|roof.Natural) != 0 }

// shadowVec is where the shadow of a caster of height h ends, relative
// to its foot, in tiles, at most colonistMaxLen long.
func shadowVec(l State, h float64) (float64, float64) {
	if l.ShadowLen == nil {
		return 0, 0
	}
	n := min(l.ShadowLen(h), colonistMaxLen)
	return l.ShadowDir[0] * n, l.ShadowDir[1] * n
}

// ColonistQuad is f's blob in world tiles (corners in texture order:
// top-left, top-right, bottom-left, bottom-right): colonistW across the
// shadow and colonistD plus its length along it, centred half that
// length away from the feet. Texture x runs along the shadow.
func ColonistQuad(f Foot, l State) [4][2]float64 {
	k := f.Scale
	if k <= 0 {
		k = 1
	}
	vx, vy := shadowVec(l, colonistH*k)
	n := math.Hypot(vx, vy)
	ux, uy := 1.0, 0.0
	if n > 1e-9 {
		ux, uy = vx/n, vy/n
	}
	along := colonistD*k + n
	across := colonistW * k
	cx, cy := f.X+vx/2, f.Y+vy/2
	hx, hy := ux*along/2, uy*along/2    // half along
	qx, qy := -uy*across/2, ux*across/2 // half across
	return [4][2]float64{
		{cx - hx - qx, cy - hy - qy},
		{cx + hx - qx, cy + hy - qy},
		{cx - hx + qx, cy - hy + qy},
		{cx + hx + qx, cy + hy + qy},
	}
}

// ColonistAlpha is the blob's alpha in light l on day dayOfYear: the
// light's shadow alpha (0 when the sun and moon are down or too low),
// times colonistGain, fading from full at the season's highest sun to
// half at the horizon and under the moon.
func ColonistAlpha(l State, dayOfYear int) float64 {
	if l.ShadowLen == nil || l.ShadowAlpha <= 0 {
		return 0
	}
	return gfx.Clamp01(l.ShadowAlpha) * colonistGain * (0.5 + 0.5*sunRatio(l, dayOfYear))
}

// sunRatio is the sun's elevation over the season's highest (0..1); 0 at
// night.
func sunRatio(l State, dayOfYear int) float64 {
	return gfx.Clamp01(SunElevation(l) / (SeasonMaxEl(dayOfYear) * math.Pi / 180))
}

// blobTexture is a soft white ellipse (alpha only), made on first use.
func (st *Shadows) blobTexture() *ebiten.Image {
	if st.blobImg == nil {
		const bw, bh = 64, 32
		src := image.NewNRGBA(image.Rect(0, 0, bw, bh))
		for y := 0; y < bh; y++ {
			for x := 0; x < bw; x++ {
				dx := (float64(x) + 0.5 - bw/2) / (bw / 2)
				dy := (float64(y) + 0.5 - bh/2) / (bh / 2)
				a := math.Exp(-3.2*(dx*dx+dy*dy)) - math.Exp(-3.2)
				src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * gfx.Clamp01(a/(1-math.Exp(-3.2))))})
			}
		}
		st.blobImg = ebiten.NewImageFromImage(src)
	}
	return st.blobImg
}

// Draw draws the frame's colonist shadows (feet, from Casts) onto screen
// in light l, placed by v's camera, in one batched draw. dayOfYear sets
// the season's highest sun.
func (st *Shadows) Draw(v *render.View, l State, screen *ebiten.Image, feet []Foot, dayOfYear int) {
	st.Drawn = 0
	if st.Off || len(feet) == 0 {
		return
	}
	a := ColonistAlpha(l, dayOfYear)
	if a <= 0 {
		return
	}
	t := ShadowTint(sunRatio(l, dayOfYear))
	c := color.RGBA{uint8(t[0]), uint8(t[1]), uint8(t[2]), 255}
	tex := st.blobTexture()
	tw, th := float64(tex.Bounds().Dx()), float64(tex.Bounds().Dy())
	uv := [4][2]float64{{0, 0}, {tw, 0}, {0, th}, {tw, th}}
	st.batch.Reset()
	for _, f := range feet {
		var i uint32
		for k, p := range ColonistQuad(f, l) {
			sx, sy := v.WorldToScreenF(p[0], p[1])
			j := st.batch.Vert(float64(sx), float64(sy), uv[k][0], uv[k][1], c, a)
			if k == 0 {
				i = j
			}
		}
		st.batch.Tri(i, i+1, i+2)
		st.batch.Tri(i+1, i+3, i+2)
		st.Drawn++
	}
	st.batch.Flush(screen, tex, ebiten.BlendSourceOver)
	st.Flushes++
}

// SunElevation is the sun's elevation (radians) in l, 0 below the
// horizon: SunL is LightDir's unit vector, y foreshortened by ProjShearK.
func SunElevation(l State) float64 {
	if l.SunL[2] <= 0 {
		return 0
	}
	return math.Atan2(float64(l.SunL[2]), math.Hypot(float64(l.SunL[0]), float64(l.SunL[1])/ProjShearK))
}

// ShadowTint is the shadow colour (0..255): bluer at low sun.
func ShadowTint(ratio float64) [3]float32 {
	return [3]float32{ShadowColor[0], ShadowColor[1], 34 - 4*float32(gfx.Clamp01(ratio))}
}
