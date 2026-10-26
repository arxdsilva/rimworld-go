package body

import "testing"

// testCond is a kind no phase defines (beyond NumCondKinds); fakeDefs
// gives it two stages so the capacity and pain folds can be checked before
// any real condition exists.
const testCond CondKind = 200

func fakeDefs(k CondKind) CondDef {
	if k != testCond {
		return DefOfCond(k)
	}
	return CondDef{Label: "test", Stages: []Stage{
		{At: 0, Label: "minor"},
		{At: 500, Label: "major", PainAdd: 300, Mods: CapMods{
			{Cap: Moving, Off: -200},
			{Cap: Consciousness, Max: 400, HasMax: true},
			{Cap: Sight, Off: -2000},
		}},
	}}
}

func TestAddCondFindRemove(t *testing.T) {
	var b Body
	if b.AddCond(Condition{Kind: CondNone}) || b.Conds != nil {
		t.Fatal("CondNone is no condition")
	}
	if !b.AddCond(Condition{Kind: testCond, Part: Torso, Sev: 100}) {
		t.Fatal("add")
	}
	if b.AddCond(Condition{Kind: testCond, Part: Torso, Sev: 900}) {
		t.Fatal("a second one of the same kind on the same part is refused")
	}
	if !b.AddCond(Condition{Kind: testCond, Part: LegL, Sev: 50}) || len(b.Conds) != 2 {
		t.Fatal("the same kind on another part is its own condition")
	}
	c, ok := b.Cond(testCond)
	if !ok || c.Part != Torso || c.Sev != 100 {
		t.Fatalf("Cond finds the first: %+v", c)
	}
	c.Sev = 300 // a live pointer into the body
	if b.Conds[0].Sev != 300 {
		t.Fatal("Cond must return the body's own condition")
	}
	if _, ok := b.Cond(CondNone); ok {
		t.Fatal("found CondNone")
	}
	if !b.RemoveCond(testCond) || b.Conds != nil {
		t.Fatalf("RemoveCond drops every condition of the kind: %+v", b.Conds)
	}
	if b.RemoveCond(testCond) {
		t.Fatal("nothing left to remove")
	}
}

func TestStageAt(t *testing.T) {
	d := fakeDefs(testCond)
	for _, c := range []struct {
		sev  int32
		want uint8
	}{{0, 0}, {499, 0}, {500, 1}, {1000, 1}} {
		if got := d.StageAt(c.sev); got != c.want {
			t.Fatalf("StageAt(%d) = %d, want %d", c.sev, got, c.want)
		}
	}
	if (CondDef{}).StageAt(800) != 0 {
		t.Fatal("no stages: stage 0")
	}
}

// TestConditionStageFoldsIntoCapacityAndPain: the current stage's pain
// and capacity mods apply; a stage with none (or no condition at all)
// leaves the body as it was.
func TestConditionStageFoldsIntoCapacityAndPain(t *testing.T) {
	var b Body
	b.Wounds = []Wound{{Part: Torso, Dmg: 10, Kind: Cut}}
	basePain, baseMove, baseCons := b.Pain(), b.Capacity(Moving), b.Capacity(Consciousness)
	b.Conds = []Condition{{Kind: testCond, Part: Torso, Stage: 0}}
	if b.pain(fakeDefs) != basePain || b.capacity(Moving, fakeDefs) != baseMove {
		t.Fatal("a stage without mods changes nothing")
	}
	b.Conds[0].Stage = 1
	if got := b.pain(fakeDefs); got != basePain+300 {
		t.Fatalf("pain %d, want %d", got, basePain+300)
	}
	cons := b.capacity(Consciousness, fakeDefs)
	if cons != 400 {
		t.Fatalf("consciousness capped at 400, got %d (base %d)", cons, baseCons)
	}
	if got := b.capacity(Moving, fakeDefs); got != min32(baseMove, cons)-200 {
		t.Fatalf("moving %d", got)
	}
	if got := b.capacity(Sight, fakeDefs); got != 0 {
		t.Fatalf("a capacity never drops below zero: %d", got)
	}
	b.Conds[0].Stage = 9 // past the table: ignored
	if b.pain(fakeDefs) != basePain {
		t.Fatal("an unknown stage has no effect")
	}
	b.Conds[0].Stage = 1
	if b.Pain() != basePain || b.Capacity(Moving) != baseMove {
		t.Fatal("a kind with no definition has no effect")
	}
}

func TestPainFoldStaysInRange(t *testing.T) {
	var b Body
	b.Wounds = []Wound{{Part: Torso, Dmg: 100, Kind: Cut}}
	b.Conds = []Condition{{Kind: testCond, Stage: 1}}
	if got := b.pain(fakeDefs); got != 1000 {
		t.Fatalf("pain caps at 1000: %d", got)
	}
}

func TestEveryCondKindHasDef(t *testing.T) {
	for k := CondNone + 1; k < NumCondKinds; k++ {
		if DefOfCond(k).Label == "" {
			t.Fatalf("condition kind %d has no definition", k)
		}
	}
	if DefOfCond(NumCondKinds).Label != "" || DefOfCond(CondNone).Label != "" {
		t.Fatal("CondNone and unknown kinds have no definition")
	}
}

func TestBuildCondDefsRejectsBadTables(t *testing.T) {
	bad := [][][]CondEntry{
		{{{Kind: CondNone, Def: CondDef{Label: "x"}}}},
		{{{Kind: NumCondKinds, Def: CondDef{Label: "x"}}}},
	}
	for i, tables := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("table %d must panic", i)
				}
			}()
			buildCondDefs(tables)
		}()
	}
	for k, d := range buildCondDefs(nil) {
		if d.Label != "" || d.Stages != nil {
			t.Fatalf("no tables: kind %d has a definition", k)
		}
	}
}
