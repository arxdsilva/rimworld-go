package ebiten

import "testing"

// TestItemSpriteFallbackUsesUnderscores: kinds with spaces find their
// sprite (item names have spaces, sprite names underscores).
func TestItemSpriteFallbackUsesUnderscores(t *testing.T) {
	for kind, want := range map[string]string{
		"servo arm":   "item/servo_arm",
		"nanomed kit": "item/nanomed_kit",
		"peg leg":     "item/peg_leg",
		"kidney":      "item/kidney",
	} {
		if got := itemSprite(kind); got != want {
			t.Errorf("itemSprite(%q) = %q, want %q", kind, got, want)
		}
	}
}
