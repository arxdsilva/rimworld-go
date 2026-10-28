package inputframe

import (
	"testing"

	"rimworld-go/internal/platform/input"
)

func TestRepeatFire(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]bool{0: false, 1: true, 2: false, 24: false, 25: true, 26: false, 28: true, 31: true} {
		if got := RepeatFire(n); got != want {
			t.Errorf("RepeatFire(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestFrameClickTakenOnce(t *testing.T) {
	t.Parallel()
	var raw Raw
	raw.Press[Left], raw.Down[Left], raw.Release[Right] = true, true, true
	raw.Pressed = []string{"A"}
	raw.Mods = input.ModShift | input.ModAlt
	f := NewFrame(raw)
	if !f.Clicked(Left) || f.Clicked(Right) || !f.Down(Left) || !f.Released(Right) {
		t.Fatalf("mouse state wrong: %+v", f.Raw)
	}
	f.Take(Left)
	if f.Clicked(Left) {
		t.Fatal("a taken click must not report twice")
	}
	if !f.Key("A") || f.Key("B") || !f.Shift() || f.Ctrl() || !f.Alt() {
		t.Fatal("keys or modifiers wrong")
	}
}

func TestFrameTop(t *testing.T) {
	t.Parallel()
	f := NewFrame(Raw{})
	if got := f.Top(); got != input.CtxGlobal {
		t.Fatalf("empty stack Top = %v, want global", got)
	}
}

func TestFakeSourceClearsOneFrameState(t *testing.T) {
	t.Parallel()
	f := &FakeSource{}
	f.Cur.Pressed, f.Cur.Chars, f.Cur.WheelY = []string{"Q"}, []rune("q"), 3
	f.Cur.Press[Left], f.Cur.Down[Left] = true, true
	f.Cur.Held = []string{"Shift"}
	first := f.Poll()
	if len(first.Pressed) != 1 || len(first.Held) != 2 || !first.Press[Left] || first.WheelY != 3 {
		t.Fatalf("first poll = %+v", first)
	}
	second := f.Poll()
	if len(second.Pressed) != 0 || len(second.Chars) != 0 || second.Press[Left] || second.WheelY != 0 || !second.Down[Left] {
		t.Fatalf("second poll = %+v, want only held state kept", second)
	}
}
