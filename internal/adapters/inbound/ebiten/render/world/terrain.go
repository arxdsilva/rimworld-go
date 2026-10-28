package world

import (
	"math"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// TerrainPriority: a higher index draws its soft edge over lower ones
// (the order the terrain edge art is made for, ART-015). Rock draws its
// ground as rough stone, smoothed rock as smooth stone.
//
// Water is the lowest of all (NG-1 §7.3): deep -2, shallow -1, so every
// ground's edge overlaps it and shallow water's own edge overlaps deep.
// A track is bare dirt like soil; mud draws over the ground it lies on.
var TerrainPriority = map[string]int{"soil": 0, "rich_soil": 1, "gravel": 2, "stone": 3, "rock": 3,
	"stone_smooth": 4, "rock_smooth": 4, "sand": 5, "marsh": 6, "ice": 7,
	"mud": 8, "track": 0, "shallow_water": -1, "deep_water": -2}

// TerrainSprite is each terrain's tile family.
var TerrainSprite = map[string]string{"soil": "soil", "rich_soil": "soil_rich", "gravel": "gravel", "stone": "stone", "rock": "stone",
	"sand": "sand", "marsh": "marsh", "ice": "ice", "stone_smooth": "stone_smooth", "rock_smooth": "stone_smooth",
	"mud": "mud", "track": "track", "shallow_water": "shallow_water", "deep_water": "deep_water"}

// TerrainByPri lists the edge-drawing terrains by priority (index =
// priority; water and track draw no ground edge of their own here, the
// water edges do).
var TerrainByPri = []string{"soil", "rich_soil", "gravel", "stone", "stone_smooth", "sand", "marsh", "ice", "mud"}

// TerrainNames, TerrainPriByCode and stoneGroundCode index the read
// model's terrain codes once (no per-tile map lookups). Built at
// declaration and never assigned after.
var (
	TerrainNames     = game.TerrainNames()
	TerrainPriByCode = func() (out [256]int8) {
		for i, n := range TerrainNames {
			out[i] = int8(TerrainPriority[n])
		}
		return out
	}()
	IsWaterCode = func() (out [256]bool) {
		for i, n := range TerrainNames {
			out[i] = n == "shallow_water" || n == "deep_water"
		}
		return out
	}()
	stoneGroundCode = func() (out [256]bool) {
		for i, n := range TerrainNames {
			out[i] = n == "stone" || n == "stone_smooth" || sprite.IsRockWallName(n)
		}
		return out
	}()
	// TerrainEdgeKeys[p][mask] is terrain/<sprite>_edge_<NN> of priority p.
	TerrainEdgeKeys = func() [][16]string {
		out := make([][16]string, len(TerrainByPri))
		for p, n := range TerrainByPri {
			for m := 1; m < 16; m++ {
				out[p][m] = "terrain/" + TerrainSprite[n] + "_edge_" + sprite.TwoDigits(m)
			}
		}
		return out
	}()
)

// terrainFallback is how a terrain is drawn while its art is missing: a
// tinted soil or gravel tile.
type terrainFallback struct {
	base string
	cs   sprite.ColorScale
}

var terrainFallbacks = map[string]terrainFallback{
	"sand": {"gravel", sprite.ColorScale{1.35, 1.15, 0.7, 1}},
	// NG-1 ground before its art: water as a cool tint, mud and a track as soil
	"shallow_water": {"gravel", sprite.ColorScale{0.45, 0.78, 1.25, 1}},
	"deep_water":    {"gravel", sprite.ColorScale{0.18, 0.38, 0.8, 1}},
	"mud":           {"soil", sprite.ColorScale{0.62, 0.5, 0.38, 1}},
	"track":         {"gravel", sprite.ColorScale{1.1, 1.0, 0.85, 1}},
	"marsh":         {"soil", sprite.ColorScale{0.55, 0.78, 0.66, 1}},
	"ice":           {"gravel", sprite.ColorScale{0.78, 0.95, 1.3, 1}},
	// smooth stone before its art: rough stone, a touch lighter
	"stone_smooth": {"stone", sprite.ColorScale{1.12, 1.12, 1.12, 1}},
	"rock_smooth":  {"stone", sprite.ColorScale{1.12, 1.12, 1.12, 1}},
}

// TerrainTileKey is the ground tile of terrain t at variant v and the
// colour scale to draw it with: its own art, else the tinted fallback.
func (b *base) TerrainTileKey(t string, v int) (string, sprite.ColorScale) {
	base := TerrainSprite[t]
	if base == "" {
		base = "soil" // a terrain this client has never heard of
	}
	k := env.ArtKey("terrain", base, v)
	if b.art().Has(k) || b.art() == nil {
		return k, sprite.NoCS
	}
	if fb, ok := terrainFallbacks[t]; ok {
		return env.ArtKey("terrain", TerrainSprite[fb.base], v), fb.cs
	}
	return k, sprite.NoCS
}

// cleanlinessNote is a room's dirt for its label: empty while it is clean,
// else ", dirty (-3)" with the impressiveness it costs.
func cleanlinessNote(rm game.RoomView) string {
	if rm.Cleanliness == "" || rm.Cleanliness == "clean" {
		return ""
	}
	if rm.FilthPenalty > 0 {
		return ", " + rm.Cleanliness + " (-" + strconv.Itoa(rm.FilthPenalty) + ")"
	}
	return ", " + rm.Cleanliness
}

// RoomLabelAt describes the room at a tile for HUD/inspectors.
func RoomLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoomGrid) {
		return ""
	}
	id := w.RoomGrid[i]
	if id < 0 || id >= len(w.Rooms) {
		if id == -1 {
			return "Outdoors"
		}
		return ""
	}
	rm := w.Rooms[id]
	s := "Room: " + rm.Role + ", " + rm.Label + " (" + strconv.Itoa(rm.Score) + ")" + cleanlinessNote(rm)
	if rm.Lit {
		s += ", lit"
	}
	return s + ", roofed " + strconv.Itoa(int(math.Round(rm.Roofed*100))) + "%"
}
