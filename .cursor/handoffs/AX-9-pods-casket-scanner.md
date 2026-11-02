# AX-9: stasis casket, transport pods, mineral scanner, hitching spot

Scope: the AX programme's A9 rows (`.cursor/ARCHITECT_GAP_ANALYSIS.md` Misc), ENG-003 wiki numbers, original names. The five kinds are no longer Planned (`construction/ax_a9.go`), so the gap doc has no placeholder left (191 rows, all working cards).

## Behaviour
- **Stasis casket** (wiki: Cryptosleep casket).
  - A colonist climbs in, or carries a downed colonist or prisoner in, from the right-click menu (`EnterCasket`; jobs `EnterCasket`, `CarryToCasket`).
  - The sleeper is a stashed pawn: off the map, so no need falls and it does not age. `isPatient` (patients.go) leaves it out, so bleeding, disease, infection, organ and aging conditions stand still.
  - A prisoner stays a prisoner: the stash now marks only caravan members as away.
  - The pane's Eject (`EjectCasket`) lets the sleeper out at once. A casket that is destroyed, deconstructed or crated breaks open (the `removeBuildingAs` hook).
  - Occupied sprite and glow.
- **Transport pods and pod launcher.**
  - Pods that touch edge to edge form a group, named by its lowest pod. Each lifts 150 kg (wiki), and each needs a launcher beside it (the lowest one). Masses come from `item.Kind.MassGrams` (wiki per-unit masses, category fallbacks), people weigh 60 kg (wiki), and animals are by body class (original).
  - The launcher holds 150 chemfuel (wiki), which haulers refuel with chemfuel only. A pod burns 2.25 chemfuel a tile (wiki), so a full launcher reaches 66 tiles, and one launcher feeding two pods reaches half as far.
  - The load dialog (`LoadPods`) picks colonists, tame animals and goods by amount, and refuses an overweight load. Colonists walk in by themselves. Haulers bring the goods and lead the animals in (`job.LoadPods`, Haul work). Unload sets everything down.
  - Launch: the World tab target picker (from the globe or a list of targets in range) sends `LaunchPods`. The pods are spent and fly ten tiles an hour (original), shown on the globe and in the list.
  - Landing:
    - at the colony, the load comes down at its drop spot;
    - as a gift, the settlement gains goodwill by market value (`faction.GiftGoodwill`);
    - as a visit or a drop elsewhere, the passengers become a caravan on that tile (`travelHost.Land` and `caravanArrived`), and the goods and animals ride along as cargo, set down when the caravan comes home.
  - Refused: attacking or founding (both need a second map), people as a gift, and landing with nobody aboard.
- **Long-range mineral scanner.**
  - A researcher works it on power (`job.ScanMinerals`, Research work).
  - A find comes after 1-480000 work (hashed; the wiki's mean of 240000 and its guarantee). It lays a site 6-25 tiles out with the wiki's lump for the target mineral (any by default, by original weights), which expires after 30 days.
  - The pane's Target menu sends `SetScannerTarget`. Sites show on the globe.
  - **Missing: the game has no second map,** so a caravan or pods reaching a site mine it in two days (original) and carry the lump home as cargo.
- **Caravan hitching spot.** A forming caravan's members gather around a spot (picked by caravan id when there are several). Once all are there they wait an hour, then head for the edge, with a half-day limit. Once they set off they never go back (no-flap test). There are no pack animals or loading.

## Code
- **New package `game/travel`**, with its state in `worldSt.ax9` and its cached host in `worldSt.ax9h` (no new Game field; A8's `ship` sits beside them).
  - Operations are split by concern: `caskets`, `pods`, `scanner` and `hitching` over a shared `core`. `Ops` is the facade the game calls.
  - Saved as `worldSave.AX9` (optional; nil when empty).
  - The adapter `travelHost` (world_state.go) embeds the industry, herd, care, supply and raid adapters.
- **Domain:** `item/mass.go` and `caravan/pods.go` (pod constants, animal masses, range and fuel).
- **UI:**
  - `inspector/ax9.go`: pane lines and the free function `TravelCommands`;
  - `dialogs/pods.go`;
  - `windows/world_ax9.go`, which adds the markers, the travel list, the picker and `LaunchFor`;
  - intents `ports.LoadPods` and `ports.LaunchPods`;
  - `render/world/travel_keys.go` and `render/overlay/travel_draw.go`, which draw the casket glow and the launcher gauge.
- **Game lines:** net about -18 lines, with no Game method or field. The moves:
  - WorldSites became `travel.SettlementViews`, with WorldSiteView an alias;
  - `joinNames` became `travel.JoinNames`;
  - `caravanStateLabel` became `caravan.State.Label`;
  - the parsing in `caravanCandidates` became `travel.ParseIDs`;
  - the `worldEdge` loop became `travel.EdgeTile` (same draws);
  - `freeLandNear` and `insetLandNear` now share `travel.Ring` (same order).
- **Shared edits (exact):**
  - `ScanMinerals` joins `ScanGround` in the postpone, occupancy, action and apparel lists;
  - the PodLauncher fuel row in `industry/fuel.go`, and chemfuel only in `fuel.go`;
  - the Misc catalog rows; `build_keys.go`, `objects.go` and `power_draw.go` (three lines).

## Tests
- `game/ax9_test.go`:
  - the casket's suspension, eject, destruction and carry;
  - pods: load, launch, visit, cargo home; gift and range; drop home;
  - the mineral site, found and mined;
  - the scanner, worked, with no flap;
  - the hitching spot, with no flap;
  - save.
- `travel/travel_test.go`, `item/mass_test.go`, `caravan/pods_test.go`.
- Journeys in `journey_ax9_test.go`:
  - every tool laid; casket enter and eject; the scanner target; the hitching pane; load and unload;
  - on a wizard colony, a launch through the World tab picker that lands.

## Follow-up (after wave 2)
- **Stasis casket:**
  - Downed prisoners and tame animals can be carried in (wiki). The game's stash (world_stash.go) now holds prisoners, who return to raiderIDs, and animals, whose whole state rides in `stashRec.Beast` and comes back through `putAnimal`. There is one stash rule for both caskets.
  - People wake with cryptosleep sickness (`body.CondCryptosleepSickness`, `cond_crypto.go`; wiki: 10000 ticks, consciousness x0.8, moving and manipulation x0.9, a vomit every 3 h on average). It is stepped hourly with food poisoning, and the wiki gives animals none.
  - At 50 HP or less a casket sparks; 120 ticks later it explodes (wiki: flame, 10 damage, radius 2.66). The fuse is one-way, with a no-flap test.
  - Carrying into the ship's caskets goes through the `Carriers.ShipCasketFree`/`LayInShip` role (A8 owns the host side). `cryptoSicken(g, id)` is shared with the ship's wake.
- **Caravans carry goods and pack animals** (`travel/pack.go`):
  - The Form caravan dialog has a pack animals and goods column, a mass against carrying capacity line (wiki: body size x 35 kg) and the pace.
  - Picks ride in FormCaravan's Name (`id`, `stack:qty`).
  - The caravan gathers at a hitching spot, or at the drop spot when it has goods and no spot. Haulers and the waiting members carry the goods there and lead the animals in, which are tied and wander nearby (`job.LoadPods` with `BillIndex` = the caravan).
  - It leaves once loaded plus an hour, or after half a day.
  - The goods become cargo, which comes home with the caravan; the animals leave the map with it.
  - Carried mass slows it to a quarter of the empty pace at full load (the wiki's x2 to x0.5, linear; original between the ends).
- **Game lines:** paid by moving `splitKit` to `scenario.SplitKit`, the `casketExit` search to `travel.ExitBeside`, and `CaravanView` to `travel.CaravanView` (aliased). The package is at 53537 of 53539, with no Game method added (the helpers are free functions).
- **Tests:**
  - `game/ax9_test.go` adds animals and prisoners in the casket, sickness and the explosion (no flap), and caravan packing with capacity;
  - `body/cond_crypto_test.go`;
  - journeys: a downed colonist carried in from the menu, and the caravan dialog's goods.

## Not done
- A hauler's Open order for the stasis casket (Eject is immediate).
- Trading a caravan's cargo at a settlement (there is no caravan trading).
- Attacking a settlement, founding a colony, and site maps (no second map).
- A range ring on the globe.
- `PawnView.AX.Casket` (sleepers are off the map, so they have no pawn view).
