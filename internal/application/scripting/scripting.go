// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scripting

import "rimworld-go/internal/domain/sim/command"

// Budgets a script call is held to.
const (
	MaxSteps    = 200_000 // Starlark steps per call or per file's first run
	MaxCommands = 16      // commands one call may queue
	MaxText     = 500     // bytes in a message, a letter's title or its text
	MaxSpawn    = 500     // items one spawn may drop
)

// MaxWasmBytes is the one cap on a Wasm module's size (PLG-063): the
// loader (W1-loader) and wasmhost both check it. A module is compiled
// binary, so it is not held to the 256 KiB cap on Starlark source.
const MaxWasmBytes = 1 << 20

// PawnEvery is the ticks between two cadence visits (on_pawns, on_pawn)
// of one colonist.
const PawnEvery = 60

// Hook is a point of the tick a script can run at.
type Hook uint8

// The hooks, by the function a script defines.
const (
	OnHour Hook = iota + 1
	OnDay
	OnIncident
	OnColonistDied
	OnPawns // cadence: the due colonists, one call per file
	OnPawn  // cadence: one call per due colonist
)

// hookNames are the hooks' function names, by value.
var hookNames = [...]string{
	OnHour: "on_hour", OnDay: "on_day", OnIncident: "on_incident",
	OnColonistDied: "on_colonist_died", OnPawns: "on_pawns", OnPawn: "on_pawn",
}

// String is the hook's function name.
func (h Hook) String() string {
	if int(h) >= len(hookNames) {
		return ""
	}
	return hookNames[h]
}

// Hooks lists every hook.
func Hooks() []Hook { return []Hook{OnHour, OnDay, OnIncident, OnColonistDied, OnPawns, OnPawn} }

// Event is one hook point: everything a script sees of it besides World.
type Event struct {
	Hook Hook
	Tick uint64
	Seed uint64 // the game's seed: ctx.rand draws from (Seed, Tick, plugin, call)
	// Incident is OnIncident's incident.
	Incident Incident
	// Name is OnColonistDied's colonist.
	Name string
	// Pawns are OnPawns' and OnPawn's due colonists, id ascending. The
	// host does not keep the slice after Fire.
	Pawns []Pawn
	// Kind is an Extensions.Event's event (0 for a Hook's Fire).
	Kind EventKind
	// Plugin and Action name a ScriptAction's plugin and panel action
	// (PanelSource.Act), or the plugin a call is for.
	Plugin, Action string
	// Research is ResearchDone's project key.
	Research string
	// Recipe is RecipeDone's craft.
	Recipe Craft
	// Map is MapGenerated's map.
	Map MapGen
}

// Pawn is a colonist as a script sees it.
type Pawn struct {
	// ID is the colonist's entity id, stable for the run.
	ID uint64
	// Name is the colonist's name.
	Name string
	// X is the column of the cell the colonist stands on.
	X int
	// Y is the row of the cell the colonist stands on.
	Y int
	// Food is the HUD's food bar, 0-100.
	Food int
	// Rest is the HUD's rest bar, 0-100.
	Rest int
	// Mood is the colonist bar's smoothed mood, 0-100.
	Mood int
	// Downed is true when the colonist is down.
	Downed bool
	// Drafted is true when the colonist is drafted.
	Drafted bool
}

// Incident is an incident as a script sees it.
type Incident struct {
	Key      string // namespaced key, "core.herd_crossing"
	Category string // "nature" or "threat"
}

// World is what a script may read of the colony. The game answers for
// its home map; the host asks only during Fire.
type World interface {
	Day() int // the colony day, 0 on the landing day
	Hour() int
	Season() string
	Colonists() int
	Wealth() int
	// ItemCount is how many of item key lie on the home map; ok is false
	// for a key the run's content does not know.
	ItemCount(key string) (n int, ok bool)
}

// PawnReader is how a script reads the colonists of the home map. The
// game answers during Fire only.
type PawnReader interface {
	// Pawns is every living (alive or downed) colonist of the home map,
	// id ascending. The slice is the reader's: the caller must not keep
	// or change it.
	Pawns() []Pawn
	// Pawn is the colonist id; ok is false for any id that is not a
	// living colonist of the home map.
	Pawn(id uint64) (Pawn, bool)
}

// Problem is a script disabled for the rest of the run.
type Problem struct {
	Plugin string
	File   string // slash path inside the plugin ("scripts/main.star")
	Line   int    // 0 when unknown
	Msg    string
}

// Result is what one Fire produced.
type Result struct {
	// Commands are the scripts' commands, in order, for Enqueue.
	Commands []command.Command
	// Disabled are the scripts this call disabled.
	Disabled []Problem
}

// Host runs the enabled plugins' scripts. The simulation owner calls it,
// never concurrently, and the same calls give the same results.
type Host interface {
	// Fire runs every enabled script's hook for ev, plugins in load order.
	// p reads the colonists; nil means there are none to read
	// (ctx.pawns() is empty and ctx.pawn(id) is None).
	Fire(ev Event, w World, p PawnReader) Result
	// Uses reports that a live (not disabled) script defines hook h. The
	// game asks every tick, so it is cheap and does not allocate.
	Uses(h Hook) bool
}
