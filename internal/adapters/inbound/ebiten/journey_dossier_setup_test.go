package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
)

// The card's setup dialog (RHUD-054…056): opened by the card's gear, with the
// tabs Look, Sections, Presets and Colours. The journeys drive it with the real
// mouse and keys.

// dossierSetup opens the setup dialog from the gear of a docked card.
func dossierSetup(t *testing.T) *journey {
	t.Helper()
	j, _ := dossierPawn(t, true)
	j.draw(1)
	var cx, cy float32
	for _, w := range j.a.uiSt().Cur {
		if strings.HasPrefix(w.ID, "dossier.d.") {
			cx, cy = w.R.Center() // any row of the card
			break
		}
	}
	sc := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(cx*sc), int(cy*sc) // the gear shows while the pointer is over the card
	j.update(2)
	j.clickID(ids.DossierGear)
	j.update(2)
	if j.a.uiModalTop() != "dossier.setup" {
		t.Fatalf("the gear opened %q, want the setup dialog", j.a.uiModalTop())
	}
	j.draw(1)
	return j
}

// typeInto replaces the text of the focused field and commits it with Enter.
func typeInto(j *journey, id, s string) {
	j.t.Helper()
	j.clickID(id)
	j.update(1)
	j.draw(1)
	if !j.a.uiSt().Editing() {
		j.t.Fatalf("%s is not being edited", id)
	}
	j.key("End", 0) // the click put the caret where it landed
	for range 12 {
		j.key("Backspace", 0)
	}
	j.typeText(s)
	j.update(1)
	j.key("Enter", 0)
	j.update(2)
}

func TestJourneyDossierColourEdit(t *testing.T) {
	j := dossierSetup(t)
	if len(j.a.settings.HUD.Colors) != 0 {
		t.Fatalf("colours start as %v", j.a.settings.HUD.Colors)
	}
	j.clickID("dossier.setup.tabs.3")
	j.update(2)
	j.draw(1)
	j.clickID("dossier.setup.col.bar_main")
	j.update(2)
	typeInto(j, "dossier.setup.t.hex", "#336699FF")
	if got := j.a.settings.HUD.Colors["bar_main"]; got != "#336699FF" {
		t.Fatalf("bar_main = %q, want #336699FF (colours %v)", got, j.a.settings.HUD.Colors)
	}
	if j.a.settings.HUD.Preset != "" {
		t.Fatalf("an edited card still names the preset %q", j.a.settings.HUD.Preset)
	}
	j.draw(2) // the card and the dialog paint with the new colour

	// a code that is not a colour is refused and the dialog says so
	typeInto(j, "dossier.setup.t.hex", "#12")
	if got := j.a.settings.HUD.Colors["bar_main"]; got != "#336699FF" {
		t.Fatalf("a bad code changed the colour to %q", got)
	}

	// lightness moves the colour but keeps the key and goes through the file
	typeInto(j, "dossier.setup.t.light", "80")
	got := j.a.settings.HUD.Colors["bar_main"]
	if got == "" || got == "#336699FF" {
		t.Fatalf("lightness left the colour at %q", got)
	}

	// reset puts the theme's colour back, and only that one
	j.clickID("dossier.setup.col.text")
	j.update(2)
	typeInto(j, "dossier.setup.t.hex", "#FF0000FF")
	if len(j.a.settings.HUD.Colors) != 2 {
		t.Fatalf("colours %v, want two differences", j.a.settings.HUD.Colors)
	}
	j.clickID("dossier.setup.col.reset1")
	j.update(2)
	if _, ok := j.a.settings.HUD.Colors["text"]; ok || len(j.a.settings.HUD.Colors) != 1 {
		t.Fatalf("after reset one: %v", j.a.settings.HUD.Colors)
	}
	j.clickID("dossier.setup.col.resetall")
	j.update(2)
	if len(j.a.settings.HUD.Colors) != 0 {
		t.Fatalf("after reset all: %v", j.a.settings.HUD.Colors)
	}

	// Escape closes the dialog
	j.key("Escape", 0)
	j.update(1)
	if j.a.uiModalTop() != "" {
		t.Fatalf("Escape left %q open", j.a.uiModalTop())
	}
}

func TestJourneyDossierBackToDefaults(t *testing.T) {
	j := dossierSetup(t)
	if j.clickID("modal.dossier.setup.defaults") {
		t.Fatal("back to defaults is live with nothing changed")
	}
	// a look change, a colour and a size
	j.clickID("dossier.setup.f.restyle")
	j.update(2)
	h := j.a.settings.HUD
	if h.Pane.Restyle || h.Docked {
		t.Fatalf("restyle off left restyle=%v docked=%v", h.Pane.Restyle, h.Docked)
	}
	j.clickID("dossier.setup.tabs.3")
	j.update(2)
	typeInto(j, "dossier.setup.t.hex", "#00FF00FF")
	if len(j.a.settings.HUD.Colors) != 1 {
		t.Fatalf("colours %v", j.a.settings.HUD.Colors)
	}
	j.draw(1)

	if !j.clickID("modal.dossier.setup.defaults") {
		t.Fatal("back to defaults is off after edits")
	}
	j.update(2)
	if j.a.uiModalTop() != "dossier.setup.confirm" {
		t.Fatalf("top modal %q, want the confirm", j.a.uiModalTop())
	}
	if len(j.a.settings.HUD.Colors) != 1 {
		t.Fatal("defaults applied before the confirm")
	}
	j.draw(1)
	j.clickID("modal.dossier.setup.confirm.ok")
	j.update(2)
	h = j.a.settings.HUD
	if !h.Pane.Restyle || !h.Docked || len(h.Colors) != 0 || !h.Enabled {
		t.Fatalf("after defaults: restyle=%v docked=%v colours=%v enabled=%v", h.Pane.Restyle, h.Docked, h.Colors, h.Enabled)
	}
	if j.a.uiModalTop() != "dossier.setup" {
		t.Fatalf("top modal %q, want the setup again", j.a.uiModalTop())
	}
	j.draw(1)
	if j.clickID("modal.dossier.setup.defaults") {
		t.Fatal("back to defaults is live again at the defaults")
	}
}

// TestJourneyDossierOptionsRows: the card's rows in Options > Accessibility.
func TestJourneyDossierOptionsRows(t *testing.T) {
	j := newJourney(t)
	if !j.a.settings.HUD.Enabled {
		t.Fatal("the card ships on")
	}
	open := func() {
		j.key("Escape", 0)
		j.clickID("pause.options")
		j.clickID("options.tab.access")
		j.draw(1)
	}
	open()
	j.clickID("options.card")
	if j.a.settings.HUD.Enabled {
		t.Fatal("the Colonist card row did not turn the card off")
	}
	j.clickID("options.card")
	if !j.a.settings.HUD.Enabled {
		t.Fatal("the Colonist card row did not turn the card on")
	}
	before := j.a.settings.HUD.Pane.Height
	j.clickID("options.cardheight.inc")
	if got := j.a.settings.HUD.Pane.Height; got != before+20 {
		t.Fatalf("pane height %d, want %d", got, before+20)
	}
	if j.clickID("options.cardspot") {
		t.Fatal("the floating place is live while the card is docked")
	}
	j.clickID("options.carddock")
	if j.a.settings.HUD.Docked {
		t.Fatal("the placement row did not let the card float")
	}
	j.clickID("options.cardspot")
	if got := j.a.settings.HUD.Card.Anchor; got != 7 {
		t.Fatalf("anchor %d, want 7 (the default 6, then the next)", got)
	}
	if j.clickID("options.cardheight.inc") {
		t.Fatal("pane height is live for a floating card")
	}
}

// TestCaptureDossierSetup: each -capture-ui dossier-setup state opens the
// dialog on its tab and paints.
func TestCaptureDossierSetup(t *testing.T) {
	for i, ui := range []string{"dossier-setup", "dossier-setup-sections", "dossier-setup-presets", "dossier-setup-colours"} {
		t.Run(ui, func(t *testing.T) {
			a, f := captureColony(t, ui, 60)
			f.Frame(t, a)
			if a.uiModalTop() != "dossier.setup" {
				t.Fatalf("top modal %q, want the setup dialog", a.uiModalTop())
			}
			if a.dos.setup == nil || !a.settings.HUD.Enabled {
				t.Fatal("the capture left the card off")
			}
			if _, ok := a.uiSt().Find("dossier.setup.tabs." + string(rune('0'+i))); !ok {
				t.Fatalf("no tab %d", i)
			}
			drawFrame(t, a, f)
		})
	}
}
