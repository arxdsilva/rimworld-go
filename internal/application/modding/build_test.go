package modding

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
)

func plugin(id string, defs, patches any) Plugin {
	p := Plugin{Manifest: Manifest{ID: id, Name: id, Version: "1", Game: []string{"0.1"}}}
	if defs != nil {
		p.Defs = []File{{Path: "defs/a.json", Data: defs}}
	}
	if patches != nil {
		p.Patches = []File{{Path: "patches/a.json", Data: patches}}
	}
	return p
}

func firstCoreKey() string { return backstory.Core().Key(1) }

// No plugin: the catalog is core, with no problem.
func TestBuildNoPluginIsCore(t *testing.T) {
	cat, probs := Build(nil, nil, "0.1.0")
	if len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	core := backstory.Core()
	if cat.Backstories.Len() != core.Len() || cat.Backstories.Key(1) != core.Key(1) {
		t.Fatalf("not core: %d defs", cat.Backstories.Len())
	}
	if ids := cat.IDs(); len(ids) != 1 || ids[0] != Core {
		t.Fatalf("IDs = %v", ids)
	}
}

// Removing or breaking a core def is refused: ids saves rely on stay put.
func TestBuildKeepsCoreDefs(t *testing.T) {
	key := firstCoreKey()
	ops := []any{
		map[string]any{"op": "remove", "target": "backstory/" + key},
		map[string]any{"op": "set", "target": "backstory/" + backstory.Core().Key(2), "path": "weight", "value": -5.0},
	}
	cat, probs := Build([]Plugin{plugin("vandal", nil, ops)}, []string{"vandal"}, "0.1.0")
	core := backstory.Core()
	if cat.Backstories.Len() != core.Len() {
		t.Fatalf("Len = %d, want %d", cat.Backstories.Len(), core.Len())
	}
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		if cat.Backstories.Key(id) != core.Key(id) || cat.Backstories.DefOf(id).Weight != core.DefOf(id).Weight {
			t.Fatalf("core id %d changed", id)
		}
	}
	var removed, broken bool
	for _, p := range probs {
		removed = removed || (p.Plugin == "vandal" && strings.Contains(p.Msg, "cannot be removed"))
		broken = broken || (p.Plugin == "vandal" && strings.Contains(p.Msg, "game's own is kept"))
	}
	if !removed || !broken {
		t.Fatalf("missing refusals: %v", probs)
	}
}

// An unknown class is reported and skipped; the plugin's good defs load.
func TestBuildUnknownClass(t *testing.T) {
	defs := map[string]any{
		"spaceship": []any{map[string]any{"name": "x"}},
		"backstory": []any{map[string]any{"name": "nomad", "slot": "adulthood", "title": "Nomad", "short": "Nomad", "weight": 3.0, "tags": []any{"neolithic"}}},
	}
	cat, probs := Build([]Plugin{plugin("demo", defs, nil)}, []string{"demo"}, "0.1.0")
	if cat.Backstories.Len() != backstory.Core().Len()+1 {
		t.Fatalf("Len = %d", cat.Backstories.Len())
	}
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "unknown definition class spaceship") {
		t.Fatalf("problems = %v", probs)
	}
}
