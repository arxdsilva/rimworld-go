package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestPartialHarvest (wiki: Plants): nothing under 65 % growth, half the
// yield at 65 %, linear to all of it ripe; blight gives nothing.
func TestPartialHarvest(t *testing.T) {
	t.Parallel()
	gt := DefOf(Corn).GrowTicks
	for pct, want := range map[int]int{50: 0, 64: 0, 65: 11, 70: 12, 100: 22} {
		p := Plant{Species: Corn, Growth: gt * pct / 100}
		if got := p.HarvestYield(22); got != want {
			t.Errorf("corn at %d%%: %d, want %d", pct, got, want)
		}
		if p.HarvestableNow() != (pct >= HarvestMinPct) {
			t.Errorf("corn at %d%%: harvestable now %v", pct, p.HarvestableNow())
		}
	}
	if (Plant{Species: Corn, Growth: gt, Blight: 30}).HarvestYield(22) != 0 {
		t.Error("a blighted crop yields something")
	}
	if (Plant{Species: Corn, Growth: gt, Damage: 50}).HarvestYield(22) != 11 {
		t.Error("damage halves the yield")
	}
	if (Plant{Species: Tree, Growth: DefOf(Tree).GrowTicks}).HarvestableNow() {
		t.Error("a tree is never harvested")
	}
}

// TestLifespan (wiki: Plants): a crop lives grow days x 5 from sowing.
func TestLifespan(t *testing.T) {
	t.Parallel()
	if got := Lifespan(Rice); got != 15*clock.TicksPerDay {
		t.Fatalf("rice lives %d ticks, want 15 days", got)
	}
	if Lifespan(Tree) != 0 || Lifespan(BerryBush) != 0 {
		t.Fatal("only crops age to death")
	}
	p := Plant{Species: Rice, Sown: 1000}
	if p.Rotted(1000+Lifespan(Rice)-1) || !p.Rotted(1000+Lifespan(Rice)) || p.LifePct(1000+Lifespan(Rice)/2) != 50 {
		t.Fatalf("rice life: %d%%", p.LifePct(1000+Lifespan(Rice)/2))
	}
	if (Plant{Species: Rice}).Rotted(1 << 40) {
		t.Fatal("a plant never seen growing has no age")
	}
}

// TestLightFactor (wiki: Plants, Sun lamp): nothing under 51 % light,
// full speed at 100 %; a plain lamp's 50 % grows nothing.
func TestLightFactor(t *testing.T) {
	t.Parallel()
	for l, want := range map[int]int{0: 0, LampGrowLight: 0, 509: 0, 510: 0, 755: 500, 1000: 1000, 1200: 1000} {
		if got := LightFactor(l); got != want {
			t.Errorf("LightFactor(%d) = %d, want %d", l, got, want)
		}
	}
	for d, want := range map[[2]int]int{{0, 0}: 1000, {5, 0}: 1000, {4, 4}: 0 + 500, {6, 0}: 500, {10, 0}: 500, {11, 0}: 300, {12, 0}: 0} {
		if got := SunLampLight(d[0], d[1]); got != want {
			t.Errorf("sun lamp at %v: %d, want %d", d, got, want)
		}
	}
	day := uint64(4 * clock.TicksPerDay)
	if SunLampOn(day+5*clock.TicksPerHour) || !SunLampOn(day+6*clock.TicksPerHour) ||
		!SunLampOn(day+19*clock.TicksPerHour+499) || SunLampOn(day+19*clock.TicksPerHour+500) {
		t.Error("the sun lamp shines 06:00-19:12")
	}
}

// TestHydroponicsAndBlight: the basin's crops (wiki: no corn, haygrass or
// trees), dying unpowered within a day; blight strikes zone crops of
// under 15 grow days and worsens a few points a day.
func TestHydroponicsAndBlight(t *testing.T) {
	t.Parallel()
	if !Hydroponic(Rice) || !Hydroponic(Healroot) || Hydroponic(Corn) || Hydroponic(Haygrass) || Hydroponic(Tree) || Hydroponic(Agave) {
		t.Error("hydroponic species")
	}
	p := Plant{Species: Rice}
	for at := uint64(0); at < clock.TicksPerDay && !p.Dead(); at += 250 {
		p = p.Starve(at, at+250)
	}
	if !p.Dead() {
		t.Fatalf("unpowered rice alive after a day: %d%%", p.Damage)
	}
	if !(Plant{Species: Rice, Zone: true}).Blightable() || (Plant{Species: Rice}).Blightable() ||
		(Plant{Species: Tree, Zone: true}).Blightable() || !(Plant{Species: Corn, Zone: true}).Blightable() {
		t.Error("blightable plants")
	}
	b := Plant{Species: Rice, Zone: true, Blight: BlightStart}.Blighten(0, 2*clock.TicksPerDay)
	if b.Blight != BlightStart+2*BlightPerDay {
		t.Errorf("blight after two days %d", b.Blight)
	}
	if (Plant{Species: Rice}).Blighten(0, clock.TicksPerDay).Blight != 0 {
		t.Error("blight grew on a healthy plant")
	}
	if Roll(1, 2, 3) == Roll(1, 2, 4) || Roll(1, 2, 3) != Roll(1, 2, 3) {
		t.Error("Roll")
	}
}

// TestFromLegacyGrowth: an older save's plants keep their share of the way
// to ripe in the new growth units; a ripe one stays ripe.
func TestFromLegacyGrowth(t *testing.T) {
	t.Parallel()
	if p := (Plant{Species: Potato, Growth: 262500}).FromLegacyGrowth(); !p.Ripe() {
		t.Fatalf("a ripe potato is %d%% after the migration", p.Percent())
	}
	if p := (Plant{Species: Tree, Growth: 30000}).FromLegacyGrowth(); p.Percent() != 50 {
		t.Fatalf("a half-grown tree is %d%%", p.Percent())
	}
	if p := (Plant{Species: Moss, Growth: 1234}).FromLegacyGrowth(); p.Growth != 1234 {
		t.Fatal("moss growth changed")
	}
	if legacyGrow[Potato] != int(7.5*35000) || legacyGrow[Radish] != growDays(2) {
		t.Fatal("legacy table off the old growDays rule")
	}
}
