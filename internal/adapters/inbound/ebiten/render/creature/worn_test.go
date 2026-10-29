package creature

import (
	"image/color"
	"os"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/pawn/look"
)

// rootLayers is a dressed root figure facing south: body, base clothes,
// a parka, beard, hair and a tuque (portrait.go's figureLayers order).
func rootLayers() []Layer {
	return []Layer{{Key: "pawn/body_south"}, {Key: "pawn/pants_basic_south"}, {Key: "pawn/shirt_basic_south"},
		{Key: "pawn/vest_utility_south"}, {Key: "pawn/parka_leather_south"}, {Key: "pawn/beard_full_south"},
		{Key: "pawn/hair_short_south"}, {Key: "pawn/tuque_knit_south"}}
}

func keysOf(ls []Layer) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Key
	}
	return out
}

// TestWornLayersDrawOrder (DEF-2): armor goes shirt → flak pants → vest
// → [parka] → jacket → plate/marine → beard → hair → helmet, whatever
// order PawnView.Worn lists it in.
func TestWornLayersDrawOrder(t *testing.T) {
	t.Parallel()
	var w WornKeys
	p := game.PawnView{Worn: []craft.WornView{
		{Kind: "simple helmet", Stuff: "gold"}, {Kind: "plate armor", Stuff: "steel"}, {Kind: "parka"},
		{Kind: "flak jacket"}, {Kind: "flak vest"}, {Kind: "flak pants"},
	}}
	pre := []Layer{{Key: "other/figure"}} // a figure already on the stack stays put
	ls := w.Dress(append(pre, rootLayers()...), 1, nil, nil, &p, "south")
	want := []string{"other/figure", "pawn/body_south", "pawn/pants_basic_south", "pawn/shirt_basic_south", "pawn/vest_utility_south",
		"pawn/flak_pants_south", "pawn/flak_vest_south", "pawn/parka_leather_south", "pawn/flak_jacket_south", "pawn/plate_armor_south",
		"pawn/beard_full_south", "pawn/hair_short_south", "pawn/tuque_knit_south", "pawn/simple_helmet_south"}
	if got := keysOf(ls); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("layers\n got %v\nwant %v", got, want)
	}
	if ls[len(ls)-1].Tint == (color.NRGBA{}) {
		t.Error("a gold helmet takes the gold tint")
	}
	if ls[9].Tint != (color.NRGBA{}) {
		t.Error("steel plate draws the art's own steel")
	}
	if got := w.Dress(rootLayers(), 0, nil, nil, &game.PawnView{}, "south"); len(got) != len(rootLayers()) {
		t.Errorf("nothing worn added layers: %v", keysOf(got))
	}
}

// TestWornLayersByBuildAndFacing: a body piece takes the pawn's build
// (male average unsuffixed), a helmet only the facing; a bare figure
// (no clothes, no head layers) still orders them.
func TestWornLayersByBuildAndFacing(t *testing.T) {
	t.Parallel()
	var w WornKeys
	p := game.PawnView{Look: look.Look{Sex: look.Female, Build: look.Heavy},
		Worn: []craft.WornView{{Kind: "marine helmet"}, {Kind: "marine armor"}, {Kind: "flak pants"}}}
	ls := w.Dress([]Layer{{Key: "pawn/body_f_heavy_north"}}, 0, nil, nil, &p, "north")
	want := "pawn/body_f_heavy_north,pawn/flak_pants_f_heavy_north,pawn/marine_armor_f_heavy_north,pawn/marine_helmet_north"
	if got := strings.Join(keysOf(ls), ","); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if b := buildIndex(look.Look{}); wornBuildKeys[b] != (look.Look{}).BuildKey() || b != avgBuild {
		t.Errorf("male average build index %d", b)
	}
	for _, l := range []look.Look{{Build: look.Thin}, {Build: look.Heavy}, {Sex: look.Female}, {Sex: look.Female, Build: look.Thin}} {
		if wornBuildKeys[buildIndex(l)] != l.BuildKey() {
			t.Errorf("%+v -> %s, want %s", l, wornBuildKeys[buildIndex(l)], l.BuildKey())
		}
	}
}

// TestWornKeysHaveArt: every key the table builds is a sprite in the
// pawn atlas, so no worn piece draws as nothing.
func TestWornKeysHaveArt(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../../../../assets/metadata/atlas/pawn.yaml")
	if err != nil {
		t.Skip("no pawn atlas")
	}
	atlas := string(raw)
	var w WornKeys
	w.build()
	check := func(k string) {
		if !strings.Contains(atlas, `id: "core:`+k+`"`) {
			t.Errorf("%s is not in the pawn atlas", k)
		}
	}
	for pc := range w.body {
		for b := range w.body[pc] {
			for _, k := range w.body[pc][b] {
				check(k)
			}
		}
	}
	for h := range w.helm {
		for _, k := range w.helm[h] {
			check(k)
		}
	}
}

// TestWornDressDoesNotAllocate: the per-frame dressing reuses the layer
// stack and the built table.
func TestWornDressDoesNotAllocate(t *testing.T) {
	var w WornKeys
	p := game.PawnView{Worn: []craft.WornView{{Kind: "flak vest"}, {Kind: "plate armor", Stuff: "silver"}, {Kind: "advanced helmet"}}}
	stack := make([]Layer, 0, 32)
	w.Dress(append(stack, rootLayers()...), 0, nil, nil, &p, "east") // build the table
	root := rootLayers()
	if n := testing.AllocsPerRun(100, func() {
		ls := append(stack[:0], root...)
		ls = w.Dress(ls, 0, nil, nil, &p, "east")
		stack = ls[:0]
	}); n != 0 {
		t.Fatalf("Dress allocates %.1f times a call", n)
	}
}

// TestWeaponBlastsLeaveTheBangToTheCueBus (DEF-2): a weapon's blast
// throws its particles but raises no creature bang (cue.Bus.ListenBursts
// voices it); an animal's burst still bangs.
func TestWeaponBlastsLeaveTheBangToTheCueBus(t *testing.T) {
	t.Parallel()
	c, v, h := testSet()
	c.EmitWorldBursts(v, game.WorldView{Bursts: []game.BurstView{{X: 3, Y: 3, Tick: 5, Blast: "bomb", Radius: 19}}})
	if len(*c.Parts) == 0 || len(h.cues) != 0 {
		t.Fatalf("blast: %d particles, cues %v", len(*c.Parts), h.cues)
	}
	c.EmitWorldBursts(v, game.WorldView{Bursts: []game.BurstView{{X: 4, Y: 4, Tick: 6, Species: "emberbuck"}}})
	if len(h.cues) != 1 {
		t.Fatalf("an animal's burst bangs once: %v", h.cues)
	}
}
