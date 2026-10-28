package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
)

// MED-03: the Assign window's medical column and default care row, and
// the drafted "Tend <name>" order.

// careSim is the Assign stub with the default care read model.
type careSim struct {
	*policySim
	def [medcare.NumGroups]uint8
}

func (s *careSim) DefaultCare() [medcare.NumGroups]uint8 { return s.def }

func TestAssignMedicalCareColumn(t *testing.T) {
	t.Parallel()
	ps := assignStub()
	ps.pawns[0].CareLevel = uint8(medcare.Best)
	ps.pawns[1].CareLevel = uint8(medcare.HerbalMax)
	s := &careSim{policySim: ps, def: [medcare.NumGroups]uint8{uint8(medcare.Best), uint8(medcare.HerbalMax), uint8(medcare.HerbalMax), uint8(medcare.HerbalMax)}}
	a, f := colonyApp(t, ps.stubSim)
	a.renderer.game = s
	a.openAssign()
	f.Frame(t, a)
	if w, ok := a.uiSt().Find(assignCareID(1)); !ok || w.Label != medcare.Best.Label() {
		t.Fatalf("Ada's care button %+v", w)
	}
	if hasWidget(a, assignCareID(3)) {
		t.Fatal("the dead get no care button")
	}
	press(t, a, f, assignCareID(1)) // the best wraps round to no care
	if c := lastOf(ps.stubSim, command.SetMedicalCare); c == nil || c.Pawn != 1 || medcare.Policy(c.Mode) != medcare.NoCare {
		t.Fatalf("SetMedicalCare = %+v", c)
	}
	if w, _ := a.uiSt().Find(assignDefCareID(medcare.Prisoners)); w.Label != "Prisoners: Herbal" || w.Disabled {
		t.Fatalf("prisoners' default %+v", w)
	}
	press(t, a, f, assignDefCareID(medcare.Prisoners))
	if c := lastOf(ps.stubSim, command.SetDefaultCare); c == nil || c.Index != int(medcare.Prisoners) || medcare.Policy(c.Mode) != medcare.IndustrialMax {
		t.Fatalf("SetDefaultCare = %+v", c)
	}
	drawFrame(t, a, f)
}

// Without the read model the default care row is disabled and says why.
func TestAssignDefaultCareWithoutReadModel(t *testing.T) {
	t.Parallel()
	ps := assignStub()
	a, f := colonyApp(t, ps.stubSim)
	a.openAssign()
	f.Frame(t, a)
	w, ok := a.uiSt().Find(assignDefCareID(medcare.Colonists))
	if !ok || !w.Disabled || w.Reason == "" {
		t.Fatalf("default care %+v: disabled with a reason", w)
	}
	if nextCare(medcare.Best) != medcare.NoCare || nextCare(medcare.NoCare) != medcare.NoMeds {
		t.Fatal("the care cycle")
	}
}

func TestDraftedRightClickOffersTend(t *testing.T) {
	a, g, id := liveColony(t, 0)
	bo, err := g.SpawnNamedPawn(6, 6, "Bo")
	if err != nil {
		t.Fatal(err)
	}
	a.renderer.syncPawns()
	selectPawn(t, a, id)
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	a.rightClickTile(6, 6, 100, 100) // a healthy colonist: just walk there
	if a.float.open {
		t.Fatal("nothing to tend: no menu")
	}
	g.Injure(bo, 75) // downed
	a.renderer.syncPawns()
	a.rightClickTile(6, 6, 100, 100)
	if !a.float.open {
		t.Fatal("a drafted right-click on a wounded colonist opens the order menu")
	}
	i := -1
	for k, o := range a.float.opts {
		if o.Label == "Tend Bo" {
			i = k
		}
	}
	if i < 0 || a.float.opts[i].Disabled {
		t.Fatalf("Tend Bo offered: %+v", a.float.opts)
	}
	a.chooseFloat("float." + strconv.Itoa(i))
	g.Step(1)
	a.renderer.syncPawns()
	if p, _ := a.selectedPawn(); !p.Drafted || !strings.HasPrefix(p.Job, "emergency tend") {
		t.Fatalf("the drafted colonist goes to tend: drafted %v, job %q", p.Drafted, p.Job)
	}
}
