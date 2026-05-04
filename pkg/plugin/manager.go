package plugin

import (
	"fmt"
	"log/slog"
	"sync"
)

type PluginLoadResult struct {
	Plugin  Plugin
	Success bool
	Error   error
}

type PluginManager struct {
	mu          sync.RWMutex
	plugins     map[string]Plugin
	factories   map[string]func() Plugin
	initialized bool
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins:   make(map[string]Plugin),
		factories: make(map[string]func() Plugin),
	}
}

func (m *PluginManager) Register(name string, factory func() Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[name] = factory
	slog.Info("plugin factory registered", "name", name)
}

func (m *PluginManager) DiscoverAndInitialize(strict bool) []PluginLoadResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []PluginLoadResult

	for name, factory := range m.factories {
		p := factory()
		if err := p.Initialize(); err != nil {
			slog.Error("plugin initialization failed", "name", name, "error", err)
			results = append(results, PluginLoadResult{
				Plugin:  p,
				Success: false,
				Error:   fmt.Errorf("plugin %s failed to initialize: %w", name, err),
			})
			if strict {
				m.initialized = false
				return results
			}
			continue
		}
		m.plugins[name] = p
		results = append(results, PluginLoadResult{
			Plugin:  p,
			Success: true,
		})
		slog.Info("plugin initialized", "name", name, "version", p.Version())
	}

	m.initialized = true
	return results
}

func (m *PluginManager) GetPlugin(name string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins[name]
}

func (m *PluginManager) GetAllPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

func (m *PluginManager) ShutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, p := range m.plugins {
		p.Shutdown()
		slog.Info("plugin shut down", "name", name)
	}
	m.plugins = make(map[string]Plugin)
	m.initialized = false
}

func (m *PluginManager) ShutdownPlugin(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.plugins[name]; ok {
		p.Shutdown()
		delete(m.plugins, name)
		slog.Info("plugin shut down", "name", name)
	}
}

func (m *PluginManager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

func (m *PluginManager) WithPlugin(name string, fn func(Plugin) error) error {
	m.mu.RLock()
	p, ok := m.plugins[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	return fn(p)
}

func (m *PluginManager) WithEachPlugin(fn func(Plugin) error) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error
	for _, p := range m.plugins {
		if err := fn(p); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
