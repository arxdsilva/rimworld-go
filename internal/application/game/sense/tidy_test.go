package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
)

func TestTidyKinds(t *testing.T) {
	t.Parallel()
	for _, k := range []job.Kind{job.Research, job.Bill, job.Recreate, job.Read, job.Warden, job.Gather, job.OperateDrill} {
		if g, l, ok := tidyRule(k); !ok || g != gameplay.CleanBeforeWork || l != gameplay.TidyStation {
			t.Errorf("%v: rule %v, limit %v, %v", k, g, l, ok)
		}
	}
	for _, k := range []job.Kind{job.Eat, job.SocialRelax} {
		if g, l, ok := tidyRule(k); !ok || g != gameplay.CleanBeforeMeal || l != gameplay.TidyBench {
			t.Errorf("%v: rule %v, limit %v, %v", k, g, l, ok)
		}
	}
	for _, k := range []job.Kind{job.Sleep, job.Construct, job.CleanRoom, job.Clean, job.Move, job.Attack, job.FightFire, job.Tend} {
		if _, _, ok := tidyRule(k); ok {
			t.Errorf("%v must never be intercepted", k)
		}
	}
}

func TestWalkOrderGoesNearestFirst(t *testing.T) {
	t.Parallel()
	const w = 10
	cells := []tidyCell{{i: 9}, {i: 0}, {i: 5}}
	walkOrder(cells, 0, 0, w)
	if cells[0].i != 0 || cells[1].i != 5 || cells[2].i != 9 {
		t.Fatalf("order %+v", cells)
	}
}

func TestSetBenchWithoutTheGameIsNothing(t *testing.T) {
	t.Parallel()
	s := New(Roles{})
	s.Tidy.SetBench(4, false)
	if len(s.NoClean) != 0 {
		t.Fatal("no roles, no effect")
	}
}
