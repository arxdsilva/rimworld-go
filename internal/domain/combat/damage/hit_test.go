package damage

import "testing"

func TestFormsTypesAndWindows(t *testing.T) {
	t.Parallel()
	for f, want := range map[Form]Type{FormBullet: Sharp, FormArrow: Sharp, FormStab: Sharp, FormBite: Sharp,
		FormBlunt: Blunt, FormCrush: Blunt, FormFlame: Heat, FormBurn: Burn, FormBomb: Bomb, FormEMP: EMP, FormStun: Stun, Form(99): Blunt} {
		if f.Type() != want {
			t.Errorf("%v is %v", f, f.Type())
		}
	}
	for _, ty := range []Type{Sharp, Blunt, Heat, Bomb, Burn, EMP, Stun} {
		if FormOf(ty).Type() != ty {
			t.Errorf("FormOf(%v) = %v", ty, FormOf(ty))
		}
	}
	if h := Of(Bomb, 50, 100); h.Form != FormBomb || h.Type() != Bomb || h.Amount != 50 || h.Pen != 100 {
		t.Fatalf("Of %+v", h)
	}
	if FormBullet.String() != "bullet" || Form(99).String() != "unknown" {
		t.Fatal("names")
	}
	// wiki windows: cut and bite 0-10 %, bullet 0-70 %, blunt and stab 40-100 %
	for f, want := range map[Form][2]int{FormCut: {0, 100}, FormBite: {0, 100}, FormBullet: {0, 700}, FormArrow: {0, 700},
		FormScratch: {0, 700}, FormBlunt: {400, 1000}, FormStab: {400, 1000}, FormCrush: {400, 1000}, Form(99): {0, 700}} {
		if lo, hi := f.Overkill(); lo != want[0] || hi != want[1] {
			t.Errorf("%v window %d-%d", f, lo, hi)
		}
	}
	if DestroyChance(399, 400, 1000) != 0 || DestroyChance(400, 400, 1000) != 0 || DestroyChance(700, 400, 1000) != 500 ||
		DestroyChance(1000, 400, 1000) != 1000 || DestroyChance(5000, 0, 700) != 1000 {
		t.Fatal("DestroyChance")
	}
	if !Staggers(1000, 1000) || Staggers(500, 1000) || Staggers(0, 0) || !Staggers(500, 250) {
		t.Fatal("Staggers")
	}
}
