package ebiten

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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

// standingPose is the live transform for an upright pawn and the walk
// frame suffix to use.
func (r *Renderer) standingPose(p game.PawnView, vis pawnVisual, traces []game.TraceView) (pose, string) {
	ps := pose{px: figurePivotX, py: figurePivotY}
	progress := moveProgress(int(r.game.Tick()-vis.moveTick), game.TicksPerCell)
	moving := vis.fromX != p.X || vis.fromY != p.Y
	suffix := ""
	if moving {
		ph := walkPhase(vis, progress)
		suffix = walkFrame(ph)
		ps.dy = -math.Abs(math.Sin(ph*math.Pi)) * pawnBobPx * 1.4
	} else {
		b := math.Sin(2 * math.Pi * float64(r.present+uint64(p.ID)*37) / 150)
		ps.sy = 1 + 0.012*b
	}
	if st, ok := styleFor(p.Acting); ok && !moving {
		suffix = workArms(st.motion, swingPhase(r.present, uint64(p.ID), st.period))
		k := workSwing(r.present, uint64(p.ID), st.period)
		ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
		if ux == 0 && uy == 0 {
			ps.sy *= 1 - 0.07*k // bending down to the ground
		}
		ps.dx += ux * st.reach * k
		ps.dy += uy * st.reach * k
		ps.rot += ux * 0.14 * k
	}
	dx, dy, fl := attackReaction(traces, p.X, p.Y)
	ps.dx, ps.dy, ps.flash = ps.dx+dx, ps.dy+dy, fl
	return ps, suffix
}

func (r *Renderer) drawPawnArt(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, sw, sh int) {
	if p.CarriedBy != 0 {
		return // drawn over its rescuer
	}
	vis := r.last[p.ID]
	if p.Lying {
		r.drawLyingPawn(screen, p, vis, wx, wy, tile)
		return
	}
	ps, suffix := r.standingPose(p, vis, r.drawW.Traces)
	held := func() { r.drawHeldWeapon(screen, p, vis.facing, suffix, wx, wy, tile, ps) }
	if st, ok := styleFor(p.Acting); ok && st.motion != motionNone && strings.HasPrefix(suffix, "_work") {
		// working: the tool (or bare hands) replaces the weapon
		held = func() { r.drawTool(screen, st.tool, vis.facing, suffix, wx, wy, tile, ps) }
	}
	if vis.facing == faceNorth {
		held() // in front, hidden by the body
	}
	for _, ly := range r.pawnLayers(r.look(p), vis.facing) {
		r.drawSpritePose(screen, r.framed(ly.key, suffix), ly.tint, wx, wy, tile, ps)
	}
	if vis.facing != faceNorth {
		held()
	}
	if p.Drafted {
		if img := r.art.image("ui_icon/draft", color.NRGBA{}); img != nil {
			sx, sy := r.worldToScreenF(wx+0.62+ps.dx, wy-0.95+ps.dy)
			drawIcon(screen, img, float64(sx), float64(sy), tile*0.3)
		}
	}
	for _, q := range r.drawPawns {
		if q.CarriedBy == p.ID {
			r.drawCarried(screen, q, wx+ps.dx, wy+ps.dy, tile)
		}
	}
}

// layersSouth draws the figure facing south with a pose.
func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps pose) {
	for _, ly := range r.pawnLayers(r.look(p), faceSouth) {
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
			r.drawInBed(screen, p, b, tile, cs)
			return
		}
	}
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
		if b.Kind != "bed" && b.Kind != "cot" {
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

// drawInBed lays the figure along the bed with its head on the pillow,
// breathing slowly, then draws the blanket over it.
func (r *Renderer) drawInBed(screen *ebiten.Image, p game.PawnView, b game.BuildingView, tile float64, cs colorScale) {
	bw, bh := float64(max(b.W, 1)), float64(max(b.H, 1))
	x0, y0 := float64(b.X), float64(b.Y)
	// pillow centre on screen (from the furniture geometry: pillow y and
	// height under the 0.5 shear); the blanket's fold starts ~0.2 tile
	// below it, so the face stays above the covers
	in := 0.1 // cot
	if b.Kind == "bed" {
		in = 0.2
	}
	hx, hy, rot := x0+0.5, y0+in, 0.0
	switch b.Facing {
	case "east":
		hx, hy, rot = x0+0.36, y0+0.3, -math.Pi/2
	case "west":
		hx, hy, rot = x0+bw-0.36, y0+0.3, math.Pi/2
	case "north":
		hx, hy, rot = x0+0.5, y0+bh-0.36-0.2, math.Pi
	}
	breath := 1 + 0.025*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/170)
	ps := pose{px: headPivotX, py: headPivotY, rot: rot, sx: 0.85, sy: 0.85 * breath, cs: cs}
	r.drawFigure(screen, p, hx-headPivotX, hy-headPivotY, tile, ps)
	key := "furniture/bed_basic_blanket_" + facingOr(b.Facing)
	if b.Kind == "cot" {
		key = "furniture/bed_cot_blanket_" + facingOr(b.Facing)
	}
	r.drawSpritePose(screen, key, r.bedTint(b), x0, y0, tile, pose{cs: cs})
}

// bedTint is the per-bed blanket colour (stable by building id).
func (r *Renderer) bedTint(b game.BuildingView) color.NRGBA {
	cloth := r.art.tintNames("cloth")
	if len(cloth) == 0 {
		return color.NRGBA{}
	}
	return r.art.tint("cloth", cloth[mix64(uint64(b.ID))%uint64(len(cloth))])
}

// drawCarried draws a patient slung across its rescuer's shoulders.
func (r *Renderer) drawCarried(screen *ebiten.Image, q game.PawnView, wx, wy, tile float64) {
	ps := pose{px: figurePivotX, py: figurePivotY, rot: math.Pi / 2, sx: 0.8, sy: 0.8, dx: -0.36, dy: -0.62}
	r.drawFigure(screen, q, wx, wy, tile, ps)
}

// heldWeaponKey maps a wielded weapon to a sprite and how it is held.
func heldWeaponKey(weapon string) (key string, scale float64, ok bool) {
	switch weapon {
	case "rifle":
		return "weapon/rifle_basic", 0.62, true
	case "club":
		return "item/club_wood", 0.5, true
	}
	return "", 0, false
}

// drawHeldWeapon draws the wielded weapon with its grip at the body's
// hand attachment for this walk frame, pointing along the facing, and
// moved with the figure's pose.
func (r *Renderer) drawHeldWeapon(screen *ebiten.Image, p game.PawnView, f facing, suffix string, wx, wy, tile float64, ps pose) {
	key, k, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return
	}
	body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
	wsp := r.art.sprite(key)
	img := r.art.image(key, color.NRGBA{})
	if body == nil || wsp == nil || img == nil {
		return
	}
	hand, okH := body.meta.Attach["hand"]
	if !okH {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	hx, hy := bx+hand[0]*s, by+hand[1]*s
	grip, okG := wsp.meta.Attach["grip"]
	if !okG {
		grip = [2]float64{float64(wsp.meta.Canvas[0]) / 2, float64(wsp.meta.Canvas[1]) / 2}
	}
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	switch f {
	case faceWest:
		g.Scale(-1, 1)
	case faceSouth:
		g.Rotate(math.Pi / 2.6)
	case faceNorth:
		g.Rotate(-math.Pi / 2.6)
	}
	g.Scale(s*k, s*k)
	g.Translate(hx, hy)
	g.Concat(ps.geo(float64(sx), float64(sy), tile))
	op := &ebiten.DrawImageOptions{GeoM: g, Filter: ebiten.FilterLinear}
	screen.DrawImage(img, op)
}

// drawTool draws a work tool with its grip in the tool hand of the
// current arm frame, angled for the swing.
func (r *Renderer) drawTool(screen *ebiten.Image, key string, f facing, suffix string, wx, wy, tile float64, ps pose) {
	if key == "" {
		return
	}
	body := r.art.sprite(r.framed("pawn/body_"+f.String(), suffix))
	tsp := r.art.sprite(key)
	img := r.art.image(key, color.NRGBA{})
	if body == nil || tsp == nil || img == nil {
		return
	}
	hand, ok := body.meta.Attach["hand"]
	if !ok {
		return
	}
	grip, ok := tsp.meta.Attach["grip"]
	if !ok {
		grip = [2]float64{float64(tsp.meta.Canvas[0]) / 2, float64(tsp.meta.Canvas[1]) / 2}
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	g := ebiten.GeoM{}
	g.Translate(-grip[0], -grip[1])
	if f == faceWest {
		g.Scale(-1, 1)
	}
	g.Rotate(toolAngle(f, suffix))
	g.Scale(s*0.62, s*0.62)
	g.Translate(bx+hand[0]*s, by+hand[1]*s)
	g.Concat(ps.geo(float64(sx), float64(sy), tile))
	screen.DrawImage(img, &ebiten.DrawImageOptions{GeoM: g, Filter: ebiten.FilterLinear})
}
