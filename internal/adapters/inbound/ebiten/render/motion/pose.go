package motion

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

// Poses, gaits and easing (ART §27). Pseudo-randomness comes from
// art.Mix64 of stable keys, so a replay looks the same.

// Pose is a transform applied to a whole figure (all of its layers).
type Pose struct {
	Dx, Dy float64    // offset in tiles
	Rot    float64    // radians (screen, clockwise) about the pivot
	Px, Py float64    // pivot in tiles from the sprite's tile top-left
	Sx, Sy float64    // scale about the pivot; 0 means 1
	Cs     [4]float32 // colour scale (the root colorScale); zero means none
	Flash  float32    // 0..1 hit flash
	Skew   float64    // horizontal shear about the pivot (wind sway)
}

// Geo is the pose part of a sprite transform, for a tile whose
// top-left is at screen (sx, sy).
func (p Pose) Geo(sx, sy, tile float64) ebiten.GeoM {
	px, py := sx+p.Px*tile, sy+p.Py*tile
	kx, ky := p.Scale()
	g := ebiten.GeoM{}
	g.Translate(-px, -py)
	if p.Skew != 0 {
		g.Skew(p.Skew, 0)
	}
	g.Scale(kx, ky)
	g.Rotate(p.Rot)
	g.Translate(px+p.Dx*tile, py+p.Dy*tile)
	return g
}

// Scale is the pose's scale with 0 read as 1.
func (p Pose) Scale() (float64, float64) {
	sx, sy := p.Sx, p.Sy
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// FigurePivotX, FigurePivotY are where a standing animal's feet meet the ground, in
// tiles from its tile's top-left (pawns: pawnFeetX, pawnFeetY).
const FigurePivotX, FigurePivotY = 0.5, 0.55

// EaseOut decelerates over t in [0, 1].
func EaseOut(t float64) float64 {
	t = clamp01(t)
	return 1 - (1-t)*(1-t)
}

// easeOutBack overshoots slightly: items "pop" in.
func easeOutBack(t float64) float64 {
	t = clamp01(t)
	const c1 = 1.70158
	const c3 = c1 + 1
	return 1 + c3*math.Pow(t-1, 3) + c1*math.Pow(t-1, 2)
}

// Hash01 is a stable pseudo-random value in [0, 1) for a key.
func Hash01(keys ...uint64) float64 {
	h := uint64(0x9E3779B97F4A7C15)
	for _, k := range keys {
		h = art.Mix64(h ^ k)
	}
	return float64(h>>11) / float64(1<<53)
}

// Gait (ART §27.1): a pawn's cycle (two footfalls) spans StrideLen tiles
// of stride. Pawns have no walk frames: WalkFrames only phases the bob so
// its peaks fall where the old cycle's contact poses were (01 and 07). An
// animal trot has TrotFrames frames and its own ground travel per cycle
// (metadata "gait", PLAY-022; animal_figure.go).
const (
	// PawnBobPx is a walking pawn's bob amplitude, in tiles.
	PawnBobPx = 0.035

	WalkFrames = 12
	TrotFrames = 8
	StrideLen  = 1.6
	// ContactEvery is the stride between two foot contacts.
	ContactEvery = StrideLen / 2

	// gait amplitudes, in tiles / radians at normal walking speed
	bobAmp   = PawnBobPx * 1.4
	swayAmp  = 0.015
	leanAmp  = 0.06
	leanBack = 0.05
	MinBlend = 0.05 // below this the static pose is used
)

// TrotSuffix is the animal trot frame suffix ("_walk01".."_walk08") for
// phase ph, in cycles (any real; the fraction picks the frame).
func TrotSuffix(ph float64) string {
	return fmt.Sprintf("_walk%02d", TrotFrame(ph)+1)
}

// TrotFrame is the 0-based trot frame at phase ph.
func TrotFrame(ph float64) int {
	f := ph - math.Floor(ph)
	return min(int(f*TrotFrames), TrotFrames-1)
}

// GaitScale is how strongly the gait shows: speed relative to a straight
// step, capped (diagonals are faster), times the walk blend.
func GaitScale(m MoveState) float64 {
	return math.Min(m.Speed*float64(game.TicksPerCell), 1) * clamp01(m.Blend)
}

// humps is |cos| with two humps per cycle whose peaks sit on the centres
// of frames 01 and 07 (foot contact); n is the cycle's frame count.
func humps(stride float64, n int) float64 {
	cyc := stride/StrideLen - 0.5/float64(n)
	return math.Abs(math.Cos(2 * math.Pi * cyc))
}

// BobRaise is how far (tiles, up) the body is lifted: A*|cos| scaled by
// speed and walk blend, peaking at foot contact.
func BobRaise(stride, speed, blend float64) float64 {
	return bobAmp * humps(stride, WalkFrames) * GaitScale(MoveState{Speed: speed, Blend: blend})
}

// GaitSway is the lateral body sway (tiles) for N/S walkers: weight
// shifts onto the stance foot once per cycle.
func GaitSway(m MoveState) float64 {
	return swayAmp * math.Sin(2*math.Pi*m.Stride/StrideLen) * GaitScale(m)
}

// GaitLean is the forward lean (radians, clockwise) of an E/W walker:
// proportional to speed, and a short lean back while slowing to a stop.
func GaitLean(f art.Facing, m MoveState) float64 {
	var k float64
	switch {
	case m.Moving:
		k = leanAmp * GaitScale(m)
	case m.Blend > 0:
		k = -leanBack * clamp01(m.Blend)
	}
	switch f {
	case art.FaceEast:
		return k
	case art.FaceWest:
		return -k
	}
	return 0
}

// WalkPose is the gait's share of a figure transform, applied to the
// standing pose base (its pivot kept).
func WalkPose(base Pose, f art.Facing, m MoveState) Pose {
	ps := base
	if m.Blend >= MinBlend {
		ps.Dy = -BobRaise(m.Stride, m.Speed, m.Blend)
	}
	if f == art.FaceNorth || f == art.FaceSouth {
		ps.Dx = GaitSway(m)
	}
	ps.Rot = GaitLean(f, m)
	return ps
}

// BlockedFacing turns a blocked creature towards whatever is in its way.
func BlockedFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev art.Facing) art.Facing {
	if p.BlockedBy == 0 {
		return prev
	}
	for _, q := range pawns {
		if q.ID == p.BlockedBy {
			return art.FacingFor(q.X-p.X, q.Y-p.Y, prev)
		}
	}
	for _, a := range animals {
		if a.ID == p.BlockedBy {
			return art.FacingFor(a.X-p.X, a.Y-p.Y, prev)
		}
	}
	return prev
}

// ContactsCrossed counts foot contacts (every ContactEvery of stride)
// passed going from one stride to the next.
func ContactsCrossed(prev, cur float64) int {
	if cur <= prev {
		return 0
	}
	return int(math.Floor(cur/ContactEvery) - math.Floor(prev/ContactEvery))
}

// FootPoint is where the foot of a given contact parity lands, in world
// tiles, for a creature whose tile corner is (x, y).
func FootPoint(x, y float64, f art.Facing, parity int) [2]float64 {
	side := 0.12
	if parity%2 != 0 {
		side = -side
	}
	if f == art.FaceEast || f == art.FaceWest {
		return [2]float64{x + 0.5 + side, y + 0.62}
	}
	return [2]float64{x + 0.5 + side, y + 0.6}
}

// EaseFrames is how long bed entry and carry lifts take, in present frames.
const EaseFrames = 12

// SmoothEase is smoothstep over frames present frames since since.
func SmoothEase(now, since uint64, frames float64) float64 {
	if now <= since {
		return 0
	}
	t := clamp01(float64(now-since) / frames)
	return t * t * (3 - 2*t)
}

// LerpPose blends two poses (same pivot assumed from a).
func LerpPose(a, b Pose, k float64) Pose {
	if k <= 0 {
		return a
	}
	if k >= 1 {
		return b
	}
	l := func(x, y float64) float64 { return x*(1-k) + y*k }
	ax, ay := a.Scale()
	bx, by := b.Scale()
	return Pose{
		Dx: l(a.Dx, b.Dx), Dy: l(a.Dy, b.Dy), Rot: l(a.Rot, b.Rot),
		Px: a.Px, Py: a.Py,
		Sx: l(ax, bx), Sy: l(ay, by),
		Cs: b.Cs, Flash: b.Flash, Skew: l(a.Skew, b.Skew),
	}
}

// Unit is (x, y) normalised; (0, 0) stays put.
func Unit(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	if n == 0 {
		return 0, 0
	}
	return x / n, y / n
}

// Framed is key+suffix when the catalog has that frame, else key.
func Framed(a *art.Catalog, key, suffix string) string {
	if suffix == "" {
		return key
	}
	if k := key + suffix; a.Has(k) {
		return k
	}
	return key
}

// PopScale is the pop-in scale of a ground item that appeared at present
// frame born (0: there from the start, never pops).
func PopScale(born, present uint64) float64 {
	if born == 0 {
		return 1
	}
	return easeOutBack(float64(present-born) / 14)
}

// Sway is the wind shear for a plant at (x, y) at present frame present:
// a slow breeze with gusts, offset per tile so a field ripples.
func Sway(present uint64, x, y int, amount float64) float64 {
	t := float64(present)
	gust := 0.6 + 0.4*math.Sin(t/260+float64(x)*0.12)
	return amount * gust * math.Sin(t/38+float64(x)*0.55+float64(y)*0.35)
}

// clamp01 is t clamped to [0, 1] (gfx.Clamp01; motion does not draw).
func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
