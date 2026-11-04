package dossier

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// fakeShell is the program around the card: it reads, acts, keeps the
// settings, is the screen and pointer, and is the kit's host.
type fakeShell struct {
	t        *testing.T
	pawn     game.PawnView
	animal   game.AnimalView
	card     dossierview.Card
	bio      game.BioView
	health   game.HealthView
	pris     game.PrisonerView
	areas    []game.AreaView
	policies []game.FoodPolicyView
	reads    int // Pawn + Animal calls
	hour     int
	area     int

	set         config.Settings
	cmds        []command.Command
	tabs        []string
	wins        []string
	hints       []string
	pickerTitle string
	pickerCur   int
	pickerBase  bool
	pick        func(int)
	setups      int

	mx, my int
	sw, sh int // the logical screen
	n      uint64
	f      inputframe.Frame
	kit    *uikit.Kit
	panel  *Panel
}

func newShell(t *testing.T) *fakeShell {
	t.Helper()
	s := &fakeShell{t: t, set: config.Defaults(), hour: 14, sw: 1280, sh: 720}
	s.set.HUD.Enabled = true
	s.pawn = game.PawnView{ID: 7, Name: "Bo", Health: 80, Mood: 60, Hunger: 40, Rest: 70, Recreation: 50,
		FoodPolicy: "Plain", Sex: "male", AgeYears: 31}
	s.card = dossierview.Card{ActiveSkill: -1, WorkType: -1, BreakAt: dossierview.BreakLevels{Minor: 20, Major: 10, Extreme: 5}}
	s.bio = game.BioView{Skills: []game.SkillView{{Key: "shooting", Label: "Shooting", Level: 8, LevelLabel: "8", Passion: 2}}}
	s.policies = []game.FoodPolicyView{{ID: 3, Name: "Plain"}, {ID: 4, Name: "Fancy"}}
	s.areas = []game.AreaView{{ID: 5, Name: "Yard", Color: [4]uint8{10, 200, 30, 255}}}
	s.kit = uikit.New(s)
	s.panel = New(s, s, s, s, s.kit, nil, s)
	s.panel.SetOnSetup(func() { s.setups++ })
	s.panel.Select(selection.Pawn, 7)
	return s
}

func (s *fakeShell) Pawn(id entity.EntityID) (game.PawnView, bool) {
	s.reads++
	return s.pawn, id == s.pawn.ID
}
func (s *fakeShell) Animal(id entity.EntityID) (game.AnimalView, bool) {
	s.reads++
	return s.animal, s.animal.ID != 0 && id == s.animal.ID
}
func (s *fakeShell) Card(entity.EntityID) (dossierview.Card, bool)      { return s.card, true }
func (s *fakeShell) Bio(entity.EntityID) (game.BioView, bool)           { return s.bio, true }
func (s *fakeShell) Health(entity.EntityID) (game.HealthView, bool)     { return s.health, true }
func (s *fakeShell) Prisoner(entity.EntityID) (game.PrisonerView, bool) { return s.pris, true }
func (s *fakeShell) Factions() []game.FactionView                       { return nil }
func (s *fakeShell) Areas() []game.AreaView                             { return s.areas }

func (s *fakeShell) Enqueue(c command.Command) { s.cmds = append(s.cmds, c) }
func (s *fakeShell) OpenTab(n string)          { s.tabs = append(s.tabs, n) }
func (s *fakeShell) ShowWindow(n string)       { s.wins = append(s.wins, n) }
func (s *fakeShell) AreaPicker(title string, cur int, base bool, pick func(int)) {
	s.pickerTitle, s.pickerCur, s.pickerBase, s.pick = title, cur, base, pick
}
func (s *fakeShell) Hint(h string) { s.hints = append(s.hints, h) }

func (s *fakeShell) Settings() config.Settings     { return s.set }
func (s *fakeShell) SetSettings(c config.Settings) { s.set = c }

func (s *fakeShell) Size() (int, int)      { return s.sw, s.sh }
func (s *fakeShell) Cursor() (int, int)    { return s.mx, s.my }
func (s *fakeShell) MapCursor() (int, int) { return s.mx, s.my }
func (s *fakeShell) MapScale() float64     { return 1 }

func (s *fakeShell) AreaOf(entity.EntityID) int          { return s.area }
func (s *fakeShell) FoodPolicies() []game.FoodPolicyView { return s.policies }
func (s *fakeShell) Hour() int                           { return s.hour }

func (s *fakeShell) Frame() *inputframe.Frame { return &s.f }
func (s *fakeShell) Updates() uint64          { return s.n }
func (s *fakeShell) KeyRepeat(string) bool    { return false }
func (s *fakeShell) Cue(string)               {}
func (s *fakeShell) ReducedMotion() bool      { return true }

// step runs one Update at the pointer: begin the kit, then declare the card.
func (s *fakeShell) step(mx, my int, press bool, r gfx.Rect, docked bool) {
	s.n++
	s.mx, s.my = mx, my
	raw := inputframe.Raw{Mx: mx, My: my}
	raw.Press[inputframe.Left], raw.Down[inputframe.Left] = press, press
	s.f = inputframe.NewFrame(raw)
	s.kit.Begin()
	if docked {
		s.panel.Declare(r)
	} else {
		s.panel.DeclareFloating(r)
	}
}

func (s *fakeShell) paint(r gfx.Rect, docked bool) {
	img := ebiten.NewImage(int(r.Right())+4, int(r.Bottom())+4)
	if docked {
		s.panel.Draw(img, r)
	} else {
		s.panel.DrawFloating(img, r)
	}
	s.kit.Paint(img)
}

// find returns the first laid-out item with the widget key.
func (s *fakeShell) find(docked bool, key string) *item {
	l := &s.panel.lay[modeIdx(docked)]
	for i := range l.items {
		if l.items[i].key == key {
			return &l.items[i]
		}
	}
	return nil
}

// at is the screen point in the middle of an item in a card at r.
func (s *fakeShell) at(it *item, r gfx.Rect, docked bool) (int, int) {
	b := body(r, docked)
	return int(b.X + it.x + it.w/2), int(b.Y + it.y + it.h/2)
}

func (s *fakeShell) setTree(docked bool, n config.HUDNode) {
	if docked {
		s.set.HUD.Layouts.Docked = &n
	} else {
		s.set.HUD.Layouts.Floating = &n
	}
	s.panel.Invalidate()
}

func column(kids ...config.HUDNode) config.HUDNode {
	return config.HUDNode{Kind: sheet.KindColumn, Kids: kids}
}
func widget(key string) config.HUDNode { return config.HUDNode{Kind: key} }
func generic(key, def string) config.HUDNode {
	return config.HUDNode{Kind: key, Def: def}
}

func rgbaOf(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xFF} }
