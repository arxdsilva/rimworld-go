package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestFogDrawsHiddenTilesNearBlack(t *testing.T) {
	t.Parallel()
	// The baked chunk (kept in fc.pix; offscreen images cannot be read
	// back before the game starts): hidden tiles are the fog colour, far
	// revealed tiles clear, the tile beside the fog a gradient.
	w := fogWorld(16, 16, 8)
	var fc FogCache
	var c FogChunk
	fc.Bake(&c, w.Fog, 16, 16, 0, 0)
	at := func(tx, ty int) [4]uint8 {
		o := 4 * ((ty*FogPx+FogPx/2)*FogChunkPx + tx*FogPx + FogPx/2)
		return [4]uint8{fc.Pix[o], fc.Pix[o+1], fc.Pix[o+2], fc.Pix[o+3]}
	}
	if got := at(12, 5); got != [4]uint8{0x0B, 0x0B, 0x0E, 255} {
		t.Fatalf("hidden tile pixel = %v, want fog colour", got)
	}
	if got := at(2, 5); got[3] != 0 {
		t.Fatalf("revealed tile far from the edge is fogged: %v", got)
	}
	if got := at(7, 5); got[3] == 0 || got[3] == 255 {
		t.Fatalf("tile beside the fog is not a gradient: %v", got)
	}
}

func TestHoverSaysUnexplored(t *testing.T) {
	t.Parallel()
	w := fogWorld(10, 10, 5)
	if got := UnexploredLines(w, 6, 3); len(got) != 1 || got[0] != "Unexplored" {
		t.Fatalf("hidden hover = %v", got)
	}
	if UnexploredLines(w, 2, 3) != nil || UnexploredLines(game.WorldView{}, 2, 3) != nil {
		t.Fatal("revealed or fogless tile reads Unexplored")
	}
}

func fogWorld(w, h, hideFrom int) game.WorldView {
	fog := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := hideFrom; x < w; x++ {
			fog[y*w+x] = 1
		}
	}
	return game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), Fog: fog, FogVer: 1}
}
