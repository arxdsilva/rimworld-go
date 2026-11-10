# internal/adapters/inbound/ebiten/options/saves

## Purpose
The client's book of saved games: saving the running colony to its slot (manual saves, the daily autosave and its rotation, a commitment colony's one slot), reporting the background saves, saving before leaving, and the Load screen's list. Split out of `options` (SAV-024…, the named-saves plan SAV-001…025) so the save features grow outside the Options screen's method budget.

## Owns
- `Book` (book.go) built by `New(sh Shell)`; the root builds it over `optionsShell` and stores it as `options.Screen.Saves`. State the root reads: `Slots`, `SlotsRead`, `SaveSlot`, `AutosaveDay`, `SlotPage`.
- Saving (store.go): `SaveNow`, `PollSaves` (the async reports), `SaveBeforeLeaving`, `RefreshSlots`. Autosave (autosave.go): `TickAutosave`, `autosaveSlot` (rotation inside the colony).
- Colony session (session.go, SAV-003): `NewColony` and `takeSession` set the colony's id, name and base slot; `metaFor` builds each save's `ports.SaveMeta`. `newID` makes the id (`crypto/rand`, 16 hex digits; tests set it).
- Save dialog (dialog.go, SAV-001, 002, 004): name field, the saves as a table, `nameReason` (from `savefmt.CheckName`), the overwrite question, `submit`, `commitSave`.
- Delete (delete.go, SAV-010): `DeleteReason`, the confirm, `deleteSlot`.
- Leaving (leave.go, SAV-005, SAV-013): `Leave`, `SaveAndLeave`, `askUnsaved`, `DrawPauseStatus`. Progress (progress.go): the dirty flag, from a save's encode tick until that save succeeds.
- Load screen (load_screen.go, rows.go, SAV-011, 012, 021, 022): `LoadButtons`, `DrawLoad`, `LoadSlot`, sort (Newest, Name) and colony filter, `FormatRow` and the compat and damaged tags. `Click` routes every `load.` id.
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
- `Saver.Load` returns the save's `ports.SaveMeta` and tick; the book keeps `savefmt.BaseSlot(meta.Name)` as the colony's slot.
- `Click` returns false for `load.back` so the root's screen intent takes it.
- Leaving goes through `Leave(proceed)`; Save & Quit through `SaveAndLeave(proceed)`, whose `proceed` is the leaving intent and so passes `Leave` again.
- Autosave writes `<slot>-autosave` (or `-autosave-1…N` with `AutosavesKept` above one) after `AutosaveDays`; a commitment colony writes its slot daily.

## Invariants
- A leave happens only after the file is written; a failed save keeps the player in the game.
- A colony is unsaved from a save's encode tick until that save succeeds; a Load or `NewColony` resets it (`TestProgressEventTable` is the event-table no-flap test).
- Save names are checked by `savefmt.CheckName` before any write; the Save button is off with the reason shown.
- No time value reaches the simulation (`time` names slots and formats the list only).
- `Book` stays under the SOLID limits; a feature that needs many entry points gets a component type of its own.

## Hot paths
- `PollSaves` and `TickAutosave` run every Update; both return at once without background saves or outside the autosave hour. `DrawLoad` paints at most `slotRows` rows.

## Tests
- `shell_test.go` (`fakeShell`), `store_test.go`, `async_test.go`, `autosave_test.go`, `book_test.go`, `dialog_test.go`, `leave_test.go`, `load_screen_test.go`, `progress_test.go`, `rows_test.go`. Root integration: `saves_test.go`, `saves_commit_test.go`, `saves_async_test.go`, `journey_saves_dialog_test.go`, `journey_saves_leave_test.go`, `journey_saves_load_test.go`.

## Notes
- `footerYOff` repeats the menu footer offset (`options`, root `app.go`).
- The book reads `options`' settings only through `ports.Settings`; the commitment lock on developer mode stays in `options` (`optionLock`).
