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

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()
	_, fromBot := post.GetProps()["from_bot"]

	if configuration.ExcludeBots && fromBot {
		return post, ""
	}

	if configuration.BlockNewUserPM && p.isDirectMessage(post.ChannelId) {
		user, err := p.API.GetUser(post.UserId)
		if err != nil {
			p.API.SendEphemeralPost(post.UserId, &model.Post{
				ChannelId: post.ChannelId,
				Message:   "Something went wrong when sending your post. Contact an administrator",
				RootId:    post.RootId,
			})
		}

		createdAt := time.Unix(user.CreateAt, 0)
		blockDuration := configuration.BlockNewUserPMTime
		duration, error := time.ParseDuration(blockDuration)

		if error != nil {
			p.API.SendEphemeralPost(post.UserId, &model.Post{
				ChannelId: post.ChannelId,
				Message:   "Something went wrong when sending your post. Contact an administrator",
				RootId:    post.RootId,
			})
		}

		if time.Since(createdAt) < duration {
			p.API.SendEphemeralPost(post.UserId, &model.Post{
				ChannelId: post.ChannelId,
				Message:   "Configuration settings limit new users from sending private messages.",
				RootId:    post.RootId,
			})
			return nil, fmt.Sprintf("New user not allowed to send DM for %s.", duration)
		}
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

func (p *Plugin) UserHasBeenCreated(ctx *plugin.Context, user *model.User) {
	validatorFunctions := []func(*plugin.Context, *model.User) error{
		p.checkBadUsername,
		p.checkBadEmail,
	}

	validationErrors := p.RequiresModeration(ctx, user, validatorFunctions...)
	if len(validationErrors) == 0 {
		return // User is OK
	}
	
	// Otherwise, let's take care of that user

	// Make a copy of the user for logging later
	original := user

	// Clean the user
	user.Nickname = fmt.Sprintf("sanitized-%s", user.Id)
	user.Username = fmt.Sprintf("sanitized-%s", user.Id)
	// user.FirstName = "Sanitized"
	// user.LastName = "Sanitized"
	p.API.UpdateUser(user)

	if err := p.RemoveUserFromTeams(user); err != nil {
		fmt.Printf("Unable to remove user from teams: %v\n", err)
	}

	// delete them - Perform a soft delete so the account _can_ be restored...
	p.API.DeleteUser(user.Id)

	// TODO: do something with the validation errors i.e. send them somewhere
	for _, err := range validationErrors {
		fmt.Println(err)
	}
	fmt.Printf("user info: %v\n", original)
}

func (p *Plugin) isDirectMessage(channelId string) bool { 
	channel, err := p.API.GetChannel(channelId)
	if err != nil {
		panic("couldn't find channel")
	}

	fmt.Println(channel.Type)
	return channel.Type == model.ChannelTypeDirect
}
