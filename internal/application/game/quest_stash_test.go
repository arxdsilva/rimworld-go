package game

// quest_stash_test.go: the item stash through the game (QST-050..054):
// offered and active at once, a caravan reaching the tile opens its site
// map, the goods lie on it, the guards stand and the quest has succeeded.

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

func TestStashSiteOpensWithGoodsAndGuards(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	home := -1
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() && len(f.Settlements) > 0 {
			f.Relation, f.Defeated = faction.Hostile, false
			home = i
			break
		}
	}
	if home < 0 {
		t.Fatal("fixture: no human faction with a settlement")
	}
	id, ok := questOf(g).Offer(qd.Stash)
	if !ok {
		t.Fatal("the stash was not offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Active || v.Tile < 0 || len(v.Rewards) != 1 || len(v.Rewards[0].Lines) == 0 {
		t.Fatalf("offer: %+v", v)
	}
	want := map[item.Kind]int{}
	for _, l := range v.Rewards[0].Lines {
		want[l.Kind] += l.Qty
	}
	// A caravan is placed on the tile and Fleet.Arrived is called, as the
	// road does when a real one stops there (walking it takes days).
	c := visitingCaravan(t, g, v.Tile)
	fleetOf(g).Arrived(c, uint64(g.clock.Now()))
	g.Step(1) // the open request is flushed at the tick's end
	if len(g.maps) != 2 {
		t.Fatalf("no site map opened: %d maps", len(g.maps))
	}
	if v := questView(t, g, id); v.State != qd.Succeeded {
		t.Fatalf("reaching the stash succeeds it: %+v", v)
	}
	got := map[item.Kind]int{}
	site := g.maps[1]
	for _, sid := range site.itemIDs {
		if st, ok := site.stacks.Get(sid); ok {
			got[st.Kind] += st.Qty
		}
	}
	for k, n := range want {
		if got[k] < n {
			t.Fatalf("%v: %d on the map, want at least %d", k, got[k], n)
		}
	}
	guards := 0
	for _, grp := range site.raidGroups {
		if grp.Faction == questView(t, g, id).Target {
			guards += grp.Size
		}
	}
	if guards == 0 {
		t.Fatalf("no guards of faction ref %d: %+v", questView(t, g, id).Target, site.raidGroups)
	}
}
