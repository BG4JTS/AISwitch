package config

import (
	"os"
	"path/filepath"
	"testing"
)

func tmpConfig(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("AIS_CONFIG_PATH", path)
	return path
}

func TestSaveAndLoad(t *testing.T) {
	path := tmpConfig(t)

	cfg := &File{}
	err := cfg.SetProfile(Profile{
		Name:     "test",
		Provider: "openai",
		Key:      "sk-123",
		Model:    "gpt-4o",
	})
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("config file not created at %s", path)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := loaded.GetProfile("test")
	if p == nil {
		t.Fatal("GetProfile returned nil")
	}
	if p.Provider != "openai" {
		t.Errorf("Provider = %v, want openai", p.Provider)
	}
	if p.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", p.Model)
	}
}

func TestDefaultProfile(t *testing.T) {
	tmpConfig(t)

	cfg := &File{}
	cfg.SetProfile(Profile{Name: "a", Provider: "x", Key: "k", Model: "m"})
	cfg.SetProfile(Profile{Name: "b", Provider: "y", Key: "k2", Model: "m2"})

	if err := cfg.SetDefault("a"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	def := cfg.Default()
	if def == nil {
		t.Fatal("Default() returned nil")
	}
	if def.Name != "a" {
		t.Errorf("Default name = %v, want a", def.Name)
	}

	// SetDefault with non-existent name should fail
	if err := cfg.SetDefault("nonexistent"); err == nil {
		t.Error("SetDefault with nonexistent name should fail")
	}
}

func TestDeleteProfile(t *testing.T) {
	tmpConfig(t)

	cfg := &File{}
	cfg.SetProfile(Profile{Name: "x", Provider: "p", Key: "k", Model: "m"})

	if err := cfg.DeleteProfile("x"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("Profiles len = %d, want 0", len(cfg.Profiles))
	}

	if err := cfg.DeleteProfile("x"); err == nil {
		t.Error("DeleteProfile on already-deleted should fail")
	}
}

func TestUpdateProfile(t *testing.T) {
	tmpConfig(t)

	cfg := &File{}
	cfg.SetProfile(Profile{Name: "x", Provider: "a", Key: "k1", Model: "m1"})
	cfg.SetProfile(Profile{Name: "x", Provider: "b", Key: "k2", Model: "m2"})

	if len(cfg.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
	p := cfg.Profiles[0]
	if p.Provider != "b" {
		t.Errorf("Provider = %v, want b", p.Provider)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmpConfig(t)

	// No config written yet — Load should return empty File without error
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("Profiles len = %d, want 0", len(cfg.Profiles))
	}
}
