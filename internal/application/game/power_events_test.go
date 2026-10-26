package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 EVENTS: the solar flare and the short circuit (power_events.go).

// circuit is a small grid: a row of conduits along y = 5 (visible ones,
// then hidden ones) and one battery two rows below, on one net.
type circuit struct {
	g       *Game
	visible []entity.EntityID
	hidden  []entity.EntityID
	battery entity.EntityID
}

// eventGridGame is a blank 40x30 map at day 20, 06:00, with a battery and
// visible+hidden conduits in a row. The battery is empty.
func eventGridGame(t *testing.T, seed uint64, visible, hidden int) *circuit {
	t.Helper()
	g := New(Config{Seed: seed, Width: 40, Height: 30})
	jumpTo(g, 20, 6)
	c := &circuit{g: g}
	x := 4
	for i := 0; i < visible; i++ {
		c.visible = append(c.visible, placeFuel(t, g, construction.PowerConduit, x, 5))
		x++
	}
	for i := 0; i < hidden; i++ {
		c.hidden = append(c.hidden, placeFuel(t, g, construction.HiddenConduit, x, 5))
		x++
	}
	c.battery = placeFuel(t, g, construction.Battery, 4, 7)
	return c
}

// charge sets the battery's stored energy to wh.
func (c *circuit) charge(wh int) {
	pw := c.g.ensurePower(uint64(c.g.clock.Now()))
	pw.charge[c.battery] = int64(wh) * power.WTicksPerWh
	c.g.bumpPower()
}

func (c *circuit) removed(ids []entity.EntityID) (gone []entity.EntityID) {
	for _, id := range ids {
		if _, ok := c.g.bldgs.Get(id); !ok {
			gone = append(gone, id)
		}
	}
	return gone
}

func eventLetters(g *Game, title string) []LetterView {
	var out []LetterView
	for _, l := range g.letterViews() {
		if l.Title == title {
			out = append(out, l)
		}
	}
	return out
}

func eventMessages(g *Game, sub string) int {
	n := 0
	for _, m := range g.messages {
		if strings.Contains(m.Text, sub) {
			n++
		}
	}
	return n
}

// --- solar flare ---------------------------------------------------------

func TestSolarFlareNeedsPowerDevices(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 30})
	jumpTo(g, 20, 6)
	now := uint64(g.clock.Now())
	if g.incidentFits(incident.SolarFlare, now) {
		t.Fatal("a solar flare fits a colony with no power devices")
	}
	if g.FireIncident("solar_flare") {
		t.Fatal("a flare fired with no devices")
	}
	if g.pw != nil || g.streams.m[streamPower] != nil || g.incidents.Last[incident.SolarFlare] != 0 {
		t.Fatal("a flare that did not fit left a trace")
	}
	placeFuel(t, g, construction.Battery, 4, 7) // a battery is a device
	if !g.incidentFits(incident.SolarFlare, now) {
		t.Fatal("a solar flare should fit a colony with a power device")
	}
}

func TestSolarFlareNotBeforeDay12AndCoolsDown(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 1, 3, 0)
	g := c.g
	d := incident.DefOf(incident.SolarFlare)
	at := func(day int) bool { return d.Allowed(g.incidents, uint64(day)*clock.TicksPerDay, g.biome) }
	if at(11) || !at(12) {
		t.Fatalf("flare day gate: day 11 %v, day 12 %v, want false, true", at(11), at(12))
	}
	if !g.FireIncident("solar_flare") {
		t.Fatal("the flare did not fire")
	}
	fired := g.rel(uint64(g.clock.Now()))
	if g.incidents.Last[incident.SolarFlare] != fired {
		t.Fatalf("flare fire time %d, want %d", g.incidents.Last[incident.SolarFlare], fired)
	}
	if d.Allowed(g.incidents, fired+11*clock.TicksPerDay, g.biome) || !d.Allowed(g.incidents, fired+12*clock.TicksPerDay, g.biome) {
		t.Fatal("the flare cooldown is 12 days")
	}
}

func TestSolarFlareFiringSetsWindowAndLetter(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 1, 3, 0)
	g := c.g
	now := uint64(g.clock.Now())
	if !g.FireIncident("solar_flare") {
		t.Fatal("the flare did not fire")
	}
	until := g.flareUntil()
	if !g.solarFlare() || until < now+3*clock.TicksPerHour || until > now+8*clock.TicksPerHour || (until-now)%clock.TicksPerHour != 0 {
		t.Fatalf("flare window %d..%d, want 3 to 8 whole hours from %d", now, until, now)
	}
	if !g.World().SolarFlare {
		t.Fatal("the world view does not show the flare")
	}
	ls := eventLetters(g, "Solar flare")
	if len(ls) != 1 || ls[0].Kind != letterCaution || ls[0].Tick != now || !strings.Contains(ls[0].Text, "hours") {
		t.Fatalf("flare letter: %+v", ls)
	}
	if g.incidentFits(incident.SolarFlare, now) {
		t.Fatal("a second flare must not fit during one")
	}
}

func TestSolarFlareDurationComesFromTheStream(t *testing.T) {
	t.Parallel()
	seen := map[uint64]bool{}
	for seed := uint64(1); seed <= 16; seed++ {
		a, b := eventGridGame(t, seed, 3, 0), eventGridGame(t, seed, 3, 0)
		now := uint64(a.g.clock.Now())
		a.g.FireIncident("solar_flare")
		b.g.FireIncident("solar_flare")
		if a.g.flareUntil() != b.g.flareUntil() {
			t.Fatalf("seed %d: the same seed gave flares ending %d and %d", seed, a.g.flareUntil(), b.g.flareUntil())
		}
		hours := (a.g.flareUntil() - now) / clock.TicksPerHour
		if hours < 3 || hours > 8 {
			t.Fatalf("seed %d: flare of %d hours", seed, hours)
		}
		seen[hours] = true
	}
	if len(seen) < 3 {
		t.Fatalf("flare lengths over 16 seeds: %v, want variety", seen)
	}
}

func TestSolarFlareEndsExactly(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 3, 3, 0)
	g := c.g
	g.FireIncident("solar_flare")
	until := g.flareUntil()
	g.Step(int(until-uint64(g.clock.Now())) - 1) // the last tick still to run: until-1
	if !g.solarFlare() || eventMessages(g, "flare has passed") != 0 {
		t.Fatal("the flare ended early")
	}
	g.Step(1) // runs tick until-1: the clock reaches the end
	if uint64(g.clock.Now()) != until || g.solarFlare() {
		t.Fatalf("at tick %d the flare (until %d) should be over", g.clock.Now(), until)
	}
	if n := eventMessages(g, "flare has passed"); n != 1 {
		t.Fatalf("%d end messages, want 1", n)
	}
	g.Step(1) // tick until: the power phase clears the flare
	g.Step(200)
	if g.flareUntil() != 0 || g.World().SolarFlare {
		t.Fatal("the flare was not cleared")
	}
	if n := eventMessages(g, "flare has passed"); n != 1 {
		t.Fatalf("%d end messages after the flare, want 1", n)
	}
	if len(eventLetters(g, "Solar flare")) != 1 {
		t.Fatal("the end is a message, not another letter")
	}
}

func TestSolarFlareSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 5, 3, 0)
	g := c.g
	g.FireIncident("solar_flare")
	until := g.flareUntil()
	g.Step(int(until-uint64(g.clock.Now())) / 2)
	var buf bytes.Buffer
	if err := g.Save(&buf, "flare"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !l.solarFlare() || l.flareUntil() != until {
		t.Fatalf("loaded flare until %d (flare %v), want %d", l.flareUntil(), l.solarFlare(), until)
	}
	rest := int(until-uint64(g.clock.Now())) + 5
	g.Step(rest)
	l.Step(rest)
	if g.solarFlare() || l.solarFlare() {
		t.Fatal("the flare outlived its window")
	}
	if a, b := eventMessages(g, "flare has passed"), eventMessages(l, "flare has passed"); a != 1 || b != 1 {
		t.Fatalf("end messages: unbroken run %d, loaded run %d, want 1 and 1", a, b)
	}
	if g.StateHash() != l.StateHash() {
		t.Fatal("a save in the middle of a flare changed the run")
	}
}

// --- short circuit -------------------------------------------------------

func TestShortCircuitNeedsStoredPowerAndAVisibleConduit(t *testing.T) {
	t.Parallel()
	now := func(c *circuit) uint64 { return uint64(c.g.clock.Now()) }

	c := eventGridGame(t, 1, 3, 0)
	if c.g.incidentFits(incident.ShortCircuit, now(c)) || c.g.FireIncident("short_circuit") {
		t.Fatal("a short circuit fits an empty battery")
	}
	c.charge(99)
	if c.g.incidentFits(incident.ShortCircuit, now(c)) {
		t.Fatal("a short circuit fits 99 Wh")
	}
	c.charge(100)
	if !c.g.incidentFits(incident.ShortCircuit, now(c)) {
		t.Fatal("a short circuit should fit 100 Wh and a visible conduit")
	}

	h := eventGridGame(t, 1, 0, 3) // buried conduits only
	h.charge(800)
	if h.g.incidentFits(incident.ShortCircuit, now(h)) || h.g.FireIncident("short_circuit") {
		t.Fatal("a short circuit fits a net with only hidden conduits")
	}
	if len(h.removed(h.hidden)) != 0 {
		t.Fatal("a hidden conduit was removed")
	}

	n := New(Config{Seed: 1, Width: 40, Height: 30})
	jumpTo(n, 20, 6)
	if n.incidentFits(incident.ShortCircuit, uint64(n.clock.Now())) {
		t.Fatal("a short circuit fits a colony with no power devices")
	}
	if n.streams.m[streamPower] != nil {
		t.Fatal("a colony without power drew from the power stream")
	}
}

func TestShortCircuitDrainsRemovesConduitAndStartsFire(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 2, 4, 0)
	g := c.g
	c.charge(1100)
	now := uint64(g.clock.Now())
	if !g.FireIncident("short_circuit") {
		t.Fatal("the short circuit did not fire")
	}
	gone := c.removed(c.visible)
	if len(gone) != 1 {
		t.Fatalf("%d conduits removed, want 1", len(gone))
	}
	if got := g.netStoredWh(g.netOf(c.battery)); got != 0 {
		t.Fatalf("the net still stores %d Wh", got)
	}
	// 1100 Wh stored: the fire is 1 + 1100/500 = 3. The conduit stood at
	// x = 4 + its position in the row, y = 5.
	var at int
	for i, id := range c.visible {
		if id == gone[0] {
			at = 4 + i
		}
	}
	if got := g.fireAt(at, 5); got != 3 {
		t.Fatalf("fire at %d,5 has size %d, want 3", at, got)
	}
	ls := eventLetters(g, "Short circuit")
	if len(ls) != 1 || ls[0].Kind != letterThreat || ls[0].X != at || ls[0].Y != 5 || ls[0].Tick != now {
		t.Fatalf("short circuit letter: %+v (conduit at %d,5)", ls, at)
	}
	if g.incidents.Last[incident.ShortCircuit] != g.rel(now) {
		t.Fatal("the short circuit was not recorded")
	}
	// the destroyed conduit refunds nothing: no wood or metal appears
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Qty > 0 {
			t.Fatalf("a short circuit left an item on the ground: %+v", st)
		}
	}
}

func TestShortCircuitCooldown(t *testing.T) {
	t.Parallel()
	c := eventGridGame(t, 1, 4, 0)
	g := c.g
	d := incident.DefOf(incident.ShortCircuit)
	at := func(day int) bool { return d.Allowed(g.incidents, uint64(day)*clock.TicksPerDay, g.biome) }
	if at(9) || !at(10) {
		t.Fatalf("short circuit day gate: day 9 %v, day 10 %v, want false, true", at(9), at(10))
	}
	c.charge(600)
	if !g.FireIncident("short_circuit") {
		t.Fatal("the short circuit did not fire")
	}
	fired := g.rel(uint64(g.clock.Now()))
	if d.Allowed(g.incidents, fired+7*clock.TicksPerDay, g.biome) || !d.Allowed(g.incidents, fired+8*clock.TicksPerDay, g.biome) {
		t.Fatal("the short circuit cooldown is 8 days")
	}
}

func TestShortCircuitPickIsDeterministicAndVaries(t *testing.T) {
	t.Parallel()
	picked := map[int]bool{}
	for seed := uint64(1); seed <= 24; seed++ {
		a, b := eventGridGame(t, seed, 8, 0), eventGridGame(t, seed, 8, 0)
		a.charge(500)
		b.charge(500)
		if !a.g.FireIncident("short_circuit") || !b.g.FireIncident("short_circuit") {
			t.Fatalf("seed %d: no short circuit", seed)
		}
		ga, gb := a.removed(a.visible), b.removed(b.visible)
		if len(ga) != 1 || len(gb) != 1 || ga[0] != gb[0] {
			t.Fatalf("seed %d: the same seed removed %v and %v", seed, ga, gb)
		}
		for i, id := range a.visible {
			if id == ga[0] {
				picked[i] = true
			}
		}
	}
	if len(picked) < 4 {
		t.Fatalf("24 seeds picked only conduits %v of 8", picked)
	}
}

func TestShortCircuitNeverPicksAHiddenConduit(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 24; seed++ {
		c := eventGridGame(t, seed, 2, 8) // 80% of the conduits are buried
		c.charge(300)
		if !c.g.FireIncident("short_circuit") {
			t.Fatalf("seed %d: no short circuit", seed)
		}
		if len(c.removed(c.hidden)) != 0 || len(c.removed(c.visible)) != 1 {
			t.Fatalf("seed %d: removed hidden %v, visible %v", seed, c.removed(c.hidden), c.removed(c.visible))
		}
	}
}

// TestShortCircuitPicksAmongNets: two nets with stored power; the pick is
// one of them and the other keeps its charge.
func TestShortCircuitDrainsOnlyItsNet(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 8; seed++ {
		c := eventGridGame(t, seed, 3, 0)
		g := c.g
		// a second, separate net: conduits and a battery on y = 15
		var other []entity.EntityID
		for x := 4; x < 7; x++ {
			other = append(other, placeFuel(t, g, construction.PowerConduit, x, 15))
		}
		bat2 := placeFuel(t, g, construction.Battery, 4, 17)
		c.charge(400)
		g.pw.charge[bat2] = 400 * power.WTicksPerWh
		g.bumpPower()
		if g.netOf(c.battery) == g.netOf(bat2) {
			t.Fatal("the two grids share a net")
		}
		if !g.FireIncident("short_circuit") {
			t.Fatalf("seed %d: no short circuit", seed)
		}
		first, second := g.netStoredWh(g.netOf(c.battery)), g.netStoredWh(g.netOf(bat2))
		if (first == 0) == (second == 0) {
			t.Fatalf("seed %d: stored %d and %d Wh, want exactly one net drained", seed, first, second)
		}
		gone := len(c.removed(c.visible)) + len(c.removed(other))
		if gone != 1 {
			t.Fatalf("seed %d: %d conduits removed", seed, gone)
		}
		if first == 0 && len(c.removed(c.visible)) != 1 || second == 0 && len(c.removed(other)) != 1 {
			t.Fatalf("seed %d: the conduit removed is not on the drained net", seed)
		}
	}
}

// --- the pool and determinism -----------------------------------------------

// TestPowerEventsJoinThePoolOnlyWithPower: the nature pool's total grows
// by the flare's weight with a device and by the short circuit's with stored
// energy; a bare map is unchanged.
func TestPowerEventsJoinThePoolOnlyWithPower(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 40, Height: 30})
	jumpTo(g, 20, 6)
	now := uint64(g.clock.Now())
	total := func() int {
		return incident.Total(incident.Nature, g.incidents, g.rel(now), g.biome, func(id incident.ID) bool { return g.incidentFits(id, now) })
	}
	bare := total()
	c := &circuit{g: g}
	for x := 4; x < 7; x++ {
		c.visible = append(c.visible, placeFuel(t, g, construction.PowerConduit, x, 5))
	}
	c.battery = placeFuel(t, g, construction.Battery, 4, 7)
	if got, want := total()-bare, incident.DefOf(incident.SolarFlare).Weight; got != want {
		t.Fatalf("a device adds %d to the pool, want the flare's %d", got, want)
	}
	c.charge(300)
	want := incident.DefOf(incident.SolarFlare).Weight + incident.DefOf(incident.ShortCircuit).Weight
	if got := total() - bare; got != want {
		t.Fatalf("a charged battery adds %d to the pool, want %d", got, want)
	}
}

// TestNatureRollCanPickThePowerEvents: through the storyteller's own roll
// (rollNature) a charged grid sees both events fire over a few seeds, and
// only through the pool.
func TestNatureRollCanPickThePowerEvents(t *testing.T) {
	t.Parallel()
	flares, shorts := 0, 0
	for seed := uint64(1); seed <= 30; seed++ {
		c := eventGridGame(t, seed, 4, 0)
		c.charge(500)
		c.g.rollNature(uint64(c.g.clock.Now()))
		if _, ok := c.g.incidents.Last[incident.SolarFlare]; ok {
			flares++
			if !c.g.solarFlare() {
				t.Fatalf("seed %d: a flare was recorded but is not on", seed)
			}
		}
		if _, ok := c.g.incidents.Last[incident.ShortCircuit]; ok {
			shorts++
			if len(c.removed(c.visible)) != 1 {
				t.Fatalf("seed %d: a short circuit was recorded but no conduit blew", seed)
			}
		}
	}
	if flares == 0 || shorts == 0 {
		t.Fatalf("over 30 rolls: %d flares, %d short circuits, want some of each", flares, shorts)
	}
}

// TestPowerEventsDrawNothingWithoutPower: a colony with no power devices
// asks the events whether they fit and gets no, drawing no randomness, so
// its run is the run it always was.
func TestPowerEventsDrawNothingWithoutPower(t *testing.T) {
	t.Parallel()
	a := NewColony(Config{Seed: 9, Width: 48, Height: 40})
	b := NewColony(Config{Seed: 9, Width: 48, Height: 40})
	a.Step(4000)
	b.Step(4000)
	now := uint64(a.clock.Now())
	for i := uint64(0); i < 30; i++ {
		day := now + i*clock.TicksPerDay
		if a.incidentFits(incident.SolarFlare, day) || a.incidentFits(incident.ShortCircuit, day) {
			t.Fatal("a power event fits a colony with no power")
		}
	}
	if a.FireIncident("solar_flare") || a.FireIncident("short_circuit") {
		t.Fatal("a power event fired in a colony with no power")
	}
	if a.streams.m[streamPower] != nil {
		t.Fatal("the power stream was drawn")
	}
	a.Step(2000)
	b.Step(2000)
	if a.StateHash() != b.StateHash() {
		t.Fatal("asking the power events changed the run")
	}
}
