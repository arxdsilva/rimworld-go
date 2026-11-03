# atomicfile

Writes a file so that a crash or a failed write leaves the old contents or the new ones, never a mix: temporary file in the same folder, flush, rename, flush the folder.

## Allowed dependencies

- The standard library only.

## Why this isn't a library

`github.com/google/renameio` is the well-known one, but its README says it exports no functions on Windows, and we release for Windows (GO_BEST_PRACTICES §86, step 3).

## Contracts

- Use it for every file the game or its tools must not leave half-written.
- `perm` is the final mode, set exactly. The process umask does not change it.
