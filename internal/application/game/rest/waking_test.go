package rest

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

func TestWakingThoughts(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.bed(10, construction.Bed, 0, 0)
	f.bed(11, construction.SleepingSpot, 0, 0)
	cases := []struct {
		name   string
		bed    uint64
		room   int
		sky    bool
		role   room.Role
		tenths int
		want   []mood.Kind
		value  int // the room thought's value (last RememberValue)
		forget bool
	}{
		{"ground outside", 0, -1, true, room.General, 0, []mood.Kind{mood.SleptOutside}, 0, false},
		{"ground roofed", 0, -1, false, room.General, 0, []mood.Kind{mood.SleptOnGround}, 0, false},
		{"spot in a room", 11, 2, false, room.Bedroom, 0, []mood.Kind{mood.SleptOnGround}, 0, false},
		{"bed outside", 10, -1, true, room.General, 0, []mood.Kind{mood.SleptInBed, mood.SleptOutside}, 0, false},
		{"bed roofed outdoors", 10, -1, false, room.General, 0, []mood.Kind{mood.SleptInBed}, 0, false},
		{"awful bedroom", 10, 2, false, room.Bedroom, 100, []mood.Kind{mood.SleptInBed, mood.SleptInBedroom}, -2, false},
		{"dull bedroom", 10, 2, false, room.Bedroom, 250, []mood.Kind{mood.SleptInBed, mood.SleptInBedroom}, 0, false},
		{"wondrous bedroom", 10, 2, false, room.Bedroom, 2500, []mood.Kind{mood.SleptInBed, mood.SleptInBedroom}, 8, false},
		{"prison cell", 10, 2, false, room.PrisonCell, 450, []mood.Kind{mood.SleptInBed, mood.SleptInBedroom}, 2, false},
		{"awful barracks", 10, 2, false, room.Barracks, 0, []mood.Kind{mood.SleptInBed, mood.SleptInBarracks}, -7, false},
		{"very barracks", 10, 2, false, room.PrisonBarracks, 1000, []mood.Kind{mood.SleptInBed, mood.SleptInBarracks}, 1, false},
		{"dull hospital", 10, 2, false, room.Hospital, 250, []mood.Kind{mood.SleptInBed, mood.SleptInHospital}, -2, false},
		{"mediocre hospital", 10, 2, false, room.Hospital, 350, []mood.Kind{mood.SleptInBed}, 0, true},
		{"bed in a workshop", 10, 2, false, room.Workshop, 2500, []mood.Kind{mood.SleptInBed}, 0, true},
	}
	for _, c := range cases {
		f.slept[1] = entity.EntityID(c.bed)
		f.room, f.sky, f.role, f.tenths = c.room, c.sky, c.role, c.tenths
		f.thought, f.forgot, f.values, f.labels = nil, nil, nil, nil
		s.Waking.After(1)
		if !slices.Equal(f.thought, c.want) || (len(f.forgot) == 1) != c.forget {
			t.Errorf("%s: thoughts %v forgot %v", c.name, f.thought, f.forgot)
		}
		if len(f.values) > 0 && f.values[len(f.values)-1] != c.value {
			t.Errorf("%s: value %v want %d", c.name, f.values, c.value)
		}
	}
}

func TestRoomMoodLaddersAndLabels(t *testing.T) {
	stage := [room.Stages]int{0, 250, 350, 450, 550, 700, 1000, 1500, 2000, 2500}
	for i, want := range [room.Stages]int{-2, 0, 1, 2, 3, 4, 5, 6, 7, 8} {
		if _, v, _, ok := RoomMood(room.Bedroom, stage[i], Slept); !ok || v != want {
			t.Errorf("bedroom stage %d = %d %v", i, v, ok)
		}
	}
	for i, want := range [room.Stages]int{-7, -5, -4, -3, -2, -1, 1, 2, 3, 4} {
		if _, v, _, ok := RoomMood(room.Barracks, stage[i], Slept); !ok || v != want {
			t.Errorf("barracks stage %d = %d %v", i, v, ok)
		}
	}
	for i, want := range [room.Stages]int{none, none, none, 2, 3, 4, 5, 6, 7, 8} {
		for _, c := range []struct {
			role room.Role
			o    Occasion
			k    mood.Kind
		}{{room.Dining, Ate, mood.AteInDiningRoom}, {room.RecRoom, Enjoyed, mood.EnjoyedRecRoom}} {
			k, v, _, ok := RoomMood(c.role, stage[i], c.o)
			if ok != (want != none) || ok && (v != want || k != c.k) {
				t.Errorf("%v stage %d = %v %d %v", c.role, i, k, v, ok)
			}
		}
	}
	// the occasion must match the role: no dining thought on waking
	if _, _, _, ok := RoomMood(room.Dining, 2500, Slept); ok {
		t.Error("slept in a dining room")
	}
	if _, _, _, ok := RoomMood(room.Bedroom, 2500, Ate); ok {
		t.Error("ate in a bedroom")
	}
	_, _, l, _ := RoomMood(room.Bedroom, 0, Slept)
	_, _, l2, _ := RoomMood(room.Barracks, 2000, Slept)
	_, _, l3, _ := RoomMood(room.Dining, 450, Ate)
	if l != "slept in an awful bedroom" || l2 != "slept in an unbelievably impressive barracks" || l3 != "ate in a decent dining room" {
		t.Errorf("labels %q %q %q", l, l2, l3)
	}
	if stageLabel("plain", 3) != "plain" {
		t.Error("label without a slot")
	}
}

func TestSharedBedThought(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.pawn(2, 1, 0)
	f.bed(10, construction.DoubleBed, 0, 0)
	b := f.bldgs[10]
	b.Claimant, b.Claimant2 = 1, 2
	f.bldgs[10] = b
	f.slept[1] = 10
	f.room, f.role, f.tenths = 2, room.Bedroom, 450
	wake := func() (int, bool) {
		f.thought, f.forgot, f.values = nil, nil, nil
		s.Waking.After(1)
		return valueOf(f, mood.SharedBed), slices.Contains(f.forgot, mood.SharedBed)
	}
	if v, _ := wake(); v != -5 {
		t.Fatalf("stranger in my bed at opinion 0 = %d", v)
	}
	f.opinion[[2]entity.EntityID{1, 2}] = -10000
	if v, _ := wake(); v != -10 {
		t.Fatalf("an enemy in my bed = %d", v)
	}
	f.opinion[[2]entity.EntityID{1, 2}] = 6000 // opinion 60: -5+3
	if v, _ := wake(); v != -2 {
		t.Fatalf("a friend in my bed = %d", v)
	}
	f.opinion[[2]entity.EntityID{1, 2}] = 10000
	if v, forgot := wake(); v != 0 || !forgot {
		t.Fatalf("a dear friend in my bed = %d forgot %v", v, forgot)
	}
	f.opinion[[2]entity.EntityID{1, 2}] = 0
	f.mate[1] = 2
	if v, forgot := wake(); v != 0 || forgot {
		t.Fatal("my partner is no stranger")
	}
	f.mate[1] = 0
	b.Claimant2 = 0
	f.bldgs[10] = b
	if v, _ := wake(); v != 0 {
		t.Fatal("sleeping alone in a double bed is not sharing")
	}
}

// valueOf is the value RememberValue gave thought k (0: none). Remember
// appends to f.thought only, RememberValue to f.values too.
func valueOf(f *fakeHost, k mood.Kind) int {
	j := 0
	for _, x := range f.thought {
		if x == mood.SleptInBed || x == mood.SleptOnGround || x == mood.SleptOutside {
			continue
		}
		if x == k {
			return f.values[j]
		}
		j++
	}
	return 0
}

func TestFeltRoomThought(t *testing.T) {
	f := newFake()
	s := New(f)
	f.pawn(1, 0, 0)
	f.room, f.role, f.tenths = 2, room.Dining, 1000
	s.Waking.Felt(1, Ate)
	if !slices.Equal(f.thought, []mood.Kind{mood.AteInDiningRoom}) || f.values[0] != 5 || f.labels[0] != "ate in a very impressive dining room" {
		t.Fatalf("very impressive dining room: %v %v %v", f.thought, f.values, f.labels)
	}
	f.thought, f.values = nil, nil
	s.Waking.Felt(1, Enjoyed) // a dining room is no recreation room
	f.room = -1
	s.Waking.Felt(1, Ate) // outdoors: no room
	if len(f.thought) != 0 {
		t.Fatalf("no room thought expected: %v", f.thought)
	}
}
