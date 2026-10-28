package sprite

import (
	"image/color"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// testPainter is a painter over a catalog holding keys (no pixels) and
// the granite and marble stone tints.
func testPainter(keys ...string) *Painter {
	c := &art.Catalog{Sprites: map[string]*art.Sprite{}, Tints: map[string]map[string]color.NRGBA{
		"stone_tint": {"granite": {0x69, 0x5f, 0x61, 255}, "marble": {0x84, 0x87, 0x84, 255}},
	}}
	for _, k := range keys {
		c.Sprites[k] = &art.Sprite{}
	}
	return NewPainter(c, nil)
}

func TestStuffMaterial(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"": "wood", "wood": "wood", "stone": "stone", "granite blocks": "stone",
		"marble blocks": "stone", "steel": "steel", "cloth carpet": "wood",
	} {
		if got := StuffMaterial(in); got != want {
			t.Fatalf("StuffMaterial(%q) = %q, want %q", in, got, want)
		}
	}
	if IsStoneStuff("granite chunk") || !IsStoneStuff("slate blocks") || !IsStoneStuff("stone") || IsStoneStuff("steel") {
		t.Fatal("IsStoneStuff: blocks and legacy stone only")
	}
	if StuffStone("limestone blocks") != "limestone" || StuffStone("stone") != "granite" || StuffStone("wood") != "" {
		t.Fatal("StuffStone")
	}
}

func TestStoneTints(t *testing.T) {
	t.Parallel()
	r := testPainter()
	want := ColorScale{0x69 / 255.0, 0x5f / 255.0, 0x61 / 255.0, 1}
	if got := TintCS(r.StuffTint("granite blocks")); got != want {
		t.Fatalf("granite cs = %v, want %v", got, want)
	}
	if r.StuffTint("stone") != r.StuffTint("granite blocks") {
		t.Fatal("legacy stone draws as granite")
	}
	if r.StuffTint("wood") != (color.NRGBA{}) || r.StuffTint("steel") != (color.NRGBA{}) {
		t.Fatal("wood and steel are not stone-tinted")
	}
	if TintCS(color.NRGBA{}) != NoCS {
		t.Fatal("no tint is the identity scale")
	}
	marble := uint8(slices.Index(StoneNames, "marble"))
	if r.StoneTint(marble) != (color.NRGBA{0x84, 0x87, 0x84, 255}) {
		t.Fatal("cell stone tint")
	}
	if r.StoneTint(0) != r.StuffTint("granite blocks") {
		t.Fatal("rock with no stone data draws as granite")
	}
	if got := MulCS(want, BlueprintCS); got[3] != BlueprintCS[3] || got[0] != want[0]*BlueprintCS[0] {
		t.Fatalf("MulCS = %v", got)
	}
}

func TestLinkKeyFallback(t *testing.T) { // not parallel: AllocsPerRun
	r := testPainter("rock/rock_wall_15", "rock/rock_wall_15_cf", "structure/wall_stone_06")
	if got := r.LinkKey("rock/rock_wall", 15, 15); got != "rock/rock_wall_15_cf" {
		t.Fatalf("baked corner variant: %s", got)
	}
	if got := r.LinkKey("rock/rock_wall", 15, CornerNE); got != "rock/rock_wall_15" {
		t.Fatalf("missing corner variant falls back to the mask: %s", got)
	}
	if got := r.LinkKey("rock/ore_steel", 15, 15); got != "rock/rock_wall_15_cf" {
		t.Fatalf("missing ore art falls back to rock: %s", got)
	}
	if got := r.LinkKey("rock/rock_smooth", 15, 0); got != "rock/rock_wall_15" {
		t.Fatalf("missing smooth art falls back to rock: %s", got)
	}
	if got := r.LinkKey("structure/wall_steel", 6, CornerSE); got != "structure/wall_stone_06" {
		t.Fatalf("missing steel walls fall back to stone: %s", got)
	}
	if got := r.LinkKey("structure/wall_wood", 6, 0); got != "" {
		t.Fatalf("nothing to draw: %q", got)
	}
	if n := testing.AllocsPerRun(50, func() { r.LinkKey("rock/rock_wall", 15, 15) }); n != 0 {
		t.Fatalf("resolved keys are cached: %v allocs", n)
	}
	if RockPrefix("rock", "") != "rock/rock_wall" || RockPrefix("rock", "steel") != "rock/ore_steel" ||
		RockPrefix("rock_smooth", "") != "rock/rock_smooth" {
		t.Fatal("RockPrefix")
	}
}
