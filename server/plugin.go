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

	// Dev Note: There is an order of operation sensitivity here - please
	// note comments on checks below.

	// Check if user should be blocked (bad username or email domain)
	// This prevents posts during the UserHasBeenCreated cleanup window
	if shouldBlock := p.shouldBlockUserPost(post.UserId); shouldBlock {
		p.sendUserEphemeralMessageForPost(post, "Your account has been flagged for moderation. Please contact an administrator.")
		return nil, "User account flagged for moderation"
	}

	if configuration.BlockNewUserPM && p.isDirectMessage(post.ChannelId) {
		return p.FilterDirectMessage(configuration, post)
	}

	// Check images before links because image URLs match both checks,
	// and images should take priority (more specific content type)
	if configuration.BlockNewUserImages && p.containsImages(post) {
		return p.FilterNewUserImages(configuration, post)
	}

	if configuration.BlockNewUserLinks && p.containsLinks(post) {
		return p.FilterNewUserLinks(configuration, post)
	}

	return p.FilterPostBadWords(configuration, post)
}

func (p *Plugin) GetUserByID(userID string) (*model.User, error) {
	if p.cache != nil {
		if user, found := p.cache.Get(userID); found {
			return user, nil
		}
	}
	user, err := p.API.GetUser(userID)
	if err != nil {
		return &model.User{}, fmt.Errorf("failed to find user with id %v", userID)
	}
	if p.cache != nil {
		cacheUser := *user
		p.cache.Put(user.Id, &cacheUser)
		return &cacheUser, nil
	}
	return user, nil
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

// Plugin Callback: UserWillLogIn
// Executed before a user authenticates. Returns an error message to prevent login.
func (p *Plugin) UserWillLogIn(_ *plugin.Context, user *model.User) string {
	// Check if user is soft-deleted
	if user.DeleteAt > 0 {
		p.API.LogWarn(fmt.Sprintf("Blocking login attempt for soft-deleted user: %s", user.Id))
		return "Your account has been deactivated due to policy violations. Please contact an administrator."
	}

	// Check if user has bad username
	if err := p.checkBadUsername(user); err != nil {
		p.API.LogWarn(fmt.Sprintf("Blocking login for user with bad username: %s (username: %s)", user.Id, user.Username))
		return "Your account violates community guidelines. Please contact an administrator."
	}

	// Check if user has bad email domain
	if err := p.checkBadEmail(user); err != nil {
		p.API.LogWarn(fmt.Sprintf("Blocking login for user with bad email domain: %s (email: %s)", user.Id, user.Email))
		return "Your account violates community guidelines. Please contact an administrator."
	}

	return "" // Allow login
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
	p.API.LogInfo(fmt.Sprintf("User validation failed for user %s (username: %s, email: %s), starting cleanup", original.Id, original.Username, original.Email))
	if err := p.cleanupUser(user); err != nil {
		p.API.LogError(fmt.Sprintf("Error cleaning up user %s (username: %s, email: %s): %v", original.Id, original.Username, original.Email, err))
		// Continue - we still want to log validation errors
	} else {
		p.API.LogInfo(fmt.Sprintf("Successfully cleaned up user %s (username: %s, email: %s)", original.Id, original.Username, original.Email))
	}

	// Log validation errors for moderation review
	for _, err := range validationErrors {
		p.API.LogError(fmt.Sprintf("User validation failed: %v (user: %s, email: %s, userID: %s)", err, original.Username, original.Email, original.Id))
	}
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

// getSystemAdminUser finds a system admin user to use for administrative operations
func (p *Plugin) getSystemAdminUser() (*model.User, error) {
	config := p.getConfiguration()

	// First, try the configured admin username if provided
	// Dev Note: Use if you want a specific admin user to be the user on record
	// for disabling user accounts.
	if config.AdminUsername != "" {
		p.API.LogInfo(fmt.Sprintf("Looking for configured admin user: %s", config.AdminUsername))
		user, appErr := p.API.GetUserByUsername(config.AdminUsername)
		if appErr == nil && user != nil && user.DeleteAt == 0 {
			// Check if user has system admin role
			if user.Roles == "system_admin" || strings.Contains(user.Roles, "system_admin") {
				p.API.LogInfo(fmt.Sprintf("Found configured admin user: %s (ID: %s)", config.AdminUsername, user.Id))
				return user, nil
			}
			// If configured user doesn't have admin role, return error (don't fall back)
			p.API.LogError(fmt.Sprintf("Configured admin user '%s' does not have system_admin role (has: %s)", config.AdminUsername, user.Roles))
			return nil, fmt.Errorf("configured admin user '%s' does not have system_admin role", config.AdminUsername)
		}
		if appErr != nil {
			p.API.LogError(fmt.Sprintf("Configured admin user '%s' not found, will try role-based search: %v", config.AdminUsername, appErr))
		}
		// If configured user not found, fall through to role-based search
		// (This allows graceful fallback if the configured admin is temporarily unavailable)
	}

	// If no configured admin or it wasn't found, search for users by system_admin role
	p.API.LogInfo("Searching for system admin users by role")
	options := &model.UserGetOptions{
		Role: "system_admin",
	}
	users, appErr := p.API.GetUsers(options)
	if appErr != nil {
		p.API.LogError(fmt.Sprintf("Failed to get users by role: %v", appErr))
		return nil, fmt.Errorf("failed to get users by role: %w", appErr)
	}

	// Find the first non-deleted system admin user
	for _, user := range users {
		if user != nil && user.DeleteAt == 0 {
			p.API.LogInfo(fmt.Sprintf("Found system admin user via role-based search: %s (ID: %s)", user.Username, user.Id))
			return user, nil
		}
	}

	// If no admin found, return an error
	p.API.LogError("No system admin user found (tried configured admin and role-based search)")
	return nil, fmt.Errorf("no system admin user found")
}

// UserHasJoinedTeam is called when a user joins a team
// This hook allows us to immediately remove users who failed validation
// Signature: (c *plugin.Context, teamMember *model.TeamMember, actor *model.User)
func (p *Plugin) UserHasJoinedTeam(_ *plugin.Context, teamMember *model.TeamMember, _ *model.User) {
	userID := teamMember.UserId
	teamID := teamMember.TeamId

	p.API.LogInfo(fmt.Sprintf("UserHasJoinedTeam hook: user %s joined team %s", userID, teamID))

	user, err := p.GetUserByID(userID)
	if err != nil {
		p.API.LogError(fmt.Sprintf("Failed to get user %s in UserHasJoinedTeam hook: %v", userID, err))
		return
	}

	// Check if user should be blocked (soft-deleted, bad username, or bad email)
	shouldBlock := false
	reason := ""

	if user.DeleteAt > 0 {
		shouldBlock = true
		reason = "user is soft-deleted"
	} else if err := p.checkBadUsername(user); err != nil {
		shouldBlock = true
		reason = fmt.Sprintf("bad username: %s", user.Username)
	} else if err := p.checkBadEmail(user); err != nil {
		shouldBlock = true
		reason = fmt.Sprintf("bad email domain: %s", user.Email)
	}

	if shouldBlock {
		p.API.LogWarn(fmt.Sprintf("SECURITY: Bad user %s attempting to join team %s (reason: %s)", userID, teamID, reason))
		p.removeUserFromTeam(userID, teamID)
	}
}

// removeUserFromTeam removes a user from a specific team
func (p *Plugin) removeUserFromTeam(userID string, teamID string) {
	admin, adminErr := p.getSystemAdminUser()
	if adminErr != nil {
		p.API.LogError(fmt.Sprintf("SECURITY: Failed to get admin user for removing user %s from team %s: %v", userID, teamID, adminErr))

		// Try to remove without admin ID (may work in some Mattermost configurations)
		if err := p.API.DeleteTeamMember(teamID, userID, ""); err != nil {
			p.API.LogError(fmt.Sprintf("CRITICAL: Failed to remove bad user %s from team %s (admin lookup failed AND direct removal failed): %v", userID, teamID, err))
			// TODO: Add to pending-removal queue or alert admins
		} else {
			p.API.LogInfo(fmt.Sprintf("Successfully removed user %s from team %s (without admin context)", userID, teamID))
		}
		return
	}

	if err := p.API.DeleteTeamMember(teamID, userID, admin.Id); err != nil {
		p.API.LogError(fmt.Sprintf("CRITICAL: Failed to remove user %s from team %s: %v", userID, teamID, err))
	} else {
		p.API.LogInfo(fmt.Sprintf("Successfully removed user %s from team %s", userID, teamID))
	}
}

func (p *Plugin) RemoveUserFromTeams(user *model.User) error {
	p.API.LogInfo(fmt.Sprintf("Attempting to remove user %s (username: %s) from teams", user.Id, user.Username))

	teams, appErr := p.API.GetTeamsForUser(user.Id)
	if appErr != nil {
		// GetTeamsForUser may return an error in different scenarios
		// Check if it's a "not found" type error (user has no teams) vs a real API error
		errorMsg := strings.ToLower(appErr.Error())

		// Mattermost API typically returns errors for "not found" scenarios
		// Check the error message for common "not found" patterns
		// Also check if the error ID indicates a not found scenario
		isNotFound := strings.Contains(errorMsg, "not found") ||
			strings.Contains(errorMsg, "no teams") ||
			strings.Contains(errorMsg, "does not exist") ||
			strings.Contains(errorMsg, "not_found") ||
			(appErr.Id != "" && strings.Contains(strings.ToLower(appErr.Id), "not_found"))

		if isNotFound {
			p.API.LogInfo(fmt.Sprintf("User %s is not in any teams (error: %s, id: %s), nothing to remove", user.Id, errorMsg, appErr.Id))
			return nil // User has no teams, that's fine
		}
		// This is a real API error - log it and return it
		p.API.LogError(fmt.Sprintf("Failed to get teams for user %s: error=%v, id=%s, message=%s", user.Id, appErr, appErr.Id, errorMsg))
		return fmt.Errorf("failed to get teams for user: %w", appErr)
	}

	if len(teams) == 0 {
		p.API.LogInfo(fmt.Sprintf("User %s is not in any teams (empty list returned)", user.Id))
		return nil // User not in any teams
	}

	p.API.LogInfo(fmt.Sprintf("User %s is in %d team(s), removing from all teams", user.Id, len(teams)))

	admin, adminErr := p.getSystemAdminUser()
	if adminErr != nil {
		p.API.LogError(fmt.Sprintf("Failed to get system admin user for removing user %s from teams: %v", user.Id, adminErr))
		return fmt.Errorf("failed to get system admin user: %w", adminErr)
	}

	p.API.LogInfo(fmt.Sprintf("Using admin user %s (ID: %s) to remove user %s from teams", admin.Username, admin.Id, user.Id))

	removedCount := 0
	failedRemovals := []string{}
	for _, team := range teams {
		teamName := team.Name
		if teamName == "" {
			teamName = "<unnamed>"
		}
		p.API.LogInfo(fmt.Sprintf("Removing user %s from team %s (ID: %s)", user.Id, teamName, team.Id))
		if err := p.API.DeleteTeamMember(team.Id, user.Id, admin.Id); err != nil {
			errorDetails := fmt.Sprintf("error=%v", err)
			if err.Id != "" {
				errorDetails += fmt.Sprintf(", id=%s", err.Id)
			}
			p.API.LogError(fmt.Sprintf("Failed to remove user %s from team %s (ID: %s): %s", user.Id, teamName, team.Id, errorDetails))
			failedRemovals = append(failedRemovals, fmt.Sprintf("%s (%s)", teamName, team.Id))
			// Continue removing from other teams even if one fails
		} else {
			removedCount++
			p.API.LogInfo(fmt.Sprintf("Successfully removed user %s from team %s (ID: %s)", user.Id, teamName, team.Id))
		}
	}

	if len(failedRemovals) > 0 {
		p.API.LogError(fmt.Sprintf("Failed to remove user %s from %d team(s): %v", user.Id, len(failedRemovals), failedRemovals))
		return fmt.Errorf("failed to remove user from %d team(s): %v", len(failedRemovals), failedRemovals)
	}

	p.API.LogInfo(fmt.Sprintf("Successfully removed user %s from %d team(s)", user.Id, removedCount))
	return nil
}

func (p *Plugin) cleanupUser(user *model.User) error {
	p.API.LogInfo(fmt.Sprintf("Starting cleanup for user %s (username: %s, email: %s)", user.Id, user.Username, user.Email))

	// Remove user from teams
	// Note: If user has no teams yet (race condition), the UserHasJoinedTeam hook will catch them
	p.API.LogInfo(fmt.Sprintf("Attempting to remove user %s from teams", user.Id))
	if err := p.RemoveUserFromTeams(user); err != nil {
		// Log error but continue - deletion is more important
		// The UserHasJoinedTeam hook will catch any teams they join later
		p.API.LogError(fmt.Sprintf("Failed to remove user %s from teams (will continue with deletion, UserHasJoinedTeam hook will catch future teams): %v", user.Id, err))
	} else {
		p.API.LogInfo(fmt.Sprintf("Successfully removed user %s from all teams (or user had no teams - UserHasJoinedTeam hook will catch future teams)", user.Id))
	}

	// Delete them - Perform a soft delete so the account _can_ be restored.
	p.API.LogInfo(fmt.Sprintf("Soft-deleting user %s", user.Id))
	if err := p.API.DeleteUser(user.Id); err != nil {
		p.API.LogError(fmt.Sprintf("Failed to soft-delete user %s: %v", user.Id, err))
		return fmt.Errorf("unable to deactivate user: %w", err)
	}
	p.API.LogInfo(fmt.Sprintf("Successfully soft-deleted user %s", user.Id))

	// Clear cache after deletion
	if p.cache != nil {
		p.cache.Remove(user.Id)
	}

	// Verify user is actually deleted
	// Note: GetUser may return an error if user is deleted, which is what we want
	// Only verify if DeleteUser succeeded (we already returned error if it failed)
	verifyUser, verifyErr := p.API.GetUser(user.Id)
	if verifyErr == nil && verifyUser != nil && verifyUser.DeleteAt == 0 {
		// User still exists - this shouldn't happen if DeleteUser succeeded
		// But we'll log it as a warning rather than failing the entire operation
		p.API.LogError(fmt.Sprintf("User deletion verification failed: user %s still exists (DeleteAt=%d)", user.Id, verifyUser.DeleteAt))
	} else if verifyUser != nil && verifyUser.DeleteAt > 0 {
		p.API.LogInfo(fmt.Sprintf("User deletion verified: user %s has DeleteAt=%d", user.Id, verifyUser.DeleteAt))
	}

	return nil
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

// isImageExtension checks if a file extension corresponds to an image format
// Extension should be normalized (lowercase, no leading dot) but the function handles common variations
func (p *Plugin) isImageExtension(extension string) bool {
	// Normalize extension by removing dot prefix and converting to lowercase
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))

	// Check if the extension matches known image formats
	return ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" || ext == "bmp" || ext == "webp" ||
		ext == "svg" || ext == "tiff" || ext == "tif" || ext == "ico" || ext == "heic" || ext == "heif" || ext == "avif"
}

// containsImages checks if a post contains images
func (p *Plugin) containsImages(post *model.Post) bool {
	// Check if the post has file attachments that are images
	// First check Metadata.Files (populated after post processing)
	if post.Metadata != nil && len(post.Metadata.Files) > 0 {
		for _, file := range post.Metadata.Files {
			if p.isImageExtension(file.Extension) {
				return true
			}
		}
	}

	// Check FileIds (available when MessageWillBePosted is called)
	// Metadata.Files may not be populated yet at hook time, but FileIds are
	if len(post.FileIds) > 0 {
		for _, fileID := range post.FileIds {
			fileInfo, err := p.API.GetFileInfo(fileID)
			if err != nil {
				// If we can't get file info, continue checking other files
				continue
			}
			if p.isImageExtension(fileInfo.Extension) {
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

	createdAt := time.UnixMilli(user.CreateAt)
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

// isSystemAdmin checks if a user has system admin role
func (p *Plugin) isSystemAdmin(user *model.User) bool {
	return user != nil && (user.Roles == "system_admin" || strings.Contains(user.Roles, "system_admin"))
}

// filterNewUserContent is a generic function to filter content from new users
func (p *Plugin) filterNewUserContent(post *model.Post, contentType string, blockDuration string, userMessage string) (*model.Post, string) {
	user, errMsg := p.getUserAndHandleError(post.UserId, post)
	if errMsg != "" {
		return nil, errMsg
	}

	// Exempt system admins from new user restrictions
	if p.isSystemAdmin(user) {
		return post, ""
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

// shouldBlockUserPost checks if a user should be blocked from posting
// based on bad username or email domain validation or if user is soft-deleted
// Returns true if the user should be blocked
func (p *Plugin) shouldBlockUserPost(userID string) bool {
	// Skip validation if userID is empty or API is not available
	if userID == "" || p.API == nil {
		return false
	}

	user, err := p.GetUserByID(userID)
	if err != nil {
		// If we can't get the user, don't block (let other errors handle it)
		return false
	}

	// Check if user is soft-deleted (deactivated)
	if user.DeleteAt > 0 {
		p.API.LogDebug(fmt.Sprintf("Blocking post from soft-deleted user: %s", user.Id))
		return true
	}

	// Check if user has bad username
	if err := p.checkBadUsername(user); err != nil {
		p.API.LogWarn(fmt.Sprintf("Blocking post from user %s with bad username: %s", user.Id, user.Username))
		return true
	}

	// Check if user has bad email domain
	if err := p.checkBadEmail(user); err != nil {
		p.API.LogWarn(fmt.Sprintf("Blocking post from user %s with bad email domain: %s", user.Id, user.Email))
		return true
	}

	return false
}
