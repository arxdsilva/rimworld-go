package saves

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game/savefmt"
)

// The colony session (SAV-003): which colony is running and how its saves
// name it. NewColony starts one (a new id from newID, the new slot as its
// name); a load takes the loaded save's header; every save carries it
// (metaFor), so the save list can tell the colony's saves from others'.
//
// SaveSlot stays the colony's slot, the key of its name: its autosaves and
// a commitment colony's one save are named after it, whatever name the
// player gives a manual save (session.Name and Base).

// metaFor is the header of a save of the running colony to slot name:
// written by the game (autosave) or by the player. A player's save is the
// session's new Base (commit sets it); an autosave keeps the Base of the
// last one, so loading it offers that name again.
func metaFor(b *Book, name string, autosave bool) ports.SaveMeta {
	s := b.session
	base := s.Base
	if base == "" {
		base = savefmt.BaseSlot(name)
	}
	return ports.SaveMeta{Name: name, Colony: s.Colony, ColonyName: s.ColonyName, Base: base,
		Autosave: autosave, Season: b.sh.World().Season}
}

// takeSession makes the save just loaded the running colony: its header names
// the colony (an older save's header is upgraded to "slot:"+<slot> by the
// store) and the save the player last made of it.
func takeSession(b *Book, meta ports.SaveMeta) {
	base := meta.Base
	if base == "" {
		base = savefmt.BaseSlot(meta.Name)
	}
	name := meta.Name
	if meta.Autosave {
		name = base // the dialog offers the player's own save, not the autosave
	}
	colonyName := meta.ColonyName
	if colonyName == "" {
		colonyName = savefmt.BaseSlot(meta.Name)
	}
	b.session = session{Colony: meta.Colony, ColonyName: colonyName, Name: name, Base: base}
	b.SaveSlot = savefmt.SlotKey(colonyName)
}
