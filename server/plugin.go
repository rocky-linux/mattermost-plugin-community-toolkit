package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	badWordsRegex     *regexp.Regexp
	badDomainsRegex   *regexp.Regexp
	badUsernamesRegex *regexp.Regexp
}


// Plugin Callback: MessageWillBePosted
func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post)
}

// Plugin Callback: MessageWillBeUpdatd
func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}


func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()
	_, fromBot := post.GetProps()["from_bot"]

	if configuration.ExcludeBots && fromBot {
		return post, ""
	}

	if configuration.BlockNewUserPM && p.isDirectMessage(post.ChannelId) {
		return p.FilterDirectMessage(configuration, post)
	}

	return p.FilterPostBadWords(configuration, post)
}

func (p *Plugin) FilterDirectMessage(configuration *configuration, post *model.Post) (*model.Post, string) {
		user, err := p.API.GetUser(post.UserId)
		if err != nil {
			p.sendUserEphemeralMessageForPost(post, "Something went wrong when sending your message. Contact an administrator.")
		}

		createdAt := time.Unix(user.CreateAt, 0)
		blockDuration := configuration.BlockNewUserPMTime
		duration, error := time.ParseDuration(blockDuration)

		if error != nil {
			p.sendUserEphemeralMessageForPost(post, "Something went wrong when sending your message. Contact an administrator.")
		}

		if time.Since(createdAt) < duration {
			p.sendUserEphemeralMessageForPost(post, "Configuration settings limit new users from sending private messages.")
			return nil, fmt.Sprintf("New user not allowed to send DM for %s.", duration)
		}
	return post, ""
}

func (p *Plugin) FilterPostBadWords(configuration *configuration, post *model.Post) (*model.Post, string) {
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

// Plugin Callback: UserHasBeenCreated
// Executed after a user has been created, no return expected
func (p *Plugin) UserHasBeenCreated(ctx *plugin.Context, user *model.User) {

	validatorFunctions := []func(*plugin.Context, *model.User) error{
		p.checkBadUsername,
		p.checkBadEmail,
	}

	validationErrors := p.RequiresModeration(ctx, user, validatorFunctions...)
	if len(validationErrors) == 0 {
		return // User is OK
	}

  // Copy the user so we can record the original attributes
  original := user

	// Perform the cleanup operation
	if !p.cleanupUser(user) {
		fmt.Println("Something went wrong when cleaning up user: ", original)
	}
	
	// TODO: do something with the validation errors i.e. send them somewhere
	for _, err := range validationErrors {
		fmt.Println(err)
	}

	fmt.Printf("user info: %v\n", original)
}


func (p *Plugin) RequiresModeration(ctx *plugin.Context, user *model.User, validators ...func(*plugin.Context, *model.User) error) []error {
	var errors []error

	for _, validator := range validators {
		if err := validator(ctx, user); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil // Does not require moderation
}

func (p *Plugin) RemoveUserFromTeams(user *model.User) error {
	teams, err := p.API.GetTeamsForUser(user.Id)
	if err != nil {
		return fmt.Errorf("unable to get any teams for user")
	}

	admin, err := p.API.GetUserByUsername("neil") // TODO should not just use me
	if err != nil {
		return fmt.Errorf("failed to get admin by username to perform removal")
	}

	for _, team := range teams {
		p.API.DeleteTeamMember(team.Id, user.Id, admin.Id)
	}
	return nil
}


func (p *Plugin) cleanupUser(user *model.User) bool {

	// Clean the user's attributes
	user.Nickname = fmt.Sprintf("sanitized-%s", user.Id)
	user.Username = fmt.Sprintf("sanitized-%s", user.Id)

	p.API.UpdateUser(user)

	// Remove user from teams
	if err := p.RemoveUserFromTeams(user); err != nil {
		fmt.Printf("Unable to remove user from teams: %v\n", err)
	}

	// delete them - Perform a soft delete so the account _can_ be restored.
	p.API.DeleteUser(user.Id)

	return true
}
