package ebiten

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// The selected zone's command buttons (PLAY-020), in RimWorld's order:
// Plant, Allow sowing, Allow cutting (growing zones), Expand, Shrink,
// Delete, Hide and Rename. Every id is cmd.zone.* with Run, so the grid
// answers by Run (hud_commands.go). A producer must not call a.hud().

const (
	zoneRenameModalID = "zone.rename"
	zoneRenameFieldID = "zone.rename.name"
)

// zoneIcon is the first of want and its fallbacks the art has. With no art
// loaded (tests, plain mode) it is want. When none exists it is want too,
// so the grid draws its plain initial tile.
func (a *App) zoneIcon(want string, fallbacks ...string) string {
	return a.arch.IconName(want, fallbacks...)
}

// zoneCommands are the selected zone's command buttons.
func (a *App) zoneCommands(id uint16) []inspector.Button {
	z, ok := a.arch.Zones.View(id)
	if !ok {
		return nil
	}
	var out []inspector.Button
	if z.Kind == game.ZoneGrow {
		out = append(out, a.zoneCropCmd(z), a.zoneSowCmd(z), a.zoneCutCmd(z))
	}
	if z.Kind == game.ZoneStockpile || z.Kind == game.ZoneDump {
		out = append(out, a.zonePriorityCmd(z))
	}
	return append(out, a.zoneExpandCmd(), a.zoneShrinkCmd(), a.zoneDeleteCmd(z), a.zoneHideCmd(z), a.zoneRenameCmd(z))
}

// zoneCropCmd is "Plant: <crop>": a click lists the crops above the button.
func (a *App) zoneCropCmd(z game.ZoneView) inspector.Button {
	zid, cur := z.ID, z.Crop
	return inspector.Button{ID: "cmd.zone.crop", Label: "Plant: " + architect.ZoneCropName(cur), Icon: a.zoneCropIcon(cur),
		Desc: "Pick what this zone grows. Colonists harvest or cut the old plants and sow the new crop.",
		Run: func() {
			x, y := a.insp.Commands().Anchor("cmd.zone.crop")
			entries, names := a.zoneCropEntries(cur)
			a.ord.OpenPick(x, y, true, entries, func(i int) { a.pickZoneCrop(zid, cur, names[i]) })
		}}
}

// zonePriorityCmd is "Priority: <level>" on a stockpile or dumping
// stockpile (PLAY-023): a click lists the five priorities above the
// button and the pick sends SetZonePriority.
func (a *App) zonePriorityCmd(z game.ZoneView) inspector.Button {
	zid, cur := z.ID, architect.ZonePriorityName(z)
	return inspector.Button{ID: "cmd.zone.priority", Label: "Priority: " + cur, Icon: a.zoneIcon("zone_priority", "zone_stockpile"),
		Desc: "Haulers fill higher-priority storage first and carry things up from lower-priority storage when there is room.",
		Run: func() {
			x, y := a.insp.Commands().Anchor("cmd.zone.priority")
			names := game.StoragePriorities()
			entries := make([]game.OrderOption, len(names))
			for i, n := range names {
				entries[i] = game.OrderOption{Label: n}
				if n == cur {
					entries[i].Label += " (current)"
				}
			}
			a.ord.OpenPick(x, y, true, entries, func(i int) { a.pickZonePriority(zid, cur, names, i) })
		}}
}

// pickZonePriority sends SetZonePriority for the picked level (nothing
// when it is the zone's own).
func (a *App) pickZonePriority(zid uint16, cur string, names []string, i int) {
	if i < 0 || i >= len(names) || names[i] == cur {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.SetZonePriority, Target: uint64(zid), Index: i})
}

// zoneCropIcon is the crop's produce icon, else the growing-zone icon.
func (a *App) zoneCropIcon(crop string) string {
	for _, c := range architect.ZoneSpecies() {
		if c.Name == crop {
			return a.zoneIcon(strings.ReplaceAll(c.Label, " ", "_"), "zone_grow")
		}
	}
	return a.zoneIcon("zone_grow")
}

// zoneCropEntries are the picker's rows and the species name of each:
// every crop and tree a zone can sow, those research has not unlocked
// disabled with the project they wait for (as the Architect's crop
// buttons), the zone's own marked.
func (a *App) zoneCropEntries(cur string) ([]game.OrderOption, []string) {
	var out []game.OrderOption
	var names []string
	for _, c := range architect.ZoneSpecies() {
		o := game.OrderOption{Label: architect.CropLabel(c.Name)}
		if c.Name == cur {
			o.Label += " (current)"
		}
		if p := a.win.Research().LockedBy("crop:" + c.Name); p != "" {
			o.Disabled, o.Reason = true, "Needs research: "+p
		}
		out = append(out, o)
		names = append(names, c.Name)
	}
	return out, names
}

// pickZoneCrop sends SetZoneCrop for the picked species (nothing when it
// is the zone's own).
func (a *App) pickZoneCrop(zid uint16, cur, name string) {
	if name == cur {
		return
	}
	id, ok := game.ZoneSpeciesID(name)
	if !ok {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.SetZoneCrop, Target: uint64(zid), Build: id})
}

// zoneSowCmd is the Allow sowing toggle.
func (a *App) zoneSowCmd(z game.ZoneView) inspector.Button {
	zid, allowed := z.ID, !z.NoSow
	c := inspector.Button{ID: "cmd.zone.sow", Label: "Allow sowing", Icon: a.zoneIcon("zone_sow", "zone_grow"), Toggle: true, On: allowed,
		Desc: "Colonists sow this zone. Turn it off to stop sowing here; harvesting goes on.",
		Run: func() {
			a.shell.Enqueue(command.Command{Kind: command.SetZoneSow, Target: uint64(zid), Enabled: !allowed})
		}}
	if !allowed {
		c.Desc = "Sowing is off here: nobody sows this zone. Harvesting goes on. Turn it on to sow again."
	}
	return c
}

// zoneCutCmd is the Allow cutting toggle.
func (a *App) zoneCutCmd(z game.ZoneView) inspector.Button {
	zid, allowed := z.ID, !z.NoCut
	c := inspector.Button{ID: "cmd.zone.cut", Label: "Allow cutting", Icon: a.zoneIcon("zone_cut", "order_cut"), Toggle: true, On: allowed,
		Desc: "Growers cut the other plants in the way and chop trees here. Turn it off to leave them; their tiles stay unsown.",
		Run: func() {
			a.shell.Enqueue(command.Command{Kind: command.SetZoneCut, Target: uint64(zid), Enabled: !allowed})
		}}
	if !allowed {
		c.Desc = "Cutting is off here: growers leave other plants and trees alone, so their tiles stay unsown. Turn it on to clear them."
	}
	return c
}

// zoneExpandCmd starts the Expand tool.
func (a *App) zoneExpandCmd() inspector.Button {
	return inspector.Button{ID: "cmd.zone.expand", Label: "Expand", Icon: a.zoneIcon("zone_expand", "zone_grow"),
		Toggle: true, On: a.arch.Zones.On(architect.ZoneToolExpand),
		Desc: "Drag a rectangle to add tiles to this zone. Stays on until right-click or Esc.",
		Run:  func() { a.arch.Zones.Start(architect.ZoneToolExpand) }}
}

// zoneShrinkCmd starts the Shrink tool.
func (a *App) zoneShrinkCmd() inspector.Button {
	return inspector.Button{ID: "cmd.zone.shrink", Label: "Shrink", Icon: a.zoneIcon("zone_shrink", "plan_clear"),
		Toggle: true, On: a.arch.Zones.On(architect.ZoneToolShrink),
		Desc: "Drag a rectangle to remove this zone's tiles (a click removes one). Other zones are untouched. Stays on until right-click or Esc.",
		Run:  func() { a.arch.Zones.Start(architect.ZoneToolShrink) }}
}

// zoneDeleteCmd deletes the zone at once.
func (a *App) zoneDeleteCmd(z game.ZoneView) inspector.Button {
	zid := z.ID
	return inspector.Button{ID: "cmd.zone.delete", Label: "Delete", Icon: a.zoneIcon("zone_delete", "order_cancel"),
		Desc: "Remove the whole zone. Its plants and items stay where they are.",
		Run: func() {
			a.shell.Enqueue(command.Command{Kind: command.DeleteZone, Target: uint64(zid)})
			a.arch.Zones.Clear()
		}}
}

// zoneHideCmd is the Hide toggle: the zone's overlay on the map.
func (a *App) zoneHideCmd(z game.ZoneView) inspector.Button {
	zid, hidden := z.ID, z.Hidden
	c := inspector.Button{ID: "cmd.zone.hide", Label: "Hide", Icon: a.zoneIcon("zone_hide", "order_forbid"), Toggle: true, On: hidden,
		Desc: "Hide this zone's fill, outline and name on the map. It keeps working; click its tiles to select it.",
		Run: func() {
			a.shell.Enqueue(command.Command{Kind: command.SetZoneHidden, Target: uint64(zid), Enabled: !hidden})
		}}
	if hidden {
		c.Desc = "This zone's overlay is hidden on the map while it is not selected. Turn it off to show it again."
	}
	return c
}

// zoneRenameCmd opens the name dialog.
func (a *App) zoneRenameCmd(z game.ZoneView) inspector.Button {
	return inspector.Button{ID: "cmd.zone.rename", Label: "Rename", Icon: a.zoneIcon("zone_rename"),
		Desc: "Give this zone a name of your own. It shows on the map and in this pane.",
		Run:  func() { a.openZoneRename(z) }}
}

// openZoneRename opens the name dialog for z: a text field holding the
// zone's own name (empty while it has the default). Enter or Rename sends
// RenameZone with the trimmed name, Esc or Cancel nothing.
func (a *App) openZoneRename(z game.ZoneView) {
	zid, name := z.ID, z.Name
	placeholder := architect.ZoneDefaultName(z)
	done, focused := false, false
	send := func(a *App) {
		if done {
			return
		}
		done = true
		a.shell.Enqueue(command.Command{Kind: command.RenameZone, Target: uint64(zid), Name: zoneRenameName(name)})
	}
	a.uiPushModal(modal{
		ID: zoneRenameModalID, Title: "Rename " + strings.ToLower(architect.ZoneKindName(z)), W: 380, H: 150,
		Draw: func(a *App, body gfx.Rect) {
			a.uiSt().Label(body.X, body.Y, "Name it, or leave it empty for the default.", uikit.DimText())
			if !focused { // the field takes the keyboard on its first frame
				focused = true
				a.uiSt().TabTo = zoneRenameFieldID
			}
			field := gfx.Rect{X: body.X, Y: body.Y + 26, W: body.W, H: 28}
			if _, committed := a.uiSt().TextField(zoneRenameFieldID, field, &name, uikit.TextFieldSpec{
				MaxLen: game.ZoneNameMax, Placeholder: placeholder, Normalize: strings.TrimSpace}); committed {
				send(a)
				a.win.CloseModal(zoneRenameModalID)
			}
		},
		Buttons: []uikit.ModalButton{
			{ID: "ok", Label: "Rename", Style: uikit.BtnPrimary, Closes: true, OnClick: func() { send(a) }},
			{ID: "cancel", Label: "Cancel", Closes: true},
		},
	})
}

// zoneRenameName is the name sent: trimmed, at most ZoneNameMax characters.
func zoneRenameName(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > game.ZoneNameMax {
		s = strings.TrimSpace(string(r[:game.ZoneNameMax]))
	}
	return s
}
