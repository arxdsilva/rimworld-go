package render

import (
	"image/color"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/vault"
)

// tinyResult is a 12x8 map of shallow water with a vault and a bridge.
func tinyResult() mapgen.Result {
	tm := terrain.NewMap(12, 8)
	for i := range tm.Cells {
		tm.Cells[i] = terrain.ShallowWater
	}
	return mapgen.Result{
		W: 12, H: 8, Terrain: tm,
		Fields:  &mapgen.Fields{},
		Bridges: []mapgen.Point{{X: 1, Y: 1}, {X: 2, Y: 1}},
		Vaults: []mapgen.Vault{{
			ID: 0, X0: 4, Y0: 0, X1: 9, Y1: 6,
			Walls:         []mapgen.Point{{X: 4, Y: 0}, {X: 5, Y: 0}},
			WallStuff:     item.Steel,
			Floors:        []mapgen.RuinFloor{{X: 5, Y: 1}},
			Caskets:       []mapgen.Casket{{X: 6, Y: 2, Occupant: vault.Grateful}},
			Loot:          []mapgen.Loot{{X: 7, Y: 4, Kind: item.Wood, Qty: 3}},
			DefenderSpots: []mapgen.Point{{X: 8, Y: 5}},
		}},
	}
}

func TestCompositeDrawsBridgesAndVaults(t *testing.T) {
	img, err := Composite(tinyResult())
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) color.RGBA { return img.RGBAAt(x, y) }
	if at(1, 1) != BridgeColor || at(2, 1) != BridgeColor {
		t.Errorf("bridge cells = %v %v, want %v", at(1, 1), at(2, 1), BridgeColor)
	}
	if at(0, 0) != TerrainColor(terrain.ShallowWater) {
		t.Errorf("untouched water changed: %v", at(0, 0))
	}
	checks := []struct {
		name string
		x, y int
		want color.RGBA
	}{
		{"wall", 4, 0, VaultWallColor}, {"floor", 5, 1, VaultFloorColor},
		{"casket top", 6, 2, CasketColor}, {"casket bottom", 6, 3, CasketColor},
		{"loot", 7, 4, LootColor}, {"defender", 8, 5, DefenderColor},
	}
	for _, c := range checks {
		if got := at(c.x, c.y); got != c.want {
			t.Errorf("%s at (%d,%d) = %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestResourcesViewDrawsVaultsToo(t *testing.T) {
	v, err := newView(tinyResult())
	if err != nil {
		t.Fatal(err)
	}
	img := v.resources()
	if img.RGBAAt(6, 2) != CasketColor || img.RGBAAt(1, 1) != BridgeColor {
		t.Errorf("resources view lacks casket/bridge: %v %v", img.RGBAAt(6, 2), img.RGBAAt(1, 1))
	}
}

func TestVaultColoursAreDistinct(t *testing.T) {
	cs := map[color.RGBA]string{}
	for n, c := range map[string]color.RGBA{"bridge": BridgeColor, "wall": VaultWallColor, "floor": VaultFloorColor,
		"casket": CasketColor, "loot": LootColor, "defender": DefenderColor, "ruin": RuinColor} {
		if p, dup := cs[c]; dup {
			t.Errorf("%s and %s share %v", n, p, c)
		}
		cs[c] = n
	}
	if PlantColor(plant.Duskmoss) == PlantColor(plant.Moss) {
		t.Error("duskmoss must read differently from moss")
	}
}
