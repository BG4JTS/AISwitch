package core

import "github.com/yourusername/ais/pkg/module"

var registeredModules = map[string]module.Module{}

// RegisterModule adds a module to the global registry.
// Modules should typically call this from their init() functions.
func RegisterModule(m module.Module) {
	registeredModules[m.Name()] = m
}

// GetModules returns a copy of the module registry.
func GetModules() map[string]module.Module {
	copy := make(map[string]module.Module, len(registeredModules))
	for k, v := range registeredModules {
		copy[k] = v
	}
	return copy
}

// GetModule returns a registered module by name, or false if not found.
func GetModule(name string) (module.Module, bool) {
	m, ok := registeredModules[name]
	return m, ok
}
