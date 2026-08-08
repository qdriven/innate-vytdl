#!/usr/bin/env bash
# cut_mp3.sh - Cut an MP3 file within a time range using FFmpeg
#
# Usage:
#   ./cut_mp3.sh -i input.mp3 -s 00:01:30 -e 00:02:45 -o output.mp3
#   ./cut_mp3.sh -i input.mp3 -s 90 -d 60 -o output.mp3
#   ./cut_mp3.sh -i input.mp3 -s 00:01:30 -d 120
#
# Options:
#   -i  Input MP3 file (required)
#   -s  Start time (required) — HH:MM:SS or seconds
#   -e  End time (optional)   — HH:MM:SS or seconds
#   -d  Duration (optional)   — HH:MM:SS or seconds
#   -o  Output file (optional, defaults to input_cut.mp3)
#   -h  Show this help

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_help() {
    sed -n '2,15p' "$0"
    exit 0
}

INPUT=""
START=""
END=""
DURATION=""
OUTPUT=""

while getopts "i:s:e:d:o:h" opt; do
    case $opt in
        i) INPUT="$OPTARG" ;;
        s) START="$OPTARG" ;;
        e) END="$OPTARG" ;;
        d) DURATION="$OPTARG" ;;
        o) OUTPUT="$OPTARG" ;;
        h) show_help ;;
        *) show_help ;;
    esac
done

# Validate required args
if [[ -z "$INPUT" || -z "$START" ]]; then
    echo "Error: -i (input) and -s (start) are required." >&2
    echo "Usage: $0 -i input.mp3 -s 00:01:30 -e 00:02:45 -o output.mp3" >&2
    exit 1
fi

if [[ ! -f "$INPUT" ]]; then
    echo "Error: Input file not found: $INPUT" >&2
    exit 1
fi

# Check ffmpeg
if ! command -v ffmpeg &> /dev/null; then
    echo "Error: ffmpeg not found. Please install ffmpeg first." >&2
    echo "  macOS: brew install ffmpeg" >&2
    echo "  Linux: sudo apt install ffmpeg" >&2
    exit 1
fi

# Default output name
if [[ -z "$OUTPUT" ]]; then
    base="${INPUT%.*}"
    OUTPUT="${base}_cut.mp3"
fi

# Build ffmpeg arguments
# Place -ss before -i for fast seeking (keyframe-accurate, good enough for MP3)
# Use -c copy to avoid re-encoding (lossless and fast)
ARGS=(-y -ss "$START")

if [[ -n "$END" ]]; then
    ARGS+=(-to "$END")
elif [[ -n "$DURATION" ]]; then
    ARGS+=(-t "$DURATION")
fi

ARGS+=(-i "$INPUT" -c copy "$OUTPUT")

echo "Cutting MP3..."
echo "  Input:    $INPUT"
echo "  Start:    $START"
[[ -n "$END" ]] && echo "  End:      $END"
[[ -n "$DURATION" ]] && echo "  Duration: $DURATION"
echo "  Output:   $OUTPUT"
echo ""

ffmpeg "${ARGS[@]}"

echo ""
echo "Done! Output saved to: $OUTPUT"
