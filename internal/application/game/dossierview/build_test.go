package dossierview

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/pawn/skill"
)

func TestNewCardHasNoSkillOrWork(t *testing.T) {
	t.Parallel()
	if c := New(); c.ActiveSkill != NoneID || c.WorkType != NoneID {
		t.Fatalf("%+v", c)
	}
}

func TestTicksLeft(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		until, now uint64
		want       int
	}{{0, 5, 0}, {5, 5, 0}, {4, 5, 0}, {7, 5, 2}} {
		if got := TicksLeft(c.until, c.now); got != c.want {
			t.Errorf("TicksLeft(%d, %d) = %d, want %d", c.until, c.now, got, c.want)
		}
	}
}

func TestWorkTypeOf(t *testing.T) {
	t.Parallel()
	for k, want := range map[job.Kind]int{
		job.None: NoneID, job.Eat: NoneID, job.Wander: NoneID, job.Attack: NoneID, job.Bill: NoneID,
		job.Mine: int(work.Mine), job.OperateDrill: int(work.Mine), job.Haul: int(work.Haul), job.Sow: int(work.Grow),
		job.Cut: int(work.PlantCut), job.Surgery: int(work.Doctor), job.Train: int(work.Handle), job.Warden: int(work.Warden),
		job.Research: int(work.Research), job.Clean: int(work.Clean), job.Construct: int(work.Construct),
		job.Hunt: int(work.Hunt), job.FightFire: int(work.Firefight),
	} {
		if got := WorkTypeOf(k); got != want {
			t.Errorf("%v: %d, want %d", k, got, want)
		}
	}
	if !RunsBill(job.Bill) || !RunsBill(job.HaulIngredients) || !RunsBill(job.PackIngredients) || RunsBill(job.Haul) {
		t.Error("RunsBill")
	}
}

func facts(level int) Facts {
	return Facts{Animals: level, Cooking: level, Medicine: level,
		Melee:        attack.Melee{Skill: level, Skilled: true, Moving: 1000, Manip: 1000, Sight: 1000},
		MineYieldPct: 100, CropYieldPct: 100, ButcherDifficultyPct: 100}
}

// A skilled worker's chances beat a novice's, and each is in range. Every
// line names the formula the work uses, so the exact values are its tests'.
func TestLinesImproveWithSkill(t *testing.T) {
	t.Parallel()
	lo, hi := Lines(facts(0)), Lines(facts(20))
	for _, w := range []struct {
		sk    skill.ID
		i     int
		label string
		less  bool // lower is better
	}{
		{skill.Animals, 0, "Training success", false},
		{skill.Cooking, 0, "Food poison chance from the cook", true},
		{skill.Cooking, 1, "Butchery yield", false},
		{skill.Medicine, 0, "Surgery success in a clean, lit room with herbal medicine", false},
		{skill.Melee, 0, "Melee hit chance", false},
		{skill.Melee, 1, "Melee dodge chance", false},
	} {
		a, b := lo[w.sk][w.i], hi[w.sk][w.i]
		if a.Label != w.label || b.Label != w.label {
			t.Errorf("%v[%d] is %q", w.sk, w.i, a.Label)
		}
		if w.less && !(b.Pct < a.Pct) || !w.less && !(b.Pct > a.Pct) {
			t.Errorf("%s: level 0 %d%%, level 20 %d%%", w.label, a.Pct, b.Pct)
		}
		if a.Pct < 0 || b.Pct > 200 {
			t.Errorf("%s out of range: %d, %d", w.label, a.Pct, b.Pct)
		}
	}
	if got := Lines(facts(7))[skill.Mining][0].Pct; got != 100 {
		t.Errorf("mining yield %d", got)
	}
	if got := Lines(Facts{CropYieldPct: 130})[skill.Plants][0].Pct; got != 130 {
		t.Errorf("harvest yield %d", got)
	}
	if got := Lines(Facts{Cooking: 7, ButcherDifficultyPct: 120})[skill.Cooking][1].Pct; got != 111 {
		t.Errorf("butchery at level 7 on +20%%: %d", got) // 92.5 % x 1.2
	}
}

func TestCleanName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"", "", true}, {"   ", "", true}, {" Ada ", "Ada", true}, {"O'Neil-Ray", "O'Neil-Ray", true},
		{"Zoë", "Zoë", true}, {"R2D2", "", false}, {"a\nb", "", false}, {"Seventeen letters", "", false},
		{"Sixteen letters!", "", false}, {"Sixteen letterss", "Sixteen letterss", true},
	} {
		got, ok := CleanName(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("CleanName(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
