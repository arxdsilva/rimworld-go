package ebiten

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// BenchmarkDrawPlants is one frame of vegetation (ART §27.3 budget: 1.2 ms):
// stepping the environment with 6 walkers brushing through, then picking
// the sprite, seasonal key and wind/contact pose of 1000 on-screen plants
// and drawing them, the way drawArt's plant queue does.
func BenchmarkDrawPlants(b *testing.B) {
	species := []string{"tree", "tree", "berry_bush", "potato", "corn", "wheat", "radish", "healroot"}
	w := game.WorldView{Weather: "rain", WeatherIntensity: 0.6, WindX: 1.8, WindY: 0.6, Season: "Fall"}
	var keys []string
	for i := 0; i < 1000; i++ {
		p := game.PlantView{ID: entity.EntityID(100 + i), X: i % 40, Y: i / 40, Species: species[i%len(species)],
			Growth: 20 + i%80, Stage: i % 4, Harvestable: i%3 == 0}
		w.Plants = append(w.Plants, p)
		k, _ := world.PlantSprite(p)
		keys = append(keys, k)
		if fb := world.PlantKeys.Fallback[k]; len(fb) > 0 { // variant 0 of a stage has none
			keys = append(keys, fb[len(fb)-1])
		}
	}
	var movers []*scriptMover
	for i := 0; i < 6; i++ {
		movers = append(movers, &scriptMover{id: entity.EntityID(i + 1), path: line(0, 2+i*4, 1, 0, 38), start: 1})
	}
	r := New(newEnvSim(w, movers...))
	r.art = envArt(keys...)
	r.cam.Zoom = 0.5 // the whole 40x25 field on screen
	screen := ebiten.NewImage(1280, 800)
	tile := r.cam.Tile()
	none := color.NRGBA{}
	kindOf := func(sp string) (env.PlantKind, float64) {
		switch sp {
		case "tree":
			return env.KindTree, 0.022
		case "wheat", "corn":
			return env.KindCrop, 0.07
		case "berry_bush":
			return env.KindBush, 0.035
		}
		return env.KindCrop, 0.04
	}
	frame := func() {
		r.present++
		r.mo.Stepped = 1
		r.stepEnvironment(w, r.game.Snapshot())
		r.pipe().Env.SetView(r.liveView(), 1280, 800, tile)
		for _, p := range w.Plants {
			key, scale := world.PlantSprite(p)
			key = r.seasonKey(key, w)
			kind, amount := kindOf(p.Species)
			ps := r.plantPose(p.X, p.Y, kind, amount, scale)
			r.pipe().DrawSpritePose(screen, key, none, float64(p.X), float64(p.Y), tile, ps)
		}
	}
	for i := 0; i < 30; i++ { // walkers under way, caches warm
		r.advance()
	}
	frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame()
	}
	b.StopTimer()
	if len(r.env.Springs) == 0 {
		b.Fatal("walkers pushed no plants")
	}
}
