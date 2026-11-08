// Package config хранит настройки приложения в ~/.config/zapret-toggle/config.json.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	// Strategy — slug последней выбранной стратегии.
	Strategy string `json:"strategy"`
	// StopOnClose — выключать zapret при закрытии окна (как закрытие батника на Windows).
	StopOnClose bool `json:"stop_on_close"`
	// Theme — тема окна: ThemeDark, ThemeLight или "" (как в системе).
	Theme string `json:"theme,omitempty"`
}

const (
	ThemeDark  = "dark"
	ThemeLight = "light"
)

func Default() Config {
	return Config{StopOnClose: true}
}

// Path возвращает путь к config.json с учётом XDG_CONFIG_HOME.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "zapret-toggle", "config.json"), nil
}

// Load читает конфиг. Если файла нет — возвращает Default() без ошибки;
// отсутствующие в файле поля тоже берутся из Default().
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	return cfg, nil
}

// Save атомарно записывает конфиг (временный файл + rename).
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
