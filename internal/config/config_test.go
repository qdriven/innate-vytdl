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

func TestLoadOutputDir(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	data := []byte(`{"output_dir":"/Volumes/cloud/Downloads"}`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("VYTDL_CONFIG", configPath)
	cfg := Load()
	if cfg.OutputDir != "/Volumes/cloud/Downloads" {
		t.Fatalf("unexpected output_dir: %q", cfg.OutputDir)
	}
}

func TestLoadOutputDirExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home: %v", err)
	}
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	for _, tc := range []struct{ in, want string }{
		{"~/Downloads", filepath.Join(home, "Downloads")},
		{"~", home},
	} {
		data := []byte(`{"output_dir":"` + tc.in + `"}`)
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		t.Setenv("VYTDL_CONFIG", configPath)
		cfg := Load()
		if cfg.OutputDir != tc.want {
			t.Fatalf("output_dir %q: got %q, want %q", tc.in, cfg.OutputDir, tc.want)
		}
	}
}

func TestLoadOutputDirEmptyByDefault(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"yt_dlp_bin":"/custom/bin/yt-dlp"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("VYTDL_CONFIG", configPath)
	cfg := Load()
	if cfg.OutputDir != "" {
		t.Fatalf("expected empty output_dir, got %q", cfg.OutputDir)
	}
}
