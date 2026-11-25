# internal/application/game/savelegacy

## Purpose
PRD-6 and SIM-074 compatibility with historical Gob field shapes before v21. This is a load-time migration seam, not another gameplay representation.

## Owns
Historical wire records and explicit typed conversions into canonical records. Current schema and version remain owned by game.

## Contracts
Preserve historical field names, array widths and stable IDs. Do not silently truncate arrays, overwrite explicit settings zeros or invent old values. Extend skill arrays explicitly, preserve field meanings and leave wire-only data at the persistence boundary.

## Tests
Root historical-save, SaveSchema, scale-golden and replay tests use this seam through the actual loader. Keep the frozen v12 fixture unchanged and independently verify old field migration.
