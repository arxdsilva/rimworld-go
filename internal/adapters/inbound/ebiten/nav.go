package ebiten

import "rimworld-go/internal/adapters/inbound/ebiten/ports"

// overlayID is a modal layer on top of the current full screen.
type overlayID int

const (
	overlayNone overlayID = iota
	overlayPause
)

func (o overlayID) String() string {
	if o == overlayPause {
		return "pause"
	}
	return "none"
}

type frame struct {
	screen  ports.Screen
	overlay overlayID
}

type nav struct {
	stack []frame
}

func newNav() nav {
	return nav{stack: []frame{{screen: ports.ScreenBoot}}}
}

func (n *nav) top() frame {
	return n.stack[len(n.stack)-1]
}

func (n *nav) screen() ports.Screen { return n.top().screen }

func (n *nav) overlay() overlayID { return n.top().overlay }

func (n *nav) reset(f frame) {
	n.stack = []frame{f}
}

func (n *nav) push(f frame) {
	n.stack = append(n.stack, f)
}

func (n *nav) pop() bool {
	if len(n.stack) <= 1 {
		return false
	}
	n.stack = n.stack[:len(n.stack)-1]
	return true
}

func (n *nav) setOverlay(o overlayID) {
	n.stack[len(n.stack)-1].overlay = o
}

type intentKind int

const (
	intentNone intentKind = iota
	intentPush
	intentBack
	intentStartGame
	intentQuit
	intentReturnToMenu
	intentResume
	intentPause
	intentFinishBoot
)

type intent struct {
	kind   intentKind
	screen ports.Screen
}

func (n *nav) escapeIntent() intent {
	t := n.top()
	if t.overlay != overlayNone {
		return intent{kind: intentResume}
	}
	if len(n.stack) > 1 {
		return intent{kind: intentBack}
	}
	if t.screen == ports.ScreenColony {
		return intent{kind: intentPause}
	}
	if t.screen == ports.ScreenBoot {
		return intent{kind: intentFinishBoot}
	}
	return intent{}
}

func wizardNext(s ports.Screen) ports.Screen {
	switch s {
	case ports.ScreenScenario:
		return ports.ScreenStoryteller
	case ports.ScreenStoryteller:
		return ports.ScreenWorldGen
	case ports.ScreenWorldGen:
		return ports.ScreenLanding
	case ports.ScreenLanding:
		return ports.ScreenPawns
	default:
		return s
	}
}
