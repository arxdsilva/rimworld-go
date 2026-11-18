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
//	                              a raid is "core.rival_raid" (storyteller threats that are no incident are not reported yet)
//	on_colonist_died(ctx, name)   a colonist died
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

// Hook is a point of the tick a script can run at.
type Hook uint8

// The hooks, by the function a script defines.
const (
	OnHour Hook = iota + 1
	OnDay
	OnIncident
	OnColonistDied
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
	}
	return ""
}

// Hooks lists every hook.
func Hooks() []Hook { return []Hook{OnHour, OnDay, OnIncident, OnColonistDied} }

// Event is one hook point: everything a script sees of it besides World.
type Event struct {
	Hook Hook
	Tick uint64
	Seed uint64 // the game's seed: ctx.rand draws from (Seed, Tick, plugin, call)
	// Incident is OnIncident's incident.
	Incident Incident
	// Name is OnColonistDied's colonist.
	Name string
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
	Fire(ev Event, w World) Result
}
