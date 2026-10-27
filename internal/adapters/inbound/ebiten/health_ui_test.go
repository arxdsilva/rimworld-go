package ebiten

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// healthSimView is a colonist missing its left leg and foot with a cut
// right arm, a peg leg queued.
func healthSimView() game.HealthView {
	parts := make([]game.PartView, body.NumParts)
	for p := body.Part(0); p < body.NumParts; p++ {
		parts[p] = game.PartView{Label: p.Label(), Status: game.PartHealthy, HP: body.DefOf(p).MaxHP}
	}
	parts[body.LegL].Status, parts[body.LegL].HP = game.PartMissing, 0
	parts[body.FootL].Status, parts[body.FootL].HP = game.PartMissing, 0
	parts[body.ArmR].Status, parts[body.ArmR].HP = "wounded: 1 wound", 25
	parts[body.Jaw].Status = game.PartArtificial + ": denture"
	return game.HealthView{
		Capacities: []game.CapacityView{{Label: "Consciousness", Pct: 96}, {Label: "Moving", Pct: 50}, {Label: "Talking", Pct: 77}},
		Parts:      parts, Pain: 12,
		Operations: []game.OperationView{{Label: "Install peg leg (left leg)", Slot: 0}},
	}
}

func healthApp(t *testing.T) (*App, *fakeInput, *stubSim) {
	t.Helper()
	s := bioSim()
	s.health = map[entity.EntityID]game.HealthView{1: healthSimView()}
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.frame(t, a)
	openPawnTab(t, a, "health") // the pane's Health tab
	f.frame(t, a)
	return a, f, s
}

func TestHealthTabListsPartsAndOperations(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	for _, id := range []string{"health.part.16", "health.part.18", "health.part.13", "health.part.6", healthRemoveID(0), healthAddID} {
		if !hasWidget(a, id) {
			t.Fatalf("widget %s missing", id)
		}
	}
	if hasWidget(a, "health.part.0") {
		t.Fatal("a healthy brain is not listed")
	}
	if w, _ := a.uiSt().find("health.part.16"); w.label != "left leg: missing" {
		t.Fatalf("leg row = %q", w.label)
	}
	press(t, a, f, healthRemoveID(0))
	if n := len(s.cmds); n == 0 || s.cmds[n-1].Kind != command.RemoveOperation || s.cmds[n-1].Index != 0 || s.cmds[n-1].Pawn != 1 {
		t.Fatalf("cancel sends RemoveOperation: %+v", s.cmds)
	}
	drawFrame(t, a, f)
}

func TestHealthOptionsFollowTheBody(t *testing.T) {
	t.Parallel()
	opts := healthOptions(healthSimView())
	labels := map[string]healthOption{}
	for _, o := range opts {
		labels[o.label] = o
	}
	if o, ok := labels["Install prosthetic leg (left leg)"]; !ok || o.op != body.InstallImplant || o.part != body.LegL || o.item != item.ProstheticLeg {
		t.Fatalf("prosthetic leg option = %+v (%v)", o, ok)
	}
	if _, ok := labels["Install peg leg (left leg)"]; ok {
		t.Fatal("a queued operation is not offered again")
	}
	if _, ok := labels["Remove implant (jaw)"]; !ok {
		t.Fatal("an implant can be removed")
	}
	if _, ok := labels["Amputate right arm"]; !ok {
		t.Fatal("a limb can be amputated")
	}
	for _, o := range opts {
		if o.part == body.Heart || o.part == body.Brain || o.part == body.Torso {
			t.Fatalf("vital parts are never amputated: %+v", o)
		}
		if o.part == body.FootL && o.op == body.InstallImplant && o.item != item.ProstheticFoot {
			t.Fatalf("only a prosthetic foot fits a missing foot: %+v", o)
		}
	}
}

func TestHealthPickerSendsAddOperation(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	press(t, a, f, healthAddID)
	f.frame(t, a)
	if a.uiModalTop() != healthPickID {
		t.Fatalf("picker not open (top %q)", a.uiModalTop())
	}
	if w, ok := a.uiSt().find("modal." + healthPickID + ".health.addop.ok"); !ok || !w.disabled {
		t.Fatalf("Add waits for a pick: %+v", w)
	}
	opts := healthOptions(s.health[1])
	want := -1
	for i, o := range opts {
		if o.item == item.ProstheticLeg {
			want = i
		}
	}
	press(t, a, f, optID(healthPickList, want))
	press(t, a, f, "modal."+healthPickID+".health.addop.ok")
	var got *command.Command
	for i := range s.cmds {
		if s.cmds[i].Kind == command.AddOperation {
			got = &s.cmds[i]
		}
	}
	if got == nil || got.Pawn != 1 || got.Index != int(body.InstallImplant) || got.Build != uint8(body.LegL) || got.Stuff != uint8(item.ProstheticLeg) {
		t.Fatalf("AddOperation = %+v", got)
	}
	if a.uiModalTop() == healthPickID {
		t.Fatal("Add closes the picker")
	}
}

func TestHealthTabStates(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	s.health = nil
	drawFrame(t, a, f)
	if hasWidget(a, healthAddID) {
		t.Fatal("no read model: No data yet, no controls")
	}
	h := healthSimView()
	h.Operations = nil
	for i := range h.Parts {
		h.Parts[i].Status = game.PartHealthy
	}
	s.health = map[entity.EntityID]game.HealthView{1: h}
	s.pawns[0].Status = "dead"
	drawFrame(t, a, f)
	if w, ok := a.uiSt().find(healthAddID); !ok || !w.disabled || w.reason == "" {
		t.Fatalf("a dead colonist's Add operation explains why: %+v", w)
	}
	if hasWidget(a, healthRemoveID(0)) || hasWidget(a, "health.part.16") {
		t.Fatal("nothing queued, nothing hurt: no rows")
	}
}

// TestHealthTabBleeding (MED-01): the headline line is red while bleeding,
// the blood loss row shows with its bar, a bleeding part's tooltip gives
// its rate, and the pane's status lines say so.
func TestHealthTabBleeding(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := healthSimView()
	h.State, h.BleedPerDay, h.BloodLossPct, h.DeathInHours = "Bleeding 140%/day · death in 9h", 140, 37, 9
	h.Parts[body.ArmR].Bleed = 140
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	w, ok := a.uiSt().find(healthStateID)
	if !ok || w.label != h.State || !strings.Contains(w.tip, "9 hours") {
		t.Fatalf("state line %+v", w)
	}
	if healthStateColor(h) != uiDanger {
		t.Fatal("bleeding is red")
	}
	if w, ok := a.uiSt().find(healthBloodID); !ok || w.label != "Blood loss: 37%" {
		t.Fatalf("blood row %+v", w)
	}
	if w, ok := a.uiSt().find("health.part.13"); !ok || !strings.Contains(w.tip, "Bleeding: 140%/day") {
		t.Fatalf("arm tooltip %+v", w)
	}
	// tended and healthy: no blood row, a calm headline
	h.State, h.BleedPerDay, h.BloodLossPct, h.DeathInHours = "Healthy", 0, 0, -1
	h.Parts[body.ArmR].Bleed = 0
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	if hasWidget(a, healthBloodID) || healthStateColor(h) != uiText {
		t.Fatal("no blood loss: no row")
	}
	if w, _ := a.uiSt().find(healthStateID); strings.Contains(w.tip, "hours") {
		t.Fatalf("healthy tip %q", w.tip)
	}
	if healthStateColor(game.HealthView{State: "Downed"}) != uiWarning {
		t.Fatal("downed is amber")
	}
	h.BloodLossPct = 20 // recovering: the row stays until the blood is back
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	if w, ok := a.uiSt().find(healthBloodID); !ok || !strings.Contains(w.tip, "third") {
		t.Fatalf("recovering row %+v", w)
	}
}

func TestPawnStatusSaysBleeding(t *testing.T) {
	t.Parallel()
	name := func(entity.EntityID) string { return "" }
	got := pawnStatusLines(game.PawnView{Status: "alive", Bleeding: true, BleedDeathHours: 7}, name)
	if !slices.Contains(got, "Bleeding (death in 7h)") {
		t.Fatalf("lines %v", got)
	}
	if got := pawnStatusLines(game.PawnView{Status: "dead", Bleeding: true}, name); slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "Bleeding") }) {
		t.Fatalf("the dead do not bleed: %v", got)
	}
}

// TestHealthTabConditions (MED-02): an infection gets a row under the
// part rows with its severity and immunity, its tend marker and the
// view's tooltip; the headline turns red while losing; a part tooltip
// lists each wound's tend quality; the pane's status says how the race
// goes.
func TestHealthTabConditions(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := healthSimView()
	h.State, h.DeathInHours = "Infection: losing", 20
	h.Parts[body.ArmR].TendTip = "cut, 10 HP: tended 74%"
	h.Conditions = []game.ConditionView{
		{Label: "Infection (major), right arm", SevPct: 42, ImmPct: 30, TendPct: 74, DueInHours: 5, Losing: true, Tip: "Infection (major), right arm\nTended at 74% quality"},
		{Label: "Infection (minor), left hand", SevPct: 5, Due: true, Tip: "untended"},
	}
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	w, ok := a.uiSt().find(healthCondID(0))
	if !ok || w.label != "Infection (major), right arm: severity 42%, immunity 30%, needs tend in 5h" || !strings.Contains(w.tip, "74% quality") {
		t.Fatalf("condition row %+v", w)
	}
	if w, ok := a.uiSt().find(healthCondID(1)); !ok || !strings.HasSuffix(w.label, "needs tend now") {
		t.Fatalf("due row %+v", w)
	}
	if healthStateColor(h) != uiDanger || healthStateColor(game.HealthView{State: "Infection: winning"}) != uiWarning {
		t.Fatal("losing is red, winning amber")
	}
	if w, _ := a.uiSt().find(healthStateID); !strings.Contains(w.tip, "20 hours") {
		t.Fatalf("losing headline tip %q", w.tip)
	}
	if w, ok := a.uiSt().find("health.part.13"); !ok || !strings.Contains(w.tip, "tended 74%") {
		t.Fatalf("arm tooltip %+v", w)
	}
	if healthCondDue(game.ConditionView{}) != "" {
		t.Fatal("no tending, no marker")
	}
	h.Conditions = nil
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	if hasWidget(a, healthCondID(0)) {
		t.Fatal("no conditions, no rows")
	}
	name := func(entity.EntityID) string { return "" }
	if got := pawnStatusLines(game.PawnView{Status: "alive", Infection: 42, InfectionLosing: true}, name); !slices.Contains(got, "Infection: losing (42%)") {
		t.Fatalf("losing lines %v", got)
	}
	if got := pawnStatusLines(game.PawnView{Status: "alive", Infection: 42}, name); !slices.Contains(got, "Infection: winning (42%)") {
		t.Fatalf("winning lines %v", got)
	}
}

// lastOf is the newest command of kind k the stub got (nil: none).
func lastOf(s *stubSim, k command.Kind) *command.Command {
	for i := len(s.cmds) - 1; i >= 0; i-- {
		if s.cmds[i].Kind == k {
			return &s.cmds[i]
		}
	}
	return nil
}

// MED-03: the care row heads the Health tab and sends SetMedicalCare.
func TestHealthCareRow(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := s.health[1]
	h.Care = uint8(medcare.Best)
	s.health[1] = h
	f.frame(t, a)
	for c := medcare.Policy(0); c < medcare.NumPolicies; c++ {
		w, ok := a.uiSt().find(healthCareID(c))
		if !ok || w.label != careShort(c) || !strings.HasPrefix(w.tip, c.Label()) {
			t.Fatalf("care button %d: %+v", c, w)
		}
	}
	press(t, a, f, healthCareID(medcare.Best)) // already the level: nothing to send
	if c := lastOf(s, command.SetMedicalCare); c != nil {
		t.Fatalf("the current level sends nothing: %+v", c)
	}
	press(t, a, f, healthCareID(medcare.NoMeds))
	if c := lastOf(s, command.SetMedicalCare); c == nil || c.Pawn != 1 || medcare.Policy(c.Mode) != medcare.NoMeds {
		t.Fatalf("SetMedicalCare = %+v", c)
	}
	drawFrame(t, a, f)
}

// MED-03: the Self-tend checkbox sends SetSelfTend; prisoners have none.
func TestHealthSelfTendCheckbox(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	press(t, a, f, healthSelfID)
	if c := lastOf(s, command.SetSelfTend); c == nil || c.Pawn != 1 || !c.Enabled {
		t.Fatalf("SetSelfTend = %+v", c)
	}
	h := s.health[1]
	h.SelfTend = true
	s.health[1] = h
	press(t, a, f, healthSelfID)
	if c := lastOf(s, command.SetSelfTend); c == nil || c.Enabled {
		t.Fatalf("unchecking sends off: %+v", c)
	}
	s.pawns[0].Prisoner = true
	f.frame(t, a)
	if hasWidget(a, healthSelfID) || !hasWidget(a, healthCareID(medcare.NoCare)) {
		t.Fatal("a prisoner has care levels but no self-tend")
	}
	s.pawns[0].Prisoner, s.pawns[0].Status = false, "dead"
	f.frame(t, a)
	if w, _ := a.uiSt().find(healthCareID(medcare.NoMeds)); !w.disabled || w.reason == "" {
		t.Fatalf("the dead: care disabled with a reason: %+v", w)
	}
}

// MED-03: queued operations move up and down through ReorderOperation.
func TestHealthReorderButtons(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := s.health[1]
	h.Operations = append(h.Operations, game.OperationView{Label: "Amputate right arm", Slot: 1})
	s.health[1] = h
	f.frame(t, a)
	if w, _ := a.uiSt().find(healthUpID(0)); !w.disabled || w.reason == "" {
		t.Fatalf("the first can't move up: %+v", w)
	}
	if w, _ := a.uiSt().find(healthDownID(1)); !w.disabled {
		t.Fatalf("the last can't move down: %+v", w)
	}
	press(t, a, f, healthDownID(0))
	if c := lastOf(s, command.ReorderOperation); c == nil || c.Pawn != 1 || c.Index != 0 || c.Count != 1 {
		t.Fatalf("down = %+v", c)
	}
	press(t, a, f, healthUpID(1))
	if c := lastOf(s, command.ReorderOperation); c == nil || c.Index != 1 || c.Count != -1 {
		t.Fatalf("up = %+v", c)
	}
	drawFrame(t, a, f)
	h.Operations = h.Operations[:1]
	s.health[1] = h
	f.frame(t, a)
	if w, _ := a.uiSt().find(healthDownID(0)); !w.disabled {
		t.Fatal("a single operation has nowhere to go")
	}
}

// MED-06: a queued operation shows its success chance, the factors on
// hover, tinted by how risky it is.
func TestHealthOperationChance(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := s.health[1]
	h.Operations[0].ChancePct = 72
	h.Operations[0].ChanceTip = "Success chance 72% (Doc)\nMedicine skill 8: 80%\nHospital bed: ×115%"
	s.health[1] = h
	drawFrame(t, a, f)
	w, ok := a.uiSt().find(healthChanceID(0))
	if !ok || w.label != "72 %" || !strings.Contains(w.tip, "Hospital bed: ×115%") {
		t.Fatalf("chance cell %+v", w)
	}
	if l := strings.Join(a.healthLines(s.pawns[0]), "|"); !strings.Contains(l, "Operation: Install peg leg (left leg) (72 %)") {
		t.Fatalf("plain lines %q", l)
	}
	if healthChanceColor(72) != uiWarning || healthChanceColor(40) != uiDanger || healthChanceColor(98) != uiText {
		t.Fatal("chance tints")
	}
	h.Operations[0].ChanceTip = "" // an older view: no chance cell
	s.health[1] = h
	drawFrame(t, a, f)
	if hasWidget(a, healthChanceID(0)) {
		t.Fatal("no breakdown, no cell")
	}
}

// MED-06: the picker offers organ harvest and transplant, euthanasia and
// anaesthesia with their reasons, and sends the whole-body operations.
func TestHealthPickerSurgeryOptions(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := s.health[1]
	h.Options = []game.OperationOptionView{
		{Label: "Harvest left kidney", Kind: uint8(body.HarvestOrgan), Part: uint8(body.KidneyL)},
		{Label: "Install organ (right kidney)", Kind: uint8(body.InstallOrgan), Part: uint8(body.KidneyR), Item: uint8(item.Kidney),
			Disabled: true, Reason: "no kidney in stock"},
		{Label: "Euthanize", Kind: uint8(body.Euthanize), Part: uint8(body.Torso)},
		{Label: "Anesthetize", Kind: uint8(body.Anesthetize), Part: uint8(body.Torso), Disabled: true, Reason: "needs 1 herbal medicine or better"},
	}
	s.health[1] = h
	f.frame(t, a)
	press(t, a, f, healthAddID)
	f.frame(t, a)
	opts := pickerOptions(s.health[1])
	if len(opts) != 4 || opts[1].reason != "no kidney in stock" || opts[3].reason == "" {
		t.Fatalf("options %+v", opts)
	}
	for i, want := range []string{"cannot", "kills", "Transplants", "woozy"} {
		_ = i
		found := false
		for _, o := range opts {
			found = found || strings.Contains(healthOptionTip(o), want)
		}
		if !found {
			t.Fatalf("no option tip says %q", want)
		}
	}
	if tip := healthOptionTip(opts[1]); !strings.Contains(tip, "no kidney in stock") || !strings.Contains(tip, "queue it anyway") {
		t.Fatalf("a blocked option explains why: %q", tip)
	}
	press(t, a, f, optID(healthPickList, 2))
	press(t, a, f, "modal."+healthPickID+".health.addop.ok")
	c := lastOf(s, command.AddOperation)
	if c == nil || c.Index != int(body.Euthanize) || c.Build != uint8(body.Torso) || c.Pawn != 1 {
		t.Fatalf("euthanize = %+v", c)
	}
}

// MED-06: a prisoner's (and a guest's) Health tab queues operations too.
func TestHealthPrisonerQueuesOperations(t *testing.T) {
	t.Parallel()
	for _, who := range []string{"prisoner", "guest"} {
		a, f, s := healthApp(t)
		if who == "prisoner" {
			s.pawns[0].Prisoner = true
		} else {
			s.pawns[0].Visitor = true
		}
		f.frame(t, a)
		if w, ok := a.uiSt().find(healthAddID); !ok || w.disabled {
			t.Fatalf("%s: Add operation is open: %+v", who, w)
		}
		press(t, a, f, healthAddID)
		f.frame(t, a)
		if a.uiModalTop() != healthPickID {
			t.Fatalf("%s: the picker opens", who)
		}
		press(t, a, f, optID(healthPickList, 0))
		press(t, a, f, "modal."+healthPickID+".health.addop.ok")
		if c := lastOf(s, command.AddOperation); c == nil || c.Pawn != 1 {
			t.Fatalf("%s: AddOperation = %+v", who, c)
		}
	}
}

// MED-06: the vitals monitor is an architect tool with its sprite and icon
// keys (the art pass renders them).
func TestArchVitalsMonitor(t *testing.T) {
	t.Parallel()
	var tool *archTool
	for i := range archTools {
		if archTools[i].id == "vitals_monitor" {
			tool = &archTools[i]
		}
	}
	if tool == nil || tool.build != "vitals monitor" || tool.icon != "vitals_monitor" || tool.category != "furniture" || !strings.Contains(tool.desc, "hospital bed") {
		t.Fatalf("tool %+v", tool)
	}
	for _, f := range []string{"south", "east", "north", "west"} {
		if got := buildSprite("vitals monitor", f, 0, false, false); got != "furniture/vitals_monitor_"+f {
			t.Fatalf("sprite %q", got)
		}
	}
}

// TestCapacityRowTooltip: each capacity row carries its breakdown.
func TestCapacityRowTooltip(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := healthSimView()
	h.Capacities[1].Tip = "Moving\nLeft leg: missing 0%\n= 50%"
	s.health = map[entity.EntityID]game.HealthView{1: h}
	drawFrame(t, a, f)
	w, ok := a.uiSt().find(healthCapID(1))
	if !ok || !strings.Contains(w.tip, "Left leg: missing 0%") || !strings.HasPrefix(w.label, "Moving") {
		t.Fatalf("capacity row %+v", w)
	}
}
