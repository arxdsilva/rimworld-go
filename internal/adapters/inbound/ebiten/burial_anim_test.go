package ebiten

import (
	"math"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func burialRenderer(pawns []game.PawnView) *Renderer {
	return combatRenderer(100, pawns, game.WorldView{})
}

func TestHumanCorpseDrawsDeceasedLying(t *testing.T) {
	t.Parallel()
	dead := game.PawnView{ID: 4, X: 3, Y: 3, Status: "dead", Embodied: true}
	r := burialRenderer([]game.PawnView{{ID: 1}, dead})
	it := game.ItemView{ID: 9, Kind: "human corpse", X: 3, Y: 3, Of: 4, OfName: "Ada"}

	p, ok := r.deceasedOf(it)
	if !ok || p.ID != 4 {
		t.Fatalf("looks the deceased up by Of: %+v %v", p, ok)
	}
	if _, ok := r.deceasedOf(game.ItemView{Of: 77}); ok {
		t.Fatal("unknown deceased")
	}
	plan := corpsePlanFor(r.art, it)
	side := math.Copysign(1, plan.ps.Rot)
	want := lyingPose(r.art, side)
	want.Cs = deadCS
	if plan.ps != want || math.Abs(math.Abs(plan.ps.Rot)-math.Pi/2) > 1e-9 {
		t.Fatalf("the standing figure laid flat on its side: %+v", plan.ps)
	}
	if plan.ps.Cs != deadCS {
		t.Fatalf("dead tint: %v", plan.ps.Cs)
	}
	it.Rotten = true
	if corpsePlanFor(r.art, it).ps.Cs != rottenCS {
		t.Fatal("rotten tint")
	}
	// the side it fell on is stable per body
	if corpsePlanFor(r.art, it).ps.Rot != plan.ps.Rot {
		t.Fatal("stable lying side")
	}
	screen := ebiten.NewImage(512, 512)
	var keys []string
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
	r.drawCorpseItem(screen, it, 64, 512, 512)
	r.lit.OnDraw = nil
	if len(keys) == 0 || keys[0] != "pawn/body_south" {
		t.Fatalf("the corpse is the deceased's south figure: %v", keys)
	}
	r.drawCorpseItem(screen, game.ItemView{ID: 2, Kind: "human corpse", Of: 99}, 64, 512, 512) // body unknown: no panic
	r.drawCorpseItem(screen, game.ItemView{ID: 3, Kind: "bones", X: 2, Y: 2}, 64, 512, 512)
}

func TestCorpseFliesWhenRotting(t *testing.T) {
	t.Parallel()
	fresh := game.ItemView{ID: 9, Kind: "human corpse", X: 3, Y: 3}
	if len(corpseFlies(fresh, 10)) != 0 {
		t.Fatal("a fresh body draws no flies")
	}
	bones := game.ItemView{ID: 9, Kind: "bones", Rotten: true}
	if len(corpseFlies(bones, 10)) != 0 {
		t.Fatal("bones draw no flies")
	}
	rot := fresh
	rot.Rotten = true
	a := corpseFlies(rot, 10)
	if len(a) < 2 || len(a) > 4 {
		t.Fatalf("a few flies: %d", len(a))
	}
	b := corpseFlies(rot, 10)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("deterministic in item ID and frame")
		}
		if math.Hypot(a[i][0]-3.5, a[i][1]-3.5) > 0.7 {
			t.Fatalf("flies circle the body: %v", a[i])
		}
	}
	if c := corpseFlies(rot, 40); c[0] == a[0] {
		t.Fatal("flies move")
	}
	other := rot
	other.ID = 10
	if corpseFlies(other, 10)[0] == a[0] {
		t.Fatal("seeded by the item ID")
	}
}

func TestBonesSprite(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for x := 0; x < 20; x++ {
		k := bonesSprite(x, 3)
		if k != bonesSprite(x, 3) {
			t.Fatal("stable per tile")
		}
		if k != "item/bones_00" && k != "item/bones_01" {
			t.Fatalf("variant key %s", k)
		}
		if want := artKey("item", "bones", int(tileHash(x, 3)%2)); k != want {
			t.Fatalf("variant by tileHash: %s vs %s", k, want)
		}
		seen[k] = true
	}
	if len(seen) != 2 {
		t.Fatal("both variants appear")
	}
	if itemSpriteAt("bones", 4, 4) != bonesSprite(4, 4) {
		t.Fatal("itemSpriteAt maps bones")
	}
	if got := itemSpriteAt("dustbuck corpse", 4, 4); !strings.HasPrefix(got, "item/corpse_dustbuck_v") {
		t.Fatalf("animal corpse variants: %s", got)
	}
	if itemSpriteAt("wood", 1, 1) != itemSprite("wood") {
		t.Fatal("other items unchanged")
	}
}

func TestGraveDugAndFilledSprites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: "grave"}, "furniture/grave_dug_south"},
		{game.BuildingView{Kind: "grave", Facing: "south"}, "furniture/grave_dug_south"},
		{game.BuildingView{Kind: "grave", Facing: "east"}, "furniture/grave_dug_east"},
		{game.BuildingView{Kind: "grave", Facing: "west"}, "furniture/grave_dug_east"},
		{game.BuildingView{Kind: "grave", Occupant: "Ada"}, "furniture/grave_filled_south"},
		{game.BuildingView{Kind: "grave", Occupant: "Ada", Facing: "east"}, "furniture/grave_filled_east"},
	}
	for _, c := range cases {
		if got := graveSprite(c.b); got != c.want {
			t.Fatalf("%+v: %s want %s", c.b, got, c.want)
		}
	}
	r := burialRenderer(nil)
	r.drawGrave(ebiten.NewImage(256, 256), game.BuildingView{Kind: "grave", X: 1, Y: 1, Occupant: "Ada"}, 64, 256, 256)
}

func TestHeadstoneVariant(t *testing.T) {
	t.Parallel()
	if headstoneSprite(game.BuildingView{Kind: "grave"}) != "" {
		t.Fatal("no headstone on an empty grave")
	}
	for i, want := range []string{"furniture/headstone_00", "furniture/headstone_01", "furniture/headstone_02", "furniture/headstone_03"} {
		if got := headstoneSprite(game.BuildingView{Occupant: "Ada", Headstone: i}); got != want {
			t.Fatalf("style %d: %s", i, got)
		}
	}
	if headstoneSprite(game.BuildingView{Occupant: "A", Headstone: 9}) != "furniture/headstone_03" ||
		headstoneSprite(game.BuildingView{Occupant: "A", Headstone: -1}) != "furniture/headstone_00" {
		t.Fatal("style clamps to 0..3")
	}
	// the head end: top of a south grave, left end of an east grave
	for _, f := range []string{"", "south", "east"} {
		b := game.BuildingView{X: 4, Y: 7, Facing: f}
		if x, y := headstoneTile(b); x != 4 || y != 7 {
			t.Fatalf("%q: head end at %v,%v", f, x, y)
		}
	}
	if x, y := headstoneTile(game.BuildingView{X: 4, Y: 7, W: 2, H: 1, Facing: "west"}); x != 5 || y != 7 {
		t.Fatalf("west grave: head at the east end, got %v,%v", x, y)
	}
	if x, y := headstoneTile(game.BuildingView{X: 4, Y: 7, W: 1, H: 2, Facing: "north"}); x != 4 || y != 8 {
		t.Fatalf("north grave: head at the south end, got %v,%v", x, y)
	}
}

func TestPyreBurningDrawsFireAndSmoke(t *testing.T) {
	t.Parallel()
	if pyreSprite(game.BuildingView{Kind: "funeral pyre"}) != "furniture/pyre" {
		t.Fatal("cold pyre")
	}
	if pyreSprite(game.BuildingView{Kind: "funeral pyre", Burning: true}) != "furniture/pyre_burning" {
		t.Fatal("burning pyre sprite")
	}
	cold := burialRenderer(nil)
	for f := uint64(0); f < 200; f++ {
		cold.present = f
		cold.emitPyre(game.BuildingView{ID: 5, X: 2, Y: 2, W: 2, H: 1})
	}
	if len(cold.parts) != 0 {
		t.Fatal("a cold pyre throws nothing")
	}
	hot := burialRenderer(nil)
	b := game.BuildingView{ID: 5, X: 2, Y: 2, W: 2, H: 1, Burning: true}
	for f := uint64(0); f < 200; f++ {
		hot.present = f
		hot.emitPyre(b)
	}
	kinds := map[partKind]int{}
	for _, p := range hot.parts {
		kinds[p.Kind]++
	}
	if kinds[partSmoke] == 0 || kinds[partEmber] == 0 {
		t.Fatalf("smoke and embers: %v", kinds)
	}
	if pyreFlameScale <= 1 {
		t.Fatal("pyre flames are bigger than a campfire's")
	}
	screen := ebiten.NewImage(256, 256)
	hot.drawPyre(screen, b, 64, 256, 256)
	hot.drawPyre(screen, game.BuildingView{ID: 5, X: 2, Y: 2, W: 2, H: 1}, 64, 256, 256)
}

func TestArchitectHasGraveAndPyre(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"grave": "Dig a grave: colonists bury the dead here.",
		"pyre":  "Funeral pyre: burn the dead (2 wood per body).",
	}
	for id, desc := range want {
		tool, ok := toolByID(id)
		if !ok {
			t.Fatalf("no %s tool", id)
		}
		if tool.category != "furniture" || tool.desc != desc {
			t.Fatalf("%s: %+v", id, tool)
		}
		if _, ok := buildable(tool.build); !ok {
			t.Fatalf("%s builds unknown kind %q", id, tool.build)
		}
	}
	if buildSprite("grave", "east", 0, false, false) != "furniture/grave_dug_east" ||
		buildSprite("funeral pyre", "", 0, false, false) != "furniture/pyre" {
		t.Fatal("ghost sprites")
	}
}

func TestGraveInspectorShowsEpitaph(t *testing.T) {
	t.Parallel()
	got := graveLines(game.BuildingView{Kind: "grave", Occupant: "Ada", Epitaph: "Ada - died of infection, day 12"})
	if len(got) != 2 || got[0] != "Here lies Ada" || got[1] != "Ada - died of infection, day 12" {
		t.Fatalf("filled grave: %q", got)
	}
	if got := graveLines(game.BuildingView{Kind: "grave", Occupant: "Ada"}); len(got) != 1 {
		t.Fatalf("no epitaph line when blank: %q", got)
	}
	if got := graveLines(game.BuildingView{Kind: "grave"}); len(got) != 1 || got[0] != "Empty grave" {
		t.Fatalf("empty grave: %q", got)
	}
	if graveLines(game.BuildingView{Kind: "wall"}) != nil {
		t.Fatal("only graves")
	}
}

func TestDigSwings(t *testing.T) {
	t.Parallel()
	d, ok := workTable[game.ActionDig]
	if !ok || d.motion != motionSwing {
		t.Fatalf("dig row: %+v", d)
	}
	if len(d.parts) == 0 || d.parts[0].kind != partSoil || d.at != atTarget {
		t.Fatalf("soil at the grave tile: %+v", d.parts)
	}
	r, _ := workRenderer(tickAt(3, d.period, strikeAt+0.01))
	if pose := r.workPose(worker(game.ActionDig), moveState{}); !pose.ok || pose.held.key != "" || pose.dx <= 0 || pose.squash <= 0 {
		t.Fatalf("digging leans into the grave, no shovel drawn: %+v", pose)
	}
}

func TestBuryKneelsAndPats(t *testing.T) {
	t.Parallel()
	d, ok := workTable[game.ActionBury]
	if !ok || d.motion != motionReach || d.squash <= 0 {
		t.Fatalf("bury row: %+v", d)
	}
	if len(d.parts) == 0 || d.parts[0].kind != partSoil {
		t.Fatalf("soil: %+v", d.parts)
	}
	r, _ := workRenderer(0)
	if pose := r.workPose(worker(game.ActionBury), moveState{}); !pose.ok || pose.held.key != "" || pose.squash <= 0 {
		t.Fatalf("bare hands: %+v", pose)
	}
}

func TestCremateUsesBench(t *testing.T) {
	t.Parallel()
	d, ok := workTable[game.ActionCremate]
	if !ok || d.motion != motionBench {
		t.Fatalf("cremate row: %+v", d)
	}
	hasFx := false
	for _, p := range d.parts {
		if p.kind == partEmber || p.kind == partSmoke {
			hasFx = true
		}
	}
	if !hasFx {
		t.Fatalf("embers or smoke: %+v", d.parts)
	}
	for _, a := range []string{game.ActionDig, game.ActionBury, game.ActionCremate} {
		if d := workTable[a]; d.motion == motionNone || d.period <= 0 {
			t.Fatalf("%s timing: %+v", a, d)
		}
	}
}

func TestCarryingCorpseShowsBody(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	p := game.PawnView{ID: 2, Carrying: "human corpse"}
	a := r.carryPose(p, moveState{})
	if !a.ok || !a.held.set || a.held.stance != stanceLoad || a.held.key != itemSprite("human corpse") || a.held.key == "" {
		t.Fatalf("body in both arms: %+v", a.held)
	}
	if itemSprite("human corpse") != corpseCarrySprite {
		t.Fatal("the carried body sprite")
	}
}

func TestPyreEmbersEmittedInAnimateNotDraw(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 5, Kind: "funeral pyre", X: 2, Y: 2, W: 2, H: 1, Burning: true}
	r := burialRenderer(nil)
	screen := ebiten.NewImage(256, 256)
	for f := uint64(0); f < 60; f++ {
		r.present = f
		r.drawPyre(screen, b, 64, 256, 256)
	}
	if len(r.parts) != 0 {
		t.Fatal("drawing spawns nothing")
	}
	for f := uint64(0); f < 60; f++ {
		r.present = f
		r.animateWork(game.WorldView{Buildings: []game.BuildingView{b}}, nil)
	}
	if len(r.parts) == 0 {
		t.Fatal("animateWork emits pyre particles")
	}
}
