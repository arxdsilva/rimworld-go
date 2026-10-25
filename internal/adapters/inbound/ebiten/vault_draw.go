package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Sealed vaults (MAPGEN-3 §4.3): the sleeper caskets in a vault draw
// closed or open, the inspector says which and offers one "Open casket"
// button (the game opens every casket of the vault), and the renderer cues
// sfx.casket_open when a sealed casket turns open.

const (
	casketKind   = "sleeper casket"
	casketClosed = "structure/sleeper_casket_closed"
	casketOpen   = "structure/sleeper_casket_open"
	casketCueSFX = "sfx.casket_open"
)

// casketSealed reports a sleeper casket that has not been opened.
func casketSealed(b game.BuildingView) bool { return b.Kind == casketKind && b.Sealed }

// casketSprite is the world sprite of casket b by state.
func casketSprite(b game.BuildingView) string { return casketSpriteFor(casketSealed(b)) }

func casketSpriteFor(sealed bool) string {
	if sealed {
		return casketClosed
	}
	return casketOpen
}

// casketTitle is the inspector title ("" for other buildings).
func casketTitle(b game.BuildingView) string {
	if b.Kind != casketKind {
		return ""
	}
	return casketTitleFor(casketSealed(b))
}

func casketTitleFor(sealed bool) string {
	if sealed {
		return "Sleeper casket (sealed)"
	}
	return "Sleeper casket (open)"
}

const (
	casketBtnID   = "bldg.casket.open"
	casketBtnTip  = "Opens every casket in this vault"
	casketOpenMsg = "This casket is already open."
)

// casketButton is the Open casket button of a casket in panel r: disabled
// with a reason once opened or when the game refuses.
func casketButton(r button, sealed bool, refusal string) button {
	b := button{id: casketBtnID, label: "Open casket", x: r.x + 40, y: r.y + 70, w: 150, h: 26, tip: casketBtnTip}
	switch {
	case !sealed:
		b.disabled, b.reason, b.quiet = true, casketOpenMsg, true
	case refusal != "":
		b.disabled, b.reason, b.quiet = true, refusal, true
	}
	return b
}

// casketButtons are the inspector buttons of building b (none unless it
// is a casket).
func (a *App) casketButtons(b game.BuildingView, r button) []button {
	return a.casketButtonsFor(b, r, casketSealed(b))
}

func (a *App) casketButtonsFor(b game.BuildingView, r button, sealed bool) []button {
	if b.Kind != casketKind {
		return nil
	}
	refusal := ""
	if sealed {
		refusal = b.OpenReason // the game's refusal ("" when it would accept)
	}
	return []button{casketButton(r, sealed, refusal)}
}

// applyCasketClick sends OpenCasket for the Open casket button.
func (a *App) applyCasketClick(id string, b game.BuildingView) bool {
	return a.applyCasketClickFor(id, b, casketSealed(b))
}

func (a *App) applyCasketClickFor(id string, b game.BuildingView, sealed bool) bool {
	if id != casketBtnID || b.Kind != casketKind {
		return false
	}
	if sealed {
		a.renderer.enqueue(command.Command{Kind: command.OpenCasket, Target: uint64(b.ID)})
	}
	return true
}

// casketMemo remembers which caskets were sealed last frame.
type casketMemo struct {
	sealed map[entity.EntityID]bool
}

// casketOpened returns the caskets of w that were sealed last frame and
// are open now, and remembers w's state. The first frame only records.
func (m *casketMemo) casketOpened(w game.WorldView) []game.BuildingView {
	return m.opened(w, casketSealed)
}

// opened is casketOpened with the sealed test injected.
func (m *casketMemo) opened(w game.WorldView, sealedOf func(game.BuildingView) bool) []game.BuildingView {
	next := map[entity.EntityID]bool{}
	var out []game.BuildingView
	for _, b := range w.Buildings {
		if b.Kind != casketKind {
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

// listenCaskets cues a casket's opening (hook of listenWorld).
func (r *Renderer) listenCaskets(w game.WorldView) {
	for _, b := range r.caskets.casketOpened(w) {
		r.cue(casketCueSFX, float64(b.X)+0.5, float64(b.Y)+1, 1)
	}
}
