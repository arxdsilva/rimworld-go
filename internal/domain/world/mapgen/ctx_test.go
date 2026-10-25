package mapgen

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
)

func TestCtxHelpers(t *testing.T) {
	s := site.Site{Biome: biome.TemperateForest}
	cfg := Config{Seed: 9, W: 32, H: 32, Site: s, Biome: biome.DefOf(s.Biome), Debug: true}
	c := NewCtx(cfg)
	if c.Rand("a").IntN(1<<30) == c.Rand("b").IntN(1<<30) || c.Rand("a").IntN(1<<30) != c.Rand("a").IntN(1<<30) {
		t.Fatal("step streams")
	}
	if c.NoiseSeed("a") == c.NoiseSeed("b") || c.Scale() <= 0 {
		t.Fatal("noise seeds collide")
	}
	if x, y := c.XY(c.Idx(5, 7)); x != 5 || y != 7 || !c.In(0, 0) || c.In(32, 0) || c.Start() != (Point{16, 16}) {
		t.Fatal("ctx helpers")
	}
	if c.F.RiverDist[0] != Far || len(c.F.Water) != 1024 {
		t.Fatal("fields not allocated")
	}
	if r := c.Finish(); r.Fields != c.F || r.W != 32 {
		t.Fatal("Finish keeps debug fields")
	}
	c.Cfg.Debug = false
	if c.Finish().Fields != nil {
		t.Fatal("Finish drops fields without Debug")
	}
	if !Deep.Wet() || !Shallow.Wet() || Beach.Wet() {
		t.Fatal("Water.Wet")
	}
}
