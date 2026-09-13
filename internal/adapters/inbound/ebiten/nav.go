package ebiten

// screenID is a full-screen route. Presentation-only; it never exists in
// the domain.
type screenID int

const (
	screenBoot screenID = iota
	screenMainMenu
	screenScenario
	screenStoryteller
	screenWorldGen
	screenLanding
	screenPawns
	screenColony
	screenOptions
	screenLoad
	screenCredits
)

func (s screenID) String() string {
	switch s {
	case screenBoot:
		return "boot"
	case screenMainMenu:
		return "main_menu"
	case screenScenario:
		return "scenario"
	case screenStoryteller:
		return "storyteller"
	case screenWorldGen:
		return "world_gen"
	case screenLanding:
		return "landing"
	case screenPawns:
		return "pawns"
	case screenColony:
		return "colony"
	case screenOptions:
		return "options"
	case screenLoad:
		return "load"
	case screenCredits:
		return "credits"
	default:
		return "unknown"
	}
}

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
	screen  screenID
	overlay overlayID
}

type nav struct {
	stack []frame
}

func newNav() nav {
	return nav{stack: []frame{{screen: screenBoot}}}
}

func (n *nav) top() frame {
	return n.stack[len(n.stack)-1]
}

func (n *nav) screen() screenID { return n.top().screen }

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
	screen screenID
}

func (n *nav) escapeIntent() intent {
	t := n.top()
	if t.overlay != overlayNone {
		return intent{kind: intentResume}
	}
	if len(n.stack) > 1 {
		return intent{kind: intentBack}
	}
	if t.screen == screenColony {
		return intent{kind: intentPause}
	}
	if t.screen == screenBoot {
		return intent{kind: intentFinishBoot}
	}
	return intent{}
}

func wizardNext(s screenID) screenID {
	switch s {
	case screenScenario:
		return screenStoryteller
	case screenStoryteller:
		return screenWorldGen
	case screenWorldGen:
		return screenLanding
	case screenLanding:
		return screenPawns
	default:
		return s
	}
}
