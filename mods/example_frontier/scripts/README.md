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

## ctx

Read only: `ctx.tick`, `ctx.day` (the colony day, 0 on the landing day),
`ctx.hour` (0-23), `ctx.season` (`"spring"`, `"summer"`, `"fall"`,
`"winter"`), `ctx.colonists`, `ctx.wealth`, `ctx.count("core.wood")` (items
on the home map), `ctx.rand(n)` (0 to n-1; the same seed, tick, plugin and
call always give the same number).

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
