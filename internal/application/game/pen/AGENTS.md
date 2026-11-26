# internal/application/game/pen

## Purpose
Native livestock pen defaults, acceptance and bounded enclosure discovery (FOOD-3). Root game owns structures, invalidation, animal assignment and jobs.

## Owns
DefaultSettings, DefaultAutoCut, Accepts and the Region returned by Fill.

## Contracts
Fill visits four-connected standable cells. A reached map edge, exceeded cell limit or invalid start cannot yield an enclosed region. Callers reuse the region until structural invalidation. Missing species settings accept the species by native default. Do not introduce duplicate pen state in animals.

## Tests
Root pens_test.go covers live membership, species/sex filters, invalidation and no-flap assignment. Herd and auto-slaughter tests cover consumers.
