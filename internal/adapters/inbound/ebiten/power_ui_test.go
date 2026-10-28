package ebiten

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// PLAY-021 (POWERUI): the Architect's power tools, the "Power grid"
// overlay toggle, its auto-show, the overlay's drawing and the placement
// ghost's link.

// toolIDsIn are the tool ids of category cat, in catalog order.
func toolIDsIn(cat string) []string {
	var out []string
	for _, t := range toolsIn(cat) {
		out = append(out, t.id)
	}
	return out
}

func TestPowerArchitectTools(t *testing.T) {
	t.Parallel()
	listed := map[string]bool{}
	for _, c := range archCategories {
		listed[c.id] = true
	}
	if !listed[archCatPower] {
		t.Fatal("no Power category")
	}
	// A ninth category overflows the four-row panel: the heater and cooler
	// sit in Misc (PLAY021_PLAN fallback), so Temperature stays unlisted.
	if len(archCategories) > 8 || listed[archCatTemperature] {
		t.Fatalf("%d categories (Temperature listed %v): the panel holds eight", len(archCategories), listed[archCatTemperature])
	}
	want := []string{"conduit", "hidden_conduit", "power_switch", "wood_generator", "solar_panel", "wind_turbine", "geothermal", "battery"}
	if got := toolIDsIn(archCatPower); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Power tools %v, want %v", got, want)
	}
	for _, id := range []string{"conduit", "hidden_conduit"} {
		if tl, _ := toolByID(id); tl.mode != placeLine {
			t.Errorf("%s places in mode %v, want a line", id, tl.mode)
		}
	}
	furn := strings.Join(toolIDsIn("furniture"), ",")
	if !strings.Contains(furn, "lamp,torch_lamp") {
		t.Errorf("Furniture %s: the torch lamp belongs beside the lamp", furn)
	}
	if !strings.Contains(furn, "campfire") {
		t.Errorf("Furniture %s lost the campfire", furn)
	}
	misc := strings.Join(toolIDsIn("misc"), ",")
	if !strings.Contains(misc, "heater") || !strings.Contains(misc, "cooler") {
		t.Errorf("Misc %s lacks the heater and cooler", misc)
	}
	prod := strings.Join(toolIDsIn("production"), ",")
	for _, pair := range []string{"stove,electric_stove", "tailor,electric_tailor", "research,electric_research"} {
		if !strings.Contains(prod, pair) {
			t.Errorf("Production %s: want %s side by side", prod, pair)
		}
	}
	// every new tool builds a real kind, has its own icon and words
	kinds := map[string]string{"conduit": conduitKind, "hidden_conduit": hiddenConduitKind, "power_switch": powerSwitchKind,
		"wood_generator": woodGeneratorKind, "solar_panel": solarPanelKind, "wind_turbine": windTurbineKind,
		"geothermal": geothermalKind, "battery": batteryKind, "torch_lamp": torchLampKind, "heater": heaterKind,
		"cooler": coolerKind, "electric_stove": "electric stove", "electric_tailor": "electric tailoring bench",
		"electric_research": "electric research bench"}
	root := repoRoot(t)
	descs := map[string]string{}
	for _, tl := range allArchTools() {
		if prev, dup := descs[tl.desc]; dup {
			t.Errorf("%s and %s share a description", tl.id, prev)
		}
		descs[tl.desc] = tl.id
	}
	for id, kind := range kinds {
		tl, ok := toolByID(id)
		if !ok {
			t.Errorf("no tool %s", id)
			continue
		}
		if tl.build != kind {
			t.Errorf("%s builds %q, want %q", id, tl.build, kind)
		}
		if _, ok := buildable(tl.build); !ok {
			t.Errorf("%s: %q is not in the buildable catalog", id, tl.build)
		}
		if len(tl.desc) < 30 {
			t.Errorf("%s: desc %q is too thin", id, tl.desc)
		}
		if _, err := os.Stat(filepath.Join(root, "assets", "metadata", "ui_icon", tl.icon+".yaml")); err != nil {
			t.Errorf("%s: icon ui_icon/%s has no art: %v", id, tl.icon, err)
		}
	}
}

// repoRoot walks up from the package to the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			t.Fatal("no go.mod above the package")
		}
		dir = up
	}
}

// Every electrical card names its draw, output or store from the
// simulation's own numbers; conduits and switches carry power only.
func TestArchPowerLine(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		heaterKind:        "Power: uses 180 W working, 20 W idle",
		coolerKind:        "Power: uses 220 W working, 20 W idle",
		"lamp":            "Power: uses 50 W",
		"electric stove":  "Power: uses 250 W working, 25 W idle",
		woodGeneratorKind: "Power: makes up to 800 W",
		geothermalKind:    "Power: makes up to 1800 W",
		batteryKind:       "Power: stores 1200 Wh",
		conduitKind:       "",
		powerSwitchKind:   "",
		torchLampKind:     "",
		"wall":            "",
	}
	for kind, want := range cases {
		bv, ok := buildable(kind)
		if !ok {
			t.Fatalf("%s is not buildable", kind)
		}
		if got := archPowerLine(bv); got != want {
			t.Errorf("%s: %q, want %q", kind, got, want)
		}
	}
	heater, _ := toolByID("heater")
	lines := archTooltipLines(heater, "steel", 0, 0)
	if !strings.Contains(strings.Join(lines, "\n"), "Power: uses 180 W working") {
		t.Errorf("heater card %q lacks its power line", lines)
	}
	if power.ConnectRange != 3 || !strings.Contains(toolDesc(t, "conduit"), "within 3 tiles") {
		t.Errorf("the conduit card names the link range: %q, range %d", toolDesc(t, "conduit"), power.ConnectRange)
	}
}

func toolDesc(t *testing.T, id string) string {
	t.Helper()
	tl, ok := toolByID(id)
	if !ok {
		t.Fatalf("no tool %s", id)
	}
	return tl.desc
}

// electricityLocked is the Locked map of a colony that has not researched
// Electricity (and so none of the power projects after it).
func electricityLocked() map[string]string {
	return map[string]string{
		"build:power conduit": "Electricity", "build:hidden conduit": "Electricity", "build:power switch": "Electricity",
		"build:wood-fired generator": "Electricity", "build:lamp": "Electricity", "build:heater": "Electricity",
		"build:cooler": "Electricity", "build:electric stove": "Electricity", "build:electric tailoring bench": "Electricity",
		"build:electric research bench": "Electricity", "build:battery": "Batteries", "build:solar panel": "Solar power",
		"build:wind turbine": "Wind power", "build:geothermal generator": "Geothermal power",
	}
}

// Before Electricity every power card is disabled with the research that
// opens it; the torch lamp is free; once researched they arm.
func TestPowerToolsLockedBeforeElectricity(t *testing.T) {
	t.Parallel()
	s := &stubSim{world: game.WorldView{Locked: electricityLocked()}}
	a, _ := powerArch(t, s)
	check := func(cat, tool, why string) {
		t.Helper()
		a.archCat = cat
		c, ok := archCmd(a, "arch.tool."+tool)
		if !ok {
			t.Fatalf("no card %s in %s", tool, cat)
		}
		if got := c.Reason; c.Disabled != (why != "") || got != why {
			t.Errorf("%s: disabled %v %q, want %q", tool, c.Disabled, got, why)
		}
	}
	check(archCatPower, "conduit", "Needs research: Electricity")
	check(archCatPower, "battery", "Needs research: Batteries")
	check(archCatPower, "geothermal", "Needs research: Geothermal power")
	check("misc", "heater", "Needs research: Electricity")
	check("furniture", "lamp", "Needs research: Electricity")
	check("furniture", "torch_lamp", "")
	check("production", "electric_stove", "Needs research: Electricity")
	archPick(t, a, "electric_stove")
	if a.archTool != "" || !strings.Contains(a.hint, "Electricity") {
		t.Errorf("a locked card armed %q (hint %q)", a.archTool, a.hint)
	}
	s.world.Locked = nil
	check(archCatPower, "conduit", "")
	archPick(t, a, "conduit")
	if a.archTool != "conduit" {
		t.Errorf("an open card did not arm: %q", a.archTool)
	}
}

// powerArch is a colony over stub s with the Architect open.
func powerArch(t *testing.T, s *stubSim) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s)
	a.openArchitect()
	return a, f
}

// The Power grid toggle is the ninth: it wraps to a second row, sits
// inside Global at every UI scale, and flips only the power overlay.
func TestPowerToggleWrapsAndToggles(t *testing.T) {
	if n := len(globalToggleList); n <= globalPerRow {
		t.Fatalf("%d toggles: the power toggle should be the one that wraps", n)
	}
	last := globalToggleList[len(globalToggleList)-1]
	if last.id != powerButtonID || last.icon != "overlay_power" {
		t.Fatalf("last toggle %s/%s, want %s/overlay_power", last.id, last.icon, powerButtonID)
	}
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			s := newGlobalSim()
			a, f := globalColony(t, s, sc.ui, sc.ww, sc.wh)
			g := a.hud().Global
			pr, ok := a.globalToggleRect(powerButtonID)
			if !ok {
				t.Fatal("no power toggle rect")
			}
			first, _ := a.globalToggleRect(roofButtonID)
			if pr.Y <= first.Y || pr.Right() != g.Right()-globalPad {
				t.Errorf("power toggle %+v: want a second row, right-aligned below %+v in %+v", pr, first, g)
			}
			if pr.X < g.X || pr.Bottom() > g.Bottom() || pr.Y < g.Y {
				t.Errorf("power toggle %+v outside Global %+v", pr, g)
			}
			if _, ok := hasButton(a.layoutHits(a.sw, a.sh), powerButtonID); !ok {
				t.Error("no hit target for the power toggle")
			}
			if !strings.Contains(tipOf(t, a, powerButtonID), "Power grid") {
				t.Errorf("tip %q", tipOf(t, a, powerButtonID))
			}
			before := a.overlayFlags()
			f.onRect(a, pr)
			f.click(t, a, mbLeft, 0)
			if !a.renderer.pov.on || !a.renderer.powerShown() {
				t.Fatal("the click did not show the power overlay")
			}
			if a.overlayFlags() != before {
				t.Errorf("the power toggle changed other overlays: %+v -> %+v", before, a.overlayFlags())
			}
			if a.globalPlan().legend == nil {
				t.Error("no legend while the power overlay is on")
			}
			f.click(t, a, mbLeft, 0)
			if a.renderer.powerShown() {
				t.Error("a second click did not hide it")
			}
		})
	}
}

// Shift+P toggles the overlay from the map; its tip names the key.
func TestPowerOverlayHotkey(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, &stubSim{})
	f.key(t, a, "P", input.ModShift)
	if !a.renderer.pov.on {
		t.Fatal("Shift+P did not show the power overlay")
	}
	f.key(t, a, "P", input.ModShift)
	if a.renderer.pov.on {
		t.Fatal("Shift+P again did not hide it")
	}
	if tip := a.powerToggleTip(); !strings.Contains(tip, "Shift") || !strings.Contains(tip, "P") {
		t.Errorf("tip %q lacks the key", tip)
	}
}

// The overlay shows by itself while a power tool or any powered building
// is in hand, and goes again with the tool; the player's toggle is kept.
func TestPowerOverlayAutoShows(t *testing.T) {
	t.Parallel()
	a, _ := powerArch(t, &stubSim{})
	r := a.renderer
	for _, c := range []struct {
		cat, tool string
		forced    bool
	}{
		{archCatPower, "conduit", true}, {archCatPower, "hidden_conduit", true}, {archCatPower, "power_switch", true},
		{archCatPower, "battery", true}, {"misc", "heater", true}, {"misc", "cooler", true}, {"furniture", "lamp", true},
		{"production", "electric_research", true}, {"furniture", "torch_lamp", false}, {"structure", "wall", false},
		{"misc", "party_spot", false},
	} {
		a.archTool, a.archCat = "", c.cat
		archPick(t, a, c.tool)
		if a.archTool != c.tool {
			t.Fatalf("%s did not arm", c.tool)
		}
		if r.pov.forced != c.forced || r.powerShown() != c.forced {
			t.Errorf("%s: forced %v shown %v, want %v", c.tool, r.pov.forced, r.powerShown(), c.forced)
		}
	}
	archPick(t, a, "party_spot") // drop it
	a.archCat = archCatPower
	archPick(t, a, "conduit")
	a.closeArchitect()
	if r.pov.forced || r.powerShown() {
		t.Error("closing the Architect left the overlay forced on")
	}
	r.togglePowerOverlay()
	a.openArchitect()
	a.archCat = "structure"
	archPick(t, a, "wall")
	if !r.powerShown() {
		t.Error("a non-power tool hid the player's own overlay")
	}
}

// Nets get palette colours in turn and keep them while they live: a new
// net never repaints an old one, and two live nets never share one
// while the palette has room. Net 0 (no grid) is grey.
func TestPowerNetColours(t *testing.T) {
	t.Parallel()
	var o powerOverlay
	o.syncNets([]game.PowerNetView{{ID: 5}, {ID: 9}})
	c5, c9 := o.netColour(5), o.netColour(9)
	if c5 == c9 {
		t.Fatalf("nets 5 and 9 share %v", c5)
	}
	if o.netColour(0) != tokenNRGBA(uiPowerNone) {
		t.Errorf("net 0 is %v, want the no-grid grey", o.netColour(0))
	}
	o.syncNets([]game.PowerNetView{{ID: 2}, {ID: 5}, {ID: 9}}) // a smaller id appears
	if o.netColour(5) != c5 || o.netColour(9) != c9 {
		t.Error("a new net repainted the old ones")
	}
	seen := map[color.NRGBA]bool{c5: true, c9: true}
	if seen[o.netColour(2)] {
		t.Errorf("net 2 took a colour in use: %v", o.netColour(2))
	}
	o.syncNets([]game.PowerNetView{{ID: 2}, {ID: 9}}) // net 5 is gone: its colour frees up
	o.syncNets([]game.PowerNetView{{ID: 2}, {ID: 9}, {ID: 40}})
	if o.netColour(40) != c5 {
		t.Errorf("net 40 got %v, want the freed colour %v", o.netColour(40), c5)
	}
	var many []game.PowerNetView
	for i := 1; i <= 7; i++ {
		many = append(many, game.PowerNetView{ID: i * 10})
	}
	var p powerOverlay
	p.syncNets(many)
	used := map[color.NRGBA]int{}
	for _, n := range many {
		used[p.netColour(n.ID)]++
	}
	if len(used) != len(powerPalette) {
		t.Errorf("seven nets use %d colours, want all %d", len(used), len(powerPalette))
	}
}

// povTrace is one overlay part drawn: what, where and in which colour.
type povTrace struct {
	part string
	x, y int
	c    color.NRGBA
}

// povRenderer is a renderer over the power art that records the
// overlay's parts.
func povRenderer(t *testing.T, w game.WorldView) (*Renderer, *[]povTrace, *[]string) {
	t.Helper()
	r, markings := pdRenderer(t, w)
	got := &[]povTrace{}
	r.pov.trace = func(part string, x, y int, c color.NRGBA) { *got = append(*got, povTrace{part, x, y, c}) }
	return r, got, markings
}

func povParts(list []povTrace, part string) []povTrace {
	var out []povTrace
	for _, p := range list {
		if p.part == part {
			out = append(out, p)
		}
	}
	return out
}

// povWorld is two nets and their devices:
//
//	net 1: conduits (1,0) (2,0) (3,0), a powered heater at (1,2) linked to
//	       (1,0), a brownout heater (3,2) linked to (3,0) and a switched-off
//	       lamp (5,1) linked to (3,0);
//	net 2: a hidden conduit (8,0) and a battery (8,2) half charged on it;
//	no net: a generator (12,4) with nothing in range, and a conduit under
//	       an open switch (6,6).
func povWorld() game.WorldView {
	w := game.WorldView{TerrainW: 20, Terrain: make([]uint8, 20*12), StructVer: 1,
		PowerNets: []game.PowerNetView{{ID: 1, Gen: 0, Load: 180, Brownout: true}, {ID: 2, StoredWh: 600, CapWh: 1200}}}
	id := entity.EntityID(1)
	add := func(b game.BuildingView) {
		b.ID = id
		id++
		if b.W == 0 {
			b.W, b.H = 1, 1
		}
		w.Buildings = append(w.Buildings, b)
	}
	for x := 1; x <= 3; x++ {
		add(game.BuildingView{Kind: conduitKind, X: x, Y: 0, PowerRole: "transmitter", PowerNet: 1})
	}
	add(game.BuildingView{Kind: hiddenConduitKind, X: 8, Y: 0, PowerRole: "transmitter", PowerNet: 2})
	add(game.BuildingView{Kind: heaterKind, X: 1, Y: 2, PowerRole: "consumer", PowerNet: 1, Powered: true, PowerOn: true,
		HasLink: true, LinkX: 1, LinkY: 0, LinkFromX: 1, LinkFromY: 2})
	add(game.BuildingView{Kind: heaterKind, X: 3, Y: 2, PowerRole: "consumer", PowerNet: 1, NoPowerReason: game.NoPowerBrownout,
		PowerOn: true, HasLink: true, LinkX: 3, LinkY: 0, LinkFromX: 3, LinkFromY: 2})
	add(game.BuildingView{Kind: "lamp", X: 5, Y: 1, PowerRole: "consumer", PowerNet: 1, NoPowerReason: game.NoPowerSwitchedOff,
		HasLink: true, LinkX: 3, LinkY: 0, LinkFromX: 5, LinkFromY: 1})
	add(game.BuildingView{Kind: batteryKind, X: 8, Y: 2, W: 1, H: 2, PowerRole: "storage", PowerNet: 2, StoredWh: 600, CapWh: 1200,
		HasLink: true, LinkX: 8, LinkY: 0, LinkFromX: 8, LinkFromY: 2})
	add(game.BuildingView{Kind: woodGeneratorKind, X: 12, Y: 4, W: 2, H: 2, PowerRole: "producer", PowerOn: true,
		NoPowerReason: game.NoPowerNotConnected})
	add(game.BuildingView{Kind: conduitKind, X: 6, Y: 6, PowerRole: "transmitter"})
	add(game.BuildingView{Kind: powerSwitchKind, X: 6, Y: 6, PowerRole: "switch"})
	return w
}

// The overlay draws each net's conduits in that net's one colour (hidden
// ones too, with their markings), a link line per linked device in its
// net's colour, producer, consumer and battery marks, the no-power sign
// on every unpowered consumer, battery charge and a frame round the
// device on no grid. Off, it draws nothing.
func TestPowerOverlayDrawsPerNet(t *testing.T) {
	w := povWorld()
	r, got, markings := povRenderer(t, w)
	dst := ebiten.NewImage(640, 400)
	r.drawPowerOverlay(dst, &w, 640, 400, r.tilePx())
	if len(*got) != 0 || len(*markings) != 0 {
		t.Fatalf("the overlay drew while off: %v %v", *got, *markings)
	}
	r.togglePowerOverlay()
	r.drawPowerOverlay(dst, &w, 640, 400, r.tilePx())

	n1, n2 := r.pov.netColour(1), r.pov.netColour(2)
	if n1 == n2 {
		t.Fatalf("both nets are %v", n1)
	}
	byCell := map[[2]int]color.NRGBA{}
	for _, f := range povParts(*got, "grid") {
		byCell[[2]int{f.x, f.y}] = f.c
	}
	for _, c := range [][2]int{{1, 0}, {2, 0}, {3, 0}} {
		if byCell[c] != n1 {
			t.Errorf("conduit %v is %v, want net 1's %v", c, byCell[c], n1)
		}
	}
	if byCell[[2]int{8, 0}] != n2 {
		t.Errorf("the hidden conduit is %v, want net 2's %v", byCell[[2]int{8, 0}], n2)
	}
	if byCell[[2]int{6, 6}] != tokenNRGBA(uiPowerNone) {
		t.Errorf("the conduit on no grid is %v, want grey", byCell[[2]int{6, 6}])
	}
	if pdCount(*markings, "marking ") != 5 {
		t.Errorf("markings %v, want one per conduit (hidden included)", *markings)
	}

	links := povParts(*got, "link")
	if len(links) != 4 {
		t.Fatalf("links %v, want the four linked devices", links)
	}
	for _, l := range links {
		want := n1
		if l.x == 8 {
			want = n2
		}
		if l.c != want {
			t.Errorf("link from %d,%d is %v, want %v", l.x, l.y, l.c, want)
		}
	}
	if n := len(povParts(*got, "consumer")); n != 3 {
		t.Errorf("%d consumer marks, want 3", n)
	}
	if n := len(povParts(*got, "producer")); n != 1 {
		t.Errorf("%d producer marks, want 1", n)
	}
	un := povParts(*got, "unpowered")
	if len(un) != 2 || un[0].x != 3 || un[1].x != 5 {
		t.Errorf("unpowered %v, want the brownout heater and the switched-off lamp", un)
	}
	bat := povParts(*got, "battery")
	if len(bat) != 1 || bat[0].x != 8 {
		t.Errorf("battery bars %v", bat)
	}
	dis := povParts(*got, "disconnected")
	if len(dis) != 1 || dis[0].x != 12 || dis[0].y != 4 {
		t.Errorf("disconnected %v, want the generator at 12,4", dis)
	}
}

// The overlay legend joins the global stack while it shows, after the
// roof legend when both are on.
func TestPowerLegend(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	if a.powerLegend() != "" {
		t.Fatal("a legend with the overlay off")
	}
	a.renderer.togglePowerOverlay()
	if !strings.HasPrefix(a.powerLegend(), "Power:") {
		t.Fatalf("legend %q", a.powerLegend())
	}
	a.renderer.roofOverlay = true
	all := strings.Join(a.globalPlan().legend, " ")
	if !strings.HasPrefix(all, "Roof:") || !strings.Contains(all, "Power:") {
		t.Errorf("legend %q, want the roof key then the power key", all)
	}
	for _, l := range a.globalPlan().legend {
		if textW(l, tsBody) > globalInnerW {
			t.Errorf("legend line %q is wider than Global", l)
		}
	}
}

// transAt is a transmitter predicate over the listed cells.
func transAt(cells ...[2]int) func(x, y int) bool {
	set := map[[2]int]bool{}
	for _, c := range cells {
		set[c] = true
	}
	return func(x, y int) bool { return set[[2]int{x, y}] }
}

// The ghost links like the simulation: the nearest transmitter by
// Chebyshev distance from the footprint, ties by y then x, within
// ConnectRange; the line leaves the nearest footprint cell.
func TestGhostLinkRule(t *testing.T) {
	t.Parallel()
	one := [][2]int{{5, 5}}
	cases := []struct {
		name     string
		cells    [][2]int
		trans    func(x, y int) bool
		from, to [2]int
		ok       bool
	}{
		{"in range", one, transAt([2]int{8, 5}), [2]int{5, 5}, [2]int{8, 5}, true},
		{"out of range", one, transAt([2]int{9, 5}), [2]int{}, [2]int{}, false},
		{"nearest wins", one, transAt([2]int{8, 5}, [2]int{6, 7}), [2]int{5, 5}, [2]int{6, 7}, true},
		{"tie: smaller y", one, transAt([2]int{7, 6}, [2]int{3, 4}), [2]int{5, 5}, [2]int{3, 4}, true},
		{"tie: then smaller x", one, transAt([2]int{7, 4}, [2]int{3, 4}), [2]int{5, 5}, [2]int{3, 4}, true},
		{"under it", one, transAt([2]int{5, 5}), [2]int{5, 5}, [2]int{5, 5}, true},
		{"2x2 from the near cell", [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}}, transAt([2]int{9, 7}),
			[2]int{6, 6}, [2]int{9, 7}, true},
		{"2x2, straight line breaks the tie", [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}}, transAt([2]int{9, 5}),
			[2]int{6, 5}, [2]int{9, 5}, true},
		{"off the map", [][2]int{{0, 0}}, transAt([2]int{-1, 0}), [2]int{}, [2]int{}, false},
	}
	for _, c := range cases {
		from, to, ok := ghostLink(c.cells, 20, 20, c.trans)
		if ok != c.ok || (ok && (from != c.from || to != c.to)) {
			t.Errorf("%s: %v -> %v ok %v, want %v -> %v ok %v", c.name, from, to, ok, c.from, c.to, c.ok)
		}
	}
}

// The ghost's transmitters are the built conduits and the switches that
// are on; an open switch carries nothing, not even a conduit under it.
func TestGhostTransmitters(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 10, StructVer: 3, Buildings: []game.BuildingView{
		{ID: 1, Kind: conduitKind, X: 1, Y: 1, W: 1, H: 1, PowerRole: "transmitter"},
		{ID: 2, Kind: hiddenConduitKind, X: 2, Y: 1, W: 1, H: 1, PowerRole: "transmitter"},
		{ID: 3, Kind: powerSwitchKind, X: 3, Y: 1, W: 1, H: 1, PowerRole: "switch", PowerOn: true},
		{ID: 4, Kind: powerSwitchKind, X: 4, Y: 1, W: 1, H: 1, PowerRole: "switch"},
		{ID: 5, Kind: conduitKind, X: 4, Y: 1, W: 1, H: 1, PowerRole: "transmitter"},
		{ID: 6, Kind: heaterKind, X: 5, Y: 1, W: 1, H: 1, PowerRole: "consumer"},
	}}
	var o powerOverlay
	is := o.transmitters(&w)
	for x, want := range map[int]bool{1: true, 2: true, 3: true, 4: false, 5: false} {
		if is(x, 1) != want {
			t.Errorf("cell %d,1 transmits %v, want %v", x, !want, want)
		}
	}
	w.Buildings[3].PowerOn = true // the switch is flicked on: a new power state
	w.PowerVer++
	if !o.transmitters(&w)(4, 1) {
		t.Error("the transmitter cache missed the switch turning on")
	}
}

// ghostApp is a colony with a built conduit at (4, 4) and the Architect
// holding tool, the cursor on (tx, ty).
func ghostApp(t *testing.T, tool string, tx, ty int) (*App, *[]povTrace) {
	t.Helper()
	s := &stubSim{w: 24, h: 24, world: game.WorldView{TerrainW: 24, Terrain: make([]uint8, 24*24), StructVer: 1,
		Buildings: []game.BuildingView{{ID: 1, Kind: conduitKind, X: 4, Y: 4, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 1}},
		PowerNets: []game.PowerNetView{{ID: 1}}}}
	a, f := powerArch(t, s)
	tl, _ := toolByID(tool)
	a.archCat = tl.category
	archPick(t, a, tool)
	f.moveTo(a, tx, ty)
	f.frame(t, a)
	got := &[]povTrace{}
	a.renderer.pov.trace = func(part string, x, y int, c color.NRGBA) { *got = append(*got, povTrace{part, x, y, c}) }
	return a, got
}

// A device ghost in range draws its link to the conduit; out of range it
// says "Not connected"; a conduit ghost links nothing.
func TestPlacementGhostLink(t *testing.T) {
	t.Parallel()
	screen := ebiten.NewImage(640, 400)
	a, got := ghostApp(t, "heater", 6, 6)
	a.drawArchitectGhost(screen)
	if l := povParts(*got, "ghost link"); len(l) != 1 || l[0].x != 6 || l[0].y != 6 || l[0].c != a.renderer.pov.netColour(1) {
		t.Errorf("ghost links %v, want one from 6,6 in net 1's colour", *got)
	}
	if n := len(povParts(*got, "not connected")); n != 0 {
		t.Errorf("an in-range ghost says not connected (%d)", n)
	}

	a, got = ghostApp(t, "heater", 10, 6)
	a.drawArchitectGhost(screen)
	if len(povParts(*got, "ghost link")) != 0 || len(povParts(*got, "not connected")) != 1 {
		t.Errorf("out of range: %v, want one Not connected", *got)
	}
	if powerGhostLabel != "Not connected" {
		t.Errorf("label %q", powerGhostLabel)
	}

	a, got = ghostApp(t, "conduit", 6, 6)
	a.drawArchitectGhost(screen)
	if len(*got) != 0 {
		t.Errorf("a conduit ghost drew links: %v", *got)
	}
	a, got = ghostApp(t, "torch_lamp", 10, 6)
	a.drawArchitectGhost(screen)
	if len(*got) != 0 {
		t.Errorf("an unpowered building's ghost drew links: %v", *got)
	}
}

// The cooler's ghost is the simulation's judgement: CanPlaceAt's reason
// (outside a wall gap) turns its cell red; the Architect adds no rule.
func TestCoolerGhostUsesCanPlace(t *testing.T) {
	t.Parallel()
	s := &wallGapSim{stubSim: &stubSim{w: 24, h: 24}}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.openArchitect()
	a.archCat = "misc"
	archPick(t, a, "cooler")
	f.moveTo(a, 5, 5)
	f.frame(t, a)
	_, cells := a.previewCells()
	if len(cells) != 1 || cells[0].ok || cells[0].reason != game.ReasonNeedsWall {
		t.Fatalf("cells %+v, want one red cell with %q", cells, game.ReasonNeedsWall)
	}
	f.moveTo(a, 7, 5)
	f.frame(t, a)
	if _, cells = a.previewCells(); len(cells) != 1 || !cells[0].ok {
		t.Fatalf("cells %+v, want the wall gap green", cells)
	}
}

// wallGapSim accepts a cooler only in the gap at (7, 5).
type wallGapSim struct{ *stubSim }

func (s *wallGapSim) CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string) {
	if kind == coolerKind && (x != 7 || y != 5) {
		return false, game.ReasonNeedsWall
	}
	return s.stubSim.CanPlaceAt(kind, stuff, facing, x, y)
}

// Placing a power building sends DesignateBuild like any other.
func TestPowerToolsPlace(t *testing.T) {
	t.Parallel()
	s := &stubSim{}
	a, _ := powerArch(t, s)
	a.archCat = archCatPower
	archPick(t, a, "conduit")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 2, 3, 5, 3
	a.submitArchitect()
	id, _ := kindID(conduitKind)
	n := 0
	for _, c := range s.cmds {
		if c.Kind == command.DesignateBuild && c.Build == id {
			n++
		}
	}
	if n != 4 {
		t.Errorf("%d conduit designations, want a line of 4: %+v", n, s.cmds)
	}
}

// The architect with the Power category open, and the global stack with
// the power legend and nine toggles, keep their regions apart at UI
// scales 1, 1.5 and 2.
func TestPowerLayoutAtScales(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			s := newGlobalSim()
			a, _ := globalColony(t, s, sc.ui, sc.ww, sc.wh)
			a.renderer.togglePowerOverlay()
			a.renderer.roofOverlay = true
			a.openArchitect()
			a.archCat = archCatPower
			archPick(t, a, "battery")
			a.updateOnce(t)
			l := a.hud()
			if l.Global.H != a.globalHeight() {
				t.Fatalf("Global %+v is not its measured stack %v", l.Global, a.globalHeight())
			}
			rows := a.globalRows()
			if rows.Legend.Empty() || rows.Legend.Y < l.Global.Y {
				t.Errorf("legend %+v outside Global %+v", rows.Legend, l.Global)
			}
			for i, r := range rows.Toggles {
				if r.X < l.Global.X || r.Right() > l.Global.Right() || r.Bottom() > l.Global.Bottom() || r.Y < l.Global.Y {
					t.Errorf("toggle %d %+v outside Global %+v", i, r, l.Global)
				}
			}
			for _, b := range archButtonsWith(a, "arch.cat.") {
				if !within(b, l.ArchCats) {
					t.Errorf("%s %+v outside ArchCats %+v", b.id, b, l.ArchCats)
				}
			}
			if cmds := a.archToolCommands(); len(l.GridCells) != len(cmds) || len(cmds) != len(toolsIn(archCatPower)) {
				t.Errorf("%d grid cells for %d power tools", len(l.GridCells), len(cmds))
			}
			regions := append([]rect{l.Tabs, l.ArchCats, l.ArchDesc, l.Global}, l.GridCells...)
			for i := range regions {
				for j := range regions[:i] {
					p, q := regions[i], regions[j]
					if p.Empty() || q.Empty() {
						continue
					}
					if in := p.Intersect(q); in.W > 0 && in.H > 0 {
						t.Errorf("region %d %+v overlaps region %d %+v", i, p, j, q)
					}
				}
			}
			screen := ebiten.NewImage(a.pw, a.ph)
			a.drawGlobalControls(screen, a.sw, a.sh)
			a.drawArchitect(screen)
		})
	}
}
