package game

import (
	"bytes"
	"math"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/weather"
)

func weatherGame(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 24, Height: 14})
	jumpTo(g, 10, 1) // spring morning, early in a weather block
	return g
}

func remembers(g *Game, id entity.EntityID, k mood.Kind) bool {
	for _, m := range g.memories[id] {
		if m.Kind == k {
			return true
		}
	}
	return false
}

func TestSetWeatherHook(t *testing.T) {
	g := weatherGame(t)
	g.SetWeather(weather.Storm, 0.9)
	w := g.Weather()
	if w.Kind != weather.Storm || w.Target != weather.Storm || w.Intensity != 0.9 || w.Goal != 0.9 {
		t.Errorf("SetWeather: %+v", w)
	}
}

func TestPhaseWeatherAdvancesDeterministically(t *testing.T) {
	a, b := weatherGame(t), weatherGame(t)
	a.Step(3 * 15000)
	b.Step(3 * 15000)
	if a.Weather() != b.Weather() {
		t.Fatalf("weather diverged: %+v vs %+v", a.Weather(), b.Weather())
	}
	if a.Weather().WindSpeed == 0 {
		t.Error("wind should be blowing after blocks have passed")
	}
	c := New(Config{Seed: 2, Width: 24, Height: 14})
	jumpTo(c, 10, 1)
	c.generate(2)
	d := New(Config{Seed: 3, Width: 24, Height: 14})
	jumpTo(d, 10, 1)
	d.generate(3)
	c.Step(40 * 15000)
	d.Step(40 * 15000)
	if c.Weather() == d.Weather() {
		t.Error("different maps should get different weather")
	}
}

func TestRainSlowsOutdoorStepsNotIndoor(t *testing.T) {
	g := weatherGame(t)
	hut(t, g, 10, 2, 16, 8)
	in, out := 13, 5
	cases := []struct {
		name  string
		set   func()
		wantO int
	}{
		{"clear", func() { g.SetWeather(weather.Clear, 0) }, 10},
		{"rain", func() { g.SetWeather(weather.Rain, 0.6) }, 12},
		{"storm", func() { g.SetWeather(weather.Storm, 1) }, 13},
		{"blizzard", func() { g.SetWeather(weather.Blizzard, 1) }, 14},
		{"deep snow", func() { g.SetWeather(weather.Clear, 0); g.weather.SnowDepth = 0.5 }, 12},
		{"shallow snow", func() { g.SetWeather(weather.Clear, 0); g.weather.SnowDepth = 0.2 }, 10},
	}
	for _, c := range cases {
		g.weather = weather.State{}
		c.set()
		if got := g.weatherSlow(out, 5, 10); got != c.wantO {
			t.Errorf("%s outdoors: %d, want %d", c.name, got, c.wantO)
		}
		if got := g.weatherSlow(in, 5, 10); got != 10 {
			t.Errorf("%s indoors: %d, want 10", c.name, got)
		}
	}
}

func TestOutdoorCampfireDousedInRain(t *testing.T) {
	g := weatherGame(t)
	jumpTo(g, 45, 1) // winter, cold enough that a fire burns
	g.BuildNow("campfire", "wood", "south", 5, 5)
	g.FuelCampfire(5, 5)
	hut(t, g, 10, 2, 16, 8)
	g.BuildNow("campfire", "wood", "south", 13, 4)
	g.FuelCampfire(13, 4)
	out := g.layerBldgAt(5, 5, construction.LayerObject)
	in := g.layerBldgAt(13, 4, construction.LayerObject)
	ob, _ := g.bldgs.Get(out)
	ib, _ := g.bldgs.Get(in)
	if !g.burning(out, ob) {
		t.Fatal("dry outdoor fire should burn")
	}
	g.SetWeather(weather.Rain, 0.2)
	if g.doused(out) {
		t.Error("light rain (<0.3) should not douse")
	}
	g.SetWeather(weather.Rain, 0.6)
	if !g.doused(out) || g.burning(out, ob) {
		t.Error("outdoor fire should be doused in rain")
	}
	if g.doused(in) || !g.burning(in, ib) {
		t.Error("indoor fire must keep burning in rain")
	}
	for _, k := range []weather.Kind{weather.Storm, weather.Snow, weather.Blizzard} {
		g.SetWeather(k, 0.5)
		if !g.doused(out) {
			t.Errorf("%v should douse", k)
		}
	}
	g.SetWeather(weather.Cloudy, 0.5)
	if g.doused(out) {
		t.Error("cloud should not douse")
	}
	g.SetWeather(weather.Clear, 0)
	if !g.burning(out, ob) || g.fuel[out] == 0 {
		t.Error("fire should resume after the rain with fuel kept")
	}
	if bid := entity.EntityID(0); g.doused(bid) {
		t.Error("unknown building cannot be doused")
	}
}

func TestDousedCampfireBurnsNoFuel(t *testing.T) {
	g := weatherGame(t)
	jumpTo(g, 45, 1)
	g.BuildNow("campfire", "wood", "south", 5, 5)
	g.FuelCampfire(5, 5)
	id := g.layerBldgAt(5, 5, construction.LayerObject)
	before := g.fuel[id]
	g.SetWeather(weather.Rain, 0.6)
	g.Step(1000)
	if g.fuel[id] != before {
		t.Errorf("doused fire burned fuel: %d -> %d", before, g.fuel[id])
	}
	g.SetWeather(weather.Clear, 0)
	g.Step(1000)
	if g.fuel[id] >= before {
		t.Errorf("fire should resume burning: %d", g.fuel[id])
	}
}

func TestSnowChillsOutdoorTemp(t *testing.T) {
	g := weatherGame(t)
	hut(t, g, 10, 2, 16, 8)
	g.ensureRooms()
	g.SetWeather(weather.Clear, 0)
	base, baseIn := g.temperatureAt(5, 5), g.temperatureAt(13, 5)
	g.SetWeather(weather.Snow, 0.5)
	if d := g.temperatureAt(5, 5) - base; d != -20 {
		t.Errorf("snow delta %d, want -20", d)
	}
	g.SetWeather(weather.Blizzard, 1)
	if d := g.temperatureAt(5, 5) - base; d != -50 {
		t.Errorf("blizzard delta %d, want -50", d)
	}
	if g.temperatureAt(13, 5) != baseIn {
		t.Error("indoor temperature must not change")
	}
	g.SetWeather(weather.Rain, 0.6)
	if g.temperatureAt(5, 5) != base {
		t.Error("rain should not change the air temperature")
	}
}

func TestSoakedAndSnowDayThoughts(t *testing.T) {
	g := weatherGame(t)
	hut(t, g, 10, 2, 16, 8)
	outside, _ := g.SpawnPawn(3, 3)
	inside, _ := g.SpawnPawn(13, 5)
	g.SetWeather(weather.Rain, 0.6)
	g.Step(250)
	if !remembers(g, outside, mood.Soaked) || remembers(g, inside, mood.Soaked) {
		t.Errorf("soaked: outside %v inside %v", g.memories[outside], g.memories[inside])
	}
	views := g.Snapshot()
	if !views[0].Soaked || views[1].Soaked {
		t.Errorf("PawnView.Soaked outside=%v inside=%v", views[0].Soaked, views[1].Soaked)
	}
	g.SetWeather(weather.Snow, 0.8) // heavy snow is no day off
	g.Step(250)
	if remembers(g, outside, mood.SnowDay) {
		t.Error("heavy snow should not give a snow day")
	}
	if g.Snapshot()[0].Soaked {
		t.Error("snow does not soak")
	}
	g.SetWeather(weather.Snow, 0.4)
	g.Step(250)
	if !remembers(g, outside, mood.SnowDay) || remembers(g, inside, mood.SnowDay) {
		t.Errorf("snow day: outside %v inside %v", g.memories[outside], g.memories[inside])
	}
	g.SetWeather(weather.Clear, 0)
	g.Step(250)
	if g.Snapshot()[0].Soaked {
		t.Error("dry weather should not soak")
	}
}

func TestPlantWiltedView(t *testing.T) {
	g := weatherGame(t)
	jumpTo(g, 10, 12) // warm spring noon
	corn, _ := g.SpawnCrop("corn", 3, 3, 40)
	check := func(label string, id entity.EntityID, want bool) {
		t.Helper()
		for _, pv := range g.World().Plants {
			if pv.ID == id && pv.Wilted != want {
				t.Errorf("%s: Wilted=%v, want %v", label, pv.Wilted, want)
			}
		}
	}
	check("warm spring", corn, false)
	jumpTo(g, 45, 5) // mid-winter pre-dawn: far below corn's frost limit
	check("winter cold", corn, true)
	hut(t, g, 10, 2, 16, 8)
	g.ensureRooms()
	jumpTo(g, 10, 12)
	indoor, _ := g.SpawnCrop("wheat", 13, 5, 40)
	check("sheltered", indoor, false)
}

func TestWeatherSaved(t *testing.T) {
	g := weatherGame(t)
	g.SetWeather(weather.Snow, 0.4)
	g.weather.SnowDepth, g.weather.WindDir, g.weather.WindSpeed = 0.35, 1.25, 1.5
	var buf bytes.Buffer
	if err := g.Save(&buf, "wx"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.Weather() != g.Weather() {
		t.Errorf("weather after load %+v, want %+v", h.Weather(), g.Weather())
	}
}

func TestWorldViewWeatherFields(t *testing.T) {
	g := weatherGame(t)
	g.SetWeather(weather.Rain, 0.6)
	g.weather.WindDir, g.weather.WindSpeed = 0, 2
	g.weather.SnowDepth, g.weather.Wetness = 0.25, 0.75
	w := g.World()
	if w.Weather != "rain" || w.WeatherIntensity != 0.6 || w.SnowDepth != 0.25 || w.Wetness != 0.75 {
		t.Errorf("view %q %v %v %v", w.Weather, w.WeatherIntensity, w.SnowDepth, w.Wetness)
	}
	if math.Abs(w.WindX-2) > 1e-9 || math.Abs(w.WindY) > 1e-9 {
		t.Errorf("wind %v,%v", w.WindX, w.WindY)
	}
	g.weather.WindDir = math.Pi / 2
	if w = g.World(); math.Abs(w.WindY-2) > 1e-9 {
		t.Errorf("wind %v,%v", w.WindX, w.WindY)
	}
}

func TestGroundBuildUpPerClimateInterval(t *testing.T) {
	g := weatherGame(t)
	jumpTo(g, 45, 1) // winter: snow does not melt
	g.SetWeather(weather.Rain, 0.6)
	g.Step(500)
	if g.Weather().Wetness <= 0 {
		t.Error("rain should wet the ground")
	}
	g.SetWeather(weather.Snow, 0.5)
	g.Step(500)
	if g.Weather().SnowDepth <= 0 {
		t.Error("snow should pile up")
	}
}
