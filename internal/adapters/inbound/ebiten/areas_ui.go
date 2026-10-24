package ebiten

import (
	"fmt"
	"strings"
	"sync"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area manager (MAIN_UI_SCREENS §18 ENG-003 contract): the
// colony's areas (name, cell count) with a rename field each, the area
// the Architect's "Allowed area" tools paint, and "New area" (disabled at
// the cap). Opened from the Assign tab. Reads Simulation.Areas; emits
// RenameArea (a new area is the next index, so named).

const (
	areasModalID = "colony.areas"
	areasW       = 520
	areasH       = 460
	areasRowH    = 34
	areasMax     = 10 // area.MaxAreas
	areasEmpty   = "No areas: add one to restrict where colonists go."
	areasCapWhy  = "A colony has at most 10 areas."
)

// areaPaint is the area each App's Architect area tools paint (an App's
// presentation choice, not simulation state).
var areaPaint = struct {
	sync.Mutex
	m map[*App]int
}{m: map[*App]int{}}

// areaPaintIndex is the area the Architect's area tools paint now: the
// one picked in the manager, else the first.
func (a *App) areaPaintIndex() int {
	areaPaint.Lock()
	defer areaPaint.Unlock()
	return areaPaint.m[a]
}

func (a *App) setAreaPaint(i int) {
	areaPaint.Lock()
	areaPaint.m[a] = i
	areaPaint.Unlock()
}

// openAreas opens the area manager.
func (a *App) openAreas() {
	if a.renderer == nil {
		return
	}
	names := map[int]string{}
	a.uiPushModal(modal{
		id: areasModalID, title: "Allowed areas", w: areasW, h: areasH,
		draw: func(a *App, body rect) { a.areasBody(body, names) },
		buttons: []modalButton{
			{id: "close", label: "Close", style: btnPrimary, closes: true},
			{id: "areas.new", label: "New area", disabled: func() string {
				if len(a.renderer.game.Areas()) >= areasMax {
					return areasCapWhy
				}
				return ""
			}, onClick: a.newArea},
		},
	})
}

// newArea makes the next area ("Area N") and picks it for painting.
func (a *App) newArea() {
	n := len(a.renderer.game.Areas())
	if n >= areasMax {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.RenameArea, Index: n, Name: fmt.Sprintf("Area %d", n+1)})
	a.setAreaPaint(n)
}

// areasBody declares one row per area: rename field, cell count, Paint.
func (a *App) areasBody(body rect, names map[int]string) {
	areas := a.renderer.game.Areas()
	head, rest := body.splitV(28)
	a.uiLabel(head.X, head.Y, "Architect → Zones → Allowed area paints the picked area.", withColor(tsSmall, uiTextDim))
	if len(areas) == 0 {
		a.uiLabel(rest.X, rest.Y+4, areasEmpty, dimText())
		return
	}
	paint := a.areaPaintIndex()
	area := a.uiScrollArea("areas.list", rest, float32(len(areas)*areasRowH), scrollOpts{Step: areasRowH * 2})
	y := area.View.Y - area.Off
	for _, ar := range areas {
		rr := rect{area.View.X, y, area.View.W - 4, areasRowH - 6}
		y += areasRowH
		if rr.Y < area.View.Y || rr.bottom() > area.View.bottom() {
			continue
		}
		v, ok := names[ar.Index]
		if !ok {
			v = ar.Name
		}
		fr := rect{rr.X, rr.Y, rr.W - 210, rr.H}
		changed, committed := a.uiTextField(fmt.Sprintf("areas.name.%d", ar.Index), fr, &v, textFieldSpec{
			MaxLen: 24, Placeholder: "Area name", Normalize: strings.TrimSpace})
		if changed {
			names[ar.Index] = v
		}
		if committed && v != "" && v != ar.Name {
			a.renderer.enqueue(command.Command{Kind: command.RenameArea, Index: ar.Index, Name: v})
			delete(names, ar.Index)
		}
		a.uiLabel(fr.right()+8, rr.Y+(rr.H-float32(lineH(tsSmall)))/2, fmt.Sprintf("%d cells", ar.Cells), withColor(tsSmall, uiTextDim))
		if a.uiButton(fmt.Sprintf("areas.paint.%d", ar.Index), rect{rr.right() - 96, rr.Y, 96, rr.H}, "Paint", btnOpts{
			Selected: paint == ar.Index, Tip: "Paint\nThe Architect's Allowed area tools paint and clear this area."}) {
			a.setAreaPaint(ar.Index)
		}
	}
}

// areaReader is the per-pawn area the Assign tab shows (consumer-owned;
// the game implements it).
type areaReader interface {
	AreaOf(id entity.EntityID) int
}

// areaLabel is the Assign tab's area button label.
func areaLabel(areas []game.AreaView, i int) string {
	for _, ar := range areas {
		if ar.Index == i {
			return ar.Name
		}
	}
	return "Unrestricted"
}

// nextArea is the assignment after i in the cycle: unrestricted, then
// each area in turn.
func nextArea(areas []game.AreaView, i int) int {
	if len(areas) == 0 {
		return -1
	}
	for k, ar := range areas {
		if ar.Index == i {
			if k+1 < len(areas) {
				return areas[k+1].Index
			}
			return -1
		}
	}
	return areas[0].Index
}
