// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package scripting is the port plugins' scripts run behind (PLG-060): the
// simulation owner calls a Host at fixed points of the tick and gets typed
// commands back, which it queues through Enqueue like a player's. The
// runtime (Starlark) lives in an outbound adapter
// (adapters/outbound/starlarkhost).
//
// # Writing a script
//
// A plugin's scripts are the files scripts/**.star in its folder, in the
// Starlark language (a small, deterministic dialect of Python). Every file
// is a module run once at start; its globals are then frozen, so a script
// keeps no state between calls: state lives in the game. Hooks are
// top-level functions a file defines; every file's hooks run, files in
// path order, plugins in load order:
//
//	on_hour(ctx)                  each in-game hour, at its first tick
//	on_day(ctx)                   each in-game day at 00:00, before that hour's on_hour
//	on_incident(ctx, incident)    an incident fired: incident.key ("core.herd_crossing"), incident.category ("nature", "threat");
//	                              in a setup colony a storyteller threat reports once, category "threat", under its own
//	                              key (packs, manhunters, hives, mad animals, mech raids, ship parts); a people raid is
//	                              "core.rival_raid"; a legacy colony (no setup) reports only its people raids and ship parts
//	on_colonist_died(ctx, name)   a colonist died
//	on_pawns(ctx, pawns)          cadence: every tick, the colonists due on it (a tuple of pawns, id ascending)
//	on_pawn(ctx, pawn)            cadence: every tick, once for each colonist due on it, by id
//
// The cadence hooks see each living home colonist once per PawnEvery (60)
// ticks: the colonists are split into buckets and each tick has its bucket
// of due colonists, so no tick visits them all. On a tick the hooks run in
// this order: on_day, on_hour, every plugin's on_pawns, then every
// plugin's on_pawn for each due colonist, plugins in load order and files
// in path order. A tick with no colonist due calls neither.
//
// ctx reads the colony (read only):
//
//	ctx.tick        the simulation tick
//	ctx.day         the colony day (0 on the landing day)
//	ctx.hour        the hour of the day, 0-23
//	ctx.season      "spring", "summer", "fall" or "winter" (the map's own calendar)
//	ctx.colonists   living colonists
//	ctx.wealth      the colony's wealth
//	ctx.count(key)  how many of item key ("core.wood") lie on the home map
//	ctx.rand(n)     a number in [0, n): the same seed, tick, plugin and call give the same number
//	ctx.pawns()     every living (alive or downed) colonist of the home map, a tuple of pawns, id ascending
//	ctx.pawn(id)    the home colonist with that id, or None
//
// ctx.colonists counts the home map's colonists and those away in
// caravans; ctx.pawns() lists the home map's only. A pawn (from ctx.pawns,
// ctx.pawn or a cadence hook's argument) has these fields:
//
//	pawn.id        the colonist's id, stable for the run
//	pawn.name      the colonist's name
//	pawn.x, pawn.y the cell the colonist stands on
//	pawn.food      0-100, the HUD's food bar
//	pawn.rest      0-100, the HUD's rest bar
//	pawn.mood      0-100, the colonist bar's smoothed mood
//	pawn.downed    True when the colonist is down
//	pawn.drafted   True when the colonist is drafted
//
// ctx.pawns() reads every colonist each time it is called: inside a
// cadence hook, which runs on every tick, use the pawns or pawn argument
// instead, which hold only the due colonists.
//
// and acts only by queueing commands, applied on the next tick:
//
//	ctx.message(text)             a message to the player
//	ctx.letter(title, text)       a letter in the letter stack
//	ctx.spawn(key, count)         count items of key dropped at the colony's drop spot (1-MaxSpawn)
//
// load("other.star", "name") loads another file of the same plugin's
// scripts folder (paths relative to the scripts folder); nothing else can
// be loaded. There is no time, file, network or thread access; print does
// nothing.
//
// # Budgets
//
// A call (and a file's first run) may take MaxSteps Starlark steps and
// queue MaxCommands commands; texts are at most MaxText bytes. A script
// that fails, runs over or breaks a rule is disabled for the rest of the
// run: the Plugins screen lists the problem and the game posts a message.
//
// on_pawns is one call per file with the budgets of any other hook. A
// file's on_pawn calls on one tick share one budget: MaxSteps steps and
// MaxCommands commands across all the due colonists' calls. The shared
// budget is all or nothing: a call that fails or runs over disables the
// file and drops every command its on_pawn calls queued on that tick.
// A broken script never stops the game.
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

// String is the hook's function name.
func (h Hook) String() string {
	switch h {
	case OnHour:
		return "on_hour"
	case OnDay:
		return "on_day"
	case OnIncident:
		return "on_incident"
	case OnColonistDied:
		return "on_colonist_died"
	case OnPawns:
		return "on_pawns"
	case OnPawn:
		return "on_pawn"
	}
	return ""
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
