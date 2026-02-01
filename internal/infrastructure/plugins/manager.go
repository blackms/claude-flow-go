// Package plugins provides plugin management functionality.
package plugins

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// extensionHandler holds an extension point handler with its metadata.
type extensionHandler struct {
	PluginID string
	Handler  func(ctx context.Context, data interface{}) (interface{}, error)
	Priority int
}

// Manager manages plugin lifecycle, dependencies, and extension point invocation.
type Manager struct {
	mu              sync.RWMutex
	plugins         map[string]shared.Plugin
	extensionPoints map[string][]extensionHandler
	eventBus        *events.EventBus
	coreVersion     string
	initialized     bool
}

// Options holds configuration options for the plugin manager.
type Options struct {
	EventBus    *events.EventBus
	CoreVersion string
}

// NewManager creates a new plugin manager.
func NewManager(opts Options) *Manager {
	coreVersion := opts.CoreVersion
	if coreVersion == "" {
		coreVersion = "3.0.0"
	}

	eventBus := opts.EventBus
	if eventBus == nil {
		eventBus = events.New()
	}

	return &Manager{
		plugins:         make(map[string]shared.Plugin),
		extensionPoints: make(map[string][]extensionHandler),
		eventBus:        eventBus,
		coreVersion:     coreVersion,
	}
}

// Initialize initializes the plugin manager.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	m.initialized = true
	return nil
}

// Shutdown shuts down the plugin manager and all plugins.
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	pluginIDs := make([]string, 0, len(m.plugins))
	for id := range m.plugins {
		pluginIDs = append(pluginIDs, id)
	}
	m.mu.Unlock()

	// Shutdown plugins in reverse order
	for i := len(pluginIDs) - 1; i >= 0; i-- {
		m.UnloadPlugin(pluginIDs[i])
	}

	m.mu.Lock()
	m.extensionPoints = make(map[string][]extensionHandler)
	m.initialized = false
	m.mu.Unlock()

	return nil
}

// LoadPlugin loads and initializes a plugin.
func (m *Manager) LoadPlugin(plugin shared.Plugin, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pluginID := plugin.ID()

	// Check if already loaded
	if _, exists := m.plugins[pluginID]; exists {
		m.mu.Unlock()
		m.UnloadPlugin(pluginID)
		m.mu.Lock()
	}

	// Validate configuration if schema provided
	if plugin.ConfigSchema() != nil && config != nil {
		if err := m.validateConfig(plugin.ConfigSchema(), config); err != nil {
			return err
		}
	}

	// Check version compatibility
	if err := m.checkVersionCompatibility(plugin); err != nil {
		return err
	}

	// Check dependencies
	for _, depID := range plugin.Dependencies() {
		if _, exists := m.plugins[depID]; !exists {
			return shared.NewPluginError(
				fmt.Sprintf("Plugin %s depends on %s which is not loaded", pluginID, depID),
				map[string]interface{}{
					"pluginId":   pluginID,
					"dependency": depID,
				},
			)
		}
	}

	// Initialize plugin
	if err := plugin.Initialize(config); err != nil {
		return shared.NewPluginError(
			fmt.Sprintf("Failed to initialize plugin %s: %s", pluginID, err.Error()),
			map[string]interface{}{
				"pluginId": pluginID,
				"error":    err.Error(),
			},
		)
	}

	// Register plugin
	m.plugins[pluginID] = plugin

	// Register extension points
	for _, ep := range plugin.GetExtensionPoints() {
		m.registerExtensionPoint(pluginID, ep)
	}

	// Emit event
	m.eventBus.EmitPluginLoaded(pluginID, plugin.Name())

	return nil
}

// UnloadPlugin unloads a plugin.
func (m *Manager) UnloadPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return nil
	}

	// Check if other plugins depend on this one
	for otherID, other := range m.plugins {
		for _, dep := range other.Dependencies() {
			if dep == pluginID {
				return shared.NewPluginError(
					fmt.Sprintf("Cannot unload %s: plugin %s depends on it", pluginID, otherID),
					map[string]interface{}{
						"pluginId":          pluginID,
						"dependentPluginId": otherID,
					},
				)
			}
		}
	}

	// Shutdown plugin
	if err := plugin.Shutdown(); err != nil {
		// Log but continue
	}

	// Remove extension points
	for name, handlers := range m.extensionPoints {
		filtered := make([]extensionHandler, 0)
		for _, h := range handlers {
			if h.PluginID != pluginID {
				filtered = append(filtered, h)
			}
		}
		m.extensionPoints[name] = filtered
	}

	// Remove plugin
	delete(m.plugins, pluginID)

	// Emit event
	m.eventBus.EmitPluginUnloaded(pluginID)

	return nil
}

// ReloadPlugin reloads a plugin with a new version.
func (m *Manager) ReloadPlugin(pluginID string, plugin shared.Plugin) error {
	if err := m.UnloadPlugin(pluginID); err != nil {
		return err
	}
	return m.LoadPlugin(plugin, nil)
}

// ListPlugins returns all loaded plugins.
func (m *Manager) ListPlugins() []shared.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]shared.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// GetPlugin returns a plugin by ID.
func (m *Manager) GetPlugin(pluginID string) shared.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.plugins[pluginID]
}

// GetPluginMetadata returns plugin metadata.
func (m *Manager) GetPluginMetadata(pluginID string) *shared.PluginMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return nil
	}

	return &shared.PluginMetadata{
		ID:          plugin.ID(),
		Name:        plugin.Name(),
		Version:     plugin.Version(),
		Description: plugin.Description(),
		Author:      plugin.Author(),
		Homepage:    plugin.Homepage(),
	}
}

// InvokeExtensionPoint invokes all handlers for an extension point.
func (m *Manager) InvokeExtensionPoint(ctx context.Context, name string, data interface{}) ([]interface{}, error) {
	m.mu.RLock()
	handlers := m.extensionPoints[name]
	m.mu.RUnlock()

	// Sort by priority (higher first)
	sortedHandlers := make([]extensionHandler, len(handlers))
	copy(sortedHandlers, handlers)
	sort.Slice(sortedHandlers, func(i, j int) bool {
		return sortedHandlers[i].Priority > sortedHandlers[j].Priority
	})

	results := make([]interface{}, 0, len(sortedHandlers))

	for _, h := range sortedHandlers {
		result, err := h.Handler(ctx, data)
		if err != nil {
			results = append(results, map[string]interface{}{
				"error":    err.Error(),
				"pluginId": h.PluginID,
			})
		} else {
			results = append(results, result)
		}
	}

	return results, nil
}

// GetCoreVersion returns the core version.
func (m *Manager) GetCoreVersion() string {
	return m.coreVersion
}

// HasExtensionPoint checks if an extension point exists.
func (m *Manager) HasExtensionPoint(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handlers, exists := m.extensionPoints[name]
	return exists && len(handlers) > 0
}

// ListExtensionPoints returns all extension point names.
func (m *Manager) ListExtensionPoints() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.extensionPoints))
	for name := range m.extensionPoints {
		names = append(names, name)
	}
	return names
}

// ============================================================================
// Private Helper Methods
// ============================================================================

func (m *Manager) registerExtensionPoint(pluginID string, ep shared.ExtensionPoint) {
	handlers := m.extensionPoints[ep.Name]
	handlers = append(handlers, extensionHandler{
		PluginID: pluginID,
		Handler:  ep.Handler,
		Priority: ep.Priority,
	})
	m.extensionPoints[ep.Name] = handlers
}

func (m *Manager) validateConfig(schema, config map[string]interface{}) error {
	// Simple validation - check required fields
	if required, ok := schema["required"].([]interface{}); ok {
		for _, field := range required {
			fieldName, ok := field.(string)
			if !ok {
				continue
			}
			if _, exists := config[fieldName]; !exists {
				return shared.NewPluginError(
					fmt.Sprintf("Missing required configuration field: %s", fieldName),
					map[string]interface{}{
						"field":      fieldName,
						"validation": "required",
					},
				)
			}
		}
	}
	return nil
}

func (m *Manager) checkVersionCompatibility(plugin shared.Plugin) error {
	coreVersion := m.parseVersion(m.coreVersion)

	if minVersion := plugin.MinCoreVersion(); minVersion != "" {
		min := m.parseVersion(minVersion)
		if m.compareVersions(coreVersion, min) < 0 {
			return shared.NewPluginError(
				fmt.Sprintf("Plugin %s requires core version >= %s, but core version is %s",
					plugin.ID(), minVersion, m.coreVersion),
				map[string]interface{}{
					"pluginId":    plugin.ID(),
					"minVersion":  minVersion,
					"coreVersion": m.coreVersion,
				},
			)
		}
	}

	if maxVersion := plugin.MaxCoreVersion(); maxVersion != "" {
		max := m.parseVersion(maxVersion)
		if m.compareVersions(coreVersion, max) > 0 {
			return shared.NewPluginError(
				fmt.Sprintf("Plugin %s requires core version <= %s, but core version is %s",
					plugin.ID(), maxVersion, m.coreVersion),
				map[string]interface{}{
					"pluginId":    plugin.ID(),
					"maxVersion":  maxVersion,
					"coreVersion": m.coreVersion,
				},
			)
		}
	}

	return nil
}

func (m *Manager) parseVersion(version string) []int {
	parts := strings.Split(version, ".")
	result := make([]int, len(parts))
	for i, p := range parts {
		// Remove any non-numeric suffix (e.g., "-alpha")
		numPart := strings.Split(p, "-")[0]
		n, _ := strconv.Atoi(numPart)
		result[i] = n
	}
	return result
}

func (m *Manager) compareVersions(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	for i := 0; i < maxLen; i++ {
		av := 0
		if i < len(a) {
			av = a[i]
		}
		bv := 0
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}

// ============================================================================
// Base Plugin Implementation
// ============================================================================

// BasePlugin provides a base implementation of the Plugin interface.
type BasePlugin struct {
	id             string
	name           string
	version        string
	description    string
	author         string
	homepage       string
	priority       int
	dependencies   []string
	configSchema   map[string]interface{}
	minCoreVersion string
	maxCoreVersion string
	extensionPoints []shared.ExtensionPoint
	initialized    bool
}

// BasePluginConfig holds configuration for creating a base plugin.
type BasePluginConfig struct {
	ID             string
	Name           string
	Version        string
	Description    string
	Author         string
	Homepage       string
	Priority       int
	Dependencies   []string
	ConfigSchema   map[string]interface{}
	MinCoreVersion string
	MaxCoreVersion string
}

// NewBasePlugin creates a new base plugin.
func NewBasePlugin(config BasePluginConfig) *BasePlugin {
	return &BasePlugin{
		id:             config.ID,
		name:           config.Name,
		version:        config.Version,
		description:    config.Description,
		author:         config.Author,
		homepage:       config.Homepage,
		priority:       config.Priority,
		dependencies:   config.Dependencies,
		configSchema:   config.ConfigSchema,
		minCoreVersion: config.MinCoreVersion,
		maxCoreVersion: config.MaxCoreVersion,
		extensionPoints: make([]shared.ExtensionPoint, 0),
	}
}

func (p *BasePlugin) ID() string                          { return p.id }
func (p *BasePlugin) Name() string                        { return p.name }
func (p *BasePlugin) Version() string                     { return p.version }
func (p *BasePlugin) Description() string                 { return p.description }
func (p *BasePlugin) Author() string                      { return p.author }
func (p *BasePlugin) Homepage() string                    { return p.homepage }
func (p *BasePlugin) Priority() int                       { return p.priority }
func (p *BasePlugin) Dependencies() []string              { return p.dependencies }
func (p *BasePlugin) ConfigSchema() map[string]interface{} { return p.configSchema }
func (p *BasePlugin) MinCoreVersion() string              { return p.minCoreVersion }
func (p *BasePlugin) MaxCoreVersion() string              { return p.maxCoreVersion }

func (p *BasePlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	return nil
}

func (p *BasePlugin) Shutdown() error {
	p.initialized = false
	return nil
}

func (p *BasePlugin) GetExtensionPoints() []shared.ExtensionPoint {
	return p.extensionPoints
}

// AddExtensionPoint adds an extension point to the plugin.
func (p *BasePlugin) AddExtensionPoint(ep shared.ExtensionPoint) {
	p.extensionPoints = append(p.extensionPoints, ep)
}
