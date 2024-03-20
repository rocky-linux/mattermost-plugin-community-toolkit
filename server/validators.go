package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) checkBadEmail(user *model.User) error {
	email := user.Email
	if p.configuration.BuiltinBadDomains && EndsWith(*p.badDomainsList, email) {
		return fmt.Errorf("email domain is in builtin list of bad domains: %v", email)
	}
	if p.badDomainsRegex != nil && p.badDomainsRegex.MatchString(email) {
		return fmt.Errorf("email domain matches moderations list: %v", email)
	}
	return nil
}

func (p *Plugin) checkBadUsername(user *model.User) error {
	if p.badUsernamesRegex != nil && (p.badUsernamesRegex.MatchString(user.Username) || p.badUsernamesRegex.MatchString(user.Nickname)) {
		return fmt.Errorf("username matches moderation list: %v", user.Username)
	}
	return nil
}
