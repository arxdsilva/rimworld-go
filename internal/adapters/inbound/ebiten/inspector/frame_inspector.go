package inspector

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// Frame inspector (UI-015, MAIN_UI §10 inspector): click a blueprint or
// frame to see its materials, work and who is on it. Reads only FrameView;
// the one action is Cancel blueprint (command.CancelFrame). PLAY-018: the
// pane (cmds_build.go framePane) shows it; this file keeps the selection,
// the click handler and the text the pane is built from.

func (a *Pane) SelectedFrame() (game.FrameView, bool) {
	if !a.sel.HasFrame || a.sim() == nil {
		return game.FrameView{}, false
	}
	for _, f := range a.shell.World().Frames {
		if f.ID == a.sel.SelFrame {
			return f, true
		}
	}
	a.sel.HasFrame = false // finished, cancelled or removed
	return game.FrameView{}, false
}

func (a Thing) ClearFrame() { a.sel.HasFrame = false }

// applyFrameClick handles a "frame.*" button.
func (a Thing) ApplyFrameClick(id string) {
	f, ok := a.SelectedFrame()
	if !ok {
		return
	}
	switch id {
	case "frame.close":
		a.ClearFrame()
	case "frame.cancel":
		a.shell.Enqueue(command.Command{Kind: command.CancelFrame, Target: uint64(f.ID)})
		a.ClearFrame()
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
func FrameName(f game.FrameView) string {
	if f.Kind == "carpet" && strings.HasSuffix(f.Stuff, " carpet") {
		return Capitalize(f.Stuff)
	}
	return materialTitle(f.Kind, f.Stuff)
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

func BuildInfo(f game.FrameView, w game.WorldView) frameInfo {
	in := frameInfo{Title: FrameName(f) + " (blueprint)"}
	if frameStarted(f) {
		in.Title = FrameName(f) + " (frame)"
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

// drawFracBar is a thin bar filled to frac (0..1).
func DrawFracBar(screen *ebiten.Image, x, y, w, h int, frac float64) {
	frac = max(0, min(1, frac))
	gfx.FillRect(screen, float32(x), float32(y), float32(w), float32(h), uikit.Field, false)
	gfx.FillRect(screen, float32(x), float32(y), float32(float64(w)*frac), float32(h), uikit.Good, false)
}
