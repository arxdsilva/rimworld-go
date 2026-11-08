# internal/adapters/inbound/cli/sfxgen

## Purpose
The `sfxgen` command (SND-003) moved out of `cmd/sfxgen` (2026-11, GAR): flags, plan, generation (MOSS local by default, ElevenLabs paid only with approved spending), the ledger, processing to Ogg, the review page and approve/reject. `cmd/sfxgen/main.go` only calls `Run`.

## Depends on
`internal/adapters/outbound/sfxapi` for the ElevenLabs HTTP calls. The MOSS runner script is `assets/source/tools/moss/generate.py` (the `-moss-runner` default, relative to the repo root).

## Must not depend on
Real network or paid APIs in tests: they use an `httptest` fake (`helpers_test.go`).

## Contracts
The API key comes only from `ELEVENLABS_API_KEY` and is never printed (`generate_test.go`). Flags are unchanged for `make` targets and `scripts/audio_import.sh`.

## Tests
`go test -race ./internal/adapters/inbound/cli/sfxgen`. Tests read `../../../../../assets`.
