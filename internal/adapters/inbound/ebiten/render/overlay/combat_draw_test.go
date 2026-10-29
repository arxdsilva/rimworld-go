package overlay

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

func TestShotLinesFactors(t *testing.T) {
	t.Parallel()
	got := ShotLines(ShotInfo{Chance: 62, Factors: []ShotFactor{{"aim", 10}, {"cover", -18}}})
	want := []string{"Hit chance 62%", "+10% aim", "-18% cover"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q, want %q", got, want)
	}
}

func TestShotLinesBlockedShowsReason(t *testing.T) {
	t.Parallel()
	if got := ShotLines(ShotInfo{Blocked: true, Reason: "wall in the way"}); len(got) != 1 || got[0] != "Blocked: wall in the way" {
		t.Fatalf("%q", got)
	}
	if got := ShotLines(ShotInfo{Blocked: true}); got[0] != "Blocked: no line of fire" {
		t.Fatalf("%q", got)
	}
}

func TestCoverAlpha(t *testing.T) {
	t.Parallel()
	if CoverAlpha(0) != 0 || CoverAlpha(4) != 0 {
		t.Fatal("little cover is not marked")
	}
	if CoverAlpha(50) >= CoverAlpha(75) || CoverAlpha(500) != CoverAlpha(100) {
		t.Fatal("alpha rises with cover and caps at 100")
	}
}

func TestCoverToggleGatesDraw(t *testing.T) {
	t.Parallel()
	c := &Combat{}
	c.Cover.Toggle()
	if !c.Cover.Shown() {
		t.Fatal("toggle on")
	}
	c.Cover.Toggle()
	c.DrawCover(&render.View{}, nil, []CoverCell{{1, 1, 50}}) // off: must not touch dst
}

func TestProjectileSegmentTrailsBehindHead(t *testing.T) {
	t.Parallel()
	v := &render.View{Cam: camera.Camera{Zoom: 1}}
	ax, _, bx, _ := ProjectileSegment(v, ProjectileMark{X: 5, Y: 5, DX: 1, Kind: "bullet"})
	if ax >= bx {
		t.Fatalf("tail %v should lie behind head %v for heading +x", ax, bx)
	}
	sx, _, ex, _ := ProjectileSegment(v, ProjectileMark{X: 5, Y: 5, DX: 1, Kind: "shell"})
	if ex-sx >= bx-ax {
		t.Fatal("a shell is shorter than a bullet")
	}
}

func TestNothingToDrawIsNoOp(t *testing.T) {
	t.Parallel()
	c := &Combat{}
	c.DrawProjectiles(nil, nil, nil)
	c.DrawShotFeedback(nil, nil, nil, 0, 0)
	c.DrawWeaponRange(nil, nil, 0, 0, WeaponReach{})
}
