package simulate

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainPrintsColony(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Main([]string{"simulate", "-ticks", "5"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "tick=") || !strings.Contains(out.String(), "pawn ") {
		t.Fatalf("output %q", out.String())
	}
	if code := Main([]string{"simulate", "-nope"}, &out, &errOut); code != 2 {
		t.Fatalf("bad flag exit %d, want 2", code)
	}
}
