package dossier

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// sized is a fakeShell whose screen has another size.
type sized struct {
	*fakeShell
	w, h int
}

func (z *sized) Size() (int, int) { return z.w, z.h }

// setupRig is a fakeShell with the setup modal open.
type setupRig struct {
	*fakeShell
	su *Setup
}

func newSetupRig(t *testing.T, w, h int) *setupRig {
	t.Helper()
	s := newShell(t)
	z := &sized{s, w, h}
	s.kit = uikit.New(z)
	s.panel = New(s, s, s, z, s.kit, nil, s)
	r := &setupRig{fakeShell: s, su: NewSetup(s.panel, s.kit, s, s, nil, nil, "", z)}
	r.su.Open()
	r.frame()
	return r
}

// frame is one Update: begin the kit, declare the modal.
func (r *setupRig) frame() {
	r.n++
	r.f = inputframe.NewFrame(inputframe.Raw{Mx: r.mx, My: r.my})
	r.kit.Begin()
	r.kit.RunModals(nil)
}

// click presses a widget of the modal and lets the next frames see it.
func (r *setupRig) click(id string) {
	r.t.Helper()
	if !r.kit.Click(id) {
		r.t.Fatalf("no widget %q to click", id)
	}
	r.frame()
	r.frame()
}

func (r *setupRig) widget(id string) uikit.Widget {
	r.t.Helper()
	w, ok := r.kit.Find(id)
	if !ok {
		r.t.Fatalf("no widget %q", id)
	}
	return w
}

// reveal scrolls the Look tab until the widget id is declared.
func (r *setupRig) reveal(id string) {
	r.t.Helper()
	for off := float32(0); off < 3000; off += 40 {
		r.kit.Scroll["dossier.setup.look"] = off
		r.frame()
		if _, ok := r.kit.Find(id); ok {
			return
		}
	}
	r.t.Fatalf("%q never came into view", id)
}

func (r *setupRig) tab(i int) {
	r.su.tab = i
	r.frame()
	r.frame()
}

func TestSetupRestyleOffGreysTheGroup(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	if w := r.widget("dossier.setup.f.docked"); w.Disabled {
		t.Fatalf("docking is off while the pane restyles: %q", w.Reason)
	}
	r.click("dossier.setup.f.restyle")
	if r.set.HUD.Pane.Restyle {
		t.Fatal("the switch did not turn Restyle off")
	}
	for _, id := range []string{"dossier.setup.f.docked", "dossier.setup.f.mintabs", "dossier.setup.f.tabw", "dossier.setup.f.emblem", "dossier.setup.f.height"} {
		w, ok := r.kit.Find(id)
		if !ok {
			continue // scrolled out of the view
		}
		if !w.Disabled || w.Reason != whyNoRestyle {
			t.Errorf("%s: disabled=%v reason=%q, want the restyle reason", id, w.Disabled, w.Reason)
		}
	}
	if w := r.widget("dossier.setup.f.docked"); !w.Disabled {
		t.Error("the dock switch stays live with a plain pane")
	}
}

func TestSetupEditsAreCopiesAndInvalidate(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	r.set.HUD.Colors = map[string]string{"text": "#112233FF"}
	before := r.set.HUD.Colors
	r.su.Edit(func(h *config.HUD) { h.Colors["text"] = "#445566FF"; h.Preset = "" })
	if before["text"] != "#112233FF" {
		t.Fatal("an edit changed the stored colour map in place")
	}
	if r.set.HUD.Colors["text"] != "#445566FF" {
		t.Fatalf("the edit did not arrive: %v", r.set.HUD.Colors)
	}
	r.set.HUD.Preset = "Roomy"
	r.su.Edit(func(h *config.HUD) { h.Card.W = 300 })
	if r.set.HUD.Preset != "" {
		t.Fatal("an edited card still names its preset")
	}
	r.su.Edit(func(h *config.HUD) { h.Preset = "Snug" })
	if r.set.HUD.Preset != "Snug" {
		t.Fatal("a preset load could not set its name")
	}
}

func TestSetupFloatingGroupSaysWhy(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	if !r.set.HUD.Docked {
		t.Fatal("the card docks by default")
	}
	r.reveal("dossier.setup.anchor.4")
	w := r.widget("dossier.setup.anchor.4")
	if !w.Disabled || w.Reason != whyDocked {
		t.Fatalf("anchor while docked: disabled=%v reason=%q", w.Disabled, w.Reason)
	}
	r.click("dossier.setup.f.docked")
	if r.set.HUD.Docked {
		t.Fatal("the dock switch did not float the card")
	}
	if w := r.widget("dossier.setup.anchor.4"); w.Disabled {
		t.Fatalf("a floating card has a place: %q", w.Reason)
	}
	r.click("dossier.setup.anchor.2")
	if r.set.HUD.Card.Anchor != 2 {
		t.Fatalf("anchor = %d, want 2", r.set.HUD.Card.Anchor)
	}
}

func TestSetupLetterGapNeedsCompressAndFloating(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	r.set.HUD.Docked = false
	r.reveal("dossier.setup.f.gap")
	if w := r.widget("dossier.setup.f.gap"); !w.Disabled || w.Reason != whyNoCompact {
		t.Fatalf("gap with compress off: disabled=%v reason=%q", w.Disabled, w.Reason)
	}
	r.click("dossier.setup.f.compress")
	r.reveal("dossier.setup.f.gap")
	if w := r.widget("dossier.setup.f.gap"); w.Disabled {
		t.Fatalf("gap with compress on and a floating card: %q", w.Reason)
	}
	r.set.HUD.Docked = true
	r.reveal("dossier.setup.f.gap")
	if w := r.widget("dossier.setup.f.gap"); !w.Disabled {
		t.Fatal("gap stays live while docked")
	}
}

func TestSetupColourEditReset(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	r.tab(tabColours)
	r.click("dossier.setup.col.bar_main")
	if r.su.col.sel != "bar_main" {
		t.Fatalf("selected %q", r.su.col.sel)
	}
	if w := r.widget("dossier.setup.col.reset1"); !w.Disabled {
		t.Fatal("reset on an untouched colour should say there is nothing to reset")
	}
	r.su.setColour("bar_main", rgbaOf(10, 20, 30))
	r.frame()
	if got := r.set.HUD.Colors["bar_main"]; got != "#0A141EFF" {
		t.Fatalf("stored %q", got)
	}
	def, _ := uikit.DossierColor("bar_main")
	r.su.setColour("bar_main", def)
	if len(r.set.HUD.Colors) != 0 {
		t.Fatalf("the theme's own colour was stored: %v", r.set.HUD.Colors)
	}
	r.su.setColour("text", rgbaOf(1, 2, 3))
	r.su.setColour("bar_low", rgbaOf(4, 5, 6))
	r.frame()
	r.click("dossier.setup.col.reset1")
	if _, ok := r.set.HUD.Colors["bar_main"]; ok {
		t.Fatal("reset left the selected colour")
	}
	r.click("dossier.setup.col.resetall")
	if len(r.set.HUD.Colors) != 0 {
		t.Fatalf("reset all left %v", r.set.HUD.Colors)
	}
}

func TestSetupHSLKeepsHueAtGrey(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	r.su.col.hsl = [3]float64{200, 0, 0.5}
	grey := sheet.HSLToRGB(200, 0, 0.5, 255)
	r.su.col.made = grey
	if h, _, _ := r.su.colourHSL(grey); h != 200 {
		t.Fatalf("hue = %v, want the kept 200", h)
	}
	if h, s, _ := r.su.colourHSL(rgbaOf(255, 0, 0)); h != 0 || s != 1 {
		t.Fatalf("a new colour must be read again, got %v %v", h, s)
	}
}

func TestSetupBackToDefaults(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	if r.su.Changed() {
		t.Fatal("a fresh card is changed")
	}
	if w := r.widget("modal.dossier.setup.defaults"); !w.Disabled || w.Reason == "" {
		t.Fatalf("defaults with nothing changed: %+v", w)
	}
	r.su.Edit(func(h *config.HUD) {
		h.Card.W = 640
		h.Colors = map[string]string{"text": "#FF0000FF"}
		h.Text.Decimals = true
	})
	r.frame()
	if !r.su.Changed() {
		t.Fatal("edits are not seen as changes")
	}
	r.click("modal.dossier.setup.defaults")
	if r.kit.Top() != "dossier.setup.confirm" {
		t.Fatalf("top modal = %q, want the confirm", r.kit.Top())
	}
	if r.set.HUD.Card.W != 640 {
		t.Fatal("defaults applied before the confirm")
	}
	r.click("modal.dossier.setup.confirm.ok")
	want := config.Defaults().HUD
	want.Enabled = r.set.HUD.Enabled
	if r.su.Changed() || r.set.HUD.Card.W != want.Card.W || r.set.HUD.Colors != nil || r.set.HUD.Text.Decimals {
		t.Fatalf("not back to defaults: %+v", r.set.HUD)
	}
	if !r.set.HUD.Enabled {
		t.Fatal("back to defaults must not switch the card off")
	}
}

func TestSetupEscapeCloses(t *testing.T) {
	r := newSetupRig(t, 1280, 720)
	if r.kit.Top() != setupID {
		t.Fatalf("top = %q", r.kit.Top())
	}
	r.kit.CloseTop()
	if r.kit.ModalOpen() {
		t.Fatal("the modal stays open")
	}
}

// TestSetupBodiesDrawInsideTheFrame: at the smallest screens every control of
// every tab is inside the dialog's body (the scroll views keep the rest out).
func TestSetupBodiesDrawInsideTheFrame(t *testing.T) {
	for _, sc := range []struct {
		name string
		w, h int
	}{{"1113x626", 1113, 626}, {"x1.5", 742, 417}, {"960x540", 960, 540}} {
		for tab, name := range setupTabLabels {
			t.Run(sc.name+"/"+name, func(t *testing.T) {
				r := newSetupRig(t, sc.w, sc.h)
				r.tab(tab)
				frame, _, body, _ := uikit.ModalGeometry(sc.w, sc.h, uikit.Modal{W: setupW, H: setupH, Buttons: make([]uikit.ModalButton, 2)})
				if frame.W > float32(sc.w) || frame.H > float32(sc.h) {
					t.Fatalf("frame %v outside the screen", frame)
				}
				n := 0
				for _, w := range r.kit.Cur {
					if len(w.ID) < 6 || w.ID[:6] != "dossie" {
						continue
					}
					n++
					if w.R.X < body.X-0.5 || w.R.Y < body.Y-0.5 || w.R.Right() > body.Right()+0.5 || w.R.Bottom() > body.Bottom()+0.5 {
						t.Errorf("%s at %v is outside the body %v", w.ID, w.R, body)
					}
				}
				if n == 0 {
					t.Fatal("the tab declared nothing")
				}
				img := ebiten.NewImage(sc.w, sc.h)
				r.kit.Paint(img)
			})
		}
	}
}
