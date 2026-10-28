package ebiten

import "rimworld-go/internal/adapters/inbound/ebiten/inputframe"

// The input frame lives in inputframe (ARCH-003). These aliases and
// wrappers keep the root's call sites unchanged while its features move
// out one package at a time; the last mover deletes this file.

type (
	inputFrame  = inputframe.Frame
	rawInput    = inputframe.Raw
	inputSource = inputframe.Source
	ebitenInput = inputframe.EbitenSource
	mouseBtn    = inputframe.MouseBtn
	uiPoint     = inputframe.UIPoint
)

const (
	mbLeft   = inputframe.Left
	mbMiddle = inputframe.Middle
	mbRight  = inputframe.Right
	numMouse = inputframe.NumMouse
)

func newFrame(raw rawInput) inputFrame { return inputframe.NewFrame(raw) }
func repeatFire(n int) bool            { return inputframe.RepeatFire(n) }
