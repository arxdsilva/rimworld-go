package animal

import (
	"reflect"
	"sort"
	"testing"
)

// TestPennedSpeciesExact pins the PLAY-015 farm livestock set: exactly these
// species are kept in pens; every other species uses allowed areas.
func TestPennedSpeciesExact(t *testing.T) {
	t.Parallel()
	want := []string{
		"alpaca", "bison", "capybara", "caribou", "cassowary", "chicken",
		"chinchilla", "cow", "deer", "donkey", "dromedary", "duck", "elk",
		"emberbuck", "emu", "gazelle", "goat", "goose", "guinea_pig", "horse",
		"ibex", "ostrich", "pig", "sheep", "timbergnaw", "turkey", "wild_boar",
		"woolhorn", "yak",
	}
	sort.Strings(want)

	var got []string
	for _, s := range All() {
		if RosterOf(s).Penned {
			got = append(got, RosterOf(s).Key)
		}
	}
	sort.Strings(got)

	if len(want) != 29 {
		t.Fatalf("expected list has %d keys, want 29", len(want))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("penned species = %v, want %v", got, want)
	}
}

// TestHiveAndPetNeverPenned keeps hive insects and pets out of pens, whatever
// their roster row says.
func TestHiveAndPetNeverPenned(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		r := RosterOf(s)
		if !r.Penned {
			continue
		}
		if r.Behaviour == Hive {
			t.Errorf("%s is Hive but penned", r.Key)
		}
		if r.Behaviour == Pet {
			t.Errorf("%s is Pet but penned", r.Key)
		}
	}
}
