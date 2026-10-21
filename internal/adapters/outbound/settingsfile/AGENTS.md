# settingsfile outbound adapter

Stores the player's settings (`internal/platform/config.Settings`) as one
JSON file, `<user config dir>/rimworld-go/settings.json`, next to the
saves folder.

## Purpose

Load and save the Options. The UI reaches it only through
`ebiten.SettingsHooks`, wired in `cmd/game` (`settingsHooks`).

## Owned concepts

- `Store{Path}`, `Default()`, `Load()`, `Save()`

## Allowed dependencies

- `internal/platform/config`, the standard library

## Forbidden dependencies

- domain, application and inbound adapter packages

## Contracts

- A missing file is a fresh install: defaults, no error.
- An unreadable file is renamed to `settings.json.bad` (never silently
  overwritten) and the defaults come back with the error.
- Saves are atomic (temp file + rename).
