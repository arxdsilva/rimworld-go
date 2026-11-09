package faction

import (
	"slices"
	"testing"

	"rimworld-go/pkg/randx"
)

// TestFactionEnemiesDeterministic (EAI-015): the same world makes the same
// wars, generating them draws nothing from the stream, wars are mutual,
// sorted and only between human peoples, and running the rule again (an
// older save) changes nothing.
func TestFactionEnemiesDeterministic(t *testing.T) {
	t.Parallel()
	slots := []Slot{{Settlers, 4}, {Clans, 4}, {Drifters, 3}, {Freebooters, 3}, {Swarm, 1}}
	r1, r2 := randx.New(42), randx.New(42)
	a, b := Generate(slots, r1), Generate(slots, r2)
	if r1.IntN(1<<30) != r2.IntN(1<<30) {
		t.Fatal("the two generators drifted apart")
	}
	wars := 0
	for i := range a {
		if !slices.Equal(a[i].Enemies, b[i].Enemies) {
			t.Fatalf("faction %d: enemies %v and %v from the same seed", i, a[i].Enemies, b[i].Enemies)
		}
		if !slices.IsSorted(a[i].Enemies) {
			t.Fatalf("faction %d: enemies %v not ascending", i, a[i].Enemies)
		}
		for _, e := range a[i].Enemies {
			wars++
			if e == a[i].ID || !a[e].Type.Human() || !a[i].Type.Human() || !a[e].AtWar(a[i].ID) {
				t.Fatalf("faction %d at war with %d: not a mutual war between two human peoples", i, e)
			}
		}
	}
	if wars == 0 || wars == len(a)*(len(a)-1) {
		t.Fatalf("%d war ends among %d factions: the rule should make some wars, not all", wars, len(a))
	}
	// Generation draws only what it drew before the wars existed.
	plain := randx.New(42)
	_ = Generate(slots, plain)
	again := randx.New(42)
	fs := Generate(slots, again)
	if plain.IntN(1<<30) != again.IntN(1<<30) {
		t.Fatal("SetEnemies moved the stream")
	}
	before := make([][]int, len(fs))
	for i := range fs {
		before[i] = slices.Clone(fs[i].Enemies)
		fs[i].Enemies = nil // an older save: no wars yet
	}
	SetEnemies(fs)
	for i := range fs {
		if !slices.Equal(fs[i].Enemies, before[i]) {
			t.Fatalf("faction %d: the migration gives %v, generation gave %v", i, fs[i].Enemies, before[i])
		}
	}
	other := Generate(slots, randx.New(43))
	same := true
	for i := range other {
		same = same && slices.Equal(other[i].Enemies, a[i].Enemies)
	}
	if same {
		t.Log("seeds 42 and 43 made the same wars (allowed, but unlikely)")
	}
}

// TestEnemyGoodwillCooldown (EAI-015): beating a raid of faction X earns
// the goodwill of every standing, non-hostile faction at war with X, once,
// and not again from the same faction within EnemyCooldownDays.
func TestEnemyGoodwillCooldown(t *testing.T) {
	t.Parallel()
	fs := []Faction{
		{ID: 0, Type: Settlers, Goodwill: 0, Enemies: []int{2}},
		{ID: 1, Type: Clans, Goodwill: 0},
		{ID: 2, Type: Drifters, Goodwill: -60, Relation: Hostile, Enemies: []int{0, 3}},
		{ID: 3, Type: Clans, Goodwill: -70, Relation: Hostile, Enemies: []int{2}},
	}
	got := EnemyBeaten(fs, 2, 5, "Day 6: an enemy's raid beaten")
	if !slices.Equal(got, []int{0}) || fs[0].Goodwill != EnemyBonus || fs[1].Goodwill != 0 || fs[3].Goodwill != -70 {
		t.Fatalf("thanked %v, goodwill %d/%d/%d: want only the settlers, +%d (clans not at war, the hostile clans never thank)",
			got, fs[0].Goodwill, fs[1].Goodwill, fs[3].Goodwill, EnemyBonus)
	}
	for day := 5; day < 5+EnemyCooldownDays; day++ {
		if got := EnemyBeaten(fs, 2, day, "again"); len(got) != 0 || fs[0].Goodwill != EnemyBonus {
			t.Fatalf("day %d: thanked again within the cooldown (%v, goodwill %d)", day, got, fs[0].Goodwill)
		}
	}
	if got := EnemyBeaten(fs, 2, 5+EnemyCooldownDays, "later"); !slices.Equal(got, []int{0}) || fs[0].Goodwill != 2*EnemyBonus {
		t.Fatalf("after the cooldown: thanked %v, goodwill %d, want +%d again", got, fs[0].Goodwill, EnemyBonus)
	}
	if EnemyBeaten(fs, 1, 30, "not an enemy") != nil {
		t.Fatal("beating a faction nobody is at war with earns nothing")
	}
}

func TestRivalIsAnEnemy(t *testing.T) {
	t.Parallel()
	fs := []Faction{
		{ID: 0, Type: Settlers, Enemies: []int{1, 2, 3}},
		{ID: 1, Type: Clans, Relation: Hostile, Enemies: []int{0}},
		{ID: 2, Type: Drifters, Relation: Hostile, Defeated: true, Enemies: []int{0}},
		{ID: 3, Type: Freebooters, Relation: Hostile, Enemies: []int{0}},
		{ID: 4, Type: Settlers, Relation: Neutral, Enemies: []int{0}},
	}
	fs[0].Enemies = append(fs[0].Enemies, 4)
	seen := map[int]bool{}
	for n := 0; n < 2; n++ {
		seen[Rival(fs, 0, func(int) int { return n })] = true
	}
	if !seen[1] || !seen[3] || len(seen) != 2 {
		t.Fatalf("rivals %v, want the clans and the freebooters (the defeated drifters and the neutral settlers never come)", seen)
	}
	if Rival(fs, 4, nil) != -1 {
		t.Fatal("a rival not hostile to the colony raided it")
	}
	fs[0].Relation = Hostile
	if Rival(fs, 1, nil) != 0 || Rival(fs, 9, nil) != -1 || Rival([]Faction{{ID: 0, Type: Clans}}, 0, nil) != -1 {
		t.Fatal("a single enemy answers without a roll; none or a bad index gives -1")
	}
	if got := EnemyNames(fs, fs[0]); !slices.Equal(got, []string{"", "", "", ""}) || len(EnemyNames(fs, fs[1])) != 1 {
		t.Fatalf("enemy names %q", got)
	}
}
