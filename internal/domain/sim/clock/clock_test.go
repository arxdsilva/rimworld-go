package clock

import "testing"

func TestNewStartsAtZero(t *testing.T) {
	c := New()
	if got := c.Now(); got != 0 {
		t.Fatalf("Now() = %d, want 0", got)
	}
}

func TestAdvanceIncrementsTick(t *testing.T) {
	c := New()
	c.Advance(5)
	if got := c.Now(); got != 5 {
		t.Fatalf("Now() = %d, want 5", got)
	}
	c.Advance(1)
	if got := c.Now(); got != 6 {
		t.Fatalf("Now() = %d, want 6", got)
	}
}

func TestAdvanceIgnoresNonPositive(t *testing.T) {
	c := New()
	c.Advance(0)
	c.Advance(-3)
	if got := c.Now(); got != 0 {
		t.Fatalf("Now() = %d, want 0 (clock must never move backward)", got)
	}
}
