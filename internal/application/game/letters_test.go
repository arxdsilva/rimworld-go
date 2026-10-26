package game

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-018 §6: every letter posted becomes a saved letter in the right-side
// stack, until the player dismisses it.

func letterGame() *Game { return New(Config{Seed: 1, Width: 16, Height: 16}) }

// letterIDs lists the stack's ids, oldest first.
func letterIDs(g *Game) []uint64 {
	var ids []uint64
	for _, l := range g.World().Letters {
		ids = append(ids, l.ID)
	}
	return ids
}

func letterByTitle(t testing.TB, g *Game, title string) LetterView {
	t.Helper()
	for _, l := range g.World().Letters {
		if l.Title == title {
			return l
		}
	}
	t.Fatalf("no letter titled %q in %+v", title, g.World().Letters)
	return LetterView{}
}

func TestLetterPostBecomesLetter(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.letter(letterGood, "Wanderer joins", "Bo has asked to join the colony.", 7)
	ls := g.World().Letters
	if len(ls) != 1 {
		t.Fatalf("%d letters, want 1", len(ls))
	}
	x, y := g.home()
	want := LetterView{ID: 1, Tick: uint64(g.clock.Now()), Kind: "good", Title: "Wanderer joins",
		Text: "Bo has asked to join the colony.", X: x, Y: y, Focus: 7}
	if ls[0] != want {
		t.Fatalf("letter %+v, want %+v", ls[0], want)
	}
	// the message is still posted: pause-on-threat and the ticker read it
	ms := g.World().Messages
	if len(ms) != 1 || !ms[0].Letter || ms[0].Title != "Wanderer joins" || ms[0].Kind != "good" {
		t.Fatalf("messages %+v", ms)
	}
	// a plain message is no letter
	g.message(1, 1, "12 meat rotted away")
	if n := len(g.World().Letters); n != 1 {
		t.Fatalf("plain messages made letters: %d", n)
	}
	// a threat is a red letter titled for the stack, and still a danger
	// message for pause-on-threat (PLAY-018)
	g.threat("Raid", 2, 2, "Raid! %d drifters coming.", 4)
	ls = g.World().Letters
	if len(ls) != 2 || ls[1].Kind != "threat" || ls[1].Title != "Raid" || ls[1].Text != "Raid! 4 drifters coming." {
		t.Fatalf("threat letter %+v", ls)
	}
	if ms := g.World().Messages; !ms[len(ms)-1].Danger {
		t.Fatalf("threat message lost its danger flag: %+v", ms[len(ms)-1])
	}
}

// A letter with no place reads X and Y as -1 (the UI offers "Jump to"
// only for a place or a creature); a message at 0, 0 means no place.
func TestLetterWithoutPlaceIsMinusOne(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.post(MessageView{Tick: 5, Text: "a", Letter: true, Title: "nowhere"})
	g.post(MessageView{Tick: 6, Text: "b", Letter: true, Title: "off map", X: 99, Y: 3, Focus: 8})
	g.post(MessageView{Tick: 7, Text: "c", Letter: true, Title: "negative", X: -4, Y: 2})
	g.post(MessageView{Tick: 8, Text: "d", Letter: true, Title: "corner", X: 0, Y: 5})
	g.post(MessageView{Tick: 9, Text: "e", Letter: true, Title: "here", X: 3, Y: 4})
	want := map[string][2]int{"nowhere": {-1, -1}, "off map": {-1, -1}, "negative": {-1, -1}, "corner": {0, 5}, "here": {3, 4}}
	for title, xy := range want {
		if l := letterByTitle(t, g, title); l.X != xy[0] || l.Y != xy[1] {
			t.Errorf("%q: place %d,%d, want %v", title, l.X, l.Y, xy)
		}
	}
	if l := letterByTitle(t, g, "off map"); l.Focus != 8 {
		t.Fatalf("the focus was lost: %+v", l)
	}
	// the message keeps what was posted
	if m := g.World().Messages[0]; m.X != 0 || m.Y != 0 {
		t.Fatalf("the message changed: %+v", m)
	}
}

func TestLetterKindDefaults(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.post(MessageView{Text: "a", Letter: true, Title: "danger", Danger: true})
	g.post(MessageView{Text: "b", Letter: true, Title: "plain"})
	g.post(MessageView{Text: "c", Letter: true, Title: "caution", Kind: "caution"})
	g.post(MessageView{Text: "d", Letter: true, Title: "bogus", Kind: "purple"})
	g.post(MessageView{Text: "e", Letter: true, Title: "bogus danger", Kind: "purple", Danger: true})
	want := map[string]string{"danger": "threat", "plain": "info", "caution": "caution", "bogus": "info", "bogus danger": "threat"}
	for title, kind := range want {
		if got := letterByTitle(t, g, title).Kind; got != kind {
			t.Errorf("%q: kind %q, want %q", title, got, kind)
		}
	}
}

func TestLetterCapDropsOldest(t *testing.T) {
	t.Parallel()
	g := letterGame()
	for i := 1; i <= maxLetters+5; i++ {
		g.letter(letterInfo, fmt.Sprintf("Letter %d", i), "text", 0)
	}
	ids := letterIDs(g)
	if len(ids) != maxLetters || maxLetters != 40 {
		t.Fatalf("%d letters kept (cap %d), want 40", len(ids), maxLetters)
	}
	for i, id := range ids { // the newest 40, oldest first: ids 6..45
		if id != uint64(i+6) {
			t.Fatalf("letter %d has id %d, want %d (ids %v)", i, id, i+6, ids)
		}
	}
	if l := g.World().Letters[maxLetters-1]; l.Title != "Letter 45" {
		t.Fatalf("newest letter is %q", l.Title)
	}
	// the counter keeps counting past dropped letters: ids are never reused
	g.letter(letterInfo, "One more", "text", 0)
	if l := letterByTitle(t, g, "One more"); l.ID != uint64(maxLetters+6) {
		t.Fatalf("id after the cap: %d", l.ID)
	}
}

func TestLetterIDsDeterministic(t *testing.T) {
	t.Parallel()
	run := func() []uint64 {
		g := letterGame()
		for i := range 7 {
			g.letter(letterCaution, fmt.Sprintf("L%d", i), "text", 0)
		}
		g.Step(3)
		return letterIDs(g)
	}
	a, b := run(), run()
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(a, []uint64{1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("ids %v and %v, want 1..7 both times", a, b)
	}
}

func TestLetterWorldViewIsACopy(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.letter(letterInfo, "Visitors", "text", 0)
	g.World().Letters[0].Title = "changed"
	if got := g.World().Letters[0].Title; got != "Visitors" {
		t.Fatalf("a caller rewrote the stack: %q", got)
	}
}

func TestDismissLetter(t *testing.T) {
	t.Parallel()
	g := letterGame()
	for i := range 4 {
		g.letter(letterInfo, fmt.Sprintf("L%d", i), "text", 0)
	}
	dismiss := func(id uint64) {
		g.Enqueue(command.Command{Kind: command.DismissLetter, Target: id})
		g.Step(1)
	}
	dismiss(2)
	if got := letterIDs(g); !reflect.DeepEqual(got, []uint64{1, 3, 4}) {
		t.Fatalf("after dismissing 2: %v", got)
	}
	// an unknown id (already dismissed, never issued, or 0) changes nothing
	for _, id := range []uint64{2, 99, 0} {
		dismiss(id)
		if got := letterIDs(g); !reflect.DeepEqual(got, []uint64{1, 3, 4}) {
			t.Fatalf("dismissing unknown %d: %v", id, got)
		}
	}
	dismiss(4)
	dismiss(1)
	if got := letterIDs(g); !reflect.DeepEqual(got, []uint64{3}) {
		t.Fatalf("after dismissing 4 and 1: %v", got)
	}
	// dismissing leaves the message log alone and ids are not reused
	if len(g.World().Messages) != 4 {
		t.Fatalf("%d messages", len(g.World().Messages))
	}
	g.letter(letterInfo, "Next", "text", 0)
	if l := letterByTitle(t, g, "Next"); l.ID != 5 {
		t.Fatalf("id after dismissals: %d, want 5", l.ID)
	}
}

func saveAndLoad(t testing.TB, g *Game) *Game {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "letters"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestLettersSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.letter(letterThreat, "Kidnapped", "Ada was kidnapped by raiders.", 3)
	g.letter(letterGood, "Wanderer joins", "Bo is here.", 4)
	g.letter(letterInfo, "Visitors", "Three people came.", 0)
	g.Enqueue(command.Command{Kind: command.DismissLetter, Target: 2})
	g.Step(1)
	h := saveAndLoad(t, g)
	if got, want := h.World().Letters, g.World().Letters; len(want) != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded letters %+v, want %+v", got, want)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("the loaded game hashes differently")
	}
	// the counter survives: the next letter does not reuse 2 or 3
	h.letter(letterInfo, "Next", "text", 0)
	if l := letterByTitle(t, h, "Next"); l.ID != 4 {
		t.Fatalf("id after a load: %d, want 4", l.ID)
	}
}

// A v3 save has no letters; the new fields are additive, so such a colony
// saves and loads to the identical state (letters stay empty, no id is
// issued by the load).
func TestSaveWithoutLettersRoundTripsIdentically(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 24, Height: 12})
	if _, err := g.SpawnNamedPawn(2, 2, "Ada"); err != nil {
		t.Fatal(err)
	}
	g.Step(300)
	if len(g.World().Letters) != 0 || g.letters.next != 0 {
		t.Fatal("set-up: the colony posted a letter")
	}
	h := saveAndLoad(t, g)
	if len(h.World().Letters) != 0 || h.letters.next != 0 {
		t.Fatalf("the load made letters: %+v (next %d)", h.World().Letters, h.letters.next)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("the loaded colony hashes differently")
	}
	g.Step(100)
	h.Step(100)
	if h.StateHash() != g.StateHash() {
		t.Fatal("the loaded colony runs differently")
	}
}

// A save written with no letters at all (a dismissed stack, a colony that
// never posted one) loads without letters, even if letter messages are
// still in the log: only a save from before the stack existed migrates.
func TestDismissedLettersStayDismissedAfterLoad(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.letter(letterInfo, "Visitors", "text", 0)
	g.Enqueue(command.Command{Kind: command.DismissLetter, Target: 1})
	g.Step(1)
	if len(g.World().Letters) != 0 || len(g.World().Messages) != 1 {
		t.Fatal("set-up: the letter should be gone but its message kept")
	}
	h := saveAndLoad(t, g)
	if n := len(h.World().Letters); n != 0 {
		t.Fatalf("%d dismissed letters came back", n)
	}
}

// A save from before the stack has letter messages and no letters:
// loading turns them into letters, oldest first, with fresh ids.
func TestOldSaveMigratesLetterMessages(t *testing.T) {
	t.Parallel()
	g := letterGame()
	g.post(MessageView{Tick: 10, Text: "A raid was repelled.", X: 3, Y: 4})
	g.post(MessageView{Tick: 20, Text: "Cold!", X: 5, Y: 6, Danger: true, Letter: true, Title: "Cold snap"})
	g.post(MessageView{Tick: 30, Text: "Bo joined.", X: 7, Y: 8, Focus: 9, Letter: true, Title: "Wanderer joins", Kind: "good"})
	g.post(MessageView{Tick: 40, Text: "Caravan left.", X: 1, Y: 2, Letter: true, Title: "Caravan departed"})
	g.letters = letterState{} // as saved before PLAY-018: messages, no letters
	h := saveAndLoad(t, g)
	want := []LetterView{
		{ID: 1, Tick: 20, Kind: "threat", Title: "Cold snap", Text: "Cold!", X: 5, Y: 6},
		{ID: 2, Tick: 30, Kind: "good", Title: "Wanderer joins", Text: "Bo joined.", X: 7, Y: 8, Focus: 9},
		{ID: 3, Tick: 40, Kind: "info", Title: "Caravan departed", Text: "Caravan left.", X: 1, Y: 2},
	}
	if got := h.World().Letters; !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated letters\n got %+v\nwant %+v", got, want)
	}
	// the messages are untouched, and a second load does not migrate twice
	if len(h.World().Messages) != 4 {
		t.Fatalf("%d messages after the load", len(h.World().Messages))
	}
	if again := saveAndLoad(t, h).World().Letters; !reflect.DeepEqual(again, want) {
		t.Fatalf("second load: %+v", again)
	}
	h.letter(letterInfo, "Next", "text", 0)
	if l := letterByTitle(t, h, "Next"); l.ID != 4 {
		t.Fatalf("id after migrating: %d, want 4", l.ID)
	}
}

// --- kinds at the real post sites ---

func TestLetterKindsAtPostSites(t *testing.T) {
	t.Parallel()
	t.Run("intro", func(t *testing.T) {
		t.Parallel()
		s := setupFor(t, "broken_hull", nil)
		g := buildSetup(t, s)
		stepUntilLanded(t, g)
		g.Step(10)
		l := letterByTitle(t, g, s.Scenario.Name)
		if l.Kind != "info" || !l.Intro {
			t.Fatalf("intro letter %+v", l)
		}
		// the UI matches the letter to its message by Tick, Title and Text
		found := false
		for _, m := range g.World().Messages {
			if m.Letter && m.Title == s.Scenario.Name {
				found = true
				if m.Tick != l.Tick || m.Text != l.Text || m.Kind != l.Kind || m.X != l.X || m.Y != l.Y {
					t.Fatalf("intro message %+v and letter %+v differ", m, l)
				}
			}
		}
		if !found {
			t.Fatal("no intro message")
		}
		g.Step(500)
		for _, o := range g.World().Letters {
			if o.Intro && o.ID != l.ID {
				t.Fatalf("a second intro letter: %+v", o)
			}
		}
	})
	t.Run("extreme weather", func(t *testing.T) {
		t.Parallel()
		g := bareSetupGame(difficulty.Seasoned)
		for day := 1; day < 3000 && len(g.World().Letters) == 0; day++ {
			now := uint64(day) * clock.TicksPerDay
			g.clock = clock.NewAt(clock.Tick(now))
			g.extremeDay(now)
		}
		ls := g.World().Letters
		if len(ls) != 1 || ls[0].Kind != "caution" || ls[0].Intro {
			t.Fatalf("weather letters %+v", ls)
		}
		if ls[0].Title != "Heat wave" && ls[0].Title != "Cold snap" {
			t.Fatalf("weather letter %+v", ls[0])
		}
	})
	t.Run("vault breach", func(t *testing.T) {
		t.Parallel()
		g := vaultGame(t, nil, nil)
		breachV1(t, g)
		if l := letterByTitle(t, g, vaultBreachTitle); l.Kind != "threat" {
			t.Fatalf("breach with defenders: %+v", l)
		}
	})
	t.Run("vault breach, nobody inside", func(t *testing.T) {
		t.Parallel()
		g := vaultGame(t, func(s *Setup) { s.Difficulty.ThreatScale = 0 }, nil)
		breachV1(t, g)
		if l := letterByTitle(t, g, vaultBreachTitle); l.Kind != "info" {
			t.Fatalf("quiet breach: %+v", l)
		}
	})
	t.Run("vault warning", func(t *testing.T) {
		t.Parallel()
		g := vaultGame(t, nil, nil)
		standColonist(g, 61, v1DoorY)
		g.vaultTick(2 * vaultCheckEvery)
		if l := letterByTitle(t, g, vaultWarnTitle); l.Kind != "caution" {
			t.Fatalf("vault warning: %+v", l)
		}
	})
	t.Run("social fight", func(t *testing.T) {
		t.Parallel()
		f, x, y := fightPair(t, 6)
		f.startSocialFight(x, y)
		if l := letterByTitle(t, f, "Social fight"); l.Kind != "caution" || l.Focus == 0 {
			t.Fatalf("social fight: %+v", l)
		}
	})
	t.Run("rejected proposal", func(t *testing.T) {
		t.Parallel()
		g, a, b := couple(t)
		adore(g, b, a)
		woo(t, g, a, b)
		advanceDays(g, 6)
		for range 5 {
			g.addOpinionMemory(b, a, social.MemInsulted, socNow(g))
		}
		g.propose(a, b, socNow(g))
		if l := letterByTitle(t, g, "Proposal rejected"); l.Kind != "caution" || l.Focus != a {
			t.Fatalf("rejected proposal: %+v", l)
		}
	})
}
