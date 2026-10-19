# Save and load

## Format (`application/game/save.go`)
- A gzip stream with two gob values in sequence:
  - `SaveInfo` header: version, slot name, in-game date, day, colonists alive.
  - `saveState`: every authoritative field of `Game`.
- `SaveVersion` = 1. Bump it on incompatible changes; there's no migration yet.
- Captured exactly:
  - The clock tick.
  - The RNG state: `randx.Rand` keeps its PCG source and gains MarshalBinary/UnmarshalBinary.
  - The entity allocator: `entity.Allocator.Peek` and `entity.Resume`.
  - Every component store, in dense order so iteration order is identical.
  - Blocked grid cells (`grid.Blocked`/`grid.Restore`).
  - Reservations (`reservation.Table.Export`/`Import`).
  - Every gameplay map, plus pending commands, messages, room grid and room temperatures.
- Unexported game types are mapped to exported DTOs: exposure, order queue, mental breaks.
- Rebuilt on load: room stats (`roomDirty`). Room temperatures carry over through the saved room grid.
- API: `(*Game).Save(w, name)`, `Load(r)`, `ReadSaveInfo(r)`, `(*Game).Info(name)`.

## Storage (`adapters/outbound/filesave`)
- `Store{Dir}`. The default is `os.UserConfigDir()/rimworld-go/saves`, which on macOS is `~/Library/Application Support/rimworld-go/saves`.
- Files are `<slot>.rwsave`. Slot names are sanitised to `[A-Za-z0-9_-]`, with spaces turned into dashes.
- Writes are atomic: a temp file in the same folder, then a rename.
- `List()` returns slots newest first. Unreadable or wrong-version files are listed with `Err` set.

## UI (`adapters/inbound/ebiten/saves.go`)
- `AppConfig.Saves` (`SaveHooks`: Save, List and Load funcs) is wired in `cmd/game/main.go` to the file store. Nil hooks disable the buttons.
- Each new colony gets the slot `colony-YYYY-MM-DD-HHMM`.
- Pause > Save overwrites that slot and resumes, with a "saved <slot>" hint.
- Pause > Load and main menu Load Game open the list (up to 9 rows). Each row shows slot name, game date, colonists and file time. Broken saves are shown disabled with the reason.
- Main menu Continue loads the newest loadable save. It's disabled when there are none.
- Autosave runs once per in-game day at 06:00 to `<slot>-autosave`. Loading an autosave resumes the base slot name.

## Tests
- `save_test.go`:
  - A busy colony is saved, loaded, then both run 15000 ticks in lock step with identical pawns, world and tick.
  - Garbage is rejected.
- `filesave_test.go`: save, list (including a broken file), load, and no leftover temp files.
- `saves_test.go`: the UI flow with stand-in hooks (pause > save > load > slot).

## Not done
- A save-name prompt or overwrite confirmation.
- Deleting saves.
- Rotating autosaves.
- Ironman mode.
- Plugin and content hash metadata.
