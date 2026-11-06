# Task Handoff

Task ID: UI-016
Status: complete (see Not done)

## Summary

One input layer for the client. Keys are named actions in a pure registry (`internal/platform/input`): each has a context, a kind and default keys, and conflicts are checked. Each `App.Update` polls the devices once. The frame is resolved through a context stack (text > modal > tool > panel > map > global > dev), so one key or click does one thing. All 35 raw Ebitengine reads are gone from the handlers.

Bindings can be changed on Options › Controls and are saved in `settings.json`. A Developer mode toggle is on Options › Gameplay.

Features built on the layer:
- multi-selection: Shift+click, drag box, double-click, a group panel;
- R, F and right-click act on the whole selection;
- formation moves;
- `,` and `.` cycle colonists; Numpad * cycles the things in a cell;
- tab keys F1–F6 (reserved ones toast);
- Home/End change speed; PgUp/PgDn zoom; middle-drag pans; Shift pans faster;
- Z search;
- F10 screenshot, F11 hide UI;
- G room-stats overlay; hold Alt for cell detail;
- Architect draw styles, with Ctrl to square;
- bill quantity modifiers;
- gizmo keys B H Y N J U M K I L O P;
- developer keys and panels.

## Files Changed

- `internal/platform/input/` (new):
  - `input.go`: types, `Conflicts`, `ConflictsWith`, `Set`/`Swap`, `FromSettings`/`ToSettings`/`Normalize`, `Resolve`;
  - `defs.go`: the action table;
  - tests.
- `internal/platform/config/settings.go`:
  - `Settings.Controls{Bindings}` holds overrides only and is normalized;
  - adds `Gameplay.DevMode`.
- `internal/adapters/inbound/ebiten/`, new files:
  - `input_source.go`: the only Ebitengine input reads;
  - `input_frame.go`: frame, stack, `keyHint`, text routing, Enter;
  - `hotkeys.go`, `multiselect.go`, `formation.go`, `drawstyle.go`, `search_ui.go`, `screenshot.go`, `toast.go`, `room_stats.go`, `gizmo.go`, `devmode.go`, `controls_ui.go`;
  - `input_ui_test.go`.
- `internal/adapters/inbound/ebiten/`, edited files:
  - `app.go`: rewritten `Update` and the longer Esc chain;
  - `renderer.go`: speed and camera intents, `speedUltra`, `stepTicks`;
  - `camera_smooth.go`: pan boost;
  - `architect.go`, `move_mode.go`, `orders.go`, `order_queue_ui.go`, `log_ui.go`, `schedule_ui.go`, `screen_newgame.go`, `hud.go`, `inspector.go`, `building_inspector.go`, `screen_options.go`, `settings.go`, `selection.go`, `widgets.go` (`button.tip`), `capture.go` (new capture states), `AGENTS.md`.
- `cmd/game/main.go`: the `-capture-ui` help text.
- `README.md`: a Controls table; the roof overlay is now Shift+R.
- Checklists and specs: UI §1.4, §2.1, §2.2, §3.2, §5, §13.1, §16.1, §16.7, §34.4, §38, §40.3; vanilla §34.6; MAIN_UI §42 Controls and Gameplay, §56 input and camera.

## Default bindings

| Group | Keys |
|---|---|
| Camera | WASD / arrows pan (Shift faster); PgDn / `=` / wheel up zoom in; PgUp / `-` / wheel down zoom out; middle-drag pan; edge scroll (option) |
| Time | Space pause; 1 / 2 / 3 speeds; Home faster; End slower |
| Tabs | Tab Architect; F1 Work; F2 Schedule; F3 Assign (reserved); F4 Animals; F5 Wildlife; F6 Research; F7 Quests, F8 World, F9 History (reserved) |
| View | F10 screenshot; F11 hide UI |
| Dialogs | Esc cancel/back/menu; Enter accept (wizard Next/Start/Generate, search) |
| Selection | `,` / `.` previous/next colonist (centres); Numpad * next thing in the cell; Shift+click add/remove (fixed) |
| Selected | R draft/undraft; F forbid/allow; V power (reserved) |
| Commands | B H Y N J U M K I L O P: the inspector's command buttons in display order (corner badge) |
| Architect | C Cancel tool; X Deconstruct tool; Q / E rotate, or change the shape of a drag tool; Ctrl square/circle (fixed) |
| Overlays | Shift+R roof; G room stats; T beauty (reserved); Alt (hold) cell detail (fixed); Z search |
| Orders | Shift queue (fixed); bill +/-: Ctrl ×10, Shift ×100, Ctrl+Shift ×1000 |
| Dev | 4 ultrafast 15×; Numpad + one tick while paused; `/` debug actions; `` ` `` debug log; `'` debug inspector |

## Public Contracts Changed

- `config.Settings` gained `Controls` and `Gameplay.DevMode`. Old files decode over the defaults.
- `Settings` is no longer comparable with `==` because it now holds a map. The config tests use `reflect.DeepEqual`.
- The new package `internal/platform/input` holds the action ids, which are the stored keys. Never rename an id.

## Invariants / Decisions

- Presentation only:
  - no command, step or RNG changed;
  - the dev step adds `stepTicks` single `Step(1)` calls while paused;
  - ultrafast is a speed tier of 15 ticks per frame.
- Debug actions call `game.Game` hooks through the consumer-owned `devTools` interface, and only in developer mode. This is the one deliberate exception to "commands only": they are debug tools.
- Modifier actions are fixed and not rebindable: fast pan, add to selection, queue, constrain, detail and quantity. They share keys by design and are left out of conflicts.
- The text context swallows every key. Its owners read the raw keys:
  - the seed field: Enter or Esc commits;
  - the search box;
  - key capture.
- A modal (the order menu, the debug menu) switches the map keys off. Global keys stay on.
- A group of two or more replaces the single selections. A group of one collapses back to a single selection (`pruneGroup`).
- Formation:
  - a single colonist's plain right-click still goes exactly to the tile;
  - groups take the nearest free walkable cells (Chebyshev rings, then distance, row, column);
  - a right-drag places colonists evenly along the line;
  - colonists are assigned to cells greedily, nearest first, ties by id.
- An oval in an area tool is sent as one rectangle command per row run. Build tools still send one command per cell.

## Behaviour changes (tests updated)

- R drafts; the roof overlay moved to Shift+R. The comment on `TestRKeyToggles` now says so; the key is covered by `TestRDraftsAndShiftRTogglesRoof`.
- The draft button reads "Draft [R]" (was "Draft (R)").
- Esc while editing the seed now commits the seed instead of going back.

## Fixed double handling

- R both drafted and toggled the roof: R is now draft only.
- A right-click that left an Architect tool also ordered the colonist: the click is now taken (`TestRightClickLeavingToolDoesNotOrder`).
- Esc in Move mode also deselected the building: the Esc chain now handles Move mode once (`TestEscInMoveModeHandledOnce`).

## Verification

- `go build ./...`, `go vet ./...` and `gofmt -l` are clean.
- `go test -race ./...` passes.
- Determinism: `-count=3 -run 'Determin|SameSeed|Hash' ./internal/...` passes.
- Input package tests cover:
  - no conflicts in the defaults;
  - conflict rules;
  - context order and single consumption;
  - text swallowing;
  - holds and modifiers;
  - parse and label;
  - a settings JSON round trip;
  - normalize;
  - swap.
- Adapter tests (`fakeInput` plus `App.Update`) cover:
  - R / Shift+R;
  - F-keys and reserved toasts;
  - search typing drafts nobody, and finding a colonist;
  - `,` and `.`;
  - Shift+click, box and double-click selection, and double-click by species;
  - group draft, formation and line;
  - formation cells;
  - F forbid;
  - F10 and F11;
  - the room overlay and card;
  - Alt detail;
  - `shapeCells` for every style, with Ctrl;
  - Q/E style and oval zone rows;
  - bill ×10, ×100 and ×1000;
  - dev tick step and ultrafast;
  - Controls rebind, Esc, Backspace, swap, persistence and reset;
  - the double-handling fixes;
  - Numpad *;
  - gizmo keys;
  - wheel, PgUp and middle-drag;
  - Home and End.
- The hauler skipping forbidden stacks is covered by `game/forbid_test.go`.
- Captures were checked by eye at 1280×720 (`-capture-ui controls|search|rooms|multiselect|drawstyle|hideui`).

## Not done

- Heal in the debug menu: the game has no heal hook. "Fill needs" takes its place.
- Control groups.
- Beauty overlay, power toggle, and the Assign/Quests/World/History tabs. They are reserved and toast "not implemented yet".
- A shared overlay registry.
- The debug inspector has no components, path or reservations.
- Item search does not list storage contents.

## Determinism Impact

None: everything is in the adapter. Captures do not use the input layer, because `tickCapture` sets its own state.
