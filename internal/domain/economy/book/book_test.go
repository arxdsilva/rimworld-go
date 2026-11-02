package book

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
)

// TestBonus is the wiki's reading bonus curve: 1 book 0.8 %, 10 books
// 6 %, 20 books 7.5 %, 100 books 15.5 %, capped at 20 %.
func TestBonus(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]int{0: 0, 1: 8, 5: 40, 10: 60, 15: 70, 20: 75, 100: 155, 145: 200, 400: 200} {
		if got := Bonus(n); got != want {
			t.Errorf("Bonus(%d) = %d, want %d", n, got, want)
		}
	}
}

// TestTables are the wiki numbers by quality.
func TestTables(t *testing.T) {
	t.Parallel()
	if Power(item.Novel, quality.Legendary) != 2500 || Power(item.Novel, quality.Awful) != 1200 || Power(item.Textbook, quality.Good) != 1000 {
		t.Error("recreation power")
	}
	if SkillXP(quality.Normal, 1) != 250 || SkillXP(quality.Normal, 2) != 156 || Cap(quality.Masterwork) != 14 {
		t.Error("textbook")
	}
	if Points(quality.Excellent, 1) != 60 || Points(quality.Excellent, 2) != 37 {
		t.Error("schematic")
	}
	if Power(item.Wood, quality.Normal) != 0 {
		t.Error("wood is no book")
	}
}

// TestMint: deterministic, never a zero seed, subjects decoded, a
// schematic covers open projects, about a quarter teach two subjects,
// and every quality turns up.
func TestMint(t *testing.T) {
	t.Parallel()
	open := func(p research.Project) bool { return p != research.Herbalism }
	two, seen := 0, map[quality.Level]bool{}
	for h := uint64(0); h < 2000; h++ {
		for _, k := range []item.Kind{item.Novel, item.Textbook, item.Schematic} {
			q, s := Mint(k, Hash(42, h), open)
			q2, s2 := Mint(k, Hash(42, h), open)
			if s == 0 || q != q2 || s != s2 {
				t.Fatalf("%s %d: %v %d / %v %d", k, h, q, s, q2, s2)
			}
			seen[q] = true
			if Title(k, s) == "" {
				t.Fatalf("%s %d: no title", k, h)
			}
			switch k {
			case item.Novel:
				if Teaches(k, s) != "" {
					t.Fatal("a novel teaches")
				}
			case item.Textbook:
				sks := Skills(k, s)
				if len(sks) == 0 || len(sks) == 2 && sks[0] == sks[1] || sks[0] >= skill.NumSkills {
					t.Fatalf("skills %v", sks)
				}
				if len(sks) == 2 {
					two++
				}
			case item.Schematic:
				for _, p := range Projects(k, s) {
					if p == research.Herbalism {
						t.Fatal("a schematic covers a finished project")
					}
				}
			}
		}
	}
	if two < 400 || two > 600 {
		t.Errorf("two-skill textbooks %d of 2000, want about 500", two)
	}
	if len(seen) != 7 {
		t.Errorf("qualities seen %v", seen)
	}
	if q, s := Mint(item.Steel, 1, nil); q != 0 || s != 0 {
		t.Error("steel minted")
	}
}
