# internal/adapters/outbound/settingsfile

## Purpose
Stores the player's settings (`internal/platform/config.Settings`) as one JSON file, `<user config dir>/rimworld-go/settings.json`, next to the saves folder. The UI reaches it only through `ebiten.SettingsHooks`, wired in `internal/compose/game` (`settingsHooks`).

## Owns
`Store{Path}`, `Default()`, `Load()`, `Save()`.

## Depends on
`internal/platform/atomicfile`, `internal/platform/config`.

## Must not depend on
Domain, application and inbound adapter packages.

## Contracts
- A missing file is a fresh install: defaults, no error.
- An unreadable file is renamed to `settings.json.bad` (never silently overwritten) and the defaults come back with the error.
- Saves are atomic (temp file + rename).
