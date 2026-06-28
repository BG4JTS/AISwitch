package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile stores a named provider configuration.
type Profile struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url,omitempty"`
}

// File represents the persistent config file (~/.ais/config.json).
type File struct {
	DefaultProfile string    `json:"default_profile,omitempty"`
	Profiles       []Profile `json:"profiles"`
}

// configPath returns the path to ~/.ais/config.json.
// If AIS_CONFIG_PATH is set, it uses that path directly (for testing).
func configPath() (string, error) {
	if p := os.Getenv("AIS_CONFIG_PATH"); p != "" {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("cannot create config directory: %w", err)
		}
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".ais")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads and returns the config file.
// Returns an empty config if the file does not exist.
func Load() (*File, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	cfg := &File{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}
	return cfg, nil
}

// Save writes the config back to disk.
func (c *File) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	// Ensure trailing newline
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

// SetProfile adds or updates a named profile and saves.
func (c *File) SetProfile(profile Profile) error {
	for i, p := range c.Profiles {
		if p.Name == profile.Name {
			c.Profiles[i] = profile
			return c.Save()
		}
	}
	c.Profiles = append(c.Profiles, profile)
	return c.Save()
}

// GetProfile returns a profile by name, or nil if not found.
func (c *File) GetProfile(name string) *Profile {
	for _, p := range c.Profiles {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// Default returns the profile currently marked as default,
// or nil if none is set.
func (c *File) Default() *Profile {
	if c.DefaultProfile == "" {
		return nil
	}
	return c.GetProfile(c.DefaultProfile)
}

// SetDefault marks the named profile as default and saves.
func (c *File) SetDefault(name string) error {
	if c.GetProfile(name) == nil {
		return fmt.Errorf("profile %q does not exist", name)
	}
	c.DefaultProfile = name
	return c.Save()
}

// DeleteProfile removes a profile by name and saves.
func (c *File) DeleteProfile(name string) error {
	for i, p := range c.Profiles {
		if p.Name == name {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			if c.DefaultProfile == name {
				c.DefaultProfile = ""
			}
			return c.Save()
		}
	}
	return fmt.Errorf("profile %q not found", name)
}
