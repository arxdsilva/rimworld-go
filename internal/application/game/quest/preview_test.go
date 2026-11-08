package quest

import (
	"testing"

	"rimworld-go/internal/domain/pawn/skill"
)

// TestPreviewIsTheSpawnedPawn: a pawn reward's preview is the person the
// seed spawns: the same look, and its best skill is the highest level, the
// first on a tie (QST-087, QST-137).
func TestPreviewIsTheSpawnedPawn(t *testing.T) {
	for seed := uint64(1); seed < 40; seed++ {
		p, v := PawnOf(seed), PreviewOf(seed)
		if v.Look != p.Look {
			t.Fatalf("seed %d: preview look %+v, spawned %+v", seed, v.Look, p.Look)
		}
		if v.BestLevel != int(p.Skills[v.Best].Level) {
			t.Fatalf("seed %d: best level %d, skill %v is at %d", seed, v.BestLevel, v.Best, p.Skills[v.Best].Level)
		}
		for s := range skill.NumSkills {
			if l := int(p.Skills[s].Level); l > v.BestLevel || l == v.BestLevel && s < v.Best {
				t.Fatalf("seed %d: skill %v at %d beats the preview's best %v at %d", seed, s, l, v.Best, v.BestLevel)
			}
		}
	}
}
