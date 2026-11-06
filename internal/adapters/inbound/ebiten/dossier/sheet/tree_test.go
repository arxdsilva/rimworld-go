package sheet

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

func TestWhoOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    game.PawnView
		want Who
	}{
		{"colonist", game.PawnView{}, WhoColonist},
		{"prisoner", game.PawnView{Prisoner: true}, WhoStranger},
		{"visitor", game.PawnView{Visitor: true}, WhoStranger},
		{"ally", game.PawnView{Ally: true}, WhoStranger},
		{"raider", game.PawnView{Hostile: true}, WhoStranger},
	}
	for _, c := range cases {
		if got := WhoOf(c.p); got != c.want {
			t.Errorf("%s: %v want %v", c.name, got, c.want)
		}
	}
	if WhoOfAnimal(game.AnimalView{Tame: true}) != WhoPrisoner || WhoOfAnimal(game.AnimalView{}) != WhoWild ||
		WhoOfAnimal(game.AnimalView{Mech: true}) != WhoWild {
		t.Fatal("animals: tame is p, anything else (mechs too) is w")
	}
	for w, want := range map[Who]byte{WhoColonist: 'c', WhoStranger: 's', WhoPrisoner: 'p', WhoWild: 'w'} {
		if w.Letter() != want {
			t.Errorf("letter of %d", w)
		}
	}
}

func TestPruneTargets(t *testing.T) {
	t.Parallel()
	tree := box(KindColumn,
		widgetFor("mood_bar", "cs"),
		boxFor(KindBlock, "c", box(KindLine, generic("skill", "melee"))),
		box(KindLine, widget("standing"), widgetFor("hour_pick", "c"), widget("doing")),
		boxFor(KindLine, "", widgetFor("hour_pick", "c")),
	)
	got := Prune(tree, WhoWild)
	if len(got.Kids) != 1 {
		t.Fatalf("wild kept %d children, want only the line: %+v", len(got.Kids), got.Kids)
	}
	line := got.Kids[0]
	if len(line.Kids) != 3 || line.Kids[1].Kind != KeyGap || line.Kids[1].Def != "hour_pick" {
		t.Fatalf("an off-target widget in a line keeps its slot as a blank carrying its key: %+v", line.Kids)
	}
	if len(tree.Kids) != 4 || tree.Kids[2].Kids[1].Kind != "hour_pick" {
		t.Fatal("Prune changed its input")
	}
	if c := Prune(tree, WhoColonist); len(c.Kids) != 4 {
		t.Fatalf("colonist sees everything, got %d", len(c.Kids))
	}
	if root := Prune(boxFor(KindColumn, "p", widget("doing")), WhoColonist); root.Kind != "" {
		t.Fatalf("an off-target root is gone: %+v", root)
	}
	// a container whose children are all blanks is dropped
	only := box(KindColumn, box(KindLine, widgetFor("hour_pick", "c")), widget("doing"))
	if p := Prune(only, WhoWild); len(p.Kids) != 1 || p.Kids[0].Kind != "doing" {
		t.Fatalf("all-blank line should drop: %+v", p)
	}
}

func testMetrics() Metrics {
	return Metrics{BodyH: 10, LargeH: 20, SmallH: 8, BarH: 12, GapH: 4}
}

func TestMeasure(t *testing.T) {
	t.Parallel()
	m := testMetrics()
	col := box(KindColumn, widget("title"), widget("doing"), widget("mood_bar"), widget("rule"))
	if got := Measure(col, m, 0); got != 46 {
		t.Fatalf("column sums its children: %v", got)
	}
	strip := box(KindStrip, box(KindColumn, widget("doing"), widget("doing")), widget("title"))
	if got := Measure(strip, m, 0); got != 20 {
		t.Fatalf("strip is as tall as its tallest: %v", got)
	}
	line := box(KindLine, widget("doing"), widget("mood_bar"))
	if got := Measure(line, m, 0); got != 12 {
		t.Fatalf("line: %v", got)
	}
	filled := col
	filled.Fill = true
	if got := Measure(filled, m, 100); got != 100 {
		t.Fatalf("a filled column grows to the room: %v", got)
	}
	if got := Measure(filled, m, 10); got != 46 {
		t.Fatalf("and never shrinks below its content: %v", got)
	}
	if got := Measure(col, m, 100); got != 46 {
		t.Fatalf("an unfilled column does not grow: %v", got)
	}
	blank := box(KindLine, config.HUDNode{Kind: KeyGap, Def: "mood_bar"})
	if got := Measure(blank, m, 0); got != 12 {
		t.Fatalf("a blank is as tall as the widget it stands for: %v", got)
	}
	empty := func(n config.HUDNode) bool { return n.Kind == "doing" }
	if got := MeasureWith(box(KindColumn, widget("doing"), widget("title")), m, 0, empty); got != 20 {
		t.Fatalf("an empty widget collapses: %v", got)
	}
	if got := MeasureWith(box(KindLine, widget("doing")), m, 0, empty); got != 0 {
		t.Fatalf("and its row with it: %v", got)
	}
}

func TestHeightsShareFill(t *testing.T) {
	t.Parallel()
	m := testMetrics()
	a := fill(box(KindColumn, widget("doing")))
	b := fill(box(KindColumn, widget("doing")))
	root := box(KindColumn, widget("title"), a, b)
	hs := Heights(root, m, 100, nil)
	if hs[0] != 20 || hs[1] != 40 || hs[2] != 40 {
		t.Fatalf("two fills share the rest evenly: %v", hs)
	}
	hs = Heights(root, m, 30, nil)
	if hs[1] != 10 || hs[2] != 10 {
		t.Fatalf("no spare room, no growth: %v", hs)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	good := box(KindColumn, box(KindBlock, box(KindLine, generic("skill", "melee"))), widget("doing"))
	if ps := Validate(en, good); len(ps) != 0 {
		t.Fatalf("good tree: %v", ps)
	}
	bad := []struct {
		name string
		n    config.HUDNode
	}{
		{"unknown", box(KindColumn, widget("nope"))},
		{"no name", box(KindColumn, config.HUDNode{})},
		{"block holds widget", box(KindBlock, widget("doing"))},
		{"line holds line", box(KindLine, box(KindLine))},
		{"widget with kids", config.HUDNode{Kind: "doing", Kids: []config.HUDNode{widget("rule")}}},
		{"bad who", box(KindColumn, widgetFor("doing", "x"))},
		{"repeated who", box(KindColumn, widgetFor("doing", "cc"))},
		{"skill without def", box(KindColumn, widget("skill"))},
		{"skill with a stat def", box(KindColumn, generic("skill", "work_speed"))},
		{"bar style on text", box(KindColumn, config.HUDNode{Kind: "doing", BarStyle: BarFadeUp})},
		{"unknown bar style", box(KindColumn, config.HUDNode{Kind: "mood_bar", BarStyle: "zig"})},
		{"fill on a widget", box(KindColumn, config.HUDNode{Kind: "doing", Fill: true})},
	}
	for _, c := range bad {
		if len(Validate(en, c.n)) == 0 {
			t.Errorf("%s: no problem reported", c.name)
		}
	}
	if len(Validate(en, box(KindColumn, config.HUDNode{Kind: "mood_bar", BarStyle: BarFadeDown}))) != 0 {
		t.Error("a bar style on a bar is fine")
	}
	deep := widget("doing")
	for i := 0; i < config.HUDMaxDepth+1; i++ {
		deep = box(KindColumn, deep)
	}
	if len(Validate(en, deep)) == 0 {
		t.Error("too deep")
	}
	wide := box(KindColumn)
	for i := 0; i < config.HUDMaxNodes; i++ {
		wide.Kids = append(wide.Kids, widget("rule"))
	}
	if len(Validate(en, wide)) == 0 {
		t.Error("too many nodes")
	}
	if ps := Validate(en, box(KindColumn, widget("zz"))); ps[0].Path != "root.0" {
		t.Errorf("path: %q", ps[0].Path)
	}
}

func TestEditing(t *testing.T) {
	t.Parallel()
	root := box(KindColumn, widget("title"), widget("doing"), box(KindLine, widget("rule")))
	path, ok := Move(&root, []int{0}, +1)
	if !ok || !reflect.DeepEqual(path, []int{1}) || root.Kids[1].Kind != "title" || root.Kids[0].Kind != "doing" {
		t.Fatalf("move down: %v %v", path, root.Kids)
	}
	if _, ok := Move(&root, []int{0}, -1); ok {
		t.Fatal("moving past the top does nothing")
	}
	if _, ok := Move(&root, nil, 1); ok {
		t.Fatal("the root cannot move")
	}
	if !Remove(&root, []int{1}) || len(root.Kids) != 2 || root.Kids[1].Kind != KindLine {
		t.Fatalf("remove: %+v", root.Kids)
	}
	if Remove(&root, nil) || Remove(&root, []int{9}) {
		t.Fatal("bad removes")
	}
	np, err := Add(&root, []int{1}, -1, NewNode("doing"))
	if err != nil || !reflect.DeepEqual(np, []int{1, 1}) {
		t.Fatalf("add to a line: %v %v", np, err)
	}
	if _, err := Add(&root, []int{1}, 0, box(KindLine)); err != ErrCannotHold {
		t.Fatalf("a line cannot hold a line: %v", err)
	}
	if _, err := Add(&root, []int{7}, 0, widget("rule")); err != ErrNoSuchNode {
		t.Fatalf("no parent: %v", err)
	}
	np, err = Add(&root, nil, 0, NewNode("skill"))
	if err != nil || root.Kids[0].Kind != "skill" || root.Kids[0].Def != "shooting" || len(np) != 1 {
		t.Fatalf("a new skill gets the first skill: %+v %v", root.Kids[0], err)
	}
	if len(Validate(en, root)) != 0 {
		t.Fatalf("edited tree: %v", Validate(en, root))
	}
}

func TestSetWho(t *testing.T) {
	t.Parallel()
	n := widget("doing")
	if WhoSet(n) != WhoLetters {
		t.Fatal("empty is all four")
	}
	if err := SetWho(&n, WhoWild, false); err != nil || n.Who != "csp" {
		t.Fatalf("w off: %q %v", n.Who, err)
	}
	if err := SetWho(&n, WhoWild, true); err != nil || n.Who != "" {
		t.Fatalf("all four on is stored as empty: %q %v", n.Who, err)
	}
	n.Who = "w"
	if err := SetWho(&n, WhoWild, false); err != ErrLastTarget || n.Who != "w" {
		t.Fatalf("the last target stays: %q %v", n.Who, err)
	}
	n.Who = "w"
	if err := SetWho(&n, WhoColonist, true); err != nil || n.Who != "cw" {
		t.Fatalf("order is cspw: %q", n.Who)
	}
}
