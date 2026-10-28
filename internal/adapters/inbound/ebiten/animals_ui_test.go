package ebiten

import (
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestAnimalsTabListsTameAnimals(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, err := g.SpawnAnimal("dustbuck", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, "Clover")
	click(t, a, "tab.animals")
	if !a.workOpen || a.panelMode != "animals" {
		t.Fatal("animals tab opens the animals panel")
	}
	sid := strconv.FormatUint(uint64(id), 10)
	click(t, a, "animals.slaughter."+sid)
	click(t, a, "animals.release."+sid)
	g.Step(1)
	if an := g.World().Animals[0]; an.Tame || !an.Designated {
		t.Fatalf("slaughter marks it for hunting, release lets it go: %+v", an)
	}
}

func penTabSim() *stubSim {
	s := penSim()
	s.world.Animals[2].AreaRef = 7 // the pet is restricted to area 7
	s.world.Animals[0].Trained = []bool{true}
	s.world.Animals[0].Master = "Ada"
	s.world.Animals[0].MasterID = 1
	s.world.Animals[0].CanFollow, s.world.Animals[0].Follows = true, true
	s.areas = []game.AreaView{{Index: 0, ID: 7, Name: "Orchard"}, {Index: 1, ID: 9, Name: "Yard"}}
	return s
}

func TestAnimalsTabAreaAndPenColumns(t *testing.T) {
	t.Parallel()
	s := penTabSim()
	a, f := colonyApp(t, s)
	click(t, a, "tab.animals")
	f.Frame(t, a)
	hits := map[string]bool{}
	for _, b := range a.layoutHits(a.sw, a.sh) {
		hits[b.ID] = true
	}
	if !hits["animals.area.52"] {
		t.Fatal("a non-penned animal has an area dropdown")
	}
	for _, id := range []string{"animals.area.50", "animals.area.51"} {
		if hits[id] {
			t.Fatalf("penned animal has no area dropdown: %s", id)
		}
	}
	if got := animalAreaCell(s.world.Animals[0], nil); got != "Pen 1" {
		t.Fatalf("penned cell %q", got)
	}
	if got := animalAreaCell(s.world.Animals[1], nil); got != "Roaming" {
		t.Fatalf("roaming cell %q", got)
	}
	roped := game.AnimalView{Penned: true, Pen: "Being roped"}
	if animalAreaCell(roped, nil) != "Being roped" {
		t.Fatal("roped cell")
	}
	if got := animalAreaCell(game.AnimalView{}, nil); got != "Unrestricted" {
		t.Fatalf("default area %q", got)
	}
	if got := animalAreaCell(game.AnimalView{AreaRef: command.AreaHome}, nil); got != "Home" {
		t.Fatalf("home area %q", got)
	}
	if got := followCell(s.world.Animals[0]); got != "on" {
		t.Fatalf("trained animal follows: %q", got)
	}
	if got := followCell(game.AnimalView{CanFollow: true}); got != "off" {
		t.Fatalf("following turned off: %q", got)
	}
	if got := followCell(s.world.Animals[1]); got != "-" {
		t.Fatalf("untrained: %q", got)
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawAnimalsTab(screen, 1280, 720)
}

func TestAnimalsTabAreaDropdownAssigns(t *testing.T) {
	t.Parallel()
	s := penTabSim()
	a, f := colonyApp(t, s)
	click(t, a, "tab.animals")
	f.Frame(t, a)
	click(t, a, "animals.area.52")
	f.Frame(t, a) // the picker opens
	f.Frame(t, a) // and declares its rows
	click(t, a, "areapick.row.9")
	f.Frame(t, a)
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.AssignArea || c.Pawn != 52 || c.Index != 9 {
		t.Fatalf("pick Yard for the labrador: %+v", s.cmds)
	}
}

func TestAnimalsTabMasterDropdownEmitsCommand(t *testing.T) {
	t.Parallel()
	s := penTabSim()
	a, _ := colonyApp(t, s)
	click(t, a, "tab.animals")
	click(t, a, "animals.master.50")
	if !a.float.open || len(a.float.opts) != 3 {
		t.Fatalf("master dropdown lists the colonists and none: %+v", a.float.opts)
	}
	click(t, a, "float.1")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.SetAnimalMaster || c.Target != 50 || c.Pawn != 2 {
		t.Fatalf("pick Bo: %+v", c)
	}
}

// PLAY-015: the Follow toggle sends SetAnimalFollow with the flipped value.
func TestAnimalsTabFollowToggle(t *testing.T) {
	t.Parallel()
	s := penTabSim()
	a, f := colonyApp(t, s)
	click(t, a, "tab.animals")
	f.Frame(t, a)
	id := "animals.follow." + strconv.FormatUint(uint64(s.world.Animals[0].ID), 10)
	found := false
	for _, b := range a.layoutHits(a.sw, a.sh) {
		found = found || b.ID == id
	}
	if !found {
		t.Fatal("trained animal has no Follow toggle")
	}
	a.applyAnimalsTab(id)
	var got []command.Command
	for _, c := range s.cmds {
		if c.Kind == command.SetAnimalFollow {
			got = append(got, c)
		}
	}
	if len(got) != 1 || got[0].Target != uint64(s.world.Animals[0].ID) || got[0].Enabled {
		t.Fatalf("follow command %+v", got)
	}
}
