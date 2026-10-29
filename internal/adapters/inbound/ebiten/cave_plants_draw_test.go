package ebiten

import (
	"reflect"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestPlantLightLines(t *testing.T) {
	t.Parallel()
	p := game.PlantView{Species: "moss"}
	if got := env.PlantLightLines(p); len(got) != 0 {
		t.Errorf("plain plant: %v", got)
	}
	for _, c := range []struct {
		flag string
		set  func(*game.PlantView)
		want []string
	}{
		{"NeedsDark", func(q *game.PlantView) { q.NeedsDark = true }, []string{"Needs darkness"}},
		{"NeedsLight", func(q *game.PlantView) { q.NeedsLight = true }, []string{"Needs light to grow"}},
		{"Withering", func(q *game.PlantView) { q.Withering = true }, []string{"Withering in the light"}},
	} {
		q := p
		c.set(&q)
		if got := env.PlantLightLines(q); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.flag, got, c.want)
		}
		if got := inspectorpkg.PlantLines(q); !contains(got, c.want[0]) {
			t.Errorf("%s missing from the plant inspector: %v", c.want[0], got)
		}
	}
}

func TestDuskmossIsAGroundPlant(t *testing.T) {
	t.Parallel()
	g, ok := world.PlantKeys.Ground["duskmoss"]
	if !ok {
		t.Fatal("duskmoss needs ground plant keys")
	}
	if g.Young[0] != "plant/duskmoss_young_v0" || g.Mature[3] != "plant/duskmoss_mature_v3" || g.Dormant[1] != "plant/duskmoss_dormant_v1" {
		t.Errorf("keys = %v %v %v", g.Young[0], g.Mature[3], g.Dormant[1])
	}
}
