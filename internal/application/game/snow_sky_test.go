package game

import (
	"testing"

	"rimworld-go/internal/application/game/snow"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/world/weather"
)

// TestClearWaitsForClearSky: nobody picks up the shovel while snow or rain
// falls; the rule off, or a clear sky, lets them.
func TestClearWaitsForClearSky(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.paintSnowClear(8, 5, 8, 5, true)
	g.snowOf().SetCell(snowIdx(g, 8, 5), 120)
	for _, k := range []weather.Kind{weather.Snow, weather.Rain, weather.Blizzard} {
		g.SetWeather(k, 0.5)
		if g.snowOf().AssignClear(id) {
			t.Fatalf("shovelling in %v", k)
		}
	}
	g.SetWeather(weather.Snow, 0.5)
	s := senseOf(g)
	s.Rules = s.Rules.With(gameplay.SnowWaitsForSky, false)
	if !g.snowOf().AssignClear(id) {
		t.Fatal("rule off: the sky is ignored")
	}
}

// TestClearSkyNoFlap: a shovelling job already under way is not dropped by
// a flurry, and the next one waits until the block's weather clears.
func TestClearSkyNoFlap(t *testing.T) {
	t.Parallel()
	g, id := shovelGame(t)
	g.paintSnowClear(8, 5, 8, 5, true)
	g.snowOf().SetCell(snowIdx(g, 8, 5), 200)
	g.SetWeather(weather.Clear, 0)
	if !g.snowOf().AssignClear(id) {
		t.Fatal("clear sky: no job")
	}
	g.SetWeather(weather.Snow, 0.4)
	g.Step(1)
	if j, _ := g.jobs.Get(id); j.Kind != job.ClearSnow {
		t.Fatalf("job dropped by snowfall: %+v", j)
	}
	g.snowOf().SetCell(snowIdx(g, 8, 5), snow.ClearStart)
	g.jobs.Set(id, job.Record{})
	if g.snowOf().AssignClear(id) {
		t.Fatal("started again mid-fall")
	}
}
