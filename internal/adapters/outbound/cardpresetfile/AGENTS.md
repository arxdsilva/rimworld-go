# cardpresetfile outbound adapter

Keeps the colonist card's presets (DOS-6, RHUD-360, 368, 369, 377) as one
JSON file each in `<user config dir>/rimworld-go/dossier/`, beside the
settings file.

## Purpose

Fill `dossier.PresetStore` (`List`, `Load`, `Save`, `Delete`). The setup
window's Presets tab reaches it only through that consumer-owned role; it is
constructed in `cmd/game`.

## Owned concepts

- `Store{Dir}`, `Default()`, `ErrName`, `MaxBytes`, `MaxName`

## Allowed dependencies

- `internal/platform/atomicfile`, the standard library

## Forbidden dependencies

- domain, application and inbound adapter packages (the preset format is
  `dossier/sheet`'s; this adapter moves bytes)

## Contracts

- A name is 1..64 letters, digits, `-` or `_`. Anything else (empty, `..`,
  slashes, dots, spaces, NUL) is `ErrName` and touches nothing.
- Files are `<name>.json`; `List` skips other files and returns sorted names.
  A missing folder is an empty list; the folder is made on first `Save`.
- Saves are atomic; a file or save over 256 KiB (`MaxBytes`) is refused.
- Deleting a missing preset is not an error.
