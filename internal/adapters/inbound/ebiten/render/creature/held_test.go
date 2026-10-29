package creature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
)

// TestHeldWeaponKeyCoversEveryWeapon (DEF-2): every equippable item's
// weapon (by its def name, as PawnView.Weapon carries it) has a held
// sprite with metadata, and every weapon sprite is held by one.
func TestHeldWeaponKeyCoversEveryWeapon(t *testing.T) {
	t.Parallel()
	const meta = "../../../../../../assets/metadata/"
	held := map[string]bool{}
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		if !k.IsWeapon() {
			continue
		}
		name := weapon.DefOf(weapon.ForItem(k)).Name
		key, ok := HeldWeaponKey(name)
		if !ok {
			t.Errorf("%s (%q) has no held sprite", k, name)
			continue
		}
		if _, err := os.Stat(meta + key + ".yaml"); err != nil {
			t.Errorf("%q -> %s: no metadata", name, key)
		}
		held[key] = true
	}
	files, _ := filepath.Glob(meta + "weapon/*.yaml")
	for _, f := range files {
		if key := "weapon/" + strings.TrimSuffix(filepath.Base(f), ".yaml"); !held[key] {
			t.Errorf("%s is held by no weapon", key)
		}
	}
	for name, want := range map[string]string{"frag grenade": "weapon/frag_grenade", "molotov cocktail": "weapon/molotov_cocktail",
		"EMP grenade": "weapon/emp_grenade"} {
		if got, _ := HeldWeaponKey(name); got != want {
			t.Errorf("%q -> %q, want %q", name, got, want)
		}
	}
	if _, ok := HeldWeaponKey("fists"); ok {
		t.Error("fists hold nothing")
	}
}
