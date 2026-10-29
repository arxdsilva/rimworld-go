package rest

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/room"
)

// beats runs pawn id's needs from tick from for steps comfort beats.
func beats(n *Needs, id entity.EntityID, from uint64, steps int) uint64 {
	t := from
	for range steps * needs.StepTicks {
		n.Tick(id, t)
		t++
	}
	return t
}

func TestNeedsTickOnlyOnItsBeat(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	s.Needs.Tick(1, 10) // neither beat
	if len(s.Needs.Export()) != 0 {
		t.Fatal("ticked off its beat")
	}
	if c, b, o := s.Needs.Percents(1); c != 50 || b != 50 || o != 100 {
		t.Fatalf("a new pawn reads %d %d %d", c, b, o)
	}
	s.Needs.Tick(1, needs.StepTicks-1) // id 1: the beat is at 149
	if r := s.Needs.Of(1); r.Comfort != needs.ComfortStart-needs.ComfortFall || r.Outdoors != needs.LevelFull {
		t.Fatalf("after one beat %+v", r)
	}
}

func TestComfortFromSeatsAndBeds(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 2, 2)
	f.bed(20, construction.Stool, 2, 2)
	f.jobs[1] = job.Record{Kind: job.Eat, State: job.Do, TX: 2, TY: 2}
	now := beats(s.Needs, 1, 0, 20)
	if r := s.Needs.Of(1); r.Comfort != 50*needs.LevelPct {
		t.Fatalf("on a stool comfort %d, want the stool's 50 %%", r.Comfort)
	}
	// walking past the stool gives nothing
	f.jobs[1] = job.Record{Kind: job.Eat, State: job.Goto}
	now = beats(s.Needs, 1, now, 1)
	if r := s.Needs.Of(1); r.Comfort != 50*needs.LevelPct-needs.ComfortFall {
		t.Fatalf("walking comfort %d", r.Comfort)
	}
	// asleep in a bed it climbs to the bed's 75 %; on the ground it falls
	f.bed(10, construction.Bed, 5, 5)
	f.jobs[1] = job.Record{Kind: job.Sleep, State: job.Do, Target: 10}
	now = beats(s.Needs, 1, now, 20)
	if r := s.Needs.Of(1); r.Comfort != 75*needs.LevelPct {
		t.Fatalf("in bed comfort %d", r.Comfort)
	}
	f.jobs[1] = job.Record{Kind: job.Sleep, State: job.Do}
	beats(s.Needs, 1, now, 1)
	if r := s.Needs.Of(1); r.Comfort != 75*needs.LevelPct-needs.ComfortFall {
		t.Fatalf("on the ground comfort %d", r.Comfort)
	}
}

func TestComfortThoughts(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 2, 2)
	f.bed(20, construction.Armchair, 2, 2)
	f.jobs[1] = job.Record{Kind: job.Relax, State: job.Do}
	beats(s.Needs, 1, 0, 30)
	if r := s.Needs.Of(1); r.Comfort != 80*needs.LevelPct || r.ComfortStage != 3 {
		t.Fatalf("armchair: %+v", r)
	}
	if last := len(f.values) - 1; last < 0 || f.values[last] != 6 || f.labels[last] != "very comfortable" {
		t.Fatalf("thoughts %v %v %v", f.thought, f.values, f.labels)
	}
	// standing it falls back through the neutral band, forgetting once
	f.jobs[1] = job.Record{}
	f.forgot = nil
	beats(s.Needs, 1, 30*needs.StepTicks, 100)
	if len(f.forgot) != 1 || f.forgot[0] != mood.ComfortMood {
		t.Fatalf("forgot %v", f.forgot)
	}
}

func TestOutdoorsNeed(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.room = 3 // in a room
	f.roofs[[2]int{0, 0}] = [2]bool{true, true}
	now := beats(s.Needs, 1, 0, 10)
	if r := s.Needs.Of(1); r.Outdoors != needs.LevelFull-10*1_125 {
		t.Fatalf("under a mountain %d", r.Outdoors)
	}
	// asleep in bed the loss is a fifth
	f.bed(10, construction.Bed, 4, 4)
	f.jobs[1] = job.Record{Kind: job.Sleep, State: job.Do, Target: 10}
	before := s.Needs.Of(1).Outdoors
	beats(s.Needs, 1, now, 1)
	if r := s.Needs.Of(1); r.Outdoors != before-225 {
		t.Fatalf("in bed %d -> %d", before, r.Outdoors)
	}
	// long indoors: the ladder runs down to entombed
	s.Needs.m[1] = NeedRecord{Comfort: needs.ComfortStart, Beauty: needs.BeautyStart, Outdoors: 0, ComfortStage: -1, BeautyStage: -1, OutdoorsStage: -1}
	f.values, f.labels = nil, nil
	beats(s.Needs, 1, now, 1)
	if r := s.Needs.Of(1); r.OutdoorsStage != 0 || f.values[len(f.values)-1] != -9 {
		t.Fatalf("entombed %+v %v", r, f.values)
	}
}

func TestTileBeauty(t *testing.T) {
	f := newFake()
	s := New(f)
	// a wood bed (1) counts on its corner only; a carpet is a floor of +2
	f.bed(10, construction.Bed, 0, 0)
	b := f.bldgs[10]
	b.Stuff = item.Wood
	f.bldgs[10] = b
	f.bed(11, construction.Carpet, 3, 3)
	f.tiles[[2]int{0, 0}] = Tile{Indoors: true, Floor: room.WoodFloor}
	f.tiles[[2]int{0, 1}] = Tile{Indoors: true, Floor: room.WoodFloor}
	f.tiles[[2]int{3, 3}] = Tile{Indoors: true}
	f.tiles[[2]int{5, 5}] = Tile{Indoors: false, Filth: filth.Full, Plant: plant.Tree, Planted: true, Mature: true}
	f.tiles[[2]int{6, 6}] = Tile{Indoors: false, Plant: plant.FireLily, Planted: true}
	f.tiles[[2]int{7, 7}] = Tile{Indoors: true, Floor: room.Bare}
	for _, c := range []struct {
		x, y, want int
	}{{0, 0, 10}, {0, 1, 0}, {3, 3, 20}, {5, 5, -45 + 10}, {6, 6, 0}, {7, 7, -10}, {9, 9, 0}} {
		if got := s.Needs.TileBeauty(c.x, c.y); got != c.want {
			t.Errorf("TileBeauty(%d, %d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestPerceivedBeautyInARoom(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 10, 10)
	f.room = room.Outdoors
	// the pawn's room is the 3x3 around it, carpeted; outside it the
	// ground is filthy but out of sight
	for y := 9; y <= 11; y++ {
		for x := 9; x <= 11; x++ {
			f.rooms[[2]int{x, y}] = 4
			f.tiles[[2]int{x, y}] = Tile{Indoors: true, Floor: room.CarpetFloor}
		}
	}
	f.tiles[[2]int{12, 10}] = Tile{Filth: filth.Full * 10}
	if got := s.Needs.Perceived(10, 10); got != 20 {
		t.Fatalf("perceived %d, want 20", got)
	}
	if f.tileReads != 9 {
		t.Fatalf("read %d tiles, want the room's 9", f.tileReads)
	}
	// outdoors it sees the open ground within the radius: 241 cells
	// minus the room's 9, standing in a doorway too
	f.tileReads = 0
	f.rooms[[2]int{20, 20}] = room.Wall
	s.Needs.Perceived(20, 20)
	if f.tileReads != 241-1 {
		t.Fatalf("outdoors read %d tiles", f.tileReads)
	}
	// a look moves the need toward the target; asleep it holds
	f.rooms[[2]int{20, 20}] = 4
	s.Needs.Tick(1, needs.BeautySampleTicks-1)
	if r := s.Needs.Of(1); r.Beauty != needs.BeautyStep(needs.BeautyStart, needs.BeautyTarget(20)) {
		t.Fatalf("after a look %+v", r)
	}
	f.jobs[1] = job.Record{Kind: job.Sleep, State: job.Do}
	before := s.Needs.Of(1).Beauty
	s.Needs.Tick(1, 2*needs.BeautySampleTicks-1)
	if s.Needs.Of(1).Beauty != before {
		t.Fatal("a sleeper's beauty moved")
	}
}

// TestBeautyThoughtDoesNotFlap: a pawn pacing between a spot that pulls
// its beauty need to 57 % and one that pulls it to 59 % (either side of
// the pretty boundary, 58 %) never flips the thought.
func TestBeautyThoughtDoesNotFlap(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.room = 2
	// k carpeted cells of the 241 in view: average 0.7 (k 85) or 0.9 (k 109)
	view := func(k int) map[[2]int]Tile {
		m := map[[2]int]Tile{}
		for y := -8; y <= 8; y++ {
			for x := -8; x <= 8; x++ {
				if x*x+y*y > beautyReachSq {
					continue
				}
				fl := room.WoodFloor
				if k > 0 {
					fl, k = room.CarpetFloor, k-1
				}
				m[[2]int{x, y}] = Tile{Indoors: true, Floor: fl}
			}
		}
		return m
	}
	low, high := view(85), view(109)
	f.tiles = low
	if s.Needs.Perceived(0, 0) != 7 {
		t.Fatalf("low view %d", s.Needs.Perceived(0, 0))
	}
	changes, stage := 0, int8(-1)
	for i := range 200 {
		f.tiles = low
		if i%2 == 1 {
			f.tiles = high
		}
		s.Needs.Tick(1, uint64(i+1)*needs.BeautySampleTicks-1)
		if r := s.Needs.Of(1); r.BeautyStage != stage {
			stage, changes = r.BeautyStage, changes+1
		}
	}
	if r := s.Needs.Of(1); changes != 1 || r.BeautyStage != 3 || r.Beauty != 59*needs.LevelPct {
		t.Fatalf("beauty thought changed %d times, ends %+v", changes, r)
	}
}

func TestNeedsSaveRoundTrip(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	beats(s.Needs, 1, 0, 3)
	saved := map[entity.EntityID]NeedRecord{}
	for k, v := range s.Needs.Export() {
		saved[k] = v
	}
	s2 := New(f)
	s2.Needs.Import(saved)
	if s2.Needs.Of(1) != s.Needs.Of(1) {
		t.Fatalf("round trip %+v vs %+v", s2.Needs.Of(1), s.Needs.Of(1))
	}
	s2.Needs.Import(nil) // an older save: everyone afresh
	if len(s2.Needs.Export()) != 0 || s2.Needs.Of(1) != newRecord() {
		t.Fatal("nil import")
	}
}

func TestSeatQualityScalesComfort(t *testing.T) {
	f := newFake()
	s := New(f)
	f.bed(20, construction.Armchair, 1, 1)
	b := f.bldgs[20]
	b.Quality = quality.Stored(quality.Legendary)
	f.bldgs[20] = b
	if c, ok := s.Seats.ComfortAt(1, 1); !ok || c <= 800 {
		t.Fatalf("legendary armchair comfort %d", c)
	}
}
