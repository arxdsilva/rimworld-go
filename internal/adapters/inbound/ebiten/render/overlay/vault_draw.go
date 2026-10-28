package overlay

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Sealed vaults (MAPGEN-3 §4.3): the sleeper caskets in a vault draw
// closed or open, the pane says which and offers one "Open casket"
// command (the game opens every casket of the vault), and the renderer cues
// sfx.casket_open when a sealed casket turns open.

const (
	CasketKind   = "sleeper casket"
	casketClosed = "structure/sleeper_casket_closed"
	CasketOpen   = "structure/sleeper_casket_open"
	casketCueSFX = "sfx.casket_open"
)

// CasketSealed reports a sleeper casket that has not been opened.
func CasketSealed(b game.BuildingView) bool { return b.Kind == CasketKind && b.Sealed }

// CasketSprite is the world sprite of casket b by state.
func CasketSprite(b game.BuildingView) string { return CasketSpriteFor(CasketSealed(b)) }

// CasketSpriteFor is the casket sprite by sealed state.
func CasketSpriteFor(sealed bool) string {
	if sealed {
		return casketClosed
	}
	return CasketOpen
}

// CasketTitle is the inspector title ("" for other buildings).
func CasketTitle(b game.BuildingView) string {
	if b.Kind != CasketKind {
		return ""
	}
	return CasketTitleFor(CasketSealed(b))
}

// CasketTitleFor is the casket inspector title by sealed state.
func CasketTitleFor(sealed bool) string {
	if sealed {
		return "Sleeper casket (sealed)"
	}
	return "Sleeper casket (open)"
}

// CasketMemo remembers which caskets were sealed last frame.
type CasketMemo struct {
	sealed map[entity.EntityID]bool
}

// CasketOpened returns the caskets of w that were sealed last frame and
// are open now, and remembers w's state. The first frame only records.
func (m *CasketMemo) CasketOpened(w game.WorldView) []game.BuildingView {
	return m.Opened(w, CasketSealed)
}

// Opened is casketOpened with the sealed test injected.
func (m *CasketMemo) Opened(w game.WorldView, sealedOf func(game.BuildingView) bool) []game.BuildingView {
	next := map[entity.EntityID]bool{}
	var out []game.BuildingView
	for _, b := range w.Buildings {
		if b.Kind != CasketKind {
			continue
		}
		s := sealedOf(b)
		next[b.ID] = s
		if m.sealed[b.ID] && !s {
			out = append(out, b)
		}
	}
	m.sealed = next
	return out
}

// ListenCaskets cues a casket's opening (hook of listenWorld).
func (o *Set) ListenCaskets(w game.WorldView) {
	for _, b := range o.Caskets.CasketOpened(w) {
		o.Host.Cue(casketCueSFX, float64(b.X)+0.5, float64(b.Y)+1, 1)
	}
}
