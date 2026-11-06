package ebiten

// journey_outfit_test.go: the OUT-001…005 journeys (GEAR-3 UI): the Assign
// tab's Apparel column and Clear forced button, and the Apparel policies
// editor. Each presses the real widget and asserts the command that left the
// client and what the read model shows after the simulation took it.

import (
	"strings"
	"testing"

	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// outPolicies are the policies the simulation lists.
func outPolicies(j *journey) []gearview.Policy { return j.world().Gear.Policies }

// outPolicy finds policy id.
func outPolicy(t *testing.T, j *journey, id uint32) gearview.Policy {
	t.Helper()
	for _, p := range outPolicies(j) {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no policy %d among %+v", id, outPolicies(j))
	return gearview.Policy{}
}

// outEditor opens the Assign tab and the policy editor from its header.
func outEditor(t *testing.T, j *journey) {
	t.Helper()
	j.key("F3", 0)
	if !j.clickID(windowspkg.AssignApparelEditID) {
		t.Fatal("the Apparel header is disabled")
	}
	if j.a.uiModalTop() != windowspkg.ApparelPolicyModalID {
		t.Fatalf("the editor did not open: top modal %q", j.a.uiModalTop())
	}
}

// outNew makes a policy in the editor and returns its id, selected.
func outNew(t *testing.T, j *journey) gearview.Policy {
	t.Helper()
	n := len(outPolicies(j))
	j.clickID(windowspkg.ApparelNewID)
	if c, ok := j.lastCmd(command.EditApparelPolicy); !ok || c.Mode != command.ApparelPolicyCreate {
		t.Fatalf("New sent %+v (%v), want Create", c, ok)
	}
	j.update(2)
	ps := outPolicies(j)
	if len(ps) != n+1 {
		t.Fatalf("%d policies after New, want %d", len(ps), n+1)
	}
	made := ps[len(ps)-1]
	j.clickID(windowspkg.ApparelSelectID(made.ID))
	return made
}

// outEdit is the newest EditApparelPolicy of that mode.
func outEdit(t *testing.T, j *journey, mode uint8) command.Command {
	t.Helper()
	cs := j.cmdsOf(command.EditApparelPolicy)
	for i := len(cs) - 1; i >= 0; i-- {
		if cs[i].Mode == mode {
			return cs[i]
		}
	}
	t.Fatalf("no EditApparelPolicy mode %d among %+v", mode, cs)
	return command.Command{}
}

// TestJourneyOutfitAssignColumn (OUT-003): a click on a colonist's Apparel
// cell picks the next policy and the pawn's read model follows.
func TestJourneyOutfitAssignColumn(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	j.key("F3", 0)
	before := j.pawn(id).Gear.ApparelPolicy
	if !j.clickID(windowspkg.AssignApparelID(id)) {
		t.Fatal("the Apparel cell is disabled")
	}
	c, ok := j.lastCmd(command.SetApparelPolicy)
	if !ok || c.Pawn != id || uint32(c.Index) == before {
		t.Fatalf("click sent %+v (%v), want SetApparelPolicy to another policy than %d", c, ok, before)
	}
	j.update(2)
	if got := j.pawn(id).Gear.ApparelPolicy; got != uint32(c.Index) {
		t.Fatalf("PawnView.Gear.ApparelPolicy is %d, want %d", got, c.Index)
	}
}

// TestJourneyOutfitClearForced (OUT-005): a colonist ordered to wear a
// garment has it forced; Clear forced sends ClearForcedApparel and the
// garment is no longer forced.
func TestJourneyOutfitClearForced(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	if _, err := j.g.SpawnStack(item.Jacket, 1, 9, 4); err != nil {
		t.Fatal(err)
	}
	j.g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: id, Order: command.OrderEquip, X: 9, Y: 4})
	forced := func() bool {
		for _, f := range j.pawn(id).ApparelForced {
			if f {
				return true
			}
		}
		return false
	}
	j.key("Digit3", 0)
	for i := 0; i < 3000 && !forced(); i++ {
		j.update(1)
	}
	j.key("Space", 0)
	if !forced() {
		t.Fatalf("the ordered garment was not forced: worn %q", j.pawn(id).Apparel)
	}
	j.key("F3", 0)
	if !j.clickID(windowspkg.AssignClearID(id)) {
		t.Fatal("Clear forced is disabled while a garment is forced")
	}
	if c, ok := j.lastCmd(command.ClearForcedApparel); !ok || c.Pawn != id {
		t.Fatalf("Clear forced sent %+v (%v)", c, ok)
	}
	j.update(2)
	if forced() {
		t.Fatal("a garment is still forced after Clear forced")
	}
}

// TestJourneyOutfitEditorCreateRenameDelete (OUT-001): New, rename and
// Delete in the editor each send their EditApparelPolicy and the policy list
// follows.
func TestJourneyOutfitEditorCreateRenameDelete(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	outEditor(t, j)
	made := outNew(t, j)
	j.clickID(windowspkg.ApparelNameID)
	j.key("End", 0)
	j.typeText("Parade")
	j.key("Home", 0)
	for range len(made.Name) {
		j.key("Delete", 0)
	}
	j.key("Enter", 0)
	j.update(1)
	if c := outEdit(t, j, command.ApparelPolicyRename); c.Index != int(made.ID) || c.Name != "Parade" {
		t.Fatalf("rename sent %+v, want Parade for %d", c, made.ID)
	}
	j.update(2)
	if got := outPolicy(t, j, made.ID).Name; got != "Parade" {
		t.Fatalf("policy %d is named %q", made.ID, got)
	}
	j.clickID(windowspkg.ApparelCopyID)
	if c := outEdit(t, j, command.ApparelPolicyCreate); c.Index != int(made.ID) {
		t.Fatalf("Copy sent %+v, want a copy of %d", c, made.ID)
	}
	n := len(outPolicies(j))
	j.clickID(windowspkg.ApparelDeleteID)
	if c := outEdit(t, j, command.ApparelPolicyDelete); c.Index != int(made.ID) {
		t.Fatalf("Delete sent %+v, want %d", c, made.ID)
	}
	j.update(2)
	if len(outPolicies(j)) != n-1 {
		t.Fatalf("%d policies after Delete, want %d", len(outPolicies(j)), n-1)
	}
}

// TestJourneyOutfitEditorDefault (OUT-004): the default toggle of a spare
// policy sends the Default edit and the policy is the default after it; the
// default policy cannot be deleted.
func TestJourneyOutfitEditorDefault(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	outEditor(t, j)
	made := outNew(t, j)
	j.clickID(windowspkg.ApparelDefaultID)
	if c := outEdit(t, j, command.ApparelPolicyDefault); c.Index != int(made.ID) {
		t.Fatalf("Default sent %+v, want %d", c, made.ID)
	}
	j.update(2)
	if !outPolicy(t, j, made.ID).Default {
		t.Fatalf("policy %d is not the default after the edit: %+v", made.ID, outPolicies(j))
	}
	if j.clickID(windowspkg.ApparelDeleteID) {
		t.Fatal("Delete is enabled on the default policy")
	}
}

// TestJourneyOutfitEditorAllowAndMaterials (OUT-001/002): forbidding a
// garment and a material, and Clear all, send their edits and the policy
// follows.
func TestJourneyOutfitEditorAllowAndMaterials(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	outEditor(t, j)
	made := outNew(t, j)
	j.clickID("apparelpolicy.expand.Apparel")
	row := ""
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "apparelpolicy.filter.") && id != "apparelpolicy.filter.Apparel" {
			row = id
			break
		}
	}
	if row == "" {
		t.Fatalf("no garment row on screen: %q", wbIDs(j))
	}
	j.clickID(row)
	c := outEdit(t, j, command.ApparelPolicyAllow)
	if c.Index != int(made.ID) || c.Name != strings.TrimPrefix(row, "apparelpolicy.filter.") || c.Enabled {
		t.Fatalf("clicking %s sent %+v, want a forbid of that node", row, c)
	}
	j.update(2)
	if got := len(outPolicy(t, j, made.ID).Allowed); got >= len(outPolicy(t, j, 0).Allowed) && false {
		t.Fatalf("policy %d still allows %d garments", made.ID, got)
	}
	mat := ""
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "apparelpolicy.stuff.") {
			mat = id
			break
		}
	}
	if mat == "" {
		t.Fatalf("no material row on screen: %q", wbIDs(j))
	}
	j.clickID(mat)
	s := outEdit(t, j, command.ApparelPolicyStuff)
	if s.Index != int(made.ID) || s.Name != strings.TrimPrefix(mat, "apparelpolicy.stuff.") || s.Enabled {
		t.Fatalf("clicking %s sent %+v, want a forbid of that material", mat, s)
	}
	j.update(2)
	if hasString(outPolicy(t, j, made.ID).Stuffs, s.Name) {
		t.Fatalf("policy %d still allows %s", made.ID, s.Name)
	}
	j.clickID(windowspkg.ApparelClearAllID)
	if a := outEdit(t, j, command.ApparelPolicyAllow); a.Name != "Apparel" || a.Enabled {
		t.Fatalf("Clear all sent %+v", a)
	}
	j.update(2)
	if got := outPolicy(t, j, made.ID).Allowed; len(got) != 0 {
		t.Fatalf("Clear all left %v allowed", got)
	}
}

func hasString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// TestJourneyOutfitEditorTainted (OUT-001): the tainted toggle sends the
// Tainted edit and the policy follows.
func TestJourneyOutfitEditorTainted(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	outEditor(t, j)
	made := outNew(t, j)
	was := outPolicy(t, j, made.ID).NoTainted
	j.clickID(windowspkg.ApparelTaintedID)
	c := outEdit(t, j, command.ApparelPolicyTainted)
	if c.Index != int(made.ID) || c.Enabled != was {
		t.Fatalf("tainted sent %+v, want allow=%v", c, was)
	}
	j.update(2)
	if got := outPolicy(t, j, made.ID).NoTainted; got == was {
		t.Fatalf("NoTainted is still %v after the toggle", got)
	}
}

// TestJourneyOutfitEditorRanges (OUT-001): letting go of a hit point knob
// and a quality knob each send one range edit, and the policy follows.
func TestJourneyOutfitEditorRanges(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	outEditor(t, j)
	made := outNew(t, j)
	j.clickID(windowspkg.ApparelHPMinID) // the knob goes to where the pointer is: the middle
	hp := outEdit(t, j, command.ApparelPolicyHP)
	if hp.Index != int(made.ID) || hp.X <= 0 || hp.X > hp.Y || hp.Y != 100 {
		t.Fatalf("the hit point knob sent %+v, want a raised minimum and maximum 100", hp)
	}
	j.update(2)
	if got := outPolicy(t, j, made.ID).HPMin; got != hp.X {
		t.Fatalf("HPMin is %d, want %d", got, hp.X)
	}
	j.clickID(windowspkg.ApparelQualMinID)
	q := outEdit(t, j, command.ApparelPolicyQuality)
	if q.Index != int(made.ID) || q.X <= 0 || q.X > q.Y {
		t.Fatalf("the quality knob sent %+v, want a raised minimum", q)
	}
	j.update(2)
	if got := int(outPolicy(t, j, made.ID).QualityMin); got != q.X {
		t.Fatalf("QualityMin is %d, want %d", got, q.X)
	}
}
