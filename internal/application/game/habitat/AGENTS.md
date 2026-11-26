# internal/application/game/habitat

## Purpose
Choose reachable dry, uncovered starting habitat for wild herds (SIM-019, WG-19). The native population policy remains in animal; the caller owns spawn IDs, placement and lifecycle.

## Owns
Selector owns one reachable-cell flood for a placement batch and consumes a caller-provided predator list. Cells is the narrow map/read and seeded-roll contract.

## Contracts
New builds reachability from a walkable start while retaining the landing cell as the distance anchor. Open and habitat selection reject fog, roofs, water and disconnected cells. Grazers require nearby forage; predators use supplied separation facts. Candidate selection returns coordinates without creating animals or mutating the map.

## Hot paths
Create a selector only for initial habitat selection and reuse the flood across the batch. Never place it inside the per-animal think loop.

## Tests
Root wildlife/ecosystem and generated-colony tests exercise placement, habitat and low-density stability. Keep generation deterministic at the existing scale-golden seeds.
