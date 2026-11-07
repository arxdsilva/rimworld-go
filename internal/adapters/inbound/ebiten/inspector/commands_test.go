package inspector

import (
	"reflect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"strings"
	"testing"
)

// A command label is drawn at gfx.Small and never shrinks (UIS-051): words
// wrap onto at most two lines, a word is never broken, and a line still too
// wide for the cell ends in an ellipsis.
func TestCommandLabelWrapsThenEllipsizes(t *testing.T) {
	maxW := float64(hudlayout.CmdCell - 2*CmdLabelPad)
	for _, label := range []string{"Deconstruct", "Mark for hauling", "Cancel blueprint", "Draft", "Supercalifragilisticexpialidocious", "Set the plant to grow in this zone"} {
		st, lines := CmdLabelLines(label, maxW, false)
		if st.Size != gfx.Small.Size {
			t.Errorf("%q: drawn at %v, want gfx.Small %v", label, st.Size, gfx.Small.Size)
		}
		if len(lines) == 0 || len(lines) > 2 {
			t.Errorf("%q: %d lines, want 1 or 2", label, len(lines))
		}
		for _, ln := range lines {
			if w, _ := gfx.Measure(ln, st); w > maxW+0.01 {
				t.Errorf("%q: line %q is %.1f wide, cell text room %.1f", label, ln, w, maxW)
			}
			for _, word := range strings.Fields(strings.TrimSuffix(ln, "…")) {
				if !strings.Contains(label, word) {
					t.Errorf("%q: line %q has a word not in the label", label, ln)
				}
			}
		}
		if joined := strings.Join(lines, " "); !strings.HasSuffix(joined, "…") && joined != label {
			t.Errorf("%q: lines %q lose text without an ellipsis", label, lines)
		}
	}
	if _, lines := CmdLabelLines("Supercalifragilisticexpialidocious", maxW, false); len(lines) != 1 || !strings.HasSuffix(lines[0], "…") {
		t.Errorf("one long word ellipsizes on one line, got %q", lines)
	}
}

func keysOf(cs []Button) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Key
	}
	return out
}

func TestMergeKeepsTheFirstDeclaredOrder(t *testing.T) {
	t.Parallel()
	sets := [][]Button{
		{{Key: "a", Label: "A"}, {Key: "b", Label: "B"}},
		{{Key: "b", Label: "B"}, {ID: "cmd.pawn.c", Label: "C"}, {Key: "a", Label: "A"}},
	}
	got := MergeCommands(sets, nil)
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	want := []string{"cmd.group.a", "cmd.group.b", "cmd.group.pawn.c"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("merged ids = %v, want %v (first declaration wins; a missing Key falls back to the ID)", ids, want)
	}
	if got[0].Label != "A" || got[2].Label != "C" {
		t.Fatalf("labels = %q %q", got[0].Label, got[2].Label)
	}
}

func TestMergeDisabledOnlyWhenEveryMemberIs(t *testing.T) {
	t.Parallel()
	off := func(reason string) Button {
		return Button{Key: "bed", Label: "Bed", Disabled: true, Reason: reason}
	}
	on := Button{Key: "bed", Label: "Bed"}

	if got := MergeCommands([][]Button{{off("no bed")}, {off("no bed")}, {on}}, nil)[0]; got.Disabled {
		t.Fatalf("one capable member keeps the merged command enabled: %+v", got)
	}
	got := MergeCommands([][]Button{{off("no bed")}, {off("no bed")}, {off("asleep")}}, nil)[0]
	if !got.Disabled || got.Reason != "no bed (2 of 3)" {
		t.Fatalf("all disabled: Disabled=%v Reason=%q, want %q", got.Disabled, got.Reason, "no bed (2 of 3)")
	}
}

func TestMergeToggleOnAndMixed(t *testing.T) {
	t.Parallel()
	tog := func(on bool) Button { return Button{Key: "draft", Label: "Draft", Toggle: true, On: on} }
	cases := []struct {
		name         string
		members      []bool
		on, mixed    bool
		wantRunsOnly []int // members a click runs
	}{
		{"all on", []bool{true, true, true}, true, false, []int{0, 1, 2}},
		{"all off", []bool{false, false}, false, false, []int{0, 1}},
		{"mixed", []bool{true, false, false}, false, true, []int{1, 2}},
	}
	for _, c := range cases {
		var sets [][]Button
		for _, on := range c.members {
			sets = append(sets, []Button{tog(on)})
		}
		var ran []int
		got := MergeCommands(sets, func(member int, _ Button) { ran = append(ran, member) })[0]
		if !got.Toggle || got.On != c.on || got.Mixed != c.mixed {
			t.Errorf("%s: Toggle=%v On=%v Mixed=%v, want On=%v Mixed=%v", c.name, got.Toggle, got.On, got.Mixed, c.on, c.mixed)
		}
		got.Run()
		if !reflect.DeepEqual(ran, c.wantRunsOnly) {
			t.Errorf("%s: a click ran members %v, want those whose On equals the merged On: %v", c.name, ran, c.wantRunsOnly)
		}
	}
}

func TestMergedClickSkipsDisabledMembersAndDropsOptions(t *testing.T) {
	t.Parallel()
	opt := func() []Option { return []Option{{Label: "x"}} }
	sets := [][]Button{
		{{Key: "k", Label: "K", Options: opt}},
		{{Key: "k", Label: "K", Disabled: true, Reason: "no"}},
		{{Key: "k", Label: "K"}},
	}
	var ran []int
	got := MergeCommands(sets, func(member int, _ Button) { ran = append(ran, member) })[0]
	if got.Options != nil {
		t.Fatal("options are per member: a merged command has none")
	}
	got.Run()
	if !reflect.DeepEqual(ran, []int{0, 2}) {
		t.Fatalf("ran members %v, want the enabled ones [0 2]", ran)
	}
}

func TestMergeKeepsAKeyFromOnlySomeMembers(t *testing.T) {
	t.Parallel()
	got := MergeCommands([][]Button{{{Key: "only"}}, {{Key: "other"}}}, nil)
	if !reflect.DeepEqual(keysOf(got), []string{"only", "other"}) {
		t.Fatalf("merged keys = %v", keysOf(got))
	}
}
