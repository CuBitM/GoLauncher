package configs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type LauncherConfig struct {
	GameDir            string         `json:"gameDir"`
	InstancesDir       string         `json:"instancesDir"`
	SelectedInstanceID string         `json:"selectedInstanceId"`
	OfflineMode        bool           `json:"offlineMode"`
	OfflineUsername    string         `json:"offlineUsername"`
	Account            *AccountConfig `json:"account"`
	SelectedVersion    string         `json:"selectedVersion"`
	CustomJVM          string         `json:"customJvm"`
	ExtraJVMArgs       []string       `json:"extraJvmArgs"`
	AllocMax           int            `json:"allocMax"`
	Fullscreen         bool           `json:"fullscreen"`
	ShowSnapshots      bool           `json:"showSnapshots"`
	ShowOld            bool           `json:"showOld"`
}

type AccountConfig struct {
	Username     string `json:"username"`
	UUID         string `json:"uuid"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func Load() (*LauncherConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		cfg := Default()
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		cfg := Default()
		_ = Save(cfg)
		return cfg, nil
	}

	var cfg LauncherConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}

	Normalize(&cfg)
	return &cfg, nil
}

func Save(cfg *LauncherConfig) error {
	Normalize(cfg)

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func Default() *LauncherConfig {
	return &LauncherConfig{
		GameDir:         DefaultGameDir(),
		InstancesDir:    DefaultInstancesDir(),
		OfflineMode:     true,
		OfflineUsername: "Player",
		SelectedVersion: "1.16.5",
		AllocMax:        2048,
		ShowSnapshots:   false,
		ShowOld:         false,
		Fullscreen:      false,
	}
}

func Normalize(cfg *LauncherConfig) {
	if cfg.GameDir == "" {
		cfg.GameDir = DefaultGameDir()
	}

	if cfg.InstancesDir == "" {
		cfg.InstancesDir = DefaultInstancesDir()
	}

	if cfg.OfflineUsername == "" {
		cfg.OfflineUsername = "Player"
	}

	if cfg.SelectedVersion == "" {
		cfg.SelectedVersion = "1.16.5"
	}

	if cfg.AllocMax <= 0 {
		cfg.AllocMax = 2048
	}
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func ConfigDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base != "" {
			return filepath.Join(base, "GoLauncher"), nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, "AppData", "Roaming", "GoLauncher"), nil

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, "Library", "Application Support", "GoLauncher"), nil

	default:
		base := os.Getenv("XDG_CONFIG_HOME")
		if base != "" {
			return filepath.Join(base, "golauncher"), nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, ".config", "golauncher"), nil
	}
}

func DataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base != "" {
			return filepath.Join(base, "GoLauncher"), nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, "AppData", "Roaming", "GoLauncher"), nil

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, "Library", "Application Support", "GoLauncher"), nil

	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base != "" {
			return filepath.Join(base, "golauncher"), nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, ".local", "share", "golauncher"), nil
	}
}

func DefaultInstancesDir() string {
	dir, err := DataDir()
	if err != nil {
		return "instances"
	}

	return filepath.Join(dir, "instances")
}

func DefaultGameDir() string {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, ".minecraft")
		}

		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "AppData", "Roaming", ".minecraft")
		}

	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "minecraft")
		}

	default:
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".minecraft")
		}
	}

	return ".minecraft"
}
