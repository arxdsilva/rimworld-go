package cue

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestHostilePodSoundsOnceEach(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	const land = 1000
	ids := func(tick uint64, hostile bool) []string {
		r.v.Tick = clockTick(tick)
		r.b.ListenArrivals(r.v, game.WorldView{Arrivals: []game.ArrivalView{{X: 4, Y: 4, Tick: land, Hostile: hostile}}})
		var out []string
		for _, c := range r.b.Take() {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids(land-RaidPodLead-1, true); len(got) != 0 {
		t.Fatalf("before the descent: %v", got)
	}
	if got := ids(land-RaidPodLead, true); len(got) != 1 || got[0] != "sfx.droppod_incoming" {
		t.Fatalf("incoming: %v", got)
	}
	if got := ids(land-10, true); len(got) != 0 {
		t.Fatalf("incoming repeated: %v", got)
	}
	if got := ids(land, true); len(got) != 1 || got[0] != "sfx.droppod_impact" {
		t.Fatalf("impact: %v", got)
	}
	if got := ids(land+1, true); len(got) != 0 {
		t.Fatalf("impact repeated: %v", got)
	}
}

func TestShipPartHumNearestOnly(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	var bs []game.BuildingView
	for i := 0; i < 4; i++ {
		bs = append(bs, game.BuildingView{ID: entity.EntityID(10 + i), Kind: shipPartKind, X: 6 + 3*i, Y: 6, W: 2, H: 2})
	}
	bs = append(bs, game.BuildingView{ID: 99, Kind: "wall", X: 5, Y: 5})
	w := game.WorldView{Buildings: bs}
	want := map[string]Loop{}
	r.b.Machines(r.v, want, w, camera.ColonyViewport(1280, 720))
	if len(want) != MaxMachineLoops {
		t.Fatalf("%d hums, want %d: %+v", len(want), MaxMachineLoops, want)
	}
	for _, l := range want {
		if l.ID != "amb.ship_part_hum" {
			t.Fatalf("loop %+v", l)
		}
	}
}
