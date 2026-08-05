package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app", "preferences.json")
	store := New(path)
	config := Config{SchemaVersion: SchemaVersion, Language: LanguageEnglish, DailyStations: []string{"tongji", "mlab"}}
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := store.Load()
	if err != nil || !exists {
		t.Fatalf("Load() = %#v, %t, %v", loaded, exists, err)
	}
	if loaded.Language != LanguageEnglish || len(loaded.DailyStations) != 2 || loaded.DailyStations[0] != "tongji" {
		t.Fatalf("loaded = %#v", loaded)
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{filepath.Dir(path), 0o700}, {path, 0o600}} {
		info, err := os.Stat(item.path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != item.mode {
			t.Fatalf("%s mode = %o, want %o", item.path, info.Mode().Perm(), item.mode)
		}
	}
}

func TestConfigRejectsWebOnlyStation(t *testing.T) {
	config := DefaultConfig()
	config.DailyStations = []string{"nju-edge"}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() accepted web-only station")
	}
}

func TestConfigRejectsUnavailableCERNETStation(t *testing.T) {
	config := DefaultConfig()
	config.DailyStations = []string{"cernet"}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate() accepted unavailable CERNET station")
	}
}
