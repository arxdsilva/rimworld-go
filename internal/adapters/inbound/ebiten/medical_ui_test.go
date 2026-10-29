package ebiten

import (
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-020 UI: the bed pane's Medical toggle and line, the owner buttons
// that give way to it, and the colonist's "Rest when hurt" toggle; the
// pane says "Resting (injured)" once, through the job line.

const medicalBedDesc = "Only the sick and injured rest here. Healthy colonists sleep elsewhere."

// medicalStub is bed 40 of kind (medical or not) over the plain colony.
func medicalStub(kind string, medical bool) *stubSim {
	return bldStub(game.BuildingView{ID: 40, Kind: kind, X: 5, Y: 5, W: 1, H: 2, Medical: medical})
}

func TestBedMedicalToggleOnEverySleepingBed(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"bed", "double bed", "hospital bed", "sleeping spot"} {
		s := medicalStub(kind, false)
		a, _ := bldApp(t, s, 40)
		c, ok := bldCmd(a.buildingCommands(40), "bldg.bed.medical")
		if !ok {
			t.Fatalf("%s: no Medical command in %v", kind, bldCmdIDs(a.buildingCommands(40)))
		}
		if c.Label != "Medical" || !c.Toggle || c.On || c.Disabled || c.Icon != "hospital_bed" || c.Desc != medicalBedDesc {
			t.Errorf("%s: Medical command %+v", kind, c)
		}
		if c.Run != nil || len(c.Cmds) != 0 {
			t.Errorf("%s: bldg.bed.medical is a legacy id: it keeps its handler, so no Run/Cmds", kind)
		}
	}
	// a building that is not a bed has no Medical toggle
	s := bldStub(game.BuildingView{ID: 40, Kind: "wall", X: 5, Y: 5, W: 1, H: 1})
	a, _ := bldApp(t, s, 40)
	if _, ok := bldCmd(a.buildingCommands(40), "bldg.bed.medical"); ok {
		t.Fatal("a wall has no Medical toggle")
	}
}

func TestBedMedicalToggleFollowsAndFlipsTheBed(t *testing.T) {
	t.Parallel()
	s := medicalStub("bed", false)
	a, _ := bldApp(t, s, 40)
	c, _ := bldCmd(a.buildingCommands(40), "bldg.bed.medical")
	pressCmd(a, c)
	got := cmdsOf(s, command.SetBedMedical)
	if len(got) != 1 || got[0].Target != 40 || !got[0].Enabled {
		t.Fatalf("turning Medical on enqueued %+v", got)
	}

	s.world.Buildings[0].Medical = true
	c, _ = bldCmd(a.buildingCommands(40), "bldg.bed.medical")
	if !c.On {
		t.Fatalf("a medical bed shows the toggle on: %+v", c)
	}
	pressCmd(a, c)
	got = cmdsOf(s, command.SetBedMedical)
	if len(got) != 2 || got[1].Target != 40 || got[1].Enabled {
		t.Fatalf("turning Medical off enqueued %+v", got)
	}
}

// The toggle needs no read model beyond the bed itself: it works on a game
// that has no prisoner mark too.
func TestBedMedicalToggleIsIndependentOfThePrisonerMark(t *testing.T) {
	t.Parallel()
	s := bedStub()
	s.world.Buildings[0].Medical = true
	a, _ := bldApp(t, s.stubSim, 40)
	a.renderer.game = s
	cs := a.buildingCommands(40)
	med, _ := bldCmd(cs, "bldg.bed.medical")
	pr, _ := bldCmd(cs, "bldg.bed.prisoners")
	if !med.On || med.Disabled || pr.On {
		t.Fatalf("medical %+v, prisoners %+v", med, pr)
	}
	// Medical sits beside Prisoners, after it
	ids := bldCmdIDs(cs)
	if slices.Index(ids, "bldg.bed.medical") != slices.Index(ids, "bldg.bed.prisoners")+1 {
		t.Fatalf("Medical should follow Prisoners: %v", ids)
	}
}

func TestMedicalBedLine(t *testing.T) {
	t.Parallel()
	a, _ := bldApp(t, medicalStub("hospital bed", true), 40)
	if !strings.Contains(bldLines(a.buildingPane(40)), "Medical bed") {
		t.Fatalf("a medical bed says so: %q", a.buildingPane(40).Lines)
	}
	a, _ = bldApp(t, medicalStub("hospital bed", false), 40)
	if strings.Contains(bldLines(a.buildingPane(40)), "Medical bed") {
		t.Fatalf("a plain bed is not called medical: %q", a.buildingPane(40).Lines)
	}
}

func TestMedicalBedOwnerButtonsAreDisabledWithTheReason(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"bed", "double bed"} {
		s := medicalStub(kind, true)
		a, _ := bldApp(t, s, 40)
		slots := map[string]int{"bed": 1, "double bed": 2}[kind]
		cs := a.buildingCommands(40)
		for slot := range slots {
			c, ok := bldCmd(cs, bedOwnerID(slot))
			if !ok {
				t.Fatalf("%s: no owner button %d in %v", kind, slot, bldCmdIDs(cs))
			}
			if !c.Disabled || c.Reason != "Medical beds have no owner" {
				t.Errorf("%s owner %d on a medical bed: %+v", kind, slot, c)
			}
			// pressing it only says why: no picker opens, nothing is sent
			pressCmd(a, c)
			if a.hint != "Medical beds have no owner" || len(s.cmds) != 0 {
				t.Errorf("%s owner %d: hint %q, commands %v", kind, slot, a.hint, s.cmds)
			}
		}
		// the same bed, no longer medical, has working owner buttons again
		s.world.Buildings[0].Medical = false
		for slot := range slots {
			if c, _ := bldCmd(a.buildingCommands(40), bedOwnerID(slot)); c.Disabled || c.Reason != "" {
				t.Errorf("%s owner %d on a plain bed: %+v", kind, slot, c)
			}
		}
	}
}

// ---- the colonist ------------------------------------------------------------

const restWhenHurtDesc = "Lie in bed to heal when in pain, wounded or sick."

func TestPawnRestWhenHurtToggleFollowsAndFlipsTheSetting(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].RestWhenHurt = true
	a, _ := paneApp(t, s)
	c := mustCmd(t, a.pawnCommands(1), "cmd.pawn.bedrest")
	if c.Label != "Rest when hurt" || !c.Toggle || !c.On || c.Disabled || c.Icon != "bed_rest" || c.Desc != restWhenHurtDesc {
		t.Fatalf("Rest when hurt command %+v", c)
	}
	if c.Run == nil {
		t.Fatal("a cmd. id carries its Run")
	}
	pressCmd(a, c)
	got := cmdsOf(s, command.SetPawnBedRest)
	if len(got) != 1 || got[0].Target != 1 || got[0].Enabled {
		t.Fatalf("turning it off enqueued %+v", got)
	}

	s.pawns[0].RestWhenHurt = false
	c = mustCmd(t, a.pawnCommands(1), "cmd.pawn.bedrest")
	if c.On {
		t.Fatalf("the toggle reads the setting back: %+v", c)
	}
	pressCmd(a, c)
	got = cmdsOf(s, command.SetPawnBedRest)
	if len(got) != 2 || got[1].Target != 1 || !got[1].Enabled {
		t.Fatalf("turning it on enqueued %+v", got)
	}
}

// Run reads the setting when it is clicked, not when the button was built.
func TestPawnRestWhenHurtClickReadsTheLiveSetting(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].RestWhenHurt = true
	a, _ := paneApp(t, s)
	c := mustCmd(t, a.pawnCommands(1), "cmd.pawn.bedrest")
	s.pawns[0].RestWhenHurt = false // changed since the grid was built
	pressCmd(a, c)
	if got := cmdsOf(s, command.SetPawnBedRest); len(got) != 1 || !got[0].Enabled {
		t.Fatalf("click enqueued %+v, want Enabled", got)
	}
}

func TestPawnRestWhenHurtIsForColonistsOnly(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, _ := paneApp(t, s)
	if _, ok := cmdByID(a.pawnCommands(1), "cmd.pawn.bedrest"); !ok {
		t.Fatal("a free colonist has Rest when hurt")
	}
	if _, ok := cmdByID(a.pawnCommands(2), "cmd.pawn.bedrest"); ok {
		t.Fatal("a prisoner has no Rest when hurt")
	}
	for _, c := range []struct {
		name string
		edit func(p *game.PawnView)
	}{
		{"a dead colonist", func(p *game.PawnView) { p.Status = "dead" }},
		{"a raider", func(p *game.PawnView) { p.Hostile = true }},
		{"a corpse", func(p *game.PawnView) { p.Embodied = true }},
	} {
		s := paneSim()
		c.edit(&s.pawns[0])
		a, _ := paneApp(t, s)
		if _, ok := cmdByID(a.pawnCommands(1), "cmd.pawn.bedrest"); ok {
			t.Errorf("%s has no Rest when hurt", c.name)
		}
	}
}

// A group of colonists merges the toggle: Mixed while they differ, and a
// click moves the members that match the shown state, as Draft's does.
func TestPawnRestWhenHurtMergesInAGroup(t *testing.T) {
	t.Parallel()
	s := &stubSim{w: 24, h: 24, pawns: []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive", RestWhenHurt: true},
		{ID: 2, Name: "Bo", Status: "alive", RestWhenHurt: true},
	}}
	a, _ := colonyApp(t, s)
	a.setGroup(selPawn, []entity.EntityID{1, 2})
	const id = "cmd.group.pawn.bedrest"
	c, ok := cmdByID(a.groupCommands(), id)
	if !ok || !c.On || c.Mixed {
		t.Fatalf("both rest when hurt: %+v ok=%v", c, ok)
	}
	pressCmd(a, c)
	got := cmdsOf(s, command.SetPawnBedRest)
	if len(got) != 2 || got[0].Enabled || got[1].Enabled || got[0].Target+got[1].Target != 3 {
		t.Fatalf("a group click turns both off: %+v", got)
	}
	s.pawns[1].RestWhenHurt = false
	a.updates++ // a new Update: the live grid asks again
	if c, _ = cmdByID(a.groupCommands(), id); !c.Mixed || c.On {
		t.Fatalf("one of two rests when hurt: %+v", c)
	}
}

func TestPawnRestWhenHurtSitsAfterEquipBeforeThePrisonerCommands(t *testing.T) {
	t.Parallel()
	a, _ := paneApp(t, paneSim())
	var ids []string
	for _, c := range a.pawnCommands(1) {
		ids = append(ids, c.ID)
	}
	want := []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", "cmd.pawn.bedrest", prisonerCapture, prisonerArrest}
	if !slices.Equal(ids, want) {
		t.Fatalf("commands %v, want %v", ids, want)
	}
}

// TestPawnPaneSaysRestingInjuredOnce: a colonist on bed rest reads
// "Resting (injured)" in the pane once, as its job (the sim names the job
// so), not a second time as a status line.
func TestPawnPaneSaysRestingInjuredOnce(t *testing.T) {
	s := paneSim()
	s.pawns[0].BedRest, s.pawns[0].Job = true, "Resting (injured)"
	a, _ := paneApp(t, s)
	n := 0
	for _, ln := range a.pawnLines(s.pawns[0]) {
		if strings.Contains(ln, "Resting (injured)") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("pane lines %q: %d mention resting, want 1 (the job line)", a.pawnLines(s.pawns[0]), n)
	}
}

// ---- layout ------------------------------------------------------------------

// At UI scales 1, 1.5 and 2 the bed's and the colonist's commands (the new
// buttons included) each get a grid cell right of the pane, inside the grid
// band and overlapping no other; every label word fits its cell; and the
// pane and the grid draw. The worst case is a medical double bed with a
// move pending, and a colonist with every pawn command.
func TestMedicalCommandsFitTheGridAtUIScales(t *testing.T) {
	for _, scale := range []float64{1, 1.5, 2} {
		withUIScale(t, scale)
		bed := bldStub(game.BuildingView{ID: 40, Kind: "double bed", X: 5, Y: 5, W: 2, H: 2, Medical: true,
			MovingTo: &[2]int{8, 8}})
		ba, _ := bldApp(t, bed, 40)
		pa, _ := paneApp(t, paneSim())
		for _, c := range []struct {
			name string
			app  *App
			cmds []cmdButton
		}{
			{"bed", ba, ba.buildingCommands(40)},
			{"pawn", pa, pa.pawnCommands(1)},
		} {
			a, cs := c.app, c.cmds
			if len(cs) == 0 {
				t.Fatalf("scale %v %s: no commands", scale, c.name)
			}
			sw, sh := a.size()
			l := computeHUDLayout(sw, sh, hudState{PaneShown: true, Commands: len(cs), GlobalH: 200})
			if len(l.GridCells) != len(cs) {
				t.Fatalf("scale %v %s: %d commands, %d cells", scale, c.name, len(cs), len(l.GridCells))
			}
			for i, cm := range cs {
				cell := l.GridCells[i]
				if cell.X < l.Pane.Right() || cell.Right() > l.Global.X || cell.Bottom() > l.Tabs.Y {
					t.Errorf("scale %v %s: %s cell %+v leaves the grid band", scale, c.name, cm.ID, cell)
				}
				// the existing Deconstruct label is wider than its cell
				// (cmdLabelLines shrinks it); only the new labels are held to it
				for _, w := range strings.Fields(cm.Label) {
					if cm.ID == "bldg.deconstruct" {
						break
					}
					if tw := textW(w, tsSmall); tw > cell.W-8 {
						t.Errorf("scale %v %s: %s label word %q (%.0fpx) does not fit %.0fpx", scale, c.name, cm.ID, w, tw, cell.W)
					}
				}
				for j, o := range l.GridCells[:i] {
					if !cell.Intersect(o).Empty() {
						t.Errorf("scale %v %s: %s overlaps command %d", scale, c.name, cm.ID, j)
					}
				}
			}
			// the live layout agrees, and drawing the pane and grid does not panic
			if got := len(a.insp.Commands().CommandCells()); got != len(cs) {
				t.Errorf("scale %v %s: the live grid shows %d of %d commands", scale, c.name, got, len(cs))
			}
			img := ebiten.NewImage(sw, sh)
			a.drawPane(img)
			a.drawCommandGrid(img)
		}
	}
}
