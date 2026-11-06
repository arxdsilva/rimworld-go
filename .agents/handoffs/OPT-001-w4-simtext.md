# OPT-001 Wave 4 (W4-SIMTEXT): the simulation's messages and letters carry their format

Every message and letter the simulation posts now carries `Msg` (`msgfmt.Text`: the catalog key and args), and `Msg.String()` is the posted `Text` byte for byte. The ebiten client (W1-LANG) translates `Msg`; English is unchanged.

## What changed
- `climate.go`: `message` and `threat` post `Msg: msgfmt.Of(format, args...)`. `post` fills `Text` from `Msg` when only `Msg` is given, keys a `Text`-only view on that finished text (`msgfmt.Plain`), and returns the text.
- New format-taking letters, free functions (no new `Game` methods): `letterf(g, kind, title, focus, format, args...)` at the colony (`world_state.go`) and `pawnLetterf(g, kind, title, focus, format, args...)` at a creature (`social_romance.go`). Both return the text, so callers that also log history post it once. `g.letter(kind, title, text, focus)` is now `letterf` with `msgfmt.Plain(text)`. `romanceLetter` and `pawnLetter` are gone (2 `Game` methods fewer).
- Converted root sites: every `fmt.Sprintf` or concatenation passed to `letter` (15 callers), the direct `post` sites with a format (manhunter, bonded grief, hunted, solar flare, heart attack, carcinoma, social fight, recruited, disease), `notice`, research's bench message, and the romance letters.
- Sub-package hosts take `format string, args ...any`: `Message` (aqua, care, combat, craft, herd, orders, prison, raid, ship, travel, visit), `Letter` (maps, travel, industry, ship, visit), `Note` and `DemandLetter` (maps), `Threat` (raid), `Note` and `GoodLetter` (gather). Their roughly 110 callers pass the literal format at the call site (`fmt.Sprintf(lit, a...)` became `lit, a...`, and concatenations became `%s` formats). Test fakes format with `fmt.Sprintf`.
- `letters.go`: `savedLetter.Msg`, copied by `addLetter` and `letterViews`. The save schema was refreshed.
- `plant_growth.go`: a merged frost notice also rewrites its `Msg`.
- Root package: 52707 lines (2 under the 52709 ceiling), 174 files.

## Tests (`msgtext_test.go`)
- `TestPostsCarryTheirFormat`: a 64² colony fires every incident that fits, takes a raid and runs 3000 ticks. Every message and letter is checked after each step: no empty `Msg`, `Msg.String() == Text`, no `%!`.
- `TestLettersKeepMsgThroughSave`: letter keys and args survive Save/Load.
- `TestPostFormatsAreLiterals`: a source scan of every posting call in the root and the posting hosts of 14 sub-packages. Each format must be a literal, literals joined with `+`, a package constant, `msgfmt.Plain(...)` or a forwarded format parameter. The arg count must equal the verb count, which stands in for vet's printf check: `msgfmt.Of` formats now, not `fmt.Sprintf`. One listed exception: `heat.go` `how`, one of two literal formats chosen just above the call.

## Hash
The state hash covers saved messages and letters, so it moves only because `Msg` is saved: HEAD `0x1d6a9a68fab5b72 / 0x1c16287300ee327f` becomes `0xc92ea94d4d3884d / 0x349e81cd0f22a29e` (`bin/benchmark -hash -sizes 120 -speeds 1 -ticks 2000`). Built again with `Msg` blanked inside `normalisedFields` (a temporary local patch, not committed), the hash equals HEAD exactly, so `Text` and all other state are identical. With the same patch, `TestScaleGolden`, `TestHabitsOffKeepTheHash` and `TestGrownGolden` all pass on their old pins. I re-pinned the first two in `game`, with the reason. `TestGrownGolden` (scenario: `0x847a4f202c0f89ba`, stable over -count=3) is left for the lead.
- Also updated: `letters_test.go`, whose two wants now include the letters' `Msg`.

## Kept as finished text (Plain, or a `"%s"` format whose one arg is the text)
- Text built by a sub-package or domain helper: mental breaks (`mentalbreak` defs), `chaseText`, animal death text, `farm.KillText`, `incident.ClimateEndText`, aqua's `diedText`/`dyingText`, craft `condition` text, intro letter (scenario data), vault constants, `research/lab` letters, ship/story letters given text, gather's `note` and `RomanceLetter` (wedding/marriage text), `RelativeLetter`.
- Text also written to a log or the history: raid mischief/tactics steals and arson, the siege and storm threats (`raid/raiders.go`, `raid/stage.go`), `visit/turn.go` (a suffix when the trader turns too), `maps/settle.go` "Settlement destroyed" (optional suffix), the `why` refusal notes in maps.
- `world_stash`-style joins take the joined names as one arg (`%s came across…`).

## Gaps
- No new screen: the representation is the existing HUD messages and letters, which W1-LANG draws through the catalog.
- The extractor must also follow `pawnLetterf`, `notice`, the host methods above and package constants (`caveMessage`, `casketEmptyText`, `orders.ReasonDefenders`).
