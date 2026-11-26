# internal/application/game/saveutil

## Purpose
Pure save and hash compatibility transforms extracted from game (SIM-074, PRD-6). This package owns no live state or I/O.

## Owns
BitIndexes, TerrainHash, NilIfEmpty, work-priority export/import CanonicalHashFields, typed entry export/import and selective ownership copies for detached save capture. Game owns field names, snapshots, version and StateHash.

## Contracts
Bit indexes ascend; geology seed derives from explicit terrain bytes. Empty maps normalize at the serialization boundary. Restore shorter historical priority lists without shifting IDs, including the PlantCut migration. Replay hashes remain independent of save-only aliases and appended field representation. Do not retain or mutate the live world. Ownership copies preserve the exact wire shape and existing entry types; copy every borrowed mutable nested payload, reuse already detached exports, and keep the synchronous save and hash path coherent with the same schema. New reference fields require populated ownership tests.

## Tests
Root save/load and schema tests, historical fixture replay and scale goldens cover actual callers.
