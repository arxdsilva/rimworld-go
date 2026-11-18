# Modding gap analysis (PLG-*)

Scope: the plugin (mod) system. A player drops a folder into `mods/`, turns it on, and the game loads its definitions, patches, translations and art in a fixed order, says clearly what failed and why, and records the plugin set in each save. The reference is what the reference game's mod system lets players and mod authors do, rebuilt as a safe and intentional API (checklist Phases 37–38, `MASTER_SPEC` §48–§52, screens §45–§48).
- Load only for PLG-* tasks. Handoffs are sections of `.agents/handoffs/MOD.md` (`PLG-…`).
- Paths are under `internal/` unless they say otherwise. Find the function; line numbers drift.
- Checked against HEAD `d6226ad01` (2026-11-17).

## Decision record

| Item | Decision |
|---|---|
| Goal | The user asked on 2026-11-17 to "build the modding extension": gap-analyse the reference game's mod system against ours, decide how to build it, and make it work. This file is the record and the loop's backlog. |
| Clean room | Behaviour only: what a mod author and a player can do. No reference-game file format, tag name, schema, text or code is copied; the manifest, file layout, patch language and every message are ours. No numbers involved, so no `CLEAN_ROOM.md` exception. |
| Format | JSON through `encoding/json` (standard library): no new dependency, and localization is already JSON (`assets/lang/en.json`). Unknown fields are errors. YAML can come later as a second reader of the same trees. |
| Where data enters | Adapter `adapters/outbound/modloader` does the file work (discovery, safe paths through `os.Root`, size and count caps, JSON to a plain tree). Package `application/modding` does everything after: manifest checks, load order, inheritance, patches, typing, reference checks, provenance. It never touches a file and imports no third-party code (HEX §20). |
| Core uses the same system | Core definitions stay Go tables, but each moddable class has a codec (tree ⇄ `Def`). Core defs are encoded to trees, patched like a mod's, and decoded back, so a patch can target a core definition exactly as it targets a mod's. |
| Registries | No runtime-assigned package vars (`SOLID_GO` §9). A moddable class gets an immutable `Book` value (core plus plugins, frozen) built once in `compose` and passed through `game.Config`. The package-level `DefOf`/`All` of that class are removed in the same change, so no caller can miss plugin defs (a leftover would read a plugin ID as "none"). |
| IDs | Every definition has a namespaced key `<plugin>.<name>` (`core.` for ours). At runtime a class keeps its dense index: core first in table order, then plugins in load order, then file order. A save records each class's key per index (`saveState.ContentKeys`) and a load maps saved indexes onto the running content by key (PLG-044): a moved plugin definition keeps its meaning, a gone one becomes none (backstory) or drops its bill, and jobs on later bills follow their bill. The header's plugin list (SAV-025) still warns before loading a save made with other plugins. |
| First classes | Backstory (pure data, 11 call sites, no art, visible in bios and pawn generation), then recipes (bills; patches to core numbers are the most visible). Further classes follow the same codec pattern, one class per change. |
| Behaviour code | Native Go plugins are never loaded. Plugins add behaviour with sandboxed Starlark scripts (user decision 2026-11-18, `go.starlark.net`, PLG-060): hooks on the sim owner, read-only queries, typed commands, step budgets, deterministic `rand`. |
| Restart | Changing the enabled set takes effect on the next start (the registry is frozen at start). The Plugins screen says so. |
| Steam Workshop | Excluded: no store integration. Plugins are folders. |

## Sources

Behaviour of the reference game's mod system, described in our own words from playing and from its public modding wiki pages (no files read from the game).

| Topic | What we took |
|---|---|
| Mod folder | A mod is a folder with a metadata file (unique id, name, authors, description, supported game versions, homepage, dependencies, load-before/after hints, incompatibilities), data definitions, patches, translations, textures and sounds, and optionally compiled code. |
| Loading | The active list is ordered; the core content loads first; definitions from later mods can replace earlier ones with the same name; abstract parent definitions supply shared fields to children; patch operations change definitions without replacing files, and can depend on another mod being active. |
| Manager | A screen lists installed and active mods, toggles them, reorders them, warns about missing dependencies, wrong order and version mismatch, offers an automatic sort, and restarts the game after a change. |
| Saves | A save records the active mods; loading a save made with another set warns and lists the differences. |
| Errors | Bad definitions and failed patches are reported in a log with the mod named; the game keeps running with the bad entries skipped. |
| Info | An item's information card names the mod that added it. Mods with settings get a page in a mod-settings menu. |

## Rows

State legend: `open`, `done` (all criteria met, with commit), `[-]` partial (what is missing), `deferred` (decision pending), `excluded` (we will not do it, why).

### Manifest and discovery

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-001 | No plugin manifest | `plugin.json`: `id` (lowercase `[a-z0-9_]`, unique, not `core`), `name`, `authors`, `version`, `game` (supported versions, `major.minor`), `description`, `url`, `requires`, `after`, `before`, `conflicts`. Strict decoding, every error names file and field. | done (`ddeb8d3ee`) |
| PLG-002 | No discovery | `modloader` lists the folders under each plugin root (`<exe>/mods`, the user data dir `mods`), reads each manifest, sorts by id; enumeration order never matters. | done (`9c91fb22e`, `ca88a0512` folders) |
| PLG-003 | No enabled set | `settings.Plugins.Enabled []string`; a new install enables nothing; `-plugins a,b` overrides for a run. | done (`cd2c2991e`, `d32c2a1c1`, `2b2e74c24`): settings, the Plugins screen toggles and `-plugins a,b` (`''` for none) for one run, not saved |
| PLG-004 | Unsafe paths | All reads through `os.Root` per plugin; symlinks out of the root, absolute paths and `..` are errors; file size (4 MiB) and file count (2,000) caps. | done (`9c91fb22e`) |

### Load order and compatibility

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-010 | No load order | Topological order over `requires` and `after`/`before`; ties broken by id; core first. Every position has a reason (`required by x`, `after y`, `alphabetical`) for the order screen. | done (`ddeb8d3ee`) |
| PLG-011 | No dependency errors | Missing requirement, requirement disabled, cycle (names the loop), duplicate id, conflicting pair both enabled, unsupported game version (warning, still loads). A plugin with an error is not loaded; its dependants are not loaded either, each with its own message. | done (`ddeb8d3ee`; release tags checked since `e266b7a9f`) |

### Definitions

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-020 | No data definitions from plugins | `defs/*.json`: a file holds `{"<class>": [ {def}, … ]}`; keys are `<plugin>.<name>`. | done (`d36ba7967`) |
| PLG-021 | No core codec | Core defs of a moddable class encode to trees and decode back with no change (round-trip test per class). | done for backstory (`fb5e6b858`) and recipe (`555ad39ff`); each new class brings its codec |
| PLG-022 | No inheritance | `"abstract": true` defs are templates only; `"parent": "<key>"` copies the parent's fields under the child's (objects merge, lists and scalars replace); cycles and missing parents are errors. | done (`d36ba7967`) |
| PLG-023 | No same-key override | A later plugin defining an existing key is an error; replacing is a patch (`replace`), so every change is explicit and traceable. | done (`d36ba7967`) |
| PLG-024 | Backstory class | `backstory.Book`; plugin backstories appear in pawn generation and bios. | done (`fb5e6b858`, `e266b7a9f`) |
| PLG-025 | Recipe class | `recipe.Book`; plugin recipes appear on their bench's bill list; patches to core recipe numbers apply. | done (`555ad39ff`, `41810c86f`, `e0450f1be`): plugin recipes on their bench's bill picker, patches to plain fields of core recipes; behaviour flags (`by_species`, `legacy`, `dry`, `ferment`, `salvage`, `builds`, inputs' `smelt`, `smelt_wear`, `wreck`, `any_dryable`) refused on plugin recipes and unchangeable on core ones; plugin products are not smeltable yet (`SmeltYield` scans core) |
| PLG-026 | More classes | Trait, research project, incident, biome, faction, drug, building, item, species: each needs its registry turned into a `Book`. One per change. | [-]: traits, research projects, incidents and items are classes (2026-11-18). Traits: add and patch numbers; generated by rarity; code-keyed fields refused. Research: plugin projects gate plugin recipes; patches change cost/label/tech. Incidents: nature incidents that run a core incident's behaviour (`like`) with their own odds and label. Items: goods with a value, a storage category, their own PNG and a smelt yield, made and used by plugin recipes; mass and stack limit stay 1 kg/1000. Open: biomes, factions, drugs, buildings, species; threat incidents and incident letter text; item mass/stack limit, trade, shelves, info card; scenario parts can't force plugin traits or research |
| PLG-027 | Def extensions | `"ext": {"<plugin>": {…}}` on any def: free data kept per plugin for later behaviour hooks; never read by core. | deferred (needs PLG-060) |

### Patches

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-030 | No patches | `patches/*.json`: a list of operations, each `{"op", "target": "<class>/<key>", "path": "field.sub", "value"}`. Ops: `set` (replace a field), `add` (append to a list or add a map entry), `remove` (field, list item by value, or whole def), `merge` (object merge), `replace` (whole def). | done (`d36ba7967`, `77f4f39fb`) |
| PLG-031 | No conditions | `"if": "<id>"` / `"missing": "<id>"` runs the op only when that plugin is (not) loaded. | done (`d36ba7967`) |
| PLG-032 | No order or provenance | Patches apply after all plugins' defs are in, in load order then file order then list order. Every def records who defined it and who patched which field, in order. | done (`d36ba7967`) |
| PLG-033 | No conflict report | Two plugins `set`-ing the same field of the same def: the later wins and a warning names both. A target that does not exist is an error unless the op says `"optional": true`. | done (`77f4f39fb`) |

### Validation and diagnostics

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-040 | `contentlint` is empty | `cmd/contentlint [-mods dir] [-enable a,b]`: runs the whole pipeline headless, prints every problem with plugin, file, def and field, exit 1 on errors. | done (`ca88a0512`, `0aa422ddf`) |
| PLG-041 | No content error screen | Errors at start open the Content Errors window (§48): plugin, file, def, field, message; Copy, Disable plugin, Continue without it, Quit. | [-] (`d32c2a1c1`, `011fd4e68`): main-menu notice, Problems view with Show plugin and Disable; no Copy (the client has no clipboard), no recovery-mode or Quit actions |
| PLG-042 | No range/reference checks | Each codec checks its fields (ranges, references to other keys) and names the field path. | done for backstory and recipe: ranges, types and references (item, bench, skill, work type) with field paths |
| PLG-043 | Content hash ignores plugins | `contentid.Current(catalog)`: hash covers every frozen book; `Content.Plugins` is the load order with versions. | done (`ac91b0589`, `e266b7a9f`) |
| PLG-044 | Saves keep raw indexes | Save keys, map to indexes at load. | done (`8c253fe35` backstories, `41810c86f` recipes): `saveState.ContentKeys`, remap by key on load; a gone backstory becomes none, a gone recipe drops its bill |

### Saves

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-050 | Plugin set in saves is always `["core"]` | Real load order (`id@version`). The existing missing-plugin notice (`savefmt.MissingPlugins`) fires for real. | done (`e266b7a9f`): ids in load order; the existing missing-plugin notice now fires for real |
| PLG-051 | No plugin-owned save state | Namespaced versioned blobs. | deferred (needs PLG-060) |

### Localization and assets

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-055 | No plugin translations | `lang/<locale>.json` merged over the base table in load order; def labels resolve through keys `def.<class>.<key>.<field>`. | done: plugin `lang/<locale>.json` merged over the game table in load order (source-text keys, or `def.<class>.<key>.<field>`); bad entries are warnings; plugin-only locales are selectable; the Plugins detail lists a plugin's locales. Open: backstory descriptions are filled before translation, so their templates can't be translated yet |
| PLG-056 | No plugin art or sound | `textures/`, `sounds/` under namespaced paths, validated by the art pipeline. | open (after a class with art exists) |

### UI

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-070 | No Plugin manager | Main menu → Plugins: list (name, author, version, id, enabled, state), detail (description, requires, conflicts, url, why it sits where it does), enable/disable, "takes effect after restart". | [-] (`d32c2a1c1`, `0d70d8b61`, `011fd4e68`): list, detail, toggles, restart banner, empty state; no search, filter or capabilities |
| PLG-071 | No load-order view | Order column with its reason on each row (§46). Manual reorder is not offered: the order is computed. | done (`d32c2a1c1`): rows in load order with `#N`, detail says "Position N: <reason>" |
| PLG-072 | No source on info cards | Inspector and bio lines name the plugin that added or patched a def. | [-]: "Added by …" / "Changed by …" on backstories (bio), traits (bio chips), recipes (bill picker, bill title). Open: research tab, item inspector and incident letters have no line yet (their hosts carry no Content); plugin ids are shown, not names |
| PLG-073 | No plugin settings | Typed schema in the manifest, generated controls (§47). | deferred (no plugin has behaviour to configure yet) |

### Behaviour and scripting (Phase 38)

| ID | Gap | Ships | State |
|---|---|---|---|
| PLG-060 | No behaviour API | Event hooks, command submission, read-only queries in a sandbox (Starlark or WASM), per-hook budgets, deterministic RNG and tick. Needs a user decision on the runtime and its dependency. | [-]: Starlark hooks `on_hour`, `on_day`, `on_incident`, `on_colonist_died`; ctx queries (tick, day, hour, season, colonists, wealth, `count`, `rand`); commands `message`, `letter`, `spawn` (`ScriptMessage`, `ScriptSpawn`) through Enqueue; 200k steps and 16 commands a call; a failing script is turned off with a message and a Problems entry; contentlint checks scripts. Open: the turned-off set is not saved; the step limit does not cap one step's allocation; threats that are not incidents (animal packs, infestations) fire no `on_incident` |
| PLG-061 | Native code plugins | Never: no Go `plugin` loading, no runtime method patching. | excluded |
| PLG-062 | Workshop | No store integration. | excluded |

## Plan

### Areas and contract (lead, wave 0)

The lead writes alone, before any agent: package `application/modding` with `Manifest`, `Plugin`, `Problem{Plugin, File, Def, Field, Msg, Warn}`, `Tree` (`map[string]any`), `Op`, `Order`, `Catalog`, and stubs of `Resolve`, `Build`; the consumer-owned port `modding.Source` (lists plugins and their files); `backstory.Book` with `Core()`; `game.Config.Content`. Shared files edited only by the lead: `go.mod`, `Makefile`, `cmd/**`, `internal/compose/**`, `game/game.go`, `solid_budget.txt`, checklists, specs, README, handoffs.

### Parallel workers

RAM: `make agent-budget` before wave 1; wave 1 is at most 4 light agents.

Wave 1

| id | model, why | job | owns | forbidden | tests |
|---|---|---|---|---|---|
| W1-order | sonnet, pure graph code | manifest checks, load order, dependency problems (PLG-001, 010, 011) | `application/modding/order*.go`, `manifest*.go` | everything else | `go test -race ./internal/application/modding/ -run 'Order|Manifest'` |
| W1-load | sonnet, file I/O | discovery, safe reads, JSON to tree (PLG-002, 004) | `adapters/outbound/modloader/**` | everything else | `go test -race ./internal/adapters/outbound/modloader/` |
| W1-patch | opus, subtle merge semantics | inheritance, patches, provenance, conflicts (PLG-022, 030–033) | `application/modding/patch*.go`, `inherit*.go` | everything else | `go test -race ./internal/application/modding/ -run 'Patch|Inherit'` |
| W1-backstory | sonnet, mechanical migration | `backstory.Book` and codec, all 11 call sites (PLG-021, 024) | `domain/pawn/backstory/**`, `domain/pawn/pawngen/**`, the 4 `application/game` callers, `application/modding/class_backstory*.go` | `game.go`, other registries | `go test -race ./internal/domain/pawn/... ./internal/application/game/...` |

Wave 2 (lead): `Build` pipeline, compose wiring, `contentid`, `contentlint`, example plugin `mods/example_frontier`, Plugins and Content Errors windows (skill `rimworld-go-ui-screens`), checklist and ledger.

Wave 3: recipe class (PLG-025), translations (PLG-055), source lines (PLG-072).

Scope blocks are written in each agent's prompt from the rows above; the rows are the acceptance criteria.

Integration after each wave: `go build ./...`, `make arch`, `go test -race` of the touched packages, then the zero-plugin check: with no plugins the state hash, goldens and content fingerprint are unchanged. Commit per wave with explicit paths.

## Decisions made while building

| Item | Decision |
|---|---|
| Core defs are fixed in place | `Build` refuses a patch that removes or breaks a core def (problem names the plugin; the game's own def is kept), because saves keep backstories by index (`keepCore`, `ac91b0589`). |
| Patch conditions | `"if": "<id>"` and `"missing": "<id>"` are plugin-id strings on the op. |
| `optional` | Excuses a missing target and a missing field or item for `remove`; `set`/`add`/`merge` on a missing path are always errors. |
| Bad def or patch | Drops only itself; the plugin still loads. A manifest or dependency error stops the plugin and its dependants. |
| Recipe IDs | A recipe ID is a byte, so a run holds at most 256 recipes; plugin recipes are never a new bench's default bill; a plugin recipe's key is its `Name` (research unlocks say `recipe:<Name>`). |
| Carpet hint | The Architect's "sew one at…" hint reads `game.CoreRecipes()`: core carpets name core recipes. |
| Where the book lives | `game.Content` on the skill state (`backstoriesOf(g)`), set in `New`; `LoadContent(r, c)` loads a save with the run's books; `Load(r)` is core. |
| Captures | Load no plugin, so their frames never depend on what is installed. |

## Known leftovers (true at HEAD)

- `go test -race ./internal/application/game/` as one local binary at default parallelism exceeds its 10-minute timeout on a 12-core Mac both before the plugin work (`d6226ad01`: 600.6 s, 3849 s CPU, `TestQuestSoak` 5 min under contention, 22 s alone) and after (`2c220bf5c`: 600.7 s, 3802 s CPU): pre-existing, not a PLG regression; CI runs the binary alone through `scripts/ci_test.sh`.
- `.claude/scheduled_tasks.lock` (the loop's harness lock) trips `TestNoToolFolders` locally while a loop runs; it is untracked.
