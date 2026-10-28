package ebiten

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-018: letters are a stack in the right column (WorldView.Letters,
// oldest at the top); a click opens one, a right-click dismisses it, and
// only the scenario's intro letter opens by itself.

func letterStub(ls ...game.LetterView) *stubSim {
	s := &stubSim{w: 80, h: 80, tick: 10, pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive", X: 20, Y: 20}}}
	s.world.Letters = ls
	return s
}

func letter(id uint64, kind, title string) game.LetterView {
	return game.LetterView{ID: id, Tick: 5, Kind: kind, Title: title, Text: "Text of " + title + ".", X: -1, Y: -1}
}

// nLetters is n letters with ids 1..n, oldest first.
func nLetters(n int) []game.LetterView {
	var out []game.LetterView
	for i := 1; i <= n; i++ {
		out = append(out, letter(uint64(i), []string{"threat", "caution", "info", "good"}[i%4], fmt.Sprintf("Letter %d", i)))
	}
	return out
}

func TestLettersStackFromTheReadModel(t *testing.T) {
	t.Parallel()
	s := letterStub(letter(11, "threat", "Raid"), letter(12, "caution", "Heat wave"), letter(13, "good", "Wedding"))
	a, _ := colonyApp(t, s)
	l := a.hud()
	rows := a.letterRows(l)
	if len(rows) != 3 || l.LetterRows != 3 || a.letterCount() != 3 {
		t.Fatalf("rows %d, layout rows %d, count %d; want 3", len(rows), l.LetterRows, a.letterCount())
	}
	for i, r := range rows {
		if r.letter.ID != uint64(11+i) {
			t.Errorf("row %d is letter %d, want %d (oldest at the top)", i, r.letter.ID, 11+i)
		}
		if r.r.H != letterRowH || r.r.X != l.Letters.X || r.r.W != l.Letters.W {
			t.Errorf("row %d rect %+v in Letters %+v", i, r.r, l.Letters)
		}
		if i > 0 && r.r.Y-rows[i-1].r.Bottom() != hudGap {
			t.Errorf("row %d is %v below row %d, want the %d px gap", i, r.r.Y-rows[i-1].r.Bottom(), i-1, hudGap)
		}
	}
	if got := rows[2].r.Bottom(); got != l.Letters.Bottom() {
		t.Errorf("newest row ends at %v, Letters ends at %v: the newest sits just above the controls", got, l.Letters.Bottom())
	}
	// every row is a button with the id letter.<ID>
	ids := map[string]button{}
	for _, b := range a.rightColumnButtons(a.sw) {
		ids[b.id] = b
	}
	for _, r := range rows {
		b, ok := ids[fmt.Sprintf("letter.%d", r.letter.ID)]
		if !ok || b.x != r.r.X || b.y != r.r.Y || b.w != r.r.W || b.h != r.r.H {
			t.Errorf("letter.%d button %+v (found %v), want %+v", r.letter.ID, b, ok, r.r)
		}
	}
	// kind colours
	for kind, want := range map[string]any{"threat": uiLetterThreat, "caution": uiLetterCaution, "good": uiLetterGood, "info": uiLetterInfo, "": uiLetterInfo, "bogus": uiLetterInfo} {
		if got := letterColor(kind); got != want {
			t.Errorf("letterColor(%q) = %v, want %v", kind, got, want)
		}
	}
	a.drawRightColumn(ebiten.NewImage(a.sw, a.sh), a.sw, a.sh) // no art, no panic: icons degrade
}

func TestLettersPlusNOlder(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, letterStub(nLetters(11)...))
	a.sw, a.sh = 1920, 1080 // room for the full stack beside the alerts' reserved rows
	l := a.hud()
	rows := a.letterRows(l)
	if l.LetterRows != maxLetters+1 || len(rows) != maxLetters+1 {
		t.Fatalf("rows %d (layout %d), want the %d newest and a +N row", len(rows), l.LetterRows, maxLetters)
	}
	if rows[0].older != 3 || letterRowLabel(rows[0]) != "+3 older" {
		t.Fatalf("top row = %+v (%q), want \"+3 older\"", rows[0], letterRowLabel(rows[0]))
	}
	for i, r := range rows[1:] {
		if want := uint64(4 + i); r.letter.ID != want || r.older != 0 {
			t.Errorf("row %d = letter %d, want %d", i+1, r.letter.ID, want)
		}
	}
	for _, b := range a.rightColumnButtons(a.sw) {
		if b.id == "letter.0" || strings.Contains(b.label, "older") {
			t.Errorf("the +N older row is a button: %+v", b)
		}
	}
	// dismissing a visible one brings an older one back
	s := a.renderer.game.(*stubSim)
	s.world.Letters = append(s.world.Letters[:5:5], s.world.Letters[6:]...) // drop letter 6
	rows = a.letterRows(a.hud())
	if rows[0].older != 2 || rows[1].letter.ID != 3 {
		t.Fatalf("after a dismissal: top row %+v, next %+v", rows[0], rows[1])
	}
}

// When the room runs out the stack keeps the newest and counts the rest.
func TestLettersStackKeepsTheNewestWhenCramped(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, letterStub(nLetters(6)...))
	l := computeHUDLayout(1280, 720, hudState{Letters: 6, GlobalH: 380, LearnH: 200})
	if l.LetterRows == 0 || l.LetterRows >= 6 {
		t.Fatalf("fixture: %d rows", l.LetterRows)
	}
	rows := a.letterRows(l)
	if len(rows) != l.LetterRows || rows[0].older != 6-(l.LetterRows-1) || rows[len(rows)-1].letter.ID != 6 {
		t.Fatalf("rows %d (layout %d): first %+v, last %+v", len(rows), l.LetterRows, rows[0], rows[len(rows)-1])
	}
	if got := rows[len(rows)-1].r.Bottom(); got != l.Letters.Bottom() {
		t.Errorf("newest ends at %v, want %v", got, l.Letters.Bottom())
	}
	if rows := a.letterRows(hudLayout{}); len(rows) != 0 {
		t.Errorf("rows without a Letters rect: %+v", rows)
	}
}

func TestLetterClickOpensTheLetterWindow(t *testing.T) {
	t.Parallel()
	s := letterStub(letter(21, "info", "Quiet day"))
	s.world.Letters[0].X, s.world.Letters[0].Y = 30, 31
	a, f := colonyApp(t, s)
	if a.uiModalOpen() {
		t.Fatal("a plain letter opened by itself")
	}
	pressButton(t, a, f, "letter.21")
	top := a.uiModalTop()
	if !strings.HasPrefix(top, "letter.") {
		t.Fatalf("top modal = %q after clicking the letter", top)
	}
	if m := a.uiSt().modals[0]; m.title != "Quiet day" {
		t.Fatalf("modal title %q", m.title)
	}
	f.frame(t, a)
	if !hasWidget(a, "modal."+top+".continue") {
		t.Fatal("no Continue button")
	}
	if !hasWidget(a, "modal."+top+".jump") {
		t.Fatal("a letter with a place has a \"Jump to\" button")
	}
	if !a.letterRead(21) {
		t.Error("an opened letter is read")
	}
	// Jump to moves the camera onto the place and closes the window
	before := a.renderer.cam
	press(t, a, f, "modal."+top+".jump")
	if a.renderer.cam == before {
		t.Error("Jump to did not move the camera")
	}
	x, y := a.renderer.TileAt(int((a.mapViewport().x0+a.mapViewport().x1)/2), int((a.mapViewport().y0+a.mapViewport().y1)/2))
	if abs(x-30) > 1 || abs(y-31) > 1 {
		t.Errorf("camera centred on %d,%d, want 30,31", x, y)
	}
	if a.uiModalOpen() {
		t.Error("Jump to leaves the window open")
	}
	// the letter stays in the stack: only a dismissal removes it
	if len(a.letterRows(a.hud())) != 1 {
		t.Error("reading a letter removed it from the stack")
	}
}

func TestLetterWithoutAPlaceHasNoJumpAndFocusSelects(t *testing.T) {
	t.Parallel()
	s := letterStub(letter(31, "info", "Nowhere"), letter(32, "good", "About Ada"))
	s.world.Letters[1].Focus = 1
	a, f := colonyApp(t, s)
	pressButton(t, a, f, "letter.31")
	f.frame(t, a)
	top := a.uiModalTop()
	if hasWidget(a, "modal."+top+".jump") {
		t.Error("\"Jump to\" on a letter with no place")
	}
	a.uiCloseTop()
	pressButton(t, a, f, "letter.32")
	f.frame(t, a)
	top = a.uiModalTop()
	if !hasWidget(a, "modal."+top+".jump") {
		t.Fatal("a letter about a creature has \"Jump to\"")
	}
	press(t, a, f, "modal."+top+".jump")
	if sel := a.renderer.Selected(); sel != 1 {
		t.Errorf("selected %v, want Ada", sel)
	}
}

func TestLetterRightClickEnqueuesDismissLetter(t *testing.T) {
	t.Parallel()
	s := letterStub(letter(41, "threat", "Raid"), letter(42, "info", "Notice"))
	a, f := colonyApp(t, s)
	rows := a.letterRows(a.hud())
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	f.cur.mx, f.cur.my = int(rows[0].r.X+rows[0].r.W/2), int(rows[0].r.Y+rows[0].r.H/2)
	f.click(t, a, mbRight, 0)
	got := cmdsOf(s, command.DismissLetter)
	if len(got) != 1 || got[0].Target != 41 {
		t.Fatalf("DismissLetter commands = %+v, want one for letter 41", got)
	}
	if a.uiModalOpen() {
		t.Error("right-click opened the letter")
	}
	// the +N older row is not a letter
	a2, f2 := colonyApp(t, letterStub(nLetters(10)...))
	r2 := a2.letterRows(a2.hud())
	f2.cur.mx, f2.cur.my = int(r2[0].r.X+r2[0].r.W/2), int(r2[0].r.Y+r2[0].r.H/2)
	f2.click(t, a2, mbRight, 0)
	if got := cmdsOf(a2.renderer.game.(*stubSim), command.DismissLetter); len(got) != 0 {
		t.Errorf("right-click on +N older dismissed %+v", got)
	}
}

// ---- the intro letter ----------------------------------------------------

func introStub() *stubSim {
	l := letter(1, "info", "Broken Hull")
	l.Tick, l.Intro, l.Text = 5, true, "The lifeboats strike the ground."
	return letterStub(l)
}

func TestIntroLetterOpensOnceFromTheStack(t *testing.T) {
	t.Parallel()
	s := introStub()
	a, f := colonyApp(t, s)
	if !strings.HasPrefix(a.uiModalTop(), "letter.") {
		t.Fatalf("top modal = %q, want the intro letter", a.uiModalTop())
	}
	if len(a.uiSt().modals) != 1 || a.uiSt().modals[0].title != "Broken Hull" {
		t.Fatalf("modals = %+v", a.uiSt().modals)
	}
	press(t, a, f, "modal."+a.uiModalTop()+".continue")
	if a.uiModalOpen() {
		t.Fatal("Continue should close the letter")
	}
	for i := 0; i < 5; i++ {
		f.frame(t, a)
	}
	if a.uiModalOpen() {
		t.Fatal("the intro must not open again")
	}
	if len(a.letterRows(a.hud())) != 1 {
		t.Error("the intro letter stays in the stack after it is read")
	}

	// no other letter pops up by itself, however fresh
	s.world.Letters = append(s.world.Letters, letter(2, "caution", "Heat wave"), letter(3, "threat", "Raid"))
	s.world.Letters[1].Tick, s.world.Letters[2].Tick = 9, 10
	f.frame(t, a)
	if a.uiModalOpen() {
		t.Fatalf("a new letter opened by itself: %+v", a.uiSt().modals)
	}

	// a colony started later shows its own intro
	a.uiCloseTop()
	a.dropSession()
	a.startSession(s)
	f.frame(t, a)
	if !a.uiModalOpen() {
		t.Fatal("a new colony should show its own intro letter")
	}
}

func TestOldIntroLetterStaysInTheStack(t *testing.T) {
	t.Parallel()
	s := introStub()
	s.tick = 5 + letterFresh + 1 // a loaded save's saved letter
	a, f := colonyApp(t, s)
	f.frame(t, a)
	if a.uiModalOpen() {
		t.Fatalf("a stale intro opened: %+v", a.uiSt().modals)
	}
	if len(a.letterRows(a.hud())) != 1 {
		t.Error("the stale intro is not in the stack")
	}
}

// ---- the new-letter highlight ---------------------------------------------

func TestNewLetterRowFlashesBriefly(t *testing.T) {
	t.Parallel()
	s := letterStub(letter(51, "info", "Old news"))
	a, f := colonyApp(t, s)
	if a.letterFlash(51) != 0 {
		t.Fatal("a letter that was there from the start flashes")
	}
	s.world.Letters = append(s.world.Letters, letter(52, "threat", "Fresh"))
	f.frame(t, a)
	_ = a.letterRows(a.hud()) // the stack notices it
	if k := a.letterFlash(52); k <= 0.9 {
		t.Fatalf("a new letter's flash = %v, want it near 1", k)
	}
	a.renderer.present += letterFlashFrames / 2
	if k := a.letterFlash(52); k <= 0 || k >= 0.9 {
		t.Fatalf("half way, flash = %v", k)
	}
	a.renderer.present += letterFlashFrames
	if k := a.letterFlash(52); k != 0 {
		t.Fatalf("after the flash, %v", k)
	}
	if a.letterFlash(51) != 0 {
		t.Error("the old letter flashed")
	}
	a.drawRightColumn(ebiten.NewImage(a.sw, a.sh), a.sw, a.sh)
}
