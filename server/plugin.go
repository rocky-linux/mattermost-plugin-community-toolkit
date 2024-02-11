package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	badWordsRegex *regexp.Regexp
}

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()
	_, fromBot := post.GetProps()["from_bot"]

	if configuration.ExcludeBots && fromBot {
		return post, ""
	}

	postMessageWithoutAccents := removeAccents(post.Message)

	if !p.badWordsRegex.MatchString(postMessageWithoutAccents) {
		return post, ""
	}

	detectedBadWords := p.badWordsRegex.FindAllString(postMessageWithoutAccents, -1)

	if configuration.RejectPosts {
		p.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   fmt.Sprintf(configuration.WarningMessage, strings.Join(detectedBadWords, ", ")),
			RootId:    post.RootId,
		})

		return nil, fmt.Sprintf("Profane word not allowed: %s", strings.Join(detectedBadWords, ", "))
	}

	for _, word := range detectedBadWords {
		post.Message = strings.ReplaceAll(
			post.Message,
			word,
			strings.Repeat(p.getConfiguration().CensorCharacter, len(word)),
		)
	}

	return post, ""
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}

func readDomainsFromFile() []string {
	// Use email list from https://raw.githubusercontent.com/unkn0w/disposable-email-domain-list/main/domains.txt
	file, err := os.Open("domains.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var badDomains []string

	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			badDomains = append(badDomains, domain)
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	return badDomains
}

func checkBadEmail(user *model.User) error {
	for _, domain := range readDomainsFromFile() {
		if strings.HasSuffix(user.Email, domain) {
			return fmt.Errorf("domain in list of known throwaway domains: (%v, %v)", user.Email, domain)
		}
	}
	return nil
}

func checkBadUsername(user *model.User) error {
	moderationRegexList := []*regexp.Regexp{
		regexp.MustCompile(`gmk`),
	}

	for _, regex := range moderationRegexList {
		if regex.MatchString(user.Username) {
			return fmt.Errorf("username matches moderation list: %v", user.Username)
		}
	}
	return nil
}

func RequiresModeration(user *model.User, validators ...func(*model.User) error) []error {
	var errors []error


	for _, validator := range validators {
		if err := validator(user); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil // Does not require moderation
}

func (p *Plugin) UserHasBeenCreated(_ *plugin.Context, user *model.User) {
	validatorFunctions := []func(*model.User) error{
		checkBadUsername,
		checkBadEmail,
	}

	validationErrors := RequiresModeration(user, validatorFunctions...)
	if len(validationErrors) != 0 {
		// ban them
		for err := range validationErrors {
			fmt.Println(err)
		}
	}
}

func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, e := transform.String(t, s)
	if e != nil {
		return s
	}

	return output
}
