package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
)

// bookWith is core's backstories plus one adulthood per plugin key.
func bookWith(keys ...string) backstory.Book {
	core := backstory.Core()
	list, all := core.All(), core.Keys()
	for _, k := range keys {
		list = append(list, backstory.Def{Slot: backstory.Adulthood, Title: "Plugin " + k, Short: "P", Weight: 1})
		all = append(all, k)
	}
	return backstory.NewBook(list, all)
}

// A save keeps its plugin backstories by key (PLG-044): loaded where the
// plugin set moved them they keep their meaning; loaded where the plugin
// is gone they become none, never another backstory.
func TestSavedBackstoriesFollowTheirKeys(t *testing.T) {
	saved := bookWith("a.first", "b.second")
	g := New(Config{Width: 12, Height: 12, Content: Content{Backstories: saved}})
	second, _ := saved.IDOf("b.second")
	id, err := g.spawnGenerated(pawngen.Pawn{First: "Ida", Childhood: 1, Adulthood: second}, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "keys"); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	for _, tc := range []struct {
		name string
		book backstory.Book
		want string // the key the colonist's adulthood must read as ("" none)
	}{
		{"same set", saved, "b.second"},
		{"first plugin gone", bookWith("b.second"), "b.second"},
		{"reordered", bookWith("b.second", "a.first"), "b.second"},
		{"core only", backstory.Core(), ""},
	} {
		l, _, err := LoadContent(bytes.NewReader(data), Content{Backstories: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		p := l.sk.bios[id]
		if got := tc.book.Key(p.Adulthood); got != tc.want {
			t.Errorf("%s: adulthood reads %q (id %d), want %q", tc.name, got, p.Adulthood, tc.want)
		}
		if p.Childhood != 1 {
			t.Errorf("%s: core childhood moved to %d", tc.name, p.Childhood)
		}
	}
}
