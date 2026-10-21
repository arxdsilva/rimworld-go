#!/usr/bin/env bash
# Import an approved ElevenLabs take into the game (SND-002).
#
#   scripts/audio_import.sh <raw-file> <file-in-prompts.json> <credits> "<license>"
#   e.g. scripts/audio_import.sh assets/source/audio/elevenlabs/raw/x.mp3 ambience/day.ogg 400 "ElevenLabs Creator plan"
#
# Loudness-normalizes (music -16 LUFS stereo, ambience -20 LUFS mono),
# resamples to 48 kHz, makes loops seamless (audiogen -loopfix), encodes
# Ogg Vorbis into assets/generated/audio/<file>, and lists it in the sound
# list with its prompt, credits and license (audiogen -register).
set -euo pipefail
if [ $# -ne 4 ]; then
  sed -n '2,5p' "$0" >&2
  exit 2
fi
raw=$1 rel=$2 credits=$3 license=$4
case $rel in
  music/*) lufs=-16 ch=2 q=4 ;;
  ambience/*) lufs=-20 ch=1 q=3 ;;
  *) echo "audio_import: $rel must be under music/ or ambience/" >&2; exit 2 ;;
esac
loop=$(go run ./cmd/audiogen -looping "$rel")
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
ffmpeg -hide_banner -loglevel error -y -i "$raw" \
  -af "loudnorm=I=${lufs}:TP=-1.5:LRA=11" -ar 48000 -ac "$ch" -c:a pcm_s16le "$tmp/norm.wav"
src=$tmp/norm.wav
if [ "$loop" = true ]; then
  go run ./cmd/audiogen -loopfix "$tmp/norm.wav" "$tmp/loop.wav"
  src=$tmp/loop.wav
fi
out=assets/generated/audio/$rel
mkdir -p "$(dirname "$out")"
ffmpeg -hide_banner -loglevel error -y -i "$src" -c:a libvorbis -q:a "$q" "$out"
go run ./cmd/audiogen -register "$rel" -credits "$credits" -license "$license"
go run ./cmd/audiogen -check
echo "audio_import: $out ($(du -h "$out" | cut -f1))"
