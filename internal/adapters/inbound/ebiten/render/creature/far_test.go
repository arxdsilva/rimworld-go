package creature

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// farMarks runs DrawFarPawns over two colonists and a raider plus a
// manhunter at the map tier and returns the marks it drew.
func farMarks(mode, style string, mapTier bool) []markTrace {
	c, v, _ := testSet()
	v.Prefs.FarPawns, v.Prefs.FarStyle = mode, style
	v.Pawns = []game.PawnView{{ID: 1, X: 3, Y: 3}, {ID: 2, X: 5, Y: 3}, {ID: 3, X: 7, Y: 3, Hostile: true}, {ID: 4, X: 9, Y: 3, Embodied: true}}
	w := game.WorldView{Animals: []game.AnimalView{{ID: 10, X: 4, Y: 6, Manhunter: true}, {ID: 11, X: 4, Y: 8}}}
	marks := traceMarks(v)
	cull := Cull{MapTier: mapTier, White: ebiten.NewImage(1, 1)}
	DrawFarPawns(c, v, &cull, ebiten.NewImage(64, 64), w)
	return *marks
}

func countKind(ms []markTrace, kind string) int {
	n := 0
	for _, m := range ms {
		if m.kind == kind {
			n++
		}
	}
	return n
}

func TestFarPawnsModesAndStyles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, mode, style string
		tier              bool
		dots, sils        int
	}{
		{"none draws nothing", "none", "both", true, 0, 0},
		{"below the map tier draws nothing", "all", "both", false, 0, 0},
		{"hostile dots: raider and manhunter", "hostile", "dots", true, 2, 0},
		{"hostile silhouettes", "hostile", "silhouettes", true, 0, 2},
		{"all dots: three pawns, one manhunter (the embodied is skipped)", "all", "dots", true, 4, 0},
		{"all both", "all", "both", true, 4, 4},
		{"unknown mode draws nothing", "bogus", "dots", true, 0, 0},
	}
	for _, tc := range cases {
		ms := farMarks(tc.mode, tc.style, tc.tier)
		if d, s := countKind(ms, "far-dot"), countKind(ms, "far-silhouette"); d != tc.dots || s != tc.sils {
			t.Errorf("%s: dots %d silhouettes %d, want %d and %d", tc.name, d, s, tc.dots, tc.sils)
		}
	}
}

func TestFarPawnsCulledToTheView(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	v.Prefs.FarPawns, v.Prefs.FarStyle = "all", "dots"
	v.Pawns = []game.PawnView{{ID: 1, X: 3, Y: 3}, {ID: 2, X: 90, Y: 90}}
	marks := traceMarks(v)
	cull := Cull{MapTier: true, ViewSet: true, X0: 0, Y0: 0, X1: 20, Y1: 20, White: ebiten.NewImage(1, 1)}
	DrawFarPawns(c, v, &cull, ebiten.NewImage(64, 64), game.WorldView{})
	if len(*marks) != 1 {
		t.Fatalf("marks %+v, want only the pawn in view", *marks)
	}
}

func TestAnimalLabelModes(t *testing.T) {
	t.Parallel()
	named := &game.AnimalView{Tame: true, Name: "Biscuit", Label: "dog"}
	plain := &game.AnimalView{Tame: true, Label: "dog"}
	wild := &game.AnimalView{Name: "Rex", Label: "wolf"}
	for _, tc := range []struct {
		mode string
		an   *game.AnimalView
		want string
	}{
		{"none", named, ""}, {"none", plain, ""},
		{"named", named, "Biscuit"}, {"named", plain, ""}, {"named", wild, ""},
		{"tame", named, "Biscuit"}, {"tame", plain, "dog"}, {"tame", wild, ""},
		{"", named, "Biscuit"},
	} {
		if got := AnimalLabel(tc.mode, tc.an); got != tc.want {
			t.Errorf("AnimalLabel(%q, %+v) = %q, want %q", tc.mode, *tc.an, got, tc.want)
		}
	}
}

func TestAnimalNamePlatesFollowTheMode(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]int{"none": 0, "named": 1, "tame": 2} {
		c, v, _ := testSet()
		v.Prefs.AnimalNames = mode
		marks := traceMarks(v)
		w := game.WorldView{Animals: []game.AnimalView{
			{ID: 1, X: 2, Y: 2, Health: 100, Tame: true, Name: "Biscuit", Label: "dog"},
			{ID: 2, X: 4, Y: 2, Health: 100, Tame: true, Label: "dog"},
		}}
		cull := Cull{White: ebiten.NewImage(1, 1)}
		c.DrawAnimalOverlays(v, sprite.Pass{}, &cull, ebiten.NewImage(64, 64), w, v.Cam.Tile())
		if n := countKind(*marks, "name-plate"); n != want {
			t.Errorf("mode %q: %d plates, want %d", mode, n, want)
		}
	}
}
