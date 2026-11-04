package dossier

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

type itemKind uint8

const (
	kText   itemKind = iota // one line of text
	kBar                    // label, bar, value
	kCell                   // a skill or training entry
	kPick                   // a selector
	kRule                   // a divider
	kMarker                 // an unknown element
)

// action is what a click on an item does besides opening a tab.
type action uint8

const (
	actNone action = iota
	actDiet
	actArea
	actHour
)

// tipKind says which tooltip an item has; the lines are built when the cursor
// rests on it.
type tipKind uint8

const (
	tipNone tipKind = iota
	tipPlain
	tipBio
	tipHealth
	tipMind
	tipFood
	tipRest
	tipFun
	tipTraits
	tipSkill
	tipTrain
	tipActivity
)

// item is one drawn element, positioned relative to the tree's origin. It is
// built at the refresh and only read by Declare and Draw, so a frame builds no
// strings and boxes no colour.
type item struct {
	kind itemKind
	key  string // the widget key
	id   string // the kit id; "" when nothing can be clicked or hovered
	x, y float32
	w, h float32

	text, value, plain string
	st, vst            gfx.TextStyle // label or text, and value
	ty                 float32       // the text's offset in the row

	bx, by, bw, bh float32 // the bar, relative to the item
	frac           float32
	fill, track    color.Color
	tick           color.Color
	ticks          [3]float32
	nticks         int

	back   color.Color // a selector's back
	passIc *ebiten.Image
	lblX   float32 // the label's offset (after a passion mark)

	tab string
	act action
	tip tipKind
	arg int // an index for the tip: skill or training
}
