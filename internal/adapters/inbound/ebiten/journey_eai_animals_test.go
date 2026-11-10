package ebiten

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// journey_eai_animals_test.go: the animal threats of the enemy AI wave as
// the player meets them (EAI-007): a manhunter pack battering the door of
// a hut a colonist holds, and a predator charging a colonist.

// TestJourneyEAIManhunterPack (EAI-001, EAI-005): a manhunter pack comes
// in while Ada holds a walled hut with a shut door. The red "Manhunter
// pack" letter is in the stack and opens, the danger alert counts the
// animals, and a click on the one battering the door shows "Breaking
// through the door" in its pane until the door is damaged.
func TestJourneyEAIManhunterPack(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 3, Width: 30, Height: 22})
	door := eaiHut(t, g, 10, 5, 16, 11, 13, 11)
	eaiHolder(t, g, 13, 8, "Ada")
	g.Step(1) // the draft lands
	j := newJourney(t, jGame(g))
	g.ManhunterPack(300)
	j.update(2)

	var pack game.LetterView
	for _, r := range j.a.letterRows(j.a.hud()) {
		if r.Letter.Title == "Manhunter pack" {
			pack = r.Letter
		}
	}
	if pack.ID == 0 || pack.Kind != "threat" || !strings.Contains(pack.Text, "is charging the colony") {
		t.Fatalf("no red Manhunter pack letter in the stack: %+v", j.world().Letters)
	}
	if !j.clickID(fmt.Sprintf("letter.%d", pack.ID)) || j.a.uiModalTop() != fmt.Sprintf("letter.%d", pack.ID) {
		t.Fatalf("the Manhunter pack letter does not open; top modal %q", j.a.uiModalTop())
	}
	j.key("Escape", 0)
	if alert, ok := eaiAlert(j, "hostile"); !ok || !strings.Contains(alert.Label, "attacking") {
		t.Fatalf("no danger alert for the pack: %+v", j.a.chrome.Alerts().Current())
	}

	// an animal comes up to the hut and batters its door
	var basher game.AnimalView
	for i := 0; i < 300 && basher.ID == 0; i++ {
		g.Step(10)
		for _, an := range j.world().Animals {
			if an.Bashing && an.Manhunter && eaiBeside(an.X, an.Y, 13, 11) {
				basher = an
			}
		}
	}
	if basher.ID == 0 {
		t.Fatal("no manhunter ever stood battering the shut door")
	}
	j.update(1)
	j.lookAt(basher.X, basher.Y)
	j.update(1)
	j.clickTile(basher.X, basher.Y, inputframe.Left, 0)
	if !j.a.sel.HasAnimal || j.a.sel.SelAnimal != basher.ID || !j.a.insp.PaneShown() {
		t.Fatalf("a click on the battering %s selected animal %v (has %v)", basher.Species, j.a.sel.SelAnimal, j.a.sel.HasAnimal)
	}
	lines := j.a.insp.Animal().AnimalPane(j.a.sel.SelAnimal).Lines
	if !slices.Contains(lines, "Breaking through the door") || !slices.Contains(lines, "MANHUNTER: hunting colonists!") {
		t.Fatalf("the basher's pane: %q", lines)
	}
	for i := 0; i < 200 && eaiDoorHP(j, door) >= 100; i++ {
		g.Step(20)
	}
	if hp := eaiDoorHP(j, door); hp >= 100 {
		t.Fatalf("the door is untouched: %d%%", hp)
	}
}

// TestJourneyEAIPredatorCharge (PLAY-010, EAI-004): a cougar charges Ada
// in the open. The danger message names her ("A cougar charges Ada!"),
// the alert says it is after her, and a click on the cougar shows it
// attacking a colonist.
func TestJourneyEAIPredatorCharge(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 13, Width: 32, Height: 20})
	cougar, err := g.SpawnAnimal("cougar", 15, 10)
	if err != nil {
		t.Fatal(err)
	}
	eaiHolder(t, g, 17, 10, "Ada")
	g.Step(1)
	j := newJourney(t, jGame(g))

	var charge game.MessageView
	for i := 0; i < 400 && charge.Tick == 0; i++ {
		g.Step(10)
		for _, m := range j.world().Messages {
			if m.Danger && m.Focus == cougar && charge.Tick == 0 {
				charge = m
			}
		}
	}
	if charge.Text != "A cougar charges Ada!" {
		t.Fatalf("no charge warning: %+v", charge)
	}
	j.update(2)
	shown := false
	for _, r := range j.a.chrome.Right().MessageRows(j.a.hud()) {
		shown = shown || r.Msg.Text == charge.Text
	}
	if !shown {
		t.Fatalf("the charge is not on screen: %+v", j.a.chrome.Right().MessageRows(j.a.hud()))
	}
	if alert, ok := eaiAlert(j, "hostile"); !ok || alert.Label != "A cougar is chasing Ada!" {
		t.Fatalf("the alert does not name the chase: %+v", j.a.chrome.Alerts().Current())
	}
	var at game.AnimalView
	for _, an := range j.world().Animals {
		if an.ID == cougar {
			at = an
		}
	}
	j.lookAt(at.X, at.Y)
	j.update(1)
	j.clickTile(at.X, at.Y, inputframe.Left, 0)
	if !j.a.sel.HasAnimal || j.a.sel.SelAnimal != cougar {
		t.Fatalf("a click on the cougar selected animal %v (has %v)", j.a.sel.SelAnimal, j.a.sel.HasAnimal)
	}
	if lines := j.a.insp.Animal().AnimalPane(cougar).Lines; !slices.Contains(lines, "attacking a colonist!") {
		t.Fatalf("the cougar's pane: %q", lines)
	}
}

// eaiHut walls in x0..x1, y0..y1 of wood with one shut door at (dx, dy)
// and returns the door.
func eaiHut(t *testing.T, g *game.Game, x0, y0, x1, y1, dx, dy int) entity.EntityID {
	t.Helper()
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "wall"
			if x == dx && y == dy {
				kind = "door"
			}
			if !g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("%s at %d,%d", kind, x, y)
			}
		}
	}
	for _, b := range g.World().Buildings {
		if b.X == dx && b.Y == dy && b.Kind == "door" {
			return b.ID
		}
	}
	t.Fatal("no door built")
	return 0
}

// eaiHolder is a fed, rested, drafted colonist who holds (x, y).
func eaiHolder(t *testing.T, g *game.Game, x, y int, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	return id
}

// eaiAlert is the HUD alert with key, if it is up.
func eaiAlert(j *journey, key string) (a hud.Alert, ok bool) {
	for _, al := range j.a.chrome.Alerts().Current() {
		if al.Key == key {
			return al, true
		}
	}
	return a, false
}

// eaiDoorHP is door's health in percent (0 once it is gone).
func eaiDoorHP(j *journey, door entity.EntityID) int {
	for _, b := range j.world().Buildings {
		if b.ID == door {
			return b.HPPct
		}
	}
	return 0
}

// eaiBeside: (x, y) is (tx, ty) or one of its eight neighbours.
func eaiBeside(x, y, tx, ty int) bool {
	return x >= tx-1 && x <= tx+1 && y >= ty-1 && y <= ty+1
}
