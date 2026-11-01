# Task Handoff

Task ID: SND-004
Status: done (2026-10-31). The user still has to listen to the 16 mech voices and the A/B page.

## Summary
The user decided to "switch creature voices to MOSS if it makes them better". `cmd/sfxgen` now has `-backend moss` (the default): it runs the open-source MOSS-SoundEffect v2.0 model locally and processes each take straight away. `-backend elevenlabs` keeps the paid path and its spending rules. The four mechanoids (harrow, spindle, bulwark, quill) now have their own 16 MOSS voices and their own `mech` voice group, replacing the insect stand-in.

## Files changed
- `cmd/sfxgen/moss.go` (new) adds the backend:
  - Command line: `<venv python> -I cmd/sfxgen/moss/generate.py --model … --prompt … --seconds … --seed … --steps 100 --cfg 4 --device auto --out <raw>.wav`.
  - Seeds: a stable hash of the id and take number, or `-moss-seed`.
  - Memory gate: before each take it waits until macOS `vm_stat` shows 24 GB free plus inactive (`-moss-min-free-gb`, `-moss-poll`, `-moss-max-wait`). Takes run one at a time.
  - Preflight checks and a dry run that prints the plan, the time it will take and the command line.
- `cmd/sfxgen/moss/generate.py` (new) is the runner. It turns off torch dynamo (MPS cannot compile the model's float64 kernels), applies the MPS float64 and complex128 patches at runtime and writes a 24-bit WAV.
- Paths: the defaults are `$XDG_CACHE_HOME`, or `~/.cache/rimworld-go/moss/{venv,MOSS-SoundEffect-v2.0}`. They can be overridden with `-moss-python`/`-moss-model` or `$SFXGEN_MOSS_PYTHON`/`$SFXGEN_MOSS_MODEL`. No model, venv or cache files are in the repo, and `assets/source/audio/moss/raw/` is gitignored.
- A take that fails processing (for example a silent take) is still logged. The run goes on to the next sound and exits with an error that names the failed ids, so the user can `-reject` them and run again; the A/B batch hit this with a silent 0.6 s wolf hurt.
- Ledger:
  - Every take now records `backend`; all 296 older takes were backfilled with `elevenlabs`.
  - MOSS takes also record `seed`.
  - A free take has no retry cap: a rejected MOSS sound is simply made again.
- `-approve` writes `source: moss`, the tool, the seed and the Apache-2.0 license line. A generated entry (ElevenLabs or MOSS) can replace another without `-replace`; for other entries `-replace` is still needed (`audiomanifest.Generated`, `SourceMOSS`). The review page has a backend column.
- `cmd/audiogen -animals` no longer overwrites approved MOSS takes.
- `render/cue/animal_sounds.go` maps the four mechs to `mech`. That group has no group sounds, so a missing mech sound falls to generic. The test's allowed groups now include `mech`.
- Tests: `cmd/sfxgen/moss_test.go` covers:
  - backend selection, defaults, env and flag precedence;
  - the exact command line;
  - the dry run and preflight;
  - generate, process and approve, using a fake runner;
  - the retry rules, the memory wait and `vm_stat` parsing;
  - ledger paths.
- Docs: AGENTS.md (creature rule), skill `rimworld-go-creatures`, `.cursor/rules/creatures.mdc`, ART §25 audio, THIRD_PARTY_NOTICES, the DEFENCE and COMBAT gap analyses and the vanilla checklist rows.

## Mech voices
- 11 takes come from the 2026-10-29 trial; 5 were regenerated on 2026-10-31 with tighter, denser prompts and new seeds. The table lists the integrated loudness of each final clip:

  | Clip | Before | After |
  |---|---|---|
  | `harrow.hurt` | −29.7 LUFS (target −18) | −18.0 LUFS, 1.0 s |
  | `harrow.die` | −23.9 LUFS (target −19) | −19.0 LUFS |
  | `spindle.hurt` | trimmed to 0.5 s | 1.0 s at −20.2 LUFS (peak-limited) |
  | `quill.hurt` | −28.0 LUFS | −18.0 LUFS |
  | `quill.die` | −23.2 LUFS | −19.0 LUFS |

- The prompt rows in `animal_prompts.json` now carry the MOSS prompt cores, with lengths inside the A3 ranges. The ledger keeps the exact prompt, the seed and the requested length (the 11 trial takes asked for 2–3 s).
- Approved with `reviewed_by: "agent V (metrics checked; user to listen)"`. Nobody has listened yet. The user can re-approve them as `arxdsilva` or `-reject` any. Review page: `sfxgen -review -only 'animal.harrow.*,animal.spindle.*,animal.bulwark.*,animal.quill.*' -o page.html`.
- `make audio-check`: 403 sounds, 24.5 MB of 30 MB.

## Follow-ups
- A/B page (scratchpad, not committed): MOSS against ElevenLabs for the timber wolf, the chicken and the hive crawler, four voices each. No ElevenLabs voice was replaced; the user decides from that page whether to regenerate the rest.
- The trial mech takes run longer than the A3 lengths. Regenerating them at the row lengths is free if the user wants shorter clips.
- `oggenc` (Homebrew `vorbis-tools`) was installed on this machine; `-process` needs it.

## Invariants
- No API key or model file in the repo. Every take is in the ledger with its backend.
- Sound is presentation only.
