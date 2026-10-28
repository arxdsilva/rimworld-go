package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

type fakeAllies []game.AllyGroupView

func (f fakeAllies) Allies() []game.AllyGroupView { return f }

// TestAlliesAlert: standing allies raise "Allies on the map (N)" with
// their spots as click targets; a lone leaving group says so; none
// standing raises nothing. The trade alerts gather it too.
func TestAlliesAlert(t *testing.T) {
	t.Parallel()
	got := allyAlertsFrom(fakeAllies{{Faction: "Ridge", Standing: 2, Total: 3, Spots: [][2]int{{4, 5}, {6, 7}}}})
	if len(got) != 1 || got[0].key != "allies" || got[0].label != "Allies on the map (2)" || len(got[0].targets) != 2 || got[0].danger {
		t.Fatalf("allies: %+v", got)
	}
	if alertExplain("allies") == alertExplain("nosuchalert") {
		t.Fatal("the allies alert has no explanation")
	}
	got = allyAlerts([]game.AllyGroupView{{Standing: 3, Leaving: true}})
	if len(got) != 1 || got[0].label != "Allies leaving (3)" {
		t.Fatalf("leaving: %+v", got)
	}
	if got := allyAlerts([]game.AllyGroupView{{Total: 2}}); len(got) != 0 {
		t.Fatalf("none standing: %+v", got)
	}
	if got := tradeAlertsFrom(fakeAllies{{Standing: 1}}); len(got) != 1 || got[0].key != "allies" {
		t.Fatalf("through the trade alerts: %+v", got)
	}
}

// TestAlliesNotInColonistBar: allies (visitor views) take no colonist bar
// slot and are not counted for its layout.
func TestAlliesNotInColonistBar(t *testing.T) {
	ps := htPawns(4)
	ps[2].Visitor = true // an ally sent as military aid
	ps[3].Visitor = true
	a, _ := htApp(t, &stubSim{pawns: ps}, 1280, 720)
	if n := a.barCount(); n != 2 {
		t.Fatalf("barCount = %d, want 2 (allies are not on the bar)", n)
	}
	if n := len(a.barPawns()); n != 2 {
		t.Fatalf("barPawns = %d, want 2", n)
	}
	for _, b := range a.colonistBarButtons(a.sw, a.sh) {
		if b.id == barSlotID(3) || b.id == barSlotID(4) {
			t.Fatalf("an ally has a bar slot: %s", b.id)
		}
	}
}

// TestCommsAidEmitsRequest: an allowed aid action shows its goodwill cost
// and sends CommsRequestAid for the faction.
func TestCommsAidEmitsRequest(t *testing.T) {
	t.Parallel()
	v := testCommsView()
	v.Factions[0].Aid = game.CommsActionView{Allowed: true, Cost: 20}
	s := &dealSim{cv: v, cok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	if w, ok := a.uiSt().find("comms.3.aid"); !ok || w.disabled || w.label != "Request aid · 20 goodwill" {
		t.Fatalf("aid = %+v ok=%v", w, ok)
	}
	clickID(t, a, f, "comms.3.aid")
	for _, c := range s.cmds {
		if c.Kind == command.CommsRequestAid && c.Index == 3 {
			return
		}
	}
	t.Fatalf("cmds = %+v", s.cmds)
}

func TestPawnSubtitleNamesAllyAndVisitor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p    game.PawnView
		want string
	}{
		{game.PawnView{Visitor: true, Ally: true, AgeYears: 30}, "Ally, Age 30"},
		{game.PawnView{Visitor: true}, "Visitor"},
		{game.PawnView{Prisoner: true, Visitor: true}, "Prisoner"},
		{game.PawnView{}, "Colonist"},
	} {
		if got := pawnSubtitle(c.p); got != c.want {
			t.Errorf("pawnSubtitle(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
