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
type rawConfiguration struct {
	MimeTypeMappings string `json:"MimeTypeMappings"` // Custom setting stored as JSON string containing both rules and default tool
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
		// Filter out any default rules that might exist (from old format or migration)
		archivalRules = p.filterDefaultRules(archivalRules)
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

	// Append synthetic default rule with kind "default" (system-generated)
	// This ensures there's always a fallback rule that matches everything
	config.ArchivalRules = append(config.ArchivalRules, ArchivalRule{
		Kind:         "default",
		Pattern:      "", // Not used for default rules
		ArchivalTool: config.DefaultArchivalTool,
	})

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
		var customConfig struct {
			ArchivalRules       []ArchivalRule `json:"archivalRules"`
			DefaultArchivalTool string         `json:"defaultArchivalTool"`
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
			archivalRules = customConfig.ArchivalRules
			if customConfig.DefaultArchivalTool != "" {
				defaultArchivalTool = customConfig.DefaultArchivalTool
			}
			// Validate rules before saving
			if err := p.validateArchivalRules(archivalRules); err != nil {
				p.API.LogError("Invalid archival rules in configuration", "error", err.Error())
				return errors.Wrap(err, "invalid archival rules")
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

	// Filter out any default rules that might exist (users shouldn't create them)
	archivalRules = p.filterDefaultRules(archivalRules)

	// Validate rules before using them
	if err := p.validateArchivalRules(archivalRules); err != nil {
		p.API.LogError("Invalid archival rules in configuration", "error", err.Error())
		return errors.Wrap(err, "invalid archival rules")
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

	// No rules stored yet, return empty slice
	return []ArchivalRule{}, nil
}

// filterDefaultRules removes any default rules from the rules slice
// This is used to clean up old format rules or rules that shouldn't be stored
func (p *Plugin) filterDefaultRules(rules []ArchivalRule) []ArchivalRule {
	filtered := make([]ArchivalRule, 0, len(rules))
	for _, rule := range rules {
		// Filter out rules with kind "default" or empty pattern (old default rule format)
		if rule.Kind != "default" && rule.Pattern != "" {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

// validateArchivalRules validates that all rules are valid
// Returns an error if any rule is invalid
func (p *Plugin) validateArchivalRules(rules []ArchivalRule) error {
	for i, rule := range rules {
		// Check that rule has a kind
		if rule.Kind == "" {
			return errors.Errorf("rule at index %d must have a kind (hostname or mimetype)", i)
		}
		// Reject "default" kind - it's system-generated only
		if rule.Kind == "default" {
			return errors.Errorf("rule at index %d has invalid kind 'default'. The default rule is system-generated and cannot be created by users", i)
		}
		// Check that kind is valid
		if rule.Kind != "hostname" && rule.Kind != "mimetype" {
			return errors.Errorf("rule at index %d has invalid kind '%s'. Must be 'hostname' or 'mimetype'", i, rule.Kind)
		}
		// Require pattern for hostname and mimetype rules
		if rule.Pattern == "" {
			return errors.Errorf("rule at index %d (kind: %s) must have a pattern", i, rule.Kind)
		}
		// Check that archival tool is specified
		if rule.ArchivalTool == "" {
			return errors.Errorf("rule at index %d must have an archival tool", i)
		}
	}
	return nil
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
