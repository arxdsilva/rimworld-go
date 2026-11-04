package sheet

import (
	"reflect"
	"sort"
	"testing"

	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestCatalogCoversEveryKey(t *testing.T) {
	t.Parallel()
	keys := WidgetKeys()
	var inCatalog []string
	for _, e := range Catalog() {
		if e.Tip == "" || e.Label == "" || e.Group == "" {
			t.Errorf("%s: missing text", e.Key)
		}
		if !e.Container {
			inCatalog = append(inCatalog, e.Key)
			if !IsWidget(e.Key) {
				t.Errorf("%s is in the catalog but not a widget", e.Key)
			}
		} else if !IsContainer(e.Key) {
			t.Errorf("%s flagged as a container", e.Key)
		}
		if got, ok := Lookup(e.Key); !ok || got.Key != e.Key {
			t.Errorf("lookup %s", e.Key)
		}
		if e.NeedsDef != NeedsDef(e.Key) || e.Bar != IsBar(e.Key) {
			t.Errorf("%s: flags disagree with the helpers", e.Key)
		}
		if e.NeedsDef && len(Defs(e.Key)) == 0 {
			t.Errorf("%s needs a definition and has none", e.Key)
		}
	}
	sort.Strings(keys)
	sort.Strings(inCatalog)
	if !reflect.DeepEqual(keys, inCatalog) {
		t.Fatalf("WidgetKeys and Catalog differ:\n%v\n%v", keys, inCatalog)
	}
	for _, k := range keys {
		if !IsWidget(k) {
			t.Errorf("IsWidget(%s)", k)
		}
	}
	if IsWidget("column") || IsWidget("") {
		t.Error("containers and empties are not widgets")
	}
	if e, _ := Lookup("clothes_pick"); !e.Reserved {
		t.Error("clothes_pick is reserved")
	}
	groups := map[string]bool{}
	for _, g := range Groups() {
		groups[g] = true
	}
	for _, e := range Catalog() {
		if !groups[e.Group] {
			t.Errorf("%s: group %s is not in Groups()", e.Key, e.Group)
		}
	}
}

func TestDefinitions(t *testing.T) {
	t.Parallel()
	if len(Defs("skill")) != int(skill.NumSkills) || Defs("skill")[0].Key != "shooting" {
		t.Fatal("skills")
	}
	if len(Defs("train")) != 3 {
		t.Fatal("training: obedience, guard, release")
	}
	if len(Defs("tally")) != 2 || len(Defs("need_bar")) != 3 || Defs("doing") != nil {
		t.Fatal("tally and need defs")
	}
	seen := map[string]bool{}
	for _, d := range Defs("stat") {
		if seen[d.Key] {
			t.Errorf("duplicate stat key %s", d.Key)
		}
		seen[d.Key] = true
		id, ok := StatByKey(d.Key)
		if !ok || StatKey(id) != d.Key || id.Label() != d.Label {
			t.Errorf("stat %s", d.Key)
		}
	}
	for _, furniture := range []stat.ID{stat.Comfort, stat.RestEffectiveness, stat.Beauty} {
		if _, ok := StatByKey(StatKey(furniture)); ok {
			t.Errorf("%s is furniture, not a card stat", furniture.Label())
		}
	}
	if !seen["work_speed"] || !seen["shooting_accuracy"] {
		t.Fatal("expected stat keys")
	}
	for _, d := range Defs("train") {
		sk, ok := TrainByKey(d.Key)
		if !ok || TrainLabel(sk) != d.Label || TrainKey(sk) != d.Key {
			t.Errorf("train %s", d.Key)
		}
		if TrainDesc(sk) == "" {
			t.Errorf("train %s has no description", d.Key)
		}
	}
	if _, ok := LookupDef("skill", "melee"); !ok {
		t.Fatal("lookup")
	}
	if _, ok := LookupDef("skill", "work_speed"); ok {
		t.Fatal("a stat is not a skill")
	}
}
