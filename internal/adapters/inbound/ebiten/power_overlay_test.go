package ebiten

import "testing"

// TestArchCategoriesHaveTools: every category in archCategories gets a
// button, so each must hold tools; the PLAY-021 Power and Temperature
// categories join the list with their first tools.
func TestArchCategoriesHaveTools(t *testing.T) {
	t.Parallel()
	listed := map[string]bool{}
	for _, c := range archCategories {
		listed[c.id] = true
		if len(toolsIn(c.id)) == 0 {
			t.Errorf("category %s has no tools", c.id)
		}
	}
	for _, id := range []string{archCatPower, archCatTemperature} {
		if len(toolsIn(id)) > 0 && !listed[id] {
			t.Errorf("%s has tools but no category button", id)
		}
	}
}

// TestGlobalToggleIDsUnique: global toggle ids are unique, and the power
// overlay's id (PLAY-021) belongs to the power toggle only.
func TestGlobalToggleIDsUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, tg := range globalToggleList {
		if seen[tg.id] {
			t.Errorf("toggle id %s twice", tg.id)
		}
		seen[tg.id] = true
		if tg.id == powerButtonID && tg.icon != "overlay_power" {
			t.Errorf("%s is the power overlay's id, not %s's", powerButtonID, tg.icon)
		}
	}
}
