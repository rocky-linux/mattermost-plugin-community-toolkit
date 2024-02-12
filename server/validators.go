package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) checkBadEmail(user *model.User) error {
	if p.badDomainsRegex.MatchString(user.Email) {
		return fmt.Errorf("email domain matches moderations list: %v", user.Email)
	}
	return nil
}

func (p *Plugin) checkBadUsername(user *model.User) error {
	if p.badUsernamesRegex.MatchString(user.Username) || p.badUsernamesRegex.MatchString(user.Nickname) {
		return fmt.Errorf("username matches moderation list: %v", user.Username)
	}
	return nil
}
