package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// Frame inspector (UI-015, MAIN_UI §10 inspector): click a blueprint or
// frame to see its materials, work and who is on it. Reads only FrameView;
// the one action is Cancel blueprint (command.CancelFrame).

func frameRect() button { return button{x: hudPad, y: hudTopH + hudPad, w: 380, h: 262} }

func (a *App) selectedFrame() (game.FrameView, bool) {
	if !a.hasFrame || a.renderer == nil {
		return game.FrameView{}, false
	}
	for _, f := range a.renderer.world().Frames {
		if f.ID == a.selFrame {
			return f, true
		}
	}
	a.hasFrame = false // finished, cancelled or removed
	return game.FrameView{}, false
}

func (a *App) clearFrame() { a.hasFrame = false }

func (a *App) frameButtons() []button {
	if _, ok := a.selectedFrame(); !ok || a.workOpen || a.archOpen {
		return nil
	}
	r := frameRect()
	return []button{
		{id: "frame.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22},
		{id: "frame.cancel", label: "Cancel blueprint", x: r.x + r.w - 150, y: r.y + r.h - 32, w: 140, h: 24},
	}
}

// applyFrameClick handles a "frame.*" button.
func (a *App) applyFrameClick(id string) {
	f, ok := a.selectedFrame()
	if !ok {
		return
	}
	switch id {
	case "frame.close":
		a.clearFrame()
	case "frame.cancel":
		a.renderer.enqueue(command.Command{Kind: command.CancelFrame, Target: uint64(f.ID)})
		a.clearFrame()
	}
}

// frameInfo is the inspector's text, computed from the read model so the
// layout and the tests share one source.
type frameInfo struct {
	Title    string
	Status   string
	Material string  // "wood 6 / 10 (+4 on the way)"; "" when none is needed
	MatFrac  float64 // delivered share, 0..1
	Stock    string  // "45 wood (12 reserved)"; "" when not relevant
	Work     string  // "0 / 40  ~ 1 in-game min" or the waiting note
	WorkFrac float64
	People   string // "Hauling: Ada (4 wood)   Building: -"
	Foot     string
}

// frameStuff is the material a frame consumes ("wood" when unset).
func frameStuff(f game.FrameView) string {
	if f.Stuff == "" {
		return "wood"
	}
	return f.Stuff
}

// frameName is the display name of the thing being built.
func frameName(f game.FrameView) string {
	name := capitalize(f.Kind)
	if f.Kind == "carpet" && strings.HasSuffix(f.Stuff, " carpet") {
		return capitalize(f.Stuff)
	}
	if f.Stuff == "stone" {
		name = "Stone " + f.Kind
	}
	return name
}

// frameStarted reports whether construction has begun (materials in or
// work done): the title then says "frame", else "blueprint".
func frameStarted(f game.FrameView) bool {
	return f.Wood > 0 || (f.WorkTotal > 0 && f.WorkLeft < f.WorkTotal)
}

// frameMinutes is the in-game minutes left for work ticks.
func frameMinutes(left int) int {
	per := clock.TicksPerHour / 60
	if per < 1 {
		per = 1
	}
	return (left + per - 1) / per
}

func buildInfo(f game.FrameView, w game.WorldView) frameInfo {
	in := frameInfo{Title: frameName(f) + " (blueprint)"}
	if frameStarted(f) {
		in.Title = frameName(f) + " (frame)"
	}
	stuff := frameStuff(f)
	switch {
	case f.Moving:
		name := f.Kind
		for _, b := range w.Buildings {
			if b.ID == f.MovingFrom {
				name = b.Kind
			}
		}
		in.Status = "Moving " + name + " here: needs no materials"
	case f.Status != "":
		in.Status = f.Status
	case f.Wood >= f.Need:
		in.Status = "Waiting for a builder"
	default:
		in.Status = "Waiting for materials"
	}
	complete := f.Moving || f.Wood >= f.Need
	if !f.Moving && f.Need > 0 {
		in.Material = fmt.Sprintf("%s %d / %d", stuff, f.Wood, f.Need)
		if f.Incoming > 0 {
			in.Material += fmt.Sprintf(" (+%d on the way)", f.Incoming)
		}
		in.MatFrac = float64(f.Wood) / float64(f.Need)
		if !complete {
			in.Stock = fmt.Sprintf("%d %s", f.InStock, stuff)
			if f.Reserved > 0 {
				in.Stock += fmt.Sprintf(" (%d reserved)", f.Reserved)
			}
		}
	}
	if f.WorkTotal > 0 {
		done := f.WorkTotal - f.WorkLeft
		in.WorkFrac = float64(done) / float64(f.WorkTotal)
		if complete {
			in.Work = fmt.Sprintf("%d / %d  ~ %d in-game min", done, f.WorkTotal, frameMinutes(f.WorkLeft))
		} else {
			in.Work = fmt.Sprintf("%d / %d  (starts once materials are in)", done, f.WorkTotal)
		}
	}
	haul, build := "-", "-"
	if len(f.Haulers) > 0 {
		haul = strings.Join(f.Haulers, ", ")
	}
	if f.Builder != "" {
		build = f.Builder
	}
	in.People = "Hauling: " + haul + "   Building: " + build
	foot := fmt.Sprintf("Footprint %dx%d", max(f.W, 1), max(f.H, 1))
	if f.Facing != "" {
		foot += " · facing " + f.Facing
	}
	in.Foot = foot + fmt.Sprintf(" · tile (%d, %d)", f.X, f.Y)
	return in
}

func (a *App) drawFrameInspector(screen *ebiten.Image) {
	f, ok := a.selectedFrame()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	in := buildInfo(f, a.renderer.world())
	r := frameRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	x, y := int(r.x)+12, int(r.y)+10
	drawLabel(screen, x, y, in.Title)
	drawLabel(screen, x, y+22, "Status: "+in.Status)
	barW := int(r.w) - 24
	y += 48
	if in.Material != "" {
		drawLabel(screen, x, y, "Materials  "+in.Material)
		drawFracBar(screen, x, y+18, barW, 8, in.MatFrac)
		y += 32
		if in.Stock != "" {
			drawLabel(screen, x, y, "In stock  "+in.Stock)
			y += 20
		}
	}
	if in.Work != "" {
		drawLabel(screen, x, y, "Work  "+in.Work)
		drawFracBar(screen, x, y+18, barW, 8, in.WorkFrac)
		y += 32
	}
	drawLabel(screen, x, y, in.People)
	drawLabel(screen, x, y+20, in.Foot)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.frameButtons(), mx, my)
	if a.renderer.art != nil {
		bw, bh := max(f.W, 1), max(f.H, 1)
		for dy := 0; dy < bh; dy++ {
			for dx := 0; dx < bw; dx++ {
				a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(f.X+dx), float64(f.Y+dy), a.renderer.tilePx(), a.sw, a.sh)
			}
		}
	}
}

// drawFracBar is a thin bar filled to frac (0..1).
func drawFracBar(screen *ebiten.Image, x, y, w, h int, frac float64) {
	frac = max(0, min(1, frac))
	vector.FillRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{R: 40, G: 30, B: 24, A: 255}, false)
	vector.FillRect(screen, float32(x), float32(y), float32(float64(w)*frac), float32(h), color.RGBA{R: 120, G: 170, B: 70, A: 255}, false)
}
