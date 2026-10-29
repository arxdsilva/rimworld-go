package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

func TestItemChipsDisabledWithReason(t *testing.T) {
	t.Parallel()
	equip := ColonistOptions([]ColonistAction{{Pawn: 1, Name: "Ada"}, {Pawn: 2, Name: "Bo", Disabled: true, Reason: "cannot reach"}})
	haul := ColonistOptions([]ColonistAction{{Pawn: 1, Name: "Ada"}})
	if b := equip[1]; !b.Disabled || b.Reason != "cannot reach" || b.Label != "Bo" || b.Cmd != nil {
		t.Fatalf("option 1 = %+v", b)
	}
	// the Haul command sits after the Equip one in the grid
	var ids []string
	for _, v := range ItemVerbs(game.ItemView{Kind: "rifle"}, false) {
		ids = append(ids, v.ID)
	}
	if strings.Join(ids, ",") != "cmd.item.equip,cmd.item.haul" {
		t.Fatalf("verbs = %v", ids)
	}
	if n := len(equip) + len(haul); n != 3 {
		t.Fatalf("options = %d", n)
	}
}

func TestItemCorpseTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		it    game.ItemView
		title string
		cat   string
	}{
		{game.ItemView{Kind: "human corpse", OfName: "Ada", Qty: 1}, "Body of Ada", "body"},
		{game.ItemView{Kind: "bones", OfName: "Ada", Qty: 1}, "Bones of Ada", "body"},
		{game.ItemView{Kind: "human corpse"}, "Body", "body"},
		{game.ItemView{Kind: "bones"}, "Bones", "body"},
		{game.ItemView{Kind: "rifle", Qty: 1}, "Rifle", "weapon"},
		{game.ItemView{Kind: "wood", Qty: 25}, "Wood ×25", "material"},
		{game.ItemView{Kind: "parka", Qty: 1}, "Parka", "apparel"},
		{game.ItemView{Kind: "meal", Qty: 2}, "Meal ×2", "food"},
		{game.ItemView{Kind: "dustbuck corpse", Qty: 1}, "Dustbuck corpse", "body"},
	}
	for _, c := range cases {
		if got := ItemTitle(c.it); got != c.title {
			t.Errorf("title(%+v) = %q, want %q", c.it, got, c.title)
		}
		if got := ItemCategory(c.it); got != c.cat {
			t.Errorf("category(%s) = %q, want %q", c.it.Kind, got, c.cat)
		}
	}
	for _, c := range []struct {
		it   game.ItemView
		want string
	}{
		{game.ItemView{Rotten: true}, "Rotten"},
		{game.ItemView{Dried: true}, "Dried"},
		{game.ItemView{FreshDays: 4}, "Fresh for 4 days"},
	} {
		if got := ItemLines(c.it, true)[0]; got != c.want {
			t.Errorf("condition = %q, want %q", got, c.want)
		}
	}
	if got := ItemLines(game.ItemView{FreshDays: -1}, true); len(got) != 1 {
		t.Fatalf("non-spoiling item lines = %v", got)
	}
}

// TestItemShowsPoisonChance: a cooked meal's pane names its food-poisoning
// chance (WS-M); raw food and older stacks show none.
func TestItemShowsPoisonChance(t *testing.T) {
	t.Parallel()
	has := func(lines []string) string {
		for _, l := range lines {
			if strings.HasPrefix(l, "Poison chance") {
				return l
			}
		}
		return ""
	}
	if l := has(ItemLines(game.ItemView{Kind: "meal", Qty: 2, FreshDays: 3, PoisonPermille: 20}, true)); l != "Poison chance 2%" {
		t.Fatalf("meal line %q", l)
	}
	if l := has(ItemLines(game.ItemView{Kind: "meal", Qty: 1, FreshDays: 3, PoisonPermille: 15}, true)); l != "Poison chance 1.5%" {
		t.Fatalf("meal line %q", l)
	}
	if l := has(ItemLines(game.ItemView{Kind: "berries", Qty: 5, FreshDays: 3}, true)); l != "" {
		t.Fatalf("raw food shows %q", l)
	}
}

func TestPlantZoneLine(t *testing.T) {
	t.Parallel()
	got := PlantLines(game.PlantView{Species: "potato", Growth: 10, Zone: "potato", Wilted: true, Cut: true})
	want := []string{"Not ready (10%)", "Wilting: too cold", "In a growing zone (Potato)", "Marked to cut"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	tree := PlantLines(game.PlantView{Species: "tree", Zone: "tree"})
	if len(tree) != 1 || tree[0] != "In a growing zone (Tree)" {
		t.Fatalf("tree lines = %v", tree)
	}
	if PlantCategory(game.PlantView{Species: "berry_bush"}) != "Wild plant" ||
		PlantTitle(game.PlantView{Species: "berry_bush"}) != "Berry bush" ||
		PlantCategory(game.PlantView{Species: "potato"}) != "Crop" {
		t.Fatal("categories/titles wrong")
	}
	if PlantStageWord(game.PlantView{Stage: 1}) != "sprouting" ||
		PlantStageWord(game.PlantView{Species: "corn", Harvestable: true, Stage: 3}) != "ready to harvest" {
		t.Fatal("stage words wrong")
	}
	if got := PlantLines(game.PlantView{Species: "corn", Harvestable: true, Designated: true}); got[1] != "Marked to harvest" {
		t.Fatalf("lines = %v", got)
	}
}

func TestPlantGrowCaption(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		p    game.PlantView
		gv   game.PlantGrowthView
		want string
	}{
		{"no state", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{}, ""},
		{"growing", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "growing"}, "42% · growing"},
		{"resting at night", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "resting"}, "42% · resting (night)"},
		{"dark", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "dark"}, "too dark: needs sun or a lamp nearby"},
		{"cold", game.PlantView{Species: "corn", Growth: 7}, game.PlantGrowthView{State: "cold"}, "7% · too cold"},
		{"hot", game.PlantView{Species: "corn", Growth: 7}, game.PlantGrowthView{State: "hot"}, "7% · too hot"},
		{"ripe crop", game.PlantView{Species: "corn", Growth: 100, Harvestable: true}, game.PlantGrowthView{State: "ripe"}, "ripe"},
		{"full-grown tree", game.PlantView{Species: "tree", Growth: 100}, game.PlantGrowthView{State: "ripe"}, "fully grown"},
		{"unknown state", game.PlantView{Species: "corn"}, game.PlantGrowthView{State: "weird"}, ""},
	} {
		if got := PlantGrowCaption(c.p, c.gv); got != c.want {
			t.Errorf("%s: caption %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPlantRipeLine(t *testing.T) {
	t.Parallel()
	d := clock.TicksPerDay
	for _, c := range []struct {
		name   string
		p      game.PlantView
		ripeIn int
		want   string
	}{
		{"unknown", game.PlantView{Species: "corn"}, -1, ""},
		{"unset", game.PlantView{Species: "corn"}, 0, ""},
		{"under two days: one decimal", game.PlantView{Species: "corn"}, d * 14 / 10, "Ripe in about 1.4 days"},
		{"just under two reads as two", game.PlantView{Species: "corn"}, d*2 - 1, "Ripe in about 2 days"},
		{"1.9 stays decimal", game.PlantView{Species: "corn"}, d * 19 / 10, "Ripe in about 1.9 days"},
		{"two days and more: whole", game.PlantView{Species: "corn"}, d*3 + d/3, "Ripe in about 3 days"},
		{"rounds", game.PlantView{Species: "corn"}, d*7 + d*6/10, "Ripe in about 8 days"},
		{"an hour left", game.PlantView{Species: "corn"}, clock.TicksPerHour, "Ripe in about 0.1 days"},
		{"a tree is fully grown, not ripe", game.PlantView{Species: "tree"}, d * 30, "Fully grown in about 30 days"},
		{"moss is fully grown", game.PlantView{Species: "moss"}, d * 2, "Fully grown in about 2 days"},
		{"a berry bush ripens", game.PlantView{Species: "berry_bush"}, d, "Ripe in about 1.0 days"},
		{"nothing once harvestable", game.PlantView{Species: "corn", Harvestable: true}, d, ""},
	} {
		if got := PlantRipeLine(c.p, game.PlantGrowthView{RipeIn: c.ripeIn}); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// TestPlantLinesCarryRipeIn: the ripening estimate sits right under the
// progress line, and the old lines stay as they were.
func TestPlantLinesCarryRipeIn(t *testing.T) {
	t.Parallel()
	got := PlantBodyLines(game.PlantView{Species: "potato", Growth: 42, Zone: "potato"},
		game.PlantGrowthView{State: "growing", RipeIn: clock.TicksPerDay * 3})
	want := []string{"Not ready (42%)", "Ripe in about 3 days", "In a growing zone (Potato)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if strings.Join(PlantLines(game.PlantView{Species: "potato", Growth: 42, Zone: "potato"}), "|") != "Not ready (42%)|In a growing zone (Potato)" {
		t.Fatal("the view-only lines carry an estimate")
	}
}

func TestItemVerbOptionsCarryDisabledReasons(t *testing.T) {
	t.Parallel()
	opts := ColonistOptions([]ColonistAction{
		{Pawn: 1, Name: "Ada", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip, Pawn: 1}},
		{Pawn: 2, Name: "Bo", Disabled: true, Reason: "cannot reach"},
	})
	if len(opts) != 2 || opts[0].Disabled || opts[0].Cmd == nil || opts[0].Cmd.Pawn != 1 {
		t.Fatalf("options = %+v", opts)
	}
	if !opts[1].Disabled || opts[1].Reason != "cannot reach" || opts[1].Label != "Bo" {
		t.Fatalf("the unreachable colonist's option = %+v", opts[1])
	}
	if got := ColonistOptions(nil); len(got) != 0 {
		t.Fatalf("no colonists: %+v", got)
	}
}
