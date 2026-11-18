package pawngen

import (
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
)

// A plugin backstory in the run's book is drawn like a core one, and the
// pawn's title and incapabilities read it through that book (PLG-024).
func TestGenerateDrawsPluginBackstories(t *testing.T) {
	core := backstory.Core()
	list := core.All()
	keys := make([]string, 0, len(list)+1)
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		keys = append(keys, core.Key(id))
	}
	list = append(list, backstory.Def{Slot: backstory.Adulthood, Title: "Ferry pilot", Short: "Pilot", Weight: 100000,
		Tags: backstory.TagNeolithic | backstory.TagIndustrial | backstory.TagSpacer, Skills: map[skill.ID]int8{skill.Social: 2}})
	keys = append(keys, "demo.ferry_pilot")
	book := backstory.NewBook(list, keys)
	plugin := backstory.ID(book.Len())

	hits := 0
	for seed := uint64(1); seed <= 20; seed++ {
		p := Generate(seed, 0, 0, Rules{Backstories: book})
		if p.Adulthood == plugin {
			hits++
			if got := p.Title(book); got != "Ferry pilot" {
				t.Fatalf("Title = %q", got)
			}
		}
		if c := Generate(seed, 0, 0, Rules{}); c.Adulthood == plugin {
			t.Fatal("the core book drew a plugin backstory")
		}
	}
	if hits < 15 {
		t.Fatalf("plugin backstory drawn %d/20 times despite its weight", hits)
	}
}
