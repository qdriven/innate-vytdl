package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// audioCodecArgs maps the requested audio format to ffmpeg encoder args
// (identical table to the previous Node backend).
func audioCodecArgs(format string) []string {
	switch format {
	case "m4a", "aac":
		return []string{"-c:a", "aac", "-b:a", "192k"}
	case "flac":
		return []string{"-c:a", "flac"}
	case "wav":
		return []string{"-c:a", "pcm_s16le"}
	case "ogg":
		return []string{"-c:a", "libvorbis", "-q:a", "4"}
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", "128k"}
	default: // mp3
		return []string{"-c:a", "libmp3lame", "-q:a", "2"}
	}
}

// ffmpegPath resolves the ffmpeg binary (env override → PATH).
func ffmpegPath() string {
	if p := strings.TrimSpace(os.Getenv("FFMPEG_PATH")); p != "" {
		return p
	}
	return "ffmpeg"
}

// ExtractAudio converts a video file to an audio file via ffmpeg.
func ExtractAudio(videoPath, outputDir, audioFormat string) (string, error) {
	if _, err := os.Stat(videoPath); err != nil {
		return "", fmt.Errorf("video file not found: %s", videoPath)
	}
	if outputDir == "" {
		outputDir = filepath.Dir(videoPath)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	ext := audioFormat
	if ext == "m4a" || ext == "aac" {
		ext = "m4a"
	}
	if audioFormat == "" {
		audioFormat = "mp3"
		ext = "mp3"
	}
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	outputPath := filepath.Join(outputDir, stem+"."+ext)

	args := []string{"-i", videoPath, "-vn", "-y"}
	args = append(args, audioCodecArgs(audioFormat)...)
	args = append(args, outputPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath(), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return outputPath, nil
}
