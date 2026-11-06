# OPT-1-misc: wizard, main menu, Architect and developer-tool options

Hash impact: none (presentation only; the Setup carries the final names).

## Done
- OPT-032 preferred names: `wizard/preferred.go`. `General.PreferredNames` go to the rolled candidates in roster order (`First`, `Nick Last`, `First 'Nick' Last`); a name the pool refuses (nickname taken, bad characters) is skipped. The pool key includes the list, so editing it rebuilds the pool. Same seed + list gives the same result.
- OPT-031 test map sizes: `wizard.MapSizeOptionsFor(settings)` leads the list with 60, 80 and 100 (labelled "(test)") when `Gameplay.DevMode` and `Developer.TestMapSizes` are on. `game.MapSizes` is unchanged (not owned); they are never the default.
- OPT-015 menu background: `mainmenu.Screen.Background()`. diorama = today's look; splash = hero art on the gradient (no diorama); random settles once per launch from `Seeder.MenuSeed()` (an optional shell interface; `menuHost.MenuSeed` uses the presentation clock). Without a seed random = diorama. The wizard and pause backdrops still use the diorama.
- OPT-025 remembered draw styles: `architect/style_memory.go` (`styleMemory` held by `Tool.styles`, no new Tool methods; `StyleMemory` optional shell interface, `architectHost.RememberStyles`).
- OPT-030 developer log: `devtools/logopts.go`. Verbose log (40 messages with position and danger flag), open on warning, pause on error (via `SetSpeed`), `Panel.Escape()`. Warning and error are both messages with `Danger` (the dev log has no other severity). A first Update catches up so old messages do not trigger.

## Open for the lead
- Escape: the root's `handleColonyEscape` must call `a.tools.Escape()` first (root file, not mine).
- Options screen rows for these settings are the options owner's.

## OPT-033 (report only)
- `Gameplay.RunOnStart` and `BirthdayLetters` (`config/settings.go:154-155`) are read nowhere in non-test Go code; `run_on_start` is only round-tripped in `settings_test.go`.
- RunOnStart: wiring = start the colony unpaused (`PauseOnLoad` is the sibling for loads); read it where a new game's `startSession` sets the speed (root). Delete if `General.PauseOnLoad` covers it.
- BirthdayLetters: `skills.go:524` has a comment "birthday (letter: optional, off; not shown)". Wiring = a `Letter` message in `skillsDaily` when a pawn's age year ticks over, gated by a Game setting passed in through Config (presentation setting must not reach the sim without a command; a client-side letter from the pawn's `Age` change in the view is the hash-safe route). Otherwise delete both fields.
