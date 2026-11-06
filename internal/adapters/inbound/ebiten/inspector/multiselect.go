package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"slices"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
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
	kind  selection.Kind
	label string
}

// groupKinds is the order a mixed group lists its kinds in.
var groupKinds = []selection.Kind{selection.Pawn, selection.Animal, selection.Item, selection.Building, selection.Frame, selection.Zone}

// groupRows lists the group's members (dropping ones that are gone), the
// kinds one after the other.
func (a Commands) groupRows() []groupRow {
	var out []groupRow
	for _, k := range groupKinds {
		if len(a.sel.Group.IDsOf(k)) == 0 {
			continue
		}
		out = append(out, a.kindRows(k)...)
	}
	return out
}

// kindRows are the rows of the group's members of kind k.
func (a Commands) kindRows(k selection.Kind) []groupRow {
	var out []groupRow
	switch k {
	case selection.Pawn:
		for _, p := range a.sel.SelectedPawns() {
			s := a.shell.PawnLabel(p)
			if p.Drafted {
				s += "  (drafted)"
			}
			if p.Status != "alive" {
				s += "  (" + p.Status + ")"
			}
			out = append(out, groupRow{p.ID, k, s})
		}
	case selection.Animal:
		for _, an := range a.sel.SelectedAnimals() {
			s := TitleCase(an.Label)
			if an.Name != "" {
				s = an.Name + ", " + an.Label
			}
			out = append(out, groupRow{an.ID, k, fmt.Sprintf("%s  %d%%", s, an.Health)})
		}
	case selection.Item:
		for _, it := range a.sel.SelectedItems() {
			s := fmt.Sprintf("%s x %d", TitleCase(it.Kind), it.Qty)
			if it.Forbidden {
				s += ", forbidden"
			}
			out = append(out, groupRow{it.ID, k, s})
		}
	case selection.Building:
		for _, b := range a.sel.SelectedBuildings() {
			s := fmt.Sprintf("%s (%d, %d)", BuildingTitleIn(a.shell, b), b.X, b.Y)
			if b.Forbidden {
				s += ", forbidden"
			}
			if b.Deconstruct {
				s += ", to deconstruct"
			}
			out = append(out, groupRow{b.ID, k, s})
		}
	case selection.Frame:
		w := a.shell.World()
		for _, f := range a.sel.SelectedFrames() {
			out = append(out, groupRow{f.ID, k, fmt.Sprintf("%s (%d, %d)", buildInfo(a.shell, f, w).Title, f.X, f.Y)})
		}
	case selection.Zone:
		for _, z := range a.sel.SelectedZones() {
			s := z.Name
			if s == "" {
				s = TitleCase(z.Kind) + " zone"
			}
			out = append(out, groupRow{entity.EntityID(z.ID), k, fmt.Sprintf("%s  %d tiles", s, len(z.Cells))})
		}
	}
	return out
}

// groupNoun is "3 colonists" / "2 animals" / "4 items" / "5 doors"; a mixed
// group is "Various (5)". Buildings of one kind are named by it.
func (a Commands) groupNoun(k selection.Kind, n int) string {
	if k == selection.Mixed {
		return a.shell.Tf("Various (%d)", n)
	}
	one := map[selection.Kind]string{selection.Pawn: a.shell.T("colonist"), selection.Animal: a.shell.T("animal"), selection.Item: a.shell.T("item"),
		selection.Building: a.shell.T("building"), selection.Frame: a.shell.T("blueprint"), selection.Zone: a.shell.T("zone")}[k]
	if k == selection.Building {
		one = a.sameBuildingNoun(one)
	}
	if n != 1 {
		one = pluralNoun(a.shell, one)
	}
	return fmt.Sprintf("%d %s", n, one)
}

// sameBuildingNoun is the lower-case title when every selected building
// has the same one, else fallback.
func (a Commands) sameBuildingNoun(fallback string) string {
	title := ""
	for _, b := range a.sel.SelectedBuildings() {
		t := strings.ToLower(BuildingTitleIn(a.shell, b))
		if title != "" && t != title {
			return fallback
		}
		title = t
	}
	if title == "" {
		return fallback
	}
	return title
}

// pluralNoun is noun in the plural ("door" -> "doors", "bench" -> "benches").
func pluralNoun(tr lang.Tr, noun string) string {
	if strings.HasSuffix(noun, "s") || strings.HasSuffix(noun, "x") || strings.HasSuffix(noun, "ch") || strings.HasSuffix(noun, "sh") {
		return lang.Tf(tr, "%ses", noun)
	}
	return lang.Tf(tr, "%ss", noun)
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
		Title:    a.groupNoun(a.sel.Group.Kind(), len(rows)),
		Subtitle: a.shell.T("Shift+click adds or removes"),
		Close:    "group.close",
	}
	for _, r := range rows {
		spec.Rows = append(spec.Rows, Row{ID: selection.PickID(selection.Cand{Kind: r.kind, ID: r.id}), Label: r.label, Tip: a.shell.T("Select only this one")})
	}
	return spec
}

// The keys of the commands a group provides itself (they act on every
// member at once through applyGroupClick); a member's command under these
// keys is not merged again, but only when the group really offers it: a
// group with no colonist leaves Draft to nobody.
var (
	groupDraftKeys  = []string{"draft", "pawn.draft", "undraft"}
	groupItemKeys   = []string{"forbid", "item.forbid", "allow", "haulmark", "item.haulmark"}
	groupForbidKeys = []string{BuildingForbidID} // a buildings' Forbid, folded into the group's own when it has items
)

// groupOwnKeys are the keys the group handles itself.
func (a Commands) groupOwnKeys() map[string]bool {
	out := map[string]bool{}
	has := func(k selection.Kind) bool { return len(a.sel.Group.IDsOf(k)) > 0 }
	if has(selection.Pawn) {
		for _, k := range groupDraftKeys {
			out[k] = true
		}
	}
	if has(selection.Item) {
		for _, k := range groupItemKeys {
			out[k] = true
		}
		if a.groupForbidsBuildings() {
			for _, k := range groupForbidKeys {
				out[k] = true
			}
		}
	}
	return out
}

// groupForbidsBuildings reports a group with items also holds a building
// Forbid applies to: its own Forbid and Allow then cover both.
func (a Commands) groupForbidsBuildings() bool {
	if a.sel.Group.Kind() != selection.Mixed {
		return false
	}
	return slices.ContainsFunc(a.sel.SelectedBuildings(), forbiddable)
}

// forbidTarget is a stack or a building Forbid acts on.
type forbidTarget struct {
	id        entity.EntityID
	forbidden bool
}

// forbidTargets are the group's stacks and forbiddable buildings.
func (a Commands) forbidTargets() []forbidTarget {
	var out []forbidTarget
	for _, it := range a.sel.SelectedItems() {
		out = append(out, forbidTarget{it.ID, it.Forbidden})
	}
	for _, b := range a.sel.SelectedBuildings() {
		if forbiddable(b) {
			out = append(out, forbidTarget{b.ID, b.Forbidden})
		}
	}
	return out
}

// forbidGroup forbids (on) or allows every stack and forbiddable building
// of the group that is not so already: the rule F uses for stacks.
func (a Commands) forbidGroup(on bool) {
	for _, t := range a.forbidTargets() {
		if t.forbidden != on {
			a.shell.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(t.id), Enabled: on})
		}
	}
}

// ToggleForbidGroup is F on a group: every target becomes forbidden, or allowed
// when all already are, stacks and doors alike.
func (a Commands) ToggleForbidGroup() {
	all := true
	for _, t := range a.forbidTargets() {
		all = all && t.forbidden
	}
	a.forbidGroup(!all)
}

// groupOwnCommands are the group-wide commands: Draft and Undraft when the
// group holds a colonist; Forbid, Allow and Mark for hauling when it holds
// a stack. Draft and Forbid show how many members already are (On: all,
// Mixed: some). A group of stacks and doors has Forbid and Allow cover
// both (Run), else they are the legacy group buttons.
func (a Commands) groupOwnCommands() []Button {
	var out []Button
	if pawns := a.sel.SelectedPawns(); len(pawns) > 0 {
		on := 0
		for _, p := range pawns {
			if p.Drafted {
				on++
			}
		}
		draft := Button{ID: "group.draft", Key: "draft", Label: a.shell.T("Draft"), Icon: "draft", Hotkey: input.Draft,
			Desc: a.shell.T("Draft every selected colonist.")}
		draft.On = on == len(pawns)
		draft.Mixed = on > 0 && on < len(pawns)
		out = append(out, draft, Button{ID: "group.undraft", Key: "undraft", Label: a.shell.T("Undraft"), Icon: "draft",
			Desc: a.shell.T("Stand every selected colonist down.")})
	}
	items := a.sel.SelectedItems()
	if len(items) == 0 {
		return out
	}
	targets := a.forbidTargets()
	on := 0
	for _, t := range targets {
		if t.forbidden {
			on++
		}
	}
	forbid := Button{ID: "group.forbid", Key: "forbid", Label: a.shell.T("Forbid"), Icon: "order_forbid", Hotkey: input.Forbid,
		Desc: a.shell.T("Colonists will not touch forbidden stacks.")}
	allow := Button{ID: "group.allow", Key: "allow", Label: a.shell.T("Allow"), Icon: "order_allow",
		Desc: a.shell.T("Let colonists use these stacks again.")}
	if a.groupForbidsBuildings() {
		forbid.ID, forbid.Run = "cmd.group.forbid", func() { a.forbidGroup(true) }
		allow.ID, allow.Run = "cmd.group.allow", func() { a.forbidGroup(false) }
		forbid.Desc = a.shell.T("Colonists will not touch forbidden stacks, doors or beds.")
		allow.Desc = a.shell.T("Let colonists use these stacks, doors and beds again.")
	}
	forbid.On = on == len(targets)
	forbid.Mixed = on > 0 && on < len(targets)
	out = append(out, forbid, allow)
	if haulMarkable(items) {
		out = append(out, Button{ID: "group.haulmark", Key: "haulmark", Label: a.shell.T("Mark for hauling"), Icon: "order_haul",
			Desc: a.shell.T("Colonists will haul the stacks found on the map.")})
	}
	return out
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
	if m.ok && m.updates == a.env().Updates && slices.Equal(m.members, a.sel.Group.Members) {
		return m.out
	}
	members := slices.Clone(a.sel.Group.Members)
	own := a.groupOwnKeys()
	sets := make([][]Button, len(members))
	for i, c := range members {
		for _, b := range a.commandsFor(c.Kind, c.ID) {
			if !own[b.mergeKey()] {
				sets[i] = append(sets[i], singleOf(b))
			}
		}
	}
	out := MergeCommands(sets, func(member int, c Button) { a.runMember(members[member], c) })
	*m = groupMemo{updates: a.env().Updates, members: members, out: out, ok: true}
	return out
}

// groupSingle are the commands of producers outside this file's reach that
// name a target or open a window of their own: not for a group.
var groupSingle = map[string]bool{
	StorageID: true, LinkStorageID: true, UnlinkStorageID: true, CasketBtnID: true,
	LoadPodsBtnID: true, LaunchPodsBtnID: true, ScanTargetBtnID: true, LaunchShipBtnID: true,
}

// singleOf marks b Single when it is one of groupSingle.
func singleOf(b Button) Button {
	b.Single = b.Single || groupSingle[b.ID]
	return b
}

// runMember runs member m's command c for one member of the group. A
// legacy command (an id, no Run or Cmds) acts on the single selection, so
// the member is selected for the call and the group put back after.
func (a Commands) runMember(m selection.Cand, c Button) {
	if c.Run != nil || len(c.Cmds) > 0 || c.Options != nil {
		a.RunCommand(c)
		return
	}
	saved := a.sel.Group
	a.sel.Select(m)
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
	if !a.sel.Grouped() {
		return
	}
	rows := a.groupRows()
	if len(rows) >= len(a.sel.Group.Members) {
		return
	}
	members := make([]selection.Cand, len(rows))
	for i, r := range rows {
		members[i] = selection.Cand{Kind: r.kind, ID: r.id}
	}
	a.sel.SetMembers(members)
}
