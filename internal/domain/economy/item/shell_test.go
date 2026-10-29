package item

import "testing"

func TestShellKinds(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"mortar shells", "incendiary shells", "EMP shells", "smoke shells"} {
		k, ok := ParseKind(n)
		if !ok || !k.IsShell() || k.Category() != "Manufactured" || k.Value() <= 0 || k.IsWeapon() || HasQuality(k) {
			t.Errorf("%q: kind %d ok %v", n, k, ok)
		}
	}
	if Steel.IsShell() || AdvancedHelmet.IsShell() {
		t.Fatal("not shells")
	}
}
