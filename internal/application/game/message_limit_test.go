package game

import (
	"testing"
)

// TestRepeatedMessageMerged (EAI-032): the same plain notice again within
// 600 ticks becomes one message with a repeat count; after the window it
// is a new message.
func TestRepeatedMessageMerged(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	g.message(3, 3, "The %s is cold.", "stove")
	g.message(3, 3, "The %s is cold.", "stove")
	g.message(3, 3, "The %s is cold.", "stove")
	g.message(3, 3, "Something else.")
	if len(g.messages) != 2 || g.messages[0].Repeat != 2 {
		t.Fatalf("three twins merge into one with Repeat 2: %+v", g.messages)
	}
	g.Step(repeatWindow + 1)
	g.message(3, 3, "The %s is cold.", "stove")
	if n := len(g.messages); n != 3 || g.messages[n-1].Repeat != 0 {
		t.Fatalf("after the window it is a new notice: %+v", g.messages)
	}
}

// TestDangerNotLimited: danger messages and letters are never merged.
func TestDangerNotLimited(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	for i := 0; i < 3; i++ {
		g.threat("Raid", 3, 3, "Raiders are coming.")
	}
	if len(g.messages) != 3 {
		t.Fatalf("danger messages stay separate: %d", len(g.messages))
	}
	for _, m := range g.messages {
		if m.Repeat != 0 {
			t.Fatal("a danger message has no repeat")
		}
	}
}
