package ebiten

import (
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func testPack(t *testing.T) Pack {
	t.Helper()
	root, err := LocatePack()
	if err != nil {
		t.Skip(err)
	}
	p, err := DiscoverPack(root)
	if err != nil {
		t.Fatalf("DiscoverPack: %v", err)
	}
	return p
}

func TestDiscoverPackInventoriesVendorKit(t *testing.T) {
	p := testPack(t)

	if len(p.Colonists) != 4 {
		t.Fatalf("colonists = %d, want 4", len(p.Colonists))
	}
	if len(p.Enemies) != 4 {
		t.Fatalf("enemies = %d, want 4", len(p.Enemies))
	}
	if len(p.Ground) < 1 || len(p.Rocks) < 1 {
		t.Fatalf("ground=%d rocks=%d", len(p.Ground), len(p.Rocks))
	}
	if p.Crosshair == "" {
		t.Fatal("missing extras/crosshair.png")
	}
	if len(p.Extras) != 3 {
		t.Fatalf("extras = %d, want 3", len(p.Extras))
	}
	if len(p.Weapons) != 3 {
		t.Fatalf("weapons = %d, want 3", len(p.Weapons))
	}
	if len(p.BodyParts) == 0 {
		t.Fatal("expected layered body parts")
	}
	for _, ch := range p.Colonists {
		if len(ch.Anims["idle"]) == 0 || len(ch.Anims["walk"]) == 0 {
			t.Fatalf("%s missing idle/walk: %+v", ch.ID, keys(ch.Anims))
		}
	}
	if p.PNGCount < 1000 {
		t.Fatalf("PNGCount = %d, pack looks truncated", p.PNGCount)
	}
}

func TestDiscoverPackClassifiesEveryPNG(t *testing.T) {
	p := testPack(t)
	families := []string{
		"Animated body parts",
		"Environment",
		"Extras",
		"Full body animated characters",
		"Generic death animation",
		"Weapons",
	}
	var unknown []string
	if err := walkPNGs(p.Root, func(path string) {
		rel, err := filepath.Rel(p.Root, path)
		if err != nil {
			t.Fatal(err)
		}
		for _, fam := range families {
			if rel == fam || strings.HasPrefix(rel, fam+string(filepath.Separator)) {
				return
			}
		}
		unknown = append(unknown, rel)
	}); err != nil {
		t.Fatal(err)
	}
	if len(unknown) > 0 {
		t.Fatalf("%d PNGs outside known families, e.g. %s", len(unknown), unknown[0])
	}
}

func TestColonistAppearanceIsStableForEntityID(t *testing.T) {
	a := &atlas{colonists: []*spriteSet{{}, {}, {}, {}}}
	if a.colonist(1) != a.colonists[0] || a.colonist(5) != a.colonists[0] {
		t.Fatal("EntityID 1 and 5 should map to the first of 4 colonists")
	}
	if a.colonist(entity.EntityID(4)) != a.colonists[3] {
		t.Fatal("EntityID 4 should map to the last colonist")
	}
	if a.colonist(1) != a.colonist(1) {
		t.Fatal("appearance must be stable")
	}
}

func TestTileDecorIsDeterministic(t *testing.T) {
	if tileHash(3, 7) != tileHash(3, 7) {
		t.Fatal("tileHash must be stable")
	}
	if tileHash(3, 7) == tileHash(7, 3) && tileHash(1, 0) == tileHash(0, 1) {
		t.Fatal("tileHash should vary with coordinates")
	}
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
