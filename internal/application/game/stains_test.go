package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

func stainGame(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 3, Width: 30, Height: 14})
	jumpTo(g, 10, 1)
	g.SetWeather(weather.Clear, 0)
	return g
}

func stainAt(g *Game, x, y int) (stain.Stain, bool) {
	s, ok := g.stains.tiles[[2]int{x, y}]
	return s, ok
}

// hurtWound gives id one open cut of dmg on the torso in place of its
// wounds (MED-01: an open cut bleeds; a bruise would not).
func hurtWound(t *testing.T, g *Game, id entity.EntityID, dmg int) *body.Body {
	t.Helper()
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	b.Wounds = []body.Wound{{Part: body.Torso, Dmg: dmg, Kind: body.Cut}}
	g.bodyChanged(id, b, &v)
	g.woundOpened(id)
	return b
}

// dripSteps walks id over three distinct tiles and returns the stains.
func dripSteps(g *Game, id entity.EntityID) {
	for x := 10; x < 13; x++ {
		g.stainStep(id, Position{X: x, Y: 3})
	}
}

func TestHitStampsTargetTile(t *testing.T) {
	g := stainGame(t)
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.strikeUntilHit(t, a, b, weapon.Fists)
	s, ok := stainAt(g, 6, 5)
	if !ok || s.Kind != stain.Blood || s.Amount < hitStain || s.Born != uint64(g.clock.Now()) {
		t.Fatalf("stain on target tile = %+v, %v", s, ok)
	}
	g.stainHit(b, Position{5, 5}, Position{9, 9}, 2)
	if s, _ := stainAt(g, 9, 9); s.Amount != hitStain+2*hitStainPerDmg || s.Variant >= 6 {
		t.Fatalf("a 2 damage hit = %+v, want %d", s, hitStain+2*hitStainPerDmg)
	}
	if _, ok := stainAt(g, 10, 10); ok {
		t.Fatal("a light hit must not spray")
	}
}

func TestHeavyHitSpraysBehindAwayFromAttacker(t *testing.T) {
	g := stainGame(t)
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.stainHit(b, Position{5, 5}, Position{6, 5}, 10) // attacker to the west
	if s, _ := stainAt(g, 6, 5); s.Amount != hitStain+10*hitStainPerDmg {
		t.Fatalf("target tile = %d, want %d", s.Amount, hitStain+10*hitStainPerDmg)
	}
	if s, ok := stainAt(g, 7, 5); !ok || s.Amount != (hitStain+10*hitStainPerDmg)*spraySharePct/100 {
		t.Fatalf("spray = %+v, want 40%% of the hit at 7,5", s)
	}
	g.stainHit(b, Position{8, 8}, Position{7, 7}, 6) // attacker to the south-east
	if _, ok := stainAt(g, 6, 6); !ok {
		t.Fatal("diagonal spray lands at 6,6")
	}
	g.stainHit(b, Position{4, 4}, Position{4, 4}, 9) // same tile: no direction
	if len(g.stains.tiles) != 5 {
		t.Fatalf("%d stains, want 5", len(g.stains.tiles))
	}
	// spray into rock or off the map is dropped
	g.setTerrain(21, 5, terrain.Rock)
	g.stainHit(b, Position{19, 5}, Position{20, 5}, 10)
	g.stainHit(b, Position{1, 1}, Position{0, 0}, 10)
	g.stainHit(b, Position{2, 2}, Position{0, 0}, 10)
	if _, ok := stainAt(g, 21, 5); ok {
		t.Fatal("rock stays clean")
	}
	if _, ok := stainAt(g, -1, -1); ok {
		t.Fatal("off the map stays clean")
	}
}

func TestMissLeavesNoStain(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 12})
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	for i := 0; i < 400; i++ {
		g.combat.Strikes.Strike(a, b, weapon.Fists)
		if !g.traces[len(g.traces)-1].Hit {
			if len(g.stains.tiles) != 0 {
				t.Fatalf("a miss left %d stains", len(g.stains.tiles))
			}
			return
		}
		g.stains = stainState{}
	}
	t.Fatal("never missed")
}

func TestDeathLeavesPool(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnNamedPawn(7, 7, "Ada")
	g.stainHit(id, Position{6, 7}, Position{7, 7}, 1) // a light hit already there
	g.die(id, health.CauseInjury)
	if s, ok := stainAt(g, 7, 7); !ok || s.Amount != hitStain+hitStainPerDmg+deathPool {
		t.Fatalf("pool = %+v, want the hit plus the death pool", s)
	}
	g.stainDeath(entity.EntityID(9999)) // gone: nothing happens
	if len(g.stains.tiles) != 1 {
		t.Fatal("unknown creature stained")
	}
}

func TestAnimalDeathPool(t *testing.T) {
	g := stainGame(t)
	id, err := g.SpawnAnimal("deer", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.killAnimal(id, 0)
	if s, ok := stainAt(g, 8, 8); !ok || s.Amount != deathPool || s.Kind != stain.Blood {
		t.Fatalf("deer pool = %+v", s)
	}
}

func TestInsectIchor(t *testing.T) {
	g := stainGame(t)
	id, err := g.SpawnAnimal("hive_beetle", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if g.stainKindOf(id) != stain.Ichor {
		t.Fatal("beetle bleeds ichor")
	}
	g.stainHit(id, Position{7, 8}, Position{8, 8}, 2)
	if s, _ := stainAt(g, 8, 8); s.Kind != stain.Ichor {
		t.Fatalf("hit on a beetle = %+v", s)
	}
	g.killAnimal(id, 0)
	if s, _ := stainAt(g, 8, 8); s.Kind != stain.Ichor || s.Amount != hitStain+2*hitStainPerDmg+deathPool {
		t.Fatalf("beetle pool = %+v", s)
	}
	if v := g.World().Stains; len(v) != 1 || v[0].Kind != "ichor" {
		t.Fatalf("view = %+v", v)
	}
}

func TestBleedingColonistDripsEvery3Tiles(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.stainStep(id, Position{9, 3}) // healthy: no count, no drip
	if len(g.stains.tiles) != 0 || len(g.stains.dripN) != 0 {
		t.Fatal("a healthy colonist dripped")
	}
	b := hurtWound(t, g, id, 20)
	g.stainStep(id, Position{10, 3})
	g.stainStep(id, Position{11, 3})
	if len(g.stains.tiles) != 0 {
		t.Fatal("dripped before the 3rd tile")
	}
	g.stainStep(id, Position{12, 3})
	if s, ok := stainAt(g, 12, 3); !ok || s.Amount != dripStain || len(g.stains.tiles) != 1 {
		t.Fatalf("3rd tile = %+v %v", s, ok)
	}
	// a tended wound (MED-01) no longer bleeds, and resets the count
	g.stainStep(id, Position{13, 3})
	b.TendAll(0, 0)
	g.stainStep(id, Position{14, 3})
	if g.stains.dripN[id] != 0 {
		t.Fatalf("count not reset: %v", g.stains.dripN)
	}
	// a wound that was never opened (no record) does not bleed either
	other, _ := g.SpawnNamedPawn(6, 5, "Bo")
	v, _ := g.vital.Get(other)
	v.Injury = 50
	g.vital.Set(other, v)
	if g.bleeding(other) {
		t.Fatal("no open wound, no bleeding")
	}
	// a dead colonist does not drip
	v.Status = health.Dead
	g.vital.Set(other, v)
	g.woundOpened(other)
	if g.bleeding(other) {
		t.Fatal("dead")
	}
	if g.bleeding(entity.EntityID(9999)) {
		t.Fatal("unknown id")
	}
}

func TestTendedColonistStopsDripping(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b := hurtWound(t, g, id, 25)
	g.stainStep(id, Position{9, 3})
	g.stainStep(id, Position{10, 3})
	b.TendAll(0, 0) // a tend stops the bleeding (MED-01)
	g.stainStep(id, Position{11, 3})
	if len(g.stains.tiles) != 0 || g.stains.dripN[id] != 0 {
		t.Fatalf("tended colonist dripped: %v %v", g.stains.tiles, g.stains.dripN)
	}
}

// TestRaiderBleedsFromAnOpenCut (MED-01; was TestRaiderBleedsAtThirty): a
// raider drips by its body, like a colonist, not past an injury threshold.
func TestRaiderBleedsFromAnOpenCut(t *testing.T) {
	g := New(Config{Seed: 6, Width: 30, Height: 12})
	g.SpawnNamedPawn(5, 5, "Ada")
	g.Raid(1)
	if len(g.raiderIDs) == 0 {
		t.Fatal("no raider")
	}
	r := g.raiderIDs[0]
	v, _ := g.vital.Get(r)
	v.Injury = 40
	g.vital.Set(r, v)
	if g.bleeding(r) {
		t.Fatal("injury with no body (a bruise at most) does not bleed")
	}
	hurtWound(t, g, r, 5)
	if !g.bleeding(r) {
		t.Fatal("an open cut bleeds")
	}
	g.stainStep(r, Position{1, 1})
	g.stainStep(r, Position{2, 1})
	g.stainStep(r, Position{3, 1})
	if _, ok := stainAt(g, 3, 1); !ok {
		t.Fatal("raider dripped nothing")
	}
}

func TestHurtAnimalDrips(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnAnimal("deer", 8, 8)
	a, _ := g.animals.Get(id)
	max := animal.DefOf(a.Species).MaxHP
	a.Injury = max / 2 // exactly half: still healthy enough
	g.animals.Set(id, a)
	dripSteps(g, id)
	if len(g.stains.tiles) != 0 {
		t.Fatal("half health is not bleeding")
	}
	a.Injury = max*6/10 + 1
	g.animals.Set(id, a)
	dripSteps(g, id)
	if s, ok := stainAt(g, 12, 3); !ok || s.Amount != dripStain {
		t.Fatalf("hurt deer drip = %+v", s)
	}
}

func TestDownedPools(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	hurtWound(t, g, id, 10) // bleeding (MED-01: only a bleeding body pools)
	v, _ := g.vital.Get(id)
	v.Status = health.Downed
	g.vital.Set(id, v)
	now := uint64(g.clock.Now())
	hour := now - now%clock.TicksPerHour
	g.stainDowned(id, hour+1) // not on the hour
	if len(g.stains.tiles) != 0 {
		t.Fatal("pooled off the hour")
	}
	for i := 0; i < 12; i++ {
		g.stainDowned(id, hour+uint64(i)*clock.TicksPerHour)
	}
	if s, _ := stainAt(g, 5, 5); s.Amount != downedPoolMax {
		t.Fatalf("pool = %d, want %d", s.Amount, downedPoolMax)
	}
	// a tended body does not pool
	g2 := stainGame(t)
	id2, _ := g2.SpawnNamedPawn(5, 5, "Ada")
	b2 := hurtWound(t, g2, id2, 10)
	b2.TendAll(0, 0)
	g2.stainDowned(id2, hour)
	if len(g2.stains.tiles) != 0 {
		t.Fatal("tended body pooled")
	}
	hurtWound(t, g2, id2, 10)
	g2.stainDowned(entity.EntityID(9999), hour)
	if len(g2.stains.tiles) != 0 {
		t.Fatal("unknown id pooled")
	}
	g2.stainHit(id2, Position{1, 1}, Position{5, 5}, 25) // 665: already over the limit
	g2.stainDowned(id2, hour)
	if s, _ := stainAt(g2, 5, 5); s.Amount != hitStain+25*hitStainPerDmg {
		t.Fatalf("over-limit pool grew to %d", s.Amount)
	}
}

func TestStainFadesSmoothlyOver7Days(t *testing.T) {
	g := stainGame(t)
	g.stamp(4, 4, stain.Blood, stain.Full)
	steps := stain.FadeTicks / climateInterval
	prev := uint16(stain.Full)
	for i := 1; i <= steps; i++ {
		g.stainDecay()
		s, ok := stainAt(g, 4, 4)
		if i == steps/2 && s.Amount != 500 {
			t.Fatalf("half way = %d", s.Amount)
		}
		if i == steps {
			if ok || len(g.stains.frac) != 0 {
				t.Fatalf("not gone after 7 days: %+v", s)
			}
			break
		}
		if !ok || s.Amount > prev {
			t.Fatalf("step %d: %+v not monotone (prev %d)", i, s, prev)
		}
		prev = s.Amount
	}
}

// rainSteps runs stainDecay n times and returns the stain on (x, y).
func rainSteps(g *Game, n, x, y int) (stain.Stain, bool) {
	for i := 0; i < n; i++ {
		g.stainDecay()
	}
	return stainAt(g, x, y)
}

func TestRainWashesOutdoors2h(t *testing.T) {
	g := stainGame(t)
	g.SetWeather(weather.Rain, 1)
	g.stamp(20, 5, stain.Blood, stain.Full)
	if !g.underSky(20, 5) {
		t.Fatal("not open ground")
	}
	n := stain.RainWash / climateInterval
	if s, ok := rainSteps(g, n-1, 20, 5); !ok || s.Amount == 0 {
		t.Fatalf("gone too early: %+v", s)
	}
	if _, ok := rainSteps(g, 1, 20, 5); ok {
		t.Fatal("still there after 2 h of rain")
	}
}

func TestSnowWashes3h(t *testing.T) {
	g := stainGame(t)
	g.SetWeather(weather.Snow, 1)
	g.stamp(20, 5, stain.Blood, stain.Full)
	n := stain.SnowWash / climateInterval
	if _, ok := rainSteps(g, n-1, 20, 5); !ok {
		t.Fatal("gone too early")
	}
	if _, ok := rainSteps(g, 1, 20, 5); ok {
		t.Fatal("still there after 3 h of snow")
	}
}

func TestRoofedStainOnlyFades(t *testing.T) {
	g := stainGame(t)
	hut(t, g, 2, 2, 8, 8)
	if g.underSky(5, 5) {
		t.Fatal("hut should be roofed")
	}
	g.SetWeather(weather.Storm, 1)
	g.stamp(5, 5, stain.Blood, stain.Full)
	var f int
	want := stain.Full
	for i := 0; i < 40; i++ {
		want = int(stain.Fade(uint16(want), climateInterval, &f))
	}
	s, ok := rainSteps(g, 40, 5, 5)
	if !ok || int(s.Amount) != want {
		t.Fatalf("roofed stain = %d, want fade only %d", s.Amount, want)
	}
}

func TestStampIgnoresRockAndOutOfBounds(t *testing.T) {
	g := stainGame(t)
	g.setTerrain(3, 3, terrain.Rock)
	g.stamp(3, 3, stain.Blood, 500)
	g.stamp(-1, 3, stain.Blood, 500)
	g.stamp(3, 99, stain.Blood, 500)
	g.stamp(4, 4, stain.Blood, 0)
	if len(g.stains.tiles) != 0 {
		t.Fatalf("stained %v", g.stains.tiles)
	}
	g.stamp(4, 4, stain.Blood, 500)
	g.stamp(4, 4, stain.Ichor, 700)
	if s, _ := stainAt(g, 4, 4); s.Amount != stain.Full || s.Kind != stain.Ichor {
		t.Fatalf("merge = %+v", s)
	}
}

// fillStains stamps tiles from..to-1 in row order with amount a.
func fillStains(g *Game, from, to int, a int) {
	for i := from; i < to; i++ {
		g.stamp(i%g.Width(), i/g.Width(), stain.Blood, a)
	}
}

func TestCapDropsFaintest(t *testing.T) {
	g := New(Config{Seed: 3, Width: 60, Height: 40})
	g.stamp(0, 0, stain.Blood, 10)
	g.stamp(1, 0, stain.Blood, 500)
	fillStains(g, 2, stain.Cap, 500)
	if len(g.stains.tiles) != stain.Cap {
		t.Fatalf("%d stains, want %d", len(g.stains.tiles), stain.Cap)
	}
	g.stamp(59, 39, stain.Blood, 500)
	if len(g.stains.tiles) != stain.Cap {
		t.Fatalf("%d stains after the cap, want %d", len(g.stains.tiles), stain.Cap)
	}
	if _, ok := stainAt(g, 0, 0); ok {
		t.Fatal("the faintest stain survived")
	}

	// equal strength: the oldest goes, then the topmost, then the leftmost
	g = New(Config{Seed: 3, Width: 60, Height: 40})
	g.stamp(5, 0, stain.Blood, 500) // oldest
	g.clock.Advance(1)
	fillStains(g, 0, 5, 500)
	fillStains(g, 6, stain.Cap, 500) // (5,0) keeps its old Born
	g.stamp(59, 39, stain.Blood, 500)
	if _, ok := stainAt(g, 5, 0); ok {
		t.Fatal("oldest should go first")
	}
	if _, ok := stainAt(g, 0, 0); !ok {
		t.Fatal("(0,0) is newer and stays")
	}
	g = New(Config{Seed: 3, Width: 60, Height: 40})
	fillStains(g, 0, stain.Cap, 500) // all equally old and strong
	g.stamp(59, 39, stain.Blood, 500)
	if _, ok := stainAt(g, 0, 0); ok {
		t.Fatal("tie: topmost leftmost goes")
	}
	// same row: leftmost goes; same Born and amount, y equal
	a := stain.Stain{X: 1, Y: 2, Amount: 5}
	b := stain.Stain{X: 3, Y: 2, Amount: 5}
	if !stainWeaker(a, b) || stainWeaker(b, a) {
		t.Fatal("x breaks the tie")
	}
	c := stain.Stain{X: 9, Y: 1, Amount: 5}
	if !stainWeaker(c, a) {
		t.Fatal("y breaks the tie before x")
	}
}

func TestStainViewSorted(t *testing.T) {
	g := stainGame(t)
	g.stamp(5, 3, stain.Blood, 500)
	g.stamp(2, 3, stain.Ichor, 1000)
	g.stamp(9, 1, stain.Blood, 250)
	g.clock.Advance(100)
	v := g.World().Stains
	if len(v) != 3 || v[0].Y != 1 || v[1].X != 2 || v[2].X != 5 {
		t.Fatalf("order: %+v", v)
	}
	if v[1].Amount != 1.0 || v[1].Kind != "ichor" || v[1].AgeTicks != 100 || v[0].Amount != 0.25 {
		t.Fatalf("fields: %+v", v)
	}
	// a born-in-the-future stain (never happens) reads as new
	g.stains.tiles[[2]int{9, 1}] = stain.Stain{X: 9, Y: 1, Amount: 5, Born: uint64(g.clock.Now()) + 9}
	if g.World().Stains[0].AgeTicks != 0 {
		t.Fatal("age underflow")
	}
}

func TestStainsSaveRoundTrip(t *testing.T) {
	g := stainGame(t)
	g.stamp(5, 3, stain.Blood, 500)
	g.stamp(2, 3, stain.Ichor, 1000)
	g.stamp(9, 1, stain.Blood, 250)
	var buf bytes.Buffer
	if err := g.Save(&buf, "stains"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.exportStains(), h.exportStains()) || len(h.exportStains()) != 3 {
		t.Fatalf("round trip: %+v vs %+v", g.exportStains(), h.exportStains())
	}
	if !reflect.DeepEqual(g.World().Stains, h.World().Stains) {
		t.Fatal("views differ after load")
	}
	// the fade remainder survives: both keep fading in lock step
	for i := 0; i < 7; i++ {
		g.stainDecay()
		h.stainDecay()
	}
	var buf2 bytes.Buffer
	if err := g.Save(&buf2, "stains"); err != nil {
		t.Fatal(err)
	}
	h2, _, err := Load(&buf2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		g.stainDecay()
		h2.stainDecay()
	}
	if !reflect.DeepEqual(g.exportStains(), h2.exportStains()) {
		t.Fatalf("decay diverged after load: %+v vs %+v", g.exportStains(), h2.exportStains())
	}
	if e := g.exportStains(); e[0].Y != 1 || e[1].X != 2 || e[2].X != 5 {
		t.Fatalf("export order: %+v", e)
	}
}

func TestOldSaveNoStains(t *testing.T) {
	g := stainGame(t)
	g.stamp(5, 3, stain.Blood, 500)
	g.importStains(nil)
	if len(g.exportStains()) != 0 || len(g.World().Stains) != 0 || g.stains.tiles != nil {
		t.Fatal("nil import should clear the stains")
	}
	g.importStains([]stain.Stain{
		{X: 1, Y: 1, Amount: 0},    // empty: dropped
		{X: -1, Y: 1, Amount: 50},  // off the map: dropped
		{X: 2, Y: 2, Amount: 5000}, // clamped
		{X: 3, Y: 3, Amount: 40, Born: 7},
	})
	e := g.exportStains()
	if len(e) != 2 || e[0].Amount != stain.Full || e[1].Born != 7 {
		t.Fatalf("import = %+v", e)
	}
	var buf bytes.Buffer
	n := stainGame(t)
	if err := n.Save(&buf, "clean"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil || len(h.exportStains()) != 0 {
		t.Fatalf("clean save: %v %v", err, h.exportStains())
	}
}

func TestStainDecayPrunesGoneCreatures(t *testing.T) {
	g := stainGame(t)
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	deer, _ := g.SpawnAnimal("deer", 9, 9)
	g.stains.dripN = map[entity.EntityID]int{id: 1, deer: 2, 9999: 2}
	dead, _ := g.SpawnNamedPawn(6, 5, "Bo")
	v, _ := g.vital.Get(dead)
	v.Status = health.Dead
	g.vital.Set(dead, v)
	g.stains.dripN[dead] = 1
	g.stainDecay()
	if len(g.stains.dripN) != 2 || g.stains.dripN[id] != 1 || g.stains.dripN[deer] != 2 {
		t.Fatalf("dripN = %v", g.stains.dripN)
	}
}

func stainBattle(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 5, Width: 30, Height: 14})
	jumpTo(g, 10, 1)
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.SpawnAnimal("hive_beetle", 12, 6)
	g.SetWeather(weather.Rain, 0.6)
	g.HoldWeather()
	g.Step(15000)
	g.SetWeather(weather.Clear, 0)
	g.strikeUntilHit(t, a, b, weapon.Fists)
	g.Step(5000)
	return g
}

func TestStainsDeterministic(t *testing.T) {
	a, b := stainBattle(t), stainBattle(t)
	if len(a.exportStains()) == 0 {
		t.Fatal("no stains to compare")
	}
	if !reflect.DeepEqual(a.exportStains(), b.exportStains()) ||
		!reflect.DeepEqual(a.World().Stains, b.World().Stains) {
		t.Fatalf("stains diverged\n%+v\n%+v", a.exportStains(), b.exportStains())
	}
}

// BenchmarkStainDecay is one climate step over a full map of stains in
// the rain (gate: <= 0.1 ms).
func BenchmarkStainDecay(b *testing.B) {
	g := New(Config{Seed: 1, Width: 60, Height: 40})
	g.SetWeather(weather.Rain, 0.1)
	seed := func() {
		g.stains = stainState{}
		fillStains(g, 0, stain.Cap, stain.Full)
	}
	seed()
	i := 0
	for b.Loop() {
		if i++; i%100 == 0 {
			b.StopTimer()
			seed()
			b.StartTimer()
		}
		g.stainDecay()
	}
}
