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
//
// # Manifest: API version and capabilities
//
// plugin.json declares "api" (the plugin API version, apart from the game
// version; pluginapi.Version is 1) and "capabilities" (a list of names).
// A plugin with "api" above 1 or below 1, an unknown capability name or
// more than 32 capability entries is refused with a problem; a repeated
// name is only a warning. A plugin with neither field is a legacy plugin:
// it gets the capabilities query, commands and events, and a warning
// "declares no plugin API version; treated as 1". Every other power is
// denied until declared, and the Plugins screen shows the API and the
// capabilities in a plugin's detail.
//
// The capabilities and what each lets a script use:
//
//	query      ctx.tick, day, hour, season, colonists, wealth, count, rand, pawns(), pawn(id)
//	commands   ctx.message, ctx.letter, ctx.spawn
//	events     on_hour, on_day, on_incident, on_colonist_died, on_pawns, on_pawn, on_event
//	world      ctx.animals(), ctx.buildings(), ctx.map, ctx.world
//	defs       register_defs
//	validate   validate_defs, validate_command
//	ui         panel, on_action
//	incidents  run_incident
//	jobs       ctx.order (in on_pawns and on_pawn)
//	stats      stat_mod defs
//	health     a stat_mod conditioned on "injured" or a bio condition
//	recipes    the recipe_done event, and ctx.place in it
//	worldgen   the map_generated event, and ctx.place in it
//
// A hook function whose capability the plugin did not declare gets a
// warning and is never called. A ctx attribute used without its
// capability is an error, and it turns that script file off. The hooks
// panel, validate_command, validate_defs and register_defs are read
// only: a ctx call that queues a command inside them is an error.
// Capabilities are not enforced for Wasm scripts yet, and Wasm scripts
// get none of the extension points below.
//
// # Extension points
//
// Reads (capability world; each list holds at most MaxQueryRows rows and
// is the home map's even while the player views another map):
//
//	ctx.animals()    one row per wild species: species, label, count, predator
//	ctx.buildings()  one row per building kind: kind, count
//	ctx.map          width, height, weather, terrain(x, y), temperature(x, y)
//	ctx.world        factions() rows (name, goodwill, hostile), sites() rows (name, faction)
//
// Definitions and validation (load time, no ctx, no clock, no commands):
//
//	register_defs()            returns {class: [def, ...]}, built like a defs/*.json file
//	validate_defs(defs)        returns a list of {"def": key, "msg": text, "warn": bool}
//
// A def's "class" is a reserved word in Starlark, so a validate_defs
// script reads it as getattr(d, "class"). A registered def can be a
// stat_mod: "stat", integer "add" (-100000 to 100000) and "mul" (0 to
// 10000, permille), and the condition "trait", "condition" or "injured";
// it is the last layer of a colonist's stat sheet. Stat modifiers are
// rebuilt from the plugins on load, so they survive a save.
//
// Events: EVENTS = ["quadrum", "year", "research_done", "recipe_done",
// "map_generated"] and on_event(ctx, event) need the events capability;
// recipe_done also needs recipes and map_generated also needs worldgen.
// event.kind is the name; research_done has event.research (the project
// key); recipe_done has key, product, count, worker, x, y (the bench);
// map_generated has seed, width, height, biome. On the first tick of an
// hour the order is: on_day (hour 0), then the quadrum event on a
// quadrum's first day, then the year event on a year's first day, then
// on_hour, then the panels are refreshed. research_done and recipe_done
// come when the game finishes the project or craft (recipe_done for home
// map crafts only); map_generated comes once for a new game's home map,
// before its first tick. In recipe_done and map_generated,
// ctx.place(key, count, x, y) adds items (seed placements with ctx.rand).
//
// Behaviour:
//
//	validate_command(ctx, cmd)   before a player's build, add_bill, research,
//	                             prioritize or draft command (cmd.kind, pawn, x, y,
//	                             name): a non-empty string refuses it and is posted
//	                             as "Plugin <id>: <reason>"
//	run_incident(ctx, incident)  for the plugin's own incident defs: True replaces
//	                             the behaviour of the def's "like" incident
//	ctx.order(pawn_id, order, x, y)  in on_pawns and on_pawn: order is one of haul,
//	                             cut, mine, harvest, chop_wood, sleep, eat; the game
//	                             checks it like a player's prioritize
//
// A pawn takes a script order only on the cadence visit where
// (tick / PawnEvery) % OrderEvery == 0, so one order per pawn per 600
// ticks, and a second order for a pawn in the same call is refused with
// False; every pawn's order therefore lands in the same 60-tick window
// once per 600 ticks.
//
// UI (capability ui): panel(ctx) runs each in-game hour and returns
// {"title": text, "lines": [text, ...], "actions": [{"id": id, "label":
// text}, ...]} with at most MaxPanelLines lines and MaxActions actions
// (ids of 1-32 of a-z, 0-9 and _). The colony's Plugins tab shows the
// panels; pressing an action runs on_action(ctx, action_id) of that
// plugin, whose commands are applied like any other hook's.
//
// # Limits
//
// MaxSteps (200_000) steps and MaxCommands (16) commands per call, texts
// of at most MaxText (500) bytes, query lists of at most MaxQueryRows
// (256) rows, panels of at most MaxPanelLines (12) lines and MaxActions
// (4) actions, and OrderEvery (10) cadence visits between two orders of
// one pawn.
//
// # Saving
//
// A save holds no script state: registrations, stat modifiers and panels
// are rebuilt from the plugins on load. Panels are empty after a load
// until the next in-game hour.
package scripting
