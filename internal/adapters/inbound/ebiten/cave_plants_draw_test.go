package ebiten

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestPlantLightLines(t *testing.T) {
	t.Parallel()
	p := game.PlantView{Species: "moss"}
	if got := plantLightLines(p); len(got) != 0 {
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
		if got := plantLightLines(q); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.flag, got, c.want)
		}
		if got := plantLines(q); !contains(got, c.want[0]) {
			t.Errorf("%s missing from the plant inspector: %v", c.want[0], got)
		}
	}
}

func TestDuskmossIsAGroundPlant(t *testing.T) {
	t.Parallel()
	g, ok := plantKeys.Ground["duskmoss"]
	if !ok {
		t.Fatal("duskmoss needs ground plant keys")
	}
	if g.Young[0] != "plant/duskmoss_young_v0" || g.Mature[3] != "plant/duskmoss_mature_v3" || g.Dormant[1] != "plant/duskmoss_dormant_v1" {
		t.Errorf("keys = %v %v %v", g.Young[0], g.Mature[3], g.Dormant[1])
	}
}
