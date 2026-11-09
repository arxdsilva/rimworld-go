# internal/adapters/inbound/ebiten/options/saves

## Purpose
The client's book of saved games: saving the running colony to its slot (manual saves, the daily autosave and its rotation, a commitment colony's one slot), reporting the background saves, saving before leaving, and the Load screen's list. Split out of `options` (SAV-024…, the named-saves plan SAV-001…025) so the save features grow outside the Options screen's method budget.

## Owns
- `Book` built by `New(sh Shell)`; the root builds it over `optionsShell` and stores it as `options.Screen.Saves`. State the root reads: `Slots`, `SlotsRead`, `SaveSlot`, `AutosaveDay`, `SlotPage`.
- Today's behaviour: `SaveNow`, `PollSaves`, `TickAutosave`, `SaveBeforeLeaving`, `RefreshSlots`, `LoadSlot`, `LoadButtons`, `DrawLoad`; `background` (the async reports), `autosaveSlot`, `baseSlot`.
- Contract stubs, each with the plan id that fills it: `OpenSaveDialog` (SAV-001), `NewColony` (SAV-003), `SaveAndLeave` (SAV-005), `Leave` and `DrawPauseStatus` (SAV-013), `Click` (every `load.` id; delete, sort and colony filter are no-ops until the load list's work).
- Declared state for those items: `session`, `progress`, `pending`, `sort`, `filter`, `dialog`, `newID` (`crypto/rand`, 16 hex digits; tests set it).
- `AutosaveHour`, `CommitLoadReason`, `EndingTag`, `NewSlotName`.
- Interfaces (`deps.go`): `Host` (3: `ports.Texts`, `SaveEnv`, `PaintMenu`), `Saver` (7: `Save`, `List`, `Load`, `Delete`, `Finished`, `Wait`, `SaveFailed`), `Shell` = `Hinter + Settings + World + Widgets + Host + Saver`; `Env`, `SaveResult`.

## Depends on
- Leaves: `gfx`, `ids`, `ports` (`SaveSlot`, `SaveMeta` and the roles), `uikit`.
- `domain/sim/clock`, `platform/config` (through `ports.Settings`).

## Must not depend on
- The ebiten root, `options` (it imports this package), other features, `render/*`.
- Files or the simulation: saves go through `Saver`; compose (`internal/compose/game/files.go`) owns the store.

## Contracts
- `Saver.Save(meta)` writes the slot `meta.Name`; a commitment colony's one slot goes through the same hook (SAV-024 removed `SaveCommit`).
- `Saver.Load` returns the save's `ports.SaveMeta` and tick; the book keeps `baseSlot(meta.Name)` as the colony's slot.
- `Click` returns false for `load.back` so the root's screen intent takes it.
- Leaving goes through `Leave(proceed)`; Save & Quit through `SaveAndLeave(proceed)`, whose `proceed` is the leaving intent and so passes `Leave` again.
- Autosave writes `<slot>-autosave` (or `-autosave-1…N` with `AutosavesKept` above one) after `AutosaveDays`; a commitment colony writes its slot daily.

## Invariants
- No time value reaches the simulation (`time` names slots and formats the list only).
- `Book` stays under the SOLID limits; a feature that needs many entry points gets a component type of its own.

## Hot paths
- `PollSaves` and `TickAutosave` run every Update; both return at once without background saves or outside the autosave hour. `DrawLoad` paints at most `slotRows` rows.

## Tests
- `shell_test.go` (`fakeShell`), `store_test.go`, `async_test.go`, `autosave_test.go`, `book_test.go`. Root integration: `saves_test.go`, `saves_commit_test.go`, `saves_async_test.go`, the journeys.

## Notes
- `footerYOff` repeats the menu footer offset (`options`, root `app.go`).
- The book reads `options`' settings only through `ports.Settings`; the commitment lock on developer mode stays in `options` (`optionLock`).
