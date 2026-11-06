package architect

import "testing"

type memShell struct {
	*fakeShell
	on bool
}

func (m memShell) RememberStyles() bool { return m.on }

// armed arms the wall tool, picks the next style and arms the fence and
// the wall again, returning the wall's style at the end.
func styleRoundTrip(t *testing.T, on bool) DrawStyle {
	t.Helper()
	tool, f := newFake(nil)
	tool.sh = memShell{f, on}
	tool.SelectTool("wall")
	tool.rotateOrStyle(1)
	picked := tool.Style
	if picked == StyleDefault {
		t.Fatal("Q/E did not pick a style")
	}
	tool.SelectTool("wall") // disarm
	tool.SelectTool("fence")
	if tool.Style != StyleDefault {
		t.Fatalf("another tool inherited style %v", tool.Style)
	}
	tool.SelectTool("wall")
	return tool.Style
}

func TestRememberStylesBringsBackTheLastStylePerTool(t *testing.T) {
	if got := styleRoundTrip(t, true); got == StyleDefault {
		t.Fatal("wall forgot its style with the option on")
	}
}

func TestStylesResetWithoutTheOption(t *testing.T) {
	if got := styleRoundTrip(t, false); got != StyleDefault {
		t.Fatalf("style %v kept with the option off", got)
	}
}
