package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
)

var wildSpecies = []string{"bush", "berry_bush", "shrub", "reeds", "cactus", "bramble", "moss", "agave",
	"ball_cactus", "elephant_ear", "fire_lily", "dandelion"}

func TestGroundPlantStageKeys(t *testing.T) {
	t.Parallel()
	for _, sp := range wildSpecies[1:] {
		young, _, _ := world.NewPlantKey(game.PlantView{X: 3, Y: 4, Species: sp, Growth: 54}, 9)
		mature, _, _ := world.NewPlantKey(game.PlantView{X: 3, Y: 4, Species: sp, Growth: 55}, 9)
		dormant, _, _ := world.NewPlantKey(game.PlantView{X: 3, Y: 4, Species: sp, Growth: 90, Wilted: true}, 9)
		if !strings.HasPrefix(young, "plant/"+sp+"_young_v") || !strings.HasPrefix(mature, "plant/"+sp+"_mature_v") ||
			!strings.HasPrefix(dormant, "plant/"+sp+"_dormant_v") {
			t.Errorf("%s: %s %s %s", sp, young, mature, dormant)
		}
	}
	h, _, _ := world.NewPlantKey(game.PlantView{Species: "berry_bush", Growth: 100, Harvestable: true}, 1)
	if h != "plant/berry_bush_harvestable_v1" {
		t.Errorf("harvestable berry = %s", h)
	}
	seen := map[string]bool{}
	for x := 0; x < 60; x++ {
		k, _ := world.PlantSprite(game.PlantView{X: x, Y: 1, Species: "moss", Growth: 100})
		seen[k] = true
	}
	if len(seen) != 4 {
		t.Errorf("moss uses %d variants, want 4", len(seen))
	}
}

func TestGroundPlantWinterDormantAndFallbacks(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64})
	// only the mature art: every species resolves to it
	for _, sp := range wildSpecies {
		mk, _ := world.PlantSprite(game.PlantView{X: 0, Y: 2, Species: sp, Growth: 100})
		var mature []string
		for v := 0; v < world.GroundVariants; v++ {
			mature = append(mature, mk[:strings.LastIndex(mk, "_v")]+"_v"+strconv.Itoa(v))
		}
		r.art = envArt(mature...)
		for _, season := range []string{"", "Fall", "Winter"} {
			for x := 0; x < 12; x++ {
				k, _ := world.PlantSprite(game.PlantView{X: x, Y: 2, Species: sp, Growth: 30 + x*6})
				if got := r.seasonKey(k, game.WorldView{Season: season}); !r.art.Has(got) {
					t.Fatalf("%s %q %s resolves to missing %s", sp, season, k, got)
				}
			}
		}
	}
	// with the new art the winter look is the dormant key
	k, _ := world.PlantSprite(game.PlantView{X: 5, Y: 5, Species: "bramble", Growth: 100})
	dormant := strings.Replace(k, "_mature_", "_dormant_", 1)
	r.art = envArt(k, dormant)
	if got := r.seasonKey(k, game.WorldView{Season: "Winter"}); got != dormant {
		t.Errorf("winter = %s, want %s", got, dormant)
	}
	if got := r.seasonKey(k, game.WorldView{Season: "Fall"}); got != k {
		t.Errorf("fall = %s, want %s", got, k)
	}
	// without dormant art, the mature stage stands in in winter
	r.art = envArt(k)
	if got := r.seasonKey(k, game.WorldView{Season: "Winter"}); got != k {
		t.Errorf("no dormant art: winter = %s, want %s", got, k)
	}
}

func TestPlantInspectorSlowsWalking(t *testing.T) {
	t.Parallel()
	for sp, want := range map[string]bool{"bramble": true, "elephant_ear": true, "moss": false, "tree": false, "bush": false} {
		got := false
		for _, l := range inspector.PlantLines(game.PlantView{Species: sp, Growth: 40}) {
			got = got || l == "Slows walking"
		}
		if got != want {
			t.Errorf("%s slows walking = %v, want %v", sp, got, want)
		}
	}
	if inspector.PlantTitle(game.PlantView{Species: "fire_lily"}) != "Fire lily" || inspector.PlantCategory(game.PlantView{Species: "bramble"}) != "Wild plant" {
		t.Error("title or category")
	}
}

func TestWildPlantMarkColors(t *testing.T) {
	t.Parallel()
	seen := map[[4]uint8]string{}
	for _, sp := range wildSpecies[5:] {
		c, k := world.PlantMarkColor(&game.PlantView{Species: sp})
		if k <= 0 {
			t.Errorf("%s mark size %v", sp, k)
		}
		key := [4]uint8{c.R, c.G, c.B, c.A}
		if other, dup := seen[key]; dup {
			t.Errorf("%s shares a mark colour with %s", sp, other)
		}
		seen[key] = sp
	}
}

func TestGrassLabelAt(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 2, Grass: []uint8{1, 1, 0, 1}, GrassGrowth: []uint8{40, 100, 30, 0}}
	if got := hud.GrassLabelAt(w, 0, 0); got != "Grass (growing, 40%)" {
		t.Errorf("growing = %q", got)
	}
	for _, c := range [][2]int{{1, 0}, {0, 1}, {5, 5}, {-1, 0}} {
		if got := hud.GrassLabelAt(w, c[0], c[1]); got != "" {
			t.Errorf("%v = %q, want empty", c, got)
		}
	}
	if got := hud.GrassLabelAt(game.WorldView{TerrainW: 2, Grass: []uint8{1}}, 0, 0); got != "" {
		t.Errorf("empty growth = %q", got)
	}
}

func TestWildHealrootKey(t *testing.T) {
	t.Parallel()
	wild, _ := world.PlantSprite(game.PlantView{X: 1, Y: 1, Species: "healroot", Growth: 80})
	if !strings.HasPrefix(wild, "plant/healroot_wild_mature_v") {
		t.Errorf("wild healroot = %s", wild)
	}
	crop, _ := world.PlantSprite(game.PlantView{X: 1, Y: 1, Species: "healroot", Zone: "healroot", Stage: 2})
	if strings.Contains(crop, "wild") {
		t.Errorf("sown healroot = %s", crop)
	}
}
