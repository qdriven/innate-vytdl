#!/usr/bin/env python3
"""Cut an MP3 file within a time range.

Uses FFmpeg if available (fast, lossless copy). Falls back to pydub
(pure Python, re-encodes) if ffmpeg is not installed.

Usage:
    python cut_mp3.py -i input.mp3 -s 00:01:30 -e 00:02:45 -o output.mp3
    python cut_mp3.py -i input.mp3 -s 90 -d 60 -o output.mp3
    python cut_mp3.py -i input.mp3 -s 00:01:30 -d 120

Options:
    -i  Input MP3 file (required)
    -s  Start time (required) — HH:MM:SS or seconds
    -e  End time (optional)   — HH:MM:SS or seconds
    -d  Duration (optional)   — HH:MM:SS or seconds
    -o  Output file (optional, defaults to input_cut.mp3)
"""

import argparse
import os
import shutil
import subprocess
import sys


def time_to_seconds(t: str) -> float:
    """Convert HH:MM:SS or seconds string to float seconds."""
    if ":" in t:
        parts = t.split(":")
        if len(parts) == 3:
            h, m, s = parts
            return int(h) * 3600 + int(m) * 60 + float(s)
        elif len(parts) == 2:
            m, s = parts
            return int(m) * 60 + float(s)
    return float(t)


def seconds_to_time(s: float) -> str:
    """Convert seconds to HH:MM:SS.ms format."""
    hours = int(s // 3600)
    minutes = int((s % 3600) // 60)
    secs = s % 60
    if hours > 0:
        return f"{hours:02d}:{minutes:02d}:{secs:06.3f}"
    return f"{minutes:02d}:{secs:06.3f}"


def cut_with_ffmpeg(input_path: str, output_path: str, start: str, end: str | None, duration: str | None) -> None:
    """Cut MP3 using ffmpeg -c copy (fast, lossless)."""
    ffmpeg = shutil.which("ffmpeg")
    if not ffmpeg:
        raise RuntimeError("ffmpeg not found")

    args = [ffmpeg, "-y", "-ss", start]
    if end:
        args += ["-to", end]
    elif duration:
        args += ["-t", duration]
    args += ["-i", input_path, "-c", "copy", output_path]

    print(f"Using FFmpeg: {ffmpeg}")
    print(f"  Input:    {input_path}")
    print(f"  Start:    {start}")
    if end:
        print(f"  End:      {end}")
    if duration:
        print(f"  Duration: {duration}")
    print(f"  Output:   {output_path}")
    print()

    subprocess.run(args, check=True)
    print(f"\nDone! Output saved to: {output_path}")


def cut_with_pydub(input_path: str, output_path: str, start_sec: float, end_sec: float | None, duration_sec: float | None) -> None:
    """Cut MP3 using pydub (pure Python, re-encodes)."""
    try:
        from pydub import AudioSegment
    except ImportError:
        print("Error: pydub not installed. Install with: pip install pydub")
        print("       Also requires ffmpeg installed for MP3 encoding.")
        sys.exit(1)

    audio = AudioSegment.from_mp3(input_path)
    start_ms = int(start_sec * 1000)

    if end_sec is not None:
        end_ms = int(end_sec * 1000)
    elif duration_sec is not None:
        end_ms = start_ms + int(duration_sec * 1000)
    else:
        end_ms = len(audio)

    cut_audio = audio[start_ms:end_ms]
    cut_audio.export(output_path, format="mp3")
    print(f"Done! Output saved to: {output_path}")
    print(f"  Duration: {len(cut_audio) / 1000:.2f}s")


def main():
    parser = argparse.ArgumentParser(description="Cut an MP3 file within a time range")
    parser.add_argument("-i", "--input", required=True, help="Input MP3 file")
    parser.add_argument("-s", "--start", required=True, help="Start time (HH:MM:SS or seconds)")
    parser.add_argument("-e", "--end", help="End time (HH:MM:SS or seconds)")
    parser.add_argument("-d", "--duration", help="Duration (HH:MM:SS or seconds)")
    parser.add_argument("-o", "--output", help="Output file (default: input_cut.mp3)")
    parser.add_argument("--pydub", action="store_true", help="Force use pydub instead of ffmpeg")
    args = parser.parse_args()

    if not os.path.isfile(args.input):
        print(f"Error: Input file not found: {args.input}")
        sys.exit(1)

    output = args.output or f"{os.path.splitext(args.input)[0]}_cut.mp3"

    # Try ffmpeg first (fast, lossless), unless --pydub is specified
    if not args.pydub and shutil.which("ffmpeg"):
        try:
            cut_with_ffmpeg(args.input, output, args.start, args.end, args.duration)
            return
        except subprocess.CalledProcessError as e:
            print(f"FFmpeg failed: {e}")
            print("Falling back to pydub...\n")

    # Fallback to pydub
    start_sec = time_to_seconds(args.start)
    end_sec = time_to_seconds(args.end) if args.end else None
    duration_sec = time_to_seconds(args.duration) if args.duration else None
    cut_with_pydub(args.input, output, start_sec, end_sec, duration_sec)


if __name__ == "__main__":
    main()
