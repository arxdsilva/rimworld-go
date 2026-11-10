package ebiten

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/options/saves"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The Save dialog (SAV-001…004, SAV-010): the pause menu's Save names the
// save, asks before replacing one, says why a name cannot be used, and
// deletes saves; autosaves follow the colony, not the name typed.

// headerSaves is an in-memory save store that keeps each save's header, as
// the file store does: the list names each save's colony and base.
type headerSaves struct {
	metas   []ports.SaveMeta // every SaveMeta the Save hook was given
	slots   []ports.SaveSlot // on disk, oldest first
	deleted []string
	load    ports.SaveMeta // the header Load returns
}

// put writes meta as Save does: the file is meta.Name's slot key, and a
// later write gets a later modified time.
func (h *headerSaves) put(meta ports.SaveMeta) {
	key := savefmt.SlotKey(meta.Name)
	h.slots = slices.DeleteFunc(h.slots, func(sl ports.SaveSlot) bool { return strings.EqualFold(sl.Name, key) })
	h.slots = append(h.slots, ports.SaveSlot{Name: key, Path: key, Title: meta.Name, Date: "Day 3, " + meta.Season,
		Colonists: 1, Modified: time.Unix(1_700_000_000, 0).Add(time.Duration(len(h.metas)+len(h.slots)) * time.Minute),
		Colony: meta.Colony, ColonyName: meta.ColonyName, Base: meta.Base, Autosave: meta.Autosave, Season: meta.Season})
}

func (h *headerSaves) hooks() SaveHooks {
	return SaveHooks{
		Save: func(_ Simulation, m ports.SaveMeta) error {
			h.metas = append(h.metas, m)
			h.put(m)
			return nil
		},
		List: func() ([]ports.SaveSlot, error) { // newest first, as the file store lists them
			out := slices.Clone(h.slots)
			slices.Reverse(out)
			return out, nil
		},
		Load: func(path string) (Simulation, ports.SaveMeta, error) {
			if h.load.Name == "" {
				return nil, ports.SaveMeta{}, errors.New("not in this journey")
			}
			return game.New(game.Config{Seed: 3, Width: 24, Height: 24}), h.load, nil
		},
		Delete: func(name string) error {
			h.deleted = append(h.deleted, name)
			h.slots = slices.DeleteFunc(h.slots, func(sl ports.SaveSlot) bool { return sl.Name == name })
			return nil
		},
	}
}

// last is the newest SaveMeta the Save hook was given.
func (h *headerSaves) last(t *testing.T) ports.SaveMeta {
	t.Helper()
	if len(h.metas) == 0 {
		t.Fatal("nothing was saved")
	}
	return h.metas[len(h.metas)-1]
}

// dialogJourney is a colony whose saves go to a header-keeping store that
// already holds a save of another colony, "Old Camp".
func dialogJourney(t *testing.T) (*journey, *headerSaves) {
	t.Helper()
	j := newJourney(t)
	h := &headerSaves{}
	h.put(ports.SaveMeta{Name: "Old Camp", Colony: "c-other", ColonyName: "Other Shore", Base: "Old Camp", Season: "Winter"})
	j.a.cfg.Saves = h.hooks()
	j.a.settings.General.AutoPause = config.AutoPauseNever // a long step brings no letter that pauses the clock
	return j, h
}

// openSave opens the Save dialog from the pause menu.
func openSave(j *journey) {
	j.t.Helper()
	j.key("Escape", 0)
	if !j.clickID("pause.save") {
		j.t.Fatal("pause menu Save is disabled")
	}
	j.update(1) // the dialog's first frame gives its field the keyboard
	if top := j.a.uiSt().Top(); top != "save" {
		j.t.Fatalf("Save opened %q, want the Save dialog", top)
	}
}

// saveOK is the dialog's Save button as the kit last declared it.
func saveOK(j *journey) (disabled bool, reason string) {
	j.t.Helper()
	j.update(1)
	w, ok := j.a.uiSt().Find("modal.save.ok")
	if !ok {
		j.t.Fatal("the Save dialog has no Save button")
	}
	return w.Disabled, w.Reason
}

// TestJourneySavesSaveAsNewName: Save opens the dialog over the pause menu
// (a click on the pause menu under it does nothing); the name typed over
// the prefilled one is the save written, with the colony's header, and the
// game resumes.
func TestJourneySavesSaveAsNewName(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	colony := j.a.opt.Saves.SaveSlot
	openSave(j)
	j.typeText("Iron Harbor")                                          // over the prefilled, selected name
	under := buttonByID(t, j.a.pauseButtons(j.a.size()), "pause.load") // under the dialog's list
	ux, uy := under.X+under.W/2, under.Y+under.H/2
	for x := under.X + 1; x < under.X+under.W; x += 4 { // a point of it none of the dialog's controls cover
		if !slices.ContainsFunc(j.a.uiSt().Cur, func(w uikit.Widget) bool { return w.R.Hit(int(x), int(uy)) }) {
			ux = x
			break
		}
	}
	s := float32(j.a.scale.S)
	j.f.clickAt(t, j.a, int(ux*s), int(uy*s))
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() != overlayPause || j.a.uiSt().Top() != "save" || len(h.metas) != 0 {
		t.Fatalf("a click under the dialog reached the pause menu: %s, overlay %v, top %q, saves %v",
			j.a.nav.screen(), j.a.nav.overlay(), j.a.uiSt().Top(), h.metas)
	}
	if !j.clickID("modal.save.ok") {
		t.Fatal("Save is disabled")
	}
	m := h.last(t)
	if m.Name != "Iron Harbor" || m.Base != "Iron Harbor" || m.Autosave || m.ColonyName != colony || m.Colony == "" || m.Season == "" {
		t.Fatalf("saved %+v: want Iron Harbor of colony %q, by the player, with its id and season", m, colony)
	}
	if j.a.nav.overlay() == overlayPause || j.a.uiSt().ModalOpen() || !strings.HasPrefix(j.a.hint, "saved") {
		t.Fatalf("after Save: overlay %v, modal %q, hint %q; want the game resumed and the save said", j.a.nav.overlay(), j.a.uiSt().Top(), j.a.hint)
	}
	openSave(j) // the name of the last save comes back
	if got := shownField(j); got != "Iron Harbor" {
		t.Fatalf("the dialog reopened with %q, want Iron Harbor", got)
	}
}

// shownField is the Save dialog's name as typed.
func shownField(j *journey) string { return j.a.opt.Saves.DialogName() }

// TestJourneySavesOverwriteAsks: a listed save's row gives its name; Save
// then asks before replacing it, naming its date and its colony (another
// one); Cancel goes back to the dialog, Replace writes over it.
func TestJourneySavesOverwriteAsks(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	openSave(j)
	j.clickID("save.list.body.0")
	if got := shownField(j); got != "Old Camp" {
		t.Fatalf("the row gave the name %q, want Old Camp", got)
	}
	j.clickID("modal.save.ok")
	if top := j.a.uiSt().Top(); top != "save.overwrite" || len(h.metas) != 0 {
		t.Fatalf("Save over Old Camp: top %q, saves %v; want the question first", top, h.metas)
	}
	j.clickID("modal.save.overwrite.cancel")
	if top := j.a.uiSt().Top(); top != "save" || len(h.metas) != 0 {
		t.Fatalf("Cancel: top %q, saves %v; want back to the dialog, nothing written", top, h.metas)
	}
	j.clickID("save.name")
	j.key("A", input.ModCtrl)
	j.typeText("old camp") // another spelling of the same file asks too
	j.clickID("modal.save.ok")
	j.clickID("modal.save.overwrite.ok")
	m := h.last(t)
	if m.Name != "old camp" || m.Colony == "c-other" || len(h.slots) != 1 || h.slots[0].Colony != m.Colony {
		t.Fatalf("Replace wrote %+v, files %+v; want this colony's save in Old Camp's file", m, h.slots)
	}
	if j.a.uiSt().ModalOpen() || j.a.nav.overlay() == overlayPause {
		t.Fatalf("after Replace: modal %q, overlay %v", j.a.uiSt().Top(), j.a.nav.overlay())
	}
}

// TestJourneySavesNameValidationVisible: a name that cannot be saved says
// why in red under the field and on the disabled Save; a name that is
// another save's file under another spelling says so. Not parallel:
// shownText reads the shared draw trace.
func TestJourneySavesNameValidationVisible(t *testing.T) {
	j, h := dialogJourney(t)
	openSave(j)
	for _, c := range []struct{ typed, why string }{
		{"con", "Windows keeps this name for a device"},
		{"Harbor-autosave-2", "Names ending in -autosave are kept for autosaves"},
		{"Harbor/2", "Use only letters, digits, spaces, - and _"},
	} {
		j.key("A", input.ModCtrl) // select all: the next text replaces it
		j.typeText(c.typed)
		if disabled, reason := saveOK(j); !disabled || reason != c.why {
			t.Fatalf("%q: Save disabled %v (%q), want %q", c.typed, disabled, reason, c.why)
		}
		if shown := shownText(t, j.a); !strings.Contains(shown, c.why) {
			t.Fatalf("%q: the dialog does not show %q:\n%s", c.typed, c.why, shown)
		}
		j.clickID("modal.save.ok")
		if len(h.metas) != 0 || j.a.uiSt().Top() != "save" {
			t.Fatalf("%q: a refused name saved %v (top %q)", c.typed, h.metas, j.a.uiSt().Top())
		}
	}
	j.key("A", input.ModCtrl)
	j.typeText("old camp")
	if disabled, reason := saveOK(j); disabled {
		t.Fatalf("old camp is refused: %q", reason)
	}
	if shown := shownText(t, j.a); !strings.Contains(shown, "Same file as Old Camp") {
		t.Fatalf("no same-file note:\n%s", shown)
	}
}

// TestJourneySavesAutosaveFollowsColony: after a save under a new name the
// morning autosave is still the colony's (its slot, its id), not the name's,
// and carries the name as its base.
func TestJourneySavesAutosaveFollowsColony(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	slot := j.a.opt.Saves.SaveSlot
	openSave(j)
	j.typeText("Iron Harbor")
	j.clickID("modal.save.ok")
	manual := h.last(t)
	j.a.settings.Gameplay.AutosaveDays = 1
	toMorning(j, j.g, 1)
	m := h.last(t)
	if len(h.metas) != 2 || m.Name != slot+"-autosave" || !m.Autosave || m.Colony != manual.Colony ||
		m.ColonyName != manual.ColonyName || m.Base != "Iron Harbor" {
		t.Fatalf("autosave %+v after %+v: want %s-autosave of the same colony, based on Iron Harbor", m, manual, slot)
	}
}

// TestJourneySavesLegacyLoadAutosaves: a save from before colony ids loads
// as the colony "slot:<base>"; its autosave keeps that id and that slot.
func TestJourneySavesLegacyLoadAutosaves(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	h.load = ports.SaveMeta{Name: "Old Camp", Colony: "slot:Old Camp", ColonyName: "Old Camp", Base: "Old Camp"} // savefmt.Info.Upgrade's
	toMenu(j)
	j.clickID(ids.MenuLoad)
	j.clickID(ids.PrefixLoadSlot + "0")
	g, ok := j.a.renderer.game.(*game.Game)
	if !ok || g == j.g || j.a.nav.screen() != ports.ScreenColony {
		t.Fatalf("the legacy save did not load: %s", j.a.nav.screen())
	}
	j.g = g
	j.a.settings.Gameplay.AutosaveDays = 1
	toMorning(j, g, 1)
	m := h.last(t)
	if m.Name != "Old-Camp-autosave" || m.Colony != "slot:Old Camp" || m.ColonyName != "Old Camp" || m.Base != "Old Camp" || !m.Autosave {
		t.Fatalf("autosave %+v: want Old-Camp-autosave of slot:Old Camp", m)
	}
}

// toMorning runs g through day's autosave hour.
func toMorning(j *journey, g *game.Game, day int) {
	j.t.Helper()
	morning := uint64(day)*clock.TicksPerDay + saves.AutosaveHour*clock.TicksPerHour
	if d := int64(morning) - 5 - int64(g.Tick()); d > 0 {
		g.Step(int(d))
	}
	j.tick(10)
}

// TestJourneySavesDeleteConfirm: a save's Delete in the dialog asks first;
// Cancel keeps it, Delete removes the file and the list is read again.
func TestJourneySavesDeleteConfirm(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	h.put(ports.SaveMeta{Name: "Bay", Colony: "c-other", ColonyName: "Other Shore", Base: "Bay", Season: "Spring"})
	openSave(j)
	if n := len(j.a.opt.Saves.Slots); n != 2 || j.a.opt.Saves.Slots[1].Name != "Old-Camp" {
		t.Fatalf("the dialog lists %+v, want Bay then Old Camp", j.a.opt.Saves.Slots)
	}
	del := ids.PrefixLoadDelete + "1" // Old Camp's
	j.clickID(del)
	if top := j.a.uiSt().Top(); top != "save.delete" || len(h.deleted) != 0 {
		t.Fatalf("Delete: top %q, deleted %v; want the question first", top, h.deleted)
	}
	j.clickID("modal.save.delete.cancel")
	if len(h.deleted) != 0 || j.a.uiSt().Top() != "save" {
		t.Fatalf("Cancel deleted %v (top %q)", h.deleted, j.a.uiSt().Top())
	}
	j.clickID(del)
	j.clickID("modal.save.delete.ok")
	if len(h.deleted) != 1 || h.deleted[0] != "Old-Camp" {
		t.Fatalf("deleted %v, want [Old-Camp]", h.deleted)
	}
	if s := j.a.opt.Saves.Slots; len(s) != 1 || s[0].Name != "Bay" || j.a.uiSt().Top() != "save" {
		t.Fatalf("after Delete the list is %+v (top %q); want Bay, the dialog still open", s, j.a.uiSt().Top())
	}
	if _, ok := j.a.uiSt().Find(del); ok {
		t.Fatalf("%s still shown after its save was deleted", del)
	}
}

// TestJourneySavesEnterSaves: Enter in the name field is Save; a name
// the rules refuse keeps the dialog open with nothing written.
func TestJourneySavesEnterSaves(t *testing.T) {
	t.Parallel()
	j, h := dialogJourney(t)
	openSave(j)
	j.typeText("Salt Flats") // over the prefilled, selected name
	j.key("Enter", 0)
	if m := h.last(t); m.Name != "Salt Flats" || j.a.uiSt().ModalOpen() {
		t.Fatalf("Enter saved %+v, modal %q; want Salt Flats saved and the dialog closed", m, j.a.uiSt().Top())
	}

	j, h = dialogJourney(t)
	openSave(j)
	j.typeText("x-autosave") // reserved
	j.key("Enter", 0)
	if len(h.metas) != 0 || j.a.uiSt().Top() != "save" {
		t.Fatalf("Enter on a refused name: saves %v, top %q; want nothing written, the dialog kept", h.metas, j.a.uiSt().Top())
	}
}
