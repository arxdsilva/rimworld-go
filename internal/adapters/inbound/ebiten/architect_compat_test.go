package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/application/game"
)

func (a *App) curStyle(t archTool) architect.DrawStyle { return a.arch.CurStyle(toDef(t)) }

func (a *App) toolStuff(t archTool) string { return a.arch.ToolStuff(toDef(t)) }

func (a *App) toolLock(t archTool) string { return a.arch.ToolLock(toDef(t)) }

func (a *App) zoneToolCells(z game.ZoneView) []ghostCell {
	return ghostCells(a.arch.Zones.Cells(z))
}

func toolsIn(category string) []archTool { return fromDefs(architect.ToolsIn(category)) }

func zoneKindOf(t archTool) string { return architect.ZoneKindOf(toDef(t)) }

func drawIconFallback(screen *ebiten.Image, t archTool, x, y float32) {
	architect.DrawIconFallback(screen, toDef(t), x, y)
}

func archTooltipLines(t archTool, stuff string, wood, stock int) []string {
	return architect.TooltipLines(toDef(t), stuff, wood, stock)
}

func zoneGhostCells(x0, y0, x1, y1 int, clip [4]int, can func(x, y int) bool) []ghostCell {
	return ghostCells(architect.ZoneGhostCells(x0, y0, x1, y1, clip, can))
}

func rowRuns(cells [][2]int) []rowRun {
	rs := architect.RowRuns(cells)
	out := make([]rowRun, len(rs))
	for i, r := range rs {
		out[i] = rowRun{y: r.Y, x0: r.X0, x1: r.X1}
	}
	return out
}
