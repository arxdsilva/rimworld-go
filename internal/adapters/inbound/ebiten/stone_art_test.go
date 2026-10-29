package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

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
		if got := inspector.BuildingTitle(game.BuildingView{Kind: c.kind, Stuff: c.stuff}); got != c.want {
			t.Fatalf("buildingTitle(%s, %s) = %q, want %q", c.kind, c.stuff, got, c.want)
		}
		if got := inspector.FrameName(game.FrameView{Kind: c.kind, Stuff: c.stuff}); got != c.want {
			t.Fatalf("frameName(%s, %s) = %q, want %q", c.kind, c.stuff, got, c.want)
		}
	}
}

func TestStoneItemSprites(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"granite chunk": "item/stone_chunk", "sandstone chunk": "item/stone_chunk",
		"marble blocks": "item/stone_blocks", "stone": "item/stone_blocks",
		"steel": "item/steel_bars", "silver": "item/silver", "gold": "item/gold",
	} {
		if got := world.ItemSprite(in); got != want {
			t.Fatalf("itemSprite(%q) = %q, want %q", in, got, want)
		}
	}
	for _, k := range []string{"granite chunk", "slate blocks", "steel", "gold"} {
		if c := inspector.ItemCategory(game.ItemView{Kind: k}); c != "material" {
			t.Fatalf("%s category = %q", k, c)
		}
	}
}

func TestStuffLabels(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"wood": "Wood", "granite blocks": "Granite blocks", "steel": "Steel", "cloth carpet": "Cloth",
	} {
		if got := architect.StuffLabel(in); got != want {
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
	c := &artpkg.Catalog{Sprites: map[string]*artpkg.Sprite{}, Tints: map[string]map[string]color.NRGBA{
		"stone_tint": {"granite": {0x69, 0x5f, 0x61, 255}, "marble": {0x84, 0x87, 0x84, 255}},
	}}
	for _, k := range keys {
		c.Sprites[k] = &artpkg.Sprite{}
	}
	return &Renderer{art: c}
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
		if got := architect.StockLine(w, 12, stuff); got != want {
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
		StoneType: []uint8{code(sprite.StoneNames, "granite"), code(sprite.StoneNames, "granite"), code(sprite.StoneNames, "slate")},
		Ore:       []uint8{0, code(sprite.OreNames, "steel"), 0}}
	l := hud.SoilLines(w, 0, 0)
	if l[0] != "Granite rock wall" || !slices.Contains(l, "Impassable, mine to clear") || !slices.Contains(l, "Smoothable") {
		t.Fatalf("rock lines = %q", l)
	}
	if l = hud.SoilLines(w, 1, 0); l[0] != "Compacted steel (in granite)" || slices.Contains(l, "Smoothable") {
		t.Fatalf("ore lines = %q", l)
	}
	if l = hud.SoilLines(w, 2, 0); l[0] != "Smooth slate floor" || slices.Contains(l, "Smoothable") {
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
	if tool.Icon != "order_smooth" || tool.Mode != architect.PlaceArea {
		t.Fatalf("smooth tool = %+v", tool)
	}
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 3, 2, 5
	a.arch.Submit()
	c := lastCmd(t, s)
	if c.Kind != command.DesignateSmooth || min(c.X, c.X2) != 2 || max(c.X, c.X2) != 6 || min(c.Y, c.Y2) != 3 || max(c.Y, c.Y2) != 5 {
		t.Fatalf("cmd = %+v", c)
	}
}

func TestStonecutterCard(t *testing.T) {
	t.Parallel()
	tool, ok := toolByID("stonecutter")
	if !ok || tool.Category != "production" || tool.Build != "stonecutter table" {
		t.Fatalf("stonecutter tool = %+v", tool)
	}
	if _, ok := architect.Buildable("stonecutter table"); !ok {
		t.Fatal("the sim offers no stonecutter table")
	}
	if got := world.BuildSprite("stonecutter table", "east", 0, false, false); got != "furniture/stonecutter_table_east" {
		t.Fatal(got)
	}
}

func TestSmoothActionAnimates(t *testing.T) {
	t.Parallel()
	act, d, ok := creature.ActionOf(game.PawnView{Action: game.ActionSmooth})
	if !ok || act != game.ActionSmooth || d.Motion != creature.MotionSwing || len(d.Parts) == 0 || cue.ActionSounds[game.ActionSmooth] == "" {
		t.Fatalf("smooth action = %q %+v %v", act, d, ok)
	}
}

func TestStoneItemTintAndFallback(t *testing.T) {
	t.Parallel()
	r := testStoneArt("item/stone_blocks", "item/steel_bars")
	key, cs := r.pipe().StoneItem(world.ItemSprite("marble chunk"), "marble chunk", sprite.NoCS)
	if key != "item/stone_blocks" || cs != sprite.TintCS(r.pipe().Paint.StuffTint("marble blocks")) || cs == sprite.NoCS {
		t.Fatalf("chunk without art: %s %v", key, cs)
	}
	if key, cs = r.pipe().StoneItem(world.ItemSprite("gold"), "gold", sprite.DriedCS); key != "item/steel_bars" || cs != sprite.DriedCS {
		t.Fatalf("gold without art: %s %v", key, cs)
	}
	if key, cs = r.pipe().StoneItem(world.ItemSprite("wood"), "wood", sprite.NoCS); key != "item/wood_pile" || cs != sprite.NoCS {
		t.Fatalf("wood: %s %v", key, cs)
	}
	a := r.carryPose(game.PawnView{ID: 1, Carrying: "granite chunk"}, motionpkg.MoveState{})
	if a.Held.Key != "item/stone_blocks" || a.Held.Cs != sprite.TintCS(r.pipe().Paint.StuffTint("granite blocks")) {
		t.Fatalf("carried chunk = %+v", a.Held)
	}
}
