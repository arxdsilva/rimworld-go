package ebiten

import (
	"fmt"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// questJourney is a landed generated colony with one quest on offer and the
// letter that announces it.
func questJourney(t *testing.T) (*journey, game.QuestView, game.LetterView) {
	t.Helper()
	g := ax4RiverColony(t)
	g.Step(4000)
	var id uint32
	for k := qd.KindID(1); k < qd.NumKinds && id == 0; k++ {
		g.Enqueue(command.Command{Kind: command.DevOfferQuest, Index: int(k)})
		g.Step(1)
		if vs := g.World().Quests; len(vs) > 0 {
			id = uint32(vs[len(vs)-1].ID)
		}
	}
	if id == 0 {
		t.Fatal("no quest kind could be offered")
	}
	j := newJourney(t, jGame(g))
	var q game.QuestView
	for _, v := range j.world().Quests {
		if v.ID == qd.ID(id) {
			q = v
		}
	}
	for _, l := range j.world().Letters {
		if l.Quest == id {
			j.update(2)
			return j, q, l
		}
	}
	t.Fatalf("no letter carries quest %d: %+v", id, j.world().Letters)
	return nil, q, game.LetterView{}
}

// TestJourneyQuestLetterOpensQuest: clicking the tile of the offer's letter
// opens the Quests window and drops the letter (QST-131).
func TestJourneyQuestLetterOpensQuest(t *testing.T) {
	t.Parallel()
	j, _, l := questJourney(t)
	if !j.clickID(fmt.Sprintf("letter.%d", l.ID)) {
		t.Fatal("the quest letter is not a button")
	}
	j.update(2)
	if j.a.uiModalTop() != windows.QuestsModalID {
		t.Fatalf("top modal %q, want the Quests window", j.a.uiModalTop())
	}
	for _, x := range j.world().Letters {
		if x.ID == l.ID {
			t.Fatal("the quest letter is still in the stack")
		}
	}
}

// TestJourneyQuestExpiringAlert: a day before an offer lapses the alert
// shows, and its click opens the Quests window.
func TestJourneyQuestExpiringAlert(t *testing.T) {
	t.Parallel()
	j, q, _ := questJourney(t)
	key := fmt.Sprintf("%s.%d", hud.QuestExpiringKey, q.ID)
	shown := func() bool {
		for _, al := range j.a.chrome.Alerts().Current() {
			if al.Key == key {
				return true
			}
		}
		return false
	}
	if q.AcceptBy > uint64(j.g.Tick())+clock.TicksPerDay && shown() {
		t.Fatal("the alert is up with more than a day left")
	}
	for uint64(j.g.Tick())+clock.TicksPerDay <= q.AcceptBy {
		j.g.Step(clock.TicksPerHour)
	}
	j.update(2)
	if !shown() {
		t.Fatalf("no alert with under a day left: %+v", j.a.chrome.Alerts().Current())
	}
	if !j.clickID(ids.PrefixAlert + key) {
		t.Fatal("the alert is not a button")
	}
	j.update(2)
	if j.a.uiModalTop() != windows.QuestsModalID {
		t.Fatalf("top modal %q, want the Quests window", j.a.uiModalTop())
	}
}
