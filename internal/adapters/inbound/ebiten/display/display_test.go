package display

import (
	"image"
	"testing"
)

func TestParseSize(t *testing.T) {
	for in, want := range map[string][2]int{"1600x900": {1600, 900}, "2560x1440": {2560, 1440}} {
		if w, h, ok := parseSize(in); !ok || w != want[0] || h != want[1] {
			t.Errorf("%q: %d %d %v", in, w, h, ok)
		}
	}
	for _, in := range []string{"", "x", "0x900", "big"} {
		if _, _, ok := parseSize(in); ok {
			t.Errorf("%q parsed", in)
		}
	}
}

func TestLoadIconsFromRepoAssets(t *testing.T) {
	icons, err := LoadIcons()
	if err != nil {
		t.Fatal(err)
	}
	if len(icons) != len(iconSizes) {
		t.Fatalf("%d icons", len(icons))
	}
	for i, n := range iconSizes {
		if b := icons[i].Bounds(); b.Dx() != n || b.Dy() != n {
			t.Errorf("icon %d is %v", n, b)
		}
	}
}

func TestLoadIconsFromMissingDir(t *testing.T) {
	if _, err := loadIconsFrom(t.TempDir()); err == nil {
		t.Error("want an error for a folder without icons")
	}
}

func TestApplyIconsCallsThroughOnlyWhenNonEmpty(t *testing.T) {
	calls := 0
	set := func([]image.Image) { calls++ }
	applyIcons(nil, set)
	if calls != 0 {
		t.Fatal("empty icons must not be set")
	}
	applyIcons([]image.Image{image.NewRGBA(image.Rect(0, 0, 16, 16))}, set)
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}
