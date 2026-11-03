# USAGE-001: usage data client

Asked 2026-11-03: collect player usage data and send it to a Postgres table behind arxdsilva.com. Basic data is always on, detailed data has an Options switch and defaults on.

## What changed
- Adapter: `internal/adapters/outbound/usagehttp`. `Tracker` turns activity into events; `sender` is a bounded queue and one goroutine. Stdlib only.
- UI: consumer-owned `ebiten.Usage` and `noUsage` in `app.go`, `AppConfig.Usage`. `App.Update` calls `Frame`; `finishStart` and `optionsSaver.Load` call `reportColony`.
- Settings: `Settings.Telemetry{Advanced, InstallID}`, `SettingsVersion` 7.
- Options: `usagedata` row on the Gameplay tab.
- Startup: `cmd/game/usage.go` builds the tracker before `NewApp` so the App loads the install ID, and `main` calls `Close` before `log.Fatal`.
- Schema: `migrations/0001_game_events.sql` in ytwebsite, next to the server, `gameevents/`.
- README: "What data the game sends". Any new event must be added there in the same change.

## Decisions
- No new root files, `App` fields or `App` methods: the root package is at 60 files and `App` is at its ceilings (`solid_budget.txt`), so all state lives in the adapter.
- `Frame` returns "snapshot due" instead of taking a callback, so the per-update call allocates nothing.
- Dev builds, captures, the frame-rate probe, `-scenario` and graphics overrides send nothing. `RIMWORLD_USAGE_ENDPOINT` overrides all of that for local testing.
- A failed request is silent (user decision, same day): no log line and no player-visible sign. Events stay queued and go with the next attempt that works; the package doesn't import `log`. `TestFailedRequestsAreSilent` guards it.
- No first-launch notice: the player sees the data policy only in the README and the option's tip. Revisit if EU players are a concern, since an install ID is likely personal data under GDPR.

## Tests
- `usagehttp_test.go`: batching, retries keep event IDs, refused batches are dropped, backoff, queue overflow, `Close` bound, tier filtering.
- `cmd/game/usage_test.go`: endpoint choice, install ID made once, unusable settings file means no data.
- `settings_test.go`, `background_test.go`: defaults, migration, the option row, `Frame` following the switch.
