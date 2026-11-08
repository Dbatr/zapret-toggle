package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingGivesDefault(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg != Default() || !cfg.StopOnClose {
		t.Fatalf("got %+v", cfg)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zapret-toggle", "config.json")
	want := Config{Strategy: "alt2", StopOnClose: false, Theme: ThemeLight}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"strategy":"alt"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Strategy != "alt" || !got.StopOnClose {
		t.Fatalf("got %+v", got)
	}
}
