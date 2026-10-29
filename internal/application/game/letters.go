package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// The letter stack (PLAY-018): every message posted with Letter set also
// becomes a saved letter that stays in the right-side stack until the
// player dismisses it (command.DismissLetter). The message itself is
// posted as before (the ticker, pause-on-threat and the intro window read
// it); the stack only adds a longer memory and a stable id.

// maxLetters is how many letters the stack keeps: the oldest are dropped.
const maxLetters = 40

// Letter kinds colour a letter: threat (raids, break-ins, kidnappings),
// caution (bad news that is not an attack), info (arrivals, the intro) and
// good (joins, weddings, parties).
const (
	letterThreat  = "threat"
	letterCaution = "caution"
	letterInfo    = "info"
	letterGood    = "good"
)

// savedLetter is one letter as kept and saved (LetterView's fields).
type savedLetter struct {
	ID, Tick uint64
	Kind     string
	Title    string
	Text     string
	X, Y     int
	Focus    entity.EntityID
	Intro    bool
	// Join: the rescued stranger whose offer to join this letter carries
	// (0: none; SIM-063). AcceptLetter takes it in, dismissing declines.
	Join entity.EntityID
}

// letterState is the stack, oldest first, and how many letters were ever
// issued: ids are never reused, so a dismissed letter's id stays dead.
type letterState struct {
	list []savedLetter
	next uint64
}

// letterKindOf is the kind a letter message shows: its own Kind when it
// is one of the four, else threat for a danger message and info for the
// rest.
func letterKindOf(m MessageView) string {
	switch m.Kind {
	case letterThreat, letterCaution, letterInfo, letterGood:
		return m.Kind
	}
	if m.Danger {
		return letterThreat
	}
	return letterInfo
}

// addLetter puts message m on the stack and returns the kind it got. The
// letter keeps the message's Tick, Title and Text (the UI matches a letter
// to its message by them); a message with no place (0, 0 or off the map)
// gives the letter X and Y of -1.
func (g *Game) addLetter(m MessageView) string {
	g.letters.next++
	kind := letterKindOf(m)
	x, y := m.X, m.Y
	if x == 0 && y == 0 || !g.grid.InBounds(x, y) {
		x, y = -1, -1 // no place (a message at 0,0 means none): the UI offers no jump
	}
	g.letters.list = append(g.letters.list, savedLetter{ID: g.letters.next, Tick: m.Tick, Kind: kind,
		Title: m.Title, Text: m.Text, X: x, Y: y, Focus: m.Focus})
	if n := len(g.letters.list); n > maxLetters {
		g.letters.list = append(g.letters.list[:0:0], g.letters.list[n-maxLetters:]...)
	}
	return kind
}

// markIntroLetter flags the newest letter as the scenario's opening one.
func (g *Game) markIntroLetter() {
	if n := len(g.letters.list); n > 0 {
		g.letters.list[n-1].Intro = true
	}
}

// dismissLetter removes letter id from the stack; an unknown id is ignored.
// Dismissing a join offer declines it: the stranger, still here, goes its
// way.
func (g *Game) dismissLetter(id uint64) {
	if i := slices.IndexFunc(g.letters.list, func(l savedLetter) bool { return l.ID == id }); i >= 0 {
		p := g.letters.list[i].Join
		g.letters.list = slices.Delete(g.letters.list, i, i+1)
		if p != 0 && g.visitor(p) {
			careGuests{g}.Leave(p) // visitors.go
		}
	}
}

// acceptLetter accepts letter id's offer (command.AcceptLetter): a rescued
// stranger still on the map joins the colony. A letter with no offer, or
// an unknown id, is left as it is.
func (g *Game) acceptLetter(id uint64) {
	i := slices.IndexFunc(g.letters.list, func(l savedLetter) bool { return l.ID == id })
	if i < 0 || g.letters.list[i].Join == 0 {
		return
	}
	p := g.letters.list[i].Join
	g.letters.list = slices.Delete(g.letters.list, i, i+1)
	if v, _ := g.vital.Get(p); !g.visitor(p) || v.Status == health.Dead {
		g.worldNote("%s is no longer here to join.", g.personName(p))
		return
	}
	careGuests{g}.join(p) // visitors.go
}

// letterViews is the stack for the read model, oldest first (a copy).
func (g *Game) letterViews() []LetterView {
	if len(g.letters.list) == 0 {
		return nil
	}
	out := make([]LetterView, len(g.letters.list))
	for i, l := range g.letters.list {
		out[i] = LetterView{ID: l.ID, Tick: l.Tick, Kind: l.Kind, Title: l.Title, Text: l.Text,
			X: l.X, Y: l.Y, Focus: l.Focus, Intro: l.Intro, JoinOffer: l.Join != 0}
	}
	return out
}

// importLetters restores the stack from a save. A save from before the
// stack existed has letter messages but never issued an id (next is 0):
// those messages become the letters, oldest first. Their intro flag is not
// recovered, so an old intro letter reads as a plain info letter.
func (g *Game) importLetters(list []savedLetter, next uint64) {
	g.letters = letterState{list: slices.Clone(list), next: next}
	if next != 0 || len(list) != 0 {
		return
	}
	for _, m := range g.messages {
		if m.Letter {
			g.addLetter(m)
		}
	}
}
