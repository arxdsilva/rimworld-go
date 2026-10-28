package ebiten

import (
	"fmt"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area manager (PLAY-015 §1.3): a centred modal listing the
// colony's areas: colour swatch, inline rename, cell count, Invert and a
// red X Delete (with a confirm when colonists or animals use the area),
// then "New area" (disabled at the cap) and "Close". Reachable from the
// Architect submenu, the Schedule, Animals and Assign tabs (every caller
// uses a.openAreaManager). Reads Simulation.Areas; emits Create/Rename/
// Invert/DeleteArea by area id.

const (
	areasModalID = "colony.areas"
	areasW       = 600
	areasH       = 460
	areasRowH    = 34
	areasMax     = 10 // area.MaxAreas
	areasEmpty   = "No areas: add one to restrict where colonists go."
	areasCapWhy  = "A colony has at most 10 areas."
	areasConfirm = "colony.areas.confirm"
)

// areaPaint is the area id each App's Architect area tools paint (an
// App's presentation choice, not simulation state).
var areaPaint = struct {
	sync.Mutex
	m map[*App]int
}{m: map[*App]int{}}

// areaPaintIndex is the id of the area the Architect's area tools paint
// now: the one chosen in the submenu.
func (a *App) areaPaintIndex() int {
	areaPaint.Lock()
	defer areaPaint.Unlock()
	return areaPaint.m[a]
}

func (a *App) setAreaPaint(id int) {
	areaPaint.Lock()
	areaPaint.m[a] = id
	areaPaint.Unlock()
}

// openAreaManager opens the area manager (exported for the Animals tab).
func (a *App) openAreaManager() {
	if a.renderer == nil {
		return
	}
	names := map[int]*string{}
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

// newArea asks for a new empty area ("Area N").
func (a *App) newArea() {
	n := len(a.renderer.game.Areas())
	if n >= areasMax {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.CreateArea, Name: fmt.Sprintf("Area %d", n+1)})
}

// areaUsers counts the colonists and animals whose allowed area is id.
func (a *App) areaUsers(id int) int {
	n := 0
	rd, ok := a.renderer.game.(areaReader)
	if ok {
		for _, p := range a.shell.Pawns() {
			if p.Status != "dead" && areaRef(rd.AreaOf(p.ID)) == id {
				n++
			}
		}
	}
	for _, an := range a.shell.World().Animals {
		if an.Tame && !an.Penned && areaRef(an.AreaRef) == id {
			n++
		}
	}
	return n
}

// deleteArea deletes area ar, asking first when pawns use it.
func (a *App) deleteArea(ar game.AreaView) {
	del := func() {
		a.shell.Enqueue(command.Command{Kind: command.DeleteArea, Index: int(ar.ID)})
	}
	n := a.areaUsers(int(ar.ID))
	if n == 0 {
		del()
		return
	}
	who := fmt.Sprintf("%d colonists and animals use", n)
	if n == 1 {
		who = "1 colonist or animal uses"
	}
	a.uiConfirm(areasConfirm, "Delete area",
		fmt.Sprintf("%s \"%s\". Deleting it makes them unrestricted.", who, ar.Name), "Delete", true, del)
}

// areasBody declares one row per area.
func (a *App) areasBody(body rect, names map[int]*string) {
	areas := a.renderer.game.Areas()
	head, rest := body.splitV(28)
	a.uiLabel(head.X, head.Y, "Architect > Zones > Expand allowed area paints one.", withColor(tsSmall, uiTextDim))
	if len(areas) == 0 {
		a.uiLabel(rest.X, rest.Y+4, areasEmpty, dimText())
		return
	}
	sa := a.uiScrollArea("areas.list", rest, float32(len(areas)*areasRowH), scrollOpts{Step: areasRowH * 2})
	y := sa.View.Y - sa.Off
	for _, ar := range areas {
		rr := rect{sa.View.X, y, sa.View.W - 4, areasRowH - 6}
		y += areasRowH
		if rr.Y < sa.View.Y || rr.bottom() > sa.View.bottom() {
			continue
		}
		id := int(ar.ID)
		tint := areaTint(ar)
		a.uiSt().queue(func(dst *ebiten.Image) {
			uiFillRect(dst, rr.X, rr.Y+3, 20, rr.H-6, tint, false)
			uiStrokeRect(dst, rr.X, rr.Y+3, 20, rr.H-6, 1, uiPanelBorder, false)
		})
		// The field edits a value that outlives the frame; it follows the
		// game's name whenever it is not being edited.
		fid := fmt.Sprintf("areas.name.%d", id)
		v := names[id]
		if v == nil {
			v = new(string)
			*v = ar.Name
			names[id] = v
		}
		fr := rect{rr.X + 28, rr.Y, rr.W - 28 - 190, rr.H}
		_, committed := a.uiTextField(fid, fr, v, textFieldSpec{
			MaxLen: 24, Placeholder: "Area name", Normalize: strings.TrimSpace})
		if committed && *v != "" && *v != ar.Name {
			a.shell.Enqueue(command.Command{Kind: command.RenameArea, Index: id, Name: *v})
		} else if a.uiSt().edit.id != fid {
			*v = ar.Name
		}
		a.uiLabel(fr.right()+8, rr.Y+(rr.H-float32(lineH(tsSmall)))/2, fmt.Sprintf("%d cells", ar.Cells), withColor(tsSmall, uiTextDim))
		if a.uiButton(fmt.Sprintf("areas.invert.%d", id), rect{rr.right() - 86, rr.Y, 56, rr.H}, "Invert", btnOpts{
			Tip: "Invert\nSwap the area's tiles: painted become empty and the rest becomes painted."}) {
			a.shell.Enqueue(command.Command{Kind: command.InvertArea, Index: id})
		}
		if a.uiButton(fmt.Sprintf("areas.delete.%d", id), rect{rr.right() - 26, rr.Y, 26, rr.H}, "X", btnOpts{
			Style: btnDanger, Tip: "Delete\nRemove this area. Anyone restricted to it becomes unrestricted."}) {
			a.deleteArea(ar)
		}
	}
}

// areaReader is the per-colonist area ref the Schedule tab, the inspector
// and the manager read (consumer-owned; the game implements it): -1
// unrestricted, -2 Home, else an area id.
type areaReader interface {
	AreaOf(id entity.EntityID) int
}

// assignArea emits AssignArea for pawn with ref.
func (a *App) assignArea(pawn entity.EntityID, ref int) {
	a.shell.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(pawn), Index: ref})
}

// colonistArea is colonist id's area ref (unrestricted when the game does
// not report areas) and whether the game reports them.
func (a *App) colonistArea(id entity.EntityID) (int, bool) {
	if rd, ok := a.renderer.game.(areaReader); ok {
		return areaRef(rd.AreaOf(id)), true
	}
	return command.AreaUnrestricted, false
}
