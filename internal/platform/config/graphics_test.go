package config

import (
	"encoding/json"
	"slices"
	"testing"
)

const gib = 1 << 30

func TestDetectTier(t *testing.T) {
	for _, c := range []struct {
		total uint64
		cores int
		want  string
	}{
		{32 * gib, 12, QualityHigh},
		{16 * gib, 8, QualityHigh},
		{16 * gib, 4, QualityMedium}, // memory enough, too few cores
		{8 * gib, 8, QualityMedium},
		{6 * gib, 8, QualityLow},
		{4 * gib, 2, QualityLow},
		{0, 8, QualityMedium}, // unknown memory: the middle
	} {
		if got := DetectTier(c.total, c.cores); got != c.want {
			t.Errorf("DetectTier(%d GiB, %d cores) = %s, want %s", c.total/gib, c.cores, got, c.want)
		}
	}
}

func TestApplyTierSetsEveryKnobAndKeepsChoices(t *testing.T) {
	g := Defaults().Graphics
	g.ApplyTier(QualityLow, 2)
	if g.Quality != QualityLow || g.Effects != EffectsMinimal || g.Shadows || g.Lighting ||
		g.Water != WaterSimple || g.Grass != GrassSparse || g.HiDPI || g.VSync || g.FrameCap != FrameCapDefault {
		t.Fatalf("low tier = %+v", g)
	}
	g = Defaults().Graphics
	g.ApplyTier(QualityMedium, 2)
	if g.Effects != EffectsReduced || !g.Shadows || !g.Lighting || g.Water != WaterSimple || g.Grass != GrassReduced || !g.HiDPI {
		t.Fatalf("medium tier at device scale 2 = %+v", g)
	}
	g.ApplyTier(QualityMedium, 3)
	if g.HiDPI {
		t.Fatal("medium tier renders at device resolution only up to scale 2")
	}
	// a row the player set by hand is kept, and the preset reads Custom
	g = Defaults().Graphics
	g.Shadows = true
	g.Choose("shadows")
	g.ApplyTier(QualityLow, 1)
	if !g.Shadows || g.Quality != QualityCustom || g.Effects != EffectsMinimal {
		t.Fatalf("a chosen row was overwritten or the preset kept its name: %+v", g)
	}
	if !slices.Equal(g.Chosen, []string{"shadows"}) {
		t.Fatalf("chosen %v", g.Chosen)
	}
}

// A settings file from before PLAY-019 has the old defaults; whatever the
// player had changed from them counts as chosen, so detection keeps it.
func TestMarkChosenFromOldDefaults(t *testing.T) {
	g := Defaults().Graphics
	g.Effects, g.VSync, g.HiDPI = EffectsReduced, true, true
	g.MarkChosenFromOldDefaults()
	if !slices.Equal(g.Chosen, []string{"effects"}) {
		t.Fatalf("chosen %v, want just effects", g.Chosen)
	}
}

func TestNormalizeGraphicsChoices(t *testing.T) {
	s := Defaults()
	s.Graphics.Quality, s.Graphics.Water, s.Graphics.Grass = "ultra", "wavy", "lush"
	s.Graphics.Chosen = []string{"shadows", "shadows", "bogus", "water"}
	n := s.Normalize()
	if n.Graphics.Quality != QualityHigh || n.Graphics.Water != WaterAnimated || n.Graphics.Grass != GrassFull {
		t.Fatalf("unknown choices not reset: %+v", n.Graphics)
	}
	if !slices.Equal(n.Graphics.Chosen, []string{"shadows", "water"}) {
		t.Fatalf("chosen %v", n.Graphics.Chosen)
	}
}

// PERF-002: the map's render scale by preset. High and Low draw every
// pixel; Medium draws 3/4 on a 2x or denser screen, all of them below.
func TestApplyTierRenderScale(t *testing.T) {
	for _, c := range []struct {
		tier string
		dev  float64
		want float64
	}{
		{QualityHigh, 1, 1}, {QualityHigh, 2, 1},
		{QualityMedium, 1, 1}, {QualityMedium, 1.5, 1}, {QualityMedium, 2, 0.75},
		{QualityLow, 1, 1}, {QualityLow, 2, 1},
	} {
		g := Defaults().Graphics
		g.ApplyTier(c.tier, c.dev)
		if g.RenderScale != c.want || g.Quality != c.tier {
			t.Errorf("%s at device scale %g: render scale %g quality %s, want %g %s", c.tier, c.dev, g.RenderScale, g.Quality, c.want, c.tier)
		}
	}
	// chosen by hand: kept, and the preset reads Custom
	g := Defaults().Graphics
	g.RenderScale = 0.5
	g.Choose("renderscale")
	g.ApplyTier(QualityHigh, 2)
	if g.RenderScale != 0.5 || g.Quality != QualityCustom {
		t.Fatalf("a chosen render scale was overwritten: %g %s", g.RenderScale, g.Quality)
	}
}

func TestNormalizeRenderScale(t *testing.T) {
	for _, v := range []float64{0, 0.6, 2, -1} {
		s := Defaults()
		s.Graphics.RenderScale = v
		if got := s.Normalize().Graphics.RenderScale; got != 1 {
			t.Errorf("render scale %g normalized to %g, want 1", v, got)
		}
	}
	for _, v := range RenderScales {
		s := Defaults()
		s.Graphics.RenderScale = v
		if got := s.Normalize().Graphics.RenderScale; got != v {
			t.Errorf("render scale %g normalized to %g", v, got)
		}
	}
	// a file from before PERF-002 has no render_scale: every pixel
	var old Settings
	if err := json.Unmarshal([]byte(`{"graphics":{"quality":"medium","hidpi":true}}`), &old); err != nil {
		t.Fatal(err)
	}
	if got := old.Normalize().Graphics.RenderScale; got != 1 {
		t.Fatalf("an old file's render scale = %g, want 1", got)
	}
	s := Defaults()
	s.Graphics.Chosen = []string{"renderscale"}
	if n := s.Normalize(); !slices.Equal(n.Graphics.Chosen, []string{"renderscale"}) {
		t.Fatalf("chosen %v lost renderscale", n.Graphics.Chosen)
	}
}

// TestFrameCapDefaults (PERF-002): every preset caps at 120 with VSync off;
// an older file keeps VSync only when the player chose it; unknown caps
// fall back.
func TestFrameCapDefaults(t *testing.T) {
	t.Parallel()
	for _, tier := range QualityLevels {
		g := Defaults().Graphics
		g.ApplyTier(tier, 2)
		if g.VSync || g.FrameCap != FrameCapDefault || g.Quality != tier {
			t.Fatalf("%s: vsync %v cap %d quality %s", tier, g.VSync, g.FrameCap, g.Quality)
		}
	}
	old := Settings{Version: 5, Graphics: Graphics{VSync: true, Effects: EffectsFull, Quality: QualityHigh,
		Shadows: true, Lighting: true, Water: WaterAnimated, Grass: GrassFull, HiDPI: true}}
	if n := old.Normalize(); n.Graphics.VSync || n.Graphics.FrameCap != FrameCapDefault {
		t.Fatalf("a version-5 file: vsync %v cap %d, want off and %d", n.Graphics.VSync, n.Graphics.FrameCap, FrameCapDefault)
	}
	old.Graphics.Chosen = []string{"vsync"}
	if n := old.Normalize(); !n.Graphics.VSync {
		t.Fatal("a version-5 file with VSync chosen by hand must keep it")
	}
	s := Defaults()
	s.Graphics.FrameCap = 75
	if n := s.Normalize(); n.Graphics.FrameCap != FrameCapDefault {
		t.Fatalf("an unknown cap normalised to %d", n.Graphics.FrameCap)
	}
	s.Graphics.FrameCap = 0
	if n := s.Normalize(); n.Graphics.FrameCap != 0 {
		t.Fatalf("Off (0) must survive: %d", n.Graphics.FrameCap)
	}
}
