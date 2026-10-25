package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/terrain"
)

// TestLightRulesOnlyMossAndDuskmoss: every species but moss and duskmoss
// keeps no light rule, so legacy growth is unchanged.
func TestLightRulesOnlyMossAndDuskmoss(t *testing.T) {
	t.Parallel()
	for s := Species(0); s < numSpecies; s++ {
		d := DefOf(s)
		switch s {
		case Moss:
			if d.MinLight != 35 || d.DiesInLight != 0 || d.Cave {
				t.Fatalf("moss: %+v", d)
			}
		case Duskmoss:
			if d.MinLight != 0 || d.DiesInLight != 45 || d.Glow != 10 || !d.Cave || d.WalkCost != 125 {
				t.Fatalf("duskmoss: %+v", d)
			}
		default:
			if d.MinLight != 0 || d.DiesInLight != 0 || d.Glow != 0 || d.Cave {
				t.Fatalf("%s has a light rule: %+v", s, d)
			}
		}
	}
}

// TestFertilityOnCaveFloor: a Cave species grows on rough or smoothed
// stone at CaveFertility; everything else keeps the soil's fertility.
func TestFertilityOnCaveFloor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    Species
		t    terrain.Type
		want int
	}{
		{Duskmoss, terrain.Stone, CaveFertility},
		{Duskmoss, terrain.StoneSmooth, CaveFertility},
		{Duskmoss, terrain.Gravel, 70},
		{Duskmoss, terrain.Rock, 0},
		{Duskmoss, terrain.ShallowWater, 0},
		{Moss, terrain.Stone, 0},
		{Moss, terrain.StoneSmooth, 0},
		{Moss, terrain.Soil, 100},
		{Potato, terrain.RichSoil, 140},
	}
	for _, c := range cases {
		if got := Fertility(c.s, c.t); got != c.want {
			t.Fatalf("Fertility(%s, %s) = %d, want %d", c.s, c.t, got, c.want)
		}
	}
}

// TestLitEnough: MinLight gates growth; no rule grows in any light.
func TestLitEnough(t *testing.T) {
	t.Parallel()
	if !LitEnough(Moss, 350) || LitEnough(Moss, 349) || LitEnough(Moss, 0) {
		t.Fatal("moss needs 35% light")
	}
	if !LitEnough(Duskmoss, 0) || !LitEnough(Tree, 0) || !LitEnough(Tree, 1000) {
		t.Fatal("no rule: grows at any light")
	}
}

// TestHarmedHysteresis: harm starts at DiesInLight and stops only below
// WitherStopLight, so a light between the two keeps the current state.
func TestHarmedHysteresis(t *testing.T) {
	t.Parallel()
	if Harmed(Duskmoss, false, 449) || !Harmed(Duskmoss, false, 450) {
		t.Fatal("harm starts at 45%")
	}
	if !Harmed(Duskmoss, true, 400) || !Harmed(Duskmoss, true, 350) || Harmed(Duskmoss, true, 349) {
		t.Fatal("harm stops below 35%")
	}
	if Harmed(Duskmoss, false, 400) {
		t.Fatal("a healthy plant between the thresholds stays healthy")
	}
	if Harmed(Moss, false, 1000) || Harmed(Tree, true, 1000) {
		t.Fatal("no DiesInLight: never harmed")
	}
}

// TestWitherLoss: 8% of full growth per hour of exposure.
func TestWitherLoss(t *testing.T) {
	t.Parallel()
	g := DefOf(Duskmoss).GrowTicks
	if got, want := WitherLoss(Duskmoss, clock.TicksPerHour), g*8/100; got != want {
		t.Fatalf("an hour: %d, want %d", got, want)
	}
	if got := WitherLoss(Duskmoss, clock.TicksPerHour/10); got != g*8/1000 {
		t.Fatalf("a tenth of an hour: %d", got)
	}
	if WitherLoss(Duskmoss, 1) < 1 {
		t.Fatal("any exposure costs at least one growth tick")
	}
	if WitherLoss(Moss, clock.TicksPerHour) != 0 || WitherLoss(Duskmoss, 0) != 0 {
		t.Fatal("no loss without a rule or exposure")
	}
}
