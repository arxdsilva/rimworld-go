package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// TestFrameKeyByAttackKind: an attacking animal (or one that just struck)
// draws the attack set named after its natural weapon, wind-up to recover.
func TestFrameKeyByAttackKind(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"bite", "claw", "kick", "gore", "headbutt", "peck"} {
		an := game.AnimalView{ID: 7, Species: "x", Attack: kind, X: 5, Y: 5, Action: game.ActionBite, AttackCycle: 0.3}
		if kind == "kick" {
			an.Action = game.ActionKick
		}
		frame := func(an game.AnimalView, traces ...game.TraceView) string {
			return combatRenderer(100, nil, game.WorldView{Animals: []game.AnimalView{an}, Traces: traces}).animalFrame(an)
		}
		if got, want := frame(an), "_"+kind+"1"; got != want {
			t.Errorf("%s wind-up: got %s want %s", kind, got, want)
		}
		an.AttackCycle = 0.8
		if got, want := frame(an), "_"+kind+"2"; got != want {
			t.Errorf("%s open: got %s want %s", kind, got, want)
		}
		hit := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Melee: true, Hit: true, Attacker: 7, Target: 2, Weapon: kind}
		if got, want := frame(an, hit), "_"+kind+"3"; got != want {
			t.Errorf("%s strike: got %s want %s", kind, got, want)
		}
		hit.Age = 6
		an.Action = ""
		if got, want := frame(an, hit), "_"+kind+"4"; got != want {
			t.Errorf("%s recover (a strike alone picks the set): got %s want %s", kind, got, want)
		}
	}
	// no attack kind on the view (an older sim): the action and weapon decide
	old := game.AnimalView{ID: 7, X: 5, Y: 5, Action: game.ActionKick, AttackCycle: 0.3}
	if got := combatRenderer(100, nil, game.WorldView{}).animalFrame(old); got != "_kick1" {
		t.Errorf("legacy kick: %s", got)
	}
}

// TestFemaleVariantByID: a female picks the "_f_" art, and its frames, but
// only when that art exists; otherwise she is drawn from the male set.
func TestFemaleVariantByID(t *testing.T) {
	t.Parallel()
	r := combatRenderer(100, nil, game.WorldView{})
	r.art.Sprites["animal/deer_south"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	doe := game.AnimalView{ID: 3, Species: "deer", Female: true, X: 2, Y: 2}
	if got := r.animalBaseKey(doe); got != "animal/deer_south" {
		t.Fatalf("no female art: falls back to the male set, got %s", got)
	}
	r.art.Sprites["animal/deer_f_south"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	r.art.Sprites["animal/deer_f_south_graze1"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	if got := r.animalBaseKey(doe); got != "animal/deer_f_south" {
		t.Fatalf("female art: got %s", got)
	}
	doe.Action = game.ActionGraze
	if got := r.animalFrameKey(doe); got != "animal/deer_f_south_graze1" && got != "animal/deer_f_south" {
		t.Fatalf("frames follow the female base: %s", got)
	}
	buck := doe
	buck.Female = false
	if got := r.animalBaseKey(buck); got != "animal/deer_south" {
		t.Fatalf("male keeps the plain set: %s", got)
	}
}

// TestBlurbsForAll: every roster species has its own short description.
func TestBlurbsForAll(t *testing.T) {
	t.Parallel()
	if len(animalBlurbs) != len(animal.All()) {
		t.Fatalf("%d blurbs for %d species", len(animalBlurbs), len(animal.All()))
	}
	seen := map[string]string{}
	for _, s := range animal.All() {
		key := s.String()
		b, ok := animalBlurbs[key]
		if !ok || strings.TrimSpace(b) == "" {
			t.Errorf("%s has no blurb", key)
			continue
		}
		if len(b) > 70 {
			t.Errorf("%s blurb is %d chars (max 70): %q", key, len(b), b)
		}
		if other, dup := seen[b]; dup {
			t.Errorf("%s repeats %s's blurb", key, other)
		}
		seen[b] = key
		if animalBlurb(key) != b {
			t.Errorf("animalBlurb(%s) does not return its entry", key)
		}
	}
	if animalBlurb("not_a_species") == "" {
		t.Error("unknown species still gets the generic blurb")
	}
}

// TestCorpseSpriteBySpecies: an animal carcass draws its own species'
// corpse art, one of three variants by tile; the legacy kinds still work.
func TestCorpseSpriteBySpecies(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for x := 0; x < 20; x++ {
		got := itemSpriteSpecies("animal corpse", "elephant", x, 3)
		if !strings.HasPrefix(got, "item/corpse_elephant_v") {
			t.Fatalf("elephant corpse: %s", got)
		}
		seen[got] = true
	}
	if len(seen) != 3 {
		t.Fatalf("three variants by tile, saw %v", seen)
	}
	if got := itemSpriteSpecies("animal corpse", "elephant", 4, 4); got != itemSpriteSpecies("animal corpse", "elephant", 4, 4) {
		t.Fatal("the variant is stable per tile")
	}
	if got := itemSpriteSpecies("animal corpse", "", 4, 4); !strings.HasPrefix(got, "item/corpse_deer_v") {
		t.Fatalf("a carcass of unknown species falls back to the deer: %s", got)
	}
	if got := itemSpriteSpecies("dustbuck corpse", "", 4, 4); !strings.HasPrefix(got, "item/corpse_dustbuck_v") {
		t.Fatalf("legacy dustbuck corpse: %s", got)
	}
	if got := itemSpriteSpecies("wood", "", 1, 1); got != itemSprite("wood") {
		t.Fatalf("other items unchanged: %s", got)
	}
	if itemSpriteAt("animal corpse", 4, 4) != itemSpriteSpecies("animal corpse", "", 4, 4) {
		t.Fatal("itemSpriteAt is the species-less form")
	}
}

// TestOverlayAboveTallAnimals: hunt and tame marks and health bars sit
// higher over bigger animals and hug small ones. PLAY-022 C8 re-expresses
// the sizes on the new length table (rat XS 1.45, deer M 1.8, elephant XL
// 3.0; they were 0.44, 0.95 and 2.0); an unknown size still counts as the
// deer. With art, the marks follow the sprite itself (TestMarksClearTheArt).
func TestOverlayAboveTallAnimals(t *testing.T) {
	t.Parallel()
	rat, deer, elephant := 1.45, 1.8, 3.0
	if animalMarkerLift(0) != animalMarkerLift(deer) || animalBarLift(0) != animalBarLift(deer) {
		t.Fatal("an unknown size is drawn like the deer")
	}
	if !(animalMarkerLift(rat) < animalMarkerLift(deer) && animalMarkerLift(deer) < animalMarkerLift(elephant)) {
		t.Fatalf("marker lift grows with size: %v %v %v", animalMarkerLift(rat), animalMarkerLift(deer), animalMarkerLift(elephant))
	}
	if !(animalBarLift(rat) < animalBarLift(deer) && animalBarLift(deer) < animalBarLift(elephant)) {
		t.Fatal("bar lift grows with size")
	}
	for _, size := range []float64{rat, deer, elephant} {
		h := animalHeight(size)
		if animalMarkerLift(size) < h+0.4 {
			t.Errorf("size %v: marker %.2f must clear height %.2f", size, animalMarkerLift(size), h)
		}
		if animalBarLift(size) < 0.1 {
			t.Errorf("size %v: bar sits on the animal", size)
		}
		if animalBarLift(size) >= animalMarkerLift(size) {
			t.Errorf("size %v: marker is above the bar", size)
		}
	}
	// drawing every marker kind over mixed sizes must not panic
	r := combatRenderer(100, nil, game.WorldView{})
	w := game.WorldView{Animals: []game.AnimalView{
		{ID: 1, Size: rat, Designated: true, Health: 50, X: 1, Y: 1},
		{ID: 2, Size: elephant, TameMark: true, Health: 90, X: 3, Y: 3},
		{ID: 3, Size: deer, Manhunter: true, Health: 100, X: 5, Y: 5},
	}}
	r.drawAnimalOverlays(ebiten.NewImage(256, 256), w, 64, 256, 256)
}

// TestAnimalLabelsShown: the UI shows the display label, not the key.
func TestAnimalLabelsShown(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{Species: "arctic_fox", Label: "arctic fox"}
	if got := animalTitle(an); got != "Arctic fox" {
		t.Fatalf("title: %q", got)
	}
	an.Tame, an.Name = true, "Frost"
	if got := animalTitle(an); got != "Frost (tame arctic fox)" {
		t.Fatalf("tame title: %q", got)
	}
	if got := animalLabel(game.AnimalView{Species: "arctic_fox"}); got != "arctic fox" {
		t.Fatalf("no label: key without underscores, got %q", got)
	}
	if got := animalLabel(game.AnimalView{}); got == "" {
		t.Fatal("a label always exists")
	}
	// the tab row keeps its columns for the longest label
	row := animalRow(game.AnimalView{Name: "Frost", Label: "yorkshire terrier", Health: 88, Mode: "wandering"})
	if !strings.Contains(row, "yorkshire terrier") || !strings.Contains(row, " 88%") {
		t.Fatalf("row: %q", row)
	}
}

// TestIconFallbackInitial: a species without a ui_icon gets a letter on a
// disc instead of nothing.
func TestIconFallbackInitial(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"arctic fox": "A", "": "?", "  yak": "Y", "hive_brute": "H"} {
		if got := animalInitial(in); got != want {
			t.Errorf("animalInitial(%q) = %q, want %q", in, got, want)
		}
	}
	r := combatRenderer(100, nil, game.WorldView{})
	screen := ebiten.NewImage(64, 64)
	// no art catalog, an art catalog without the icon: both draw the disc
	drawAnimalIcon(screen, nil, game.AnimalView{Species: "yak", Label: "yak"}, 8, 8, 40)
	drawAnimalIcon(screen, r.art, game.AnimalView{Species: "yak", Label: "yak"}, 8, 8, 40)
}

// TestBurstEmitsOnce: an explosion stamp sparks a flash, embers and smoke
// the first time it is seen, and never again.
func TestBurstEmitsOnce(t *testing.T) {
	t.Parallel()
	r := combatRenderer(100, nil, game.WorldView{})
	var tr burstTracker
	ev := []burstEvent{{ID: 4, X: 6, Y: 7, Tick: 90}}
	tr.emit(r, ev)
	n := len(r.parts)
	if n < 10 {
		t.Fatalf("a burst throws many particles, got %d", n)
	}
	kinds := map[partKind]bool{}
	for _, p := range r.parts {
		kinds[p.kind] = true
	}
	for _, k := range []partKind{partEmber, partSmoke} {
		if !kinds[k] {
			t.Errorf("missing particle kind %d", k)
		}
	}
	tr.emit(r, ev)
	if len(r.parts) != n {
		t.Fatalf("the same stamp does not burst twice: %d -> %d", n, len(r.parts))
	}
	tr.emit(r, append(ev, burstEvent{ID: 5, X: 1, Y: 1, Tick: 95}))
	if len(r.parts) <= n {
		t.Fatal("a new stamp bursts")
	}
	// determinism: the same stamp makes the same particles
	r2 := combatRenderer(100, nil, game.WorldView{})
	var tr2 burstTracker
	tr2.emit(r2, ev)
	if fmt.Sprint(r2.parts) != fmt.Sprint(r.parts[:n]) {
		t.Fatal("bursts are deterministic")
	}
}

// TestWorldItemsHaveSprites: every ground item the game shows draws a real
// sprite. Carcasses come from the read model as "<species> corpse", which
// used to fall through to a missing key (orange squares).
func TestWorldItemsHaveSprites(t *testing.T) {
	c := realArt(t)
	g := game.New(game.Config{Seed: 1, Width: 16, Height: 12})
	for i, sp := range []string{"cobra", "capybara", "deer", "elephant", "chicken"} {
		id, err := g.SpawnAnimal(sp, 2+i*2, 4)
		if err != nil {
			t.Fatal(err)
		}
		g.KillAnimal(id)
	}
	w := g.World()
	if len(w.Items) < 5 {
		t.Fatalf("%d items, want the 5 carcasses", len(w.Items))
	}
	for _, it := range w.Items {
		if key := itemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y); !c.Has(key) {
			t.Errorf("%q (animal %q) draws %q, which has no sprite", it.Kind, it.Animal, key)
		}
	}
}

// TestCarriedCarcassShowsItsSpecies: a hauler carrying a carcass holds that
// species' body, not nothing.
func TestCarriedCarcassShowsItsSpecies(t *testing.T) {
	c := testArt(t)
	key := "item/corpse_" + "capybara" + "_v0"
	if !c.Has(key) {
		t.Fatalf("no %s", key)
	}
	p := game.PawnView{Carrying: "animal corpse", CarryingAnimal: "capybara"}
	r := &Renderer{art: c}
	if a := r.carryPose(p, moveState{}); a.held.key != key {
		t.Fatalf("held %q, want %q", a.held.key, key)
	}
}

// --- PLAY-022: sprite density, picking and marks ----------------------------

// bodySprite is an animal sprite baked at ppt px per tile whose opaque
// pixels fill body (canvas px); its pixels are the full canvas.
func bodySprite(canvas [2]int, origin pt, ppt float64, body image.Rectangle) *artSprite {
	pix := image.NewNRGBA(image.Rect(0, 0, canvas[0], canvas[1]))
	draw.Draw(pix, body, image.NewUniform(color.NRGBA{120, 90, 60, 255}), image.Point{}, draw.Src)
	return &artSprite{Meta: artMeta{Canvas: canvas, Origin: origin, PPT: ppt, Gait: 1.136}, Pix: pix}
}

// xlArt is a catalog whose "elephant" (every facing, female none) is an XL
// body at 64 px per tile: canvas 6 x 3 tiles, the body 5 tiles long, from
// 2 tiles west of its cell to 3 east, and from 1 tile above its cell's top
// to 0.75 into it.
func xlArt() *artCatalog {
	c := &artCatalog{Sprites: map[string]*artSprite{}, Tinted: map[tintKey]*ebiten.Image{}}
	for _, f := range []string{"south", "east", "north"} {
		c.Sprites["animal/elephant_"+f] = bodySprite([2]int{384, 192}, pt{160, 128}, 64, image.Rect(32, 64, 352, 176))
	}
	return c
}

// A sprite baked at 64 px per tile covers the same screen box as one at
// 128 (artOrigin, spritePoseGeo), posed, mirrored and shrunk alike; its
// silhouette's κ and height match too (shadow_cast.go), and a carried
// carcass baked at 64 sits and scales like one at 128 (held.go).
func TestAnimalDensityKeepsTileSize(t *testing.T) {
	t.Parallel()
	hi := artMeta{Canvas: [2]int{512, 384}, Origin: pt{128, 256}, PPT: 128}
	lo := artMeta{Canvas: [2]int{256, 192}, Origin: pt{64, 128}, PPT: 64}
	box := func(m artMeta, ps pose) [4]float64 {
		g := spritePoseGeo(m, 300, 200, 48, ps)
		x0, y0 := g.Apply(0, 0)
		x1, y1 := g.Apply(float64(m.Canvas[0]), float64(m.Canvas[1]))
		return [4]float64{math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1)}
	}
	base := pose{px: figurePivotX, py: figurePivotY}
	for _, ps := range []pose{base, mirrored(base), youngPose(base, 0.6), {px: 0.5, py: 0.55, rot: 0.1, sy: 1.02}} {
		a, b := box(hi, ps), box(lo, ps)
		for i := range a {
			if math.Abs(a[i]-b[i]) > 1e-9 {
				t.Fatalf("pose %+v: ppt 128 box %v, ppt 64 box %v", ps, a, b)
			}
		}
	}
	if k1, k2 := spriteKappa(1.6, 200, 128), spriteKappa(1.6, 100, 64); math.Abs(k1-k2) > 1e-12 {
		t.Fatalf("kappa %v at 128 ppt, %v at 64", k1, k2)
	}
	if a, b := shadowFallback("animal/x", &hi), shadowFallback("animal/x", &lo); a != b {
		t.Fatalf("fallback caster %+v at 128 ppt, %+v at 64", a, b)
	}
	r := artRenderer()
	r.art.Sprites["item/corpse_hi"] = &artSprite{Meta: artMeta{Canvas: [2]int{384, 160}, Origin: pt{128, 16}, PPT: 128}}
	r.art.Sprites["item/corpse_lo"] = &artSprite{Meta: artMeta{Canvas: [2]int{192, 80}, Origin: pt{64, 8}, PPT: 64}}
	rig, ps := rigFor(r, faceSouth)
	place := func(key string) [2]pt {
		pl, ok := r.heldPlace(rig, heldItem{key: key, stance: stanceLoad}, 5, 5, 64, ps)
		if !ok {
			t.Fatalf("%s not placed", key)
		}
		m := r.art.Sprites[key].Meta
		return [2]pt{apply(pl.g, pt{0, 0}), apply(pl.g, pt{float64(m.Canvas[0]), float64(m.Canvas[1])})}
	}
	if a, b := place("item/corpse_hi"), place("item/corpse_lo"); !near2(a[0], b[0], 1e-6) || !near2(a[1], b[1], 1e-6) {
		t.Fatalf("a carried carcass at 128 ppt spans %v, at 64 %v", a, b)
	}
}

// pickRenderer is a renderer over a stub world holding w, with xlArt, the
// camera at 64 px tiles from the map's corner, synced once.
func pickRenderer(w game.WorldView) *Renderer {
	r := New(&stubSim{w: 40, h: 40, world: w})
	r.art = xlArt()
	r.cam = camera{Zoom: 1, Placed: true}
	r.advance()
	return r
}

// C7: a big animal's body covers the tiles its standing frame's opaque box
// overlaps by pickCover, from its drawn position (west mirrored, a
// juvenile smaller); the box select takes it by any of them too.
func TestBigAnimalBodyTilesPick(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{ID: 5, Species: "elephant", X: 10, Y: 10, Size: 3}
	w := game.WorldView{Animals: []game.AnimalView{an}}
	r := pickRenderer(w)
	x0, y0, x1, y1, ok := r.animalBodyRect(an)
	if !ok || math.Abs(x0-8) > 1e-9 || math.Abs(x1-13) > 1e-9 || math.Abs(y0-9) > 1e-9 || math.Abs(y1-10.75) > 1e-9 {
		t.Fatalf("body box %v,%v - %v,%v, want 8,9 - 13,10.75", x0, y0, x1, y1)
	}
	covers := func(tx, ty int) bool {
		got := r.animalsCovering(w, tx, ty)
		return len(got) == 1 && got[0] == (selCand{selAnimal, 5})
	}
	for _, c := range [][2]int{{8, 9}, {12, 10}, {11, 9}, {9, 10}} {
		if !covers(c[0], c[1]) {
			t.Errorf("tile %v is under the body", c)
		}
	}
	for _, c := range [][2]int{{10, 10}, {13, 10}, {7, 10}, {10, 11}, {10, 8}} {
		if covers(c[0], c[1]) {
			t.Errorf("tile %v: not a body tile (or the animal's own cell)", c)
		}
	}
	young := an
	young.Scale = 0.5
	if r.animalsCovering(game.WorldView{Animals: []game.AnimalView{young}}, 8, 9) != nil {
		t.Error("a calf half the size does not reach two tiles west")
	}
	// the box select: a box holding the centre of tile (12, 10) only
	sx, sy := r.worldToScreenF(12.4, 10.4)
	ex, ey := r.worldToScreenF(12.6, 10.6)
	if !r.bodyInRect(an, sx, sy, ex, ey) {
		t.Error("a box over a flank tile takes the animal")
	}
	sx, sy = r.worldToScreenF(14.2, 10.2)
	ex, ey = r.worldToScreenF(14.8, 10.8)
	if r.bodyInRect(an, sx, sy, ex, ey) {
		t.Error("a box beside the body takes it")
	}
}

// C7 precedence: whatever stands on the clicked tile comes first (pawn,
// animal, item, frame, building); a body covering it from another cell
// comes next, before the tile's plants, floor and zone.
func TestBodyPickComesAfterTheTilesOwnThings(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		Animals: []game.AnimalView{{ID: 5, Species: "elephant", X: 10, Y: 10}},
		Items:   []game.ItemView{{ID: 6, Kind: "wood", Qty: 3, X: 12, Y: 10}},
		Plants:  []game.PlantView{{ID: 7, Species: "grass", X: 12, Y: 10}, {ID: 8, Species: "grass", X: 11, Y: 9}},
	}
	out, solid := selectablesSplit(w, nil, 12, 10)
	if solid != 1 || len(out) != 2 || out[0].kind != selItem || out[1].kind != selPlant {
		t.Fatalf("tile (12, 10): %+v, solid %d", out, solid)
	}
	a := testApp(&stubSim{w: 40, h: 40, world: w})
	a.renderer = pickRenderer(w)
	got := a.selectables(w, 12, 10)
	if len(got) != 3 || got[0].kind != selItem || got[1] != (selCand{selAnimal, 5}) || got[2].kind != selPlant {
		t.Fatalf("item, then the body, then the grass: %+v", got)
	}
	if got := a.selectables(w, 11, 9); len(got) != 2 || got[0] != (selCand{selAnimal, 5}) {
		t.Fatalf("a flank tile with grass: %+v", got)
	}
}

// C8: with art, the hunt and hostile marks float animalMarkAbove over the
// standing canvas's top (a calf's lower), and the bar sits between it and
// the body; the size estimate serves animals without art.
func TestMarksClearTheArt(t *testing.T) {
	tall := game.AnimalView{ID: 5, Species: "elephant", X: 10, Y: 10, Size: 3, Hostile: true, Health: 50}
	r := pickRenderer(game.WorldView{Animals: []game.AnimalView{tall}})
	top := 128.0 / 64 // the canvas reaches 2 tiles over the cell's top
	if got := r.animalTop(tall, 64); math.Abs(got-top) > 1e-9 {
		t.Fatalf("top %v, want %v", got, top)
	}
	young := tall
	young.Scale = 0.5
	if got, want := r.animalTop(young, 64), (top+figurePivotY)*0.5-figurePivotY; math.Abs(got-want) > 1e-9 {
		t.Fatalf("a calf's top %v, want %v", got, want)
	}
	if got := r.animalTop(tall, 8); got <= top {
		t.Fatalf("drawn at the minimum creature size the body rises higher: %v", got)
	}
	marks := traceMarks(t, r)
	r.drawAnimalOverlays(ebiten.NewImage(1024, 1024), game.WorldView{Animals: []game.AnimalView{tall}}, 64, 1024, 1024)
	lift := ofKind(*marks, "mark-lift")
	_, topY := r.worldToScreenF(10, 10-top)
	if len(lift) != 1 || lift[0].b > topY-float32(animalMarkAbove*64)+0.5 {
		t.Fatalf("the mark at y %v must float %.1f tiles over the body's top (y %v)", lift, animalMarkAbove, topY)
	}
	if barLift(top) <= top || barLift(top) >= markerLift(top) {
		t.Fatalf("bar %v between the top %v and the mark %v", barLift(top), top, markerLift(top))
	}
	bare := &Renderer{}
	if got := bare.animalTop(game.AnimalView{Size: 2.2}, 64); got != animalHeight(2.2) {
		t.Fatalf("no art: top %v, want the estimate %v", got, animalHeight(2.2))
	}
}

// TestRealAnimalBodies checks the renderer's PLAY-022 assumptions on the
// generated art: each size class's median drawn area (the east standing
// frame's opaque box, at its own ppt) within 15% of its target (C1: XS 1,
// S 1.5, M 2, L 4, XL 6 tiles²); east canvases centred on the footprint
// (C2: west is east mirrored about it); a gait on every standing frame
// (C3); and the culls' reach (animalExtent, visPad) covering every animal
// and corpse canvas.
func TestRealAnimalBodies(t *testing.T) {
	t.Parallel()
	c := realArt(t)
	target := map[animal.Size]float64{animal.SizeXS: 1, animal.SizeS: 1.5, animal.SizeM: 2, animal.SizeL: 4, animal.SizeXL: 6}
	areas := map[animal.Size][]float64{}
	for _, s := range animal.All() {
		ro := animal.RosterOf(s)
		for _, f := range []string{"east", "south", "north"} {
			key := "animal/" + ro.Key + "_" + f
			sp := c.Sprite(key)
			if sp == nil {
				t.Fatalf("no %s", key)
			}
			m := sp.Meta
			if m.Gait <= 0 {
				t.Errorf("%s: no gait", key)
			}
			if f != "east" {
				continue
			}
			p := m.PxPerTile()
			if cx := (float64(m.Canvas[0])/2 - m.Origin[0]) / p; math.Abs(cx-0.5) > 1/p {
				t.Errorf("%s: canvas centre %.3f tiles, want the footprint's 0.5 (west mirrors east)", key, cx)
			}
			x0, y0, x1, y1 := sp.OpaqueBox()
			areas[ro.Size] = append(areas[ro.Size], (x1-x0)*(y1-y0)/(p*p))
		}
	}
	for size, want := range target {
		a := areas[size]
		if len(a) == 0 {
			continue
		}
		slices.Sort(a)
		med := a[len(a)/2]
		if len(a)%2 == 0 {
			med = (a[len(a)/2-1] + a[len(a)/2]) / 2
		}
		t.Logf("class %d: median drawn area %.2f tiles² over %d species (target %.1f)", size, med, len(a), want)
		if math.Abs(med-want) > 0.15*want {
			t.Errorf("class %d: median drawn area %.2f tiles², want %.1f ± 15%%", size, med, want)
		}
	}
	e := c.AnimalExtent()
	t.Logf("animal reach past the cell: side %.2f, up %.2f, down %.2f tiles", e.Side, e.Up, e.Down)
	if e.Side <= 0 || e.Side > animalSpanFallback || e.Up > animalSpanFallback || e.Down > animalSpanFallback {
		t.Errorf("animal extent %+v outside the fallback's %v", e, animalSpanFallback)
	}
	var corpse float64
	for k, sp := range c.Sprites {
		if !strings.HasPrefix(k, "item/corpse_") {
			continue
		}
		m := sp.Meta
		p := m.PxPerTile()
		corpse = max(corpse, m.Origin[0]/p, (float64(m.Canvas[0])-m.Origin[0])/p-1, m.Origin[1]/p, (float64(m.Canvas[1])-m.Origin[1])/p-1)
	}
	t.Logf("corpse reach past the cell: %.2f tiles", corpse)
	if corpse+shadowMaxLen > visPad {
		t.Errorf("a corpse reaches %.2f tiles past its cell and its shadow %v more: over visPad %d", corpse, shadowMaxLen, visPad)
	}
}

// TestRealElephantFlankPicks: on the generated art, an elephant standing
// east covers the tiles beside its cell (C7), and a rat does not reach two
// tiles away.
func TestRealElephantFlankPicks(t *testing.T) {
	t.Parallel()
	c := realArt(t)
	w := game.WorldView{Animals: []game.AnimalView{
		{ID: 5, Species: "elephant", X: 10, Y: 10, Size: 3},
		{ID: 6, Species: "rat", X: 20, Y: 10, Size: 1.45},
	}}
	r := New(&stubSim{w: 40, h: 40, world: w})
	r.art = c
	r.cam = camera{Zoom: 1, Placed: true}
	r.advance()
	for _, id := range []entity.EntityID{5, 6} {
		v := r.last[id]
		v.facing = faceEast
		r.last[id] = v
	}
	for _, tile := range [][2]int{{9, 10}, {11, 10}} {
		if got := r.animalsCovering(w, tile[0], tile[1]); len(got) != 1 || got[0].id != 5 {
			t.Errorf("tile %v beside the elephant: %+v", tile, got)
		}
	}
	if got := r.animalsCovering(w, 22, 10); len(got) != 0 {
		t.Errorf("a rat reaches two tiles east: %+v", got)
	}
	x0, y0, x1, y1, _ := r.animalBodyRect(w.Animals[0])
	t.Logf("elephant east body box: %.2f,%.2f - %.2f,%.2f", x0-10, y0-10, x1-10, y1-10)
}
