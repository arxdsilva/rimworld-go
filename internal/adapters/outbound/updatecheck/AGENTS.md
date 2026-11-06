# internal/adapters/outbound/updatecheck

## Purpose
Asks GitHub whether a newer release exists, `GET https://api.github.com/repos/arxdsilva/rimworld-go/releases/latest`, and opens the download page. Lets a player who downloaded a release find out there is a newer one; it never downloads or installs anything.

## Owns
- `Checker`, `Start(endpoint, current)`: the one check. It implements the UI's consumer-owned `mainmenu.Updates`.
- `Newer(latest, current)`: version comparison.
- `Endpoint`.

## Depends on
`golang.org/x/mod/semver` and `github.com/cli/browser`. Their types stay in this package.

## Must not depend on
Domain, application, ports and inbound adapter packages.

## Contracts
- One request per run, from one goroutine that ends with it, within 10 s. No retry: the next launch asks again.
- The request carries nothing but the game version in its `User-Agent`. It has no install ID, and `usagehttp`'s Options switch doesn't affect it. Document any change in the README's "What data the game sends" section.
- A failed check is silent: nothing is logged or shown, and no notice appears.
- `UpdateAvailable` and `DownloadUpdate` run on the game loop and never wait on the network or the browser.
- Only a tag that parses as a full release (`vX.Y.Z`) is offered. Anything else, including `dev`, is neither newer nor older.
- The browser only ever opens the constant `pageURL`. Nothing read from the network reaches the browser.
- Off for version `dev`, captures, the frame-rate probe and developer overrides (`cmd/game/update.go`).
