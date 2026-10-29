package research

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
)

// TestWaveDGates: every wave D kind the wiki research tree gates has a
// build gate; the free ones (spots, sculpting bench, stool, barricade,
// sculptures) have none; a key under two projects needs both.
func TestWaveDGates(t *testing.T) {
	t.Parallel()
	var pr Progress
	gated := map[construction.Kind]Project{
		construction.FueledSmithy: Metalworking, construction.MachiningTable: MachineTools,
		construction.FabricationBench: Fabrication, construction.ElectricSmelter: Electricity,
		construction.BiofuelRefinery: FuelRefining, construction.FermentingBarrel: Brewing,
		construction.Armchair: Upholstery, construction.IEDHighExplosive: Charges, construction.IEDIncendiary: Charges,
		construction.IEDSmoke: Charges, construction.Autocannon: HeavyTurrets,
		construction.UraniumSlugTurret: SlugTurrets, construction.Mortar: Artillery, construction.Autodoor: PoweredDoors,
	}
	for k, want := range gated {
		if p, ok := pr.Locks("build:" + k.String()); !ok || p != want {
			t.Errorf("%s: locked by %v %v, want %v", k, p, ok, want)
		}
	}
	for _, k := range []construction.Kind{construction.CraftingSpot, construction.ButcherSpot, construction.ArtBench,
		construction.Stool, construction.Barricade, construction.SmallSculpture, construction.LargeSculpture, construction.GrandSculpture} {
		if p, ok := pr.Locks("build:" + k.String()); ok {
			t.Errorf("%s needs no research, locked by %v", k, p)
		}
	}
	// the electric forge needs both Electricity and Metalworking
	pr.Add(Electricity, DefOf(Electricity).Cost)
	if p, ok := pr.Locks("build:electric forge"); !ok || p != Metalworking {
		t.Fatalf("electric forge after Electricity: %v %v", p, ok)
	}
	pr.Add(Metalworking, DefOf(Metalworking).Cost)
	if _, ok := pr.Locks("build:electric forge"); ok {
		t.Fatal("electric forge open with both done")
	}
	pr.Add(Traps, DefOf(Traps).Cost)
	pr.Add(Charges, DefOf(Charges).Cost)
	if p, ok := pr.Locks("build:pulse charge"); !ok || p != Microcircuits {
		t.Fatalf("pulse charge needs Microcircuits too: %v %v", p, ok)
	}
}

// TestUnlockKeysExist: every build and recipe key names a real kind or
// recipe, every project is reachable and none requires itself.
func TestUnlockKeysExist(t *testing.T) {
	t.Parallel()
	recipes := map[string]bool{}
	for _, d := range recipe.All() {
		recipes[d.Name] = true
	}
	for _, d := range All() {
		for _, u := range d.Unlocks {
			switch {
			case strings.HasPrefix(u, "build:"):
				if _, ok := construction.Parse(strings.TrimPrefix(u, "build:")); !ok {
					t.Errorf("%s: unknown kind %q", d.Name, u)
				}
			case strings.HasPrefix(u, "recipe:"):
				if !recipes[strings.TrimPrefix(u, "recipe:")] {
					t.Errorf("%s: unknown recipe %q", d.Name, u)
				}
			}
		}
		for _, r := range d.Requires {
			if r >= d.Project {
				t.Errorf("%s requires a later project %v", d.Name, r)
			}
		}
	}
}
