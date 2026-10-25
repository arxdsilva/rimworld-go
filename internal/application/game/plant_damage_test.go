package game

import (
	"testing"

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
	if g.residual == nil {
		g.residual = map[int]temperature.Temp{}
	}
	g.residual[y*g.grid.Width()+x] = temperature.Temp(tenths)
}

// frostFor runs the whole-map frost pass once a climate interval for
// ticks, the clock moving on as it goes.
func frostFor(g *Game, ticks int) {
	for i := 0; i < ticks/climateInterval; i++ {
		g.clock.Advance(climateInterval)
		g.frostCrops()
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

// nearDeath leaves plant id one window from dying of frost.
func nearDeath(g *Game, id entity.EntityID) {
	p, _ := g.plants.Get(id)
	p.Damage, p.Damaging = plant.DamageDead-1, true
	g.plants.Set(id, p)
}

// TestFrostDamagesCropsOverHours: corn just past its frost limit is hurt
// for about four hours (one warning, a damage readout) and then dies with
// the batched kill notice.
func TestFrostDamagesCropsOverHours(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 12) // summer: only the pinned tiles freeze
	g.SetWeather(weather.Clear, 0)
	kill := plant.DefOf(plant.Corn).FrostKill
	var ids []entity.EntityID
	for x := 2; x < 4; x++ {
		id, _ := g.spawnPlant(plant.Corn, x, 3, 100)
		setAir(g, x, 3, kill-1)
		ids = append(ids, id)
	}
	warm, _ := g.spawnPlant(plant.Corn, 10, 3, 100)
	frostFor(g, clock.TicksPerHour)
	for _, id := range ids {
		p, ok := g.plants.Get(id)
		if !ok || p.Damage < 24 || p.Damage > 26 || !p.Damaging || p.DamageHeat {
			t.Fatalf("an hour past the frost limit: %+v alive %v, want about 25%% cold damage", p, ok)
		}
		pv, _ := plantView(g, id)
		if pv.Damage != int(p.Damage) || pv.DamageKind != "cold" {
			t.Fatalf("view %d%% %q, plant %d%%", pv.Damage, pv.DamageKind, p.Damage)
		}
	}
	if pv, _ := plantView(g, warm); pv.Damage != 0 || pv.DamageKind != "" {
		t.Fatalf("warm corn reads %d%% %q", pv.Damage, pv.DamageKind)
	}
	if n := countText(g, "Frost is damaging the corn"); n != 1 {
		t.Fatalf("%d frost warnings, want 1: %+v", n, g.messages)
	}
	frostFor(g, 3*clock.TicksPerHour-climateInterval)
	if _, ok := g.plants.Get(ids[0]); !ok {
		t.Fatal("corn died before four hours just past its limit")
	}
	frostFor(g, climateInterval)
	for _, id := range ids {
		if _, ok := g.plants.Get(id); ok {
			t.Fatal("corn alive after four hours past its frost limit")
		}
	}
	if m := g.messages[len(g.messages)-1].Text; m != "Frost killed the corn (2 plants)" {
		t.Fatalf("kill notice %q", m)
	}
	if n := countText(g, "Frost is damaging the corn"); n != 1 {
		t.Fatalf("%d frost warnings, want 1", n)
	}
	for _, id := range g.plantIDs {
		if _, ok := g.plants.Get(id); !ok {
			t.Fatal("a dead plant is still listed")
		}
	}
}

func TestHeatScorchesWildPlants(t *testing.T) {
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

// TestFrostDamageHealsAndWarnsOncePerDay: damage below 60% heals once
// the air is safe again, a second frost the same day posts no second
// warning, and a frost the next day warns again.
func TestFrostDamageHealsAndWarnsOncePerDay(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 1)
	g.SetWeather(weather.Clear, 0)
	kill := plant.DefOf(plant.Corn).FrostKill
	id, _ := g.spawnPlant(plant.Corn, 3, 3, 100)
	const warning = "Frost is damaging the corn"
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

// TestFrostDamageNoFlap: air hovering at the frost limit keeps the crop
// damaging from the first frosty window until it dies: one warning,
// damage never falling.
func TestFrostDamageNoFlap(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	bareGround(g)
	jumpTo(g, 10, 1)
	kill := plant.DefOf(plant.Corn).FrostKill
	id, _ := g.spawnPlant(plant.Corn, 3, 3, 100)
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
		t.Fatal("corn hovering at its frost limit never died")
	}
	if n := countText(g, "Frost is damaging the corn"); n != 1 {
		t.Fatalf("%d warnings, want 1", n)
	}
}

// TestStaggeredFrostKillsOverHours: in a real temperate midwinter night
// the chunked plant pass hurts corn everywhere on the map, and kills it
// within a few hours rather than at once.
func TestStaggeredFrostKillsOverHours(t *testing.T) {
	g := New(Config{Seed: 1, Width: 64, Height: 64})
	bareGround(g)
	jumpTo(g, 45, 0)
	g.SetWeather(weather.Clear, 0)
	a, _ := g.spawnPlant(plant.Corn, 1, 1, 100)
	b, _ := g.spawnPlant(plant.Corn, 60, 60, 100)
	g.Step(clock.TicksPerHour / 2)
	for _, id := range []entity.EntityID{a, b} {
		p, ok := g.plants.Get(id)
		if !ok || p.Damage == 0 || p.Dead() {
			t.Fatalf("half an hour of a midwinter night: %+v alive %v", p, ok)
		}
	}
	g.Step(3 * clock.TicksPerHour)
	for _, id := range []entity.EntityID{a, b} {
		if _, ok := g.plants.Get(id); ok {
			t.Fatal("corn alive after three and a half hours of a midwinter night")
		}
	}
}

func TestDamagedHarvestYieldsLess(t *testing.T) {
	g := dirtGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	pid, _ := g.spawnPlant(plant.Wheat, 6, 5, plant.DefOf(plant.Wheat).GrowTicks)
	p, _ := g.plants.Get(pid)
	p.Damage = 30
	g.plants.Set(pid, p)
	g.doHarvest(id, job.Record{Kind: job.Harvest, State: job.Do, Target: pid, TX: 6, TY: 5, WorkLeft: 1})
	d := plant.DefOf(plant.Wheat)
	if w, ok := stackOf(g, item.Wheat); !ok || w.Qty != d.Yield*70/100 {
		t.Fatalf("wheat at 30%% damage = %+v %v, want %d", w, ok, d.Yield*70/100)
	}
	if s, ok := stackOf(g, item.Straw); !ok || s.Qty != d.ExtraQty*70/100 {
		t.Fatalf("straw at 30%% damage = %+v %v, want %d", s, ok, d.ExtraQty*70/100)
	}
	// a ruined crop yields nothing (and drops no empty stack)
	h := dirtGame(t)
	hid, _ := h.SpawnNamedPawn(5, 5, "Ada")
	cid, _ := h.spawnPlant(plant.Corn, 6, 5, plant.DefOf(plant.Corn).GrowTicks)
	c, _ := h.plants.Get(cid)
	c.Damage = 99
	h.plants.Set(cid, c)
	h.doHarvest(hid, job.Record{Kind: job.Harvest, State: job.Do, Target: cid, TX: 6, TY: 5, WorkLeft: 1})
	if s, ok := stackOf(h, item.Corn); ok {
		t.Fatalf("99%% damaged corn yielded %+v", s)
	}
}

func TestPlantDamageSaveRoundTrip(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	cold, _ := g.spawnPlant(plant.Corn, 3, 3, 100)
	hot, _ := g.spawnPlant(plant.Pine, 5, 3, 100)
	for _, c := range []struct {
		id   entity.EntityID
		dmg  uint8
		heat bool
	}{{cold, 42, false}, {hot, 63, true}} {
		p, _ := g.plants.Get(c.id)
		p.Damage, p.DamageHeat, p.Damaging = c.dmg, c.heat, true
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
