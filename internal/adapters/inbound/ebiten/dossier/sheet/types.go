// Package sheet is the colonist card's pure logic (RHUD): layout trees, targets,
// the widget catalog, built-in presets, the text and tone of every line, colour
// maths. It imports no ebiten, so all of it is testable headless.
package sheet

import "rimworld-go/internal/platform/config"

// Who is which kind of target a layout node shows for.
type Who uint8

const (
	WhoColonist Who = iota // c
	WhoStranger            // s, another humanlike that is not hostile
	WhoPrisoner            // p
	WhoWild                // w, creatures and mechs
)

// Tone is what a line's colour means. Every tone sits on a line whose words
// state the same fact (spec §59).
type Tone uint8

const (
	ToneText Tone = iota
	ToneOff
	ToneCritical
	ToneWarning
	ToneNeutral
	ToneGood
	ToneTop
)

// Line is one built text line: Tab is the inspector tab a click opens.
type Line struct {
	Text string
	Tone Tone
	Tip  string
	Tab  string
}

// Preset is a stored card arrangement.
type Preset struct {
	Version                                int
	Name                                   string
	Docked, Floating                       *config.HUDNode
	DockW, DockH, DockTabs, FloatW, FloatH int
	Text                                   *config.HUDText
}

// SaveFlags are the parts of the current look a preset saves.
type SaveFlags struct{ Docked, Floating, Width, Height, Tabs, Text bool }

// Problem is one thing wrong with a stored tree.
type Problem struct {
	Path string
	Text string
}

// Metrics measures nodes: the sizes of the current text style.
type Metrics struct {
	BodyH, LargeH, SmallH, BarH, GapH float32
	LabelW, ValueW                    float32
}

// Cell is one skill or training entry: its parts and tone.
type Cell struct {
	Label, Value string
	Tone         Tone
	Pct          int // the bar's fill, 0..100
	Tip          string
	Active       bool
}

// Widget keys. A key names a value widget in a layout tree; containers are
// KindColumn, KindStrip, KindBlock and KindLine.
const (
	KindColumn = "column"
	KindStrip  = "strip"
	KindBlock  = "block"
	KindLine   = "line"
)

// WidgetKeys lists every value widget's key.
func WidgetKeys() []string {
	return []string{"gap", "rule", "title", "diet_pick", "clothes_pick", "area_pick", "hour_pick",
		"standing", "identity", "vigour_bar", "body_state", "mood_bar", "mind_state", "hunger_bar",
		"rest_bar", "fun_bar", "need_bar", "skill", "train", "stat", "tally", "keeper", "doing",
		"next_up", "in_hand", "hauling", "traits", "notes", "captive"}
}
