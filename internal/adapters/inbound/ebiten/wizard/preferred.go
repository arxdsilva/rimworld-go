package wizard

import (
	"strings"

	"rimworld-go/internal/application/game"
)

// Preferred names (OPT-032): the names the player wants to see are given to
// the rolled candidates in roster order, skipping any a candidate or the
// colony already uses. Pure on the pool, so the same seed and the same list
// always give the same colonists, and the Setup carries the final names.

// preferredParts splits "Ana", "Ana Silva" or "Ana 'Ace' Silva" into first,
// nickname and last: a lone word is both first name and nickname.
func preferredParts(name string) (first, nick, last string) {
	name = strings.TrimSpace(name)
	if i := strings.IndexByte(name, '\''); i >= 0 {
		if j := strings.IndexByte(name[i+1:], '\''); j >= 0 {
			nick = strings.TrimSpace(name[i+1 : i+1+j])
			first = strings.TrimSpace(name[:i])
			last = strings.TrimSpace(name[i+2+j:])
			if nick != "" {
				return first, nick, last
			}
		}
	}
	f := strings.Fields(name)
	switch len(f) {
	case 0:
		return "", "", ""
	case 1:
		return f[0], f[0], ""
	}
	return f[0], f[0], strings.Join(f[1:], " ")
}

// applyPreferredNames renames candidates from the front of the roster, one
// name each. A name the pool refuses (a nickname already used, characters a
// name cannot hold) is skipped for the next one.
func applyPreferredNames(p candidatePool, names []string) {
	n, next := len(p.Views()), 0
	for i := 0; i < n && next < len(names); {
		first, nick, last := preferredParts(names[next])
		next++
		if nick == "" {
			continue
		}
		if p.Rename(i, first, nick, last) == nil {
			i++
		}
	}
}

// newPool is the candidate pool of a draft's seed and scenario with the
// preferred names applied.
func (w pawnsPage) newPool(ng *Draft) *game.CandidatePool {
	p := game.NewCandidatePool(SetupSeedOf(ng), ng.Scenario.Apply())
	applyPreferredNames(p, w.sh.Settings().General.PreferredNames)
	return p
}
