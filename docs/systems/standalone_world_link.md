# Linking a historical standalone colony to a world (WG-19)

game.AttachPlanet(g, planet.Params) attaches an explicitly generated planet
to a loaded standalone colony. It preserves the original local map, residents,
items, clock, simulation RNG and historical diplomacy. The operation returns
changed=false for a colony that already has a world. Invalid settings,
mismatched original faction provenance, multiple maps and active world transport
or markets refuse attachment without mutating the colony.

## Ownership and persistence

game/worldlink.Bind owns deterministic planet landing and faction provenance.
Original faction goodwill, relationships, names and history are retained;
the generated world contributes settlement locations. The current game/maps
BuildPlanet and Markets remain the native world and transaction owners.

worldState.link records this historical placement. worldSave.Link is an
additive field on the existing World v3 record; old standalone saves leave it
nil and remain worldless until an explicit caller attaches one. The link saves
a cloned copy of the current factions because older colonies have no NG1 setup
record to persist their diplomacy. The planet geometry itself is rebuilt on
first access and is never saved.

Existing WorldParams, WorldSites, caravan formation, map promotion and
World-tab read models expose the attached world. No new player-facing screen
is required. The caller is responsible for verifying the historical source
save and choosing when to persist the upgrade; this game API performs no
external account, database, backup or server operation.

## Compatibility

Existing SaveVersion 21, command numbering, ordinary NG1 setup world path
and all pre-existing schema fields remain unchanged. Adding the world-link
record introduces 38 additive gob schema paths and no removals.
The game root remains under its 52,650-line SOLID limit by moving mineral
site placement and map roster census into game/maps, behind narrow ports.

The existing stream codec retains a single authoritative seed. World
attachment does not introduce a second stream codec or consume the
simulation RNG.

## Verification

Integration tests exercise 64 and 250 tile standalone colony maps,
historical diplomacy and entity identities, save/load parity, idempotence,
invalid settings, incompatible factions and native caravan formation.
Additional tests cover later goodwill changes, detached WorldParams views,
and refusal when world activity exists. The game/maps tests prove mineral
placement, ore yields, roster census, stable boarding order and detached
read slices.

Planet geometry is built only on explicit attachment or first world access.
Existing save/replay scale goldens, the historical v12 fixture and world
market and race tests remain authoritative.
