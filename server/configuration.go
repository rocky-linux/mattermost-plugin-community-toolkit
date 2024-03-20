package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

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
type configuration struct {
	BadDomainsList     string
	BadUsernamesList   string
	BuiltinBadDomains  bool
	BadWordsList       string
	BlockNewUserPM     bool
	BlockNewUserPMTime string
	CensorCharacter    string
	ExcludeBots        bool
	RejectPosts        bool
	WarningMessage     string `json:"WarningMessage"`
}

//go:embed bad-domains.txt
var builtinDomainList string

// The default regex template
const defaultRegexTemplate = `(?mi)\b(%s)\b`

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
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
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	p.badWordsRegex = splitWordListToRegex(configuration.BadWordsList)
	p.badDomainsRegex = splitWordListToRegex(configuration.BadDomainsList)
	p.badUsernamesRegex = splitWordListToRegex(configuration.BadUsernamesList, `(?mi)(%s)`)

	domainList, err := jsonArrayToStringSlice(builtinDomainList)
	if err != nil {
		return errors.Wrap(err, "failed to pase builtin domains list")
	}
	p.badDomainsList = domainList

	return nil
}

func jsonArrayToStringSlice(jsonArray string) (*[]string, error) {
	var result []string
	err := json.Unmarshal([]byte(jsonArray), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func splitWordListToRegex(wordList string, regexTemplateOptional ...string) *regexp.Regexp {
	// If there's no list of words, don't make them into a regex
	if len(wordList) < 1 {
		return nil
	}

	// Choose the regex template
	regexTemplate := defaultRegexTemplate
	if len(regexTemplateOptional) > 0 {
		regexTemplate = regexTemplateOptional[0]
	}

	// Compile the regex
	regexString := wordListToRegex(wordList, regexTemplate)
	regex, err := regexp.Compile(regexString)
	if err != nil {
		panic(fmt.Errorf("unable to split wordlist to regex: %v", err))
	}
	return regex
}

func wordListToRegex(wordList string, regexTemplate string) string {
	split := strings.Split(wordList, ",")

	// Sorting by length so that longer words come first
	sort.Slice(split, func(i, j int) bool { return len(split[i]) > len(split[j]) })

	return fmt.Sprintf(regexTemplate, strings.Join(split, "|"))
}
