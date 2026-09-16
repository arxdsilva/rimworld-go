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
