package ebiten

import (
	"fmt"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
)

// Quests window journeys (QST-3): the wizard's landed colony (worldColony)
// gets a quest through the DevOfferQuest command, the player opens the window with
// F7 and clicks through it; each journey asserts the command or intent
// that left the client and the read model after it.

// questsOffer offers a quest of kind k (the dev command, not the client),
// opens the Quests window with F7 and picks the quest's row.
func questsOffer(t *testing.T, k qd.KindID) (*journey, game.QuestView) {
	t.Helper()
	j, _ := worldColony(t)
	before := len(j.world().Quests)
	j.g.Enqueue(command.Command{Kind: command.DevOfferQuest, Index: int(k)})
	j.g.Step(1)
	qs := j.world().Quests
	if len(qs) <= before {
		t.Fatalf("no quest of kind %d offered (%d quests)", k, len(qs))
	}
	q := qs[len(qs)-1]
	j.update(2)
	j.key("F7", 0)
	if j.a.win.KitID() != windows.QuestsModalID {
		t.Fatalf("F7 opened %q, want the Quests window", j.a.win.KitID())
	}
	j.clickID(ids.QuestRow + fmt.Sprint(q.ID))
	return j, q
}

// questNow is quest id's current view.
func questNow(t *testing.T, j *journey, id qd.ID) game.QuestView {
	t.Helper()
	for _, q := range j.world().Quests {
		if q.ID == id {
			return q
		}
	}
	t.Fatalf("no quest %d", id)
	return game.QuestView{}
}

// TestJourneyQuestAccept: a supply request offers three rewards; clicking
// the second option's Accept sends AcceptQuest with that option, and the
// quest turns active with it chosen.
func TestJourneyQuestAccept(t *testing.T) {
	t.Parallel()
	j, q := questsOffer(t, qd.Request)
	if len(q.Rewards) < 2 {
		t.Fatalf("the request offers %d rewards, want a choice", len(q.Rewards))
	}
	j.clickID(fmt.Sprintf("%s%d.%d", ids.QuestAccept, q.ID, 1))
	c, ok := j.lastCmd(command.AcceptQuest)
	if !ok || c.Index != int(q.ID) || c.Count != 1 {
		t.Fatalf("AcceptQuest %+v (%v), want quest %d option 1", c, ok, q.ID)
	}
	j.tick(1)
	if got := questNow(t, j, q.ID); got.State != qd.Active || got.Chosen != 1 {
		t.Fatalf("quest %d is %v with option %d, want active with 1", q.ID, got.State, got.Chosen)
	}
}

// TestJourneyQuestDismissRestore: the bin sends DismissQuest and the quest
// moves under Historical; its restore arrow there sends RestoreQuest.
func TestJourneyQuestDismissRestore(t *testing.T) {
	t.Parallel()
	j, q := questsOffer(t, qd.Request)
	j.clickID(ids.QuestDismiss + fmt.Sprint(q.ID))
	if c, ok := j.lastCmd(command.DismissQuest); !ok || c.Index != int(q.ID) {
		t.Fatalf("DismissQuest %+v (%v), want quest %d", c, ok, q.ID)
	}
	j.tick(1)
	if !questNow(t, j, q.ID).Dismissed {
		t.Fatal("the quest is not dismissed")
	}
	j.clickID(ids.QuestsHistory)
	j.clickID(ids.QuestRow + fmt.Sprint(q.ID))
	j.clickID(ids.QuestRestore + fmt.Sprint(q.ID))
	if c, ok := j.lastCmd(command.RestoreQuest); !ok || c.Index != int(q.ID) {
		t.Fatalf("RestoreQuest %+v (%v), want quest %d", c, ok, q.ID)
	}
	j.tick(1)
	if got := questNow(t, j, q.ID); got.Dismissed || got.State != qd.Offered {
		t.Fatalf("restored quest: dismissed %v state %v", got.Dismissed, got.State)
	}
}

// TestJourneyQuestRewardPref: Reward preferences lists the giver; unticking
// it sends SetRewardPref off and the read model declines its goodwill;
// ticking it again sends it back on.
func TestJourneyQuestRewardPref(t *testing.T) {
	t.Parallel()
	j, q := questsOffer(t, qd.Camp)
	if q.Giver == 0 {
		t.Fatal("the camp quest has no giver")
	}
	j.clickID(ids.QuestsPrefs)
	if j.a.uiSt().Top() != windows.QuestPrefsModalID {
		t.Fatalf("top modal %q, want the preferences", j.a.uiSt().Top())
	}
	pref := ids.QuestPref + fmt.Sprint(q.Giver)
	j.clickID(pref)
	if c, ok := j.lastCmd(command.SetRewardPref); !ok || c.Index != q.Giver || c.Enabled {
		t.Fatalf("SetRewardPref %+v (%v), want faction %d off", c, ok, q.Giver)
	}
	j.tick(1)
	if !slices.Contains(j.world().NoGoodwill, q.Giver) {
		t.Fatalf("NoGoodwill %v lacks %d", j.world().NoGoodwill, q.Giver)
	}
	j.reset()
	j.clickID(pref)
	if c, ok := j.lastCmd(command.SetRewardPref); !ok || c.Index != q.Giver || !c.Enabled {
		t.Fatalf("SetRewardPref %+v (%v), want faction %d on", c, ok, q.Giver)
	}
	j.tick(1)
	if slices.Contains(j.world().NoGoodwill, q.Giver) {
		t.Fatalf("NoGoodwill %v still holds %d", j.world().NoGoodwill, q.Giver)
	}
}

// TestJourneyQuestJumpToSite: Jump to the site closes the window and opens
// the world view on the quest's tile; back on the map, Jump to the colony
// closes the window on the home map.
func TestJourneyQuestJumpToSite(t *testing.T) {
	t.Parallel()
	j, q := questsOffer(t, qd.Request)
	if q.Tile < 0 {
		t.Fatal("the request has no site")
	}
	j.clickID(ids.QuestJump + fmt.Sprint(q.ID))
	if !j.a.world.Open() || j.a.world.Selected() != fmt.Sprintf("tile:%d", q.Tile) {
		t.Fatalf("world open %v selected %q, want tile:%d", j.a.world.Open(), j.a.world.Selected(), q.Tile)
	}
	if j.a.win.KitID() == windows.QuestsModalID {
		t.Fatal("the window stayed open over the world")
	}
	j.key("F8", 0)
	j.update(worldFly)
	if j.a.world.Open() {
		t.Fatal("F8 did not close the world view")
	}
	j.key("F7", 0)
	j.clickID(ids.QuestColony + fmt.Sprint(q.ID))
	if j.a.win.KitID() == windows.QuestsModalID || j.g.ViewedMap() != 0 {
		t.Fatalf("after Jump to the colony: window %q, map %d", j.a.win.KitID(), j.g.ViewedMap())
	}
}
