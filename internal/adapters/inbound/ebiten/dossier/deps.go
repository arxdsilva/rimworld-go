package dossier

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// Reader is what the card reads. A host in the composition root looks Pawn and
// Animal up in the snapshot and forwards the rest: selection-scoped, at the
// refresh rate, so there is no tick scan.
type Reader interface {
	Pawn(id entity.EntityID) (game.PawnView, bool)
	Animal(id entity.EntityID) (game.AnimalView, bool)
	Card(id entity.EntityID) (dossierview.Card, bool)
	Bio(id entity.EntityID) (game.BioView, bool)
	Health(id entity.EntityID) (game.HealthView, bool)
	Prisoner(id entity.EntityID) (game.PrisonerView, bool)
	Factions() []game.FactionView
	Areas() []game.AreaView
}

// Actions is what the card does. It carries the language catalog (ports.Texts)
// the card and its setup modal write their words with.
type Actions interface {
	ports.Texts
	Enqueue(command.Command)
	OpenTab(name string)
	ShowWindow(tab string)
	AreaPicker(title string, cur int, base bool, pick func(ref int))
	Hint(string)
}

// Prefs reads and stores the player's settings.
type Prefs interface {
	Settings() config.Settings
	SetSettings(config.Settings)
}

// PresetStore keeps the player's card presets; cardpresetfile fills it.
type PresetStore interface {
	List() ([]string, error)
	Load(name string) ([]byte, error)
	Save(name string, data []byte) error
	Delete(name string) error
}

// FolderOpener shows a folder in the system's file browser.
type FolderOpener func(path string) error
