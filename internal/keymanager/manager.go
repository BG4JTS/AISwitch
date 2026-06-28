// Package keymanager 提供线程安全的 API Key 内存管理。
//
// Manager 支持两级 Key 解析（优先级从高到低）：
//   1. 通过 SetKey() 存入内存的 Key
//   2. 环境变量 AIS_{PROVIDER}_KEY（大小写不敏感）
//
// Manager 的所有公开方法均线程安全。
package keymanager

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Manager stores and retrieves API keys for different providers.
type Manager struct {
	mu        sync.RWMutex
	keys      map[string]string
	envPrefix string
}

// New creates a Manager with the given environment variable prefix.
func New(envPrefix string) *Manager {
	return &Manager{
		keys:      make(map[string]string),
		envPrefix: envPrefix,
	}
}

// GetKey returns the API key for a provider.
// Priority: in-memory > environment variable > error.
func (m *Manager) GetKey(provider string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. In-memory
	if key, ok := m.keys[provider]; ok && key != "" {
		return key, nil
	}

	// 2. Environment variable (case-insensitive)
	envKey := fmt.Sprintf("%s_%s_KEY", strings.ToUpper(m.envPrefix), strings.ToUpper(provider))
	if key := os.Getenv(envKey); key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no API key found for provider %q (set with --key or %s)", provider, envKey)
}

// SetKey stores a key in memory.
func (m *Manager) SetKey(provider, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[provider] = key
}

// ListProviders returns all in-memory provider names.
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]string, 0, len(m.keys))
	for k := range m.keys {
		providers = append(providers, k)
	}
	return providers
}
