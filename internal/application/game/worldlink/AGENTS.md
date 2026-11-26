# internal/application/game/worldlink

## Purpose
WG-19 binds a historical standalone colony to a deterministically generated world without replacing its map or diplomatic identity.

## Owns
Link, Context, Bind, Planet and Restore. Link preserves planet parameters, tile and faction history; the rebuilt globe is derived. Root owns installation and save lifecycle.

## Contracts
An existing binding is idempotent. Require a compatible standalone colony with no conflicting prior activity. Validate coverage, landing and faction IDs/types before merging generated settlements with old faction state. Clone and Restore isolate mutable slices. Rejection has no side effects.

## Tests
Root historical world attachment and multi-map save/load tests exercise valid, rejected and repeated attachment, preserved map and faction state. World UI journeys consume the native link.
