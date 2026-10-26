package job

import "testing"

func TestLineIdleWhenEmpty(t *testing.T) {
	if (Record{}).Line() != "idle" {
		t.Fatalf("empty record line = %q", Record{}.Line())
	}
}

func TestLineIncludesFailReason(t *testing.T) {
	r := Record{Kind: Move, State: Fail, Fail: Unreachable}
	if r.Line() != "goto (unreachable)" {
		t.Fatalf("line = %q", r.Line())
	}
}

func TestActiveOnlyGotoOrDo(t *testing.T) {
	if (Record{Kind: Eat, State: Complete}).Active() {
		t.Fatal("complete must not be active")
	}
	if !(Record{Kind: Haul, State: Goto}).Active() {
		t.Fatal("goto must be active")
	}
}

func TestBuryIsAJob(t *testing.T) {
	if Bury.String() != "bury" || Bury.IsIdle() {
		t.Fatal("bury is real work named bury")
	}
}

func TestFlickIsAJob(t *testing.T) {
	if Flick.String() != "flick switch" || Flick.IsIdle() || (Record{Kind: Flick, State: Do}).Line() != "flick switch do" {
		t.Fatalf("flick: %q", (Record{Kind: Flick, State: Do}).Line())
	}
}
