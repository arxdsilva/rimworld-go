package infocard

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
)

type fakeShell struct {
	kit    *uikit.Kit
	f      inputframe.Frame
	pawns  []game.PawnView
	w      game.WorldView
	sw, sh int
}

type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return -1, -1 }
func (h fakeHost) Size() (int, int)         { return h.s.sw, h.s.sh }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(string)              {}
func (h fakeHost) ReducedMotion() bool      { return true }

func (s *fakeShell) World() game.WorldView  { return s.w }
func (s *fakeShell) Pawns() []game.PawnView { return s.pawns }
func (s *fakeShell) MapSize() (int, int)    { return 0, 0 }
func (s *fakeShell) Tick() clock.Tick       { return 0 }
func (s *fakeShell) Date() clock.Date       { return clock.Date{} }
func (s *fakeShell) Kit() *uikit.Kit        { return s.kit }

func newFake() (*fakeShell, *Window) {
	s := &fakeShell{sw: 1280, sh: 720}
	s.kit = uikit.New(fakeHost{s})
	return s, New(s)
}

// text is every row and the description, flattened.
func text(c Card) string {
	var b strings.Builder
	b.WriteString(c.Desc + "\n" + c.Sub + "\n")
	for _, r := range c.Rows {
		b.WriteString(r.Label + ": " + r.Value + "\n")
	}
	return b.String()
}

func wants(t *testing.T, c Card, parts ...string) {
	t.Helper()
	got := text(c)
	for _, p := range parts {
		if !strings.Contains(got, p) {
			t.Errorf("card %q lacks %q:\n%s", c.Title, p, got)
		}
	}
}

func pawnWithGear() game.PawnView {
	flak := armor.Set{Sharp: 1000, Blunt: 360, Heat: 270}
	return game.PawnView{
		ID: 7, Name: "Ada",
		Apparel:      []string{"flak vest"},
		Worn:         []craft.WornView{{Kind: "flak vest", Quality: "good", HP: 30, MaxHP: 60}},
		ApparelArmor: []armor.Set{flak},
		Weapon:       "rifle", WeaponStats: weapon.Stats(weapon.Rifle),
		Spare: "knife",
		Pack:  []game.PackItemView{{ID: 3, Name: "rice", Qty: 4, Gear: gearview.Pack{Grams: 1200}}, {ID: 4, Name: "beer", Qty: 1}},
	}
}

func TestApparelCard(t *testing.T) {
	s, _ := newFake()
	s.pawns = []game.PawnView{pawnWithGear()}
	c := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeApparel, Index: 0})
	wants(t, c, "Mass: 6.00 kg", "Quality: good", "Hit points: 30 / 60 (50%)", "Sharp: 100%", "Blunt: 36%", "Heat: 27%",
		"Covers: torso", "Layers: middle", "Insulation", "Move speed: -3%", "carried by Ada")
	if c.Desc != wordsApparel {
		t.Fatalf("description %q", c.Desc)
	}
}

func TestWeaponCard(t *testing.T) {
	s, _ := newFake()
	s.pawns = []game.PawnView{pawnWithGear()}
	c := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeWeapon})
	wants(t, c, "Mass: 4.20 kg", "Damage:", "Armour penetration:", "Range:", "Accuracy:", "Cooldown:")
	sp := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeSpare})
	wants(t, sp, "Mass: 500 g", "Reach: adjacent")
}

func TestFoodCard(t *testing.T) {
	s, _ := newFake()
	s.pawns = []game.PawnView{pawnWithGear()}
	c := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModePack, Index: 3})
	wants(t, c, "Count: 4", "Nutrition:", "Stack mass:")
	if c.Desc != wordsFood {
		t.Fatalf("description %q", c.Desc)
	}
}

func TestDrugCard(t *testing.T) {
	s, _ := newFake()
	s.pawns = []game.PawnView{pawnWithGear()}
	c := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModePack, Index: 4})
	if c.Desc != wordsDrug {
		t.Fatalf("description %q", c.Desc)
	}
	wants(t, c, "Kind: drug")
}

func TestResourceCardFromMapItem(t *testing.T) {
	s, _ := newFake()
	s.w.Items = []game.ItemView{{ID: 90, Kind: "steel", Qty: 40, FreshDays: -1}}
	c := Build(s, ports.OpenInfoCard{ID: 90})
	wants(t, c, "Count: 40", "Mass:", "Stack limit:")
	if c.Desc != wordsMaterial || c.Title != "Steel" {
		t.Fatalf("got %q %q", c.Title, c.Desc)
	}
	if strings.Contains(text(c), "Keeps for") || strings.Contains(text(c), "Armour") {
		t.Fatalf("a resource has no shelf life or armour:\n%s", text(c))
	}
}

func TestKindOnlyAndGone(t *testing.T) {
	s, _ := newFake()
	wants(t, Build(s, ports.OpenInfoCard{Kind: "plate armor"}), "Layers:", "Move speed: -17%")
	if c := Build(s, ports.OpenInfoCard{ID: 5}); c.Desc != wordsGone {
		t.Fatalf("a missing item says so: %q", c.Desc)
	}
	if c := Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeApparel, Index: 9}); c.Desc != wordsGone {
		t.Fatalf("a missing row says so: %q", c.Desc)
	}
	if c := Build(s, ports.OpenInfoCard{Kind: "zzz"}); c.Desc != wordsUnknown {
		t.Fatalf("unknown kind: %q", c.Desc)
	}
}

func TestOpenAndClose(t *testing.T) {
	s, w := newFake()
	w.Open(ports.OpenInfoCard{Kind: "rifle"})
	if !w.Opened() {
		t.Fatal("not open")
	}
	w.Open(ports.OpenInfoCard{Kind: "knife"}) // replaced, not stacked
	if n := len(s.kit.Modals); n != 1 {
		t.Fatalf("%d modals", n)
	}
	s.kit.CloseTop()
	if w.Opened() {
		t.Fatal("still open")
	}
}

// TestFitsEveryUIScale: at 75-200 % the window sits inside the screen and
// no line is wider than its column.
func TestFitsEveryUIScale(t *testing.T) {
	s, _ := newFake()
	s.pawns = []game.PawnView{pawnWithGear()}
	cards := []Card{
		Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeApparel}),
		Build(s, ports.OpenInfoCard{Pawn: 7, Mode: ModeWeapon}),
	}
	for _, pc := range []int{75, 100, 125, 150, 200} {
		sw, sh := 1280*100/pc, 720*100/pc
		frame, _, body, footer := uikit.ModalGeometry(sw, sh, uikit.Modal{W: winW, H: winH, Buttons: []uikit.ModalButton{{}}})
		win := gfx.WinRect(sw, sh)
		if frame.X < win.X || frame.Y < win.Y || frame.Right() > win.Right() || frame.Bottom() > win.Bottom() {
			t.Errorf("scale %d%%: frame %v off the %dx%d screen", pc, frame, sw, sh)
		}
		if footer.Y < body.Bottom() {
			t.Errorf("scale %d%%: footer over the body", pc)
		}
		for _, c := range cards {
			ls, h := Layout(c, body.W-uikit.ScrollBarW)
			for _, l := range ls {
				st := l.style()
				if w, _ := gfx.Measure(l.Text, st); float32(w)+l.X > body.W-uikit.ScrollBarW+1 {
					t.Errorf("scale %d%%: %q is %.0f px wide at x=%v in %v", pc, l.Text, w, l.X, body.W)
				}
				if l.Y+l.H > h+0.5 {
					t.Errorf("scale %d%%: line %q ends past the content (%v > %v)", pc, l.Text, l.Y+l.H, h)
				}
			}
		}
	}
}

func ExampleBuild() {
	s, _ := newFake()
	c := Build(s, ports.OpenInfoCard{Kind: "rice"})
	fmt.Println(c.Title, "-", c.Sub)
	// Output: Rice - Food
}
