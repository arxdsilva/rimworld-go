package inspector

import (
	"fmt"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Multi-selection (UI-016, MAIN_UI §9 Selection): Shift+click adds or
// removes, a left drag boxes colonists (else animals, else items), a
// double-click takes every visible thing of the same kind. A group of two
// or more replaces the single selections and gets its own panel; R, F and
// right-click orders act on every member. Presentation state only.

// groupRow is one member line of the group panel.
type groupRow struct {
	id    entity.EntityID
	label string
}

// groupRows lists the group's members (dropping ones that are gone).
func (a Commands) groupRows() []groupRow {
	var out []groupRow
	switch a.sel.Group.Kind {
	case selection.Pawn:
		for _, p := range a.sel.SelectedPawns() {
			s := a.shell.PawnLabel(p)
			if p.Drafted {
				s += "  (drafted)"
			}
			if p.Status != "alive" {
				s += "  (" + p.Status + ")"
			}
			out = append(out, groupRow{p.ID, s})
		}
	case selection.Animal:
		for _, an := range a.sel.SelectedAnimals() {
			s := TitleCase(an.Label)
			if an.Name != "" {
				s = an.Name + ", " + an.Label
			}
			out = append(out, groupRow{an.ID, fmt.Sprintf("%s  %d%%", s, an.Health)})
		}
	case selection.Item:
		for _, it := range a.sel.SelectedItems() {
			s := fmt.Sprintf("%s x %d", TitleCase(it.Kind), it.Qty)
			if it.Forbidden {
				s += ", forbidden"
			}
			out = append(out, groupRow{it.ID, s})
		}
	}
	return out
}

// groupNoun is "3 colonists" / "2 animals" / "4 items".
func groupNoun(k selection.Kind, n int) string {
	noun := map[selection.Kind]string{selection.Pawn: "colonist", selection.Animal: "animal", selection.Item: "item"}[k]
	if n != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s", n, noun)
}

// groupButtons are the group's command buttons in the grid (the pane's
// close and member rows are paneButtons).
func (a Commands) GroupButtons() []uikit.Button {
	if !a.sel.Grouped() || a.env().WinOpen || a.env().ArchOpen || a.sim() == nil {
		return nil
	}
	if len(a.groupRows()) < 2 {
		return nil // pruneGroup falls back to one
	}
	return a.cellButtons()
}

// groupPane is the pane for a group selection: its size, and the members
// as clickable rows.
func (a Commands) GroupPane() Spec {
	rows := a.groupRows()
	if len(rows) < 2 {
		return Spec{} // pruneGroup falls back to one
	}
	spec := Spec{
		Title:    groupNoun(a.sel.Group.Kind, len(rows)),
		Subtitle: "Shift+click adds or removes",
		Close:    "group.close",
	}
	for _, r := range rows {
		spec.Rows = append(spec.Rows, Row{ID: "group.pick." + strconv.FormatUint(uint64(r.id), 10), Label: r.label, Tip: "Select only this one"})
	}
	return spec
}

// groupOwnKeys are the commands a group provides itself (they act on every
// member at once through applyGroupClick); members' commands under these
// keys are not merged again.
var groupOwnKeys = map[string]bool{
	"draft": true, "pawn.draft": true, "undraft": true,
	"forbid": true, "item.forbid": true, "allow": true,
	"haulmark": true, "item.haulmark": true,
}

// groupOwnCommands are the group-wide commands that already exist
// (applyGroupClick), each pair as the old panel had it: Draft and Undraft
// for colonists; Forbid, Allow and Mark for hauling for items. Draft and
// Forbid show how many members already are (On: all, Mixed: some).
func (a Commands) groupOwnCommands() []Button {
	switch a.sel.Group.Kind {
	case selection.Pawn:
		pawns := a.sel.SelectedPawns()
		on := 0
		for _, p := range pawns {
			if p.Drafted {
				on++
			}
		}
		draft := Button{ID: "group.draft", Key: "draft", Label: "Draft", Icon: "draft", Hotkey: input.Draft,
			Desc: "Draft every selected colonist."}
		draft.On = len(pawns) > 0 && on == len(pawns)
		draft.Mixed = on > 0 && on < len(pawns)
		return []Button{draft, {ID: "group.undraft", Key: "undraft", Label: "Undraft", Icon: "draft",
			Desc: "Stand every selected colonist down."}}
	case selection.Item:
		items := a.sel.SelectedItems()
		on := 0
		for _, it := range items {
			if it.Forbidden {
				on++
			}
		}
		forbid := Button{ID: "group.forbid", Key: "forbid", Label: "Forbid", Icon: "order_forbid", Hotkey: input.Forbid,
			Desc: "Colonists will not touch forbidden stacks."}
		forbid.On = len(items) > 0 && on == len(items)
		forbid.Mixed = on > 0 && on < len(items)
		out := []Button{forbid, {ID: "group.allow", Key: "allow", Label: "Allow", Icon: "order_allow",
			Desc: "Let colonists use these stacks again."}}
		if haulMarkable(items) {
			out = append(out, Button{ID: "group.haulmark", Key: "haulmark", Label: "Mark for hauling", Icon: "order_haul",
				Desc: "Colonists will haul the stacks found on the map."})
		}
		return out
	}
	return nil
}

// groupCommands are a group's commands: its own group-wide ones, then its
// members' commands merged by key (mergeCommands).
func (a Commands) GroupCommands() []Button {
	own := a.groupOwnCommands()
	return append(own, a.groupMerged()...)
}

// groupMerged are the members' commands merged by key, without the keys
// the group handles itself. It is remembered for one Update (and one
// selection): the layout asks many times a frame.
func (a Commands) groupMerged() []Button {
	m := &a.memo
	if m.ok && m.updates == a.env().Updates && m.kind == a.sel.Group.Kind && slicesEqualIDs(m.ids, a.sel.Group.Ids) {
		return m.out
	}
	kind := a.sel.Group.Kind
	ids := append([]entity.EntityID(nil), a.sel.Group.Ids...)
	sets := make([][]Button, len(ids))
	for i, id := range ids {
		for _, c := range a.commandsFor(kind, id) {
			if !groupOwnKeys[c.mergeKey()] {
				sets[i] = append(sets[i], c)
			}
		}
	}
	out := MergeCommands(sets, func(member int, c Button) { a.runMember(kind, ids[member], c) })
	*m = groupMemo{updates: a.env().Updates, kind: kind, ids: ids, out: out, ok: true}
	return out
}

func slicesEqualIDs(a, b []entity.EntityID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// runMember runs member id's command c for one member of the group. A
// legacy command (an id, no Run or Cmds) acts on the single selection, so
// the member is selected for the call and the group put back after.
func (a Commands) runMember(kind selection.Kind, id entity.EntityID, c Button) {
	if c.Run != nil || len(c.Cmds) > 0 || c.Options != nil {
		a.RunCommand(c)
		return
	}
	saved := a.sel.Group
	a.sel.Select(selection.Cand{Kind: kind, ID: id})
	a.shell.Click(c.ID)
	a.sel.Clear(selection.None)
	a.sel.Group = saved
}

// haulMarkable reports whether any stack in items was found on the map and
// is not marked for hauling yet.
func haulMarkable(items []game.ItemView) bool {
	for _, it := range items {
		if it.Unclaimed && !it.HaulMarked {
			return true
		}
	}
	return false
}

// pruneGroup drops members that are gone; a group of one becomes the
// single selection.
func (a Commands) PruneGroup() {
	if a.sel.Grouped() {
		if rows := a.groupRows(); len(rows) < 2 {
			a.sel.SetGroup(a.sel.Group.Kind, groupIDs(rows))
		}
	}
}

func groupIDs(rows []groupRow) []entity.EntityID {
	out := make([]entity.EntityID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.id)
	}
	return out
}
