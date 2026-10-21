package sfxsynth

import (
	"encoding/binary"
	"errors"
	"io"
)

// WriteWAV writes mono 16-bit PCM at rate as a RIFF WAVE file.
func WriteWAV(w io.Writer, pcm []int16, rate int) error {
	return WriteWAVChannels(w, pcm, rate, 1)
}

// WriteWAVChannels writes interleaved 16-bit PCM with ch channels.
func WriteWAVChannels(w io.Writer, pcm []int16, rate, ch int) error {
	data := uint32(len(pcm) * 2)
	hdr := []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + data, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(ch),
		uint32(rate), uint32(rate * 2 * ch), uint16(2 * ch), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, data,
	}
	for _, v := range hdr {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	return binary.Write(w, binary.LittleEndian, pcm)
}

// ReadWAV reads a mono or stereo 16-bit PCM WAVE file, returning mono
// samples (stereo is averaged), the rate and the channel count.
func ReadWAV(r io.Reader) (pcm []int16, rate, channels int, err error) {
	var riff [12]byte
	if _, err = io.ReadFull(r, riff[:]); err != nil {
		return nil, 0, 0, err
	}
	if string(riff[:4]) != "RIFF" || string(riff[8:]) != "WAVE" {
		return nil, 0, 0, errors.New("not a WAVE file")
	}
	bits := 0
	for {
		var id [4]byte
		var size uint32
		if _, err = io.ReadFull(r, id[:]); err != nil {
			return nil, 0, 0, err
		}
		if err = binary.Read(r, binary.LittleEndian, &size); err != nil {
			return nil, 0, 0, err
		}
		body := make([]byte, size+size%2)
		if _, err = io.ReadFull(r, body); err != nil {
			return nil, 0, 0, err
		}
		switch string(id[:]) {
		case "fmt ":
			if size < 16 || binary.LittleEndian.Uint16(body[0:]) != 1 {
				return nil, 0, 0, errors.New("WAVE is not PCM")
			}
			channels = int(binary.LittleEndian.Uint16(body[2:]))
			rate = int(binary.LittleEndian.Uint32(body[4:]))
			bits = int(binary.LittleEndian.Uint16(body[14:]))
		case "data":
			if bits != 16 || channels < 1 || channels > 2 {
				return nil, 0, 0, errors.New("WAVE must be 16-bit mono or stereo")
			}
			n := int(size) / 2 / channels
			pcm = make([]int16, n)
			for i := 0; i < n; i++ {
				sum := 0
				for c := 0; c < channels; c++ {
					sum += int(int16(binary.LittleEndian.Uint16(body[(i*channels+c)*2:])))
				}
				pcm[i] = int16(sum / channels)
			}
			return pcm, rate, channels, nil
		}
	}
}
