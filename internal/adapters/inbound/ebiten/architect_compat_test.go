package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/application/game"
)

func (a *App) curStyle(t architect.ToolDef) architect.DrawStyle { return a.arch.CurStyle(t) }

func (a *App) toolStuff(t architect.ToolDef) string { return a.arch.ToolStuff(t) }

func (a *App) toolLock(t architect.ToolDef) string { return a.arch.ToolLock(t) }

func (a *App) zoneToolCells(z game.ZoneView) []architect.GhostCell {
	return a.arch.Zones.Cells(z)
}

func toolsIn(category string) []architect.ToolDef { return architect.ToolsIn(category) }

func zoneKindOf(t architect.ToolDef) string { return architect.ZoneKindOf(t) }

func drawIconFallback(screen *ebiten.Image, t architect.ToolDef, x, y float32) {
	architect.DrawIconFallback(screen, t, x, y)
}

func archTooltipLines(t architect.ToolDef, stuff string, wood, stock int) []string {
	return architect.TooltipLines(t, stuff, wood, stock)
}

func zoneGhostCells(x0, y0, x1, y1 int, clip [4]int, can func(x, y int) bool) []architect.GhostCell {
	return architect.ZoneGhostCells(x0, y0, x1, y1, clip, can)
}

func rowRuns(cells [][2]int) []architect.RowRun {
	rs := architect.RowRuns(cells)
	out := make([]architect.RowRun, len(rs))
	for i, r := range rs {
		out[i] = architect.RowRun{Y: r.Y, X0: r.X0, X1: r.X1}
	}
	return out
}
