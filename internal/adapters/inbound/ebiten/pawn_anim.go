package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Pawn figure animation: walk frames synced to tile progress, idle
// breathing, work swings towards the worked tile, recoil/lunge/hit
// reactions, falling over when downed, lying in bed under the blanket,
// and being carried.

const (
	fallFrames = 16
	deadCSR    = 0.62
)

var deadCS = colorScale{deadCSR, 0.6, 0.64, 1}

// poseFacing is the facing to draw: the motion's smoothed facing, turned
// to the worked tile while working in place and to the blocker while
// stuck behind one.
func (r *Renderer) poseFacing(p game.PawnView, m moveState) facing {
	f := m.Facing
	if m.Blend >= minBlend {
		return f
	}
	if p.Acting != "" && (p.ActX != p.X || p.ActY != p.Y) {
		return facingFor(p.ActX-p.X, p.ActY-p.Y, f)
	}
	return blockedFacing(p, r.drawPawns, r.drawW.Animals, f)
}

// actionPose is one action layer's contribution to an upright pawn (ART
// §27.2): the body frame suffix, what it holds and how, and an extra
// transform added to the standing pose. Layers are tried in priority
// order by standingPose: hit reaction, combat, carrying, work.
type actionPose struct {
	ok     bool
	suffix string   // body frame suffix ("_aim", "_stab3", "_chop5"); "" keeps the base frame
	held   heldItem // replaces the default held item when held.set
	dx, dy float64  // added offset, tiles
	rot    float64  // added rotation, radians
	squash float64  // vertical squash 0..1 (bending, crouching)
	flash  float32  // hit flash 0..1
}

// standingPose is the live transform for an upright pawn, the frame
// suffix to use and what it holds: walking or breathing, then the first
// action layer that applies.
func (r *Renderer) standingPose(p game.PawnView, m moveState) (pose, string, heldItem) {
	ps := walkPose(m.Facing, m)
	suffix := ""
	moving := m.Blend >= minBlend
	if moving {
		suffix = walkSuffix(m)
	} else {
		b := math.Sin(2 * math.Pi * float64(r.present+uint64(p.ID)*37) / 150)
		ps.sy = 1 + 0.012*b
	}
	held := r.idleHeld(p, m)
	for _, layer := range []func(game.PawnView, moveState) actionPose{r.hitReaction, r.combatPose, r.carryPose, r.workPose} {
		a := layer(p, m)
		if !a.ok {
			continue
		}
		if a.suffix != "" {
			suffix = a.suffix
		}
		if a.held.set {
			held = a.held
		}
		ps.dx += a.dx
		ps.dy += a.dy
		ps.rot += a.rot
		if a.squash != 0 {
			_, sy := ps.scale()
			ps.sy = sy * (1 - a.squash)
		}
		if a.flash > ps.flash {
			ps.flash = a.flash
		}
		break
	}
	dx, dy, fl := r.combatOffset(p)
	ps.dx, ps.dy = ps.dx+dx, ps.dy+dy
	if fl > ps.flash {
		ps.flash = fl
	}
	return ps, suffix, held
}

func isHeadLayer(key string) bool {
	return strings.HasPrefix(key, "pawn/hair_") || strings.HasPrefix(key, "pawn/tuque_")
}

func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	st := r.animState()
	if p.CarriedBy != 0 {
		return // drawn over its rescuer
	}
	st.dropCarry(p.ID)
	vis := r.last[p.ID]
	if p.Lying {
		r.drawLyingPawn(screen, p, vis, wx, wy, tile)
		return
	}
	m := r.pawnMotion(p.ID)
	m.Facing = r.poseFacing(p, m)
	ps, suffix, item := r.standingPose(p, m)
	face := m.Facing
	r.drawHeld(screen, p, face, suffix, item, wx, wy, tile, ps, true) // the parts behind the body
	hdx, hdy := headDelta(r.art, face, suffix)
	headPs := ps
	headPs.dx += hdx
	headPs.dy += hdy
	for _, ly := range r.pawnLayers(r.look(p), face, p.Apparel...) {
		lp := ps
		if isHeadLayer(ly.key) {
			lp = headPs // hair and hats ride the head
		}
		r.drawSpritePose(screen, r.framed(ly.key, suffix), ly.tint, wx, wy, tile, lp)
	}
	r.drawHeld(screen, p, face, suffix, item, wx, wy, tile, ps, false) // the parts in front
	if p.Hostile && !p.Lying {
		r.drawHostileMarker(screen, p, wx+ps.dx, wy+ps.dy, tile)
	}
	if p.Drafted {
		if img := r.art.image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := r.worldToScreenF(wx+0.62+ps.dx, wy-0.95+ps.dy)
			drawIcon(screen, img, float64(sx), float64(sy), tile*0.3)
		}
	}
	for _, q := range r.drawPawns {
		if q.CarriedBy == p.ID {
			r.drawCarried(screen, q, face, wx+ps.dx, wy+ps.dy, tile)
		}
	}
}

// layersSouth draws the figure facing south with a pose.
func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps pose) {
	for _, ly := range r.pawnLayers(r.look(p), faceSouth, p.Apparel...) {
		r.drawSpritePose(screen, ly.key, ly.tint, wx, wy, tile, ps)
	}
}

// drawLyingPawn: in a bed under the blanket, or fallen on the ground.
func (r *Renderer) drawLyingPawn(screen *ebiten.Image, p game.PawnView, vis pawnVisual, wx, wy, tile float64) {
	cs := noCS
	if p.Status == "dead" {
		cs = deadCS
	}
	if p.InBed {
		if b, ok := bedUnder(r.drawW, p.X, p.Y); ok {
			r.drawInBed(screen, p, b, vis.lyingSince, tile, cs)
			return
		}
	}
	// on the bare ground
	k := easeOut(float64(r.present-vis.lyingSince) / fallFrames)
	side := 1.0
	if vis.facing == faceWest {
		side = -1
	}
	ps := pose{px: figurePivotX, py: figurePivotY, cs: cs}
	ps.rot = side * math.Pi / 2 * k
	ps.dx = -side * 0.42 * k
	ps.dy = 0.1 * k
	if p.Status != "dead" {
		ps.sy = 1 + 0.02*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/110)
	}
	if k < 1 && p.Status != "alive" {
		ps.flash = float32(0.5 * (1 - k))
	}
	r.drawFigure(screen, p, wx, wy, tile, ps)
}

// bedUnder is the sleeping furniture covering tile (x, y).
func bedUnder(w game.WorldView, x, y int) (game.BuildingView, bool) {
	for _, b := range w.Buildings {
		if b.Kind != "bed" && b.Kind != "double bed" && b.Kind != "sleeping spot" {
			continue
		}
		bw, bh := max(b.W, 1), max(b.H, 1)
		if x >= b.X && y >= b.Y && x < b.X+bw && y < b.Y+bh {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// headPivot is a standing figure's head centre, in tiles from its tile
// corner (measured on pawn/body_south: head rows 81..140, tile top 153).
const headPivotX, headPivotY = 0.5, -0.34

// drawInBed lays the figure along the bed (or sleeping spot) with its
// head on the pillow, breathing slowly; a bed then draws its blanket over
// it, while on a floor marking the figure lies flatter in the open.
func (r *Renderer) drawInBed(screen *ebiten.Image, p game.PawnView, b game.BuildingView, since uint64, tile float64, cs colorScale) {
	bw, bh := float64(max(b.W, 1)), float64(max(b.H, 1))
	x0, y0 := float64(b.X), float64(b.Y)
	spot := b.Kind == "sleeping spot"
	// pillow centre on screen: along the bed it sits at the head end
	// (bed: 0.36 in and raised by the shear; spot: the flat painted
	// pillow, 0.27 in); across the bed it is the sleeper's own column or
	// row, so two can share a double bed.
	cx, cy := float64(p.X)+0.5, float64(p.Y)
	hx, hy, rot := cx, y0+0.2, 0.0
	switch {
	case spot && b.Facing == "east":
		hx, hy, rot = x0+0.27, cy+0.5, -math.Pi/2
	case spot && b.Facing == "west":
		hx, hy, rot = x0+bw-0.27, cy+0.5, math.Pi/2
	case spot && b.Facing == "north":
		hx, hy, rot = cx, y0+bh-0.27, math.Pi
	case spot:
		hy = y0 + 0.27
	case b.Facing == "east":
		hx, hy, rot = x0+0.36, cy+0.3, -math.Pi/2
	case b.Facing == "west":
		hx, hy, rot = x0+bw-0.36, cy+0.3, math.Pi/2
	case b.Facing == "north":
		hx, hy, rot = cx, y0+bh-0.56, math.Pi
	}
	breath := 1 + 0.025*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/170)
	sy := 0.85
	if spot && (b.Facing == "" || b.Facing == "south" || b.Facing == "north") {
		sy = 0.72 // lying flat along the floor, seen from above
	}
	ps := pose{px: headPivotX, py: headPivotY, rot: rot, sx: 0.85, sy: sy * breath, cs: cs}
	// lie down over easeFrames: from standing on the tile to the bed pose
	k := smoothEase(r.present, since, easeFrames)
	ox, oy := hx-headPivotX, hy-headPivotY
	if k < 1 {
		stand := pose{px: headPivotX, py: headPivotY, sx: 1, sy: 1, cs: cs}
		ps = lerpPose(stand, ps, k)
		ox, oy = float64(p.X)*(1-k)+ox*k, float64(p.Y)*(1-k)+oy*k
	}
	r.drawFigure(screen, p, ox, oy, tile, ps)
	if p.Status == "alive" {
		defer r.drawSnore(screen, p, hx, hy, tile)
	}
	if spot {
		return
	}
	key := "furniture/bed_basic_blanket_" + facingOr(b.Facing)
	if b.Kind == "double bed" {
		key = "furniture/bed_double_blanket_" + facingOr(b.Facing)
	}
	bcs := cs
	if k < 1 {
		bcs[3] *= float32(k) // the blanket settles over the sleeper
	}
	r.drawSpritePose(screen, key, r.bedTint(b), x0, y0, tile, pose{cs: bcs})
}

// bedTint is the per-bed blanket colour (stable by building id).
func (r *Renderer) bedTint(b game.BuildingView) color.NRGBA {
	cloth := r.colonyCloths()
	if len(cloth) == 0 {
		return color.NRGBA{}
	}
	return r.art.tint("cloth", cloth[mix64(uint64(b.ID))%uint64(len(cloth))])
}

// carryPoses are a patient's pose lying where it was picked up and
// slung across the rescuer's shoulders.
func carryPoses(f facing) (ground, slung pose) {
	side := 1.0
	if f == faceWest {
		side = -1
	}
	ground = pose{px: figurePivotX, py: figurePivotY, rot: side * math.Pi / 2, dx: -side * 0.42, dy: 0.1}
	slung = pose{px: figurePivotX, py: figurePivotY, rot: math.Pi / 2, sx: 0.8, sy: 0.8, dx: -0.36, dy: -0.62}
	return ground, slung
}

// drawCarried draws a patient slung across its rescuer's shoulders,
// lifted from the ground over easeFrames.
func (r *Renderer) drawCarried(screen *ebiten.Image, q game.PawnView, f facing, wx, wy, tile float64) {
	ground, slung := carryPoses(f)
	k := r.animState().carryEase(q.ID, r.present)
	r.drawFigure(screen, q, wx, wy, tile, lerpPose(ground, slung, k))
}

// drawSnore floats small "z"s up and away from a sleeper's head.
func (r *Renderer) drawSnore(screen *ebiten.Image, p game.PawnView, hx, hy, tile float64) {
	const period = 75
	for i := 0; i < 3; i++ {
		t := float64((r.present+uint64(p.ID)*29+uint64(i*period/3))%period) / period
		size := tile * (0.07 + 0.06*t)
		x := hx + 0.18 + 0.28*t + 0.04*math.Sin(t*6)
		y := hy - 0.25 - 0.55*t
		sx, sy := r.worldToScreenF(x, y)
		c := fade(color.RGBA{238, 240, 250, 255}, 0.9*math.Sin(math.Pi*t))
		w := float32(size)
		lw := float32(math.Max(1.5, tile*0.022))
		vector.StrokeLine(screen, sx, sy, sx+w, sy, lw, c, true)
		vector.StrokeLine(screen, sx+w, sy, sx, sy+w, lw, c, true)
		vector.StrokeLine(screen, sx, sy+w, sx+w, sy+w, lw, c, true)
	}
}
