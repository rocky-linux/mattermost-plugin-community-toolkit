package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) checkBadEmail(_ *plugin.Context, user *model.User) error {
	if p.badDomainsRegex.MatchString(user.Email) {
		return fmt.Errorf("email domain matches moderations list: %v", user.Email)
	}
	return nil
}

func (p *Plugin) checkBadUsername(_ *plugin.Context, user *model.User) error {
	fmt.Printf("Username is: %s\n", user.Username)
	fmt.Printf("Nickname is: %s\n", user.Nickname)
	fmt.Printf("regex is: %v\n", p.badUsernamesRegex)
	if p.badUsernamesRegex.MatchString(user.Username) || p.badUsernamesRegex.MatchString(user.Nickname) {
		return fmt.Errorf("username matches moderation list: %v", user.Username)
	}
	return nil
}
