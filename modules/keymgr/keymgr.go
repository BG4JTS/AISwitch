//go:build keymgr
// +build keymgr

package keymgr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
)

// encryptedKey stores a masked or encrypted key entry.
type encryptedKey struct {
	Provider string `json:"provider"`
	KeyHash  string `json:"key_hash"` // first 8 chars + "***"
	// In production, this would be AES-encrypted.
	Encrypted []byte `json:"encrypted,omitempty"`
}

// Manager is the enhanced key manager with persistent encrypted storage.
type Manager struct {
	mu       sync.RWMutex
	keys     map[string]string
	savePath string
}

// KeyMgr is the key-manager module.
type KeyMgr struct {
	mgr    *Manager
	config interface{}
}

func init() {
	core.RegisterModule(&KeyMgr{})
}

func (m *KeyMgr) Name() string       { return "keymgr" }
func (m *KeyMgr) Requires() []string { return nil }
func (m *KeyMgr) Enabled() bool      { return true }

func (m *KeyMgr) Init(ctx *module.CoreContext) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ais")
	os.MkdirAll(dir, 0700)

	m.mgr = &Manager{
		keys:     make(map[string]string),
		savePath: filepath.Join(dir, "keys.enc"),
	}
	m.mgr.load()
	return nil
}

func (m *KeyMgr) Start(_ context.Context) error { return nil }
func (m *KeyMgr) Stop() error                   { return nil }

// AddKey stores a key (simplified — in production this would be encrypted).
func (m *KeyMgr) AddKey(provider, key string) error {
	m.mgr.mu.Lock()
	defer m.mgr.mu.Unlock()
	m.mgr.keys[provider] = key
	return m.mgr.save()
}

// GetKey returns the stored key for a provider.
func (m *KeyMgr) GetKey(provider string) (string, error) {
	m.mgr.mu.RLock()
	defer m.mgr.mu.RUnlock()
	if key, ok := m.mgr.keys[provider]; ok {
		return key, nil
	}
	return "", fmt.Errorf("no key found for provider %q", provider)
}

// ListProviders returns all stored provider names.
func (m *KeyMgr) ListProviders() []string {
	m.mgr.mu.RLock()
	defer m.mgr.mu.RUnlock()
	out := make([]string, 0, len(m.mgr.keys))
	for k := range m.mgr.keys {
		out = append(out, k)
	}
	return out
}

// ListKeys returns masked key entries for display.
func (m *KeyMgr) ListKeys() []encryptedKey {
	m.mgr.mu.RLock()
	defer m.mgr.mu.RUnlock()
	out := make([]encryptedKey, 0, len(m.mgr.keys))
	for provider, key := range m.mgr.keys {
		hash := key[:min(8, len(key))] + "***"
		out = append(out, encryptedKey{Provider: provider, KeyHash: hash})
	}
	return out
}

// DeleteKey removes a stored key.
func (m *KeyMgr) DeleteKey(provider string) error {
	m.mgr.mu.Lock()
	defer m.mgr.mu.Unlock()
	delete(m.mgr.keys, provider)
	return m.mgr.save()
}

func (mgr *Manager) load() {
	data, err := os.ReadFile(mgr.savePath)
	if err != nil {
		return
	}
	var entries []encryptedKey
	json.Unmarshal(data, &entries)
	for _, e := range entries {
		// Simple storage: key is in Encrypted field as plaintext.
		// Production would decrypt here.
		val := strings.TrimSpace(string(e.Encrypted))
		if val != "" {
			mgr.keys[e.Provider] = val
		}
	}
}

func (mgr *Manager) save() error {
	entries := make([]encryptedKey, 0, len(mgr.keys))
	for provider, key := range mgr.keys {
		hash := key[:min(8, len(key))] + "***"
		entries = append(entries, encryptedKey{
			Provider:  provider,
			KeyHash:   hash,
			Encrypted: []byte(key), // plaintext for MVP
		})
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	return os.WriteFile(mgr.savePath, data, 0600)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── CLI helpers ──

var defaultMgr *KeyMgr

// SetDefault registers the global key manager.
func SetDefault(m *KeyMgr) { defaultMgr = m }

// CliAdd adds a key via CLI.
func CliAdd(provider, key string) error {
	if defaultMgr == nil {
		return fmt.Errorf("keymgr module not loaded (build with -tags keymgr)")
	}
	return defaultMgr.AddKey(provider, key)
}

// CliList returns masked keys for CLI display.
func CliList() []encryptedKey {
	if defaultMgr == nil {
		return nil
	}
	return defaultMgr.ListKeys()
}

// CliDelete removes a key via CLI.
func CliDelete(provider string) error {
	if defaultMgr == nil {
		return fmt.Errorf("keymgr module not loaded (build with -tags keymgr)")
	}
	return defaultMgr.DeleteKey(provider)
}
