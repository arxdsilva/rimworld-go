package game

import (
	"testing"

	"rimworld-go/internal/application/game/farm"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/weather"
)

// setAir pins the air on (x, y) to tenths of a degree (the residual-heat
// hook temperatureAt reads first; frostCrops never moves it).
func setAir(g *Game, x, y, tenths int) {
	if g.thermo.Residual == nil {
		g.thermo.Residual = map[int]temperature.Temp{}
	}
	g.thermo.Residual[y*g.grid.Width()+x] = temperature.Temp(tenths)
}

// frostFor runs the whole-map frost pass once a climate interval for
// ticks, the clock moving on as it goes.
func frostFor(g *Game, ticks int) {
	for i := 0; i < ticks/climateInterval; i++ {
		g.clock.Advance(climateInterval)
		frostCrops(g)
	}
}

// countText is how many recent messages read text.
func countText(g *Game, text string) int {
	n := 0
	for _, m := range g.messages {
		if m.Text == text {
			n++
		}
	}
	return n
}

func plantView(g *Game, id entity.EntityID) (PlantView, bool) {
	for _, pv := range g.World().Plants {
		if pv.ID == id {
			return pv, true
		}
	}
	return PlantView{}, false
}

// frostLimit is plant id's own frost limit (FOOD-2, plant.ColdLimit).
func frostLimit(g *Game, id entity.EntityID) int {
	p, _ := g.plants.Get(id)
	lim, _ := p.ColdLimit()
	return lim
}

// nearDeath leaves plant id one window from dying of frost.
func nearDeath(g *Game, id entity.EntityID) {
	p, _ := g.plants.Get(id)
	p.Damage, p.Damaging = plant.DamageDead-1, true
	g.plants.Set(id, p)
}

// TestColdKillsCropsAtOnce (TEMP-2, wiki: Plants): corn just below its
// own cold threshold dies in the first window, with the batched kill
// notice and no damage warning; corn at its threshold lives.
func TestColdKillsCropsAtOnce(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 12) // summer: only the pinned tiles freeze
	g.SetWeather(weather.Clear, 0)
	var ids []entity.EntityID
	for x := 2; x < 4; x++ {
		id, _ := g.spawnPlant(plant.Corn, x, 3, 100)
		setAir(g, x, 3, plant.ColdThreshold(uint64(id))-1) // each plant its own threshold, from its id
		ids = append(ids, id)
	}
	edge, _ := g.spawnPlant(plant.Corn, 10, 3, 100)
	setAir(g, 10, 3, plant.ColdThreshold(uint64(edge)))
	frostFor(g, climateInterval)
	for _, id := range ids {
		if _, ok := g.plants.Get(id); ok {
			t.Fatal("corn alive a window below its cold threshold")
		}
	}
	if p, ok := g.plants.Get(edge); !ok || p.Damage != 0 {
		t.Fatalf("corn at its threshold: %+v alive %v", p, ok)
	}
	if m := g.messages[len(g.messages)-1].Text; m != "Frost killed the corn (2 plants)" {
		t.Fatalf("kill notice %q", m)
	}
	if n := countText(g, "Frost is damaging the corn"); n != 0 {
		t.Fatalf("%d frost warnings, want none", n)
	}
	for _, id := range g.plantIDs {
		if _, ok := g.plants.Get(id); !ok {
			t.Fatal("a dead plant is still listed")
		}
	}
}

func TestHeatScorchesWildPlants(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	pine, _ := g.spawnPlant(plant.Pine, 3, 3, 1000)
	setAir(g, 3, 3, plant.DefOf(plant.Pine).HeatKill+1)
	frostFor(g, clock.TicksPerHour)
	pv, _ := plantView(g, pine)
	if pv.DamageKind != "heat" || pv.Damage < 24 || pv.Damage > 26 {
		t.Fatalf("pine an hour in the heat: %d%% %q", pv.Damage, pv.DamageKind)
	}
	if countText(g, "Heat is scorching the pine plants") != 1 {
		t.Fatalf("no heat warning: %+v", g.messages)
	}
	frostFor(g, 3*clock.TicksPerHour)
	if _, ok := g.plants.Get(pine); ok {
		t.Fatal("pine alive after four hours past its heat limit")
	}
	if m := g.messages[len(g.messages)-1].Text; m != "The heat killed a pine" {
		t.Fatalf("kill notice %q", m)
	}
}

// TestFrostDamageHealsAndWarnsOncePerDay: a wild plant's cold damage
// (a jungle tree below its -8 C limit; crops die at once since TEMP-2)
// below 60% heals once the air is safe again, a second frost the same
// day posts no second warning, and a frost the next day warns again.
func TestFrostDamageHealsAndWarnsOncePerDay(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 1)
	g.SetWeather(weather.Clear, 0)
	id, _ := g.spawnPlant(plant.JungleTree, 3, 3, 100)
	kill := frostLimit(g, id)
	warning := farm.HurtText(plant.JungleTree, false)
	setAir(g, 3, 3, kill-1)
	frostFor(g, 6*clock.TicksPerHour/4) // 1.5 h: about 37%
	setAir(g, 3, 3, kill+plant.DamageStopMargin)
	frostFor(g, clock.TicksPerHour)
	p, _ := g.plants.Get(id)
	if p.Damaging || p.Damage < 26 || p.Damage > 28 {
		t.Fatalf("an hour of safe air after 37%%: %+v, want about 27%% and healing", p)
	}
	frostFor(g, 3*clock.TicksPerHour)
	if p, _ = g.plants.Get(id); p.Damage != 0 {
		t.Fatalf("not healed: %+v", p)
	}
	setAir(g, 3, 3, kill-1) // a second frost the same day
	frostFor(g, clock.TicksPerHour/2)
	if n := countText(g, warning); n != 1 {
		t.Fatalf("%d warnings on one day, want 1", n)
	}
	setAir(g, 3, 3, kill+plant.DamageStopMargin)
	frostFor(g, 2*clock.TicksPerHour)
	g.clock.Advance(clock.TicksPerDay) // the next day
	setAir(g, 3, 3, kill-1)
	frostFor(g, climateInterval)
	if n := countText(g, warning); n != 2 {
		t.Fatalf("%d warnings over two days, want 2", n)
	}
	// from 60% on it keeps dying in safe air
	p, _ = g.plants.Get(id)
	p.Damage = plant.DamageDying
	g.plants.Set(id, p)
	setAir(g, 3, 3, 200)
	frostFor(g, 2*clock.TicksPerHour)
	if _, ok := g.plants.Get(id); ok {
		t.Fatal("a plant past 60% recovered in warm air")
	}
}

// TestFrostDamageNoFlap: air hovering at a wild plant's cold limit keeps
// it damaging from the first frosty window until it dies: one warning,
// damage never falling.
func TestFrostDamageNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 1)
	id, _ := g.spawnPlant(plant.JungleTree, 3, 3, 100)
	kill := frostLimit(g, id)
	last := uint8(0)
	for i := 0; i < 60; i++ {
		setAir(g, 3, 3, kill-1+2*(i%2)) // 0.1 °C either side of the limit
		frostFor(g, climateInterval)
		p, ok := g.plants.Get(id)
		if !ok {
			break
		}
		if !p.Damaging || p.Damage < last {
			t.Fatalf("window %d: %+v after %d%%", i, p, last)
		}
		last = p.Damage
	}
	if _, ok := g.plants.Get(id); ok {
		t.Fatal("a jungle tree hovering at its cold limit never died")
	}
	if n := countText(g, farm.HurtText(plant.JungleTree, false)); n != 1 {
		t.Fatalf("%d warnings, want 1", n)
	}
}

// TestStaggeredFrostKillsCrops: a real temperate midwinter night (TEMP-2:
// down to -25 C, wiki: Biomes) is past every crop's cold threshold, and
// the chunked plant pass kills corn everywhere on the map within one
// climate interval.
func TestStaggeredFrostKillsCrops(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 64, Height: 64})
	bareGround(g)
	jumpTo(g, 45, 2)
	g.SetWeather(weather.Clear, 0)
	if out := g.climate.Outdoor(g.clock.Now()); out >= -180 {
		t.Fatalf("midwinter night at %v", out)
	}
	a, _ := g.spawnPlant(plant.Corn, 1, 1, 100)
	b, _ := g.spawnPlant(plant.Corn, 60, 60, 100)
	g.Step(climateInterval)
	for _, id := range []entity.EntityID{a, b} {
		if _, ok := g.plants.Get(id); ok {
			t.Fatal("corn alive after a climate interval of a midwinter night")
		}
	}
}

func TestDamagedHarvestYieldsLess(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	setSkill(g, id, skPlants, 8) // HRV-005, HRV-009: full speed and yield, no waste roll
	pid, _ := g.spawnPlant(plant.Wheat, 6, 5, plant.DefOf(plant.Wheat).GrowTicks)
	p, _ := g.plants.Get(pid)
	p.Damage = 30
	g.plants.Set(pid, p)
	g.doHarvest(id, job.Record{Kind: job.Harvest, State: job.Do, Target: pid, TX: 6, TY: 5, WorkLeft: 1})
	d := plant.DefOf(plant.Wheat)
	if w, ok := stackOf(g, item.Wheat); !ok || w.Qty != d.Yield*85/100 { // HRV-004: half the damage is lost
		t.Fatalf("wheat at 30%% damage = %+v %v, want %d", w, ok, d.Yield*85/100)
	}
	if s, ok := stackOf(g, item.Straw); !ok || s.Qty != d.ExtraQty*85/100 {
		t.Fatalf("straw at 30%% damage = %+v %v, want %d", s, ok, d.ExtraQty*85/100)
	}
	// a nearly dead crop loses just about half; a blighted one yields nothing
	// (and drops no empty stack)
	h := dirtGame(t)
	hid, _ := h.SpawnNamedPawn(5, 5, "Ada")
	setSkill(h, hid, skPlants, 8)
	cid, _ := h.spawnPlant(plant.Corn, 6, 5, plant.DefOf(plant.Corn).GrowTicks)
	c, _ := h.plants.Get(cid)
	c.Damage = 99
	h.plants.Set(cid, c)
	h.doHarvest(hid, job.Record{Kind: job.Harvest, State: job.Do, Target: cid, TX: 6, TY: 5, WorkLeft: 1})
	if s, ok := stackOf(h, item.Corn); !ok || s.Qty != plant.DefOf(plant.Corn).Yield*101/200 {
		t.Fatalf("99%% damaged corn yielded %+v, want %d", s, plant.DefOf(plant.Corn).Yield*101/200)
	}
	b := dirtGame(t)
	bid, _ := b.SpawnNamedPawn(5, 5, "Ada")
	setSkill(b, bid, skPlants, 8)
	cid, _ = b.spawnPlant(plant.Corn, 6, 5, plant.DefOf(plant.Corn).GrowTicks)
	c, _ = b.plants.Get(cid)
	c.Blight = 30
	b.plants.Set(cid, c)
	b.doHarvest(bid, job.Record{Kind: job.Harvest, State: job.Do, Target: cid, TX: 6, TY: 5, WorkLeft: 1})
	if s, ok := stackOf(b, item.Corn); ok {
		t.Fatalf("blighted corn yielded %+v", s)
	}
}

func TestPlantDamageSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	cold, _ := g.spawnPlant(plant.Corn, 3, 3, 100)
	hot, _ := g.spawnPlant(plant.Pine, 5, 3, 100)
	for _, c := range []struct {
		id   entity.EntityID
		dmg  uint8
		heat bool
	}{{cold, 42, false}, {hot, 63, true}} {
		p, _ := g.plants.Get(c.id)
		p.Damage, p.DamageHeat, p.Damaging, p.Bare = c.dmg, c.heat, true, 77777 // TEMP-2: leafless until
		g.plants.Set(c.id, p)
	}
	h := saveLoad(t, g)
	for _, id := range []entity.EntityID{cold, hot} {
		want, _ := g.plants.Get(id)
		got, ok := h.plants.Get(id)
		if !ok || got != want {
			t.Fatalf("plant %d: saved %+v, loaded %+v", id, want, got)
		}
	}
	if pv, _ := plantView(h, hot); pv.Damage != 63 || pv.DamageKind != "heat" {
		t.Fatalf("loaded view %d%% %q", pv.Damage, pv.DamageKind)
	}
}
