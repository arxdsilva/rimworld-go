package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
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
	plan := creature.CorpsePlanFor(r.art, it)
	side := math.Copysign(1, plan.Ps.Rot)
	want := creature.LyingPose(r.art, side)
	want.Cs = creature.DeadCS
	if plan.Ps != want || math.Abs(math.Abs(plan.Ps.Rot)-math.Pi/2) > 1e-9 {
		t.Fatalf("the standing figure laid flat on its side: %+v", plan.Ps)
	}
	if plan.Ps.Cs != creature.DeadCS {
		t.Fatalf("dead tint: %v", plan.Ps.Cs)
	}
	it.Rotten = true
	if creature.CorpsePlanFor(r.art, it).Ps.Cs != spritepkg.RottenCS {
		t.Fatal("rotten tint")
	}
	// the side it fell on is stable per body
	if creature.CorpsePlanFor(r.art, it).Ps.Rot != plan.Ps.Rot {
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

func TestBonesSprite(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for x := 0; x < 20; x++ {
		k := creature.BonesSprite(x, 3)
		if k != creature.BonesSprite(x, 3) {
			t.Fatal("stable per tile")
		}
		if k != "item/bones_00" && k != "item/bones_01" {
			t.Fatalf("variant key %s", k)
		}
		if want := env.ArtKey("item", "bones", int(spritepkg.TileHash(x, 3)%2)); k != want {
			t.Fatalf("variant by tileHash: %s vs %s", k, want)
		}
		seen[k] = true
	}
	if len(seen) != 2 {
		t.Fatal("both variants appear")
	}
	if itemSpriteAt("bones", 4, 4) != creature.BonesSprite(4, 4) {
		t.Fatal("itemSpriteAt maps bones")
	}
	if got := itemSpriteAt("dustbuck corpse", 4, 4); !strings.HasPrefix(got, "item/corpse_dustbuck_v") {
		t.Fatalf("animal corpse variants: %s", got)
	}
	if itemSpriteAt("wood", 1, 1) != world.ItemSprite("wood") {
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
		if got := creature.GraveSprite(c.b); got != c.want {
			t.Fatalf("%+v: %s want %s", c.b, got, c.want)
		}
	}
	r := burialRenderer(nil)
	r.drawGrave(ebiten.NewImage(256, 256), game.BuildingView{Kind: "grave", X: 1, Y: 1, Occupant: "Ada"}, 64, 256, 256)
}

func TestPyreBurningDrawsFireAndSmoke(t *testing.T) {
	t.Parallel()
	if creature.PyreSprite(game.BuildingView{Kind: "funeral pyre"}) != "furniture/pyre" {
		t.Fatal("cold pyre")
	}
	if creature.PyreSprite(game.BuildingView{Kind: "funeral pyre", Burning: true}) != "furniture/pyre_burning" {
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
	kinds := map[creature.PartKind]int{}
	for _, p := range hot.parts {
		kinds[p.Kind]++
	}
	if kinds[creature.PartSmoke] == 0 || kinds[creature.PartEmber] == 0 {
		t.Fatalf("smoke and embers: %v", kinds)
	}
	if creature.PyreFlameScale <= 1 {
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
		if _, ok := architect.Buildable(tool.build); !ok {
			t.Fatalf("%s builds unknown kind %q", id, tool.build)
		}
	}
	if world.BuildSprite("grave", "east", 0, false, false) != "furniture/grave_dug_east" ||
		world.BuildSprite("funeral pyre", "", 0, false, false) != "furniture/pyre" {
		t.Fatal("ghost sprites")
	}
}

func TestGraveInspectorShowsEpitaph(t *testing.T) {
	t.Parallel()
	got := inspector.GraveLines(game.BuildingView{Kind: "grave", Occupant: "Ada", Epitaph: "Ada - died of infection, day 12"})
	if len(got) != 2 || got[0] != "Here lies Ada" || got[1] != "Ada - died of infection, day 12" {
		t.Fatalf("filled grave: %q", got)
	}
	if got := inspector.GraveLines(game.BuildingView{Kind: "grave", Occupant: "Ada"}); len(got) != 1 {
		t.Fatalf("no epitaph line when blank: %q", got)
	}
	if got := inspector.GraveLines(game.BuildingView{Kind: "grave"}); len(got) != 1 || got[0] != "Empty grave" {
		t.Fatalf("empty grave: %q", got)
	}
	if inspector.GraveLines(game.BuildingView{Kind: "wall"}) != nil {
		t.Fatal("only graves")
	}
}

func TestDigSwings(t *testing.T) {
	t.Parallel()
	d, ok := creature.WorkTable[game.ActionDig]
	if !ok || d.Motion != creature.MotionSwing {
		t.Fatalf("dig row: %+v", d)
	}
	if len(d.Parts) == 0 || d.Parts[0].Kind != creature.PartSoil || d.At != creature.AtTarget {
		t.Fatalf("soil at the grave tile: %+v", d.Parts)
	}
	r, _ := workRenderer(tickAt(3, d.Period, creature.StrikeAt+0.01))
	if pose := r.workPose(worker(game.ActionDig), motionpkg.MoveState{}); !pose.Ok || pose.Held.Key != "" || pose.Dx <= 0 || pose.Squash <= 0 {
		t.Fatalf("digging leans into the grave, no shovel drawn: %+v", pose)
	}
}

func TestBuryKneelsAndPats(t *testing.T) {
	t.Parallel()
	d, ok := creature.WorkTable[game.ActionBury]
	if !ok || d.Motion != creature.MotionReach || d.Squash <= 0 {
		t.Fatalf("bury row: %+v", d)
	}
	if len(d.Parts) == 0 || d.Parts[0].Kind != creature.PartSoil {
		t.Fatalf("soil: %+v", d.Parts)
	}
	r, _ := workRenderer(0)
	if pose := r.workPose(worker(game.ActionBury), motionpkg.MoveState{}); !pose.Ok || pose.Held.Key != "" || pose.Squash <= 0 {
		t.Fatalf("bare hands: %+v", pose)
	}
}

func TestCarryingCorpseShowsBody(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	p := game.PawnView{ID: 2, Carrying: "human corpse"}
	a := r.carryPose(p, motionpkg.MoveState{})
	if !a.Ok || !a.Held.Set || a.Held.Stance != creature.StanceLoad || a.Held.Key != world.ItemSprite("human corpse") || a.Held.Key == "" {
		t.Fatalf("body in both arms: %+v", a.Held)
	}
	if world.ItemSprite("human corpse") != creature.CorpseCarrySprite {
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
