package game

import (
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/scenario"
)

// Character creation (NG-1 §2.9, §5.4): the candidate pool exists before
// any Game. Owned by PAWN-G.

// CandidatePool is the Create Characters roster.
type CandidatePool struct {
	seed  uint64
	fx    scenario.Effects
	pawns []pawngen.Pawn
	rolls []int
	sel   int // the first sel candidates are Selected
}

// NewCandidatePool generates ChooseFrom candidates; the first People are
// Selected.
func NewCandidatePool(seed uint64, fx scenario.Effects) *CandidatePool {
	return &CandidatePool{seed: seed, fx: fx} // NG-1 stub (PAWN-G)
}

// Views, Reroll, RandomizeAll, Rename, Move, Swap, Team and Selected are
// the screen's operations.
func (p *CandidatePool) Views() []CandidateView                       { return nil }          // NG-1 stub (PAWN-G)
func (p *CandidatePool) Reroll(i int, keepName bool)                  {}                      // NG-1 stub (PAWN-G)
func (p *CandidatePool) RandomizeAll()                                {}                      // NG-1 stub (PAWN-G)
func (p *CandidatePool) Rename(i int, first, nick, last string) error { return nil }          // NG-1 stub (PAWN-G)
func (p *CandidatePool) Move(i, j int)                                {}                      // NG-1 stub (PAWN-G)
func (p *CandidatePool) Swap(i, j int)                                {}                      // NG-1 stub (PAWN-G)
func (p *CandidatePool) Team() TeamReport                             { return TeamReport{} } // NG-1 stub (PAWN-G)
func (p *CandidatePool) Selected() []pawngen.Pawn                     { return nil }          // NG-1 stub (PAWN-G)
