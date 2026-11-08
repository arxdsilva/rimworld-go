package worldview

import (
	"slices"
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
)

func TestQuestObjectsFollowTheRules(t *testing.T) {
	t.Parallel()
	r := readModels{
		tiles: 100,
		maps:  []game.MapView{{ID: 1, Kind: "site", Label: "Camp", Tile: 5, Colonists: 1}},
		sites: []game.WorldSiteView{{Tile: 6, Name: "Oakhold"}},
		quests: []game.QuestView{
			{ID: 3, Kind: qd.Camp, State: qd.Offered, Tile: 9},
			{ID: 4, Kind: qd.Stash, State: qd.Active, Tile: 5},    // a map holds the tile
			{ID: 5, Kind: qd.Request, State: qd.Active, Tile: 6},  // a settlement holds it
			{ID: 6, Kind: qd.Joiner, State: qd.Offered, Tile: -1}, // no site
			{ID: 7, Kind: qd.Rescue, State: qd.Succeeded, Tile: 8},
			{ID: 8, Kind: qd.Talks, State: qd.Active, Tile: 700}, // off the planet
		},
	}
	var keys []string
	for _, o := range buildObjects(nil, r) {
		keys = append(keys, o.Key)
	}
	if want := []string{"map:1", "settlement:6", "quest:3"}; !slices.Equal(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
}

func TestQuestSiteMarkerAppearsAndClears(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	sh, v := newFake(g)
	tile := landTile(t, 90)
	set := func(s qd.State) {
		sh.world.Quests = []game.QuestView{{ID: 4, Kind: qd.Camp, State: s, Tile: tile, Deadline: 100000}}
		sh.tick += refreshTicks
		frame(sh, v, -1, -1, inputframe.Raw{})
	}
	has := func() bool { return find(v.objs, "quest:4") >= 0 }
	v.Show("")
	frame(sh, v, -1, -1, inputframe.Raw{})
	if has() {
		t.Fatal("no quest, no marker")
	}
	set(qd.Offered)
	if !has() {
		t.Fatal("an offered quest has a marker")
	}
	set(qd.Active)
	if !has() {
		t.Fatal("an active quest keeps its marker")
	}
	v.selectKey("quest:4")
	set(qd.Failed)
	if has() || v.Selected() != "" {
		t.Fatalf("an ended quest clears its marker and selection (sel %q)", v.Selected())
	}
}

func TestQuestSiteInspectorViewsQuest(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	sh, v := newFake(g)
	sh.world.Quests = []game.QuestView{{ID: 4, Kind: qd.Camp, State: qd.Active, Tile: landTile(t, 90), Deadline: 4 * 60000}}
	v.Show("quest:4")
	frame(sh, v, -1, -1, inputframe.Raw{})
	title, lines, gs := (inspector{v}).inspect()
	if title != "Hostile camp" || len(lines) != 3 || lines[0].label != "Faction" || lines[1].label != "Time left" {
		t.Fatalf("pane %q %v", title, lines)
	}
	if len(gs) != 1 || gs[0].why != "" {
		t.Fatalf("gizmos %v", gs)
	}
	sh.intents = nil
	gizmoOf(t, v, "quest").run()
	if len(sh.intents) != 1 || sh.intents[0] != (ports.ShowQuest{ID: 4}) {
		t.Fatalf("intents %v", sh.intents)
	}
}

func TestCaravanOffersTheQuestActionsOnTheQuestTile(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	sh, v := newFake(g)
	c := g.world.Caravans[0]
	sh.world.Quests = []game.QuestView{
		{ID: 4, Kind: qd.Request, State: qd.Active, Tile: c.Tile},
		{ID: 5, Kind: qd.Talks, State: qd.Offered, Tile: c.Tile},
		{ID: 6, Kind: qd.Request, State: qd.Offered, Tile: c.Tile}, // not yet active
		{ID: 7, Kind: qd.Talks, State: qd.Active, Tile: c.Tile + 1},
	}
	v.Show("caravan:" + strconv.FormatUint(c.ID, 10))
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, _, gs := (inspector{v}).inspect()
	n := 0
	for _, gz := range gs {
		if gz.name == "fulfil" || gz.name == "negotiate" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d quest gizmos in %v", n, gs)
	}
	gizmoOf(t, v, "fulfil").run()
	gizmoOf(t, v, "negotiate").run()
	want := []command.Command{{Kind: command.FulfilRequest, Index: 4, Target: c.ID}, {Kind: command.Negotiate, Index: 5, Target: c.ID}}
	if !slices.Equal(sh.cmds, want) {
		t.Fatalf("cmds %+v", sh.cmds)
	}
	sh.world.Quests = nil
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, _, gs = (inspector{v}).inspect()
	for _, gz := range gs {
		if gz.name == "fulfil" || gz.name == "negotiate" {
			t.Fatalf("gizmo %q with no quest", gz.name)
		}
	}
}
