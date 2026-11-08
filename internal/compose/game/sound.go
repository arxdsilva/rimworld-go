package game

import (
	"log"

	"rimworld-go/internal/adapters/outbound/ebitenaudio"
	"rimworld-go/internal/platform/sysinfo"
)

// openSound opens the audio device and the sound list; nil when off
// (captures, -noaudio, RIMWORLD_GO_NOAUDIO=1) or when they can't be
// opened, and the game runs silent.
func openSound(off bool) *ebitenaudio.Engine {
	if off {
		return nil
	}
	root, err := ebitenaudio.LocateAudio()
	if err != nil {
		log.Printf("sound off: %v", err)
		return nil
	}
	snd, err := ebitenaudio.New(root)
	if err != nil {
		log.Printf("sound off: %v", err)
		return nil
	}
	return snd
}

// hardware reads the computer's memory and cores once: the menu asks every
// frame and the total never changes while the game runs. Captures get none,
// so media never depends on the machine that made it.
func hardware(capture bool) func() sysinfo.Info {
	if capture {
		return nil
	}
	info := sysinfo.Read()
	return func() sysinfo.Info { return info }
}
