package kinds

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/pkg/randx"
)

// TestRewardPoolsPayRareGoods: each item pool's rare goods (checklist 29.3)
// do turn up across rolls, never above their Max per line.
func TestRewardPoolsPayRareGoods(t *testing.T) {
	pools := []struct {
		name string
		kind qd.KindID
		pool []qd.PoolItem
		rare []item.Kind
	}{
		{"camp", qd.Camp, campPool, []item.Kind{item.ChargeRifle, item.ShieldBelt, item.AdvancedHelmet}},
		{"stash", qd.Stash, stashPool, []item.Kind{item.AdvancedComponents, item.NanomedKit, item.Schematic}},
		{"request", qd.Request, requestRewards, []item.Kind{item.ServoArm, item.OpticImplant, item.NanomedKit}},
	}
	for _, p := range pools {
		max := map[item.Kind]int{}
		for _, pi := range p.pool {
			max[pi.Kind] = pi.Max
		}
		seen := map[item.Kind]bool{}
		for seed := uint64(1); seed <= 400; seed++ {
			r := randx.New(seed)
			in := qd.RewardIn{Budget: 2500, Def: qd.DefOf(p.kind), Pool: p.pool}
			for _, rw := range qd.PickRewards(in, r.IntN) {
				for _, l := range rw.Lines {
					if m := max[l.Kind]; m > 0 && l.Qty > m {
						t.Fatalf("%s: a line of %d %v, over its Max %d", p.name, l.Qty, l.Kind, m)
					}
					seen[l.Kind] = true
				}
			}
		}
		for _, k := range p.rare {
			if !seen[k] {
				t.Errorf("%s: rare %v never paid in 400 rolls", p.name, k)
			}
		}
	}
}
