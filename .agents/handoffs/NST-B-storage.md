# NST-B: storage behaviour of the 27 Neat Storage pieces (WP-B)

Rows F1-F5, F9, F10 of `.agents/gaps/NEAT_STORAGE_GAP_ANALYSIS.md`, and the read model of "Display of stored items".

## Done

- `zone/profile.go`: `Profile` constants 1..27 (saved, append only), table of slots, default priority and item set (`profileRows`), `UniverseOf`, `NewProfile`, `ProfileAt`. Ceilings are built once in `sets.profile`.
- `zone/filter.go`: `SetAllowed` and `PasteFrom` clamp to `UniverseOf`; `RowsForProfile(k, p, names)` beside `RowsFor`; a piece without a filter goes by its ceiling. Older zones' switches are untouched.
- `zone/group.go`: `Link` refuses unequal profiles. `zone/map.go`: `Set` and `Update` keep `Profile`. No new `Map` method (36).
- `store/buildings.go`: `ZoneKind` for all 27; `Of`, `ZoneOf` match the profile; `Lay` adopts only a zone of its own profile, and lifts and unlinks a stale zone of another; `ProfileKind`, `ProfileName`, `ProfileRows`, `Buildings.Slot`. `store/deps.go`: role `Ground`.
- The five `Stacks()` sites read `z.Slots()`. `onShelf` is `Kind.Building() && != KindHopper`.
- `ItemView.Store/Slot` filled in `appendItemViews` (one `zone.ProfileAt`, no copy; a piece's stacks read the ground index's cell list). `ZoneView.Profile` in `buildZoneViews`. `game.ProfileFilterRows(profile, filter)` for the window. Net lines in `game`: 0.

## Invariants

- A profile means something only on a `KindShelf` zone, and never changes after the zone is laid.
- A piece's filter stays inside its ceiling through every command; a group shares one profile.
- Sheltering is as before for older kinds, proved by `TestShelterStaysAsItWas`: shelf, small shelf, egg box, bookcases yes; hopper, grave, sarcophagus no.
- `Store` is set for the 27 only, not plain shelves or bookcases: they have no slot attachments, so their draw doesn't change. It deviates from the doc's "the stored-in building's kind" on purpose.
- `Slot` is the index in the ground index's cell list. The doc's "counter kept from the previous item" can't work: stacks of one cell aren't adjacent in id order.

## Tests (all `-race`)

`zone/profile_test.go` (`TestProfileTable`, `TestProfileCeilingHolds`, `TestProfileOnTheMap`), `store/neat_test.go` (`TestZoneKindNeat`, `TestLayKeepsItsOwnProfile`, `TestLinkEqualProfilesOnly`, `TestSearchFillsToSlots`, `TestItemSlot`, `BenchmarkSearchManyShelves`), `game/neat_storage_test.go` (`TestNeatStorageHauling`, `TestNeatStorageHaulersFill`, `TestNoFlapStorage`, `TestNeatStorageSettings`, `TestShelterStaysAsItWas`, `TestNeatItemViews`, `TestNeatStorageRoundTrip`, `TestNeatProfileMatchesConstruction`).

Benchmark, 300 zones, one search, no allocations: plain shelves 20.2 us, crates 20.5 us, mixed profiles and priorities 3.3 us (filter and priority prune first).

## For the lead

- `TestSaveSchema` fails until `save_schema.txt` gains `saveState.ZoneMap.Zones[].Profile`. Lead pin.
- `TestSameItemViewSeesEveryField` (`food_contract_test.go`) fails on the contract's `uint8` fields: the test's switch needs `reflect.Uint8` beside `reflect.Uint64`. `sameItemView` covers both; `TestNeatItemViews` checks that. Not my file.
- For WP-C: the name is `zone.RowsForProfile`, not the doc's `RowsFor(k, p)`. `windows/storage.go:192` and `architect/zone_select.go:268` call `game.StorageFilterRows`; when `z.Profile != ""` they must call `game.ProfileFilterRows(z.Profile, z.Filter)`.
- For NST-D: `Slot` can pass a piece's slot count if a cell was overfilled by a spill chain's last resort (`mergeOrSetFrom` sets a stack down where it is), so clamp when looking up `slot<N>`.

## UI

Player-facing surfaces (Architect cards, inspector, renderer) are NST-C and NST-D; this package only feeds them through the read models.
