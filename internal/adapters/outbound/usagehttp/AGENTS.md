# internal/adapters/outbound/usagehttp

## Purpose
Sends usage data to the website, `POST https://arxdsilva.com/api/rimworld-go/v1/events`. The server is `gameevents/` in the ytwebsite repo, and the table's migration is `migrations/0001_game_events.sql` there. Tells the project how many people play, on what, and how far they get, without slowing the game or touching the simulation.

## Owns
- `Tracker`, `New(Config)`: turns game activity into events. It implements the UI's consumer-owned `ebiten.Usage`.
- `sender` (unexported): the bounded queue and the one goroutine that posts it.
- `Endpoint`, `NewID`, `ValidID`.

## Depends on
`github.com/google/uuid`. Its types stay in this package; IDs cross as strings.

## Must not depend on
Domain, application, ports and inbound adapter packages. Values arrive as plain arguments.

## Contracts
- Basic events (`session_start`, `session_heartbeat`, `session_end`) are always sent. Advanced events (`hardware`, `colony_started`, `colony_snapshot`) only while `Settings.Telemetry.Advanced` is on. Turning it off drops what is queued.
- Never send anything the player named (colony, pawn, scenario), any file path, seed or save data. Document any new event in the README's "What data the game sends" section in the same change.
- `Frame`, `Colony`, `Snapshot` and `Close` run on the game loop. They never wait on the network, except `Close`, which spends at most 2 s.
- A retry resends the same event IDs; the server ignores duplicates.
- 2xx: done. 429, 5xx and network errors: keep and back off, up to 15 minutes, honouring `Retry-After`. Any other status: drop the batch. Redirects are not followed.
- A failed request is silent: nothing is logged or shown, and nothing can stop the game. The events wait in the queue for the next attempt that works. This package does not import `log`.
- The queue holds 256 events and drops the oldest. Nothing is written to disk.
- Off for version `dev`, captures, the frame-rate probe and developer overrides (`cmd/game/usage.go`). `RIMWORLD_USAGE_ENDPOINT` points a build at another server, and also enables `dev` builds, for local testing.
