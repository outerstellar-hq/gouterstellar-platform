package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPlugin struct {
	name           string
	version        string
	description    string
	initErr        error
	shutdownCalled bool
}

func (m *mockPlugin) Name() string        { return m.name }
func (m *mockPlugin) Version() string     { return m.version }
func (m *mockPlugin) Description() string { return m.description }
func (m *mockPlugin) Initialize() error {
	if m.initErr != nil {
		return m.initErr
	}
	return nil
}

func (m *mockPlugin) Shutdown() {
	m.shutdownCalled = true
}

func TestRegister(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("test", func() Plugin {
		return &mockPlugin{name: "test", version: "1.0.0"}
	})
	assert.False(t, mgr.IsInitialized())
}

func TestDiscoverAndInitialize(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("alpha", func() Plugin {
		return &mockPlugin{name: "alpha", version: "1.0.0", description: "Alpha plugin"}
	})
	mgr.Register("beta", func() Plugin {
		return &mockPlugin{name: "beta", version: "2.0.0", description: "Beta plugin"}
	})

	results := mgr.DiscoverAndInitialize(false)
	assert.Len(t, results, 2)
	assert.True(t, mgr.IsInitialized())

	allPlugins := mgr.GetAllPlugins()
	assert.Len(t, allPlugins, 2)
}

func TestGetPlugin(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("test", func() Plugin {
		return &mockPlugin{name: "test", version: "1.0.0"}
	})
	mgr.DiscoverAndInitialize(false)

	p := mgr.GetPlugin("test")
	require.NotNil(t, p)
	assert.Equal(t, "test", p.Name())
	assert.Equal(t, "1.0.0", p.Version())

	assert.Nil(t, mgr.GetPlugin("nonexistent"))
}

func TestDiscoverAndInitializeStrictFailure(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("bad", func() Plugin {
		return &mockPlugin{name: "bad", version: "1.0.0", initErr: errors.New("init failed")}
	})

	results := mgr.DiscoverAndInitialize(true)
	assert.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Error(t, results[0].Error)
	assert.False(t, mgr.IsInitialized())
}

func TestDiscoverAndInitializeNonStrictFailure(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("bad", func() Plugin {
		return &mockPlugin{name: "bad", version: "1.0.0", initErr: errors.New("init failed")}
	})
	mgr.Register("good", func() Plugin {
		return &mockPlugin{name: "good", version: "1.0.0"}
	})

	results := mgr.DiscoverAndInitialize(false)
	assert.Len(t, results, 2)
	assert.True(t, mgr.IsInitialized())
	assert.NotNil(t, mgr.GetPlugin("good"))
	assert.Nil(t, mgr.GetPlugin("bad"))
}

func TestShutdownAll(t *testing.T) {
	mgr := NewPluginManager()
	mp := &mockPlugin{name: "test", version: "1.0.0"}
	mgr.Register("test", func() Plugin { return mp })
	mgr.DiscoverAndInitialize(false)

	mgr.ShutdownAll()
	assert.True(t, mp.shutdownCalled)
	assert.False(t, mgr.IsInitialized())
	assert.Nil(t, mgr.GetPlugin("test"))
}

func TestShutdownPlugin(t *testing.T) {
	mgr := NewPluginManager()
	mp1 := &mockPlugin{name: "a", version: "1.0.0"}
	mp2 := &mockPlugin{name: "b", version: "1.0.0"}
	mgr.Register("a", func() Plugin { return mp1 })
	mgr.Register("b", func() Plugin { return mp2 })
	mgr.DiscoverAndInitialize(false)

	mgr.ShutdownPlugin("a")
	assert.True(t, mp1.shutdownCalled)
	assert.False(t, mp2.shutdownCalled)
	assert.Nil(t, mgr.GetPlugin("a"))
	assert.NotNil(t, mgr.GetPlugin("b"))
}

func TestWithPlugin(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("test", func() Plugin {
		return &mockPlugin{name: "test", version: "1.0.0"}
	})
	mgr.DiscoverAndInitialize(false)

	var received Plugin
	err := mgr.WithPlugin("test", func(p Plugin) error {
		received = p
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "test", received.Name())

	err = mgr.WithPlugin("nonexistent", func(p Plugin) error { return nil })
	assert.Error(t, err)
}

func TestWithEachPlugin(t *testing.T) {
	mgr := NewPluginManager()
	mgr.Register("a", func() Plugin {
		return &mockPlugin{name: "a", version: "1.0.0"}
	})
	mgr.Register("b", func() Plugin {
		return &mockPlugin{name: "b", version: "1.0.0"}
	})
	mgr.DiscoverAndInitialize(false)

	var names []string
	errs := mgr.WithEachPlugin(func(p Plugin) error {
		names = append(names, p.Name())
		return nil
	})
	assert.Empty(t, errs)
	assert.Len(t, names, 2)
	assert.Contains(t, names, "a")
	assert.Contains(t, names, "b")
}
