package pipeline

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

func testCfg(seed uint64, w, h int) mapgen.Config {
	s := site.Site{Biome: biome.TemperateForest, Hills: site.LargeHills}
	return mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)}
}

func TestGenerateRejectsEmpty(t *testing.T) {
	if _, err := Generate(testCfg(1, 0, 10)); err != mapgen.ErrSize {
		t.Fatalf("err %v", err)
	}
}

func TestBuilderWalksEveryStep(t *testing.T) {
	b, err := NewBuilder(testCfg(1, 40, 30))
	if err != nil {
		t.Fatal(err)
	}
	n, last := 0, float32(-1)
	for {
		label, frac, done, err := b.Step()
		if err != nil {
			t.Fatal(err)
		}
		n++
		if done {
			break
		}
		if label == "" || frac <= last {
			t.Fatalf("step %d: label %q frac %v", n, label, frac)
		}
		last = frac
	}
	if n != Steps() {
		t.Fatalf("%d steps, want %d", n, Steps())
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

func TestDebugKeepsFields(t *testing.T) {
	cfg := testCfg(9, 32, 32)
	cfg.Debug = true
	r, err := Generate(cfg)
	if err != nil || r.Fields == nil || len(r.Fields.Elevation) != 32*32 {
		t.Fatalf("debug fields missing: %v", err)
	}
}
