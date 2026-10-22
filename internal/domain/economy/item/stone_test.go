package item

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/geology"
)

func TestStoneKindsRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stone          geology.Stone
		chunk, blocks  Kind
		chunkName, blk string
	}{
		{geology.Sandstone, SandstoneChunk, SandstoneBlocks, "sandstone chunk", "sandstone blocks"},
		{geology.Granite, GraniteChunk, GraniteBlocks, "granite chunk", "granite blocks"},
		{geology.Limestone, LimestoneChunk, LimestoneBlocks, "limestone chunk", "limestone blocks"},
		{geology.Slate, SlateChunk, SlateBlocks, "slate chunk", "slate blocks"},
		{geology.Marble, MarbleChunk, MarbleBlocks, "marble chunk", "marble blocks"},
	}
	if len(tests) != len(geology.Stones()) {
		t.Fatalf("table covers %d stones, geology has %d", len(tests), len(geology.Stones()))
	}
	for _, tc := range tests {
		t.Run(tc.stone.String(), func(t *testing.T) {
			if ChunkOf(tc.stone) != tc.chunk || BlocksOf(tc.stone) != tc.blocks {
				t.Fatalf("ChunkOf/BlocksOf = %v/%v", ChunkOf(tc.stone), BlocksOf(tc.stone))
			}
			if StoneOf(tc.chunk) != tc.stone || StoneOf(tc.blocks) != tc.stone {
				t.Fatalf("StoneOf = %v/%v", StoneOf(tc.chunk), StoneOf(tc.blocks))
			}
			if tc.chunk.String() != tc.chunkName || tc.blocks.String() != tc.blk {
				t.Fatalf("names %q/%q", tc.chunk.String(), tc.blocks.String())
			}
			for _, k := range []Kind{tc.chunk, tc.blocks} {
				if got, ok := ParseKind(k.String()); !ok || got != k {
					t.Fatalf("ParseKind(%q) = %v, %v", k.String(), got, ok)
				}
			}
			if !IsChunk(tc.chunk) || IsChunk(tc.blocks) || !IsStoneBlocks(tc.blocks) || IsStoneBlocks(tc.chunk) {
				t.Fatal("IsChunk/IsStoneBlocks mixed up")
			}
			if tc.chunk.Value() != 0 || tc.blocks.Value() != 1 {
				t.Fatalf("values %d/%d", tc.chunk.Value(), tc.blocks.Value())
			}
		})
	}
	if ChunkOf(geology.None) != Food || BlocksOf(geology.None) != Food || ChunkOf(geology.Stone(99)) != Food {
		t.Fatal("no stone must map to the zero kind")
	}
	if !slices.Equal(StoneBlocks(), []Kind{SandstoneBlocks, GraniteBlocks, LimestoneBlocks, SlateBlocks, MarbleBlocks}) {
		t.Fatalf("StoneBlocks() = %v", StoneBlocks())
	}
}

func TestMetalKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		k     Kind
		name  string
		value int
	}{
		{Steel, "steel", 2},
		{Silver, "silver", 1},
		{Gold, "gold", 10},
	}
	for _, tc := range tests {
		if !IsMetal(tc.k) || tc.k.String() != tc.name || tc.k.Value() != tc.value || StoneOf(tc.k) != geology.None {
			t.Errorf("%v: metal=%v name=%q value=%d", tc.k, IsMetal(tc.k), tc.k.String(), tc.k.Value())
		}
		if got, ok := ParseKind(tc.name); !ok || got != tc.k {
			t.Errorf("ParseKind(%q) = %v, %v", tc.name, got, ok)
		}
	}
	if IsMetal(Wood) || IsMetal(GraniteBlocks) {
		t.Fatal("wood and stone are not metal")
	}
}

func TestIsStuff(t *testing.T) {
	t.Parallel()
	yes := append([]Kind{Wood, Steel}, StoneBlocks()...)
	no := []Kind{Stone, Silver, Gold, Food, Cloth, SandstoneChunk, GraniteChunk, LimestoneChunk, SlateChunk, MarbleChunk}
	for _, k := range yes {
		if !k.IsStuff() {
			t.Errorf("%v must be stuff", k)
		}
	}
	for _, k := range no {
		if k.IsStuff() {
			t.Errorf("%v must not be stuff", k)
		}
	}
}

func TestStoneOfLegacyIsGranite(t *testing.T) {
	t.Parallel()
	if StoneOf(Stone) != geology.Granite {
		t.Fatalf("legacy stone reads as %v", StoneOf(Stone))
	}
	if Stone.String() != "stone" {
		t.Fatalf("legacy stone renamed to %q", Stone.String())
	}
	if StoneOf(Wood) != geology.None {
		t.Fatal("wood has no stone")
	}
}
