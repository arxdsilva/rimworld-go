// Package audiogen renders the game's procedural sound effects
// (recipes.go, pkg/sfxsynth) to assets/generated/audio as 48 kHz mono
// 16-bit WAV and updates the sound list (assets/metadata/audio/audio.json),
// keeping every non-procedural entry. Output is byte-identical on every
// machine. cmd/audiogen only calls Run.
//
//	audiogen [-assets assets]          render + update the sound list (make audio)
//	audiogen -music                    compose the music to Ogg (make music; needs ffmpeg)
//	audiogen -animals                  render the animal placeholder voices to Ogg (needs oggenc)
//	audiogen -check                    validate the sound list against the files
//	audiogen -loopfix in.wav out.wav   make an imported loop seamless
//	audiogen -looping music/menu.ogg   print whether prompts.json marks it a loop
//	audiogen -register ambience/day.ogg -license "..." [-seed 7 | -credits 400]
//	                                   list an imported MOSS (-seed) or ElevenLabs take
package audiogen
