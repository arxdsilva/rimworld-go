package game

import "testing"

func TestSoilsFarmability(t *testing.T) {
	byName := map[string]SoilView{}
	for _, s := range Soils() {
		if s.Label == "" {
			t.Errorf("soil %q has no label", s.Name)
		}
		byName[s.Name] = s
	}
	for _, n := range []string{"soil", "rich_soil", "gravel"} {
		if !byName[n].Farmable {
			t.Errorf("%s should be farmable", n)
		}
	}
	for _, n := range []string{"stone", "rock"} {
		if byName[n].Farmable {
			t.Errorf("%s should not be farmable", n)
		}
	}
	if byName["rich_soil"].Fertility <= byName["soil"].Fertility || byName["gravel"].Fertility >= byName["soil"].Fertility {
		t.Errorf("fertility order wrong: %+v", byName)
	}
}
