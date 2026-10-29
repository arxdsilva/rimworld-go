package door

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// Speed is a door's opening speed (percent) for its kind, stuff and
// power: a plain door by its stuff, an autodoor four times faster while
// powered and a plain door of its stuff when not. wiki: Door, Autodoor
func Speed(k construction.Kind, stuff item.Kind, powered bool) int {
	auto := k == construction.Autodoor && powered
	switch {
	case item.IsStoneBlocks(stuff) || stuff == item.Stone:
		return pick(auto, 180, 45) // wiki: Door (stone blocks 45), Autodoor (180)
	case item.IsMetal(stuff):
		return pick(auto, 400, 100) // wiki: Door (metals 100), Autodoor (400)
	}
	return pick(auto, 480, 120) // wood; wiki: Door (120), Autodoor (480)
}

func pick(auto bool, a, b int) int {
	if auto {
		return a
	}
	return b
}

// BaseOpenTicks is how long a door at 100 % speed takes to open fully.
// original tuning: the wiki publishes the speeds, not the base time.
const BaseOpenTicks = 45

// OpenTicks is how long a door of speed (percent) takes to open (at least
// one tick).
func OpenTicks(speed int) int {
	if speed <= 0 {
		return BaseOpenTicks
	}
	return max(1, BaseOpenTicks*100/speed)
}

// StayOpen is how long a door stays open after the last creature near it
// left: it never closes on someone still walking through, and a stream
// of passers keeps it open instead of slamming it between each.
// original tuning.
const StayOpen = 120

// State is one door's state. OpenedAt is the tick it started opening (0:
// shut), LastUse the last tick a creature was in or beside it, Hold the
// player's "hold open" order. All fields are exported for the save (gob).
type State struct {
	OpenedAt uint64
	LastUse  uint64
	Hold     bool
}

// Shut reports a door that is closed and unheld: the zero State.
func (s State) Shut() bool { return s == State{} }

// Use notes a creature in or beside the door at now: a shut door starts
// to open.
func (s *State) Use(now uint64) {
	if s.OpenedAt == 0 {
		s.OpenedAt = max(now, 1)
	}
	s.LastUse = now
}

// SetHold holds the door open (it starts opening) or lets it close on the
// usual timer, counted from the release.
func (s *State) SetHold(on bool, now uint64) {
	s.Hold = on
	if on || s.OpenedAt != 0 {
		s.Use(now)
	}
}

// Step closes an open, unheld door once StayOpen ticks have passed since
// its last use. It reports whether the door closed.
func (s *State) Step(now uint64) bool {
	if s.OpenedAt == 0 || s.Hold || now < s.LastUse+StayOpen {
		return false
	}
	s.OpenedAt, s.LastUse = 0, 0
	return true
}

// Open reports a door fully open at now, given its opening time.
func (s State) Open(now uint64, openTicks int) bool {
	return s.OpenedAt != 0 && now >= s.OpenedAt+uint64(openTicks)
}

// Wait is how many more ticks a creature arriving at now waits for the
// door to open (0: open already).
func (s State) Wait(now uint64, openTicks int) int {
	if s.OpenedAt == 0 {
		return openTicks
	}
	if end := s.OpenedAt + uint64(openTicks); now < end {
		return int(end - now)
	}
	return 0
}
