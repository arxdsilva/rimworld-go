package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The Gear tab's own tests are in inspector/gear; these are the inspector
// sides that stayed: the item pane's quality and condition, its Info
// button, the pane's inspiration line and the door's forbid toggle.

func TestItemPaneShowsQualityAndCondition(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.world = game.WorldView{Items: []game.ItemView{{ID: 7, Kind: "club", Qty: 1, FreshDays: -1,
		Quality: "good", Stuff: "steel", HP: 20, MaxHP: 100, Tainted: true}}}
	spec := New(f).Thing().ItemPane(7)
	if spec.Title != "Steel club (good)" {
		t.Fatalf("title %q", spec.Title)
	}
	got := strings.Join(spec.Lines, "|")
	if !strings.Contains(got, "Hit points 20 / 100 (20%)") || !strings.Contains(got, "Worn by a dead pawn") {
		t.Fatal(got)
	}
	f.world.Items[0].MaxHP, f.world.Items[0].Tainted = 0, false
	if strings.Contains(strings.Join(New(f).Thing().ItemPane(7).Lines, "|"), "Hit points") {
		t.Fatal("no condition without MaxHP")
	}
}

// intentShell records the intents the pane sends.
type intentShell struct {
	*fakeShell
	intents []ports.Intent
}

func (s *intentShell) Do(it ports.Intent) { s.intents = append(s.intents, it) }

// GEAR-006: the item pane's Info button opens the Info card on the stack.
func TestItemInfoButtonOpensTheCard(t *testing.T) {
	t.Parallel()
	f := &intentShell{fakeShell: newFake()}
	f.world = game.WorldView{Items: []game.ItemView{{ID: 7, Kind: "club", Qty: 1, FreshDays: -1}}}
	var info *Button
	cmds := New(f).Thing().ItemCommands(7)
	for i := range cmds {
		if cmds[i].ID == ItemInfoID {
			info = &cmds[i]
		}
	}
	if info == nil || info.Disabled || info.Run == nil {
		t.Fatalf("no Info button in %+v", cmds)
	}
	info.Run()
	if len(f.intents) != 1 || f.intents[0] != (ports.OpenInfoCard{ID: 7, Kind: "club"}) {
		t.Fatalf("intents %+v", f.intents)
	}
}

func TestInspirationInStatus(t *testing.T) {
	t.Parallel()
	if InspirationLine("") != "" || InspirationLine("inspired creativity") != "Inspired: creativity" {
		t.Fatal("line")
	}
	f := newFake()
	f.pawns = []game.PawnView{{ID: entity.EntityID(1), Status: "alive", Inspiration: "inspired creativity"}}
	lines := New(f).Pawn().PawnLines(f.pawns[0])
	if !strings.Contains(strings.Join(lines, "|"), "Inspired: creativity") {
		t.Fatal(lines)
	}
}

func TestDoorForbidToggle(t *testing.T) {
	t.Parallel()
	d := game.BuildingView{ID: 5, Kind: "door"}
	if !forbiddable(d) {
		t.Fatal("a door is forbiddable")
	}
	if c := forbidCommand(lang.English{}, d); c.Label != "Forbid" || c.On || !c.Cmds[0].Enabled {
		t.Fatalf("open door %+v", c)
	}
	d.Forbidden = true
	if c := forbidCommand(lang.English{}, d); c.Label != "Allow" || !c.On || c.Cmds[0].Enabled {
		t.Fatalf("forbidden door %+v", c)
	}
}
