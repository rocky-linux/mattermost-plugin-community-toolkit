package main

import (
	"regexp"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

func TestCheckBadEmail(t *testing.T) {
	// Setup bad domains regex for testing (using domain template without word boundaries)
	badDomainsRegex, _ := splitWordListToRegex("spam.com,junk.org", `(?mi)(%s)`)

	// Setup builtin bad domains list
	builtinDomains := []string{"hoo.com", "disposable.email"}

	tests := []struct {
		name              string
		email             string
		badDomainsRegex   *regexp.Regexp
		builtinBadDomains bool
		badDomainsList    *[]string
		expectError       bool
		errorContains     string
	}{
		{
			name:              "valid email with good domain",
			email:             "user@example.com",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: false,
			expectError:       false,
		},
		{
			name:              "email matches custom bad domain regex",
			email:             "user@spam.com",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: false,
			expectError:       true,
			errorContains:     "matches moderations list",
		},
		{
			name:              "email matches builtin bad domain list",
			email:             "user@hoo.com",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: true,
			badDomainsList:    &builtinDomains,
			expectError:       true,
			errorContains:     "builtin list of bad domains",
		},
		{
			name:              "email with subdomain containing bad domain",
			email:             "user@sub.spam.com",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: false,
			expectError:       true, // Regex matches spam.com within sub.spam.com
			errorContains:     "matches moderations list",
		},
		{
			name:              "email matches second domain in list",
			email:             "user@junk.org",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: false,
			expectError:       true,
			errorContains:     "matches moderations list",
		},
		{
			name:              "nil regex allows all emails",
			email:             "user@spam.com",
			badDomainsRegex:   nil,
			builtinBadDomains: false,
			expectError:       false,
		},
		{
			name:              "builtin disabled does not check builtin list",
			email:             "user@hoo.com",
			badDomainsRegex:   nil,
			builtinBadDomains: false,
			badDomainsList:    &builtinDomains,
			expectError:       false,
		},
		{
			name:              "case insensitive domain matching",
			email:             "user@SPAM.COM",
			badDomainsRegex:   badDomainsRegex,
			builtinBadDomains: false,
			expectError:       true,
			errorContains:     "matches moderations list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin{
				configuration: &configuration{
					BuiltinBadDomains: tt.builtinBadDomains,
				},
				badDomainsRegex: tt.badDomainsRegex,
				badDomainsList:  tt.badDomainsList,
			}

			user := &model.User{Email: tt.email}
			err := p.checkBadEmail(user)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckBadUsername(t *testing.T) {
	// Setup bad usernames regex for testing
	badUsernamesRegex, _ := splitWordListToRegex("spammer,bot,admin", `(?mi)(%s)`)

	tests := []struct {
		name              string
		username          string
		nickname          string
		badUsernamesRegex *regexp.Regexp
		expectError       bool
		errorContains     string
	}{
		{
			name:              "valid username and nickname",
			username:          "gooduser",
			nickname:          "Good User",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       false,
		},
		{
			name:              "username matches bad pattern",
			username:          "spammer123",
			nickname:          "Good User",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       true,
			errorContains:     "matches moderation list",
		},
		{
			name:              "nickname matches bad pattern",
			username:          "gooduser",
			nickname:          "I am a bot",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       true,
			errorContains:     "matches moderation list",
		},
		{
			name:              "both username and nickname match",
			username:          "botspammer",
			nickname:          "Admin Bot",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       true,
			errorContains:     "matches moderation list",
		},
		{
			name:              "nil regex allows all usernames",
			username:          "spammer",
			nickname:          "bot admin",
			badUsernamesRegex: nil,
			expectError:       false,
		},
		{
			name:              "empty username and nickname",
			username:          "",
			nickname:          "",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       false,
		},
		{
			name:              "case insensitive matching",
			username:          "SPAMMER",
			nickname:          "Good User",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       true,
			errorContains:     "matches moderation list",
		},
		{
			name:              "partial match in username",
			username:          "notaspammer",
			nickname:          "Good User",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       true,
			errorContains:     "matches moderation list",
		},
		{
			name:              "username with special characters",
			username:          "user.name+test",
			nickname:          "Test User",
			badUsernamesRegex: badUsernamesRegex,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin{
				badUsernamesRegex: tt.badUsernamesRegex,
			}

			user := &model.User{
				Username: tt.username,
				Nickname: tt.nickname,
			}
			err := p.checkBadUsername(user)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCheckBadEmailWithRealData tests email validation with realistic email patterns
func TestCheckBadEmailWithRealData(t *testing.T) {
	// Setup realistic bad domains (using domain template without word boundaries)
	badDomainsRegex, _ := splitWordListToRegex("tempmail.com,10minutemail.com,guerrillamail.com", `(?mi)(%s)`)
	builtinDomains := []string{"throwaway.email", "fakeinbox.com"}

	p := Plugin{
		configuration: &configuration{
			BuiltinBadDomains: true,
		},
		badDomainsRegex: badDomainsRegex,
		badDomainsList:  &builtinDomains,
	}

	tests := []struct {
		name        string
		email       string
		shouldBlock bool
	}{
		// Valid emails
		{"gmail", "user@gmail.com", false},
		{"corporate", "john.doe@company.com", false},
		{"subdomain", "admin@mail.company.com", false},
		{"with plus", "user+tag@example.com", false},
		{"with dash", "user-name@example.com", false},

		// Bad emails - custom list
		{"temp mail", "user@tempmail.com", true},
		{"10 minute mail", "user@10minutemail.com", true},
		{"guerrilla mail", "user@guerrillamail.com", true},

		// Bad emails - builtin list
		{"throwaway", "user@throwaway.email", true},
		{"fake inbox", "user@fakeinbox.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{Email: tt.email}
			err := p.checkBadEmail(user)

			if tt.shouldBlock {
				assert.Error(t, err, "Expected %s to be blocked", tt.email)
			} else {
				assert.NoError(t, err, "Expected %s to be allowed", tt.email)
			}
		})
	}
}

// TestCheckBadUsernameWithRealData tests username validation with realistic patterns
func TestCheckBadUsernameWithRealData(t *testing.T) {
	// Setup realistic bad username patterns
	badUsernamesRegex, _ := splitWordListToRegex("admin,moderator,support,spam,bot", `(?mi)(%s)`)

	p := Plugin{
		badUsernamesRegex: badUsernamesRegex,
	}

	tests := []struct {
		name        string
		username    string
		nickname    string
		shouldBlock bool
	}{
		// Valid usernames
		{"regular user", "john_doe", "John Doe", false},
		{"with numbers", "user123", "User 123", false},
		{"with underscore", "super_user", "Super User", false},
		{"short name", "joe", "Joe", false},

		// Bad usernames - contains reserved words
		{"contains admin", "admin_user", "Admin User", true},
		{"contains moderator", "moderator123", "Mod", true},
		{"contains support", "support_team", "Support", true},
		{"contains spam", "spammer99", "Spammer", true},
		{"contains bot", "chatbot", "Bot", true},

		// Bad nicknames
		{"admin nickname", "gooduser", "Admin Person", true},
		{"bot nickname", "user123", "I'm a bot", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{
				Username: tt.username,
				Nickname: tt.nickname,
			}
			err := p.checkBadUsername(user)

			if tt.shouldBlock {
				assert.Error(t, err, "Expected %s/%s to be blocked", tt.username, tt.nickname)
			} else {
				assert.NoError(t, err, "Expected %s/%s to be allowed", tt.username, tt.nickname)
			}
		})
	}
}

// TestCheckBadEmailWithRegexPatterns tests email validation with regex patterns
func TestCheckBadEmailWithRegexPatterns(t *testing.T) {
	// Test regex patterns like .*spam and spam.*
	badDomainsRegex, err := splitWordListToRegex("10minutemail.com, tempmail.com, .*spam, spam.*", `(?mi)(%s)`)
	assert.NoError(t, err)
	assert.NotNil(t, badDomainsRegex)

	p := Plugin{
		configuration: &configuration{
			BuiltinBadDomains: false,
		},
		badDomainsRegex: badDomainsRegex,
		badDomainsList:  &[]string{},
	}

	tests := []struct {
		name        string
		email       string
		shouldBlock bool
	}{
		// Valid emails
		{"good domain", "user@example.com", false},
		{"another good domain", "user@company.org", false},
		{"domain with spam in middle", "user@gooddomain.com", false},

		// Should match exact domains
		{"tempmail.com", "user@tempmail.com", true},
		{"10minutemail.com", "user@10minutemail.com", true},

		// Should match regex patterns
		{"domain ending with spam", "user@spamdomain.com", true},      // matches spam.*
		{"domain containing spam", "user@testspam.com", true},         // matches .*spam
		{"domain starting with spam", "user@spamtest.com", true},       // matches spam.*
		{"spam in middle", "user@testspamdomain.com", true},            // matches .*spam
		{"spam at end", "user@testspam.com", true},                     // matches both patterns
		{"domain is spam", "user@spam.com", true},                     // matches both patterns

		// Edge cases
		{"spam as subdomain", "user@spam.subdomain.com", true},        // matches spam.*
		{"spam in subdomain", "user@subdomain.spam.com", true},        // matches .*spam
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{Email: tt.email}
			err := p.checkBadEmail(user)

			if tt.shouldBlock {
				assert.Error(t, err, "Expected %s to be blocked", tt.email)
			} else {
				assert.NoError(t, err, "Expected %s to be allowed", tt.email)
			}
		})
	}
}
