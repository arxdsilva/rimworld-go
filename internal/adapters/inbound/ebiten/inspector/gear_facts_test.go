package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

func TestQualifiedTitle(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ title, stuff, q, want string }{
		{"Longsword", "steel", "good", "Steel longsword (good)"},
		{"Steel ×20", "steel", "", "Steel ×20"},
		{"Rifle", "", "", "Rifle"},
	} {
		if got := QualifiedTitle(c.title, c.stuff, c.q); got != c.want {
			t.Fatalf("%q != %q", got, c.want)
		}
	}
}

func TestItemPaneShowsQualityAndCondition(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.world = game.WorldView{Items: []game.ItemView{{ID: 7, Kind: "club", Qty: 1, FreshDays: -1,
		Quality: "good", Stuff: "steel", HP: 20, MaxHP: 100, Tainted: true}}}
	spec := New(f).Thing().ItemPane(7)
	if spec.Title != "Steel club (good)" {
		t.Fatalf("title %q", spec.Title)
	}
	got := strings.Join(spec.Lines, "|")
	if !strings.Contains(got, "Hit points 20 / 100 (20%)") || !strings.Contains(got, "Worn by a dead pawn") {
		t.Fatal(got)
	}
	f.world.Items[0].MaxHP, f.world.Items[0].Tainted = 0, false
	if strings.Contains(strings.Join(New(f).Thing().ItemPane(7).Lines, "|"), "Hit points") {
		t.Fatal("no condition without MaxHP")
	}
}

func TestGearStatLines(t *testing.T) {
	t.Parallel()
	p := game.PawnView{
		WeaponStats: weapon.View{Name: "Rifle", Damage: 12, ArmorPen: 350, Warmup: 90, Cooldown: 120, Burst: 3, Range: 25,
			Accuracy: [4]int{900, 800, 600, 400}, Blast: "fire", BlastRadius: 15},
		Apparel:      []string{"flak vest", "tuque"},
		Worn:         []game.WornView{{Kind: "flak vest", Stuff: "steel", Quality: "good", Tattered: true, Tainted: true}, {Kind: "tuque"}},
		ApparelArmor: []armor.Set{{Sharp: 400, Blunt: 300, Heat: 50}, {}},
	}
	got := strings.Join(GearStatLines(p), "|")
	for _, w := range []string{"Damage 12   Armor penetration 35%", "Warmup 1.5s   Cooldown 2.0s   Burst 3   Range 25",
		"touch 90%  short 80%  medium 60%  long 40%", "Blast: fire, radius 1.5",
		"Steel flak vest (good) [tattered, worn by a dead pawn]: sharp 40%  blunt 30%  heat 5%", "Tuque: sharp 0%"} {
		if !strings.Contains(got, w) {
			t.Fatalf("%q lacks %q", got, w)
		}
	}
	melee := WeaponLines(game.PawnView{WeaponStats: weapon.View{Name: "Club", Melee: true, Cooldown: 90}})
	if len(melee) != 2 || strings.Contains(strings.Join(melee, "|"), "Accuracy") {
		t.Fatal(melee)
	}
	if len(GearStatLines(game.PawnView{})) != 0 {
		t.Fatal("empty")
	}
}

func TestInspirationInStatus(t *testing.T) {
	t.Parallel()
	if InspirationLine("") != "" || InspirationLine("inspired creativity") != "Inspired: creativity" {
		t.Fatal("line")
	}
	f := newFake()
	f.pawns = []game.PawnView{{ID: entity.EntityID(1), Status: "alive", Inspiration: "inspired creativity"}}
	lines := New(f).Pawn().PawnLines(f.pawns[0])
	if !strings.Contains(strings.Join(lines, "|"), "Inspired: creativity") {
		t.Fatal(lines)
	}
}
