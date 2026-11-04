package architect

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/domain/sim/command"
)

func orderCard(t *testing.T, id string) Card {
	t.Helper()
	tl, _ := newFake(&fakeSim{})
	tl.Open, tl.Cat = true, "orders"
	for _, c := range tl.Cards() {
		if c.ID == ids.PrefixArchTool+id {
			return c
		}
	}
	t.Fatalf("no card %q", id)
	return Card{}
}

func TestOrderCardsCarryWholeMapOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, label string
		want      command.Command
	}{
		{"forbid", "Forbid all on the map", command.Command{Kind: command.DesignateForbid, X: 0, Y: 0, X2: 39, Y2: 29, Enabled: true}},
		{"allow", "Allow all on the map", command.Command{Kind: command.DesignateForbid, X: 0, Y: 0, X2: 39, Y2: 29, Enabled: false}},
		{"hunt", "Cancel all hunting", command.Command{Kind: command.Undesignate, Order: command.OrderHunt, X: 0, Y: 0, X2: 39, Y2: 29}},
		{"tame", "Cancel all taming", command.Command{Kind: command.Undesignate, Order: command.OrderTame, X: 0, Y: 0, X2: 39, Y2: 29}},
	}
	for _, tc := range cases {
		opts := orderCard(t, tc.id).Options
		if len(opts) != 1 || opts[0].Label != tc.label || opts[0].Cmd != tc.want {
			t.Errorf("%s options %+v, want %q %+v", tc.id, opts, tc.label, tc.want)
		}
	}
	if o := orderCard(t, "mine").Options; len(o) != 0 {
		t.Errorf("mine has options %+v", o)
	}
}
