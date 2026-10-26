package ebiten

import (
	"image/color"
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestStuffMaterial(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"": "wood", "wood": "wood", "stone": "stone", "granite blocks": "stone",
		"marble blocks": "stone", "steel": "steel", "cloth carpet": "wood",
	} {
		if got := stuffMaterial(in); got != want {
			t.Fatalf("stuffMaterial(%q) = %q, want %q", in, got, want)
		}
	}
	if isStoneStuff("granite chunk") || !isStoneStuff("slate blocks") || !isStoneStuff("stone") || isStoneStuff("steel") {
		t.Fatal("isStoneStuff: blocks and legacy stone only")
	}
	if stuffStone("limestone blocks") != "limestone" || stuffStone("stone") != "granite" || stuffStone("wood") != "" {
		t.Fatal("stuffStone")
	}
}

func TestStuffTitles(t *testing.T) {
	t.Parallel()
	cases := []struct{ kind, stuff, want string }{
		{"wall", "granite blocks", "Granite wall"},
		{"door", "steel", "Steel door"},
		{"wall", "stone", "Stone wall"}, // a pre-migration read model
		{"wall", "wood", "Wall"},
		{"floor", "marble blocks", "Marble floor"},
	}
	for _, c := range cases {
		if got := buildingTitle(game.BuildingView{Kind: c.kind, Stuff: c.stuff}); got != c.want {
			t.Fatalf("buildingTitle(%s, %s) = %q, want %q", c.kind, c.stuff, got, c.want)
		}
		if got := frameName(game.FrameView{Kind: c.kind, Stuff: c.stuff}); got != c.want {
			t.Fatalf("frameName(%s, %s) = %q, want %q", c.kind, c.stuff, got, c.want)
		}
	}
}

func TestBuildSpriteStuffMaterials(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, stuff string
		mask        int
		ew, open    bool
		want        string
	}{
		{"wall", "steel", 5, false, false, "structure/wall_steel_05"},
		{"wall", "slate blocks", 15, false, false, "structure/wall_stone_15"},
		{"wall", "stone", 3, false, false, "structure/wall_stone_03"},
		{"wall", "wood", 3, false, false, "structure/wall_wood_03"},
		{"door", "steel", 10, true, true, "structure/door_steel_ew_open"},
		{"door", "steel", 5, false, false, "structure/door_steel_ns_closed"},
		{"door", "wood", 5, false, false, "structure/door_wood_ns_closed"},
		{"door", "wood", 0, doorAxisEW("south"), false, "structure/door_wood_ew_closed"}, // axis from the stored facing, not the links mask
		{"door", "steel", 15, doorAxisEW("west"), true, "structure/door_steel_ns_open"},
		{"floor", "granite blocks", 0, false, false, "structure/floor_stone"},
		{"fence", "marble blocks", 15, false, false, "structure/fence_stone_15"},
	}
	for _, c := range cases {
		if got := buildSpriteStuff(c.kind, c.stuff, "", c.mask, c.ew, c.open); got != c.want {
			t.Fatalf("buildSpriteStuff(%s, %s) = %s, want %s", c.kind, c.stuff, got, c.want)
		}
	}
	if wallPrefix("granite blocks") != "structure/wall_stone" || wallPrefix("steel") != "structure/wall_steel" || wallPrefix("wood") != "structure/wall_wood" {
		t.Fatal("wallPrefix")
	}
}

func TestStoneItemSprites(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"granite chunk": "item/stone_chunk", "sandstone chunk": "item/stone_chunk",
		"marble blocks": "item/stone_blocks", "stone": "item/stone_blocks",
		"steel": "item/steel_bars", "silver": "item/silver", "gold": "item/gold",
	} {
		if got := itemSprite(in); got != want {
			t.Fatalf("itemSprite(%q) = %q, want %q", in, got, want)
		}
	}
	for _, k := range []string{"granite chunk", "slate blocks", "steel", "gold"} {
		if c := itemCategory(game.ItemView{Kind: k}); c != "material" {
			t.Fatalf("%s category = %q", k, c)
		}
	}
}

func TestStuffLabels(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"wood": "Wood", "granite blocks": "Granite blocks", "steel": "Steel", "cloth carpet": "Cloth",
	} {
		if got := stuffLabel(in); got != want {
			t.Fatalf("stuffLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultStoneStuff(t *testing.T) {
	t.Parallel()
	if got := defaultStoneStuff(game.WorldView{}); got != "granite blocks" {
		t.Fatalf("no stock: %q", got)
	}
	w := game.WorldView{Items: []game.ItemView{
		{Kind: "sandstone blocks", Qty: 9, Forbidden: true},
		{Kind: "marble blocks", Qty: 4},
		{Kind: "slate blocks", Qty: 2},
	}}
	// first block kind (enum order) with free stock: slate before marble
	if got := defaultStoneStuff(w); got != "slate blocks" {
		t.Fatalf("with stock: %q", got)
	}
}

func testStoneArt(keys ...string) *Renderer {
	c := &artCatalog{sprites: map[string]*artSprite{}, tints: map[string]map[string]color.NRGBA{
		"stone_tint": {"granite": {0x69, 0x5f, 0x61, 255}, "marble": {0x84, 0x87, 0x84, 255}},
	}}
	for _, k := range keys {
		c.sprites[k] = &artSprite{}
	}
	return &Renderer{art: c}
}

func TestStoneTints(t *testing.T) {
	t.Parallel()
	r := testStoneArt()
	want := colorScale{0x69 / 255.0, 0x5f / 255.0, 0x61 / 255.0, 1}
	if got := tintCS(r.stuffTint("granite blocks")); got != want {
		t.Fatalf("granite cs = %v, want %v", got, want)
	}
	if r.stuffTint("stone") != r.stuffTint("granite blocks") {
		t.Fatal("legacy stone draws as granite")
	}
	if r.stuffTint("wood") != (color.NRGBA{}) || r.stuffTint("steel") != (color.NRGBA{}) {
		t.Fatal("wood and steel are not stone-tinted")
	}
	if tintCS(color.NRGBA{}) != noCS {
		t.Fatal("no tint is the identity scale")
	}
	marble := uint8(slices.Index(stoneNames, "marble"))
	if r.stoneTint(marble) != (color.NRGBA{0x84, 0x87, 0x84, 255}) {
		t.Fatal("cell stone tint")
	}
	if r.stoneTint(0) != r.stuffTint("granite blocks") {
		t.Fatal("rock with no stone data draws as granite")
	}
	if got := mulCS(want, blueprintCS); got[3] != blueprintCS[3] || got[0] != want[0]*blueprintCS[0] {
		t.Fatalf("mulCS = %v", got)
	}
}

func TestLinkKeyFallback(t *testing.T) { // not parallel: AllocsPerRun
	r := testStoneArt("rock/rock_wall_15", "rock/rock_wall_15_cf", "structure/wall_stone_06")
	if got := r.linkKey("rock/rock_wall", 15, 15); got != "rock/rock_wall_15_cf" {
		t.Fatalf("baked corner variant: %s", got)
	}
	if got := r.linkKey("rock/rock_wall", 15, cornerNE); got != "rock/rock_wall_15" {
		t.Fatalf("missing corner variant falls back to the mask: %s", got)
	}
	if got := r.linkKey("rock/ore_steel", 15, 15); got != "rock/rock_wall_15_cf" {
		t.Fatalf("missing ore art falls back to rock: %s", got)
	}
	if got := r.linkKey("rock/rock_smooth", 15, 0); got != "rock/rock_wall_15" {
		t.Fatalf("missing smooth art falls back to rock: %s", got)
	}
	if got := r.linkKey("structure/wall_steel", 6, cornerSE); got != "structure/wall_stone_06" {
		t.Fatalf("missing steel walls fall back to stone: %s", got)
	}
	if got := r.linkKey("structure/wall_wood", 6, 0); got != "" {
		t.Fatalf("nothing to draw: %q", got)
	}
	if n := testing.AllocsPerRun(50, func() { r.linkKey("rock/rock_wall", 15, 15) }); n != 0 {
		t.Fatalf("resolved keys are cached: %v allocs", n)
	}
	if rockPrefix("rock", "") != "rock/rock_wall" || rockPrefix("rock", "steel") != "rock/ore_steel" ||
		rockPrefix("rock_smooth", "") != "rock/rock_smooth" {
		t.Fatal("rockPrefix")
	}
}

func TestArchStockLineShowsSelectedStuff(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Stone: 30, Items: []game.ItemView{
		{Kind: "granite blocks", Qty: 25}, {Kind: "granite blocks", Qty: 5, Forbidden: true}, {Kind: "steel", Qty: 40},
	}}
	for stuff, want := range map[string]string{
		"":               "Wood 12   Stone 30",
		"wood":           "Wood 12   Stone 30",
		"granite blocks": "Wood 12   Granite blocks 25",
		"steel":          "Wood 12   Steel 40",
		"marble blocks":  "Wood 12   Marble blocks 0",
	} {
		if got := archStockLine(w, 12, stuff); got != want {
			t.Fatalf("archStockLine(%q) = %q, want %q", stuff, got, want)
		}
	}
}

func TestSoilLinesNameStoneAndOre(t *testing.T) {
	t.Parallel()
	code := func(names []string, n string) uint8 { return uint8(slices.Index(names, n)) }
	tn := game.TerrainNames()
	w := game.WorldView{TerrainW: 3,
		Terrain:   []uint8{code(tn, "rock"), code(tn, "rock"), code(tn, "stone_smooth")},
		StoneType: []uint8{code(stoneNames, "granite"), code(stoneNames, "granite"), code(stoneNames, "slate")},
		Ore:       []uint8{0, code(oreNames, "steel"), 0}}
	l := soilLines(w, 0, 0)
	if l[0] != "Granite rock wall" || !slices.Contains(l, "Impassable, mine to clear") || !slices.Contains(l, "Smoothable") {
		t.Fatalf("rock lines = %q", l)
	}
	if l = soilLines(w, 1, 0); l[0] != "Compacted steel (in granite)" || slices.Contains(l, "Smoothable") {
		t.Fatalf("ore lines = %q", l)
	}
	if l = soilLines(w, 2, 0); l[0] != "Smooth slate floor" || slices.Contains(l, "Smoothable") {
		t.Fatalf("smooth floor lines = %q", l)
	}
}

func TestSmoothToolSendsOneRect(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.orders")
	click(t, a, "arch.tool.smooth")
	tool, _ := toolByID("smooth")
	if tool.icon != "order_smooth" || tool.mode != placeArea {
		t.Fatalf("smooth tool = %+v", tool)
	}
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 3, 2, 5
	a.submitArchitect()
	c := lastCmd(t, s)
	if c.Kind != command.DesignateSmooth || min(c.X, c.X2) != 2 || max(c.X, c.X2) != 6 || min(c.Y, c.Y2) != 3 || max(c.Y, c.Y2) != 5 {
		t.Fatalf("cmd = %+v", c)
	}
}

func TestStonecutterCard(t *testing.T) {
	t.Parallel()
	tool, ok := toolByID("stonecutter")
	if !ok || tool.category != "production" || tool.build != "stonecutter table" {
		t.Fatalf("stonecutter tool = %+v", tool)
	}
	if _, ok := buildable("stonecutter table"); !ok {
		t.Fatal("the sim offers no stonecutter table")
	}
	if got := buildSprite("stonecutter table", "east", 0, false, false); got != "furniture/stonecutter_table_east" {
		t.Fatal(got)
	}
}

func TestSmoothActionAnimates(t *testing.T) {
	t.Parallel()
	act, d, ok := actionOf(game.PawnView{Action: game.ActionSmooth})
	if !ok || act != game.ActionSmooth || d.tool == "" || actionSounds[game.ActionSmooth] == "" {
		t.Fatalf("smooth action = %q %+v %v", act, d, ok)
	}
}

func TestTerrainEdgePriorityMatchesArt(t *testing.T) {
	t.Parallel()
	// the edge art's order: soil < soil_rich < gravel < stone < stone_smooth < sand < marsh < ice
	order := []string{"soil", "rich_soil", "gravel", "stone", "stone_smooth", "sand", "marsh", "ice"}
	for i, n := range order {
		if terrainPriority[n] != i || terrainByPri[i] != n {
			t.Fatalf("%s: priority %d, want %d", n, terrainPriority[n], i)
		}
	}
	if terrainPriority["rock"] != terrainPriority["stone"] || terrainPriority["rock_smooth"] != terrainPriority["stone_smooth"] {
		t.Fatal("rock draws its ground as the matching stone")
	}
	if terrainSprite["rock_smooth"] != "stone_smooth" || terrainSprite["stone_smooth"] != "stone_smooth" {
		t.Fatal("smooth ground sprite")
	}
}

func TestStoneItemTintAndFallback(t *testing.T) {
	t.Parallel()
	r := testStoneArt("item/stone_blocks", "item/steel_bars")
	key, cs := r.stoneItem(itemSprite("marble chunk"), "marble chunk", noCS)
	if key != "item/stone_blocks" || cs != tintCS(r.stuffTint("marble blocks")) || cs == noCS {
		t.Fatalf("chunk without art: %s %v", key, cs)
	}
	if key, cs = r.stoneItem(itemSprite("gold"), "gold", driedCS); key != "item/steel_bars" || cs != driedCS {
		t.Fatalf("gold without art: %s %v", key, cs)
	}
	if key, cs = r.stoneItem(itemSprite("wood"), "wood", noCS); key != "item/wood_pile" || cs != noCS {
		t.Fatalf("wood: %s %v", key, cs)
	}
	a := r.carryPose(game.PawnView{ID: 1, Carrying: "granite chunk"}, moveState{})
	if a.held.key != "item/stone_blocks" || a.held.cs != tintCS(r.stuffTint("granite blocks")) {
		t.Fatalf("carried chunk = %+v", a.held)
	}
}
