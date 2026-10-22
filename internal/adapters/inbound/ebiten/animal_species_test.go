package ebiten

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
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
	r.art.sprites["animal/deer_south"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	doe := game.AnimalView{ID: 3, Species: "deer", Female: true, X: 2, Y: 2}
	if got := r.animalBaseKey(doe); got != "animal/deer_south" {
		t.Fatalf("no female art: falls back to the male set, got %s", got)
	}
	r.art.sprites["animal/deer_f_south"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	r.art.sprites["animal/deer_f_south_graze1"] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
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
// higher over bigger animals and hug small ones.
func TestOverlayAboveTallAnimals(t *testing.T) {
	t.Parallel()
	rat, deer, elephant := 0.35, 0.95, 2.0
	if animalMarkerLift(0) != animalMarkerLift(deer) || animalBarLift(0) != animalBarLift(deer) {
		t.Fatal("an unknown size is drawn like the former deer")
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
