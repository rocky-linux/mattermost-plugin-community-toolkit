package main

import (
	"fmt"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
)

func TestWordListToRegex(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			BadWordsList: "abc,def ghi",
		},
	}

	t.Run("Build Regex", func(t *testing.T) {
		regexStr := wordListToRegex(p.getConfiguration().BadWordsList, defaultRegexTemplate)

		assert.Equal(t, regexStr, `(?mi)\b(def ghi|abc)\b`)
	})

	p2 := Plugin{
		configuration: &configuration{
			BadWordsList: "abc,abc def",
		},
	}

	t.Run("Build regex with duplicate words using default template", func(t *testing.T) {
		regexStr := wordListToRegex(p2.getConfiguration().BadWordsList, defaultRegexTemplate)

		assert.Equal(t, regexStr, `(?mi)\b(abc def|abc)\b`)
	})

	t.Run("Build regex with duplicate words using custom template", func(t *testing.T) {
		regexStr := wordListToRegex(p2.getConfiguration().BadWordsList, `(?mi)^(%s)$`)

		assert.Equal(t, regexStr, `(?mi)^(abc def|abc)$`)
	})
}

// Mock API for configuration tests
type MockConfigAPI struct {
	plugin.API
	LoadPluginConfigurationFunc func(dest interface{}) error
}

func (m *MockConfigAPI) LoadPluginConfiguration(dest interface{}) error {
	if m.LoadPluginConfigurationFunc != nil {
		return m.LoadPluginConfigurationFunc(dest)
	}
	return nil
}

func TestOnConfigurationChange(t *testing.T) {
	t.Run("loads valid configuration successfully", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				// Cast to configuration and set values
				if cfg, ok := dest.(*configuration); ok {
					cfg.BadWordsList = "test,word"
					cfg.BadDomainsList = "bad.com"
					cfg.BadUsernamesList = "baduser"
					cfg.CensorCharacter = "\\*"
					cfg.RejectPosts = true
					cfg.ExcludeBots = true
					cfg.BlockNewUserPM = true
					cfg.BlockNewUserPMTime = "24h"
					cfg.WarningMessage = "Blocked: %s"
					cfg.BuiltinBadDomains = true
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.NoError(t, err)
		assert.NotNil(t, p.configuration)
		assert.NotNil(t, p.cache)
		assert.NotNil(t, p.badWordsRegex)
		assert.NotNil(t, p.badDomainsRegex)
		assert.NotNil(t, p.badUsernamesRegex)
		assert.NotNil(t, p.badDomainsList)

		// Verify configuration values
		cfg := p.getConfiguration()
		assert.Equal(t, "test,word", cfg.BadWordsList)
		assert.Equal(t, "bad.com", cfg.BadDomainsList)
		assert.Equal(t, "baduser", cfg.BadUsernamesList)
		assert.Equal(t, "\\*", cfg.CensorCharacter)
		assert.True(t, cfg.RejectPosts)
		assert.True(t, cfg.ExcludeBots)
		assert.True(t, cfg.BlockNewUserPM)
		assert.Equal(t, "24h", cfg.BlockNewUserPMTime)
		assert.True(t, cfg.BuiltinBadDomains)

		// Verify cache is initialized
		assert.Equal(t, 50, p.cache.capacity)
	})

	t.Run("handles configuration load error", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				return fmt.Errorf("failed to load config")
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load plugin configuration")
	})

	t.Run("initializes cache only once", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				return nil
			},
		}
		p.SetAPI(mockAPI)

		// First call - cache should be created
		err := p.OnConfigurationChange()
		assert.NoError(t, err)
		firstCache := p.cache
		assert.NotNil(t, firstCache)

		// Add something to cache to verify it persists
		firstCache.Put("test", &model.User{Id: "test"})

		// Second call - cache should persist
		err = p.OnConfigurationChange()
		assert.NoError(t, err)
		assert.Equal(t, firstCache, p.cache) // Same cache instance

		// Verify cached data persists
		user, found := p.cache.Get("test")
		assert.True(t, found)
		assert.Equal(t, "test", user.Id)
	})

	t.Run("compiles regex patterns from configuration", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				if cfg, ok := dest.(*configuration); ok {
					cfg.BadWordsList = "word1,word2,phrase with spaces"
					cfg.BadDomainsList = "spam.com,junk.org"
					cfg.BadUsernamesList = "spammer,bot.*"
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.NoError(t, err)

		// Test bad words regex
		assert.NotNil(t, p.badWordsRegex)
		assert.True(t, p.badWordsRegex.MatchString("this contains word1"))
		assert.True(t, p.badWordsRegex.MatchString("phrase with spaces here"))
		assert.False(t, p.badWordsRegex.MatchString("clean text"))

		// Test bad domains regex
		assert.NotNil(t, p.badDomainsRegex)
		assert.True(t, p.badDomainsRegex.MatchString("user@spam.com"))
		assert.True(t, p.badDomainsRegex.MatchString("test@junk.org"))
		assert.False(t, p.badDomainsRegex.MatchString("good@example.com"))

		// Test bad usernames regex
		assert.NotNil(t, p.badUsernamesRegex)
		assert.True(t, p.badUsernamesRegex.MatchString("spammer"))
		assert.True(t, p.badUsernamesRegex.MatchString("botnet"))
		assert.False(t, p.badUsernamesRegex.MatchString("gooduser"))
	})

	t.Run("handles empty word lists", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				if cfg, ok := dest.(*configuration); ok {
					cfg.BadWordsList = ""
					cfg.BadDomainsList = ""
					cfg.BadUsernamesList = ""
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.NoError(t, err)

		// Empty lists should result in nil regex
		assert.Nil(t, p.badWordsRegex)
		assert.Nil(t, p.badDomainsRegex)
		assert.Nil(t, p.badUsernamesRegex)
	})

	t.Run("loads builtin bad domains", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				if cfg, ok := dest.(*configuration); ok {
					cfg.BuiltinBadDomains = true
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.NoError(t, err)
		assert.NotNil(t, p.badDomainsList)
		assert.Greater(t, len(*p.badDomainsList), 0) // Should have loaded builtin domains
	})

	t.Run("updates configuration atomically", func(t *testing.T) {
		p := Plugin{}

		// Initial configuration
		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				if cfg, ok := dest.(*configuration); ok {
					cfg.BadWordsList = "original"
					cfg.CensorCharacter = "*"
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()
		assert.NoError(t, err)

		originalConfig := p.getConfiguration()
		assert.Equal(t, "original", originalConfig.BadWordsList)

		// Update configuration
		mockAPI.LoadPluginConfigurationFunc = func(dest interface{}) error {
			if cfg, ok := dest.(*configuration); ok {
				cfg.BadWordsList = "updated"
				cfg.CensorCharacter = "#"
			}
			return nil
		}

		err = p.OnConfigurationChange()
		assert.NoError(t, err)

		updatedConfig := p.getConfiguration()
		assert.Equal(t, "updated", updatedConfig.BadWordsList)
		assert.Equal(t, "#", updatedConfig.CensorCharacter)

		// Verify original config wasn't modified
		assert.Equal(t, "original", originalConfig.BadWordsList)
		assert.Equal(t, "*", originalConfig.CensorCharacter)
	})

	t.Run("returns error for invalid duration config", func(t *testing.T) {
		p := Plugin{}

		mockAPI := &MockConfigAPI{
			LoadPluginConfigurationFunc: func(dest interface{}) error {
				if cfg, ok := dest.(*configuration); ok {
					cfg.BlockNewUserPMTime = "invalid-duration"
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.OnConfigurationChange()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid duration format")
	})
}

func TestJsonArrayToStringSlice(t *testing.T) {
	t.Run("parses valid JSON array", func(t *testing.T) {
		jsonArray := `["domain1.com", "domain2.com", "domain3.com"]`
		result, err := jsonArrayToStringSlice(jsonArray)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 3, len(*result))
		assert.Equal(t, "domain1.com", (*result)[0])
		assert.Equal(t, "domain2.com", (*result)[1])
		assert.Equal(t, "domain3.com", (*result)[2])
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		invalidJSON := `["domain1.com", "domain2.com"` // Missing closing bracket
		result, err := jsonArrayToStringSlice(invalidJSON)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unexpected")
	})

	t.Run("returns error for non-array JSON", func(t *testing.T) {
		nonArrayJSON := `{"domains": ["domain1.com"]}` // Object instead of array
		result, err := jsonArrayToStringSlice(nonArrayJSON)

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("parses empty array", func(t *testing.T) {
		emptyArray := `[]`
		result, err := jsonArrayToStringSlice(emptyArray)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, len(*result))
	})

	t.Run("returns error for array with non-string values", func(t *testing.T) {
		// This should fail because JSON contains non-string values
		mixedArray := `["domain1.com", 123, "domain2.com"]`
		result, err := jsonArrayToStringSlice(mixedArray)

		// JSON unmarshal will fail when trying to unmarshal number into string slice
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "cannot unmarshal")
	})
}

func TestSetupBadDomainList(t *testing.T) {
	t.Run("successfully sets up bad domain list from embedded file", func(t *testing.T) {
		p := Plugin{}

		err := p.setupBadDomainList()

		// This should succeed because the embedded file is valid JSON
		assert.NoError(t, err)
		assert.NotNil(t, p.badDomainsList)
		assert.Greater(t, len(*p.badDomainsList), 0)
	})
}

func TestSetConfiguration(t *testing.T) {
	t.Run("sets configuration successfully", func(t *testing.T) {
		p := Plugin{}

		cfg := &configuration{
			BadWordsList:    "test",
			CensorCharacter: "*",
		}

		p.setConfiguration(cfg)

		assert.Equal(t, cfg, p.configuration)
	})

	t.Run("detects self-assignment", func(t *testing.T) {
		p := Plugin{}

		cfg := &configuration{
			BadWordsList:    "test",
			CensorCharacter: "*",
		}

		p.setConfiguration(cfg)

		// This should panic if we try to set the same configuration
		assert.Panics(t, func() {
			p.setConfiguration(cfg)
		})
	})

	t.Run("allows nil configuration", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadWordsList: "test",
			},
		}

		p.setConfiguration(nil)

		assert.Nil(t, p.configuration)
	})

	t.Run("allows empty configuration replacement", func(t *testing.T) {
		p := Plugin{}

		emptyConfig1 := &configuration{}
		emptyConfig2 := &configuration{}

		p.setConfiguration(emptyConfig1)
		assert.Equal(t, emptyConfig1, p.configuration)

		// Should not panic even though configs are empty
		assert.NotPanics(t, func() {
			p.setConfiguration(emptyConfig2)
		})
	})
}

func TestClone(t *testing.T) {
	t.Run("creates independent copy", func(t *testing.T) {
		original := &configuration{
			BadWordsList:       "word1,word2",
			BadDomainsList:     "bad.com",
			BadUsernamesList:   "baduser",
			CensorCharacter:    "*",
			RejectPosts:        true,
			ExcludeBots:        true,
			BlockNewUserPM:     true,
			BlockNewUserPMTime: "24h",
			WarningMessage:     "Blocked",
			BuiltinBadDomains:  true,
		}

		cloned := original.Clone()

		// Verify all fields are copied
		assert.Equal(t, original.BadWordsList, cloned.BadWordsList)
		assert.Equal(t, original.BadDomainsList, cloned.BadDomainsList)
		assert.Equal(t, original.BadUsernamesList, cloned.BadUsernamesList)
		assert.Equal(t, original.CensorCharacter, cloned.CensorCharacter)
		assert.Equal(t, original.RejectPosts, cloned.RejectPosts)
		assert.Equal(t, original.ExcludeBots, cloned.ExcludeBots)
		assert.Equal(t, original.BlockNewUserPM, cloned.BlockNewUserPM)
		assert.Equal(t, original.BlockNewUserPMTime, cloned.BlockNewUserPMTime)
		assert.Equal(t, original.WarningMessage, cloned.WarningMessage)
		assert.Equal(t, original.BuiltinBadDomains, cloned.BuiltinBadDomains)

		// Verify they are different instances
		assert.NotSame(t, original, cloned)

		// Modify clone and verify original is unchanged
		cloned.BadWordsList = "modified"
		assert.Equal(t, "word1,word2", original.BadWordsList)
		assert.Equal(t, "modified", cloned.BadWordsList)
	})
}

func TestSplitWordListToRegex(t *testing.T) {
	t.Run("creates regex from word list", func(t *testing.T) {
		regex, err := splitWordListToRegex("word1,word2,word3")

		assert.NoError(t, err)
		assert.NotNil(t, regex)
		assert.True(t, regex.MatchString("contains word1 here"))
		assert.True(t, regex.MatchString("word2"))
		assert.True(t, regex.MatchString("end with word3"))
		assert.False(t, regex.MatchString("nothing here"))
	})

	t.Run("returns nil for empty list", func(t *testing.T) {
		regex, err := splitWordListToRegex("")

		assert.NoError(t, err)
		assert.Nil(t, regex)
	})

	t.Run("uses custom template", func(t *testing.T) {
		regex, err := splitWordListToRegex("test", `^(%s)$`)

		assert.NoError(t, err)
		assert.NotNil(t, regex)
		assert.True(t, regex.MatchString("test"))
		assert.False(t, regex.MatchString("test123"))
		assert.False(t, regex.MatchString("123test"))
	})

	t.Run("sorts by length descending", func(t *testing.T) {
		regex, err := splitWordListToRegex("a,abc,ab")

		assert.NoError(t, err)
		// The regex should be ordered as: abc|ab|a
		assert.NotNil(t, regex)
		// This ensures longest match first
		match := regex.FindString("abc")
		assert.Equal(t, "abc", match)
	})

	t.Run("returns error for invalid regex pattern", func(t *testing.T) {
		// Use a malformed regex pattern
		regex, err := splitWordListToRegex("test", `(?P<invalid`)

		assert.Error(t, err)
		assert.Nil(t, regex)
		assert.Contains(t, err.Error(), "unable to compile regex")
	})
}
