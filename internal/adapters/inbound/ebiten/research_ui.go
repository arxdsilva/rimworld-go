package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Research tab: the project list. Click an available project to research
// it; colonists with Research work then study at a research bench.

const researchRowH = 58

// lockedBy is the research project still gating an unlock key ("" if none).
func (a *App) lockedBy(key string) string {
	if a.renderer == nil {
		return ""
	}
	return a.renderer.world().Locked[key]
}

func (a *App) researchButtons() []button {
	r := workRect(a.sw, a.sh)
	var out []button
	for i, p := range a.renderer.world().Research {
		y := r.y + 44 + float32(i*researchRowH)
		if y+researchRowH > r.y+r.h {
			break
		}
		b := button{id: "research.pick." + strconv.Itoa(int(p.ID)), x: r.x + 12, y: y, w: 220, h: researchRowH - 8,
			label: p.Label, selected: p.Current, quiet: true}
		switch {
		case p.Done:
			b.disabled, b.reason = true, p.Label+" is already researched"
		case !p.Available:
			b.disabled, b.reason = true, "Needs "+strings.Join(p.Requires, ", ")+" first"
		}
		out = append(out, b)
	}
	return out
}

func (a *App) pickResearch(id string) {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "research.pick."))
	if err != nil || a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.SetResearch, Build: uint8(n)})
}

func (a *App) drawResearch(screen *ebiten.Image, sw, sh int) {
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawText(screen, "Research", float64(r.x)+12, float64(r.y)+6, withColor(tsLabel, uiHeading))
	hintX := float64(r.x) + 12 + float64(textW("Research", tsLabel)) + 16
	drawText(screen, ellipsize("pick a project; colonists with Research work study it at a research bench", tsSmall, float64(r.x+r.w)-12-hintX),
		hintX, float64(r.y)+6+(lineH(tsLabel)-lineH(tsSmall))/2, withColor(tsSmall, uiTextDim))
	if !a.hasResearchBench() {
		drawText(screen, "No research bench yet: build one from Architect > Production.", float64(r.x)+12, float64(r.y)+25, withColor(tsSmall, uiWarning))
	}
	mx, my := a.currentCursor()
	views := a.renderer.world().Research
	for i, b := range a.researchButtons() {
		p := views[i]
		drawButton(screen, b, b.contains(mx, my) && !b.disabled)
		x, y := int(b.x+b.w)+14, int(b.y)
		status := "available"
		switch {
		case p.Done:
			status = "done"
		case p.Current:
			status = "researching"
		case !p.Available:
			status = "needs " + strings.Join(p.Requires, ", ")
		}
		maxW := float64(r.x+r.w) - 12 - float64(x)
		drawResearchBar(screen, float32(x), float32(y+6), 120, 10, p)
		drawText(screen, ellipsize(fmt.Sprintf("%d / %d  %s", p.Progress, p.Cost, status), tsBody, maxW-130), float64(x+130), float64(y), tsBody)
		drawText(screen, ellipsize(p.Desc, tsSmall, maxW), float64(x), float64(y)+20, tsSmall)
		drawText(screen, ellipsize("Unlocks: "+strings.Join(p.Unlocks, ", "), tsSmall, maxW), float64(x), float64(y)+37, withColor(tsSmall, uiTextDim))
	}
}

func drawResearchBar(screen *ebiten.Image, x, y, w, h float32, p game.ResearchView) {
	uiFillRect(screen, x, y, w, h, color.RGBA{30, 26, 22, 255}, false)
	c := color.RGBA{90, 160, 210, 255}
	if p.Done {
		c = color.RGBA{120, 200, 90, 255}
	}
	if p.Cost > 0 {
		uiFillRect(screen, x, y, w*float32(min(p.Progress, p.Cost))/float32(p.Cost), h, c, false)
	}
	uiStrokeRect(screen, x, y, w, h, 1, color.RGBA{90, 80, 70, 255}, false)
}

func (a *App) hasResearchBench() bool {
	for _, b := range a.renderer.world().Buildings {
		if b.Kind == "research bench" {
			return true
		}
	}
	return false
}
