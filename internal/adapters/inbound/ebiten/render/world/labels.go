package world

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// StackCountMinTile: below this tile size (px) counts would just clutter.
const StackCountMinTile = 36

// drawStackCounts prints each ground stack's quantity in the bottom-right
// corner of its tile, on a small dark tag so it reads over any sprite.
func (lb *Labels) drawStackCounts(screen *ebiten.Image, w game.WorldView, tile float64) {
	if tile < StackCountMinTile {
		return
	}
	for i := range w.Items {
		it := &w.Items[i]
		if it.Carried || it.Qty <= 1 || !lb.LOD.InView(it.X, it.Y, 1, 1) { // lod.go: off screen
			continue
		}
		s := strconv.Itoa(it.Qty)
		sx, sy := lb.worldToScreen(it.X, it.Y)
		// the tag is UI chrome anchored to a world tile: place it in
		// logical pixels (world position / ui scale), text and rect via the canvas
		k := float32(1 / gfx.UIScale())
		tw := gfx.TextW(s, gfx.Small) + 6
		th := gfx.LineStep(gfx.Small) - 2
		x := sx*k + float32(tile)*k - tw - 2
		y := sy*k + float32(tile)*k - th - 2
		gfx.FillRect(screen, x, y, tw, th, uikit.Shade(190), false)
		gfx.Text(screen, s, float64(x)+3, float64(y)-1, gfx.Small)
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

// InteractionIconKey is the art key of an interaction kind's icon ("" for
// an unknown kind).
func InteractionIconKey(kind string) string {
	if !interactionKinds[kind] {
		return ""
	}
	return "ui_icon/social_" + kind
}

// InteractionRecent reports p has a social interaction whose bubble is
// still showing at tick now.
func InteractionRecent(p game.PawnView, now clock.Tick) bool {
	if p.Interaction == "" {
		return false
	}
	t := uint64(now)
	return t < p.InteractTick || t-p.InteractTick <= interactBubbleTicks
}

// DrawChatBubbles floats a speech bubble over colonists chatting and over
// colonists with a recent social interaction, and a red lightning marker
// over colonists in a social fight. The bubble is the interaction's round
// icon (the chit-chat one for a plain chat) with nothing behind it
// (PLAY-020); without art it is a small dark disc with three dots.
func (lb *Labels) DrawChatBubbles(screen *ebiten.Image, tile float64) {
	v := lb.v
	now := v.Tick
	for _, p := range v.Pawns {
		if p.SocialFight {
			wx, wy := lb.interpWorld(p)
			lb.drawFightMarker(screen, wx, wy, tile)
		}
		social := InteractionRecent(p, now)
		if !social && !strings.HasPrefix(p.Job, "chat do") {
			continue
		}
		// chat bubbles come and go so two chatting colonists take turns
		if !social && (v.Present/50+uint64(p.ID))%2 == 1 {
			continue
		}
		wx, wy := lb.interpWorld(p)
		_, hy := creature.PawnHead(lb.art())
		sx, sy := lb.worldToScreenF(wx+0.75, wy+hy-ChatAboveHead)
		bw := float32(tile * 0.42)
		key := "ui_icon/social_chitchat" // a plain chat
		if social {
			key = InteractionIconKey(p.Interaction)
		}
		var icon *ebiten.Image
		if key != "" {
			icon = lb.art().Image(key, color.NRGBA{})
		}
		if icon != nil {
			// physical anchor and size (the bubble is world-sized): the
			// icon size is logical, so divide the physical size by the UI scale
			gfx.WorldIcon(screen, icon, float64(sx-bw/2)+2, float64(sy-bw/2)+2, float64(bw-4)/gfx.UIScale())
			continue
		}
		s := float32(gfx.UIScale()) // the canvas takes logical pixels
		gfx.FillCircle(screen, sx/s, sy/s, bw*0.42/s, uikit.Panel, true)
		for i := -1; i <= 1; i++ {
			gfx.FillCircle(screen, (sx+float32(i)*bw*0.2)/s, sy/s, float32(tile*0.025)/s, uikit.TextDim, true)
		}
	}
}

// drawFightMarker floats a red lightning bolt over a colonist in a social
// fight (overlay/fight_lightning; vector bolt while the art is missing).
func (lb *Labels) drawFightMarker(screen *ebiten.Image, wx, wy, tile float64) {
	sx, sy := lb.worldToScreenF(wx+0.5, wy-PawnMarkerLift(lb.art()))
	if img := lb.art().Image("overlay/fight_lightning", color.NRGBA{}); img != nil {
		size := tile * 0.32 // physical, like the position; WorldIcon takes it logical
		gfx.WorldIcon(screen, img, float64(sx)-size/2, float64(sy)-size/2, size/gfx.UIScale())
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
func (lb *Labels) drawBreakClouds(screen *ebiten.Image, tile float64) {
	v := lb.v
	for _, p := range v.Pawns {
		if p.MentalBreak == "" {
			continue
		}
		wx, wy := lb.interpWorld(p)
		bob := math.Sin(float64(v.Present)/14+float64(p.ID)) * 0.04
		sx, sy := lb.worldToScreenF(wx+0.5, wy-PawnMarkerLift(lb.art())-0.05+bob)
		c := color.RGBA{70, 70, 84, 230}
		for _, o := range [][3]float64{{-0.12, 0.02, 0.10}, {0.0, -0.03, 0.13}, {0.13, 0.02, 0.10}} {
			vector.FillCircle(screen, sx+float32(o[0]*tile), sy+float32(o[1]*tile), float32(o[2]*tile), c, true)
		}
		if (v.Present/8+uint64(p.ID))%6 == 0 { // a flicker of lightning
			vector.StrokeLine(screen, sx, sy+float32(tile*0.08), sx-float32(tile*0.04), sy+float32(tile*0.2), 2, color.RGBA{250, 230, 120, 255}, true)
		}
	}
}

// Marker heights over a pawn, from its head centre (creature.PawnHead,
// the body metadata): status markers float MarkerAboveHead tiles above
// it, clear of hair and hats; a chat bubble sits ChatAboveHead above it,
// beside the head.
const (
	MarkerAboveHead = 0.5
	ChatAboveHead   = 0.2
)

// PawnMarkerLift is how far above its tile's top a pawn's status markers
// float (hostile chevron, draft icon, fight bolt), in tiles.
func PawnMarkerLift(c *art.Catalog) float64 {
	_, hy := creature.PawnHead(c)
	return MarkerAboveHead - hy
}

// DrawHostileMarker floats a red chevron lift tiles above the top of tile
// (wx, wy): over a raider (orange while it flees) or a hostile animal.
func (lb *Labels) DrawHostileMarker(screen *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64) {
	c := color.RGBA{220, 50, 40, 240}
	if p.Fleeing {
		c = color.RGBA{230, 150, 50, 220}
	}
	sx, sy := lb.worldToScreenF(wx+0.5, wy-lift)
	w, h := float32(tile*0.13), float32(tile*0.1)
	th := float32(tile * 0.06)
	vector.StrokeLine(screen, sx-w, sy-h, sx, sy+h, th, c, true)
	vector.StrokeLine(screen, sx, sy+h, sx+w, sy-h, th, c, true)
}

// drawBuildingDamage puts a small health bar under damaged buildings.
func (lb *Labels) drawBuildingDamage(screen *ebiten.Image, w game.WorldView, tile float64) {
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.Floor || b.HPPct >= 100 || b.HPPct <= 0 || !lb.LOD.InView(b.X, b.Y, 4, 1) { // lod.go: off screen
			continue
		}
		sx, sy := lb.worldToScreen(b.X, b.Y)
		bw := float32(tile) * float32(max(1, b.W)) * 0.8
		x := sx + (float32(tile)*float32(max(1, b.W))-bw)/2
		y := sy + float32(tile)*float32(max(1, b.H)) - 7
		vector.FillRect(screen, x-1, y-1, bw+2, 6, uikit.Shade(210), false)
		c := uikit.Good
		switch {
		case b.HPPct < 35:
			c = uikit.Danger
		case b.HPPct < 70:
			c = uikit.Warning
		}
		vector.FillRect(screen, x, y, bw*float32(b.HPPct)/100, 4, c, false)
	}
}
