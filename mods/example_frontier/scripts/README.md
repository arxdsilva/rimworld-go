# Plugin scripts (PLG-060)

A plugin adds behaviour with Starlark scripts: every `scripts/**.star` file
in its folder. Starlark is a small, deterministic dialect of Python. The
full reference is the doc comment of `internal/application/scripting`.
`frontier.star` here is a working example.

## Hooks

Define any of these as top-level functions. Every file's hooks run, files
in path order, plugins in load order.

| hook | when |
|---|---|
| `on_hour(ctx)` | each in-game hour, at its first tick |
| `on_day(ctx)` | each day at 00:00, before that hour's `on_hour` |
| `on_incident(ctx, incident)` | an incident fired: `incident.key` (`"core.herd_crossing"`), `incident.category` (`"nature"` or `"threat"`); a raid is `"core.rival_raid"`. Storyteller threats that are not incidents (wolf packs, manhunter packs, infestations, the threat pool's mad animal) don't call it yet. |
| `on_colonist_died(ctx, name)` | a colonist died |
| `on_pawns(ctx, pawns)` | cadence: every tick, the colonists due on it (a tuple of pawns, id ascending) |
| `on_pawn(ctx, pawn)` | cadence: every tick, once for each colonist due on it, by id |

The cadence hooks see each living home colonist once per 60 ticks: the
colonists are split into buckets and each tick has its bucket, so no tick
visits them all. A tick with no colonist due calls neither. On a tick the
order is `on_day`, `on_hour`, every plugin's `on_pawns`, then every
plugin's `on_pawn` for each due colonist. All of a file's `on_pawn` calls
on one tick share one budget (200000 steps, 16 commands, all or nothing:
a failure drops that tick's commands from the file).

## ctx

Read only: `ctx.tick`, `ctx.day` (the colony day, 0 on the landing day),
`ctx.hour` (0-23), `ctx.season` (`"spring"`, `"summer"`, `"fall"`,
`"winter"`), `ctx.colonists`, `ctx.wealth`, `ctx.count("core.wood")` (items
on the home map), `ctx.rand(n)` (0 to n-1; the same seed, tick, plugin and
call always give the same number), `ctx.pawns()` (every living, alive or
downed, colonist of the home map, a tuple of pawns, id ascending),
`ctx.pawn(id)` (the home colonist with that id, or `None`).

`ctx.colonists` counts the home map's colonists and those away in
caravans; `ctx.pawns()` lists the home map's only. A pawn (from
`ctx.pawns()`, `ctx.pawn(id)` or a cadence hook's argument) has `pawn.id`,
`pawn.name`, `pawn.x`, `pawn.y` (the cell), `pawn.food`, `pawn.rest`,
`pawn.mood` (each 0-100), `pawn.downed` and `pawn.drafted` (`True` or
`False`). `ctx.pawns()` reads every colonist each time it is called: inside
a cadence hook use the `pawns` or `pawn` argument instead.

Commands, applied on the next tick like a player's:
`ctx.message(text)`, `ctx.letter(title, text)`, `ctx.spawn(key, count)`
(items dropped at the colony's drop spot, 1-500).

`load("lib/util.star", "name")` loads another file of this plugin's
`scripts/` folder. Nothing else can be loaded, and scripts have no access
to time, files, the network or threads. `print` does nothing.

## Rules

- Scripts keep no state: globals are frozen after a file's first run, so
  any state lives in the game.
- There are no `while` loops and no recursion. Each call may take 200000
  steps and queue 16 commands, and a text may be up to 500 bytes.
- A script that fails, runs over a budget or breaks a rule is turned off
  for the rest of the run. The game posts a message and Plugins > Problems
  lists it with its file and line. The game keeps running.
- `contentlint` reports syntax errors with `file:line` before you play.
