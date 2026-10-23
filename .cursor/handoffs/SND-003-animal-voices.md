# Task Handoff

Task ID: SND-003
Status: complete. All 281 ElevenLabs animal voices (61 species × 4, the 9 voice groups × 4, the hive burst) were generated, reviewed by ear and approved on 2026-10-23. Only `animal.generic.*` stays a code-made placeholder, as the last fallback.

## Summary
All 61 species have their own voices: idle calls while wandering, a flee cry, attack, hurt, death, and a "whump" for explosive deaths. The fallback chain (species, then voice group, then generic) still covers any id that goes missing; the generic set is code-made (CC0).

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

## Generation run (2026-10-23)
- **Spent:** 3,349 credits on the free tier. 3,331 for the 281 clips at the measured 10 credits a second; 10 for a 1 s cost sketch (ledger id `cost_sketch.animal_1s`); 8 for an unneeded retry of `animal.tortoise.attack`. Take 1 of that clip was wrongly rejected as silent; it was a processing bug (below), and take 2 is the approved one. The API counter matched the estimates exactly; ledger total 3,949 including last month's 600.
- **Review:** the user listened to every clip on the `sfxgen -review` page and approved all 281 (`reviewed_by: arxdsilva` in `audio.json`); no rejects.
- **Audio budget:** `make audio-check` reports 325 sounds, 18.3 MB of 20 MB. The 73 group and burst placeholder files nothing plays any more were deleted.
- **Fixes found during the run** (each with a test that failed first):
  - `sfxgen` charged lagged counter jumps on top of estimates already logged for the same takes (3,993 logged against about 2,070 spent), which stopped the first run early at its cap. It now logs `max(estimate, counter movement this run - credits logged this run)`, and the affected ledger entries were corrected (`TestLaggingCounterNotCountedTwice`).
  - Clips trimmed below 0.4 s read as `-inf` LUFS, because loudnorm measures 400 ms blocks. They are padded with silence for measuring and cut back after (`TestProcessShortClip`).
  - `audiogen -animals` would have put the code-made placeholders back over approved ElevenLabs group and burst takes. It now skips any id whose entry is from ElevenLabs (`TestAnimalsKeepApprovedTakes`).
- Coverage: `cmd/sfxgen` 87.1%; `cmd/audiogen` 69.1% (68.8% before; the gap is the older music and effect renderers, not this change).

## Follow-ups
- License: free-tier output is non-commercial and needs attribution (`THIRD_PARTY_NOTICES.md`). Regenerate on a paid plan before any commercial release.
- New creatures follow skill `rimworld-go-creatures`: four prompts, then `sfxgen -dry-run -only animal.<key>.*`, user-approved spending, review, and `-approve`.

## Invariants
- The key is never printed, logged or committed. Every paid take is appended to `assets/source/audio/elevenlabs/ledger.json`.
- Sound is presentation-only: no sim state, no audio in captures or tests.
- A missing id is silent; the fallback chain is species, then group, then generic.
