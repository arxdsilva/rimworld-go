package mainmenu

import (
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/platform/config"
)

// fakeShell is a Shell with nothing behind it but a save flag.
type fakeShell struct {
	Shell
	saves bool
}

func (f fakeShell) HasSaves() bool                   { return f.saves }
func (f fakeShell) Settings() config.Settings        { return config.Settings{} }
func (f fakeShell) MenuTick() uint64                 { return 0 }
func (f fakeShell) LoadMenuScene() (*Diorama, error) { return nil, ErrNoLoader }

func TestButtonsContinueLeadsOnlyWithSaves(t *testing.T) {
	t.Parallel()
	with := New(fakeShell{saves: true}).Buttons(1280, 720)
	if with[0].ID != ids.MenuContinue || with[0].Disabled {
		t.Fatalf("first button %+v, want an enabled Continue", with[0])
	}
	without := New(fakeShell{}).Buttons(1280, 720)
	if without[0].ID != ids.MenuNewGame || without[1].ID != ids.MenuContinue || !without[1].Disabled {
		t.Fatalf("buttons %+v, want New Colony then a disabled Continue", without[:2])
	}
}

func TestNewestSlotSkipsBrokenSaves(t *testing.T) {
	t.Parallel()
	now := time.Now()
	if NewestSlot(nil) != -1 {
		t.Fatal("no slots must give -1")
	}
	got := NewestSlot([]ports.SaveSlot{
		{Modified: now.Add(-time.Hour)},
		{Modified: now, Err: "corrupt"},
		{Modified: now.Add(-time.Minute)},
	})
	if got != 2 {
		t.Fatalf("newest = %d, want 2", got)
	}
}

func TestLoadSceneRetriesWhileNoLoader(t *testing.T) {
	t.Parallel()
	s := New(fakeShell{})
	s.LoadScene()
	if s.tried {
		t.Fatal("a missing loader must not count as a try")
	}
	if s.DrawDiorama(ebiten.NewImage(64, 36), 64, 36, 0) {
		t.Fatal("drew a diorama that was never loaded")
	}
}
