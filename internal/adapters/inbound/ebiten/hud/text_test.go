package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The room card and the Alt detail lines read the room view and the
// stacks on the cell.
func TestRoomLines(t *testing.T) {
	t.Parallel()
	rm := game.RoomView{ID: 4, Role: "bedroom", Score: 12, Label: "decent", Temp: 214, Lit: true, Roofed: 0.5, Cells: 20}
	got := strings.Join(RoomCardLines(rm), "|")
	want := "Bedroom|Impressiveness 12 (decent)|Cleanliness: clean|Temperature 21C|Lit: yes   Roofed: 50%|20 cells"
	if got != want {
		t.Fatalf("card = %q, want %q", got, want)
	}
	if RoomTint(-10).R <= RoomTint(30).R || RoomTint(30).G <= RoomTint(-10).G || RoomTint(5).A != 90 {
		t.Fatalf("tints: %v %v %v", RoomTint(-10), RoomTint(5), RoomTint(30))
	}
	w := game.WorldView{Items: []game.ItemView{{ID: 9, Kind: "rice", Qty: 3, X: 1, Y: 1, Forbidden: true}}}
	if d := strings.Join(DetailLines(w, 1, 1), "|"); !strings.Contains(d, "Cell (1, 1)") || !strings.Contains(d, "#9 rice x 3 [forbidden]") {
		t.Fatalf("detail = %q", d)
	}
}

// Messages fade over their last frames; a letter notice only names it and
// a danger is drawn red.
func TestMessageLooks(t *testing.T) {
	t.Parallel()
	if MessageAlpha(0) != 1 || MessageAlpha(MessageFrames) != 0 || MessageAlpha(MessageFrames-MessageFadeFrames/2) != 0.5 {
		t.Fatal("alpha")
	}
	if MessageLine(game.MessageView{Letter: true, Title: "Raid", Text: "long"}) != "Letter: Raid" || MessageLine(game.MessageView{Text: "x"}) != "x" {
		t.Fatal("line")
	}
	if MessageStyle(game.MessageView{Danger: true}).Color != uikit.Danger || MessageEdge(game.MessageView{}).A != 0 ||
		MessageEdge(game.MessageView{Kind: "good"}) != LetterColor("good") {
		t.Fatal("style or edge")
	}
	if FmtTemp(-25) != "-3C" || FmtTemp(25) != "3C" || FmtDegrees(-25) != "-3 °C" {
		t.Fatal("temperatures")
	}
}

// The text rows of the HUD: gatherings, colonists, conditions.
func TestHUDTexts(t *testing.T) {
	t.Parallel()
	if s := GatheringText(game.GatheringView{Kind: "wedding", Hosts: []string{"Ada", "Bo"}, Phase: "ceremony"}); s != "Wedding of Ada and Bo: ceremony" {
		t.Fatalf("wedding = %q", s)
	}
	if s := GatheringText(game.GatheringView{Kind: "party"}); s != "Party" {
		t.Fatalf("party = %q", s)
	}
	if PawnLabel(game.PawnView{ID: 7}) != "Colonist 7" || BarTip(game.PawnView{Name: "Ada", AgeYears: 31, Title: "Mechanic"}) != "Ada, 31, Mechanic" {
		t.Fatal("colonist labels")
	}
	chips := StatusChips(game.WorldView{Extreme: "cold_snap", SolarFlare: true})
	if len(chips) != 2 || chips[0].Label != "Cold snap" || chips[1].Label != "Solar flare" {
		t.Fatalf("chips = %+v", chips)
	}
}

// Without stored goods the list falls back to the loose totals, flat or
// under one header per category.
func TestBuildResourceRowsFallsBackToTotals(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Food: 5, Wood: 7, Silver: 2}
	flat := BuildResourceRows(w, false, nil)
	if len(flat) != 5 || flat[0].Kind != "food" || flat[4].Kind != "silver" {
		t.Fatalf("flat = %+v", flat)
	}
	cat := BuildResourceRows(w, true, map[string]bool{"Materials": true})
	if len(cat) != 3 || !cat[0].Header || cat[0].Cat != "Food" || !cat[2].Header || cat[2].Qty != 7+2 {
		t.Fatalf("categorised = %+v", cat)
	}
}
