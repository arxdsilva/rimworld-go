package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Sealed vaults (MAPGEN-3 §4.3): the sleeper caskets in a vault draw
// closed or open, the pane says which and offers one "Open casket"
// command (the game opens every casket of the vault), and the renderer cues
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

// casketCommand is the Open casket command of a casket: disabled with a
// reason once opened or when the game refuses. The click goes through
// applyBuildingClick (applyCasketClick).
func casketCommand(sealed bool, refusal string) cmdButton {
	c := cmdButton{ID: casketBtnID, Label: "Open casket", Icon: "open_casket", Desc: casketBtnTip}
	switch {
	case !sealed:
		c.Disabled, c.Reason = true, casketOpenMsg
	case refusal != "":
		c.Disabled, c.Reason = true, refusal
	}
	return c
}

// casketCommands are the commands of building b (none unless it is a
// casket).
func (a *App) casketCommands(b game.BuildingView) []cmdButton {
	return a.casketCommandsFor(b, casketSealed(b))
}

func (a *App) casketCommandsFor(b game.BuildingView, sealed bool) []cmdButton {
	if b.Kind != casketKind {
		return nil
	}
	refusal := ""
	if sealed {
		refusal = b.OpenReason // the game's refusal ("" when it would accept)
	}
	return []cmdButton{casketCommand(sealed, refusal)}
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
		a.shell.Enqueue(command.Command{Kind: command.OpenCasket, Target: uint64(b.ID)})
	}
	return true
}

// casketMemo remembers which caskets were sealed last frame.
type casketMemo struct {
	sealed map[entity.EntityID]bool
}

// CasketOpened returns the caskets of w that were sealed last frame and
// are open now, and remembers w's state. The first frame only records.
func (m *casketMemo) CasketOpened(w game.WorldView) []game.BuildingView {
	return m.Opened(w, casketSealed)
}

// Opened is casketOpened with the sealed test injected.
func (m *casketMemo) Opened(w game.WorldView, sealedOf func(game.BuildingView) bool) []game.BuildingView {
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
	for _, b := range r.caskets.CasketOpened(w) {
		r.cue(casketCueSFX, float64(b.X)+0.5, float64(b.Y)+1, 1)
	}
}
