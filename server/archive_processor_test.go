package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupTestProcessor creates a minimal ArchiveProcessor for testing
func setupTestProcessor() *ArchiveProcessor {
	api := &plugintest.API{}
	// Mock API calls that might be used during testing
	// LogDebug, LogInfo, and LogError accept a message string followed by variadic key-value pairs
	// We need to match all possible argument combinations, so we use mock.Anything for each position
	// and Maybe() to make the mock optional (won't fail if not called)
	// Add mocks for different argument counts (1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21 arguments)
	for i := 1; i <= 21; i += 2 {
		args := make([]interface{}, i)
		for j := range args {
			args[j] = mock.Anything
		}
		api.On("LogDebug", args...).Maybe().Return(nil)
		api.On("LogInfo", args...).Maybe().Return(nil)
		api.On("LogError", args...).Maybe().Return(nil)
	}

	processor := &ArchiveProcessor{
		api: api,
	}

	return processor
}

func TestHostnameMatches(t *testing.T) {
	processor := setupTestProcessor()

	tests := []struct {
		name     string
		hostname string
		pattern  string
		expected bool
	}{
		// Exact matches
		{
			name:     "exact match",
			hostname: "example.com",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "exact match different domain",
			hostname: "other.com",
			pattern:  "other.com",
			expected: true,
		},
		{
			name:     "exact match no match",
			hostname: "example.com",
			pattern:  "other.com",
			expected: false,
		},

		// Wildcard prefix matches
		{
			name:     "wildcard matches subdomain",
			hostname: "www.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard matches api subdomain",
			hostname: "api.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard matches nested subdomain",
			hostname: "sub.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard matches base domain",
			hostname: "example.com",
			pattern:  "*.example.com",
			expected: true,
		},

		// Wildcard doesn't match
		{
			name:     "wildcard doesn't match different domain",
			hostname: "example.org",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "wildcard doesn't match other domain",
			hostname: "other.com",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "wildcard doesn't match parent domain",
			hostname: "com",
			pattern:  "*.example.com",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty hostname",
			hostname: "",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "empty pattern",
			hostname: "example.com",
			pattern:  "",
			expected: false,
		},
		{
			name:     "invalid wildcard pattern",
			hostname: "example.com",
			pattern:  "*.",
			expected: false,
		},
		{
			name:     "case sensitive exact match",
			hostname: "Example.COM",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "case sensitive wildcard",
			hostname: "WWW.EXAMPLE.COM",
			pattern:  "*.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.hostnameMatches(tt.hostname, tt.pattern)
			assert.Equal(t, tt.expected, result, "hostnameMatches(%q, %q) = %v, want %v", tt.hostname, tt.pattern, result, tt.expected)
		})
	}
}

func TestMimeTypeMatches(t *testing.T) {
	processor := setupTestProcessor()

	tests := []struct {
		name     string
		mimeType string
		pattern  string
		expected bool
	}{
		// Exact matches
		{
			name:     "exact match application/pdf",
			mimeType: "application/pdf",
			pattern:  "application/pdf",
			expected: true,
		},
		{
			name:     "exact match image/jpeg",
			mimeType: "image/jpeg",
			pattern:  "image/jpeg",
			expected: true,
		},
		{
			name:     "exact match no match",
			mimeType: "application/pdf",
			pattern:  "image/jpeg",
			expected: false,
		},

		// Wildcard suffix matches
		{
			name:     "wildcard matches image/jpeg",
			mimeType: "image/jpeg",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "wildcard matches image/png",
			mimeType: "image/png",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "wildcard matches image/gif",
			mimeType: "image/gif",
			pattern:  "image/*",
			expected: true,
		},
		{
			name:     "wildcard matches application/json",
			mimeType: "application/json",
			pattern:  "application/*",
			expected: true,
		},

		// Wildcard doesn't match
		{
			name:     "wildcard doesn't match different type",
			mimeType: "application/pdf",
			pattern:  "image/*",
			expected: false,
		},
		{
			name:     "wildcard doesn't match text/html",
			mimeType: "text/html",
			pattern:  "image/*",
			expected: false,
		},
		{
			name:     "wildcard doesn't match without slash",
			mimeType: "image",
			pattern:  "image/*",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty mimeType",
			mimeType: "",
			pattern:  "image/*",
			expected: false,
		},
		{
			name:     "empty pattern",
			mimeType: "image/jpeg",
			pattern:  "",
			expected: false,
		},
		{
			name:     "pattern without wildcard",
			mimeType: "image/jpeg",
			pattern:  "image",
			expected: false,
		},
		{
			name:     "malformed mime type",
			mimeType: "invalid",
			pattern:  "image/*",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.mimeTypeMatches(tt.mimeType, tt.pattern)
			assert.Equal(t, tt.expected, result, "mimeTypeMatches(%q, %q) = %v, want %v", tt.mimeType, tt.pattern, result, tt.expected)
		})
	}
}

func TestRuleMatches(t *testing.T) {
	processor := setupTestProcessor()

	tests := []struct {
		name     string
		hostname string
		mimeType string
		rule     ArchivalRule
		expected bool
	}{
		// Hostname kind matching
		{
			name:     "hostname kind exact match",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "hostname",
				Pattern:      "example.com",
				ArchivalTool: "direct_download",
			},
			expected: true,
		},
		{
			name:     "hostname kind wildcard match",
			hostname: "www.example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "hostname",
				Pattern:      "*.example.com",
				ArchivalTool: "direct_download",
			},
			expected: true,
		},
		{
			name:     "hostname kind no match",
			hostname: "other.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "hostname",
				Pattern:      "example.com",
				ArchivalTool: "direct_download",
			},
			expected: false,
		},

		// MIME type kind matching
		{
			name:     "mimetype kind exact match",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "mimetype",
				Pattern:      "application/pdf",
				ArchivalTool: "direct_download",
			},
			expected: true,
		},
		{
			name:     "mimetype kind wildcard match",
			hostname: "example.com",
			mimeType: "image/jpeg",
			rule: ArchivalRule{
				Kind:         "mimetype",
				Pattern:      "image/*",
				ArchivalTool: "direct_download",
			},
			expected: true,
		},
		{
			name:     "mimetype kind no match",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "mimetype",
				Pattern:      "image/*",
				ArchivalTool: "direct_download",
			},
			expected: false,
		},

		// Default rule (kind: "default")
		{
			name:     "default kind always matches",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "default",
				Pattern:      "",
				ArchivalTool: "do_nothing",
			},
			expected: true,
		},
		{
			name:     "default kind matches any hostname",
			hostname: "any-domain.com",
			mimeType: "any/type",
			rule: ArchivalRule{
				Kind:         "default",
				Pattern:      "",
				ArchivalTool: "do_nothing",
			},
			expected: true,
		},

		// Invalid kind
		{
			name:     "unknown kind returns false",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "unknown",
				Pattern:      "example.com",
				ArchivalTool: "direct_download",
			},
			expected: false,
		},

		// Empty kind
		{
			name:     "empty kind returns false",
			hostname: "example.com",
			mimeType: "application/pdf",
			rule: ArchivalRule{
				Kind:         "",
				Pattern:      "example.com",
				ArchivalTool: "direct_download",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ruleMatches(tt.hostname, tt.mimeType, tt.rule)
			assert.Equal(t, tt.expected, result, "ruleMatches(%q, %q, %+v) = %v, want %v", tt.hostname, tt.mimeType, tt.rule, result, tt.expected)
		})
	}
}

func TestFindArchivalTool(t *testing.T) {
	processor := setupTestProcessor()

	t.Run("hostname rule matching", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "example.com",
					ArchivalTool: "direct_download",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "do_nothing",
				},
			},
		}

		result := processor.findArchivalTool("https://example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "direct_download", result)
	})

	t.Run("mimetype rule matching", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "mimetype",
					Pattern:      "application/pdf",
					ArchivalTool: "direct_download",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "do_nothing",
				},
			},
		}

		result := processor.findArchivalTool("https://example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "direct_download", result)
	})

	t.Run("default rule fallback", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "direct_download",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "obelisk",
				},
			},
		}

		result := processor.findArchivalTool("https://other.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "obelisk", result)
	})

	t.Run("no rules fallback", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{},
		}

		result := processor.findArchivalTool("https://example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "do_nothing", result)
	})

	t.Run("invalid URL handling", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "do_nothing",
				},
			},
		}

		result := processor.findArchivalTool("not-a-valid-url", "application/pdf", config)
		assert.Equal(t, "do_nothing", result)
	})

	t.Run("rule ordering - first match wins with wildcard before specific", func(t *testing.T) {
		// This tests that *.example.com matches before www.example.com would
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "tool1",
				},
				{
					Kind:         "hostname",
					Pattern:      "www.example.com",
					ArchivalTool: "tool2",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://www.example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "tool1", result, "First rule should match, not the second")
	})

	t.Run("rule ordering - first match wins with different domains", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.github.com",
					ArchivalTool: "obelisk",
				},
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "direct_download",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://api.github.com/page.html", "text/html", config)
		assert.Equal(t, "obelisk", result, "First rule should match")
	})

	t.Run("rule ordering - first match wins with MIME types", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "mimetype",
					Pattern:      "image/*",
					ArchivalTool: "tool1",
				},
				{
					Kind:         "mimetype",
					Pattern:      "image/png",
					ArchivalTool: "tool2",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://example.com/image.png", "image/png", config)
		assert.Equal(t, "tool1", result, "First rule should match, not the second")
	})

	t.Run("rule ordering - default rule is last", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "tool1",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://other.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "default", result, "Default rule should match when no other rules match")
	})

	t.Run("default rule matches correctly after other rules are checked", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "specific_tool",
				},
				{
					Kind:         "mimetype",
					Pattern:      "image/*",
					ArchivalTool: "image_tool",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default_tool",
				},
			},
		}

		// Test cases where other rules should match first (default should NOT be used)
		t.Run("other rules match first", func(t *testing.T) {
			// Hostname rule should match
			result := processor.findArchivalTool("https://www.example.com/file.pdf", "application/pdf", config)
			assert.Equal(t, "specific_tool", result, "Hostname rule should match, not default rule")

			// MIME type rule should match
			result = processor.findArchivalTool("https://other.com/image.png", "image/png", config)
			assert.Equal(t, "image_tool", result, "MIME type rule should match, not default rule")
		})

		// Test cases where no other rules match (default should match)
		testCases := []struct {
			name     string
			url      string
			mimeType string
		}{
			{"no matching hostname or MIME", "https://other.com/file.pdf", "application/pdf"},
			{"different hostname", "https://github.com/page.html", "text/html"},
			{"subdomain hostname not matching pattern", "https://www.test.com/file.pdf", "application/pdf"},
			{"different MIME type not matching pattern", "https://test.com/file.json", "application/json"},
			{"empty MIME type", "https://test.com/file", ""},
			{"invalid URL hostname", "not-a-valid-url", "application/pdf"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := processor.findArchivalTool(tc.url, tc.mimeType, config)
				assert.Equal(t, "default_tool", result, "Default rule should match when no other rules match for URL: %s, MIME: %s", tc.url, tc.mimeType)
			})
		}
	})

	t.Run("rule ordering - exact match before wildcard", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "example.com",
					ArchivalTool: "tool1",
				},
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "tool2",
				},
				{
					Kind:         "mimetype",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "tool1", result, "Exact match rule should match first")
	})

	t.Run("rule ordering - multiple matching rules, first wins", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "first_tool",
				},
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "second_tool",
				},
				{
					Kind:         "hostname",
					Pattern:      "*.example.com",
					ArchivalTool: "third_tool",
				},
				{
					Kind:         "mimetype",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		result := processor.findArchivalTool("https://www.example.com/file.pdf", "application/pdf", config)
		assert.Equal(t, "first_tool", result, "First matching rule should be selected")
	})

	t.Run("rule ordering - mixed rule kinds", func(t *testing.T) {
		config := &configuration{
			ArchivalRules: []ArchivalRule{
				{
					Kind:         "hostname",
					Pattern:      "*.github.com",
					ArchivalTool: "hostname_tool",
				},
				{
					Kind:         "mimetype",
					Pattern:      "text/html",
					ArchivalTool: "mimetype_tool",
				},
				{
					Kind:         "default",
					Pattern:      "",
					ArchivalTool: "default",
				},
			},
		}

		// Hostname rule should match first
		result := processor.findArchivalTool("https://api.github.com/file.html", "text/html", config)
		assert.Equal(t, "hostname_tool", result, "First rule (hostname) should match before second rule (mimetype)")
	})
}
