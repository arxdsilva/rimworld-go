# Saves gap analysis (SAV-*)

Scope: several named saves per colony, as the reference game offers them: naming a save, overwrite and delete questions, a list that says what each file holds, warnings about older, newer and damaged files, and unsaved progress on leaving.
- Load only for SAV-* tasks.
- Handoffs are sections of `.agents/handoffs/UI.md` (`SAV-0…3`).
- Paths are under `internal/` unless they say otherwise. Find the function; line numbers drift.
- Every row below was checked against the commits named in it and the code at HEAD (2026-11-09).

## Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for "multiple saves just like the reference game" and for every gap found to be implemented. The gap survey is in the plan; this file is its record. |
| What "like the reference game" means | A save has a player-chosen name; many saves of one colony can sit side by side; saving over a file asks first; a save can be deleted after a question; the list shows colony, day, build and state; a save from another build is flagged; a leave with unsaved progress asks. Commitment mode keeps its rule: one slot per colony, written by the game, no reloading at will. |
| Kept as not vanilla | Rename of a save, thumbnails in the list, and a quicksave hotkey. The reference game does none of them, so we do not add them. Recorded as kept, not as gaps. |
| Blocked | Plugin discovery and load order. There is no mod system, so there is nothing to discover or order. SAV-025 ships what works today: a content hash, the plugin list `["core"]` and the missing-plugin check tested with fakes. |
| Clean room | `CLEAN_ROOM.md`: behaviour only. Wiki pages are read for what the player can do; no wiki text, number, name or art is copied. We own the dialog layout, wording and file format. |
| Colony identity | A colony has a random id made in the adapter (`crypto/rand`), never in simulation state, so determinism and the state hash are untouched. Rotation, Continue and the commitment guard key on the id. A save from before v16 gets `"slot:"+<base slot>`. |
| Format | `SaveVersion` 16 is a header-only change: no state hash moves (`statehash` pins its own info version) and every v1..v15 field keeps its gob name and type. The header type lives in `game/savefmt` and `game.SaveInfo` is an alias. |
| Layering | The saves book left `options` for `options/saves` (type `Book`), with its own small consumer-owned `Host` and `Saver` interfaces. `options.Screen` keeps one field. The store, queue and verifier are in `filesave`; compose maps headers to `ports.SaveSlot`. |
| Dead code | The `SaveCommit` hook was removed; a commitment colony's one slot goes through `Save`. |

## Sources

The wiki pages were read for behaviour and are described here in our own words only.

| Page | What we took from it |
|---|---|
| Save | A game is stored as a file chosen by name; several files can exist; the list shows when each was written and which game version made it; a file from another version loads with a warning. |
| Commitment mode | One save per colony, written by the game; the player cannot reload at will; leaving is by saving and quitting, and death ends the colony. |

The reference game's Save and Load windows were not measured for this file; the dialogs use the shared `uikit` widgets and the screens spec.

## Rows

State legend: `done` (all acceptance criteria met, with the commit), `[!]` blocked.

### Naming and saving

| ID | Missing before | Ships now | Evidence | State |
|---|---|---|---|---|
| SAV-001 | The pause menu's Save wrote the colony's one slot with no name field. | A Save dialog over the pause menu: a name field prefilled with the colony's last save and selected, Save and Cancel, and the saves on disk as a table; clicking a row takes its name. | `adapters/inbound/ebiten/options/saves/dialog.go`; `TestJourneySavesSaveAsNewName` (`ebiten/journey_saves_dialog_test.go`); `TestSaveDialogSavesUnderTheTypedName`, `TestSaveDialogRowTakesItsName`, `TestSaveDialogCancelAndEscapeSaveNothing` | done, `c3e5ebb72` |
| SAV-002 | Saving over a file replaced it with no question. | A danger confirm names the file's date, and the other colony when it differs. | `TestJourneySavesOverwriteAsks`; `TestSaveDialogAsksBeforeReplacing` | done, `c3e5ebb72` |
| SAV-003 | No colony identity: autosaves and Continue worked on slot names, so a second name would have forked the rotation. | A colony session (id, name, base) from `NewColony` or the loaded header. The header carries `Colony`, `ColonyName`, `Base`, `Autosave`, `Season`. Autosaves rotate inside the colony and follow it after a Save As; legacy autosaves keep rotating. | `game/savefmt/info.go` (`Info`, `Stamp`, `Upgrade`, `BaseSlot`, `SlotKey`); `options/saves/session.go`, `autosave.go`; `TestUpgradeNamesTheColonyOfAnOldHeader`, `TestAutosavesRotate`, `TestNewColonyStartsTheSession`, `TestRandomIDIs16HexDigits`, `TestJourneySavesAutosaveFollowsColony`, `TestJourneySavesLegacyLoadAutosaves`; compose round trip `TestSaveHeaderRoundTripsThroughTheHooks`; `testdata/save_schema.txt` refreshed | done, `0f6700b19`, `6e935dfec`, `c3e5ebb72` |
| SAV-004 | The name was sanitised silently and the header kept the sanitised form. | `savefmt.CheckName` refuses an empty name, one that is too long, bad characters, the reserved autosave suffix and device names. The reason shows in red and the Save button is disabled. A different spelling of an existing file shows a same-file note. The header keeps the name as typed. | `game/savefmt/names.go`; `TestCheckNameRules`; `TestJourneySavesNameValidationVisible`; `TestSaveDialogShowsWhyANameIsRefused` | done, `3f46a8a97`, `c3e5ebb72` |
| SAV-005 | Leaving was Return to menu or Quit with no save step. | Pause menu has nine rows in both modes: Save & Quit to Menu and to Desktop. A commitment colony's Save row is shown off (the game saves itself). The leave happens only once the file is written, and a failed save keeps the player in the game. The row ids and routes are in `ids`. | `options/saves/leave.go`; `ebiten/screen_pause.go`; `TestJourneySavesSaveAndQuitToMenuReloadAnytime`, `TestJourneySavesSaveAndQuitToDesktopReloadAnytime`; `TestLeavingStaysWhenTheCommitmentSaveFails` | done, `b118decff` (ids, routes), `f4eaca8b8`, `b94dd2f32`, `fe3c8a910` (catalog keys) |

### Managing and reading saves

| ID | Missing before | Ships now | Evidence | State |
|---|---|---|---|---|
| SAV-010 | No way to delete a save. | `Store.Delete` removes the `.rwsave` then the `.rwview`; a missing file is not an error. `Queue.Delete` drops the slot's waiting jobs and runs right after the write in flight. A Delete button on each row asks first; the running commitment colony's save cannot be deleted. The list is read again afterwards. | `adapters/outbound/filesave/delete.go`, `platform/atomicfile` (`Remove`); `TestStoreDeleteRemovesSidecar`, `TestQueueDeleteWaitsForInFlightWrite`, `TestQueueSaveAfterDeleteRecreates`, `TestDeleteHookRemovesTheSave` (compose); `TestJourneySavesDeleteConfirm`, `TestDeleteAsksAndRefreshes`, `TestDeleteRefusesTheRunningCommitmentSave` | done, `6ea621105`, `c3e5ebb72` |
| SAV-011 | A row showed the file name and date only. | Two-line rows: title and tags, then colony, day and season, colonists, storyteller, biome and build, each line cut to fit; the hover card holds every field. Old headers leave blanks, never "Day 0". `FormatRow` is shared by the Load screen and the Save dialog. | `options/saves/rows.go` (`FormatRow`); `rows_test.go`; `TestJourneySavesLoadRowShowsColonyAndBuild` | done, `f4eaca8b8` |
| SAV-012 | The list was one page-through sorted by time. | Sort toggles Newest and Name (case-folded, stable); a colony filter cycles All and each colony. Both reset paging. | `options/saves/load_screen.go`; `TestLoadSortAndColonyFilter`; `TestJourneySavesSortAndFilter` | done, `f4eaca8b8` |
| SAV-013 | Leaving or loading over a colony lost unsaved progress without warning. | A colony is dirty from a save's encode tick until that save succeeds; Load and `NewColony` reset it. A leave or an in-game load in a reload-anytime colony asks "Unsaved progress since <date>" with Save & leave, Leave, Cancel. The pause panel says "Saving…", "Not saved yet" or "Saved N in-game hours ago". | `options/saves/progress.go`, `leave.go`; `TestProgressEventTable` (the event-table no-flap test), `TestSaveNowTracksProgress`, `TestLeaveAsksAboutUnsavedProgress`, `TestInGameLoadAsksAboutUnsavedProgress`, `TestPauseStatus`; `TestJourneySavesUnsavedProgressConfirm` | done, `f4eaca8b8`, `b94dd2f32` |

### Compatibility and integrity

| ID | Missing before | Ships now | Evidence | State |
|---|---|---|---|---|
| SAV-020 | A header did not say which build wrote it. | `Build` from `buildinfo.Version` is stamped into the header by compose. | `compose/game/files.go`; `TestSaveHeaderRoundTripsThroughTheHooks` (checks the build, season and version); `TestStampFillsTheClientFields` | done, `6e935dfec` |
| SAV-021 | A newer or too-old save failed late and the player could not tell why. | `savefmt.CompatOf` compares versions: older loads after a confirm ("will be upgraded"); newer or too old is disabled with the reason and what to do; a same-version save with other content asks. `filesave.List` keeps the header of another version with an error that names both versions. Continue takes the same path. | `game/savefmt/compat.go`; `TestCompatOf`; `TestSlotOfSaysWhyASaveCannotLoad` (compose); `TestJourneySavesOlderVersionAsks`, `TestJourneySavesNewerRefused` | done, `6ea621105`, `f4eaca8b8` |
| SAV-022 | A cut or flipped file passed as fine until it failed in play: the gzip checksum was never read. | `game.Load` ends the body through `savefmt.Finish`: a decode error is `ErrCorrupt`, and the rest of the gzip stream is drained so its CRC and size are checked. `filesave.Verifier` (built once in compose) checks each unseen file to its end in parallel, remembered by path, size and modification time; a damaged file is disabled with "damaged file". A file that fails to load is marked damaged until the list is read again. | `game/savefmt` (`Drain`, `Finish`); `adapters/outbound/filesave/verify.go`; `TestDrainDetectsFlippedByte`, `TestDrainFindsACutFile`, `TestFinishWrapsDecodeErrors`, `TestListMarksCorruptBody`, `BenchmarkListVerify10Large`; `TestJourneySavesCorruptDisabled` | done, `6ea621105`, `f4eaca8b8` |
| SAV-023 | `-save-dir` did not reach the Options "Saves folder" button. | `SaveHooks.Dir` is set from the store and flows into `Env.SavesDir`. | `compose/game/files.go`; `TestSaveDirReachesTheSavesFolderRow`, `TestSaveDirReplacesThePlayersSaves` | done, `b118decff` |
| SAV-024 | The `SaveCommit` hook was dead code, and the everyone-gone Load dialog showed a stale list. | `SaveCommit` and `App.saveHook` are deleted (a commitment colony's slot goes through `Save`); a pushed Load screen re-reads the list. `App` methods go from 243 to 242. | `ebiten/options_compat.go`, `app.go`; commitment tests in `ebiten/saves_commit_test.go` assert the slot through `Save`; `TestCommitmentAutosavesItsOwnSlotDaily` | done, `b118decff` |
| SAV-025 | No way to tell that a save was made with other content. | New `game/contentid`: an FNV-64a fingerprint of the content registries (research, recipes, buildings, biomes, factions, incidents, traits, drugs, backstories, quests, animals), computed once in compose. `Content{Hash, Plugins:["core"]}` goes into every header. A same-build save with another hash asks before loading. The missing-plugin check is tested with fakes. | `game/contentid`; `TestFingerprintIsStable`, `TestFingerprintOfFollowsTheContent`, `TestDefsPrintDeterministically` (no func, pointer, interface or channel in a definition); `TestMissingPlugins` (`savefmt`); `TestMissingPluginNotice` (compose) | done, `6ea621105` (hash, header), `0f6700b19` (types) |
| SAV-025 (discovery) | Discovering plugins and choosing their load order. | Not built: the game has no mod system. | n/a | `[!]` blocked: needs a mod system |

## Known leftovers (true at HEAD)

- **Enter in the name field does not save.** `uikit/textfield.go` ends the edit on Enter and stores the value, but the dialog binds Save only to its button (`options/saves/dialog.go`, `submit`). The player must click Save.
- **The commitment daily save is written with `Autosave:false`.** `Book.SaveNow` (`options/saves/store.go`) builds its header with the autosave flag off, so the commitment colony's one daily slot is not tagged Autosave in the list. This is harmless today (the colony has one slot, found by id), but a future rule that reads the flag will see a manual save.
