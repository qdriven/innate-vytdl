package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds runtime defaults loaded from config.json.
type Config struct {
	YTDLPBin string `json:"yt_dlp_bin"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		YTDLPBin: "/Applications/ServBay/package/python/3.14/3.14.0b1/Python.framework/Versions/3.14/bin/yt-dlp",
	}
}

// Load returns config from disk when available, otherwise built-in defaults.
func Load() Config {
	cfg := Default()
	path := configPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}

	if v := strings.TrimSpace(fileCfg.YTDLPBin); v != "" {
		cfg.YTDLPBin = v
	}
	return cfg
}

func configPath() string {
	if path := strings.TrimSpace(os.Getenv("VYTDL_CONFIG")); path != "" {
		return path
	}

	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "config.json"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "config.json"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
