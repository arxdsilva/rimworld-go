package architect

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The allowed-area manager (PLAY-015 §1.3): a centred modal listing the
// colony's areas: colour swatch, inline rename, cell count, Invert and a
// red X Delete (with a confirm when colonists or animals use the area),
// then "New area" (disabled at the cap) and "Close". Reachable from the
// Architect submenu, the Schedule, Animals and Assign tabs (every caller
// uses Areas.OpenManager). Reads Sim.Areas; emits Create/Rename/
// Invert/DeleteArea by area id.

const (
	// AreasModalID is the manager's modal id.
	AreasModalID = "colony.areas"
	areasW       = 600
	areasH       = 460
	areasRowH    = 34
	// AreasMax is the colony's area cap (area.MaxAreas).
	AreasMax    = 10
	areasEmpty  = "No areas: add one to restrict where colonists go."
	areasCapWhy = "A colony has at most 10 areas."
	// AreasConfirm is the delete confirmation's modal id.
	AreasConfirm = "colony.areas.confirm"
)

// OpenManager opens the area manager.
func (a *Areas) OpenManager() {
	if a.sim() == nil {
		return
	}
	names := map[int]*string{}
	a.sh.Kit().Push(uikit.Modal{
		ID: AreasModalID, Title: "Allowed areas", W: areasW, H: areasH,
		Draw: func(body gfx.Rect) { a.body(body, names) },
		Buttons: []uikit.ModalButton{
			{ID: "close", Label: "Close", Style: uikit.BtnPrimary, Closes: true},
			{ID: "areas.new", Label: "New area", Disabled: func() string {
				if len(a.sim().Areas()) >= AreasMax {
					return areasCapWhy
				}
				return ""
			}, OnClick: a.newArea},
		},
	})
}

// newArea asks for a new empty area ("Area N").
func (a *Areas) newArea() {
	n := len(a.sim().Areas())
	if n >= AreasMax {
		return
	}
	a.sh.Enqueue(command.Command{Kind: command.CreateArea, Name: fmt.Sprintf("Area %d", n+1)})
}

// Users counts the colonists and animals whose allowed area is id.
func (a *Areas) Users(id int) int {
	n := 0
	rd, ok := a.sim().(AreaReader)
	if ok {
		for _, p := range a.sh.Pawns() {
			if p.Status != "dead" && areaRef(rd.AreaOf(p.ID)) == id {
				n++
			}
		}
	}
	for _, an := range a.sh.World().Animals {
		if an.Tame && !an.Penned && areaRef(an.AreaRef) == id {
			n++
		}
	}
	return n
}

// deleteArea deletes area ar, asking first when pawns use it.
func (a *Areas) deleteArea(ar game.AreaView) {
	del := func() {
		a.sh.Enqueue(command.Command{Kind: command.DeleteArea, Index: int(ar.ID)})
	}
	n := a.Users(int(ar.ID))
	if n == 0 {
		del()
		return
	}
	who := fmt.Sprintf("%d colonists and animals use", n)
	if n == 1 {
		who = "1 colonist or animal uses"
	}
	a.sh.Kit().Confirm(AreasConfirm, "Delete area",
		fmt.Sprintf("%s \"%s\". Deleting it makes them unrestricted.", who, ar.Name), "Delete", true, del)
}

// body declares one row per area.
func (a *Areas) body(body gfx.Rect, names map[int]*string) {
	k := a.sh.Kit()
	areas := a.sim().Areas()
	head, rest := body.SplitV(28)
	k.Label(head.X, head.Y, "Architect > Zones > Expand allowed area paints one.", gfx.WithColor(gfx.Small, uikit.TextDim))
	if len(areas) == 0 {
		k.Label(rest.X, rest.Y+4, areasEmpty, uikit.DimText())
		return
	}
	sa := k.ScrollArea("areas.list", rest, float32(len(areas)*areasRowH), uikit.ScrollOpts{Step: areasRowH * 2})
	y := sa.View.Y - sa.Off
	for _, ar := range areas {
		rr := gfx.Rect{X: sa.View.X, Y: y, W: sa.View.W - 4, H: areasRowH - 6}
		y += areasRowH
		if rr.Y < sa.View.Y || rr.Bottom() > sa.View.Bottom() {
			continue
		}
		id := int(ar.ID)
		tint := AreaTint(ar)
		k.Queue(func(dst *ebiten.Image) {
			gfx.FillRect(dst, rr.X, rr.Y+3, 20, rr.H-6, tint, false)
			gfx.StrokeRect(dst, rr.X, rr.Y+3, 20, rr.H-6, 1, uikit.PanelBorder, false)
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
		fr := gfx.Rect{X: rr.X + 28, Y: rr.Y, W: rr.W - 28 - 190, H: rr.H}
		_, committed := k.TextField(fid, fr, v, uikit.TextFieldSpec{
			MaxLen: 24, Placeholder: "Area name", Normalize: strings.TrimSpace})
		if committed && *v != "" && *v != ar.Name {
			a.sh.Enqueue(command.Command{Kind: command.RenameArea, Index: id, Name: *v})
		} else if k.Edit.ID != fid {
			*v = ar.Name
		}
		k.Label(fr.Right()+8, rr.Y+(rr.H-float32(gfx.LineH(gfx.Small)))/2, fmt.Sprintf("%d cells", ar.Cells), gfx.WithColor(gfx.Small, uikit.TextDim))
		if k.Button(fmt.Sprintf("areas.invert.%d", id), gfx.Rect{X: rr.Right() - 86, Y: rr.Y, W: 56, H: rr.H}, "Invert", uikit.BtnOpts{
			Tip: "Invert\nSwap the area's tiles: painted become empty and the rest becomes painted."}) {
			a.sh.Enqueue(command.Command{Kind: command.InvertArea, Index: id})
		}
		if k.Button(fmt.Sprintf("areas.delete.%d", id), gfx.Rect{X: rr.Right() - 26, Y: rr.Y, W: 26, H: rr.H}, "X", uikit.BtnOpts{
			Style: uikit.BtnDanger, Tip: "Delete\nRemove this area. Anyone restricted to it becomes unrestricted."}) {
			a.deleteArea(ar)
		}
	}
}

// Assign emits AssignArea for pawn with ref.
func (a *Areas) Assign(pawn entity.EntityID, ref int) {
	a.sh.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(pawn), Index: ref})
}

// ColonistArea is colonist id's area ref (unrestricted when the game does
// not report areas) and whether the game reports them.
func (a *Areas) ColonistArea(id entity.EntityID) (int, bool) {
	if rd, ok := a.sim().(AreaReader); ok {
		return areaRef(rd.AreaOf(id)), true
	}
	return command.AreaUnrestricted, false
}
