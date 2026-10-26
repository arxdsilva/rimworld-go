package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"os"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 (INFRA): save version 5. Lamps need power from now on, so an
// older save's lamps become torch lamps (which burn logs and need no
// power) and an old colony keeps its light.

// lampColony is a small colony with two lamps, a planned lamp and some
// pieces that must be left alone.
func lampColony(t *testing.T) (g *Game, lamps []entity.EntityID, frame entity.EntityID, camp, chair entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 16, Height: 10})
	lamps = append(lamps,
		buildNow(t, g, construction.Lamp, item.Wood, construction.South, 3, 3),
		buildNow(t, g, construction.Lamp, item.Wood, construction.South, 9, 6))
	camp = buildNow(t, g, construction.Campfire, item.Wood, construction.South, 6, 2)
	g.setFuel(camp, 1234)
	chair = buildNow(t, g, construction.Chair, item.Wood, construction.South, 12, 7)
	b, _ := g.bldgs.Get(lamps[1])
	b.Damage = 3 // a hurt lamp keeps its hurt
	g.bldgs.Set(lamps[1], b)
	g.placeFrameStuff(construction.Lamp, item.Wood, construction.South, 13, 2)
	frame = g.layerFrameAt(13, 2, construction.LayerObject)
	if frame == 0 {
		t.Fatal("no lamp frame")
	}
	fr, _ := g.frames.Get(frame)
	fr.Wood = 1 // one log delivered
	fr.WorkLeft = construction.WorkFor(construction.Lamp, fr.Stuff) / 2
	g.frames.Set(frame, fr)
	return g, lamps, frame, camp, chair
}

func TestOlderSaveLampsBecomeTorchLamps(t *testing.T) {
	t.Parallel()
	if torchFullFuel() <= 0 {
		t.Fatal("a torch lamp has no tank")
	}
	for _, v := range []int{4, 3, 1} { // the v < 5 step runs for every older version
		g, lamps, frame, camp, chair := lampColony(t)
		l := loadAsVersion(t, g, v)
		for _, id := range lamps {
			was, _ := g.bldgs.Get(id)
			got, ok := l.bldgs.Get(id)
			if !ok || got.Kind != construction.TorchLamp {
				t.Fatalf("v%d: lamp %d is %v, want a torch lamp", v, id, got.Kind)
			}
			was.Kind = construction.TorchLamp
			if got != was {
				t.Fatalf("v%d: lamp %d changed beyond its kind: %+v, was %+v", v, id, got, was)
			}
			if l.layerBldgAt(was.X, was.Y, construction.LayerObject) != id {
				t.Fatalf("v%d: the cell index lost the torch lamp at %d,%d", v, was.X, was.Y)
			}
			// a full tank: the colony's old light keeps burning
			if full := torchFullFuel(); l.fuel[id] != full {
				t.Fatalf("v%d: torch lamp %d fuel %d, want a full tank of %d", v, id, l.fuel[id], full)
			}
		}
		// nothing else was touched: ids, order, other pieces, other fuel
		if len(l.bldgIDs) != len(g.bldgIDs) {
			t.Fatalf("v%d: %d buildings, want %d", v, len(l.bldgIDs), len(g.bldgIDs))
		}
		for i, id := range g.bldgIDs {
			if l.bldgIDs[i] != id {
				t.Fatalf("v%d: building order changed at %d", v, i)
			}
		}
		for _, id := range []entity.EntityID{camp, chair} {
			was, _ := g.bldgs.Get(id)
			if got, _ := l.bldgs.Get(id); got != was {
				t.Fatalf("v%d: building %d changed: %+v, was %+v", v, id, got, was)
			}
		}
		if l.fuel[camp] != 1234 || len(l.fuel) != len(g.fuel)+len(lamps) {
			t.Fatalf("v%d: fuel table %v: only the campfire's and the torches' tanks are expected", v, l.fuel)
		}
		// a planned lamp becomes a planned torch lamp, with what it had
		fr, ok := l.frames.Get(frame)
		if !ok || fr.Kind != construction.TorchLamp {
			t.Fatalf("v%d: lamp frame is %v, want a torch lamp", v, fr.Kind)
		}
		if fr.Need != construction.WoodCost(construction.TorchLamp) || fr.Wood != 1 {
			t.Fatalf("v%d: frame needs %d has %d", v, fr.Need, fr.Wood)
		}
		if want := construction.WorkFor(construction.TorchLamp, fr.Stuff) / 2; fr.WorkLeft < want-1 || fr.WorkLeft > want+1 {
			t.Fatalf("v%d: frame work left %d, want about half (%d)", v, fr.WorkLeft, want)
		}
		if l.layerFrameAt(13, 2, construction.LayerObject) != frame {
			t.Fatalf("v%d: the frame index lost the torch lamp blueprint", v)
		}
		// no lamp is left, anywhere
		for _, id := range l.bldgIDs {
			if b, _ := l.bldgs.Get(id); b.Kind == construction.Lamp {
				t.Fatalf("v%d: a lamp is left", v)
			}
		}
		// re-saved it is a v5 save and loads as it is
		h, info := saveLoadInfo(t, l)
		if info.Version != SaveVersion || h.StateHash() != l.StateHash() {
			t.Fatalf("v%d: re-save: version %d, same state %v", v, info.Version, h.StateHash() == l.StateHash())
		}
	}
}

func TestCurrentSaveKeepsItsLamps(t *testing.T) {
	t.Parallel()
	g, lamps, frame, _, _ := lampColony(t)
	l := loadAsVersion(t, g, SaveVersion)
	for _, id := range lamps {
		if b, _ := l.bldgs.Get(id); b.Kind != construction.Lamp {
			t.Fatalf("a v%d save's lamp became %v", SaveVersion, b.Kind)
		}
		if _, burns := l.fuel[id]; burns {
			t.Fatalf("a lamp holds fuel in a current save")
		}
	}
	if fr, _ := l.frames.Get(frame); fr.Kind != construction.Lamp {
		t.Fatalf("a v%d save's lamp frame became %v", SaveVersion, fr.Kind)
	}
	g.Step(10) // rooms and roofs are worked out lazily: save a settled colony
	if h, _ := saveLoadInfo(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("a plain round trip changed the state")
	}
}

// TestFixtureSavesMigrate: the shipped old saves load, hold no lamp (none
// has one today), and every torch lamp they got has a tank.
func TestFixtureSavesMigrate(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"testdata/premotion_v1.sav", ng1V2Fixture} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lamps := countKind(t, data, construction.Lamp)
		g, info, err := Load(bytes.NewReader(data))
		if err != nil || info.Version >= SaveVersion {
			t.Fatalf("%s: load: %v (version %d)", path, err, info.Version)
		}
		torches := 0
		for _, id := range g.bldgIDs {
			b, _ := g.bldgs.Get(id)
			switch b.Kind {
			case construction.Lamp:
				t.Fatalf("%s: a lamp survived the migration", path)
			case construction.TorchLamp:
				torches++
				if g.fuel[id] != torchFullFuel() {
					t.Fatalf("%s: torch lamp %d has %d fuel", path, id, g.fuel[id])
				}
			}
		}
		if torches != lamps {
			t.Fatalf("%s: %d lamps became %d torch lamps", path, lamps, torches)
		}
		g.Step(300) // and it keeps running
	}
}

// countKind counts the buildings of kind k in a raw save.
func countKind(t *testing.T, data []byte, k construction.Kind) int {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range s.Bldgs {
		if e.V.Kind == k {
			n++
		}
	}
	return n
}

// TestConduitUnderWallSurvivesSaveAndLoad: a conduit (built, hidden and
// planned) under a wall, a door and a bench is back after a save, on its
// own layer, and the loaded colony continues exactly as the original.
func TestConduitUnderWallSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 4, 4)
	door := buildNow(t, g, construction.Door, item.Wood, construction.South, 5, 4)
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 8, 4)
	c1 := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 4, 4)
	c2 := buildNow(t, g, construction.HiddenConduit, item.Steel, construction.South, 5, 4)
	c3 := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 9, 4)
	g.placeFrameStuff(construction.PowerConduit, item.Steel, construction.South, 6, 4)
	planned := g.layerFrameAt(6, 4, construction.LayerConduit)
	if planned == 0 {
		t.Fatal("no planned conduit")
	}
	if _, err := g.SpawnNamedPawn(2, 8, "Ada"); err != nil {
		t.Fatal(err)
	}
	g.Step(10) // rooms and roofs are worked out lazily: save a settled colony
	h, info := saveLoadInfo(t, g)
	if info.Version != SaveVersion || h.StateHash() != g.StateHash() {
		t.Fatalf("round trip: version %d, same hash %v", info.Version, h.StateHash() == g.StateHash())
	}
	for _, c := range []struct {
		id     entity.EntityID
		x, y   int
		under  entity.EntityID
		layerO construction.Layer
	}{
		{c1, 4, 4, wall, construction.LayerObject}, {c2, 5, 4, door, construction.LayerObject},
		{c3, 9, 4, bench, construction.LayerObject},
	} {
		if got := h.layerBldgAt(c.x, c.y, construction.LayerConduit); got != c.id {
			t.Errorf("conduit at %d,%d = %d, want %d", c.x, c.y, got, c.id)
		}
		if got := h.layerBldgAt(c.x, c.y, c.layerO); got != c.under {
			t.Errorf("piece over the conduit at %d,%d = %d, want %d", c.x, c.y, got, c.under)
		}
	}
	if h.layerFrameAt(6, 4, construction.LayerConduit) != planned {
		t.Error("the planned conduit is lost")
	}
	if h.layerBlockerAt(4, 4) != wall || h.grid.Walkable(4, 4) {
		t.Error("the wall over the conduit stopped blocking")
	}
	if h.pw != nil || g.pw != nil {
		t.Error("conduits alone start no power state")
	}
	// the loaded colony carries on as the original does
	for i := 0; i < 6; i++ {
		g.Step(250)
		h.Step(250)
		if a, b := g.StateHash(), h.StateHash(); a != b {
			t.Fatalf("step %d: state hash %#x (continued) vs %#x (loaded)", i, a, b)
		}
	}
	sameState(t, "after 1500 ticks", g, h)
}
