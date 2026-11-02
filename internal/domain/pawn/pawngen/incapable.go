package pawngen

import (
	"strings"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/trait"
)

// IncapableReasons lists each incapability with what causes it:
// "Violence (backstory: Convoy mechanic)", or several causes joined:
// "Violence (backstory: Convoy mechanic, trait: Gentle)". Bits go in
// backstory.Bits order; causes go childhood, adulthood, then traits.
func (p Pawn) IncapableReasons() []string {
	all := p.Incapable()
	if all == 0 {
		return nil
	}
	type cause struct {
		bits backstory.Bits
		text string
	}
	var causes []cause
	for _, id := range []backstory.ID{p.Childhood, p.Adulthood} {
		if d := backstory.DefOf(id); id != 0 && d.Incapable != 0 {
			causes = append(causes, cause{d.Incapable, "backstory: " + d.Title})
		}
	}
	for _, t := range p.Traits {
		if d := trait.DefOf(t); d.Incapable != 0 {
			causes = append(causes, cause{d.Incapable, "trait: " + d.Label})
		}
	}
	var out []string
	for b := backstory.Bits(1); b&backstory.AllBits != 0; b <<= 1 {
		if all&b == 0 {
			continue
		}
		var why []string
		for _, c := range causes {
			if c.bits&b != 0 {
				why = append(why, c.text)
			}
		}
		out = append(out, b.Labels()[0]+" ("+strings.Join(why, ", ")+")")
	}
	return out
}
