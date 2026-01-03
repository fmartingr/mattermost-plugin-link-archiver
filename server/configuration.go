package main

import (
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type ArchivalRule struct {
	Kind         string `json:"kind"`         // "hostname" or "mimetype"
	Pattern      string `json:"pattern"`      // Pattern value (e.g., "*.example.com" or "image/*")
	ArchivalTool string `json:"archivalTool"` // e.g., "direct_download"
}

type configuration struct {
	ArchivalRules       []ArchivalRule `json:"archivalRules"`
	DefaultArchivalTool string         `json:"defaultArchivalTool"`
}

// rawConfiguration is used to load the raw config from Mattermost
// ArchivalRules is stored as a JSON string for custom settings
// The JSON tag must match the key in plugin.json exactly
// Since DefaultArchivalTool is now part of the custom setting, we don't need it here
type rawConfiguration struct {
	MimeTypeMappings string `json:"MimeTypeMappings"` // Custom setting stored as JSON string containing both rules and default tool (kept for backward compatibility)
}

// Clone deep copies the configuration to handle the slice field.
func (c *configuration) Clone() *configuration {
	var clone = *c
	if c.ArchivalRules != nil {
		clone.ArchivalRules = make([]ArchivalRule, len(c.ArchivalRules))
		copy(clone.ArchivalRules, c.ArchivalRules)
	}
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
// MIME type mappings are loaded from KV store and merged with the configuration.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	config := &configuration{}
	if p.configuration != nil {
		config.DefaultArchivalTool = p.configuration.DefaultArchivalTool
		config.ArchivalRules = p.configuration.ArchivalRules
	}

	// Load archival rules from KV store (always use latest from KV store)
	// Note: This is done here to ensure the archive processor always has the latest rules
	// In a production system, you might want to cache this and invalidate on updates
	archivalRules, err := p.loadArchivalRules()
	if err != nil {
		// Log error but continue with existing rules if available
		p.API.LogError("Failed to load archival rules from KV store", "error", err.Error())
		if config.ArchivalRules == nil {
			config.ArchivalRules = []ArchivalRule{}
		}
	} else {
		config.ArchivalRules = archivalRules
		p.API.LogDebug("Loaded archival rules from KV store", "count", len(archivalRules))
	}

	// Load default archival tool from KV store (always use latest from KV store)
	defaultTool, err := p.loadDefaultArchivalTool()
	switch {
	case err != nil:
		// Log error but continue with existing default if available
		p.API.LogError("Failed to load default archival tool from KV store", "error", err.Error())
		if config.DefaultArchivalTool == "" {
			config.DefaultArchivalTool = "do_nothing"
		}
	case defaultTool != "":
		config.DefaultArchivalTool = defaultTool
		p.API.LogDebug("Loaded default archival tool from KV store", "tool", defaultTool)
	case config.DefaultArchivalTool == "":
		// Fallback if nothing is set
		config.DefaultArchivalTool = "do_nothing"
	}

	return config
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var rawConfig = new(rawConfiguration)

	// Load the raw configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(rawConfig); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	// Parse the custom setting value which contains both archival rules and default tool
	var archivalRules []ArchivalRule
	defaultArchivalTool := "do_nothing" // Default fallback

	if rawConfig.MimeTypeMappings != "" {
		// The custom setting value is a JSON string containing the full config
		// Try to parse as new format first (archivalRules), then fall back to old format (mimeTypeMappings) for migration
		var customConfig struct {
			ArchivalRules    []ArchivalRule `json:"archivalRules"`
			MimeTypeMappings []struct {
				MimeTypePattern string `json:"mimeTypePattern"`
				ArchivalTool    string `json:"archivalTool"`
			} `json:"mimeTypeMappings"` // For backward compatibility
			DefaultArchivalTool string `json:"defaultArchivalTool"`
		}
		if err := json.Unmarshal([]byte(rawConfig.MimeTypeMappings), &customConfig); err != nil {
			p.API.LogWarn("Failed to parse custom setting value, will use KV store", "error", err.Error())
			// If parsing fails, try loading from KV store instead
			var loadErr error
			archivalRules, loadErr = p.loadArchivalRules()
			if loadErr != nil {
				p.API.LogError("Failed to load archival rules from KV store", "error", loadErr.Error())
				archivalRules = []ArchivalRule{}
			}
			// Try to load default tool from existing config
			currentConfig := p.getConfiguration()
			if currentConfig != nil && currentConfig.DefaultArchivalTool != "" {
				defaultArchivalTool = currentConfig.DefaultArchivalTool
			}
		} else {
			// Successfully parsed from custom setting
			if len(customConfig.ArchivalRules) > 0 {
				// New format
				archivalRules = customConfig.ArchivalRules
			} else if len(customConfig.MimeTypeMappings) > 0 {
				// Old format - migrate to new format
				archivalRules = make([]ArchivalRule, len(customConfig.MimeTypeMappings))
				for i, mapping := range customConfig.MimeTypeMappings {
					archivalRules[i] = ArchivalRule{
						Kind:         "mimetype",
						Pattern:      mapping.MimeTypePattern,
						ArchivalTool: mapping.ArchivalTool,
					}
				}
				p.API.LogInfo("Migrated MIME type mappings to archival rules", "count", len(archivalRules))
			}
			if customConfig.DefaultArchivalTool != "" {
				defaultArchivalTool = customConfig.DefaultArchivalTool
			}
			// Save to KV store for consistency (both rules and default tool)
			if err := p.saveArchivalRules(archivalRules); err != nil {
				p.API.LogWarn("Failed to save archival rules to KV store after parsing from custom setting", "error", err.Error())
			}
			// Also save default tool to a separate KV key for quick access
			if err := p.saveDefaultArchivalTool(defaultArchivalTool); err != nil {
				p.API.LogWarn("Failed to save default archival tool to KV store", "error", err.Error())
			}
		}
	} else {
		// No custom setting value, try loading from KV store
		var loadErr error
		archivalRules, loadErr = p.loadArchivalRules()
		if loadErr != nil {
			p.API.LogError("Failed to load archival rules from KV store", "error", loadErr.Error())
			archivalRules = []ArchivalRule{}
		}
		// Try to load default tool from KV store
		loadedDefault, loadErr := p.loadDefaultArchivalTool()
		if loadErr == nil && loadedDefault != "" {
			defaultArchivalTool = loadedDefault
		} else {
			// Fallback to existing config
			currentConfig := p.getConfiguration()
			if currentConfig != nil && currentConfig.DefaultArchivalTool != "" {
				defaultArchivalTool = currentConfig.DefaultArchivalTool
			}
		}
	}

	// Ensure there's always a default rule at the end (empty pattern)
	if len(archivalRules) == 0 || archivalRules[len(archivalRules)-1].Pattern != "" {
		// Add default rule if it doesn't exist
		archivalRules = append(archivalRules, ArchivalRule{
			Kind:         "mimetype",
			Pattern:      "", // Empty pattern means always match
			ArchivalTool: defaultArchivalTool,
		})
	} else {
		// Update existing default rule
		archivalRules[len(archivalRules)-1].ArchivalTool = defaultArchivalTool
	}

	// Create the configuration struct
	config := &configuration{
		DefaultArchivalTool: defaultArchivalTool,
		ArchivalRules:       archivalRules,
	}

	p.setConfiguration(config)

	return nil
}

const archivalRulesKey = "archival_rules"
const mimeTypeMappingsKey = "mime_type_mappings" // Kept for backward compatibility migration
const defaultArchivalToolKey = "default_archival_tool"

// saveArchivalRules saves archival rules to KV store
func (p *Plugin) saveArchivalRules(rules []ArchivalRule) error {
	data, err := json.Marshal(rules)
	if err != nil {
		return err
	}

	appErr := p.API.KVSet(archivalRulesKey, data)
	if appErr != nil {
		return appErr
	}

	return nil
}

// loadArchivalRules loads archival rules from KV store
// Also attempts to migrate old mimeTypeMappings if archivalRules don't exist
func (p *Plugin) loadArchivalRules() ([]ArchivalRule, error) {
	data, appErr := p.API.KVGet(archivalRulesKey)
	if appErr != nil {
		return nil, appErr
	}

	if data != nil {
		var rules []ArchivalRule
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, err
		}
		return rules, nil
	}

	// Try to migrate from old format
	oldData, appErr := p.API.KVGet(mimeTypeMappingsKey)
	if appErr != nil {
		return []ArchivalRule{}, nil
	}

	if oldData != nil {
		// Old format - migrate to new format
		var oldMappings []struct {
			MimeTypePattern string `json:"mimeTypePattern"`
			ArchivalTool    string `json:"archivalTool"`
		}
		if err := json.Unmarshal(oldData, &oldMappings); err == nil {
			rules := make([]ArchivalRule, len(oldMappings))
			for i, mapping := range oldMappings {
				rules[i] = ArchivalRule{
					Kind:         "mimetype",
					Pattern:      mapping.MimeTypePattern,
					ArchivalTool: mapping.ArchivalTool,
				}
			}
			// Save in new format
			if err := p.saveArchivalRules(rules); err != nil {
				p.API.LogWarn("Failed to save migrated archival rules", "error", err.Error())
			} else {
				p.API.LogInfo("Migrated MIME type mappings to archival rules", "count", len(rules))
			}
			return rules, nil
		}
	}

	// No rules stored yet, return empty slice
	return []ArchivalRule{}, nil
}

// saveDefaultArchivalTool saves the default archival tool to KV store
func (p *Plugin) saveDefaultArchivalTool(tool string) error {
	if tool == "" {
		tool = "do_nothing"
	}
	appErr := p.API.KVSet(defaultArchivalToolKey, []byte(tool))
	if appErr != nil {
		return appErr
	}
	return nil
}

// loadDefaultArchivalTool loads the default archival tool from KV store
func (p *Plugin) loadDefaultArchivalTool() (string, error) {
	data, appErr := p.API.KVGet(defaultArchivalToolKey)
	if appErr != nil {
		return "", appErr
	}

	if data == nil {
		return "", nil
	}

	return string(data), nil
}
