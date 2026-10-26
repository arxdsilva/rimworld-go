package ebiten

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// stackCountMinTile: below this tile size (px) counts would just clutter.
const stackCountMinTile = 36

// drawStackCounts prints each ground stack's quantity in the bottom-right
// corner of its tile, on a small dark tag so it reads over any sprite.
func (r *Renderer) drawStackCounts(screen *ebiten.Image, w game.WorldView, tile float64) {
	if tile < stackCountMinTile {
		return
	}
	for i := range w.Items {
		it := &w.Items[i]
		if it.Carried || it.Qty <= 1 || !r.lod.inView(it.X, it.Y, 1, 1) { // lod.go: off screen
			continue
		}
		s := strconv.Itoa(it.Qty)
		sx, sy := r.worldToScreen(it.X, it.Y)
		// the tag is UI chrome anchored to a world tile: place it in
		// logical pixels (world position / ui scale), text and rect via the canvas
		k := float32(1 / uiScale())
		tw := textW(s, tsSmall) + 6
		th := lineStep(tsSmall) - 2
		x := sx*k + float32(tile)*k - tw - 2
		y := sy*k + float32(tile)*k - th - 2
		uiFillRect(screen, x, y, tw, th, uiShade(190), false)
		drawText(screen, s, float64(x)+3, float64(y)-1, tsSmall)
	}
}

// interactBubbleTicks is how long a social interaction's bubble stays over
// the colonist after InteractTick.
const interactBubbleTicks = 300

// interactionKinds are the interaction kinds that have a bubble icon
// (ui_icon/social_<kind>).
var interactionKinds = map[string]bool{
	"chitchat": true, "deep_talk": true, "slight": true, "insult": true, "kind_words": true,
	"romance": true, "rebuff": true, "proposal": true, "breakup": true, "fight": true, "lovin": true,
}

// interactionIconKey is the art key of an interaction kind's icon ("" for
// an unknown kind).
func interactionIconKey(kind string) string {
	if !interactionKinds[kind] {
		return ""
	}
	return "ui_icon/social_" + kind
}

// interactionRecent reports p has a social interaction whose bubble is
// still showing at tick now.
func interactionRecent(p game.PawnView, now clock.Tick) bool {
	if p.Interaction == "" {
		return false
	}
	t := uint64(now)
	return t < p.InteractTick || t-p.InteractTick <= interactBubbleTicks
}

// drawChatBubbles floats a small speech bubble over colonists chatting and
// over colonists with a recent social interaction (with the interaction's
// icon when the art has it), and a red lightning marker over colonists in
// a social fight.
func (r *Renderer) drawChatBubbles(screen *ebiten.Image, tile float64) {
	now := r.game.Tick()
	for _, p := range r.drawPawns {
		if p.SocialFight {
			wx, wy := r.interpWorld(p)
			r.drawFightMarker(screen, wx, wy, tile)
		}
		social := interactionRecent(p, now)
		if !social && !strings.HasPrefix(p.Job, "chat do") {
			continue
		}
		// chat bubbles come and go so two chatting colonists take turns
		if !social && (r.present/50+uint64(p.ID))%2 == 1 {
			continue
		}
		wx, wy := r.interpWorld(p)
		sx, sy := r.worldToScreenF(wx+0.75, wy-0.55)
		bw, bh := float32(tile*0.42), float32(tile*0.24)
		var icon *ebiten.Image
		if social {
			if key := interactionIconKey(p.Interaction); key != "" {
				icon = r.art.image(key, noTint)
			}
			if icon != nil {
				bh = bw // a square bubble around the icon
			}
		}
		vector.FillRect(screen, sx-bw/2, sy-bh/2, bw, bh, color.RGBA{250, 248, 240, 235}, true)
		vector.StrokeRect(screen, sx-bw/2, sy-bh/2, bw, bh, 1, color.RGBA{60, 50, 40, 220}, true)
		vector.FillCircle(screen, sx-bw/2+bw*0.2, sy+bh/2+2, float32(tile*0.035), color.RGBA{250, 248, 240, 235}, true)
		if icon != nil {
			// physical anchor and size (the bubble is world-sized): the
			// icon size is logical, so divide the physical size by the UI scale
			worldIcon(screen, icon, float64(sx-bw/2)+2, float64(sy-bh/2)+2, float64(bw-4)/uiScale())
			continue
		}
		for i := -1; i <= 1; i++ {
			vector.FillCircle(screen, sx+float32(i)*bw*0.25, sy, float32(tile*0.025), color.RGBA{70, 60, 50, 255}, true)
		}
	}
}

// drawFightMarker floats a red lightning bolt over a colonist in a social
// fight (overlay/fight_lightning; vector bolt while the art is missing).
func (r *Renderer) drawFightMarker(screen *ebiten.Image, wx, wy, tile float64) {
	sx, sy := r.worldToScreenF(wx+0.5, wy-0.95)
	if img := r.art.image("overlay/fight_lightning", noTint); img != nil {
		size := tile * 0.32 // physical, like the position; worldIcon takes it logical
		worldIcon(screen, img, float64(sx)-size/2, float64(sy)-size/2, size/uiScale())
		return
	}
	c := color.RGBA{220, 40, 36, 245}
	h, w := float32(tile*0.12), float32(tile*0.06)
	th := float32(tile * 0.035)
	vector.StrokeLine(screen, sx+w, sy-h, sx-w, sy, th, c, true)
	vector.StrokeLine(screen, sx-w, sy, sx+w, sy, th, c, true)
	vector.StrokeLine(screen, sx+w, sy, sx-w, sy+h, th, c, true)
}

// drawBreakClouds floats a small dark storm cloud over colonists having a
// mental break.
func (r *Renderer) drawBreakClouds(screen *ebiten.Image, tile float64) {
	for _, p := range r.drawPawns {
		if p.MentalBreak == "" {
			continue
		}
		wx, wy := r.interpWorld(p)
		bob := math.Sin(float64(r.present)/14+float64(p.ID)) * 0.04
		sx, sy := r.worldToScreenF(wx+0.5, wy-1.0+bob)
		c := color.RGBA{70, 70, 84, 230}
		for _, o := range [][3]float64{{-0.12, 0.02, 0.10}, {0.0, -0.03, 0.13}, {0.13, 0.02, 0.10}} {
			vector.FillCircle(screen, sx+float32(o[0]*tile), sy+float32(o[1]*tile), float32(o[2]*tile), c, true)
		}
		if (r.present/8+uint64(p.ID))%6 == 0 { // a flicker of lightning
			vector.StrokeLine(screen, sx, sy+float32(tile*0.08), sx-float32(tile*0.04), sy+float32(tile*0.2), 2, color.RGBA{250, 230, 120, 255}, true)
		}
	}
}

// drawHostileMarker floats a red chevron over a raider (orange while it
// flees).
func (r *Renderer) drawHostileMarker(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64) {
	c := color.RGBA{220, 50, 40, 240}
	if p.Fleeing {
		c = color.RGBA{230, 150, 50, 220}
	}
	sx, sy := r.worldToScreenF(wx+0.5, wy-0.95)
	w, h := float32(tile*0.13), float32(tile*0.1)
	th := float32(tile * 0.06)
	vector.StrokeLine(screen, sx-w, sy-h, sx, sy+h, th, c, true)
	vector.StrokeLine(screen, sx, sy+h, sx+w, sy-h, th, c, true)
}

// drawBuildingDamage puts a small health bar under damaged buildings.
func (r *Renderer) drawBuildingDamage(screen *ebiten.Image, w game.WorldView, tile float64) {
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.Floor || b.HPPct >= 100 || b.HPPct <= 0 || !r.lod.inView(b.X, b.Y, 4, 1) { // lod.go: off screen
			continue
		}
		sx, sy := r.worldToScreen(b.X, b.Y)
		bw := float32(tile) * float32(max(1, b.W)) * 0.8
		x := sx + (float32(tile)*float32(max(1, b.W))-bw)/2
		y := sy + float32(tile)*float32(max(1, b.H)) - 7
		vector.FillRect(screen, x-1, y-1, bw+2, 6, uiShade(210), false)
		c := uiGood
		switch {
		case b.HPPct < 35:
			c = uiDanger
		case b.HPPct < 70:
			c = uiWarning
		}
		vector.FillRect(screen, x, y, bw*float32(b.HPPct)/100, 4, c, false)
	}
}
