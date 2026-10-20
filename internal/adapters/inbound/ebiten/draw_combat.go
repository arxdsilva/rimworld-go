package ebiten

import (
	"cmp"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Presentation of animals, held weapons and attack traces. Everything
// here reads the WorldView/PawnView; nothing writes back.

// syncAnimals advances per-animal presentation state (motion, facing).
// Footstep dust comes from animate, at the feet on contact frames.
func (r *Renderer) syncAnimals(w game.WorldView) {
	tick := r.game.Tick()
	running := r.mo.stepped > 0
	for _, an := range w.Animals {
		prev, seen := r.last[an.ID]
		v := nextVisualStep(prev, seen, an.X, an.Y, tick, r.present, an.StepTicks)
		k := r.mo.observe(an.ID, animalMoView(an), v.facing)
		want, moving := k.wantFacing(r.mo.ref)
		if !moving {
			want = faceAttack(pawnVisual{facing: want}, w.Traces, an.X, an.Y).facing
		}
		v.facing = k.turn(want, an.ID, running)
		r.last[an.ID] = v
	}
}

// faceAttack turns an attacker standing at (x, y) towards its latest
// target so weapons and bites point the right way.
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			v.facing = facingFor(t.ToX-x, t.ToY-y, v.facing)
		}
	}
	return v
}

// interpAnimal is where an animal is drawn: its motion at animT, or its
// tile before the first sync.
func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	m, ok := r.motionOf(an.ID)
	if !ok {
		return float64(an.X), float64(an.Y)
	}
	return m.X, m.Y
}

// addAnimalDraws queues animals into the depth-sorted object pass.
func (r *Renderer) addAnimalDraws(w game.WorldView, add func(row, col float64, fn func()), screen *ebiten.Image, tile float64, sw, sh int) {
	for _, an := range w.Animals {
		an := an
		wx, wy := r.interpAnimal(an)
		add(wy+0.01, wx, func() {
			vis := r.last[an.ID]
			key := "animal/" + an.Species + "_" + vis.facing.String()
			if !r.art.has(key) {
				sx, sy := r.worldToScreenF(wx, wy)
				vector.FillCircle(screen, sx+float32(tile)/2, sy+float32(tile)/2, float32(tile)*0.3, color.RGBA{150, 120, 80, 255}, false)
				return
			}
			r.drawSpritePose(screen, r.framed(key, r.animalFrame(an)), color.NRGBA{}, wx, wy, tile, r.animalPose(an))
		})
	}
}

// drawAnimalOverlays draws hunt marks and health bars above animals.
func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	for _, an := range w.Animals {
		wx, wy := r.interpAnimal(an)
		switch {
		case an.Manhunter:
			r.drawHostileMarker(screen, game.PawnView{}, wx, wy, tile)
		case an.Designated:
			spin := pose{px: 0.5, py: 0.5, sx: 0.5, sy: 0.5, rot: float64(r.present) * 0.035}
			r.drawSpritePose(screen, "overlay/designation_hunt", color.NRGBA{}, wx, wy-0.95, tile, spin)
		case an.TameMark:
			bob := pose{px: 0.5, py: 0.5, sx: 0.5, sy: 0.5}
			r.drawSpritePose(screen, "overlay/designation_tame", color.NRGBA{}, wx, wy-0.95-0.05*math.Sin(float64(r.present)*0.08), tile, bob)
		}
		if an.Tame && an.Name != "" && tile >= stackCountMinTile {
			sx, sy := r.worldToScreenF(wx+0.5, wy+1.0)
			tw := float32(len(an.Name)*6 + 6)
			vector.FillRect(screen, sx-tw/2, sy, tw, 14, color.RGBA{16, 12, 8, 150}, false)
			ebitenutil.DebugPrintAt(screen, an.Name, int(sx-tw/2)+3, int(sy)-1)
		}
		if an.Health < 100 {
			sx, sy := r.worldToScreenF(wx+0.15, wy-0.32)
			bw := float32(tile) * 0.7
			vector.FillRect(screen, sx, sy, bw, 4, color.RGBA{30, 24, 20, 220}, false)
			vector.FillRect(screen, sx, sy, bw*float32(an.Health)/100, 4, healthColor(an.Health), false)
		}
	}
}

// Trace styling.
var (
	tracerClr = color.RGBA{255, 226, 140, 255}
	flashClr  = color.RGBA{255, 240, 190, 255}
	bloodClr  = color.RGBA{190, 44, 36, 255}
	swingClr  = color.RGBA{245, 240, 230, 255}
)

func fade(c color.RGBA, k float64) color.RGBA {
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
	tracerTicks = 6
	flashTicks  = 3
	swingTicks  = 10
	bloodTicks  = 12
	puffTicks   = 14
)

// traceAge is the age an effect of dur ticks is drawn at.
func traceAge(rt reactTrace, dur float64) float64 { return reactionAge(rt.sim, rt.frames, dur) }

// traceList is the traces to draw: the tracked ones (kept on screen after
// the sim drops them), or the sim's own when none are tracked (captures).
// Oldest first (newest on top), in a stable order so overlapping effects
// blend the same every frame.
func (r *Renderer) traceList(w game.WorldView) []reactTrace {
	rs := r.animState().reactions(r.present, nil)
	if len(rs) == 0 {
		for _, t := range w.Traces {
			rs = append(rs, reactTrace{TraceView: t, sim: float64(t.Age), frames: math.Inf(1)})
		}
	}
	slices.SortFunc(rs, func(a, b reactTrace) int {
		ka := [6]float64{-a.sim, -a.frames, float64(a.FromX), float64(a.FromY), float64(a.ToX), float64(a.ToY)}
		kb := [6]float64{-b.sim, -b.frames, float64(b.FromX), float64(b.FromY), float64(b.ToX), float64(b.ToY)}
		for i := range ka {
			if c := cmp.Compare(ka[i], kb[i]); c != 0 {
				return c
			}
		}
		return 0
	})
	return rs
}

// traceSpots maps each occupied cell to where its creature is drawn.
func (r *Renderer) traceSpots(w game.WorldView) map[[2]int]vec {
	spots := map[[2]int]vec{}
	put := func(x, y int, id entity.EntityID) {
		if m, ok := r.motionOf(id); ok {
			spots[[2]int{x, y}] = vec{m.X, m.Y}
		}
	}
	for _, p := range r.game.Snapshot() {
		if p.CarriedBy == 0 && !p.Lying {
			put(p.X, p.Y, p.ID)
		}
	}
	for _, an := range w.Animals {
		put(an.X, an.Y, an.ID)
	}
	return spots
}

// traceEnds is where a trace starts and lands in world tiles: at the drawn
// (interpolated) shooter and target when a creature stands on the cell.
func traceEnds(spots map[[2]int]vec, tr game.TraceView) (from, to vec) {
	from, ok := spots[[2]int{tr.FromX, tr.FromY}]
	if !ok {
		from = vec{float64(tr.FromX), float64(tr.FromY)}
	}
	to, ok = spots[[2]int{tr.ToX, tr.ToY}]
	if !ok {
		to = vec{float64(tr.ToX), float64(tr.ToY)}
	}
	return vec{from.x + 0.5, from.y + 0.3}, vec{to.x + 0.5, to.y + 0.4}
}

// drawTraces renders recent attacks: rifle tracers with a muzzle flash
// and a hit puff, or a short swing arc for melee. They start at the drawn
// shooter and stay visible for at least minReactionFrames frames.
func (r *Renderer) drawTraces(screen *ebiten.Image, w game.WorldView, tile float64) {
	t := float32(tile)
	rs := r.traceList(w)
	if len(rs) == 0 {
		return
	}
	spots := r.traceSpots(w)
	for _, rt := range rs {
		tr := rt.TraceView
		from, to := traceEnds(spots, tr)
		fx, fy := r.worldToScreenF(from.x, from.y)
		tx, ty := r.worldToScreenF(to.x, to.y)
		if tr.Melee {
			if age := traceAge(rt, swingTicks); age < swingTicks {
				k := 1 - age/swingTicks
				a0 := math.Atan2(float64(ty-fy), float64(tx-fx)) - 0.9
				for i := 0; i < 3; i++ {
					a1, a2 := a0+float64(i)*0.6, a0+float64(i+1)*0.6
					rad := float64(t) * 0.35
					vector.StrokeLine(screen,
						tx+float32(math.Cos(a1)*rad), ty+float32(math.Sin(a1)*rad),
						tx+float32(math.Cos(a2)*rad), ty+float32(math.Sin(a2)*rad),
						3, fade(swingClr, k), true)
				}
			}
			if age := traceAge(rt, bloodTicks); tr.Hit && age < bloodTicks {
				vector.FillCircle(screen, tx, ty, t*0.08*float32(1+math.Floor(age/4)), fade(bloodClr, 1-age/bloodTicks), true)
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
		if age := traceAge(rt, tracerTicks); age < tracerTicks {
			k := 1 - age/tracerTicks
			// a short bright streak travelling along the line
			p := float32(age+1) / tracerTicks
			sx, sy := mx+(ex-mx)*(p-0.25), my+(ey-my)*(p-0.25)
			if p < 0.25 {
				sx, sy = mx, my
			}
			p = min(p, 1)
			vector.StrokeLine(screen, mx, my, ex, ey, 1, fade(tracerClr, k*0.35), true)
			vector.StrokeLine(screen, sx, sy, mx+(ex-mx)*p, my+(ey-my)*p, 3, fade(tracerClr, k), true)
		}
		if age := traceAge(rt, flashTicks); age < flashTicks {
			vector.FillCircle(screen, mx, my, t*0.1, fade(flashClr, 1-age/flashTicks), true)
		}
		if age := traceAge(rt, puffTicks); tr.Hit && age >= 3 && age < puffTicks {
			k := 1 - (age-3)/(puffTicks-3)
			vector.FillCircle(screen, tx, ty, t*(0.05+0.012*float32(age)), fade(bloodClr, k), true)
		}
	}
}

// nextVisualStep is nextVisual for movers with their own pace.
func nextVisualStep(prev pawnVisual, seen bool, x, y int, tick clock.Tick, present uint64, step int) pawnVisual {
	v := nextVisual(prev, seen, x, y, tick, present)
	if seen && prev.x == x && prev.y == y && tick-prev.moveTick < clock.Tick(max(step, 1)) {
		v.fromX, v.fromY = prev.fromX, prev.fromY
	}
	return v
}

// settleVisuals finishes every in-flight step. Captures freeze the
// simulation, so without this a mover is drawn mid-step.
func (r *Renderer) settleVisuals() {
	for id, v := range r.last {
		v.fromX, v.fromY = v.x, v.y
		r.last[id] = v
	}
	for _, k := range r.mo.tracks {
		k.settle()
	}
}

// animalFrame is the trot frame for a moving animal ("" standing).
func (r *Renderer) animalFrame(an game.AnimalView) string {
	return trotSuffix(r.animalMotion(an.ID))
}

// animalPose: a trot bob (more when running), idle breathing, bite
// lunges and hit reactions.
func (r *Renderer) animalPose(an game.AnimalView) pose {
	ps := pose{px: figurePivotX, py: figurePivotY}
	m := r.animalMotion(an.ID)
	if m.Blend >= minBlend {
		ps.dy = -trotBob(m, an.Mode != "wandering")
	} else {
		ps.sy = 1 + 0.015*math.Sin(2*math.Pi*float64(r.present+uint64(an.ID)*29)/120)
	}
	dx, dy, fl := r.reaction(an.X, an.Y)
	ps.dx, ps.dy, ps.flash = ps.dx+dx, ps.dy+dy, fl
	return ps
}
