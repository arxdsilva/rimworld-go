// Package sfxgen implements the sfxgen command (cmd/sfxgen only calls Run).
//
// It makes the animal voices (SND-003): it turns the prompts in
// assets/source/audio/animal_prompts.json into takes, logs every take in the
// ledger, turns the raw takes into game-ready Ogg files (ffmpeg + oggenc),
// writes a review page and, once a person has listened, lists the approved
// clips in the sound list (assets/metadata/audio/audio.json).
//
// Two backends make the takes (-backend):
//
//   - moss (the default, user decision 2026-10-31): the open-source
//     MOSS-SoundEffect v2.0 model (Apache-2.0) run locally through
//     assets/source/tools/moss/generate.py. Free; each take is processed to Ogg at
//     once. A take needs about 4 min and 16-22 GB of memory on an Apple
//     M-series GPU, so takes run one at a time and wait until
//     -moss-min-free-gb is free.
//   - elevenlabs: the paid text-to-sound-effects API, only with
//     user-approved spending (dry-run, approval, ledger, reserve).
//
// MOSS setup, outside the repo (defaults under $XDG_CACHE_HOME/rimworld-go/moss,
// else ~/.cache/rimworld-go/moss; override with -moss-python/-moss-model or
// $SFXGEN_MOSS_PYTHON/$SFXGEN_MOSS_MODEL):
//
//	python3.12 -m venv ~/.cache/rimworld-go/moss/venv
//	~/.cache/rimworld-go/moss/venv/bin/pip install <MOSS-TTS repo checkout> soundfile
//	huggingface-cli download OpenMOSS-Team/MOSS-SoundEffect-v2.0 \
//	    --local-dir ~/.cache/rimworld-go/moss/MOSS-SoundEffect-v2.0
//
// Usage:
//
//	sfxgen -dry-run [-only ids]            the plan (moss: time and command; elevenlabs: credits)
//	sfxgen [-only ids]                     generate with MOSS and process (free, local)
//	sfxgen -supersede elevenlabs [-only ids]
//	                                       also make sounds again whose approved take came from that backend
//	sfxgen -backend elevenlabs [-only ids] [-max-credits N]
//	                                       paid; key from $ELEVENLABS_API_KEY only
//	sfxgen -process [-only ids]            raw takes -> sfx/animals/<key>_<event>.ogg (+ _b)
//	sfxgen -review -o page.html            a page with a player for every processed clip
//	sfxgen -approve ids / -reject ids      record the listener's verdict (ledger + sound list)
//
// Every take records its backend in the ledger. The ElevenLabs key is read
// from the environment and is never printed, logged or saved.
package sfxgen
