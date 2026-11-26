// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sheet

import (
	"reflect"
	"sort"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestCatalogCoversEveryKey(t *testing.T) {
	t.Parallel()
	keys := WidgetKeys()
	var inCatalog []string
	for _, e := range Catalog(en) {
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
		if e.NeedsDef && len(Defs(en, e.Key)) == 0 {
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
	for _, e := range Catalog(en) {
		if !groups[e.Group] {
			t.Errorf("%s: group %s is not in Groups()", e.Key, e.Group)
		}
	}
}

func TestDefinitions(t *testing.T) {
	t.Parallel()
	if len(Defs(en, "skill")) != int(skill.NumSkills) || Defs(en, "skill")[0].Key != "shooting" {
		t.Fatal("skills")
	}
	wantTraining := []Def{{"obedience", "Obedience"}, {"guard", "Guard"}, {"release", "Release"}, {"haul", "Haul"}}
	if got := Defs(en, "train"); len(got) != int(training.NumSkills) || !reflect.DeepEqual(got, wantTraining) {
		t.Fatalf("training definitions = %+v, want %+v", got, wantTraining)
	}
	if _, ok := TrainByKey(""); ok {
		t.Fatal("empty training key must not resolve")
	}
	if len(Defs(en, "tally")) != 2 || len(Defs(en, "need_bar")) != 3 || Defs(en, "doing") != nil {
		t.Fatal("tally and need defs")
	}
	seen := map[string]bool{}
	for _, d := range Defs(en, "stat") {
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
	for _, d := range Defs(en, "train") {
		sk, ok := TrainByKey(d.Key)
		if !ok || TrainLabel(en, sk) != d.Label || TrainKey(sk) != d.Key {
			t.Errorf("train %s", d.Key)
		}
		if TrainDesc(en, sk) == "" {
			t.Errorf("train %s has no description", d.Key)
		}
	}
	if _, ok := LookupDef(en, "skill", "melee"); !ok {
		t.Fatal("lookup")
	}
	if _, ok := LookupDef(en, "skill", "work_speed"); ok {
		t.Fatal("a stat is not a skill")
	}
}

// A translated label must never change the layout's saved stable definition key.
type trainingTranslation struct{}

func (trainingTranslation) T(src string) string {
	if src == "Haul" {
		return "Transporter"
	}
	return src
}

func TestHaulTrainingDefinitionAndCell(t *testing.T) {
	t.Parallel()
	tr := trainingTranslation{}
	def, ok := LookupDef(tr, "train", "haul")
	if !ok || def != (Def{"haul", "Transporter"}) {
		t.Fatalf("translated definition: %+v, %v", def, ok)
	}
	sk, ok := TrainByKey(def.Key)
	if !ok || sk != training.Haul {
		t.Fatalf("key resolved to %v, %v", sk, ok)
	}
	if _, ok := TrainByKey(def.Label); ok {
		t.Fatal("translated label became a persistent key")
	}
	a := game.AnimalView{}
	v := game.AnimalTrainingView{Skill: int(sk), Label: "Haul", Allowed: true, On: true, Steps: 2, Needed: int(training.Needed(sk))}
	c := TrainCell(tr, a, v)
	if c.Label != "Transporter" || c.Value != "2 of 7" || c.Pct != 28 || c.Tip == "" {
		t.Fatalf("haul training cell: %+v", c)
	}
	v.Allowed = false
	if c := TrainCell(tr, a, v); c != (Cell{}) {
		t.Fatalf("untrainable species shows haul: %+v", c)
	}
}
