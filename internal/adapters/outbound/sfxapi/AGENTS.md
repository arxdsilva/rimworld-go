# internal/adapters/outbound/sfxapi

## Purpose
The ElevenLabs sound-effects HTTP client for `cli/sfxgen` (moved from `cmd/sfxgen/client.go`, 2026-11, GAR): sound generation, the subscription (credit) read, retries on 429/5xx with doubling backoff.

## Owns
`Client`, `Subscription`, `FatalError` (a refusal no retry can fix).

## Contracts
The key is redacted from every error and never logged. Tested through `cli/sfxgen` against an `httptest` fake; never call the real API from tests.
