package config

import (
	"os"
	"path/filepath"
	"runtime"
)

func DefaultConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "relith"), nil
}

func DefaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "relith"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "relith"), nil
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Relith"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "relith"), nil
	}
}

func DefaultSocketPath() (string, error) {
	dataDir, err := DefaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "relith.sock"), nil
}
