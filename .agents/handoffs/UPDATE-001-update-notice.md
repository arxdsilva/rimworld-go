# UPDATE-001: running version and update notice

Asked 2026-11-03: the client should show the running version and tell the player when a newer one can be downloaded. Built on top of the uncommitted USAGE-001 work.

## What changed
- Adapter: `internal/adapters/outbound/updatecheck`. `Start` makes one `GET` to GitHub's latest-release endpoint from one goroutine. `Newer` compares versions. Stdlib only.
- Menu: `mainmenu/update_notice.go` has the consumer-owned `Updates`, the card, `VersionLine`. `noticeButton` is shared with the RAM card, which moved to `declareRAMNotice`.
- Pause menu: the version sits at the panel's top right, with the newer one in brackets when known.
- Root: `AppConfig.Updates`, `noUpdates`, two `menuHost` methods. No new root file, `App` field or `App` method.
- Startup: `cmd/game/update.go`.
- README: the check is described under "What data the game sends".

## Decisions
- GitHub, not the website: `/releases/latest` already skips drafts and prereleases, and release builds need no new server.
- The browser only opens a constant URL, so nothing from the network reaches `exec`. Offered tags must parse as `vX.Y.Z`.
- The card shows the page's address, so a player whose browser can't be opened still has it.
- Later lasts until the next launch; nothing is saved, so no settings change.
- Checked once per launch, no retry. A player in a long session hears about a release at the next launch, or on the pause menu if it was found after they left the menu.
- A failed check is silent, like usage data.
- `mainmenu.Updates` is its own interface because `Host` is at the 8-method limit.

## Tests
- `updatecheck_test.go`: version comparison, failed checks stay quiet, nothing offered before the answer, the fixed page, per-OS browser commands.
- `journey_flow_test.go`: `TestJourneyFlowUpdateNotice`, `TestJourneyFlowNoUpdateNoNotice`, `TestJourneyFlowPauseShowsTheVersion`, `TestUpdateNoticeClearsTheRailAndTheRAMCard`.
- `mainmenu/screen_test.go`, `cmd/game/update_test.go`.

## UI impact
Main menu: a new card on the left, drawn only when a newer release is known. Pause menu: a version line.
