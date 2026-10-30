package inspector

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Storage buildings' panes and buttons (STO-3): shelf, hopper, sarcophagus
// and grave are storage whose settings live in the Storage window (the
// building's own zone, ZoneView.Shelf is its id). Shelves also link into a
// group sharing one set of settings (LinkStorage, UnlinkStorage). The
// nutrient paste dispenser shows the hoppers beside it and how full they
// are. Everything is read from ZoneView and BuildingView.

// Button ids (journeys press them by these).
const (
	StorageID       = "cmd.shelf.storage"
	LinkStorageID   = "cmd.shelf.link"
	UnlinkStorageID = "cmd.shelf.unlink"
)

// Storage building kinds (construction names).
const (
	hopperKind      = "hopper"
	dispenserKind   = "nutrient paste dispenser"
	sarcophagusKind = "sarcophagus"
	graveKind       = "grave"
)

// storageKinds are the buildings with a Storage button.
var storageKinds = map[string]bool{shelfKind: true, hopperKind: true, sarcophagusKind: true, graveKind: true}

// storageZoneOf is the storage zone building b owns.
func storageZoneOf(a Building, b game.BuildingView) (game.ZoneView, bool) {
	for _, z := range a.shell.World().Zones {
		if z.Shelf != 0 && z.Shelf == uint64(b.ID) {
			return z, true
		}
	}
	return game.ZoneView{}, false
}

// shelfStorageCommand is a storage building's Storage button (it opens the
// Storage window on its zone), plus, on a shelf, the group buttons.
func (a Building) shelfStorageCommand(b game.BuildingView) []Button {
	z, ok := storageZoneOf(a, b)
	if !ok {
		return nil
	}
	zid := z.ID
	out := []Button{{ID: StorageID, Label: "Storage", Icon: "zone_stockpile",
		Desc: "Choose which items this accepts, set its priority and special filters, and copy or paste settings.",
		Run:  func() { a.shell.Do(ports.OpenStorage{Zone: zid}) }}}
	if b.Kind != shelfKind {
		return out
	}
	return append(out, linkCommand(a, b, z), unlinkCommand(a, b, z))
}

// otherShelves are the shelves (other than b) that b can link to.
func otherShelves(a Building, b game.BuildingView) []game.ZoneView {
	var out []game.ZoneView
	for _, z := range a.shell.World().Zones {
		if z.Kind == game.ZoneShelf && z.Shelf != 0 && z.Shelf != uint64(b.ID) {
			out = append(out, z)
		}
	}
	return out
}

// linkCommand is "Link settings": a pick of the other shelves; the chosen
// one's group takes this shelf in and they share one set of settings.
func linkCommand(a Building, b game.BuildingView, z game.ZoneView) Button {
	others := otherShelves(a, b)
	c := Button{ID: LinkStorageID, Label: "Link settings", Icon: "zone_stockpile", Desc: "Join another shelf's group so both share one set of settings, priority and name."}
	if len(others) == 0 {
		c.Disabled, c.Reason = true, "no other shelf to link to"
		return c
	}
	self := b.ID
	c.Run = func() {
		entries := make([]game.OrderOption, len(others))
		for i, o := range others {
			entries[i] = game.OrderOption{Label: shelfLabel(o)}
			if z.Group != 0 && o.Group == z.Group {
				entries[i].Disabled, entries[i].Reason = true, "already linked"
			}
		}
		x, y := a.Commands().Anchor(LinkStorageID)
		a.shell.Pick(x, y, true, entries, func(i int) {
			if i >= 0 && i < len(others) {
				a.shell.Enqueue(command.Command{Kind: command.LinkStorage, Target: uint64(self), Zone: others[i].Shelf})
			}
		})
	}
	return c
}

// unlinkCommand takes the shelf out of its group.
func unlinkCommand(a Building, b game.BuildingView, z game.ZoneView) Button {
	c := Button{ID: UnlinkStorageID, Label: "Unlink", Icon: "order_cancel", Desc: "Leave the group: this shelf keeps its current settings on its own.",
		Cmds: []command.Command{{Kind: command.UnlinkStorage, Target: uint64(b.ID)}}}
	if z.Group == 0 {
		c.Disabled, c.Reason = true, "not linked to another shelf"
	}
	return c
}

// shelfLabel names a shelf for the link pick: its zone name, else its tile.
func shelfLabel(z game.ZoneView) string {
	if z.Name != "" {
		return z.Name
	}
	return fmt.Sprintf("Shelf at (%d, %d)", z.MinX, z.MinY)
}

// storageLines are the stored-of-capacity line and the group line of a
// storage building, and a dispenser's hoppers.
func storageLines(a Building, b game.BuildingView) []string {
	var out []string
	if z, ok := storageZoneOf(a, b); ok {
		out = append(out, fmt.Sprintf("Stored: %d of %d stacks", z.Stored, z.Capacity), "Priority: "+zonePriority(z))
		if z.Group != 0 {
			out = append(out, "Linked with other shelves")
		}
	}
	if b.Kind == dispenserKind {
		out = append(out, hopperLines(a, b)...)
	}
	return out
}

func zonePriority(z game.ZoneView) string {
	if z.Priority == "" {
		return "Normal"
	}
	return z.Priority
}

// hopperLines are the hoppers touching dispenser b (a tile away, any side
// of its footprint) and how full they are.
func hopperLines(a Building, b game.BuildingView) []string {
	w := a.shell.World()
	n, stored, capacity := 0, 0, 0
	for _, h := range w.Buildings {
		if h.Kind != hopperKind || h.X < b.X-1 || h.X > b.X+max(b.W, 1) || h.Y < b.Y-1 || h.Y > b.Y+max(b.H, 1) {
			continue
		}
		n++
		for _, z := range w.Zones {
			if z.Shelf == uint64(h.ID) {
				stored, capacity = stored+z.Stored, capacity+z.Capacity
			}
		}
	}
	if n == 0 {
		return []string{"No hopper beside it: nothing to make paste from"}
	}
	return []string{fmt.Sprintf("Hoppers: %d", n), fmt.Sprintf("Hopper fill: %d of %d stacks", stored, capacity)}
}
