package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

type combatEv struct {
	Tick uint64
	Text string
}
type worldWithLog struct{ CombatLog []combatEv }

func TestCombatRowsNewestFirstAndMissingFieldIsEmpty(t *testing.T) {
	t.Parallel()
	if got := CombatRows(game.WorldView{}); len(got) != 0 {
		t.Fatalf("no field: %v", got)
	}
	got := CombatRows(worldWithLog{CombatLog: []combatEv{{10, "a"}, {20, "b"}}})
	if len(got) != 2 || got[0].Text != "b" || got[1].Text != "a" {
		t.Fatalf("rows = %v", got)
	}
}

func TestTreeRowsHideCollapsedChildrenAndCarrySpecials(t *testing.T) {
	t.Parallel()
	sp := game.StockpileSpecials()
	z := game.ZoneView{ID: 3, Special: []string{sp[0], "hp 20-80%"}}
	closed := treeRows(z, map[string]bool{})
	for _, r := range closed {
		if !r.head && r.rng == 0 && r.special < 0 && r.depth > 0 {
			t.Fatalf("collapsed tree shows a child row %q", r.path)
		}
	}
	var hp treeRow
	var on bool
	for _, r := range closed {
		if r.rng == specialHP {
			hp = r
		}
		if r.special == 0 {
			on = r.on
		}
	}
	if hp.lo != 20 || hp.hi != 80 || !on {
		t.Errorf("hp range %d-%d, first special on %v", hp.lo, hp.hi, on)
	}
}

func TestStorageSendsFilterSpecialCopyAndPaste(t *testing.T) {
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.Zones = []game.ZoneView{{ID: 3, Kind: "stockpile"}, {ID: 5, Kind: "stockpile"}}
	w.Storage().Open(3)
	frame := func() {
		sh.f = inputframe.NewFrame(inputframe.Raw{Mx: -1, My: -1})
		sh.kit.Begin()
		w.Declare()
	}
	click := func(id string) {
		frame()
		if !sh.kit.Click(id) {
			t.Fatalf("no widget %s", id)
		}
		frame()
	}
	sp := game.StockpileSpecials()[0]
	click("storage.special." + sp)
	click("storage.filter.Food")
	click("storage.copy")
	if w.Storage().StorageClipboard() != 3 {
		t.Fatalf("clip = %d", w.Storage().StorageClipboard())
	}
	w.Storage().Open(5)
	click("storage.paste")
	want := []command.Command{
		{Kind: command.SetStockpileSpecial, Target: 3, Index: 0, Enabled: true},
		{Kind: command.SetStockpileFilter, Target: 3, Name: "Food", Enabled: true},
		{Kind: command.PasteStockpile, Target: 5, Zone: 3},
	}
	if len(sh.cmds) != len(want) {
		t.Fatalf("cmds = %+v", sh.cmds)
	}
	for i := range want {
		if sh.cmds[i] != want[i] {
			t.Errorf("cmd %d = %+v, want %+v", i, sh.cmds[i], want[i])
		}
	}
}
