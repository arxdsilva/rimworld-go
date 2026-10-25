package mapgen

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
)

func testCfg(seed uint64, w, h int) Config {
	s := site.Site{Biome: biome.TemperateForest, Hills: site.LargeHills}
	return Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)}
}

func TestGenerateRejectsEmpty(t *testing.T) {
	if _, err := Generate(testCfg(1, 0, 10)); err != ErrSize {
		t.Fatalf("err %v", err)
	}
}

func TestBuilderWalksEveryStep(t *testing.T) {
	b, err := NewBuilder(testCfg(1, 40, 30))
	if err != nil {
		t.Fatal(err)
	}
	steps, last := 0, float32(-1)
	for {
		label, frac, done, err := b.Step()
		if err != nil {
			t.Fatal(err)
		}
		steps++
		if done {
			break
		}
		if label == "" || frac <= last {
			t.Fatalf("step %d: label %q frac %v", steps, label, frac)
		}
		last = frac
	}
	if steps != Steps() {
		t.Fatalf("%d steps, want %d", steps, Steps())
	}
	if _, _, done, _ := b.Step(); !done {
		t.Fatal("a finished builder stays done")
	}
	r := b.Result()
	if r.W != 40 || r.H != 30 || r.Terrain.W != 40 || len(r.Roof) != 1200 || r.Fields != nil {
		t.Fatalf("result %dx%d fields %v", r.W, r.H, r.Fields != nil)
	}
}

func TestStepNamesUniqueAndLabelled(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range StepNames() {
		if seen[n] {
			t.Fatalf("duplicate step %q", n)
		}
		seen[n] = true
		if _, ok := labels[n]; !ok {
			t.Fatalf("step %q has no loading label", n)
		}
	}
	if Label("nope") == "" {
		t.Fatal("unknown steps still get a label")
	}
}

func TestDebugKeepsFieldsAndStreamsDiffer(t *testing.T) {
	cfg := testCfg(9, 32, 32)
	cfg.Debug = true
	r, err := Generate(cfg)
	if err != nil || r.Fields == nil || len(r.Fields.Elevation) != 32*32 {
		t.Fatalf("debug fields missing: %v", err)
	}
	c := newCtx(cfg)
	if c.Rand("a").IntN(1<<30) == c.Rand("b").IntN(1<<30) {
		t.Fatal("step streams collide")
	}
	if c.NoiseSeed("a") == c.NoiseSeed("b") || c.Scale() <= 0 {
		t.Fatal("noise seeds collide")
	}
	if x, y := c.XY(c.Idx(5, 7)); x != 5 || y != 7 || !c.In(0, 0) || c.In(32, 0) || c.Start() != (Point{16, 16}) {
		t.Fatal("ctx helpers")
	}
	if !Deep.Wet() || !Shallow.Wet() || Beach.Wet() {
		t.Fatal("Water.Wet")
	}
}
