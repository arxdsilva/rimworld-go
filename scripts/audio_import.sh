#!/usr/bin/env bash
# Import an approved ElevenLabs take into the game (SND-002).
#
#   scripts/audio_import.sh <raw-file> <file-in-prompts.json> <credits> "<license>"
#   e.g. scripts/audio_import.sh assets/source/audio/elevenlabs/raw/x.mp3 ambience/day.ogg 400 "ElevenLabs Creator plan"
#
# Loudness-normalizes (measured, then gained: music -16 LUFS stereo,
# ambience -20 LUFS mono),
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
# measure, then apply the exact gain (single-pass loudnorm undershoots on
# short, quiet clips), with a limiter keeping peaks under -1.5 dBFS
ffmpeg -hide_banner -loglevel error -y -i "$raw" -ar 48000 -ac "$ch" -c:a pcm_s16le "$tmp/in.wav"
have=$(ffmpeg -hide_banner -i "$tmp/in.wav" -af ebur128=framelog=quiet -f null - 2>&1 | awk '/^ +I:/ {print $2}' | tail -1)
gain=$(awk -v t="$lufs" -v h="$have" 'BEGIN { printf "%.2f", t - h }')
ffmpeg -hide_banner -loglevel error -y -i "$tmp/in.wav" \
  -af "volume=${gain}dB,alimiter=limit=0.84:attack=5:release=50:level=false" -c:a pcm_s16le "$tmp/norm.wav"
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
