package configs

import (
	"encoding/json"
	"mclauncher/internal/auth"
	"os"
	"path/filepath"
	"runtime"
)

type LauncherConfig struct {
	Account         *auth.Account `json:"account"`
	OfflineUsername string        `json:"offlineUsername"`
	OfflineMode     bool          `json:"offlineMode"`
	SelectedVersion string        `json:"selectedVersion"`
	ModLoader       string        `json:"modLoader"`
	AllocMax        int           `json:"allocMax"`
	Fullscreen      bool          `json:"fullscreen"`
	CustomJVM       string        `json:"customJVM"`
	ExtraJVMArgs    []string      `json:"extraJvmArgs"`
	ShowSnapshots   bool          `json:"showSnapshots"`
	ShowOld         bool          `json:"showOld"`
	GameDir         string        `json:"gameDir"`
}

func DefaultConfig() *LauncherConfig {
	return &LauncherConfig{
		OfflineUsername: "Hrac",
		OfflineMode:     true,
		AllocMax:        2048,
		Fullscreen:      false,
		GameDir:         DefaultGameDir(),
	}
}

func DefaultGameDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), ".minecraft")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "minecraft")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".minecraft")
	}
}

func ConfigPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".golauncher")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

func Load() (*LauncherConfig, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig(), nil
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), nil
	}
	return cfg, nil
}

func Save(cfg *LauncherConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}
