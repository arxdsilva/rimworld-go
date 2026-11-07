package game

// quest_save_test.go: quests and saves (QST-1). A v14 save has no quest
// settings; the v15 migration gives it rewards at 100 % and quest threats
// as its big threats.

import (
	"testing"

	"rimworld-go/internal/domain/difficulty"
)

func TestSaveV14GetsQuestDifficulty(t *testing.T) {
	t.Parallel()
	for _, p := range []difficulty.Preset{difficulty.Wanderer, difficulty.Grim} {
		g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
			s.MapSize, s.Preset, s.Difficulty = 60, p, difficulty.Of(p)
		}))
		stepUntilLanded(t, g)
		old := loadAsVersionWith(t, g, 14, func(s *saveState) {
			d := &s.NG1.Setup.Difficulty
			d.QuestRewardPct, d.QuestThreats = 0, false // a v14 save has neither
		})
		got, want := old.ng.setup.Difficulty, difficulty.Of(p)
		if got.QuestRewardPct != 100 || got.QuestThreats != want.BigThreats || got != want {
			t.Fatalf("%v: a v14 save migrates to %+v, want %+v", p, got, want)
		}
		if difficulty.Match(got) != p {
			t.Fatalf("%v: the migrated settings still match their preset", p)
		}
	}
}
