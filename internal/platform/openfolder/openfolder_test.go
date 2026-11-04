package openfolder

import "testing"

func TestCommandPassesThePathAsOneArgument(t *testing.T) {
	const p = `/a b/c; rm -rf /`
	for _, os := range []string{"darwin", "windows", "linux"} {
		name, args, err := command(os, p)
		if err != nil || name == "" || len(args) != 1 || args[0] != p {
			t.Errorf("%s: %q %q %v", os, name, args, err)
		}
	}
	if _, _, err := command("plan9", p); err == nil {
		t.Error("an unknown OS must be an error")
	}
}

func TestOpenRefusesEmpty(t *testing.T) {
	if Open("") == nil {
		t.Fatal("empty path accepted")
	}
}
