package contentid

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
)

// With no plugin the run's content is the build's own: same hash, ["core"].
func TestOfCoreIsCurrent(t *testing.T) {
	got, want := Of([]string{Core}, backstory.Core()), Current()
	if got.Hash != want.Hash || !slices.Equal(got.Plugins, want.Plugins) || got.Hash != Fingerprint() {
		t.Fatalf("Of(core) = %+v, Current = %+v", got, want)
	}
}

// A plugin's backstory changes the fingerprint and the plugin list (PLG-043).
func TestOfCoversPluginDefs(t *testing.T) {
	core := backstory.Core()
	list, keys := core.All(), make([]string, 0, core.Len()+1)
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		keys = append(keys, core.Key(id))
	}
	list = append(list, backstory.Def{Slot: backstory.Adulthood, Title: "Ferry pilot", Short: "Pilot", Weight: 5})
	keys = append(keys, "demo.ferry_pilot")
	got := Of([]string{Core, "demo"}, backstory.NewBook(list, keys))
	if got.Hash == Fingerprint() {
		t.Fatal("a plugin backstory left the fingerprint unchanged")
	}
	if !slices.Equal(got.Plugins, []string{Core, "demo"}) {
		t.Fatalf("Plugins = %v", got.Plugins)
	}
}
