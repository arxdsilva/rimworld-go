package ebiten

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// bondSim is a stub colony with two colonists and a tame dog, master Ada,
// bonded with Bo, its Obedience half learnt and Guard allowed.
func bondSim() *stubSim {
	dog := game.AnimalView{ID: 50, Species: "labrador", Label: "labrador", X: 6, Y: 6, Tame: true, Name: "Rook",
		Mode: "wandering", State: "wandering", Health: 100, Food: 80, Rest: 80,
		Master: "Ada", MasterID: 1, Bond: "Bo", BondID: 2,
		Trained: []bool{false, false, false},
		Training: []game.AnimalTrainingView{
			{Skill: 0, Label: "Obedience", Allowed: true, On: true, Steps: 2, Needed: 3},
			{Skill: 1, Label: "Guard", Allowed: true, Steps: 0, Needed: 3},
			{Skill: 2, Label: "Release", Needed: 2},
		},
		Mother: "Clover", Offspring: []string{"Ash", "Moss"}}
	return &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 2, Y: 2}, {ID: 2, Name: "Bo", X: 3, Y: 2}},
		world: game.WorldView{Animals: []game.AnimalView{dog}}}
}

func TestAnimalInspectorBondBlock(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	if !a.selectAnimalAt(6, 6) {
		t.Fatal("select the dog")
	}
	an, _ := a.selectedAnimal()
	rows := strings.Join(animalBondRows(an), "|")
	for _, want := range []string{"Master: Ada", "Bond: Bo", "Mother: Clover", "Young: Ash, Moss"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("bond rows %q lack %q", rows, want)
		}
	}
	// the pane shows those rows, and the commands carry the training skills
	if lines := thingLines(a.animalPane(an.ID)); !strings.Contains(lines, rows) {
		t.Fatalf("pane lines %q lack the bond rows %q", lines, rows)
	}
	cs := a.animalCommands(an.ID)
	ids := map[string]cmdButton{}
	for _, c := range cs {
		ids[c.ID] = c
	}
	if _, ok := ids["animal.train.2"]; ok {
		t.Fatal("a skill the species can't learn has no checkbox")
	}
	if !ids["animal.train.0"].On || ids["animal.train.1"].On || ids["animal.train.0"].Label != "Obedience 2/3" {
		t.Fatalf("checkboxes %+v %+v", ids["animal.train.0"], ids["animal.train.1"])
	}
	// every command has a cell in the grid beside the pane (the old panel
	// check that no button fell out of it)
	l := computeHUDLayout(1280, 720, hudState{PaneShown: true, Commands: len(cs)})
	if len(l.GridCells) != len(cs) {
		t.Fatalf("%d commands, %d grid cells", len(cs), len(l.GridCells))
	}
	click(t, a, "animal.train.1")
	click(t, a, "animal.train.0")
	if len(s.cmds) != 2 {
		t.Fatalf("cmds %+v", s.cmds)
	}
	if c := s.cmds[0]; c.Kind != command.SetTraining || c.Target != 50 || c.Index != 1 || !c.Enabled {
		t.Fatalf("guard on: %+v", c)
	}
	if c := s.cmds[1]; c.Kind != command.SetTraining || c.Index != 0 || c.Enabled {
		t.Fatalf("obedience off: %+v", c)
	}
	click(t, a, "cmd.animal.master")
	if !a.float.open || len(a.float.opts) != 3 {
		t.Fatalf("the master picker lists both colonists and none: %+v", a.float.opts)
	}
	if a.float.opts[0].Label != "Ada (master)" || a.float.opts[2].Cmd.Pawn != 0 {
		t.Fatalf("picker %+v", a.float.opts)
	}
	click(t, a, "float.1")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.SetAnimalMaster || c.Target != 50 || c.Pawn != 2 {
		t.Fatalf("pick Bo: %+v", c)
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawAnimalMark(screen) // no art: must not panic
}

func TestWildAnimalHasNoMasterRow(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{ID: 9, Species: "deer", Mode: "wandering", State: "wandering", Mother: "a deer"}
	rows := animalBondRows(an)
	if len(rows) != 1 || rows[0] != "Mother: a deer" {
		t.Fatalf("wild rows %v", rows)
	}
	st := &stubSim{world: game.WorldView{Animals: []game.AnimalView{an}}}
	a, _ := colonyApp(t, st)
	a.selectCand(selCand{selAnimal, 9})
	for _, c := range a.animalCommands(9) {
		if strings.Contains(c.ID, "master") || strings.Contains(c.ID, "train") {
			t.Fatalf("no master or training command for a wild animal: %s", c.ID)
		}
	}
}

func TestAnimalsTabMasterAndTrainingColumns(t *testing.T) {
	t.Parallel()
	an := bondSim().world.Animals[0]
	if animalMasterCell(an) != "Ada" || animalMasterCell(game.AnimalView{}) != "-" {
		t.Fatal("master cell")
	}
	if got := animalTrainingCell(an); got != "Obedience 2/3" {
		t.Fatalf("training cell %q", got)
	}
	s := bondSim()
	a, _ := colonyApp(t, s)
	click(t, a, "tab.animals")
	// the Colony list fits the window at 1280 px: columns give way, and
	// every button stays inside the Window rect
	if w := a.hud().Window; a.sw != 1280 || w.Empty() {
		t.Fatalf("no window at %d px: %+v", a.sw, w)
	} else {
		for _, b := range a.animalsTabButtons() {
			if b.x < w.X-0.5 || b.x+b.w > w.Right()+0.5 || b.y < w.Y-0.5 || b.y+b.h > w.Bottom()+0.5 {
				t.Errorf("%s %v,%v %vx%v runs out of the window %+v", b.id, b.x, b.y, b.w, b.h, w)
			}
		}
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawAnimalsTab(screen, 1280, 720)
}

// The pane's tab bodies (the log and the training rows) draw only inside
// the inner rectangle the pane gives them (the expanded panel less its
// inset), at both UI scales, and every training label reads in full in its
// command cell. (Not parallel: the draw trace is global.)
func TestAnimalTabBodiesStayInTheirPanel(t *testing.T) {
	s := bondSim()
	s.world.Animals[0].Log = thingLog(40)
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	sp := a.animalPane(50)
	for _, scale := range []float64{1, 1.5} {
		withUIScale(t, scale)
		for _, id := range []string{"inspector.log", "inspector.training"} {
			tb, ok := thingTab(sp, id)
			if !ok {
				t.Fatalf("no tab %s", id)
			}
			if tb.WantW < expandMinW || tb.WantW > expandMaxW {
				t.Errorf("%s wants %v px wide, the panel clamps to %d..%d", id, tb.WantW, expandMinW, expandMaxW)
			}
			inner := rect{X: 40, Y: 260, W: tb.WantW - 2*paneInset, H: tb.WantH - 2*paneInset}
			img := ebiten.NewImage(1280, 720)
			events := thingTraceDraw(t, func() { tb.Draw(img, inner) })
			if len(events) == 0 {
				t.Errorf("%s drew nothing", id)
			}
			for _, e := range events {
				x0, y0 := inner.X*float32(scale), inner.Y*float32(scale)
				x1, y1 := inner.Right()*float32(scale), inner.Bottom()*float32(scale)
				// the log scrolls: a row cut by the bottom edge is clipped by the
				// pane, so it only has to start inside; everything else must fit
				bottom := y1 + 0.5
				if id == "inspector.log" {
					bottom = y1 - 1
					if e.Y+e.H < y0 {
						t.Errorf("scale %v: %q is above the log view", scale, e.Text)
					}
				}
				edge := e.Y + e.H
				if id == "inspector.log" {
					edge = e.Y
				}
				if e.X < x0-0.5 || e.Y < y0-0.5 || e.X+e.W > x1+0.5 || edge > bottom {
					t.Errorf("scale %v, %s: %s %q (%.0f,%.0f %.0fx%.0f) leaves the body (%.0f,%.0f)-(%.0f,%.0f)",
						scale, id, e.Kind, e.Text, e.X, e.Y, e.W, e.H, x0, y0, x1, y1)
				}
			}
		}
	}
	for _, c := range a.animalCommands(50) {
		if !strings.HasPrefix(c.ID, "animal.train.") {
			continue
		}
		for _, word := range strings.Fields(c.Label) {
			if w := textW(word, tsSmall); w > cmdCell-8 {
				t.Errorf("%s: %q (%.0f px) is clipped in a %d px cell", c.ID, word, w, cmdCell)
			}
		}
	}
}

// The Log tab scrolls with the wheel over its panel (and the camera does not
// zoom), starts at the top for another animal, and a short log has nothing
// to scroll.
func TestAnimalLogTabScrollsWithWheel(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.world.Animals[0].Log = thingLog(80)
	a, f := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	f.frame(t, a)
	a.selectPaneTab("inspector.log")
	f.frame(t, a)
	exp := a.expandedRect()
	if exp.Empty() {
		t.Fatal("the Log tab opens an expanded panel")
	}
	cx, cy := exp.Center()
	zoom := a.renderer.cam.zoom
	f.cur.mx, f.cur.my, f.cur.wheelY = int(cx), int(cy), -1
	f.frame(t, a)
	if off := a.uiSt().scroll[animalLogID]; off <= 0 {
		t.Fatalf("the wheel scrolled the log by %v", off)
	}
	if a.renderer.cam.zoom != zoom {
		t.Fatal("the wheel over the panel must not zoom the map")
	}
	// another animal starts at the top
	s.world.Animals = append(s.world.Animals, game.AnimalView{ID: 51, Species: "labrador", Label: "labrador", X: 7, Y: 7, Tame: true, Name: "Ash", Log: thingLog(80)})
	a.selectCand(selCand{selAnimal, 51})
	f.frame(t, a)
	if off := a.uiSt().scroll[animalLogID]; off != 0 {
		t.Fatalf("a new animal's log scrolls from the top, got %v", off)
	}
}
