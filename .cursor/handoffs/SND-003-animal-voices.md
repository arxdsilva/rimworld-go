# Task Handoff

Task ID: SND-003
Status: triggers, engine and placeholder voices complete. The ElevenLabs species voices are blocked: `ELEVENLABS_API_KEY` is not set in the agent's shell.

## Summary
All 61 species have voices: idle calls while wandering, a flee cry, attack, hurt, death, and a "whump" for explosive deaths. Until the generated species clips are approved, they play procedural placeholders: 9 voice groups plus a generic set (CC0, code-made). The ElevenLabs generator, processing and review flow are built and tested offline, with 281 prompts ready.

## Files changed
- **Engine** (`internal/adapters/outbound/ebitenaudio`): OGG one-shots are decoded once into a 24 MiB PCM LRU (`oggcache.go`) and panned like WAVs. Loops and music are unchanged.
- **Placeholders** (`cmd/audiogen/animal_recipes.go`, `-animals`): 81 OGG files, 0.8 MiB (`oggenc -q 3`, seeded).
- **Triggers** (`animal_sounds.go`, the `traceSound` animal branch, the cue in `animal_burst.go`):
  - **Timing:** the first call comes after `600 + hash%3000` frames, then every `1500 + hash(id, n)%2700`.
  - **Interval scale:** pets and birds ×0.7, size ≥ 1.4 ×1.3, insects ×0.5.
  - **Clock:** the timer runs at game speed and stops while paused.
  - **Caps:** at most 2 calls per second and 3 species per 10 s; extra calls are deferred.
  - **Volumes:** call 0.7, attack and hurt 0.85, die and burst 1.0; the melee hit plays under them at 0.6.
  - **Gating:** on screen only; a death sounds only when a corpse lies there.
- **Generator** (`cmd/sfxgen`, stdlib only): dry-run, budget guard from the API, a ledger of every take, 429/5xx backoff, the key read from the env only, then `-process` (ffmpeg + oggenc: trim, loudness, ±4% pitch variant), `-review` (HTML page), and `-approve` / `-reject` / `-retry`. Coverage 86.6%.
- **Prompts:** `assets/source/audio/animal_prompts.json` has 281 rows: 61 × 4, the 9 groups and generic × 4, and the burst. They're written in our own words; originals are never named after RimWorld creatures (`TestPromptsCoverRoster`).
- **`audiogen -check`:** OGG decodes at 48 kHz, every roster key's four events resolve through the fallback chain, and the files stay within the 20 MiB budget (11 MiB now).
- **Rules:** `.cursor/rules/creatures.mdc` and skill `rimworld-go-creatures`: every new creature gets the same setup, including its four generated sounds.

## Remaining (needs the key, and user approval at each stop)
1. `export ELEVENLABS_API_KEY=...` in the shell that starts the agent.
2. **Measure:** `go run ./cmd/sfxgen -only animal.cow.hurt -seconds 1 -budget-from-api` (one 1 s sketch, to read the real credit cost).
3. **Pilot:** cow, timber_wolf, chicken, elephant, cat and hive_crawler × 4 events, about 300 credits. Then `make animal-audio`, `sfxgen -review`, and the user listens.
4. **Full run:** the 9 voice-group batches (about 3,330 credits estimated in total), each checked against the 1,500 reserve, plus at most one retry per rejected clip.
5. Run `sfxgen -approve <ids>` and `make audio-check`. Add the "Animal voices" line to `THIRD_PARTY_NOTICES.md` (ElevenLabs free tier: non-commercial, credit ElevenLabs).

## Invariants
- The key is never printed, logged or committed. Every paid take is appended to `assets/source/audio/elevenlabs/ledger.json`.
- Sound is presentation-only: no sim state, no audio in captures or tests.
- A missing id is silent; the fallback chain is species, then group, then generic.
