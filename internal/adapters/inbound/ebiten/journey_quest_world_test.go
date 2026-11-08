package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// QST-3 world journeys: a quest site on the globe, and the caravan's quest
// actions on the tile of a request or of peace talks.

// offerQuest offers a quest of kind k with the dev command and returns its
// view; accept makes it active.
func offerQuest(t *testing.T, j *journey, k qd.KindID, accept bool) game.QuestView {
	t.Helper()
	j.g.Enqueue(command.Command{Kind: command.DevOfferQuest, Index: int(k)})
	j.g.Step(2)
	var q game.QuestView
	for _, v := range j.g.World().Quests {
		if v.Kind == k {
			q = v
		}
	}
	if q.ID == 0 || q.Tile < 0 {
		t.Fatalf("no quest of kind %d offered with a site: %+v", k, j.g.World().Quests)
	}
	if accept {
		j.g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(q.ID), Count: 0})
		j.g.Step(2)
		for _, v := range j.g.World().Quests {
			if v.ID == q.ID {
				q = v
			}
		}
		if q.State != qd.Active {
			t.Fatalf("quest %d is %v, want active", q.ID, q.State)
		}
	}
	return q
}

// TestJourneyQuestSiteViewQuest: an offered quest is marked on the globe;
// picking the mark shows its pane and View quest opens the Quests window.
func TestJourneyQuestSiteViewQuest(t *testing.T) {
	t.Parallel()
	j, _ := worldColony(t)
	q := offerQuest(t, j, qd.Camp, false)
	worldOpen(j)
	j.update(2)
	key := "quest:" + strconv.FormatUint(uint64(q.ID), 10)
	j.a.world.Show(key)
	j.update(worldFly)
	if j.a.world.Selected() != key {
		t.Fatalf("selected %q, want %q", j.a.world.Selected(), key)
	}
	id := "world.obj.quest." + strconv.FormatUint(uint64(q.ID), 10)
	if !hasWidget(j.a, id) {
		t.Fatalf("no marker %s on the globe", id)
	}
	if !j.clickID("world.act.quest") {
		t.Fatal("View quest is disabled")
	}
	j.update(2)
	if j.a.uiModalTop() != windows.QuestsModalID {
		t.Fatalf("modal %q, want the Quests window (ShowQuest %d)", j.a.uiModalTop(), q.ID)
	}
}

// talksColony is a wizard colony with a hostile people that can be friends
// (an attack on a neutral settlement made one) and a reformed caravan
// standing where the assault was.
func talksColony(t *testing.T) *journey {
	t.Helper()
	j, p := worldColony(t)
	s, _ := nearestSite(j, p, false)
	c := sendCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	j.g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: c.ID})
	j.g.Step(60)
	for _, m := range j.g.Maps() { // the assault opened a map: pack up again
		if m.Kind != "colony" {
			j.g.Enqueue(command.Command{Kind: command.CaravanReform, Target: uint64(m.ID)})
			j.g.Step(60)
		}
	}
	if len(j.g.WorldMap().Caravans) != 1 {
		t.Fatalf("caravans %+v", j.g.WorldMap().Caravans)
	}
	return j
}

// caravanAtQuest is a wizard colony whose caravan stands on the tile of an
// active quest of kind k, selected in the world view. A caravan that walks
// onto active talks holds them at once, so for talks the colony is built
// twice: the first run learns the tile the offer picks (the games are
// deterministic), the second sets the caravan down there before the offer.
func caravanAtQuest(t *testing.T, k qd.KindID) (*journey, game.QuestView, game.CaravanView) {
	t.Helper()
	var j *journey
	var q game.QuestView
	var c game.CaravanView
	if k == qd.Talks {
		tile := offerQuest(t, talksColony(t), k, false).Tile
		j = talksColony(t)
		cid := j.g.WorldMap().Caravans[0].ID
		game.DevArrive(j.g, entity.EntityID(cid), tile)
		j.g.Step(2)
		q = offerQuest(t, j, k, false)
		if q.Tile != tile {
			t.Fatalf("the offer moved from tile %d to %d", tile, q.Tile)
		}
		c = j.g.WorldMap().Caravans[0]
	} else {
		j, _ = worldColony(t)
		q = offerQuest(t, j, k, true)
		c = sendCaravan(j, j.g.Snapshot()[0].ID, q.Tile)
	}
	worldOpen(j)
	key := "caravan:" + strconv.FormatUint(c.ID, 10)
	j.a.world.Show(key)
	j.update(worldFly)
	return j, q, c
}

// TestJourneyCaravanFulfilRequest: Fulfil request sends FulfilRequest with
// the quest and the caravan.
func TestJourneyCaravanFulfilRequest(t *testing.T) {
	t.Parallel()
	j, q, c := caravanAtQuest(t, qd.Request)
	j.update(2)
	if !j.clickID("world.act.fulfil") {
		t.Fatal("Fulfil request is disabled or missing")
	}
	got, ok := j.lastCmd(command.FulfilRequest)
	if !ok || got.Index != int(q.ID) || got.Target != c.ID {
		t.Fatalf("FulfilRequest %+v ok=%v, want quest %d caravan %d", got, ok, q.ID, c.ID)
	}
}

// TestJourneyCaravanNegotiate: Negotiate sends Negotiate with the quest and
// the caravan.
func TestJourneyCaravanNegotiate(t *testing.T) {
	t.Parallel()
	j, q, c := caravanAtQuest(t, qd.Talks)
	j.update(2)
	if !j.clickID("world.act.negotiate") {
		t.Fatal("Negotiate is disabled or missing")
	}
	got, ok := j.lastCmd(command.Negotiate)
	if !ok || got.Index != int(q.ID) || got.Target != c.ID {
		t.Fatalf("Negotiate %+v ok=%v, want quest %d caravan %d", got, ok, q.ID, c.ID)
	}
}
