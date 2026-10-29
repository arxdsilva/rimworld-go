package hud

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestBuildAlerts(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{Name: "Bo", Status: "downed", Reason: "injured", InBed: true},
		{Name: "Cora", Status: "alive", Mood: 20, X: 7, Y: 7},
	}
	w := game.WorldView{Food: 2, Animals: []game.AnimalView{{Species: "ridge_wolf", Label: "ridge wolf", Hostile: true, X: 9, Y: 9}}}
	got := BuildAlerts(pawns, w)
	// the label ("ridge wolf"), not the species key; no target, so generic
	want := []string{"A ridge wolf is attacking!", "Ada needs rescue", "Low food (2 left)", "Cora is unhappy"}
	if len(got) != len(want) {
		t.Fatalf("alerts = %+v", got)
	}
	for i, a := range got {
		if a.Label != want[i] {
			t.Fatalf("alert %d = %q, want %q", i, a.Label, want[i])
		}
	}
	if !got[0].Danger || !got[1].Danger || got[2].Danger || got[1].Targets[0] != [2]int{3, 4} {
		t.Fatalf("severity/targets: %+v", got)
	}
}

// hostileLabel is the "hostile" alert's text for animals and pawns.
func hostileLabel(t *testing.T, pawns []game.PawnView, animals ...game.AnimalView) Alert {
	t.Helper()
	for _, a := range BuildAlerts(pawns, game.WorldView{Animals: animals}) {
		if a.Key == "hostile" {
			return a
		}
	}
	t.Fatalf("no hostile alert for %+v", animals)
	return Alert{}
}

// PLAY-017: the hostile alert says what is chasing whom.
func TestHostileAlertNamesTheChasedColonist(t *testing.T) {
	t.Parallel()
	ada := game.PawnView{ID: entity.EntityID(7), Name: "Ada", Status: "alive", Mood: 80, X: 3, Y: 4}
	wolf := game.AnimalView{ID: 20, Species: "timber_wolf", Label: "timber wolf", Hostile: true, TargetID: ada.ID, X: 9, Y: 9}

	for _, c := range []struct {
		desc    string
		pawns   []game.PawnView
		animals []game.AnimalView
		want    string
	}{
		{"label and target name", []game.PawnView{ada}, []game.AnimalView{wolf}, "A timber wolf is chasing Ada!"},
		{"a vowel takes an", []game.PawnView{ada},
			[]game.AnimalView{{ID: 21, Species: "arctic_fox", Label: "arctic fox", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"An arctic fox is chasing Ada!"},
		{"no label falls back to the spaced key", []game.PawnView{ada},
			[]game.AnimalView{{ID: 22, Species: "timber_wolf", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"A timber wolf is chasing Ada!"},
		{"no target stays generic", []game.PawnView{ada},
			[]game.AnimalView{{ID: 23, Species: "timber_wolf", Label: "timber wolf", Hostile: true, X: 1, Y: 1}},
			"A timber wolf is attacking!"},
		{"a target nobody knows stays generic", nil, []game.AnimalView{wolf}, "A timber wolf is attacking!"},
		{"several animals stay a count", []game.PawnView{ada},
			[]game.AnimalView{wolf, {ID: 24, Species: "boar", Label: "boar", Hostile: true, X: 5, Y: 5}},
			"2 animals are attacking!"},
	} {
		al := hostileLabel(t, c.pawns, c.animals...)
		if al.Label != c.want {
			t.Errorf("%s: label %q, want %q", c.desc, al.Label, c.want)
		}
		if !al.Danger || len(al.Targets) != len(c.animals) {
			t.Errorf("%s: danger=%v targets=%v, want a danger alert aimed at every animal", c.desc, al.Danger, al.Targets)
		}
	}

	// a peaceful animal raises nothing, and a chased prisoner is still named
	if got := BuildAlerts(nil, game.WorldView{Animals: []game.AnimalView{{Species: "deer", Label: "deer", X: 1, Y: 1}}}); len(got) != 0 {
		t.Errorf("peaceful animal alerts: %+v", got)
	}
	pris := game.PawnView{ID: 8, Name: "Bo", Prisoner: true, Status: "alive", Mood: 80}
	boar := game.AnimalView{ID: 25, Species: "boar", Label: "boar", Hostile: true, TargetID: pris.ID, X: 1, Y: 1}
	if al := hostileLabel(t, []game.PawnView{pris}, boar); al.Label != "A boar is chasing Bo!" {
		t.Errorf("prisoner target: %q", al.Label)
	}
}

// rightWorld is the right-column scene of the root tests: three
// colonists in trouble, two raiders, a hostile wolf and little food.
func rightWorld() (pawns []game.PawnView, world game.WorldView) {
	pawns = []game.PawnView{
		{ID: 1, Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "downed", Reason: "no bed", X: 5, Y: 6},
		{ID: 3, Name: "Cora", Status: "alive", Mood: 10, X: 7, Y: 7},
		{ID: 90, Name: "Reaver", Hostile: true, Status: "alive", X: 30, Y: 30},
		{ID: 91, Name: "Reaver", Hostile: true, Status: "alive", X: 50, Y: 40},
	}
	world.Food = 1
	world.Animals = []game.AnimalView{{ID: 20, Species: "timber_wolf", Label: "timber wolf", Hostile: true, X: 9, Y: 9}}
	return pawns, world
}

type fakeTradeStatus struct {
	o game.OrbitalView
	c bool
}

func (f fakeTradeStatus) TradeStatus() (game.OrbitalView, bool) { return f.o, f.c }

func alertKeys(als []Alert) []string {
	var out []string
	for _, a := range als {
		out = append(out, a.Key)
	}
	return out
}

// Danger alerts come before warnings; within a severity they follow the
// key, so the list never reshuffles while its alerts persist.
func TestAlertsAreOrderedBySeverityThenKey(t *testing.T) {
	t.Parallel()
	pawns, world := rightWorld()
	got := BuildAlerts(append(pawns[:0:0], pawns...), world)
	want := []string{"hostile", "raid", "rescue", "collapsed", "food", "unhappy"}
	if keys := alertKeys(got); strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("alert order = %v, want %v", keys, want)
	}
	for _, al := range got {
		wantSev := SevWarning
		if al.Danger {
			wantSev = SevDanger
		}
		if al.Sev != wantSev {
			t.Errorf("%s: sev = %v, want %v for danger=%v", al.Key, al.Sev, wantSev, al.Danger)
		}
	}

	// the same alerts, built from a shuffled roster, come out in the same order
	rev := append(pawns[:0:0], pawns...)
	slices.Reverse(rev)
	if keys := alertKeys(BuildAlerts(rev, world)); strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("reversed roster: alert order = %v, want %v", keys, want)
	}
}

// Every alert the game can raise explains itself in a sentence or two.
func TestEveryAlertCarriesAnExplanation(t *testing.T) {
	t.Parallel()
	pawns, w := rightWorld()
	w.Pens = []game.PenView{{ID: 3, Name: "Pen 3", Enclosed: false, X: 2, Y: 2}}
	w.Animals = append(w.Animals,
		game.AnimalView{ID: 21, Species: "timbergnaw", Label: "timbergnaw", X: 1, Y: 1},
		game.AnimalView{ID: 22, Species: "cow", Label: "cow", Tame: true, Penned: true, Pen: "Roaming", X: 4, Y: 4})
	w.Frames = []game.FrameView{{ID: 5, Kind: "wall", X: 8, Y: 8, Status: "No wood"}}
	w.Buildings = []game.BuildingView{{ID: 6, Kind: "campfire", Fuel: true, FuelPct: 0, X: 12, Y: 12}}
	w.Fires = []game.FireView{{X: 2, Y: 2}}
	w.TerrainW = 80
	w.Home = make([]uint8, 80*80)
	w.Home[2*80+2] = 1
	pawns = append(pawns[:0:0], pawns...)
	pawns = append(pawns,
		game.PawnView{ID: 4, Name: "Dee", Status: "downed", Reason: "no food", X: 1, Y: 1},
		game.PawnView{ID: 5, Name: "Eli", Status: "alive", Mood: 80, MentalBreak: "wander", X: 2, Y: 2},
		game.PawnView{ID: 6, Name: "Fay", Status: "alive", Mood: 80, Hypothermia: 500, Heatstroke: 500, X: 3, Y: 3},
		game.PawnView{ID: 7, Name: "Gil", Status: "alive", Mood: 80, Bleeding: true, BleedDeathHours: 5, X: 4, Y: 4},
		game.PawnView{ID: 8, Name: "Hal", Status: "alive", Mood: 80, Infection: 40, InfectionLosing: true, Wounded: true, UntendedHours: 6, X: 5, Y: 5})
	got := BuildAlerts(pawns, w)
	seen := map[string]bool{}
	for _, al := range got {
		seen[strings.SplitN(al.Key, ".", 2)[0]] = true
		if len(al.Explain) < 24 {
			t.Errorf("alert %q has explanation %q", al.Key, al.Explain)
		}
		if al.Explain == AlertExplain("no-such-alert") {
			t.Errorf("alert %q fell back to the generic explanation", al.Key)
		}
	}
	for _, k := range []string{"bleeding", "infection", "untended", "hostile", "raid", "rescue", "starving", "collapsed", "food", "break", "hypothermia",
		"heatstroke", "nofuel", "waitingframes", "unhappy", "fire", "penopen", "unpenned", "timbergnaw"} {
		if !seen[k] {
			t.Errorf("fixture does not raise %q (got %v)", k, alertKeys(got))
		}
	}
	if AlertExplain("no-such-alert") == "" {
		t.Error("an unknown alert still gets a generic explanation")
	}
}

// A card for a row at the bottom of a short screen is pulled back up.
func TestRightCardIsClampedOnScreen(t *testing.T) {
	t.Parallel()
	c := RightCard{Title: "Title", Body: strings.Repeat("A long explanation that wraps over several lines. ", 6)}
	for _, anchor := range []gfx.Rect{{X: 1040, Y: 690, W: 232, H: 24}, {X: 1040, Y: 0, W: 232, H: 24}, {X: 1040, Y: 360, W: 232, H: 24}} {
		box, title, body := LayoutRightCard(c, anchor, 1280, 720)
		if box.Right() > anchor.X || box.X < 0 || box.Y < 0 || box.Bottom() > 720 {
			t.Errorf("anchor %+v: card %+v", anchor, box)
		}
		if len(title) == 0 || len(body) < 2 {
			t.Errorf("anchor %+v: title %v body %v", anchor, title, body)
		}
	}
	// a screen too narrow for the card still keeps it on screen
	box, _, _ := LayoutRightCard(c, gfx.Rect{X: 150, Y: 100, W: 100, H: 24}, 300, 300)
	if box.X < 0 || box.Right() > 300 {
		t.Errorf("narrow screen: card %+v", box)
	}
}

// The danger backing pulses; reduced motion holds it still.
func TestAlertPulse(t *testing.T) {
	t.Parallel()
	lo, hi := 2.0, -1.0
	for p := uint64(0); p < 120; p++ {
		v := AlertPulse(p, false)
		lo, hi = min(lo, v), max(hi, v)
		if v < 0.5 || v > 1.0001 {
			t.Fatalf("pulse(%d) = %v out of range", p, v)
		}
		if AlertPulse(p, true) != 1 {
			t.Fatalf("reduced motion pulse(%d) = %v, want a steady 1", p, AlertPulse(p, true))
		}
	}
	if hi-lo < 0.2 {
		t.Fatalf("pulse barely moves: %v..%v", lo, hi)
	}
}

// TestBleedingAlert (MED-01): a colonist bleeding out within 12 hours
// raises a danger alert on it; a slower bleed, a dead colonist, a
// prisoner and a raider do not.
func TestBleedingAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "downed", Reason: "injured", InBed: true, Bleeding: true, BleedDeathHours: 9, X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80, Bleeding: true, BleedDeathHours: 13},
		{ID: 3, Name: "Cy", Status: "dead", Bleeding: true, BleedDeathHours: 2},
		{ID: 4, Name: "Dorn", Status: "downed", Prisoner: true, Bleeding: true, BleedDeathHours: 2},
		{ID: 5, Name: "Krell", Status: "downed", Hostile: true, Lying: true, Bleeding: true, BleedDeathHours: 2},
		{ID: 6, Name: "Eve", Status: "alive", Mood: 80, BleedDeathHours: 0}, // not bleeding: the zero hours mean nothing
	}
	got := BuildAlerts(pawns, game.WorldView{Food: 100})
	var bleed []Alert
	for _, a := range got {
		if a.Key == "bleeding" {
			bleed = append(bleed, a)
		}
	}
	if len(bleed) != 1 || bleed[0].Label != "Ada is bleeding out" || !bleed[0].Danger ||
		len(bleed[0].Targets) != 1 || bleed[0].Targets[0] != [2]int{3, 4} {
		t.Fatalf("bleeding alerts %+v (all %v)", bleed, alertKeys(got))
	}
	if got[0].Key != "bleeding" {
		t.Fatalf("a danger sorts first: %v", alertKeys(got))
	}
	pawns[1].BleedDeathHours = 12
	got = BuildAlerts(pawns, game.WorldView{Food: 100})
	for _, a := range got {
		if a.Key == "bleeding" && a.Label != "2 colonists are bleeding out" {
			t.Fatalf("two bleeding: %q", a.Label)
		}
	}
}

// TestInfectionAlert (MED-02): a colonist losing to an infection and one
// with a wound untended 4 hours or more raise warnings; a winning
// infection, a fresh wound, the dead and prisoners do not.
func TestInfectionAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive", Mood: 80, Infection: 40, InfectionLosing: true, X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80, Infection: 40},
		{ID: 3, Name: "Cy", Status: "dead", Infection: 90, InfectionLosing: true, Wounded: true, UntendedHours: 9},
		{ID: 4, Name: "Dorn", Status: "downed", Prisoner: true, Infection: 60, InfectionLosing: true, Wounded: true, UntendedHours: 9},
		{ID: 5, Name: "Eve", Status: "alive", Mood: 80, Wounded: true, UntendedHours: 4, X: 5, Y: 6},
		{ID: 6, Name: "Fin", Status: "alive", Mood: 80, Wounded: true, UntendedHours: 3},
	}
	got := BuildAlerts(pawns, game.WorldView{Food: 100})
	find := func(key string) []Alert {
		var out []Alert
		for _, a := range got {
			if a.Key == key {
				out = append(out, a)
			}
		}
		return out
	}
	inf := find("infection")
	if len(inf) != 1 || inf[0].Label != "Ada is losing to an infection" || inf[0].Danger || inf[0].Sev != SevWarning ||
		len(inf[0].Targets) != 1 || inf[0].Targets[0] != [2]int{3, 4} || inf[0].Explain == AlertExplain("no-such-alert") {
		t.Fatalf("infection alerts %+v (all %v)", inf, alertKeys(got))
	}
	un := find("untended")
	if len(un) != 1 || un[0].Label != "Eve has an untended wound" || un[0].Danger || un[0].Targets[0] != [2]int{5, 6} ||
		un[0].Explain == AlertExplain("no-such-alert") {
		t.Fatalf("untended alerts %+v (all %v)", un, alertKeys(got))
	}
	pawns[1].InfectionLosing = true
	got = BuildAlerts(pawns, game.WorldView{Food: 100})
	if inf := find("infection"); len(inf) != 1 || inf[0].Label != "2 colonists are losing to infections" {
		t.Fatalf("two losing: %+v", inf)
	}
}

// TestDiseaseAlert (MED-04): a warning lists the colonists with a disease,
// naming it for one; prisoners and the dead are left out.
func TestDiseaseAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive", Mood: 80, Disease: "flu", X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80},
		{ID: 3, Name: "Cy", Status: "dead", Disease: "plague"},
		{ID: 4, Name: "Dorn", Status: "alive", Prisoner: true, Disease: "flu"},
	}
	find := func(got []Alert) []Alert {
		var out []Alert
		for _, a := range got {
			if a.Key == "disease" {
				out = append(out, a)
			}
		}
		return out
	}
	d := find(BuildAlerts(pawns, game.WorldView{Food: 100}))
	if len(d) != 1 || d[0].Label != "Ada is sick: flu" || d[0].Danger || d[0].Sev != SevWarning ||
		len(d[0].Targets) != 1 || d[0].Targets[0] != [2]int{3, 4} || d[0].Explain == AlertExplain("no-such-alert") {
		t.Fatalf("disease alerts %+v", d)
	}
	pawns[1].Disease = "gut worms"
	if d := find(BuildAlerts(pawns, game.WorldView{Food: 100})); len(d) != 1 || d[0].Label != "2 colonists are sick" || len(d[0].Targets) != 2 {
		t.Fatalf("two sick: %+v", d)
	}
	pawns[0].Disease, pawns[1].Disease = "", ""
	if d := find(BuildAlerts(pawns, game.WorldView{Food: 100})); len(d) != 0 {
		t.Fatalf("no one sick: %+v", d)
	}
}

// TestOverdoseAlert (MED-07): a major overdose is a warning naming the
// colonist; any at the extreme stage turns it into a danger; below the
// major stage, the dead and prisoners raise none.
func TestOverdoseAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive", Mood: 80, OverdosePct: 55, OverdoseStage: 1, X: 3, Y: 4},
		{ID: 2, Name: "Bo", Status: "alive", Mood: 80, OverdosePct: 30},
		{ID: 3, Name: "Cy", Status: "dead", OverdosePct: 90, OverdoseStage: 2},
		{ID: 4, Name: "Dorn", Status: "alive", Prisoner: true, OverdosePct: 90, OverdoseStage: 2},
	}
	find := func(got []Alert) []Alert {
		var out []Alert
		for _, a := range got {
			if a.Key == "overdose" {
				out = append(out, a)
			}
		}
		return out
	}
	d := find(BuildAlerts(pawns, game.WorldView{Food: 100}))
	if len(d) != 1 || d[0].Label != "Ada has overdosed" || d[0].Danger || d[0].Sev != SevWarning ||
		len(d[0].Targets) != 1 || d[0].Targets[0] != [2]int{3, 4} || d[0].Explain == AlertExplain("no-such-alert") {
		t.Fatalf("overdose alerts %+v", d)
	}
	pawns[1].OverdosePct, pawns[1].OverdoseStage = 80, 2
	d = find(BuildAlerts(pawns, game.WorldView{Food: 100}))
	if len(d) != 1 || d[0].Label != "2 colonists have overdosed" || !d[0].Danger || len(d[0].Targets) != 2 {
		t.Fatalf("an extreme overdose is a danger: %+v", d)
	}
	pawns[0].OverdoseStage, pawns[1].OverdoseStage = 0, 0
	if d := find(BuildAlerts(pawns, game.WorldView{Food: 100})); len(d) != 0 {
		t.Fatalf("below major: no alert: %+v", d)
	}
}

func alertByKeyPrefix(als []Alert, prefix string) []Alert {
	var out []Alert
	for _, a := range als {
		if a.Key == prefix || strings.HasPrefix(a.Key, prefix+".") {
			out = append(out, a)
		}
	}
	return out
}

func TestBrownoutAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		PowerNets: []game.PowerNetView{{ID: 3, Gen: 800, Load: 800, Brownout: true}, {ID: 9, Gen: 900, Load: 100}},
		Buildings: []game.BuildingView{
			{ID: 1, Kind: "electric stove", X: 4, Y: 5, PowerRole: "consumer", PowerNet: 3, Powered: true, PowerUse: 250},
			{ID: 2, Kind: "heater", X: 8, Y: 9, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 3, Kind: "cooler", X: 1, Y: 2, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 4, Kind: "lamp", X: 6, Y: 6, PowerRole: "consumer", PowerNet: 3, Powered: true, PowerUse: 50},
			{ID: 5, Kind: "heater", X: 12, Y: 12, PowerRole: "consumer", PowerNet: 9, Powered: true, PowerUse: 180},
			{ID: 6, Kind: "heater", X: 14, Y: 14, PowerRole: "consumer", PowerNet: 9, NoPowerReason: game.NoPowerSwitchedOff},
		},
	}
	got := alertByKeyPrefix(BuildAlerts(nil, w), "brownout")
	if len(got) != 1 {
		t.Fatalf("brownout alerts = %+v", got)
	}
	al := got[0]
	if !al.Danger {
		t.Error("a brownout is a danger")
	}
	if !strings.Contains(al.Label, "Brownout") || !strings.Contains(al.Label, "electric stove") {
		t.Errorf("label %q should say brownout and name the biggest consumer", al.Label)
	}
	if !slices.Equal(al.Targets, [][2]int{{8, 9}, {1, 2}}) {
		t.Errorf("targets = %v, want the shed heater and cooler", al.Targets)
	}
	if al.Explain == "" || al.Explain == "Select this alert to look at the cause." {
		t.Errorf("explain = %q", al.Explain)
	}
	// dangers come first
	if all := BuildAlerts(nil, w); len(all) == 0 || !strings.HasPrefix(all[0].Key, "brownout") {
		t.Errorf("order = %+v", all)
	}
}

func TestBrownoutAlertPerNetAndWithoutAConsumerName(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		PowerNets: []game.PowerNetView{{ID: 3, Brownout: true}, {ID: 9, Brownout: true}},
		Buildings: []game.BuildingView{
			{ID: 2, Kind: "heater", X: 8, Y: 9, PowerRole: "consumer", PowerNet: 3, NoPowerReason: game.NoPowerBrownout},
			{ID: 6, Kind: "cooler", X: 14, Y: 14, PowerRole: "consumer", PowerNet: 9, NoPowerReason: game.NoPowerBrownout},
		},
	}
	got := alertByKeyPrefix(BuildAlerts(nil, w), "brownout")
	if len(got) != 2 || got[0].Key == got[1].Key {
		t.Fatalf("one alert per net, with its own key: %+v", got)
	}
	for _, al := range got {
		if !strings.Contains(al.Label, "Brownout") || len(al.Targets) != 1 {
			t.Errorf("alert %+v", al)
		}
	}
	if !strings.Contains(got[0].Label, "grid") || got[0].Label == got[1].Label {
		t.Errorf("two grids need telling apart: %q / %q", got[0].Label, got[1].Label)
	}
	// no net in brownout, no alert
	w.PowerNets = []game.PowerNetView{{ID: 3}, {ID: 9}}
	if got := alertByKeyPrefix(BuildAlerts(nil, w), "brownout"); len(got) != 0 {
		t.Fatalf("alerts without a brownout: %+v", got)
	}
}

func TestNotConnectedAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Buildings: []game.BuildingView{
		{ID: 1, Kind: "heater", X: 2, Y: 3, PowerRole: "consumer", NoPowerReason: game.NoPowerNotConnected},
		{ID: 2, Kind: "heater", X: 5, Y: 6, PowerRole: "consumer", PowerNet: 4, Powered: true},
		{ID: 3, Kind: "heater", X: 7, Y: 7, PowerRole: "consumer", PowerNet: 4, NoPowerReason: game.NoPowerSwitchedOff},
	}}
	got := alertByKeyPrefix(BuildAlerts(nil, w), "notconnected")
	if len(got) != 1 {
		t.Fatalf("alerts = %+v", got)
	}
	al := got[0]
	if al.Danger || al.Label != "Heater not connected" || !slices.Equal(al.Targets, [][2]int{{2, 3}}) {
		t.Errorf("single device: %+v", al)
	}
	// several devices, any role (a generator and a battery count too)
	w.Buildings = append(w.Buildings,
		game.BuildingView{ID: 4, Kind: "battery", X: 9, Y: 9, PowerRole: "storage", NoPowerReason: game.NoPowerNotConnected},
		game.BuildingView{ID: 5, Kind: "solar panel", X: 10, Y: 10, PowerRole: "producer", PowerOut: 600, NoPowerReason: game.NoPowerNotConnected})
	al = alertByKeyPrefix(BuildAlerts(nil, w), "notconnected")[0]
	if al.Label != "3 devices not connected" || !slices.Equal(al.Targets, [][2]int{{2, 3}, {9, 9}, {10, 10}}) {
		t.Errorf("three devices: %+v", al)
	}
	if al.Explain == "" || al.Explain == "Select this alert to look at the cause." {
		t.Errorf("explain = %q", al.Explain)
	}
	// a colony with all devices connected raises nothing
	w.Buildings = w.Buildings[1:3]
	if got := alertByKeyPrefix(BuildAlerts(nil, w), "notconnected"); len(got) != 0 {
		t.Fatalf("alert without a loose device: %+v", got)
	}
}

func TestNoFuelAlertCoversGeneratorsAndTorches(t *testing.T) {
	t.Parallel()
	camp := game.BuildingView{ID: 1, Kind: "campfire", X: 1, Y: 1, Fuel: true}
	torch := game.BuildingView{ID: 2, Kind: "torch lamp", X: 2, Y: 2, Fuel: true}
	gen := game.BuildingView{ID: 3, Kind: "wood-fired generator", X: 3, Y: 3, Fuel: true, PowerRole: "producer", NoPowerReason: game.NoPowerNoFuel}
	full := game.BuildingView{ID: 4, Kind: "torch lamp", X: 4, Y: 4, Fuel: true, FuelPct: 60}
	one := func(bs ...game.BuildingView) Alert {
		t.Helper()
		got := alertByKeyPrefix(BuildAlerts(nil, game.WorldView{Buildings: bs}), "nofuel")
		if len(got) != 1 {
			t.Fatalf("nofuel alerts for %d buildings = %+v", len(bs), got)
		}
		return got[0]
	}
	if al := one(camp, full); al.Label != "Campfire out of wood" || len(al.Targets) != 1 {
		t.Errorf("campfire alone: %+v", al)
	}
	if al := one(torch); al.Label != "Torch lamp out of wood" || !slices.Equal(al.Targets, [][2]int{{2, 2}}) {
		t.Errorf("torch alone: %+v", al)
	}
	if al := one(gen); al.Label != "Wood-fired generator out of wood" || !slices.Equal(al.Targets, [][2]int{{3, 3}}) {
		t.Errorf("generator alone: %+v", al)
	}
	if al := one(camp, torch); al.Label != "Out of wood (2)" || len(al.Targets) != 2 || al.Danger {
		t.Errorf("a campfire and a torch: %+v", al)
	}
	two := camp
	two.ID, two.X = 9, 9
	if al := one(camp, two); al.Label != "2 campfires out of wood" {
		t.Errorf("two campfires: %+v", al)
	}
	// a generator that reports no fuel but whose view lacks the Fuel flag still counts
	bare := gen
	bare.Fuel = false
	if al := one(bare); len(al.Targets) != 1 {
		t.Errorf("generator by reason: %+v", al)
	}
	// and it is counted once
	if al := one(gen, camp); len(al.Targets) != 2 || al.Label != "Out of wood (2)" {
		t.Errorf("generator and campfire: %+v", al)
	}
	if got := alertByKeyPrefix(BuildAlerts(nil, game.WorldView{Buildings: []game.BuildingView{full}}), "nofuel"); len(got) != 0 {
		t.Errorf("a torch with fuel raises %+v", got)
	}
	if e := AlertExplain("nofuel"); !strings.Contains(e, "generator") || !strings.Contains(e, "torch") {
		t.Errorf("explanation = %q", e)
	}
}

// The alert memo is keyed on the power data too, so an alert follows a net
// that browns out between two ticks.
func TestAlertsKeyFollowsPowerData(t *testing.T) {
	t.Parallel()
	snap, w := rightWorld()
	k0 := NewAlertsKey(1, 7, snap, w)
	w1 := w
	w1.PowerVer = w.PowerVer + 1
	if NewAlertsKey(1, 7, snap, w1) == k0 {
		t.Fatal("PowerVer is not in the key")
	}
	w2 := w
	w2.PowerNets = []game.PowerNetView{{ID: 1, Brownout: true}}
	if NewAlertsKey(1, 7, snap, w2) == k0 {
		t.Fatal("the nets are not in the key")
	}
	w3 := w
	w3.SolarFlare = !w.SolarFlare
	if NewAlertsKey(1, 7, snap, w3) == k0 {
		t.Fatal("the flare is not in the key")
	}
	if NewAlertsKey(2, 7, snap, w) == k0 || NewAlertsKey(1, 8, snap, w) == k0 {
		t.Fatal("the session and the presentation frame are not in the key")
	}
}

type fakeAllies []game.AllyGroupView

func (f fakeAllies) Allies() []game.AllyGroupView { return f }

// TestAlliesAlert: standing allies raise "Allies on the map (N)" with
// their spots as click targets; a lone leaving group says so; none
// standing raises nothing. The trade alerts gather it too.
func TestAlliesAlert(t *testing.T) {
	t.Parallel()
	got := AllyAlertsFrom(fakeAllies{{Faction: "Ridge", Standing: 2, Total: 3, Spots: [][2]int{{4, 5}, {6, 7}}}})
	if len(got) != 1 || got[0].Key != "allies" || got[0].Label != "Allies on the map (2)" || len(got[0].Targets) != 2 || got[0].Danger {
		t.Fatalf("allies: %+v", got)
	}
	if AlertExplain("allies") == AlertExplain("nosuchalert") {
		t.Fatal("the allies alert has no explanation")
	}
	got = AllyAlerts([]game.AllyGroupView{{Standing: 3, Leaving: true}})
	if len(got) != 1 || got[0].Label != "Allies leaving (3)" {
		t.Fatalf("leaving: %+v", got)
	}
	if got := AllyAlerts([]game.AllyGroupView{{Total: 2}}); len(got) != 0 {
		t.Fatalf("none standing: %+v", got)
	}
	if got := TradeAlertsFrom(fakeAllies{{Standing: 1}}); len(got) != 1 || got[0].Key != "allies" {
		t.Fatalf("through the trade alerts: %+v", got)
	}
}

// TestShipAloneRaisesNoCaravanAlert: an orbital ship in range raises only
// the ship alert, never "Trader caravan visiting", even though
// TradeStatus reports something present.
func TestShipAloneRaisesNoCaravanAlert(t *testing.T) {
	t.Parallel()
	got := TradeAlertsFrom(fakeTradeStatus{game.OrbitalView{Present: true, HoursLeft: 30}, true})
	if len(got) != 1 || got[0].Key != "tradeorbit" {
		t.Fatalf("ship only: %+v", got)
	}
	got = TradeAlertsFrom(fakeTradeStatus{game.OrbitalView{CaravanVisiting: true, CaravanHoursLeft: 9}, true})
	if len(got) != 1 || got[0].Key != "tradecaravan" || got[0].Label != "Trader caravan visiting (9h left)" {
		t.Fatalf("caravan only: %+v", got)
	}
}

func TestTopBarNamesTheBiome(t *testing.T) {
	t.Parallel()
	got := SeasonText(game.WorldView{Biome: "Boreal forest", BiomeKey: "boreal_forest", Season: "Spring", OutdoorTemp: -20})
	if got != "Boreal forest | Spring  -2C outside" {
		t.Fatalf("season text = %q", got)
	}
	if tip := strings.Join(BiomeTooltip(game.WorldView{BiomeKey: "boreal_forest"}), "\n"); !strings.Contains(tip, "Growing days") || !strings.Contains(tip, "winter low") {
		t.Fatalf("tooltip = %q", tip)
	}
	if BiomeTooltip(game.WorldView{}) != nil {
		t.Fatal("no biome key, no tooltip")
	}
}

func TestAnimalAlerts(t *testing.T) {
	t.Parallel()
	// PLAY-015: a 2x2 marker pen (cells 0, 1, 10, 11 of a 10-wide map), an
	// open pen and a roaming cow
	pen := game.PenView{ID: 3, Name: "Pen 1", Enclosed: true, Bits: []uint64{1<<0 | 1<<1 | 1<<10 | 1<<11}}
	open := game.PenView{ID: 7, Name: "Pen 2", X: 6, Y: 6}
	grow := game.ZoneView{ID: 4, Kind: game.ZoneGrow, Cells: [][2]int{{5, 5}}}
	w := game.WorldView{
		TerrainW: 10,
		Zones:    []game.ZoneView{grow},
		Pens:     []game.PenView{pen, open},
		Animals: []game.AnimalView{
			{ID: 1, Tame: true, State: "wandering", Food: 10, X: 0, Y: 0},
			{ID: 2, Tame: true, State: "wandering", Food: 90, X: 1, Y: 0},
			{ID: 3, Species: "deer", State: "feeding", X: 5, Y: 5},
			{ID: 4, Species: "timbergnaw", X: 9, Y: 9},
			{ID: 5, Species: "cow", Tame: true, Penned: true, Pen: "Roaming", Food: 90, X: 8, Y: 2},
		},
	}
	keys := map[string]bool{}
	for _, al := range AnimalAlerts(w) {
		keys[al.Key] = true
	}
	for _, want := range []string{"animalhungry", "cropraid", "timbergnaw", "penfull.3", "penopen.7", "unpenned"} {
		if !keys[want] {
			t.Errorf("missing alert %q in %v", want, keys)
		}
	}
	if len(AnimalAlerts(game.WorldView{})) != 0 {
		t.Error("no animals, no alerts")
	}
}

// An aggressive break raises the red break alert naming the break.
func TestAggressiveBreakAlert(t *testing.T) {
	t.Parallel()
	got := BuildAlerts([]game.PawnView{{ID: 5, Name: "Eli", Status: "alive", MentalBreak: "berserk", X: 2, Y: 2}}, game.WorldView{})
	for _, al := range got {
		if al.Key == "break" && al.Danger && strings.Contains(al.Label, "berserk") {
			return
		}
	}
	t.Fatalf("no berserk alert in %+v", got)
}
