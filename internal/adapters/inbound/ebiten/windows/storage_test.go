package windows

import (
	"strings"
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
	closed := treeRows(z, map[string]bool{}, "")
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
	w.Storage().sendNode(3, treeRow{special: 0}, true) // the specials sit below the tree, past this small test window
	click("storage.filter.Foods")
	click("storage.copy")
	if w.Storage().StorageClipboard() != 3 {
		t.Fatalf("clip = %d", w.Storage().StorageClipboard())
	}
	w.Storage().Open(5)
	click("storage.paste")
	want := []command.Command{
		{Kind: command.SetStockpileSpecial, Target: 3, Index: 0, Enabled: true},
		{Kind: command.SetStockpileFilter, Target: 3, Name: "Foods", Enabled: true},
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

func TestStorageAllowClearPriorityAndSearch(t *testing.T) {
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.Zones = []game.ZoneView{{ID: 3, Kind: "shelf", Shelf: 9}}
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
	click("storage.allow_all")
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetStockpileFilter || !sh.cmds[0].Enabled || sh.cmds[0].Name != game.FilterRoot {
		t.Fatalf("allow all: %+v", sh.cmds)
	}
	sh.cmds = nil
	click("storage.clear_all")
	if len(sh.cmds) != 1 || sh.cmds[0].Enabled || sh.cmds[0].Name != game.FilterRoot {
		t.Fatalf("clear all: %+v", sh.cmds)
	}
	sh.cmds = nil
	click("storage.priority." + game.StoragePriorities()[0])
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetZonePriority || sh.cmds[0].Index != 0 || sh.cmds[0].Target != 3 {
		t.Fatalf("priority: %+v", sh.cmds)
	}
	rows := treeRows(game.ZoneView{Kind: "stockpile"}, map[string]bool{}, "meal")
	start := 0
	for i, r := range rows {
		if r.id == "storage.h.items" {
			start = i + 1
		}
	}
	for _, r := range rows[start:] {
		if !strings.Contains(strings.ToLower(r.label), "meal") {
			t.Errorf("search kept %q", r.label)
		}
	}
	if len(rows) <= start {
		t.Fatalf("search found no meal rows")
	}
}

func TestStorageQualityRangeUsesNames(t *testing.T) {
	t.Parallel()
	if storageQualities[0] != "awful" || storageQualities[qualityMax] != "legendary" || len(storageQualities) != qualityMax+1 {
		t.Fatalf("names = %v", storageQualities)
	}
}
