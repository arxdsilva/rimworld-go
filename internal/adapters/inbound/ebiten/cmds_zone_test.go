package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// entityOf is zone id as a selection candidate's id.
func entityOf(id uint16) entity.EntityID { return entity.EntityID(id) }

// PLAY-020 ZONEUI: the selected zone's command buttons (cmds_zone.go).

// zoneCmdsOf selects zone id and returns its commands.
func zoneCmdsOf(a *App, id uint16) []cmdButton {
	a.selectCand(selCand{Kind: selZone, ID: entityOf(id)})
	return a.zoneCommands(id)
}

func zoneCmdIDs(cs []cmdButton) string { return strings.Join(thingIDs(cs), " ") }

func lastZoneCmd(t *testing.T, s *zoneSim) command.Command {
	t.Helper()
	if len(s.cmds) == 0 {
		t.Fatal("no command enqueued")
	}
	return s.cmds[len(s.cmds)-1]
}

func TestZoneCommandsComeInRimWorldsOrderPerKind(t *testing.T) {
	t.Parallel()
	a, _, _ := zoneApp(t,
		growZone(1, "potato", 0, 0, 1, 1),
		rectZone(2, game.ZoneStockpile, 3, 0, 4, 1),
		rectZone(3, game.ZonePen, 6, 0, 7, 1))
	tail := " cmd.zone.expand cmd.zone.shrink cmd.zone.delete cmd.zone.hide cmd.zone.rename"
	for _, c := range []struct {
		id   uint16
		want string
	}{
		{1, "cmd.zone.crop cmd.zone.sow cmd.zone.cut" + tail},
		{2, "cmd.zone.priority" + tail}, // PLAY-023: storage priority
		{3, strings.TrimSpace(tail)},
	} {
		cs := zoneCmdsOf(a, c.id)
		if got := zoneCmdIDs(cs); got != c.want {
			t.Errorf("zone %d commands = %q, want %q", c.id, got, c.want)
		}
		for _, cm := range cs {
			if cm.Run == nil || cm.Label == "" || cm.Desc == "" || cm.Icon == "" {
				t.Errorf("%s: needs a Run, a label, a description and an icon: %+v", cm.ID, cm)
			}
		}
	}
}

func TestZoneIconsFallBackToExistingOnesUntilTheNewOnesLand(t *testing.T) {
	t.Parallel()
	a, _, _ := zoneApp(t, growZone(1, "potato", 0, 0, 1, 1))
	cs := zoneCmdsOf(a, 1)
	want := map[string]string{"cmd.zone.sow": "zone_sow", "cmd.zone.cut": "zone_cut", "cmd.zone.expand": "zone_expand",
		"cmd.zone.shrink": "zone_shrink", "cmd.zone.delete": "zone_delete", "cmd.zone.hide": "zone_hide", "cmd.zone.rename": "zone_rename"}
	for id, icon := range want {
		if got := thingCmd(t, cs, id).Icon; got != icon {
			t.Errorf("%s icon %q, want %q with no art loaded", id, got, icon)
		}
	}
	// With an art catalog that has only the old icons, each falls back.
	art := &artCatalog{Sprites: map[string]*artSprite{}}
	for _, k := range []string{"zone_grow", "order_cancel", "plan_clear", "order_cut", "order_forbid"} {
		art.Sprites["ui_icon/"+k] = fakeSprite([2]int{64, 64}, pt{}, nil)
	}
	a.art = art
	cs = a.zoneCommands(1)
	for id, icon := range map[string]string{"cmd.zone.sow": "zone_grow", "cmd.zone.cut": "order_cut", "cmd.zone.expand": "zone_grow",
		"cmd.zone.shrink": "plan_clear", "cmd.zone.delete": "order_cancel", "cmd.zone.hide": "order_forbid"} {
		if got := thingCmd(t, cs, id).Icon; got != icon {
			t.Errorf("%s fell back to %q, want %q", id, got, icon)
		}
	}
	// Once the new icon exists it wins.
	art.Sprites["ui_icon/zone_sow"] = fakeSprite([2]int{64, 64}, pt{}, nil)
	if got := thingCmd(t, a.zoneCommands(1), "cmd.zone.sow").Icon; got != "zone_sow" {
		t.Errorf("sow icon %q once ART's lands", got)
	}
}

func TestZoneToggleCommands(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		id    string
		kind  command.Kind
		set   func(z *game.ZoneView, off bool)
		onIs  string // On while the setting is "allowed" or "visible"
		label string
	}{
		{"cmd.zone.sow", command.SetZoneSow, func(z *game.ZoneView, off bool) { z.NoSow = off }, "allowed", "Allow sowing"},
		{"cmd.zone.cut", command.SetZoneCut, func(z *game.ZoneView, off bool) { z.NoCut = off }, "allowed", "Allow cutting"},
		{"cmd.zone.hide", command.SetZoneHidden, func(z *game.ZoneView, off bool) { z.Hidden = !off }, "hidden", "Hide"},
	} {
		for _, off := range []bool{false, true} {
			z := growZone(4, "potato", 0, 0, 1, 1)
			c.set(&z, off)
			a, s, _ := zoneApp(t, z)
			cm := thingCmd(t, zoneCmdsOf(a, 4), c.id)
			wantOn := !off // sow and cut: On = allowed
			if c.id == "cmd.zone.hide" {
				wantOn = z.Hidden
			}
			if !cm.Toggle || cm.On != wantOn || cm.Label != c.label {
				t.Errorf("%s (off=%v): toggle=%v on=%v label=%q, want a toggle, on=%v, %q", c.id, off, cm.Toggle, cm.On, cm.Label, wantOn, c.label)
			}
			thingPress(a, cm)
			got := lastZoneCmd(t, s)
			wantEnabled := !wantOn // a press flips the setting
			if got.Kind != c.kind || got.Target != 4 || got.Enabled != wantEnabled {
				t.Errorf("%s (off=%v) enqueued %+v, want kind %d on zone 4 Enabled=%v", c.id, off, got, c.kind, wantEnabled)
			}
		}
	}
}

func TestZonePlantCommandListsCropsAndEnqueuesSetZoneCrop(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(4, "potato", 0, 0, 1, 1))
	s.world.Locked = map[string]string{"crop:corn": "Agriculture"}
	cm := thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.crop")
	if cm.Label != "Plant: Potato" {
		t.Fatalf("label %q, want the zone's crop", cm.Label)
	}
	thingPress(a, cm)
	if !a.float.open || a.float.pick == nil {
		t.Fatal("the Plant button opens the crop list")
	}
	idx := map[string]int{}
	for i, o := range a.float.opts {
		idx[strings.ToLower(strings.TrimSuffix(o.Label, " (current)"))] = i
	}
	if len(idx) != len(zoneSpecies()) {
		t.Fatalf("%d crops listed, the architect's picker has %d", len(idx), len(zoneSpecies()))
	}
	if !strings.HasSuffix(a.float.opts[idx["potato"]].Label, "(current)") {
		t.Errorf("the zone's own crop is marked: %q", a.float.opts[idx["potato"]].Label)
	}
	if o := a.float.opts[idx["corn"]]; !o.Disabled || o.Reason != "Needs research: Agriculture" {
		t.Errorf("corn is locked like the architect's: %+v", o)
	}
	// A locked crop only shows its reason.
	a.chooseFloat("float." + strconv.Itoa(idx["corn"]))
	if len(s.cmds) != 0 || a.hint != "Needs research: Agriculture" {
		t.Fatalf("locked pick: cmds %v hint %q", s.cmds, a.hint)
	}
	// The zone's own crop sends nothing.
	a.chooseFloat("float." + strconv.Itoa(idx["potato"]))
	if len(s.cmds) != 0 {
		t.Fatalf("picking the current crop sent %+v", s.cmds)
	}
	// Wheat goes out as SetZoneCrop with the species id.
	thingPress(a, cm)
	a.chooseFloat("float." + strconv.Itoa(idx["wheat"]))
	wheat, _ := game.ZoneSpeciesID("wheat")
	got := lastZoneCmd(t, s)
	if got.Kind != command.SetZoneCrop || got.Target != 4 || got.Build != wheat {
		t.Fatalf("picked wheat: %+v, want SetZoneCrop on zone 4 with Build %d", got, wheat)
	}
	if a.float.open {
		t.Fatal("picking closes the list")
	}
}

func TestZonePlantListHasTreesToo(t *testing.T) {
	t.Parallel()
	a, _, _ := zoneApp(t, growZone(4, "tree", 0, 0, 1, 1))
	cm := thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.crop")
	if cm.Label != "Plant: Tree" {
		t.Fatalf("label %q", cm.Label)
	}
	thingPress(a, cm)
	var trees int
	for _, o := range a.float.opts {
		if strings.Contains(strings.ToLower(o.Label), "pine") {
			trees++
		}
	}
	if trees != 1 {
		t.Fatalf("pine listed %d times, want once", trees)
	}
}

func TestZoneDeleteEnqueuesAndClearsTheSelection(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(4, "potato", 0, 0, 1, 1), rectZone(5, game.ZoneStockpile, 5, 0, 6, 1))
	thingPress(a, thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.delete"))
	if got := lastZoneCmd(t, s); got.Kind != command.DeleteZone || got.Target != 4 {
		t.Fatalf("delete enqueued %+v", got)
	}
	if a.sel.HasZone {
		t.Fatal("Delete clears the selection")
	}
	if len(s.cmds) != 1 {
		t.Fatalf("one command, got %d", len(s.cmds))
	}
}

func TestZoneExpandAndShrinkButtonsArmTheTools(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(4, "potato", 8, 2, 9, 3))
	exp := thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.expand")
	if exp.On {
		t.Fatal("no tool is on yet")
	}
	thingPress(a, exp)
	if !a.zoneToolOn(zoneToolExpand) || !thingCmd(t, a.zoneCommands(4), "cmd.zone.expand").On {
		t.Fatal("Expand arms its tool and shows on")
	}
	thingPress(a, thingCmd(t, a.zoneCommands(4), "cmd.zone.shrink"))
	if a.zoneToolOn(zoneToolExpand) || !a.zoneToolOn(zoneToolShrink) {
		t.Fatal("Shrink replaces Expand")
	}
	thingPress(a, thingCmd(t, a.zoneCommands(4), "cmd.zone.shrink"))
	if a.zoneToolOn(zoneToolShrink) {
		t.Fatal("pressing the armed tool again turns it off")
	}
	if len(s.cmds) != 0 {
		t.Fatalf("arming a tool sends nothing, got %+v", s.cmds)
	}
}

// ---- the Rename dialog ----

func TestZoneRenameDialogSendsTheTrimmedName(t *testing.T) {
	t.Parallel()
	a, s, f := zoneApp(t, growZone(4, "potato", 0, 0, 1, 1))
	thingPress(a, thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.rename"))
	f.Frame(t, a)
	if a.uiModalTop() != zoneRenameModalID {
		t.Fatalf("top modal %q, want the rename dialog", a.uiModalTop())
	}
	f.TypeText(t, a, "  North field  ")
	f.Key(t, a, "Enter", 0)
	f.Frame(t, a)
	got := cmdsOf(&stubSim{cmds: s.cmds}, command.RenameZone)
	if len(got) != 1 || got[0].Target != 4 || got[0].Name != "North field" {
		t.Fatalf("rename enqueued %+v, want one RenameZone \"North field\" on zone 4", got)
	}
	if a.uiModalTop() == zoneRenameModalID {
		t.Fatal("Enter closes the dialog")
	}
}

func TestZoneRenameDialogOkButtonAndCap(t *testing.T) {
	t.Parallel()
	a, s, f := zoneApp(t, growZone(4, "potato", 0, 0, 1, 1))
	thingPress(a, thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.rename"))
	f.Frame(t, a)
	f.TypeText(t, a, "abcdefghijklmnopqrstuvwxyz0123") // 30 characters
	press(t, a, f, "modal."+zoneRenameModalID+".ok")
	f.Frame(t, a)
	got := cmdsOf(&stubSim{cmds: s.cmds}, command.RenameZone)
	if len(got) != 1 || got[0].Name != "abcdefghijklmnopqrstuvwx" || len([]rune(got[0].Name)) != game.ZoneNameMax {
		t.Fatalf("rename enqueued %+v, want one name cut to %d characters", got, game.ZoneNameMax)
	}
}

func TestZoneRenameDialogCancelAndEmptyName(t *testing.T) {
	t.Parallel()
	named := growZone(4, "potato", 0, 0, 1, 1)
	named.Name = "Old"
	a, s, f := zoneApp(t, named)
	thingPress(a, thingCmd(t, zoneCmdsOf(a, 4), "cmd.zone.rename"))
	f.Frame(t, a)
	f.TypeText(t, a, "xyz")
	press(t, a, f, "modal."+zoneRenameModalID+".cancel")
	f.Frame(t, a)
	if len(s.cmds) != 0 || a.uiModalTop() == zoneRenameModalID {
		t.Fatalf("Cancel sends nothing and closes: cmds %+v modal %q", s.cmds, a.uiModalTop())
	}
	// Emptying the field restores the default name ("" is the default).
	thingPress(a, thingCmd(t, a.zoneCommands(4), "cmd.zone.rename"))
	f.Frame(t, a)
	for range 3 { // "Old"
		f.Key(t, a, "Backspace", 0)
	}
	f.Key(t, a, "Enter", 0)
	f.Frame(t, a)
	got := cmdsOf(&stubSim{cmds: s.cmds}, command.RenameZone)
	if len(got) != 1 || got[0].Name != "" || got[0].Target != 4 {
		t.Fatalf("empty name enqueued %+v, want RenameZone with Name \"\"", got)
	}
}

func TestZoneRenameName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"  hi  ":                    "hi",
		"":                          "",
		"   ":                       "",
		"aaaaaaaaaaaaaaaaaaaaaaaa":  "aaaaaaaaaaaaaaaaaaaaaaaa",
		"aaaaaaaaaaaaaaaaaaaaaaaaZ": "aaaaaaaaaaaaaaaaaaaaaaaa",
		"aaaaaaaaaaaaaaaaaaaaaaa b": "aaaaaaaaaaaaaaaaaaaaaaa",
		"ñññññññññññññññññññññññññññññññññññ": strings.Repeat("ñ", game.ZoneNameMax),
	} {
		if got := zoneRenameName(in); got != want {
			t.Errorf("zoneRenameName(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- the grid and the pane through the layout, at every UI scale ----

// Every kind's pane is up, its commands each have a grid cell and a hit
// target, and no region overlaps another, at UI scales 1, 1.5 and 2 on the
// default, a big and a small screen. It sets the UI scale: not parallel.
func TestZonePaneAndGridAtUIScales(t *testing.T) {
	zones := []game.ZoneView{
		growZone(1, "potato", 10, 10, 15, 14),
		rectZone(2, game.ZoneStockpile, 20, 10, 24, 12),
		rectZone(3, game.ZonePen, 30, 10, 34, 12),
	}
	zones[0].Name = "A rather long field name to be sure it wraps"
	for _, ui := range []float64{1, 1.5, 2} {
		for _, sz := range layoutScreens {
			s := busyStub(6)
			s.world.Zones = zones
			a, f := colonyApp(t, s)
			sw, sh := layoutScreen(t, a, sz[0], sz[1], ui)
			for _, z := range zones {
				name := zoneKindName(z) + " ui" + strconv.FormatFloat(ui, 'g', -1, 64) + " " + strconv.Itoa(sw) + "x" + strconv.Itoa(sh)
				a.selectCand(selCand{Kind: selZone, ID: entityOf(z.ID)})
				f.Frame(t, a)
				l := a.hud()
				if l.Pane.Empty() || !a.paneShown() {
					t.Fatalf("%s: no pane", name)
				}
				if bad := layoutOverlaps(l, sw, sh); len(bad) > 0 {
					t.Fatalf("%s:\n\t%s", name, strings.Join(bad, "\n\t"))
				}
				assertChromeIsTheLayout(t, a)
				cs := a.selectionCommands()
				if len(cs) == 0 || len(l.GridCells) != len(cs) {
					t.Fatalf("%s: %d commands, %d cells", name, len(cs), len(l.GridCells))
				}
				hits := append(a.layoutHits(a.sw, a.sh), a.colonyButtons(a.sw, a.sh)...)
				for i, cm := range cs {
					b, ok := hasButton(hits, cm.ID)
					if !ok {
						t.Errorf("%s: %s has no hit target", name, cm.ID)
						continue
					}
					if got := (rect{X: b.X, Y: b.Y, W: b.W, H: b.H}); got != l.GridCells[i] {
						t.Errorf("%s: %s is at %+v, its cell is %+v", name, cm.ID, got, l.GridCells[i])
					}
					if cell := l.GridCells[i]; cell.X < l.Pane.Right() || cell.Right() > l.Global.X {
						t.Errorf("%s: cell %+v of %s outside the grid band", name, cell, cm.ID)
					}
				}
				if _, ok := hasButton(hits, "zone.close"); !ok {
					t.Errorf("%s: no close button", name)
				}
				px, py := l.Pane.Center()
				if !a.colonyChromeHit(int(px), int(py), a.sw, a.sh) {
					t.Errorf("%s: a click on the pane would reach the map", name)
				}
			}
		}
	}
}
