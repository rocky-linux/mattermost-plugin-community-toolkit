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

	badDomainsList *[]string

	cache *LRUCache
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

	if configuration.BlockNewUserLinks && p.containsLinks(post) {
		return p.FilterNewUserLinks(configuration, post)
	}

	if configuration.BlockNewUserImages && p.containsImages(post) {
		return p.FilterNewUserImages(configuration, post)
	}

	return p.FilterPostBadWords(configuration, post)
}

func (p *Plugin) GetUserByID(userID string) (*model.User, error) {
	if user, found := p.cache.Get(userID); found {
		return user, nil
	}
	user, err := p.API.GetUser(userID)
	if err != nil {
		return &model.User{}, fmt.Errorf("failed to find user with id %v", userID)
	}
	cacheUser := *user
	p.cache.Put(user.Id, &cacheUser)

	return &cacheUser, nil
}

func (p *Plugin) FilterDirectMessage(configuration *configuration, post *model.Post) (*model.Post, string) {
	return p.filterNewUserContent(
		post,
		"direct messages",
		configuration.BlockNewUserPMTime,
		"Configuration settings limit new users from sending private messages.",
	)
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
func (p *Plugin) UserHasBeenCreated(_ *plugin.Context, user *model.User) {
	validatorFunctions := []func(*model.User) error{
		p.checkBadUsername,
		p.checkBadEmail,
	}

	validationErrors := p.RequiresModeration(user, validatorFunctions...)
	if len(validationErrors) == 0 {
		return // User is OK
	}

	// Copy the user so we can record the original attributes
	original := *user

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

func (p *Plugin) RequiresModeration(user *model.User, validators ...func(*model.User) error) []error {
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
		if err := p.API.DeleteTeamMember(team.Id, user.Id, admin.Id); err != nil {
			return fmt.Errorf("failed to remove user from team: (%v, %v)", user, team)
		}
	}
	return nil
}

func (p *Plugin) cleanupUser(user *model.User) bool {
	// Clean the user's attributes
	user.Nickname = fmt.Sprintf("sanitized-%s", user.Id)
	user.Username = fmt.Sprintf("sanitized-%s", user.Id)

	fmt.Println(p.API)
	user, err := p.API.UpdateUser(user)
	if err != nil {
		fmt.Printf("Unable to sanitize user")
	}

	// Remove user from teams
	if err := p.RemoveUserFromTeams(user); err != nil {
		fmt.Printf("Unable to remove user from teams: %v\n", err)
	}

	// delete them - Perform a soft delete so the account _can_ be restored.
	if err = p.API.DeleteUser(user.Id); err != nil {
		fmt.Printf("unable to deactivate user: %v", err)
	}

	return true
}

// FilterNewUserLinks checks if a new user is trying to post links and blocks them if they're too new
func (p *Plugin) FilterNewUserLinks(configuration *configuration, post *model.Post) (*model.Post, string) {
	return p.filterNewUserContent(
		post,
		"links",
		configuration.BlockNewUserLinksTime,
		"Configuration settings limit new users from posting links.",
	)
}

// FilterNewUserImages checks if a new user is trying to post images and blocks them if they're too new
func (p *Plugin) FilterNewUserImages(configuration *configuration, post *model.Post) (*model.Post, string) {
	return p.filterNewUserContent(
		post,
		"images",
		configuration.BlockNewUserImagesTime,
		"Configuration settings limit new users from posting images.",
	)
}

// containsLinks checks if a post contains links
func (p *Plugin) containsLinks(post *model.Post) bool {
	// Check if the post has embeds (which includes OpenGraph metadata for links)
	if post.Metadata != nil && len(post.Metadata.Embeds) > 0 {
		return true
	}

	// Check if the post message contains URLs
	// This is a simple regex to detect URLs in the message
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+|www\.[^\s<>"]+`)
	return urlRegex.MatchString(post.Message)
}

// containsImages checks if a post contains images
func (p *Plugin) containsImages(post *model.Post) bool {
	// Check if the post has file attachments that are images
	if post.Metadata != nil && len(post.Metadata.Files) > 0 {
		for _, file := range post.Metadata.Files {
			// Normalize extension by removing dot prefix and converting to lowercase
			ext := strings.ToLower(strings.TrimPrefix(file.Extension, "."))
			// Check if the file is an image based on its extension
			if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" || ext == "bmp" || ext == "webp" ||
				ext == "svg" || ext == "tiff" || ext == "tif" || ext == "ico" || ext == "heic" || ext == "heif" || ext == "avif" {
				return true
			}
		}
	}

	// Check if the post has image embeds
	if post.Metadata != nil && len(post.Metadata.Images) > 0 {
		return true
	}

	// Check if the post message contains Markdown image syntax
	imageRegex := regexp.MustCompile(`!\[.*?\]\(.*?\)`)
	return imageRegex.MatchString(post.Message)
}

// isUserTooNew checks if a user is too new based on the configured duration
// Returns (isTooNew, errorMessage, error)
func (p *Plugin) isUserTooNew(user *model.User, blockDuration string, contentType string) (bool, string, error) {
	// Check if the filter is enabled indefinitely (duration is -1)
	if blockDuration == "-1" {
		return true, fmt.Sprintf("New user not allowed to post %s indefinitely.", contentType), nil
	}

	userCreateSeconds := user.CreateAt / 1000
	createdAt := time.Unix(userCreateSeconds, 0)
	duration, parseErr := time.ParseDuration(blockDuration)

	if parseErr != nil {
		return false, "", fmt.Errorf("failed to parse duration: %w", parseErr)
	}

	if time.Since(createdAt) < duration {
		return true, fmt.Sprintf("New user not allowed to post %s for %s.", contentType, duration), nil
	}

	return false, "", nil
}

// getUserAndHandleError retrieves a user by ID and handles any errors
func (p *Plugin) getUserAndHandleError(userID string, post *model.Post) (*model.User, string) {
	user, err := p.GetUserByID(userID)
	if err != nil {
		p.sendUserEphemeralMessageForPost(post, "Something went wrong when sending your message. Contact an administrator.")
		return nil, "Failed to get user"
	}
	return user, ""
}

// handleFilterError handles errors from the isUserTooNew function
func (p *Plugin) handleFilterError(err error, post *model.Post) (*model.Post, string) {
	if err != nil {
		p.sendUserEphemeralMessageForPost(post, "Something went wrong when sending your message. Contact an administrator.")
		return nil, err.Error()
	}
	return nil, ""
}

// filterNewUserContent is a generic function to filter content from new users
func (p *Plugin) filterNewUserContent(post *model.Post, contentType string, blockDuration string, userMessage string) (*model.Post, string) {
	user, errMsg := p.getUserAndHandleError(post.UserId, post)
	if errMsg != "" {
		return nil, errMsg
	}

	isTooNew, errorMsg, err := p.isUserTooNew(user, blockDuration, contentType)
	if err != nil {
		return p.handleFilterError(err, post)
	}

	if isTooNew {
		p.sendUserEphemeralMessageForPost(post, userMessage)
		return nil, errorMsg
	}

	return post, ""
}
