# AX-8: the escape ship, end to end

Scope: the AX programme's A8 rows (`.cursor/ARCHITECT_GAP_ANALYSIS.md` Ship), built at ENG-003 wiki numbers with original names and prose. The wiki pages are Ship, Ship reactor and Persona core. MASTER_SPEC §89 lists "space travel; planetary endgames".

## Behaviour
- **Parts.** Six kinds: structural beam, ship stasis casket, computer core, reactor, engine and sensor cluster. Costs, work and research are the AX-0 contract's defs. All six are now live (`construction/ax_a8.go` plans nothing), and every one has an Architect card in the Ship category.
- **Connectivity (wiki: Ship).** Parts that touch side to side are one ship. A casket joins only through a beam. The ship id is its lowest part id. `game/ship` recomputes the ships only when the structure version moves.
- **Launch report.** It lists what the ship lacks:
  - three engines;
  - a reactor, a computer core, a sensor cluster and a beam;
  - a casket joined to a beam;
  - a started reactor whose 15-day countdown has finished;
  - at least one sleeper.

  `WorldView.AX.LaunchIssues` reports on the primary ship: the biggest one with a reactor. `ShipID`, `Countdown` and `Escaped` are new World fields, approved by the lead.
- **Mind core.** The computer core's blueprint takes `item.MindCore` as a secondary material, delivered by A4's extras. Its source is the curio merchant, the orbital trader, which sometimes stocks one (`domain/economy/trade` pool: 1 at 15 %, original tuning; wiki: trade or quests).
- **Reactor.**
  - It gives 1000 W at all times (wiki), both in `power/ax.go` and in the game's `genOutput`.
  - StartReactor starts the 15-day countdown at once: the wiki says no colonist is needed, and it can start before the rest of the ship is built. So `job.StartReactor` was renamed `job.BoardShip`, keeping the same number.
  - Countdown raids go through the storyteller (`storyRaid` at its threat points, or `scaledRaid` on a legacy colony). People and mechanoids come by turns (`MechRaid`). The difficulty's no-big-threats setting is honoured.
  - Spacing is original tuning (the wiki only says "constant"): the first raid comes half a day after the start, then one every day plus up to half a day, hashed from the seed, which makes about 10 raids. The spacing is a cooldown and is no-flap tested.
  - Raids stop when the countdown ends or the reactor is gone. A destroyed reactor loses its waking.
  - Two letters: "waking" (red) and "ready".
- **Boarding.**
  - A colonist boards through the casket's right-click order "Board the ship stasis casket" (`command.BoardShip` → `job.BoardShip`). It is stashed off the map with world_stash (holder = the casket), so its needs are frozen.
  - A downed pawn is carried in through game/travel's flow, with no second copy. The right-click row on the downed pawn is "Carry <name> to ship stasis casket": `command.EnterCasket` with `Worker`, then `job.CarryToCasket`. A9 added the travel `Ships` role. The game's `travelHost.ShipCasketFree` and `LayInShip`, in story.go and owned by A8, delegate to `ship.State.CasketFree` and `LayIn`. Prisoners and animals follow A9's stash rule, the same rule as the stasis casket.
  - The pane's Wake sleeper button (`command.LeaveShip`) brings the sleeper out beside the casket. A woken sleeper gets the wiki's cryptosleep sickness (`cryptoSicken`, shared with A9).
- **Connectivity overlay.** Selecting a ship part sets `overlay.Set.ShipSel`, its ship id, in architect_compat.go `syncPowerForce`. `DrawShipLinks` then tints every part of that ship, so a part left out stands plain.
  - If a casket is destroyed, its sleeper wakes where it stood.
- **Launch.**
  - LaunchShip is refused with the report in a message until nothing is missing.
  - On launch, the ship's parts are lifted off the map. The sleepers leave and stay stashed for good.
  - The colony gets the letter "The ship has launched", posted at the ship's centre, and `WorldView.AX.ShipLaunched`, `Ending` ("ship") and `Escaped` are set.
  - The save header gains `SaveInfo.Ending`, and the Load list tags the save "Complete: escaped by ship".
  - The game keeps running so the player can keep watching.
- **UI (MAIN_UI §32, §37, §38).**
  - **Panes** (`inspector/ax8.go`): part, reactor and casket lines and the launch report. Start reactor sits behind a warning about the raids. Launch ship has a confirmation naming who leaves, and it is disabled with the first missing item. There is also Wake sleeper.
  - **Alerts** (`hud/ending.go`): "Ship reactor waking: Nd Nh" and "Ship ready to launch".
  - **Ending screen:** original prose, plus who escaped and who stayed, the colony's days and its research. It opens once a session and offers Keep watching or Main menu (`ports.ReturnToMenu`).
- **Art (AR4's keys).**
  - `render/world/ship_keys.go` holds the part sprites by facing and the occupied casket.
  - The reactor glows once started (`overlay.poweredLook`).
  - `overlay/ship_draw.go` draws the other parts' indicator glows, the casket's glow while it is occupied, and the launch flare, which grows and fades for 240 ticks.

## Code
- New `internal/application/game/ship`: State, graph, report, reactor, casket, launch, apply, view, save and tests.
  - The adapter `shipHost`, `shipOf` and `DevWakeShip(g, ticks)` live at the end of `story.go`. `DevWakeShip` is a free function, so Game gains no method.
  - The state is `worldState.ship`, built on first use. It is saved in the optional `saveState.AX8 *ship.Save` (nil when nothing has started), with no version bump.
- Contract edits, approved by the lead:
  - BoardShip and LeaveShip are appended at the end of `command.Kind`, after A9's, so no number moves.
  - `job.StartReactor` is renamed `job.BoardShip`.
  - `ShipReactor` is a producer(1000) in `power/ax.go` and its test.
  - The `ax_hp` comment now says the 500 HP is the wiki figure.
- Moves paying for the game lines:
  - the trade offer codec → `trade.EncodeOffer`/`DecodeOffer` (`domain/economy/trade/offer.go`);
  - `operationDone` → `body.Operation.DoneText`;
  - `stockAdd` → `trade.(*Stock).Add`.

  The game package's net from A8 is about -5 lines and 0 Game methods or fields. The follow-up (the delegates) is paid inside story.go and save.go: Save, Load and Info now go through `worldS()` and `shipOf()`, and an empty world state saves nothing.

## Tests
- `game/ship/ship_test.go`: joins, report, raid spacing (no flap), a destroyed reactor, the launch, waking and fallen caskets, view and save, Rewind, Duration.
- `game/ax8_test.go`: parts, report and power; board → countdown raid → launch → save round trip; waking a sleeper.
- `ebiten/journey_ax8_test.go`:
  - `TestJourneyAX8EscapeShip`: every tool is laid from the Architect; selecting a part highlights the ship; board, wake, a downed colonist carried into a second casket, board again; the reactor starts behind its warning; a countdown raid comes and is beaten (dev hooks); the alerts follow; the launch goes through its confirmation; the ending screen and flare show; the save is marked.
  - `TestAX8ShipArt`: footprints and glows.
- Determinism gate: no golden moved.

## Not done (noted in the gap and checklist rows)
- There is no dedicated §37 screen: the pane carries it.
- No quest offer or faction request sources the mind core.
- The ending summary has no deaths, relationships, wealth or credits.
- There is no take-off animation beyond the flare.
- The ship reactor isn't refused under a roof.
- Visual in-game review: the footprint and glow checks pass, but no screenshot was reviewed.
