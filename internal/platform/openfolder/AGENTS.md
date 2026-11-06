# internal/platform/openfolder

## Purpose
Opens a folder in the system file browser (RHUD-058, DOS-6). Fills `dossier.FolderOpener`; wired in `cmd/game`.

## Contracts
- `Open(path)`: `open` (macOS), `explorer` (Windows), `xdg-open` (others) with the path as one `exec.Command` argument, never a shell string. Creates the folder when missing. Starts and returns; an error means the player sees a disabled reason.
