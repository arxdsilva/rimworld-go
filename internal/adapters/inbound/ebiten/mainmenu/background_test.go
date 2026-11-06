package mainmenu

import (
	"testing"

	"rimworld-go/internal/platform/config"
)

type bgShell struct {
	Shell
	mode string
	seed uint64
}

func (b bgShell) Settings() config.Settings {
	var s config.Settings
	s.Interface.MenuBackground = b.mode
	return s
}
func (b bgShell) MenuSeed() uint64 { return b.seed }

func TestBackgroundFollowsTheOption(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]string{config.MenuDiorama: config.MenuDiorama, config.MenuSplash: config.MenuSplash, "": config.MenuDiorama} {
		if got := New(bgShell{mode: mode}).Background(); got != want {
			t.Errorf("%q -> %q, want %q", mode, got, want)
		}
	}
}

func TestRandomBackgroundIsSeededAndSettlesOncePerLaunch(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for seed := uint64(0); seed < 64; seed++ {
		s := New(bgShell{mode: config.MenuRandom, seed: seed})
		first := s.Background()
		if again := s.Background(); again != first {
			t.Fatalf("seed %d flapped %q -> %q", seed, first, again)
		}
		if first != New(bgShell{mode: config.MenuRandom, seed: seed}).Background() {
			t.Fatalf("seed %d not deterministic", seed)
		}
		seen[first] = true
	}
	if !seen[config.MenuDiorama] || !seen[config.MenuSplash] {
		t.Fatalf("random only ever gave %v", seen)
	}
}

func TestRandomBackgroundWithoutASeedIsTheDiorama(t *testing.T) {
	t.Parallel()
	if got := New(fakeShellMode{mode: config.MenuRandom}).Background(); got != config.MenuDiorama {
		t.Fatalf("got %q", got)
	}
}

type fakeShellMode struct {
	Shell
	mode string
}

func (f fakeShellMode) Settings() config.Settings {
	var s config.Settings
	s.Interface.MenuBackground = f.mode
	return s
}
