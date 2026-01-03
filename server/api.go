package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
// The root URL is currently <siteUrl>/plugins/com.mattermost.link-archiver/api/v1/. Replace com.mattermost.link-archiver with the plugin ID.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()

	// Middleware to require that the user is logged in
	router.Use(p.MattermostAuthorizationRequired)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/hello", p.HelloWorld).Methods(http.MethodGet)
	apiRouter.HandleFunc("/config", p.GetConfig).Methods(http.MethodGet)
	apiRouter.HandleFunc("/config", p.UpdateConfig).Methods(http.MethodPost)
	apiRouter.HandleFunc("/archives/{postId}", p.GetArchives).Methods(http.MethodGet)
	apiRouter.HandleFunc("/archival-tools", p.GetArchivalTools).Methods(http.MethodGet)

	router.ServeHTTP(w, r)
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			// Log for debugging - Mattermost should automatically add this header
			p.API.LogWarn("Missing Mattermost-User-ID header in request", "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) HelloWorld(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		p.API.LogError("Failed to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetConfig returns the current plugin configuration (admin only)
func (p *Plugin) GetConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is system admin
	user, appErr := p.API.GetUser(userID)
	if appErr != nil || !user.IsInRole(model.SystemAdminRoleId) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	config := p.getConfiguration()

	// getConfiguration already loads archival rules from KV store
	// Return the full configuration
	fullConfig := struct {
		ArchivalRules       []ArchivalRule `json:"archivalRules"`
		DefaultArchivalTool string         `json:"defaultArchivalTool"`
	}{
		ArchivalRules:       config.ArchivalRules,
		DefaultArchivalTool: config.DefaultArchivalTool,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fullConfig); err != nil {
		p.API.LogError("Failed to encode config", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UpdateConfig updates the plugin configuration (admin only)
func (p *Plugin) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is system admin
	user, appErr := p.API.GetUser(userID)
	if appErr != nil || !user.IsInRole(model.SystemAdminRoleId) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var requestConfig struct {
		ArchivalRules    []ArchivalRule `json:"archivalRules"`
		MimeTypeMappings []struct {
			MimeTypePattern string `json:"mimeTypePattern"`
			ArchivalTool    string `json:"archivalTool"`
		} `json:"mimeTypeMappings"` // For backward compatibility
		DefaultArchivalTool string `json:"defaultArchivalTool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestConfig); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate configuration
	if requestConfig.DefaultArchivalTool == "" {
		requestConfig.DefaultArchivalTool = "do_nothing"
	}

	// Migrate old format to new format if needed
	archivalRules := requestConfig.ArchivalRules
	if len(archivalRules) == 0 && len(requestConfig.MimeTypeMappings) > 0 {
		// Migrate old format to new format
		archivalRules = make([]ArchivalRule, len(requestConfig.MimeTypeMappings))
		for i, mapping := range requestConfig.MimeTypeMappings {
			archivalRules[i] = ArchivalRule{
				Kind:         "mimetype",
				Pattern:      mapping.MimeTypePattern,
				ArchivalTool: mapping.ArchivalTool,
			}
		}
		p.API.LogInfo("Migrated MIME type mappings to archival rules", "count", len(archivalRules))
	}

	// Validate that each rule has required fields
	for i, rule := range archivalRules {
		if rule.Kind == "" {
			http.Error(w, fmt.Sprintf("Rule at index %d must have a kind (hostname or mimetype)", i), http.StatusBadRequest)
			return
		}
		if rule.Kind != "hostname" && rule.Kind != "mimetype" {
			http.Error(w, fmt.Sprintf("Rule at index %d has invalid kind '%s'. Must be 'hostname' or 'mimetype'", i, rule.Kind), http.StatusBadRequest)
			return
		}
		if rule.Pattern == "" {
			http.Error(w, fmt.Sprintf("Rule at index %d must have a pattern", i), http.StatusBadRequest)
			return
		}
		if rule.ArchivalTool == "" {
			http.Error(w, fmt.Sprintf("Rule at index %d must have an archival tool", i), http.StatusBadRequest)
			return
		}
	}

	// Save default archival tool to KV store (this persists)
	if err := p.saveDefaultArchivalTool(requestConfig.DefaultArchivalTool); err != nil {
		p.API.LogError("Failed to save default archival tool to KV store", "error", err.Error())
		http.Error(w, "Failed to save default archival tool", http.StatusInternalServerError)
		return
	}

	// Save archival rules to KV store (this persists)
	if err := p.saveArchivalRules(archivalRules); err != nil {
		p.API.LogError("Failed to save archival rules to KV store", "error", err.Error())
		http.Error(w, "Failed to save archival rules", http.StatusInternalServerError)
		return
	}

	// Update in-memory configuration
	p.configurationLock.Lock()
	if p.configuration == nil {
		p.configuration = &configuration{}
	}
	p.configuration.DefaultArchivalTool = requestConfig.DefaultArchivalTool
	p.configuration.ArchivalRules = archivalRules
	p.configurationLock.Unlock()

	// Return the full configuration
	responseConfig := struct {
		ArchivalRules       []ArchivalRule `json:"archivalRules"`
		DefaultArchivalTool string         `json:"defaultArchivalTool"`
	}{
		ArchivalRules:       archivalRules,
		DefaultArchivalTool: requestConfig.DefaultArchivalTool,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(responseConfig); err != nil {
		p.API.LogError("Failed to encode config", "error", err)
	}
}

// GetArchivalTools returns the list of available archival tools (admin only)
func (p *Plugin) GetArchivalTools(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is system admin
	user, appErr := p.API.GetUser(userID)
	if appErr != nil || !user.IsInRole(model.SystemAdminRoleId) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get available tools from archive processor
	if p.archiveProcessor == nil {
		http.Error(w, "Archive processor not initialized", http.StatusInternalServerError)
		return
	}

	tools := p.archiveProcessor.GetAvailableArchivalTools()

	// Return as JSON
	response := struct {
		Tools []string `json:"tools"`
	}{
		Tools: tools,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.API.LogError("Failed to encode archival tools", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetArchives returns archive information for a specific post
func (p *Plugin) GetArchives(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	postID := vars["postId"]
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Get post to verify user has access
	post, appErr := p.API.GetPost(postID)
	if appErr != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Check if user has permission to view the channel
	channel, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	// Check channel membership
	if !channel.IsOpen() && !channel.IsGroupOrDirect() {
		member, appErr := p.API.GetChannelMember(post.ChannelId, userID)
		if appErr != nil || member == nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Get archive metadata from KV store
	// Note: This is a simplified implementation. In practice, you'd need to
	// maintain an index of URLs per post or scan keys.
	// For now, we'll return an empty list as the storage service handles metadata differently
	archives := []*ArchiveMetadata{}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(archives); err != nil {
		p.API.LogError("Failed to encode archives", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
