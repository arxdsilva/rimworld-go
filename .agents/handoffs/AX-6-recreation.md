# AX-6: recreation

AX wave 2, 2026-11-02, agent A6. Every A6 kind is live: `construction.plannedA6` is now empty. ENG-003 applies: the numbers are the wiki's (`scratchpad/wiki-ax.md`, Recreation page), the names original (ring toss, ur board, big screen television).

## Behaviour
- **game/recreate** (new sub-package, `deps.go` Host roles; the game passes its rest adapters plus `recHost`, rest_host.go). Its `State` hangs off `socialState.rec` (no new Game field), made by `recOf`; the views use `recView`, which never makes it. Saved as `saveState.Recreation` (optional; nil when no colonist has a tolerance).
- **Pieces** (`SpecOf`, wiki power, users, joy kind, skill):
  - horseshoe pin (dexterity 1.0, 8 throwing spots 3 tiles off, Shooting), ring toss (dexterity 1.0, 3 throwers, Shooting);
  - ur board (cerebral 0.8, 2), chess table (cerebral 1.0, 2), poker table (cerebral 1.3, 4), all Intellectual; billiards table (dexterity 1.3, 2, Shooting). Players stand or sit on the ring of cells around the piece (`construction.RingCells`); a chair there is taken first. Quality scales their power (`quality.Recreation`, 0.76..1.8; `craft.QualityBuilding` lists them).
  - tube, flatscreen and big screen televisions (television 1.2/1.4/1.6) are watched from their watch area in front of the screen (`construction.WatchCells`, 3x5, 5x6, 7x6; first row beside the set), seats first, only while powered; the watcher faces the screen (`Acting`'s cell).
  - telescope (telescope 1.2, 1 user, Intellectual) from its spot, only with no roof over it and at night: starts at daylight 250 permille or darker, holds on till 400 (hysteresis, original tuning).
  - Gain: 36 %/h x power (`recreation.HourGain`), milli-points carried per colonist. Skills: 1 XP every 250 ticks (wiki telescope 0.004/tick; the others take the same rate, original). A session ends at full recreation.
- **Tolerance and variety** (`recreation.Tolerance`, domain): each step's gain is cut by the kind's tolerance, and the tolerance rises by 2/3 of what was gained; it wears off 18/13/11/10/8/7 % a day by colony wealth (wiki expectations; wealth read once a day). Bored above 50 % until below 30 %. `Seek` skips bored kinds and scores each piece by distance plus the kind's tolerance percent, minus 5 for joining a game in progress (original tuning). The free activities (wander, chat, relax, fireside) keep no tolerance: their gains stay below a day's drain.
- **No-flap.** A session cut short (screen unpowered, sky light, piece gone) makes the colonist seek no piece for an hour (`RetryTicks`); `TestTelevisionNoFlap`, `TestTelescopeAtNightOutdoors`, `TestBoredHysteresis`.
- **Social games.** A colonist playing a game counts as chatting (`social.go chatting`), so co-players interact sooner.
- **Sources of the found pieces.** Big screen television and telescope are `Unbuildable`: map gen leaves one in a ruin shed (`features.placeAntiques`, own stream `ruin_antiques`; 1 shed in 4 a screen, 1 in 4 a telescope, original tuning) as Ruin pieces; A1's Claim makes them the colony's. The wiki sources (shrines, traders, quests) are stood in for by ruins.
- Room tally counts every recreation piece (`Kind.Recreation`).

## UI
- Architect: the seven buildable cards and the two locked found-only cards (`UnbuildableReason`). Art and icons are AR3's (`build_keys.go`: furniture/ring_toss, ur_board, chess_table, poker_table, billiards_table_<axis>, <tv>_<facing> with glow layers drawn while powered, telescope_<facing>).
- Pane (`inspector/ax6.go`): "Recreation  <joy kind>" and "In use  N colonists" / "Free".
- Needs tab: "Enjoying  <kind>" and "Tolerance  cerebral 42% (bored), ..." under the mood line (`PawnView.AX.Joy`, `AX.Tolerance`, new `axview.JoyTol`, lead-approved).
- Acting: "play chess", "watch television", "stargaze" toward the piece.

## Budget
- Game package: about -5 lines, -1 Game method. Moves: `idleHorseshoes` (generalised into `recreate.Seek`) and `activityOf` (`recreate.ActivityOf`) from idle.go, the recreation thought switch from rooms.go (`recreate.Mood`), the passion drain switch from skills.go (`recreate.PassionDrain`).

## Tests
- recreate/recreate_test.go (fake host): wiki hour gains with quality, variety pick, bored hysteresis, decay by wealth, TV no-flap, telescope night/roof, poker seats and chairs, watch cells per facing.
- game/ax6_test.go: chess end to end with views, armchair watching and power, tolerance save/load and hash, found pieces in ruins and no blueprint.
- mapgen features/antiques_test.go; journey_ax6_test.go (cards, locked cards, playing chess with the pane and Needs rows).

## Open
- Trader stock for crated buildings (big screen, telescope, urn) needs trade lines that carry a building kind.
- Chemical and food joy are not recreation kinds; reading (A7) can feed its gain through `State.Gain(id, recreation.JoyReading, milli)` to share the tolerance.
- Tolerance wears off while asleep too (vanilla: awake only).
