package rest

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
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
		score  int
		want   []mood.Kind
		forget bool
	}{
		{"ground outside", 0, -1, true, 0, []mood.Kind{mood.SleptOutside}, false},
		{"ground roofed", 0, -1, false, 0, []mood.Kind{mood.SleptOnGround}, false},
		{"spot in a room", 11, 2, false, 0, []mood.Kind{mood.SleptOnGround}, false},
		{"bed outside", 10, -1, true, 0, []mood.Kind{mood.SleptInBed, mood.SleptOutside}, false},
		{"bed roofed outdoors", 10, -1, false, 0, []mood.Kind{mood.SleptInBed}, false},
		{"awful room", 10, 2, false, -1, []mood.Kind{mood.SleptInBed, mood.SleptInAwfulRoom}, false},
		{"nice room", 10, 2, false, niceRoomScore, []mood.Kind{mood.SleptInBed, mood.SleptInNiceRoom}, false},
		{"plain room", 10, 2, false, 5, []mood.Kind{mood.SleptInBed}, true},
	}
	for _, c := range cases {
		f.slept[1] = entity.EntityID(c.bed)
		f.room, f.sky, f.score = c.room, c.sky, c.score
		f.thought, f.forgot = nil, nil
		s.Waking.After(1)
		if !slices.Equal(f.thought, c.want) || (len(f.forgot) == 1) != c.forget {
			t.Errorf("%s: thoughts %v forgot %v", c.name, f.thought, f.forgot)
		}
	}
}
