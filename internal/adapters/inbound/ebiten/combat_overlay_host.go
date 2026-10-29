package ebiten

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
)

// DEF-1 combat feedback on the colony map: the weapon range ring and the
// hit-chance card of the selected drafted colonist, the cover overlay and
// the projectiles in flight. The draws live in render/overlay; this file
// feeds them from the read models (WorldView.Shots, HitChance) and the
// map cursor, in map pixels.

// coverRadius is how far from the selected colonist the cover overlay marks
// tiles, in tiles.
const coverRadius = 14

// drawCombatOverlay draws the combat feedback into the map's target (a
// plain function, not an App method: App is at its method budget).
func drawCombatOverlay(a *App, dst *ebiten.Image) {
	r := a.renderer
	if r == nil || r.game == nil || r.art == nil {
		return
	}
	set := &r.overlays().Combat
	v := r.liveView()
	w := r.game.World()
	set.Marks = projectileMarks(set.Marks[:0], w.Shots, projectileAlpha(v.Alpha, v.Paused))
	set.DrawProjectiles(v, dst, set.Marks)
	sel, ok := selectedColonist(r)
	if !ok {
		return
	}
	if set.Cover.Shown() {
		set.Cells = coverCells(set.Cells[:0], w.Buildings, sel.X, sel.Y)
		set.DrawCover(v, dst, set.Cells)
	}
	if !sel.Drafted {
		return
	}
	set.DrawWeaponRange(v, dst, sel.X, sel.Y, weaponReach(sel.Weapon))
	mx, my := a.mapCursor()
	tx, ty := r.TileAt(mx, my)
	for _, h := range r.hostiles() {
		if h.X != tx || h.Y != ty {
			continue
		}
		if rep, ok := r.game.HitChance(sel.ID, h.ID); ok {
			info := shotInfo(set.Factors[:0], rep, sel.X, sel.Y, h.X, h.Y)
			set.Factors = info.Factors
			set.DrawShotFeedback(v, dst, &info, mx, my)
		}
		break
	}
}

// selectedColonist is the selected colonist's view (a plain function:
// Renderer is at its method budget).
func selectedColonist(r *Renderer) (game.PawnView, bool) {
	if !r.HasSelect() {
		return game.PawnView{}, false
	}
	for _, p := range r.snapshot() {
		if p.ID == r.Selected() {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// projectileAlpha is the share of a tick to advance projectiles by: none
// while paused, where the sim does not move.
func projectileAlpha(alpha float64, paused bool) float64 {
	if paused {
		return 0
	}
	return min(max(alpha, 0), 1)
}

// projectileMarks are the shots in flight in world tiles, advanced alpha of
// a tick along their velocity (positions are thousandths of a tile).
func projectileMarks(out []overlay.ProjectileMark, shots []projectile.View, alpha float64) []overlay.ProjectileMark {
	for _, s := range shots {
		dx, dy := float64(s.VX), float64(s.VY)
		if dx == 0 && dy == 0 {
			dx, dy = float64(s.ToX-s.FromX), float64(s.ToY-s.FromY)
		}
		if d := math.Hypot(dx, dy); d > 0 {
			dx, dy = dx/d, dy/d
		}
		out = append(out, overlay.ProjectileMark{
			X:  (float64(s.X) + 500 + float64(s.VX)*alpha) / 1000,
			Y:  (float64(s.Y) + 500 + float64(s.VY)*alpha) / 1000,
			DX: dx, DY: dy, Kind: "bullet",
		})
	}
	return out
}

// weaponReach is the reach of the weapon named name (fists when unknown).
func weaponReach(name string) overlay.WeaponReach {
	for k := weapon.Kind(0); k < 255; k++ {
		if d := weapon.DefOf(k); d.Name == name {
			return overlay.WeaponReach{Max: float64(d.Range)}
		}
	}
	return overlay.WeaponReach{}
}

// coverCells are the cover the buildings give near (x, y), by the sim's
// own values (construction.CoverPct).
func coverCells(out []overlay.CoverCell, bs []game.BuildingView, x, y int) []overlay.CoverCell {
	for _, b := range bs {
		if b.Floor || b.X < x-coverRadius || b.X > x+coverRadius || b.Y < y-coverRadius || b.Y > y+coverRadius {
			continue
		}
		k, ok := construction.Parse(b.Kind)
		if !ok {
			continue
		}
		if pct := construction.CoverPct(k); pct > 0 {
			out = append(out, overlay.CoverCell{X: b.X, Y: b.Y, Value: pct})
		}
	}
	return out
}

// shotInfo is the hit-chance card of a report: the chance with each
// factor's effect, or why the shot is blocked.
func shotInfo(factors []overlay.ShotFactor, rep attack.Report, fx, fy, tx, ty int) overlay.ShotInfo {
	s := overlay.ShotInfo{FromX: fx, FromY: fy, ToX: tx, ToY: ty, Chance: int(rep.Total()) / 10, Factors: factors}
	switch {
	case !rep.InRange:
		s.Blocked, s.Reason = true, "out of range"
	case !rep.InSight:
		s.Blocked, s.Reason = true, "no line of sight"
	}
	if rep.Melee {
		return s
	}
	add := func(label string, permille int) {
		if p := permille / 10; p != 0 {
			s.Factors = append(s.Factors, overlay.ShotFactor{Label: label, Pct: p})
		}
	}
	add("shooter", int(rep.Shooter)-1000)
	add("target size", int(rep.Size)-1000)
	add("light", int(rep.Light)-1000)
	add("weather", int(rep.Weather)-1000)
	add("cover", -int(rep.Cover))
	return s
}
