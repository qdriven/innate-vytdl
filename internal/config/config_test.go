package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromExplicitConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	data := []byte(`{"yt_dlp_bin":"/custom/bin/yt-dlp"}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("VYTDL_CONFIG", configPath)
	cfg := Load()
	if cfg.YTDLPBin != "/custom/bin/yt-dlp" {
		t.Fatalf("unexpected yt_dlp_bin: %q", cfg.YTDLPBin)
	}
}

func TestLoadFallsBackToDefault(t *testing.T) {
	t.Setenv("VYTDL_CONFIG", filepath.Join(t.TempDir(), "missing.json"))

	cfg := Load()
	if cfg.YTDLPBin != Default().YTDLPBin {
		t.Fatalf("expected default yt_dlp_bin %q, got %q", Default().YTDLPBin, cfg.YTDLPBin)
	}
}
