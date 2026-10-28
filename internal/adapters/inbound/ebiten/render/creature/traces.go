package creature

import (
	"cmp"
	"image/color"
	"math"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Presentation of animals, held weapons and attack traces. Everything
// here reads the WorldView/PawnView; nothing writes back.

// FaceAttack turns an attacker standing at (x, y) and facing f towards
// its latest target so weapons and bites point the right way.
func FaceAttack(f art.Facing, traces []game.TraceView, x, y int) art.Facing {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			f = art.FacingFor(t.ToX-x, t.ToY-y, f)
		}
	}
	return f
}

// AnimalAt is where an animal is drawn: its motion at animT, or its
// tile before the first sync.
func (c *Set) AnimalAt(v *render.View, an game.AnimalView) (float64, float64) {
	m, ok := c.motionOf(v, an.ID)
	if !ok {
		return float64(an.X), float64(an.Y)
	}
	return m.X, m.Y
}

// DrawAnimalOverlays draws hunt marks and health bars above animals, then
// the sleepers' snores. The snores (vector strokes) go in a pass of their
// own: a vector stroke between two sprite draws flushes the vector batch,
// a path fill per animal (milliseconds a frame with the map in view).
func (c *Set) DrawAnimalOverlays(v *render.View, pass sprite.Pass, cull *Cull, screen *ebiten.Image, w game.WorldView, tile float64) {
	pad, padS := cull.Pad, cull.PadS+1 // off screen, as the figure pass culls; the marks float over the tallest body
	for i := range w.Animals {
		an := &w.Animals[i]
		if !cull.InView(an.X, an.Y, pad, padS) {
			continue
		}
		// the map tier (< 16 px tiles) draws no decals (lod.go): a snore's
		// "z" would be under two pixels there
		if an.Sleeping && !cull.MapTier && c.AnimalResting(v, *an) {
			wx, wy := c.AnimalAt(v, *an)
			c.DrawSnoreFor(v, screen, uint64(an.ID), wx+0.25, wy+snoreDrop-c.AnimalRestTop(v, *an), tile)
		}
	}
	c.DrawChaseLines(v, cull, screen, w, tile) // PLAY-017: under the marks
	for i := range w.Animals {
		an := &w.Animals[i]
		if !cull.InView(an.X, an.Y, pad, padS) {
			continue
		}
		wx, wy := c.AnimalAt(v, *an)
		top := c.AnimalTop(v, *an, tile, cull.MinTile)
		lift := MarkerLift(top)
		switch {
		case an.Hostile || an.Manhunter: // PLAY-017: every hostile animal, not only manhunters
			if v.Tracing() {
				sx, sy := v.WorldToScreenF(wx, wy)
				v.MarkDrawn("hostile-marker", sx, sy, 0, 0)
				_, my := v.WorldToScreenF(wx, wy-lift)
				v.MarkDrawn("mark-lift", sx, my, 0, 0)
			}
			c.Host.HostileMarker(screen, game.PawnView{}, wx, wy, lift, tile)
		case an.Designated: // the icon (a half-size tile) centred where the chevron would be
			spin := motion.Pose{Px: 0.5, Py: 0.5, Sx: 0.5, Sy: 0.5, Rot: float64(v.Present) * 0.035}
			c.DrawSpritePose(v, pass, screen, "overlay/designation_hunt", color.NRGBA{}, wx, wy-lift-0.5, tile, spin)
		case an.TameMark:
			bob := motion.Pose{Px: 0.5, Py: 0.5, Sx: 0.5, Sy: 0.5}
			c.DrawSpritePose(v, pass, screen, "overlay/designation_tame", color.NRGBA{}, wx, wy-lift-0.5-0.05*math.Sin(float64(v.Present)*0.08), tile, bob)
		}
		if an.Tame && an.Name != "" && tile >= NameMinTile {
			c.DrawAnimalName(v, cull, screen, an.Name, wx+0.5, wy+1.0)
		}
		if an.Health < 100 {
			sx, sy := v.WorldToScreenF(wx+0.15, wy-BarLift(top))
			bw := float32(tile) * 0.7
			c.solidRect(cull, screen, sx, sy, bw, 4, color.RGBA{30, 24, 20, 220}) // lod.go: a sprite, not a path fill
			c.solidRect(cull, screen, sx, sy, bw*float32(an.Health)/100, 4, c.Host.HealthColor(an.Health))
		}
	}
}

// chaseLineClr is the chase line's colour: the hostile marker's red,
// translucent so it reads without hiding the ground.
var chaseLineClr = color.NRGBA{R: 220, G: 50, B: 40, A: 170}

const (
	// chaseDotStep is the gap between the line's dots, as a share of a
	// tile, and chaseDotSize a dot's side; chaseMaxDots caps a very long
	// line.
	chaseDotStep = 0.22
	chaseDotSize = 0.05
	chaseMaxDots = 120
)

// DrawChaseLines draws a dotted red line from every hostile animal to the
// pawn it is chasing, between the positions they are drawn at (world
// pass, physical pixels). Dots are sprite draws of the white pixel
// (solidRect), not a vector stroke, which would flush the batch for every
// line. A line with both ends off screen is skipped.
func (c *Set) DrawChaseLines(v *render.View, cull *Cull, screen *ebiten.Image, w game.WorldView, tile float64) {
	pad, padS := cull.Pad, cull.PadS
	for i := range w.Animals {
		an := &w.Animals[i]
		if !an.Hostile || an.TargetID == 0 {
			continue
		}
		p, ok := ChasedPawn(v.Pawns, an.TargetID) // chase.go
		if !ok || (!cull.InView(an.X, an.Y, pad, padS) && !cull.InView(p.X, p.Y, 3, 4)) {
			continue
		}
		ax, ay := c.AnimalAt(v, *an)
		px, py := c.pawnAt(v, p)
		x0, y0 := v.WorldToScreenF(ax+0.5, ay+0.5)
		x1, y1 := v.WorldToScreenF(px+0.5, py+0.5)
		if v.Tracing() {
			v.MarkDrawn("chase-line", x0, y0, x1, y1)
		}
		dist := math.Hypot(float64(x1-x0), float64(y1-y0))
		n := min(chaseMaxDots, max(1, int(math.Round(dist/(tile*chaseDotStep)))))
		d := float32(max(2.5, tile*chaseDotSize))
		for k := 0; k <= n; k++ {
			t := float32(k) / float32(n)
			c.solidRect(cull, screen, x0+(x1-x0)*t-d/2, y0+(y1-y0)*t-d/2, d, d, chaseLineClr)
		}
	}
}

// Name plate metrics, in logical pixels like the text they back: the
// padding around the name, the plate's height, and how far the text sits
// above the plate's top edge.
const (
	namePlatePad  = 6
	namePlateH    = 14
	nameTextRaise = 2
)

// NameMinTile is the smallest tile (physical px) a tame animal's name
// shows at: the zoom the stack counts show from (labels.go).
const NameMinTile = 36

// DrawAnimalName puts a tame animal's name on a dark plate, centred
// under the tile point (wx, wy) in tiles. The anchor is a world
// position (physical pixels): the text goes through worldText, which
// converts it to logical once, and the plate is sized in physical pixels
// from the same logical text size, so both scale with the UI together.
func (c *Set) DrawAnimalName(v *render.View, cull *Cull, screen *ebiten.Image, name string, wx, wy float64) {
	sx, sy := v.WorldToScreenF(wx, wy)
	k := float32(gfx.UIScale())
	pw, ph := (gfx.TextW(name, gfx.Small)+namePlatePad)*k, float32(namePlateH)*k
	if v.Tracing() {
		v.MarkDrawn("name-plate", sx-pw/2, sy, pw, ph)
		v.MarkDrawn("name-text", sx, sy-nameTextRaise*k, 0, 0)
	}
	c.solidRect(cull, screen, sx-pw/2, sy, pw, ph, color.RGBA{16, 12, 8, 150})
	st := gfx.Small
	st.Align = gfx.AlignCenter
	gfx.WorldText(screen, name, float64(sx), float64(sy-nameTextRaise*k), st)
}

// Trace styling.
var (
	tracerClr = color.RGBA{255, 226, 140, 255}
	flashClr  = color.RGBA{255, 240, 190, 255}
	bloodClr  = color.RGBA{190, 44, 36, 255}
	swingClr  = color.RGBA{245, 240, 230, 255}
)

// Fade scales a colour (alpha included) by k, clamped to 0..1.
func Fade(c color.RGBA, k float64) color.RGBA {
	if k <= 0 {
		return color.RGBA{}
	}
	if k > 1 {
		k = 1
	}
	return color.RGBA{uint8(float64(c.R) * k), uint8(float64(c.G) * k), uint8(float64(c.B) * k), uint8(float64(c.A) * k)}
}

// Trace effect durations in sim ticks; each also lasts at least
// minReactionFrames present frames (C7).
const (
	TracerTicks = 6
	flashTicks  = 3
	swingTicks  = 10
	bloodTicks  = 12
	puffTicks   = 14
)

// TraceAge is the age an effect of dur ticks is drawn at.
func TraceAge(rt ReactTrace, dur float64) float64 { return reactionAge(rt.Sim, rt.Frames, dur) }

// TraceList is the traces to draw: the tracked ones (kept on screen after
// the sim drops them), or the sim's own when none are tracked (captures).
// Oldest first (newest on top), in a stable order so overlapping effects
// blend the same every frame.
func (c *Set) TraceList(v *render.View, w game.WorldView) []ReactTrace {
	rs := c.Reactions(v.Present, nil)
	if len(rs) == 0 {
		for _, t := range w.Traces {
			born := float64(v.Tick) - float64(t.Age)
			rs = append(rs, ReactTrace{TraceView: t, Sim: float64(t.Age), Frames: math.Inf(1), born: born})
		}
	}
	slices.SortFunc(rs, func(a, b ReactTrace) int {
		ka := [6]float64{-a.Sim, -a.Frames, float64(a.FromX), float64(a.FromY), float64(a.ToX), float64(a.ToY)}
		kb := [6]float64{-b.Sim, -b.Frames, float64(b.FromX), float64(b.FromY), float64(b.ToX), float64(b.ToY)}
		for i := range ka {
			if c := cmp.Compare(ka[i], kb[i]); c != 0 {
				return c
			}
		}
		return 0
	})
	return rs
}

// TraceSpots maps each occupied cell to where its creature is drawn.
func (c *Set) TraceSpots(v *render.View, w game.WorldView, pawns []game.PawnView) map[[2]int]Vec {
	return c.traceSpotsAt(v, w, pawns, nil)
}

// traceSpotsAt is TraceSpots for the cells in want only (nil: every
// cell): a frame samples the motion of just the creatures its traces
// touch, not of every creature on the map.
func (c *Set) traceSpotsAt(v *render.View, w game.WorldView, pawns []game.PawnView, want map[[2]int]bool) map[[2]int]Vec {
	spots := map[[2]int]Vec{}
	put := func(x, y int, id entity.EntityID) {
		if want != nil && !want[[2]int{x, y}] {
			return
		}
		if m, ok := c.motionOf(v, id); ok {
			spots[[2]int{x, y}] = Vec{X: m.X, Y: m.Y}
		}
	}
	for _, p := range pawns {
		if p.CarriedBy == 0 && !p.Lying {
			put(p.X, p.Y, p.ID)
		}
	}
	for i := range w.Animals {
		put(w.Animals[i].X, w.Animals[i].Y, w.Animals[i].ID)
	}
	return spots
}

// TraceEnds is where a trace starts and lands in world tiles: at the drawn
// (interpolated) shooter and target when a creature stands on the cell.
func TraceEnds(spots map[[2]int]Vec, tr game.TraceView) (from, to Vec) {
	from, ok := spots[[2]int{tr.FromX, tr.FromY}]
	if !ok {
		from = Vec{X: float64(tr.FromX), Y: float64(tr.FromY)}
	}
	to, ok = spots[[2]int{tr.ToX, tr.ToY}]
	if !ok {
		to = Vec{X: float64(tr.ToX), Y: float64(tr.ToY)}
	}
	return Vec{X: from.X + 0.5, Y: from.Y + 0.3}, Vec{X: to.X + 0.5, Y: to.Y + 0.4}
}

// DrawTraces renders recent attacks: rifle tracers from the muzzle with
// a muzzle flash and a hit puff, or a trail along the weapon tip for
// melee. They stay visible for at least minReactionFrames frames.
func (c *Set) DrawTraces(v *render.View, screen *ebiten.Image, w game.WorldView, pawns []game.PawnView, tile float64) {
	t := float32(tile)
	rs := c.TraceList(v, w)
	if len(rs) == 0 {
		return
	}
	want := make(map[[2]int]bool, 2*len(rs))
	for _, rt := range rs {
		want[[2]int{rt.FromX, rt.FromY}], want[[2]int{rt.ToX, rt.ToY}] = true, true
	}
	spots := c.traceSpotsAt(v, w, pawns, want)
	for _, rt := range rs {
		tr := rt.TraceView
		from, to := TraceEnds(spots, tr)
		fx, fy := v.WorldToScreenF(from.X, from.Y)
		tx, ty := v.WorldToScreenF(to.X, to.Y)
		if tr.Melee {
			if age := TraceAge(rt, swingTicks); age < swingTicks {
				c.drawMeleeTrail(v, screen, tr, 1-age/swingTicks, tx, ty, t)
			}
			if age := TraceAge(rt, bloodTicks); tr.Hit && age < bloodTicks {
				vector.FillCircle(screen, tx, ty, t*0.08*float32(1+math.Floor(age/4)), Fade(bloodClr, 1-age/bloodTicks), true)
			}
			continue
		}
		dx, dy := tx-fx, ty-fy
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n < 1 {
			n = 1
		}
		ux, uy := dx/n, dy/n
		ex, ey := tx, ty
		if !tr.Hit { // a miss flies past, off to one side
			side := float32(1)
			if (tr.FromX+tr.ToY)%2 == 0 {
				side = -1
			}
			ex, ey = tx+ux*t*0.8-uy*t*0.35*side, ty+uy*t*0.8+ux*t*0.35*side
		}
		mx, my := fx+ux*t*0.5, fy+uy*t*0.5 // muzzle
		if mz, ok := c.TraceMuzzle(v, tr); ok {
			mx, my = v.WorldToScreenF(mz.X, mz.Y)
		}
		if age := TraceAge(rt, TracerTicks); age < TracerTicks {
			k := 1 - age/TracerTicks
			// a short bright streak travelling along the line
			p := float32(age+1) / TracerTicks
			sx, sy := mx+(ex-mx)*(p-0.25), my+(ey-my)*(p-0.25)
			if p < 0.25 {
				sx, sy = mx, my
			}
			p = min(p, 1)
			vector.StrokeLine(screen, mx, my, ex, ey, 1, Fade(tracerClr, k*0.35), true)
			vector.StrokeLine(screen, sx, sy, mx+(ex-mx)*p, my+(ey-my)*p, 3, Fade(tracerClr, k), true)
		}
		if age := TraceAge(rt, flashTicks); age < flashTicks {
			vector.FillCircle(screen, mx, my, t*0.1, Fade(flashClr, 1-age/flashTicks), true)
		}
		if age := TraceAge(rt, puffTicks); tr.Hit && age >= 3 && age < puffTicks {
			k := 1 - (age-3)/(puffTicks-3)
			vector.FillCircle(screen, tx, ty, t*(0.05+0.012*float32(age)), Fade(bloodClr, k), true)
		}
	}
}

// TraceMuzzle is the shooter's muzzle for a shot, in world tiles, when the
// shooter is a drawn pawn holding a rifle.
func (c *Set) TraceMuzzle(v *render.View, tr game.TraceView) (Vec, bool) {
	if tr.FromBuilding { // a sentry turret: its gun's muzzle (turret_draw.go)
		return c.Host.TurretMuzzle(tr.Attacker)
	}
	p, ok := pawnByStrike(v.Pawns, tr)
	if !ok {
		return Vec{}, false
	}
	return c.PawnHeldPoint(v, p, "muzzle")
}

// meleeTrailSteps are the moments a melee trail samples: the end of the
// wind-up (AttackCycle), then strike ages through the follow-through.
var meleeTrailSteps = []struct {
	cycle, age float64
	struck     bool
}{{0.95, 0, false}, {0, 0, true}, {0, 4, true}, {0, 6, true}, {0, 8, true}}

// MeleeTrail is the path of a melee attacker's weapon tip (or fist)
// through its strike motion, in world tiles; nil when the attacker is
// not a drawn pawn or the motion does not move the tip.
func (c *Set) MeleeTrail(v *render.View, tr game.TraceView) []Vec {
	p, ok := pawnByStrike(v.Pawns, tr)
	if !ok || p.Lying || p.CarriedBy != 0 || v.Art == nil {
		return nil
	}
	wx, wy := c.pawnAt(v, p)
	m := c.motion(v, p.ID)
	m.Facing = c.PoseFacing(v, p, m)
	ps, h := c.StandingPose(v, p, m)
	rig := c.bodyRig(v, p, m.Facing)
	if rig.Mirror {
		ps = Mirrored(ps)
	}
	attach := "tip"
	if h.Key == "" {
		attach = "hold"
	}
	var pts []Vec
	lo, hi := Vec{X: math.Inf(1), Y: math.Inf(1)}, Vec{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, st := range meleeTrailSteps {
		_, h.Angle, h.Push = MeleeMotion(p.Weapon, st.cycle, st.age, st.struck)
		pt, ok := c.HeldPoint(v, rig, h, wx, wy, ps, attach)
		if !ok {
			return nil
		}
		pts = append(pts, pt)
		lo = Vec{X: math.Min(lo.X, pt.X), Y: math.Min(lo.Y, pt.Y)}
		hi = Vec{X: math.Max(hi.X, pt.X), Y: math.Max(hi.Y, pt.Y)}
	}
	if math.Hypot(hi.X-lo.X, hi.Y-lo.Y) < 0.08 {
		return nil
	}
	return pts
}

// drawMeleeTrail draws a fading swish along the weapon tip's strike path,
// or a short arc at the target when the path is unknown.
func (c *Set) drawMeleeTrail(v *render.View, screen *ebiten.Image, tr game.TraceView, k float64, tx, ty, t float32) {
	if pts := c.MeleeTrail(v, tr); pts != nil {
		for i := 1; i < len(pts); i++ {
			ax, ay := v.WorldToScreenF(pts[i-1].X, pts[i-1].Y)
			bx, by := v.WorldToScreenF(pts[i].X, pts[i].Y)
			vector.StrokeLine(screen, ax, ay, bx, by, 3, Fade(swingClr, k*float64(i)/float64(len(pts)-1)), true)
		}
		return
	}
	fx, fy := v.WorldToScreenF(float64(tr.FromX)+0.5, float64(tr.FromY)+0.3)
	a0 := math.Atan2(float64(ty-fy), float64(tx-fx)) - 0.9
	for i := 0; i < 3; i++ {
		a1, a2 := a0+float64(i)*0.6, a0+float64(i+1)*0.6
		rad := float64(t) * 0.35
		vector.StrokeLine(screen,
			tx+float32(math.Cos(a1)*rad), ty+float32(math.Sin(a1)*rad),
			tx+float32(math.Cos(a2)*rad), ty+float32(math.Sin(a2)*rad),
			3, Fade(swingClr, k), true)
	}
}

// AnimalFrame is the animal's body frame: a flinch while a hit is
// fresh, the bite or kick (strike frame 3 on the attack's trace, wind-up
// 1-2 by AttackCycle), else what its body player shows (AnimalShow: the
// walk cycle, graze bouts, standing).
func (c *Set) AnimalFrame(v *render.View, an game.AnimalView) string {
	m := c.motion(v, an.ID)
	show := c.AnimalShow(v, an) // advanced every draw, under a flinch or strike too
	if age, ok := c.HitAge(v, an.ID, an.HitTick, an.X, an.Y); ok && age < StaggerTicks {
		if age < 4 {
			return "_flinch1"
		}
		return "_flinch2"
	}
	st, age, struck := c.LastStrike(v, an.ID, an.X, an.Y, MeleeTicks)
	moving := m.Blend >= motion.MinBlend
	if set := animalAttackSet(an, st, struck); set != "" {
		switch {
		case struck && age < 3:
			return set + "3"
		case struck && age < MeleeTicks:
			return set + "4"
		case !moving && attackAction(an.Action):
			if an.AttackCycle < aimAt {
				return set + "1"
			}
			return set + "2"
		}
	}
	return show.Suffix
}

// attackAction reports an Action that is a natural-weapon attack.
func attackAction(a string) bool { return a == game.ActionBite || a == game.ActionKick }

// animalAttackSet is the attack pose set while the animal attacks or just
// struck: its natural weapon's set ("_claw", "_gore", ...). A view without
// an attack kind (an older sim) falls back to the action, then the weapon
// of its last strike.
func animalAttackSet(an game.AnimalView, st ReactTrace, struck bool) string {
	if !struck && !attackAction(an.Action) {
		return ""
	}
	switch {
	case an.Attack != "":
		return "_" + an.Attack
	case an.Action == game.ActionKick:
		return "_kick"
	case an.Action == game.ActionBite:
		return "_bite"
	case st.Weapon == "hooves":
		return "_kick"
	}
	return "_bite"
}

// Overlay heights over an animal, in tiles above its tile's top edge
// (PLAY-022 C8). The height is how far the drawn body rises above that
// edge: from its art (animalTop: the standing canvas's top) or, without
// art, estimated from its body length by animalHeights, the class
// medians of the PLAY-022 sprites at the class lengths (XS 1.45 … XL
// 3.0). Marks float animalMarkAbove over it, the health bar animalBarAbove.
const (
	animalRefSize   = 1.8 // an unknown size counts as an M body (a deer)
	animalMinMark   = 0.55
	animalMinBar    = 0.1
	AnimalMarkAbove = 0.4 // a mark floats at least this far over the height
	animalBarAbove  = 0.1 // the bar's top edge sits this far over the height
)

// animalHeights are (body length, height above the tile's top) points of
// the size → height estimate: the class medians of the PLAY-022 standing
// canvases (XS 0.3, S 0.5, M 0.8, L 1.45, XL 1.7 tiles).
var animalHeights = [...][2]float64{{1.45, 0.3}, {1.65, 0.5}, {1.8, 0.8}, {2.2, 1.45}, {3.0, 1.7}}

// AnimalHeight is an animal's estimated height above its tile's top in
// tiles from its body length: linear between the class points, clamped
// past the ends (an unknown size counts as the reference M body).
func AnimalHeight(size float64) float64 {
	if size <= 0 {
		size = animalRefSize
	}
	hs := animalHeights
	if size <= hs[0][0] {
		return hs[0][1] * size / hs[0][0]
	}
	for i := 1; i < len(hs); i++ {
		if size <= hs[i][0] {
			a, b := hs[i-1], hs[i]
			return a[1] + (b[1]-a[1])*(size-a[0])/(b[0]-a[0])
		}
	}
	last := hs[len(hs)-1]
	return last[1] * size / last[0]
}

// snoreDrop is where a sleeper's snores start below the top of its lying
// body (drawSnoreFor rises from 0.25 above its anchor).
const snoreDrop = 0.25

// AnimalRestTop is how far a lying animal's body rises above its tile's
// top edge: its rest frame's opaque top (that sprite is drawn, so it is
// decoded or measured), else half the size estimate.
func (c *Set) AnimalRestTop(v *render.View, an game.AnimalView) float64 {
	if sp := v.Art.Sprite(c.AnimalBaseKey(v, an) + "_rest"); sp != nil {
		_, y0, _, _ := sp.OpaqueBox()
		return (sp.Meta.Origin[1] - y0) / sp.Meta.PxPerTile()
	}
	return 0.5 * AnimalHeight(an.Size)
}

// MarkerLift is how far above its tile's top a mark hovers over a body h
// tiles high; barLift the health bar.
func MarkerLift(h float64) float64 { return max(animalMinMark, h+AnimalMarkAbove) }
func BarLift(h float64) float64    { return max(animalMinBar, h+animalBarAbove) }

// AnimalMarkerLift is how far above its tile the hunt and tame marks
// hover over a body of this length, without art.
func AnimalMarkerLift(size float64) float64 { return MarkerLift(AnimalHeight(size)) }

// AnimalBarLift is how far above its tile the health bar sits, without art.
func AnimalBarLift(size float64) float64 { return BarLift(AnimalHeight(size)) }

// AnimalTop is how far an animal's drawn body rises above its tile's top
// edge, in tiles, at camera tile size tile: its standing canvas's top (the
// union of the facing's poses, so no frame pokes through a mark) or the
// size estimate without art, shrunk for a juvenile about its feet and
// grown by the minimum creature size (minTile, creatureAt) about the
// tile's foot.
func (c *Set) AnimalTop(v *render.View, an game.AnimalView, tile, minTile float64) float64 {
	h := AnimalHeight(an.Size)
	if m := v.Art.MetaOf(c.AnimalBaseKey(v, an)); m != nil {
		h = m.Origin[1] / m.PxPerTile()
	}
	if s := an.Scale; s > 0 && s < 1 {
		h = (h+motion.FigurePivotY)*s - motion.FigurePivotY
	}
	if ct := max(tile, minTile); tile > 0 && ct > tile {
		h = (h+1)*ct/tile - 1
	}
	return h
}

// Animal motion on top of the frames (PLAY-022 B.3): the baked walk carries
// the body's rise and the head's nod, so the renderer adds no bob; a run
// leans forward a little; standing, lying and grazing bodies breathe.
const (
	RunLean = 0.035 // radians at full run speed (east/west)
	// Breathe: the body's top moves breatheBase + breatheK x its height
	// (tiles), once per breathePeriod + breathePeriodK x height frames, so
	// a rat's breath shows as well as an elephant's.
	BreatheBase    = 0.02
	breatheK       = 0.01
	breathePeriod  = 90
	breathePeriodK = 30
)

// Breathe is the idle breath's vertical scale about the feet for a body h
// tiles from its feet to its top, at present frame p.
func Breathe(h float64, p uint64, id entity.EntityID) float64 {
	h = math.Max(h, 0.2)
	period := breathePeriod + breathePeriodK*h
	return 1 + (BreatheBase+breatheK*h)/h*math.Sin(2*math.Pi*float64(p+uint64(id)*29)/period)
}

// AnimalPose: idle breathing, a slight lean into a run, a lunge into each
// bite or kick, and a stagger away from the attacker when hit.
func (c *Set) AnimalPose(v *render.View, an game.AnimalView) motion.Pose {
	ps := motion.Pose{Px: motion.FigurePivotX, Py: motion.FigurePivotY}
	m := c.motion(v, an.ID)
	if m.Blend >= motion.MinBlend {
		if an.Gait == game.GaitRun {
			switch m.Facing {
			case art.FaceEast:
				ps.Rot = RunLean * motion.GaitScale(m)
			case art.FaceWest:
				ps.Rot = -RunLean * motion.GaitScale(m)
			}
		}
	} else {
		ps.Sy = Breathe(c.AnimalTop(v, an, 0, 0)+motion.FigurePivotY, v.Present, an.ID)
	}
	if st, age, struck := c.LastStrike(v, an.ID, an.X, an.Y, MeleeTicks); struck && age < MeleeTicks {
		k := math.Sin(math.Pi * age / MeleeTicks)
		ux, uy := motion.Unit(float64(st.ToX-st.FromX), float64(st.ToY-st.FromY))
		ps.Dx, ps.Dy = ps.Dx+ux*0.2*k, ps.Dy+uy*0.2*k
	}
	if an.HitTick == 0 {
		dx, dy, fl := struckNudge(c.Reactions(v.Present, nil), an.X, an.Y)
		ps.Dx, ps.Dy, ps.Flash = ps.Dx+dx, ps.Dy+dy, fl
		return ps
	}
	if age, ok := c.HitAge(v, an.ID, an.HitTick, an.X, an.Y); ok && age < hitTicks {
		ux, uy := awayFrom(an.X, an.Y, an.HitFromX, an.HitFromY, m.Facing)
		k := staggerK(age)
		ps.Dx, ps.Dy = ps.Dx+ux*StaggerDist*k, ps.Dy+uy*StaggerDist*k
		ps.Flash = float32(1 - age/hitTicks)
	}
	return ps
}
