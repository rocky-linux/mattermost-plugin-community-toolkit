package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) checkBadEmail(user *model.User) error {
	email := user.Email
	if p.configuration.BuiltinBadDomains && EndsWith(*p.badDomainsList, email) {
		return fmt.Errorf("email domain is in builtin list of bad domains: %v", email)
	}
	if p.badDomainsRegex != nil {
		// Extract domain from email for regex matching
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			domain := parts[1]
			if p.badDomainsRegex.MatchString(domain) {
				return fmt.Errorf("email domain matches moderations list: %v", email)
			}
		}
	}
	return nil
}

func (p *Plugin) checkBadUsername(user *model.User) error {
	if p.badUsernamesRegex != nil && (p.badUsernamesRegex.MatchString(user.Username) || p.badUsernamesRegex.MatchString(user.Nickname)) {
		return fmt.Errorf("username matches moderation list: %v", user.Username)
	}
	return nil
}
