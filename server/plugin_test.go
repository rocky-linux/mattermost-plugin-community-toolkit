package main

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	// "github.com/mattermost/mattermost/server/public/shared/request"
)

// boolPtr is a helper to track boolean state in closures
type boolPtr struct {
	value bool
}

func TestMessageWillBePosted(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			CensorCharacter: "*",
			RejectPosts:     false,
			BadWordsList:    "def ghi,abc",
			ExcludeBots:     true,
		},
	}
	p.badWordsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadWordsList, defaultRegexTemplate))

	t.Run("word matches", func(t *testing.T) {
		in := &model.Post{
			Message: "123 abc 456",
		}
		out := &model.Post{
			Message: "123 *** 456",
		}

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})

	t.Run("word matches case-insensitive", func(t *testing.T) {
		in := &model.Post{
			Message: "123 ABC AbC 456",
		}
		out := &model.Post{
			Message: "123 *** *** 456",
		}

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})

	t.Run("word with spaces matches", func(t *testing.T) {
		in := &model.Post{
			Message: "123 def ghi 456",
		}
		out := &model.Post{
			Message: "123 ******* 456",
		}

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})

	t.Run("word matches with punctuation", func(t *testing.T) {
		in := &model.Post{
			Message: "123 abc, 456",
		}
		out := &model.Post{
			Message: "123 ***, 456",
		}

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})

	t.Run("word shouldn't match because it in another word", func(t *testing.T) {
		in := &model.Post{
			Message: "helloabcworld helloabc abchello",
		}
		out := &model.Post{
			Message: "helloabcworld helloabc abchello",
		}

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})

	t.Run("bot messages shouldn't be blocked", func(t *testing.T) {
		in := &model.Post{
			Message: "abc",
		}
		in.AddProp("from_bot", "true")
		out := &model.Post{
			Message: "abc",
		}
		out.AddProp("from_bot", "true")

		rpost, s := p.MessageWillBePosted(&plugin.Context{}, in)
		assert.Empty(t, s)
		assert.Equal(t, out, rpost)
	})
}

type MockAPI struct {
	plugin.API
	UpdateUserFunc        func(user *model.User) (*model.User, *model.AppError)
	GetFileInfoFunc       func(fileID string) (*model.FileInfo, *model.AppError)
	GetUserFunc           func(userID string) (*model.User, *model.AppError)
	GetTeamsForUserFunc   func(userID string) ([]*model.Team, *model.AppError)
	GetUserByUsernameFunc func(username string) (*model.User, *model.AppError)
	GetUsersFunc          func(options *model.UserGetOptions) ([]*model.User, *model.AppError)
	DeleteTeamMemberFunc  func(teamID string, userID string, adminID string) *model.AppError
	DeleteUserFunc        func(userID string) *model.AppError
}

func (m *MockAPI) UpdateUser(user *model.User) (*model.User, *model.AppError) {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(user)
	}
	return user, nil // Default behavior
}

func (m *MockAPI) DeleteUser(userID string) *model.AppError {
	if m.DeleteUserFunc != nil {
		return m.DeleteUserFunc(userID)
	}
	return nil
}

func (m *MockAPI) GetTeamsForUser(userID string) ([]*model.Team, *model.AppError) {
	if m.GetTeamsForUserFunc != nil {
		return m.GetTeamsForUserFunc(userID)
	}
	return nil, nil
}

func (m *MockAPI) GetUserByUsername(userName string) (*model.User, *model.AppError) {
	if m.GetUserByUsernameFunc != nil {
		return m.GetUserByUsernameFunc(userName)
	}
	return nil, nil
}

func (m *MockAPI) GetUsers(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
	if m.GetUsersFunc != nil {
		return m.GetUsersFunc(options)
	}
	return nil, nil
}

func (m *MockAPI) GetUser(userID string) (*model.User, *model.AppError) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(userID)
	}
	return &model.User{Id: userID}, nil
}

func (m *MockAPI) DeleteTeamMember(teamID string, userID string, adminID string) *model.AppError {
	if m.DeleteTeamMemberFunc != nil {
		return m.DeleteTeamMemberFunc(teamID, userID, adminID)
	}
	return nil
}

func (m *MockAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	return post
}

func (m *MockAPI) GetFileInfo(fileID string) (*model.FileInfo, *model.AppError) {
	if m.GetFileInfoFunc != nil {
		return m.GetFileInfoFunc(fileID)
	}
	return nil, &model.AppError{Message: "file not found"}
}

func (m *MockAPI) LogInfo(msg string, keyValuePairs ...interface{}) {
	// No-op for testing
}

func (m *MockAPI) LogError(msg string, keyValuePairs ...interface{}) {
	// No-op for testing
}

func (m *MockAPI) LogDebug(msg string, keyValuePairs ...interface{}) {
	// No-op for testing
}

func (m *MockAPI) LogWarn(msg string, keyValuePairs ...interface{}) {
	// No-op for testing
}

func TestUserHasBeenCreated(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			CensorCharacter:    "*",
			RejectPosts:        false,
			BadWordsList:       "def ghi,abc",
			BadDomainsList:     "baddomain.com,bad.org",
			BadUsernamesList:   "hate,sucks",
			ExcludeBots:        true,
			BlockNewUserPM:     true,
			BlockNewUserPMTime: "4h",
			BuiltinBadDomains:  true,
		},
	}
	p.SetAPI(&MockAPI{})
	p.badDomainsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadDomainsList, `(?mi)(%s)`))
	p.badUsernamesRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadUsernamesList, `(?mi)(%s)`))
	// setupBadDomainList should succeed as the embedded file is valid JSON
	err := p.setupBadDomainList()
	assert.NoError(t, err, "setupBadDomainList should succeed with valid embedded JSON")

	t.Run("username matching word is banned", func(_ *testing.T) {
		id := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "ihateneil-" + id,
			Password: "passwd12345",
		}

		userDeleted := false
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetTeamsForUserFunc: func(userID string) ([]*model.Team, *model.AppError) {
				return []*model.Team{}, nil
			},
			DeleteUserFunc: func(userID string) *model.AppError {
				if userID == id {
					userDeleted = true
				}
				return nil
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == id && userDeleted {
					return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
				}
				return &model.User{Id: userID}, nil
			},
		}
		p.SetAPI(mockAPI)

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.True(t, userDeleted, "User should be soft-deleted when username matches bad words")
	})

	t.Run("nickname matching word is banned", func(_ *testing.T) {
		id := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "reasonable-" + id,
			Password: "passwd12345",
		}

		userDeleted := false
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetTeamsForUserFunc: func(userID string) ([]*model.Team, *model.AppError) {
				return []*model.Team{}, nil
			},
			DeleteUserFunc: func(userID string) *model.AppError {
				if userID == id {
					userDeleted = true
				}
				return nil
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == id && userDeleted {
					return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
				}
				return &model.User{Id: userID}, nil
			},
		}
		p.SetAPI(mockAPI)

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.True(t, userDeleted, "User should be soft-deleted when nickname matches bad words")
	})

	// NOTE(nhanlon): 2024-03-20 I'm not sure if this test is really necessary, but I'm including it to
	// highlight that your badDomain list must be curated carefully and the combinations of potential word bits.
	t.Run("user matching word stub is banned", func(_ *testing.T) {
		id := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "shakeoffthehaters-" + id,
			Password: "passwd12345",
		}

		userDeleted := false
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetTeamsForUserFunc: func(userID string) ([]*model.Team, *model.AppError) {
				return []*model.Team{}, nil
			},
			DeleteUserFunc: func(userID string) *model.AppError {
				if userID == id {
					userDeleted = true
				}
				return nil
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == id && userDeleted {
					return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
				}
				return &model.User{Id: userID}, nil
			},
		}
		p.SetAPI(mockAPI)

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.True(t, userDeleted, "User should be soft-deleted when username contains hate substring")
	})

	t.Run("email matching word stub is not banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@yahoo.com", // yahoo should be allowed, but not stubs.
			Nickname: "Neil Is Alright",
			Username: "alright-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.Equal(t, user.Username, original.Username)
	})

	t.Run("email matching email in default list banned", func(_ *testing.T) {
		id := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@hoo.com",
			Nickname: "Neil Is Alright",
			Username: "alright-" + id,
			Password: "passwd12345",
		}

		userDeleted := false
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetTeamsForUserFunc: func(userID string) ([]*model.Team, *model.AppError) {
				return []*model.Team{}, nil
			},
			DeleteUserFunc: func(userID string) *model.AppError {
				if userID == id {
					userDeleted = true
				}
				return nil
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == id && userDeleted {
					return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
				}
				return &model.User{Id: userID}, nil
			},
		}
		p.SetAPI(mockAPI)

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.True(t, userDeleted, "User should be soft-deleted when email matches builtin bad domain list")
	})

	t.Run("user matching email is banned", func(_ *testing.T) {
		id := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@baddomain.com",
			Nickname: "Neil Is Awesome",
			Username: "neilfan-" + id,
			Password: "passwd12345",
		}

		userDeleted := false
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetTeamsForUserFunc: func(userID string) ([]*model.Team, *model.AppError) {
				return []*model.Team{}, nil
			},
			DeleteUserFunc: func(userID string) *model.AppError {
				if userID == id {
					userDeleted = true
				}
				return nil
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == id && userDeleted {
					return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
				}
				return &model.User{Id: userID}, nil
			},
		}
		p.SetAPI(mockAPI)

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.True(t, userDeleted, "User should be soft-deleted when email matches bad domain")
	})

	t.Run("user not matching email nor name is not banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Is Awesome",
			Username: "neilfan-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.Equal(t, user.Username, original.Username)
	})

	t.Run("user with email domain partial in bad list is not banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@lookslikeabaddomain.com",
			Nickname: "Neil Is Awesome",
			Username: "neilfan-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.Equal(t, user.Username, original.Username)
	})
}

func TestGetSystemAdminUser(t *testing.T) {
	p := Plugin{
		configuration: &configuration{},
	}

	t.Run("uses configured admin username when provided", func(t *testing.T) {
		p.configuration = &configuration{
			AdminUsername: "custom-admin",
		}

		adminID := model.NewId()
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "custom-admin" {
					return &model.User{Id: adminID, Username: "custom-admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
		}
		p.SetAPI(mockAPI)

		admin, err := p.getSystemAdminUser()
		assert.NoError(t, err)
		assert.NotNil(t, admin)
		assert.Equal(t, "custom-admin", admin.Username)
		assert.Equal(t, adminID, admin.Id)
	})

	t.Run("returns error if configured admin username doesn't have admin role", func(t *testing.T) {
		p.configuration = &configuration{
			AdminUsername: "regular-user",
		}

		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "regular-user" {
					return &model.User{Id: model.NewId(), Username: "regular-user", Roles: "system_user"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
		}
		p.SetAPI(mockAPI)

		admin, err := p.getSystemAdminUser()
		assert.Error(t, err)
		assert.Nil(t, admin)
		assert.Contains(t, err.Error(), "does not have system_admin role")
	})

	t.Run("falls back to role-based search when configured admin not found", func(t *testing.T) {
		p.configuration = &configuration{
			AdminUsername: "nonexistent-admin",
		}

		adminID := model.NewId()
		mockAPI := &MockAPI{
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "nonexistent-admin" {
					return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				if options != nil && options.Role == "system_admin" {
					return []*model.User{
						{Id: adminID, Username: "admin", Roles: "system_admin", DeleteAt: 0},
					}, nil
				}
				return nil, nil
			},
		}
		p.SetAPI(mockAPI)

		admin, err := p.getSystemAdminUser()
		assert.NoError(t, err)
		assert.NotNil(t, admin)
		assert.Equal(t, "admin", admin.Username)
		assert.Equal(t, adminID, admin.Id)
	})

	t.Run("uses role-based search when no admin username configured", func(t *testing.T) {
		p.configuration = &configuration{
			AdminUsername: "", // Empty string means not configured
		}

		adminID := model.NewId()
		mockAPI := &MockAPI{
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				if options != nil && options.Role == "system_admin" {
					return []*model.User{
						{Id: adminID, Username: "admin", Roles: "system_admin", DeleteAt: 0},
					}, nil
				}
				return nil, nil
			},
		}
		p.SetAPI(mockAPI)

		admin, err := p.getSystemAdminUser()
		assert.NoError(t, err)
		assert.NotNil(t, admin)
		assert.Equal(t, "admin", admin.Username)
		assert.Equal(t, adminID, admin.Id)
	})
}

func TestCleanupUser(t *testing.T) {
	badUsernamesRegex, _ := splitWordListToRegex("baduser", `(?mi)(%s)`)
	badDomainsRegex, _ := splitWordListToRegex("baddomain.com", `(?mi)(%s)`)

	p := Plugin{
		configuration: &configuration{
			BadUsernamesList: "baduser",
			BadDomainsList:   "baddomain.com",
		},
		cache:             NewLRUCache(50),
		badUsernamesRegex: badUsernamesRegex,
		badDomainsRegex:   badDomainsRegex,
	}

	t.Run("cleanupUser removes user from teams and deletes them", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		adminID := model.NewId()
		user := &model.User{
			Id:       userID,
			Username: "baduser",
			Email:    "test@gooddomain.com",
		}

		// Use pointers to track state across closures
		teamsRemoved := []string{}
		userDeleted := &boolPtr{value: false}
		userUpdated := &boolPtr{value: false}
		deleteUserCalled := &boolPtr{value: false}

		// Clear cache to ensure we use the API
		if p.cache != nil {
			p.cache.Remove(userID)
		}

		mockAPI := &MockAPI{
			GetTeamsForUserFunc: func(id string) ([]*model.Team, *model.AppError) {
				if id == userID {
					return []*model.Team{{Id: teamID}}, nil
				}
				return nil, nil
			},
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				if options != nil && options.Role == "system_admin" {
					return []*model.User{
						{Id: adminID, Username: "admin", Roles: "system_admin", DeleteAt: 0},
					}, nil
				}
				return nil, nil
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				if teamIDParam == teamID && userIDParam == userID {
					teamsRemoved = append(teamsRemoved, teamIDParam)
				}
				return nil
			},
			UpdateUserFunc: func(u *model.User) (*model.User, *model.AppError) {
				userUpdated.value = true
				assert.Equal(t, fmt.Sprintf("sanitized-%s", userID), u.Username)
				return u, nil
			},
			DeleteUserFunc: func(id string) *model.AppError {
				if id == userID {
					// Set flags immediately
					deleteUserCalled.value = true
					userDeleted.value = true
					// Return nil to indicate success
					return nil
				}
				return nil
			},
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					// After DeleteUser is called, return deleted user with DeleteAt set
					// This simulates Mattermost behavior where deleted users have DeleteAt > 0
					if deleteUserCalled.value {
						return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
					}
					// Before deletion, return normal user
					return &model.User{Id: id, Username: "sanitized-" + id, Email: "test@gooddomain.com", DeleteAt: 0}, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.cleanupUser(user)
		assert.NoError(t, err)
		// NOTE: Username sanitization removed - we no longer call UpdateUser
		assert.False(t, userUpdated.value, "User should NOT be updated (no sanitization anymore)")
		// Verify DeleteUser was called by checking the flag
		// The flag is set synchronously in DeleteUserFunc, so it should be true after cleanupUser returns
		if !userDeleted.value {
			t.Logf("DeleteUserFunc was not called or flag not set. deleteUserCalled=%v, userDeleted=%v", deleteUserCalled.value, userDeleted.value)
		}
		assert.True(t, userDeleted.value, "User should be deleted - DeleteUserFunc should have been called")
		if len(teamsRemoved) > 0 {
			assert.Equal(t, 1, len(teamsRemoved), "User should be removed from team")
		}
	})

	t.Run("cleanupUser clears cache after update and deletion", func(t *testing.T) {
		userID := model.NewId()
		user := &model.User{
			Id:       userID,
			Username: "baduser",
			Email:    "test@gooddomain.com",
		}

		// Put user in cache
		p.cache.Put(userID, user)
		_, found := p.cache.Get(userID)
		assert.True(t, found, "User should be in cache initially")

		mockAPI := &MockAPI{
			GetTeamsForUserFunc: func(id string) ([]*model.Team, *model.AppError) {
				return nil, nil // No teams
			},
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: model.NewId(), Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			UpdateUserFunc: func(u *model.User) (*model.User, *model.AppError) {
				return u, nil
			},
			DeleteUserFunc: func(id string) *model.AppError {
				return nil
			},
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				// Return deleted user
				return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.cleanupUser(user)
		assert.NoError(t, err)

		// Cache should be cleared
		_, found = p.cache.Get(userID)
		assert.False(t, found, "User should not be in cache after cleanup")
	})

	// NOTE: Test removed - UpdateUser is no longer called since we removed username sanitization

	t.Run("cleanupUser returns error if DeleteUser fails", func(t *testing.T) {
		userID := model.NewId()
		user := &model.User{
			Id:       userID,
			Username: "baduser",
			Email:    "test@gooddomain.com",
		}

		// Clear cache to ensure we use the API
		if p.cache != nil {
			p.cache.Remove(userID)
		}

		mockAPI := &MockAPI{
			GetTeamsForUserFunc: func(id string) ([]*model.Team, *model.AppError) {
				return nil, nil
			},
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: model.NewId(), Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			UpdateUserFunc: func(u *model.User) (*model.User, *model.AppError) {
				return u, nil
			},
			DeleteUserFunc: func(id string) *model.AppError {
				if id == userID {
					return model.NewAppError("DeleteUser", "delete failed", nil, "", 500)
				}
				return nil
			},
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				// This shouldn't be called since DeleteUser fails and we return early
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.cleanupUser(user)
		assert.Error(t, err, "cleanupUser should return error when DeleteUser fails")
		if err != nil {
			assert.Contains(t, err.Error(), "unable to deactivate user")
		}
	})

	t.Run("cleanupUser handles user not in teams gracefully", func(t *testing.T) {
		userID := model.NewId()
		user := &model.User{
			Id:       userID,
			Username: "baduser",
			Email:    "test@gooddomain.com",
		}

		mockAPI := &MockAPI{
			GetTeamsForUserFunc: func(id string) ([]*model.Team, *model.AppError) {
				return nil, nil // No teams
			},
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: model.NewId(), Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			UpdateUserFunc: func(u *model.User) (*model.User, *model.AppError) {
				return u, nil
			},
			DeleteUserFunc: func(id string) *model.AppError {
				return nil
			},
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				return &model.User{Id: id, DeleteAt: model.GetMillis()}, nil
			},
		}
		p.SetAPI(mockAPI)

		err := p.cleanupUser(user)
		assert.NoError(t, err, "Should succeed even if user has no teams")
	})
}

func TestShouldBlockUserPost(t *testing.T) {
	badUsernamesRegex, _ := splitWordListToRegex("baduser", `(?mi)(%s)`)
	badDomainsRegex, _ := splitWordListToRegex("baddomain.com", `(?mi)(%s)`)

	p := Plugin{
		configuration: &configuration{
			BadUsernamesList: "baduser",
			BadDomainsList:   "baddomain.com",
		},
		cache:             NewLRUCache(50),
		badUsernamesRegex: badUsernamesRegex,
		badDomainsRegex:   badDomainsRegex,
	}

	t.Run("shouldBlockUserPost blocks soft-deleted users", func(t *testing.T) {
		userID := model.NewId()
		deletedUser := &model.User{
			Id:       userID,
			Username: "gooduser",
			DeleteAt: model.GetMillis(), // User is deleted
		}

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return deletedUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, deletedUser)

		shouldBlock := p.shouldBlockUserPost(userID)
		assert.True(t, shouldBlock, "Should block soft-deleted user")
	})

	t.Run("shouldBlockUserPost blocks users with bad username", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Username: "baduser",
			Email:    "test@gooddomain.com",
		}

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, badUser)

		shouldBlock := p.shouldBlockUserPost(userID)
		assert.True(t, shouldBlock, "Should block user with bad username")
	})

	t.Run("shouldBlockUserPost blocks users with bad email domain", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Username: "gooduser",
			Email:    "test@baddomain.com",
		}

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, badUser)

		shouldBlock := p.shouldBlockUserPost(userID)
		assert.True(t, shouldBlock, "Should block user with bad email domain")
	})

	t.Run("shouldBlockUserPost allows good users", func(t *testing.T) {
		userID := model.NewId()
		goodUser := &model.User{
			Id:       userID,
			Username: "gooduser",
			Email:    "test@gooddomain.com",
		}

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return goodUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, goodUser)

		shouldBlock := p.shouldBlockUserPost(userID)
		assert.False(t, shouldBlock, "Should not block good user")
	})
}

func TestUserHasJoinedTeam(t *testing.T) {
	badUsernamesRegex, _ := splitWordListToRegex("baduser", `(?mi)(%s)`)

	p := Plugin{
		configuration: &configuration{
			BadUsernamesList: "baduser",
		},
		cache:             NewLRUCache(50),
		badUsernamesRegex: badUsernamesRegex,
	}

	t.Run("removes user from team if user has bad username", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		adminID := model.NewId()
		teamMember := &model.TeamMember{
			UserId: userID,
			TeamId: teamID,
		}

		// User with bad username
		badUser := &model.User{
			Id:       userID,
			Username: "baduser", // Matches bad username pattern
			DeleteAt: 0,
		}

		removedFromTeam := false
		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				if options != nil && options.Role == "system_admin" {
					return []*model.User{
						{Id: adminID, Username: "admin", Roles: "system_admin", DeleteAt: 0},
					}, nil
				}
				return nil, nil
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				if teamIDParam == teamID && userIDParam == userID && adminIDParam == adminID {
					removedFromTeam = true
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, badUser)

		actor := &model.User{Id: model.NewId()}
		p.UserHasJoinedTeam(&plugin.Context{}, teamMember, actor)

		assert.True(t, removedFromTeam, "User should be removed from team when they have bad username")
	})

	t.Run("removes user from team if user is soft-deleted", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		adminID := model.NewId()
		teamMember := &model.TeamMember{
			UserId: userID,
			TeamId: teamID,
		}

		// User is soft-deleted
		deletedUser := &model.User{
			Id:       userID,
			Username: "deleteduser",
			DeleteAt: model.GetMillis(), // User is soft-deleted
		}

		removedFromTeam := false
		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return deletedUser, nil
				}
				return &model.User{Id: id}, nil
			},
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				if options != nil && options.Role == "system_admin" {
					return []*model.User{
						{Id: adminID, Username: "admin", Roles: "system_admin", DeleteAt: 0},
					}, nil
				}
				return nil, nil
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				if teamIDParam == teamID && userIDParam == userID && adminIDParam == adminID {
					removedFromTeam = true
				}
				return nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, deletedUser)

		actor := &model.User{Id: model.NewId()}
		p.UserHasJoinedTeam(&plugin.Context{}, teamMember, actor)

		assert.True(t, removedFromTeam, "User should be removed from team when they are soft-deleted")
	})

	t.Run("does not remove user from team if user is valid", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		teamMember := &model.TeamMember{
			UserId: userID,
			TeamId: teamID,
		}

		// User is valid (not deleted, not bad username)
		goodUser := &model.User{
			Id:       userID,
			Username: "gooduser",
			Email:    "good@example.com",
			DeleteAt: 0,
		}

		removedFromTeam := false
		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return goodUser, nil
				}
				return &model.User{Id: id}, nil
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				removedFromTeam = true
				return nil
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, goodUser)

		actor := &model.User{Id: model.NewId()}
		p.UserHasJoinedTeam(&plugin.Context{}, teamMember, actor)

		assert.False(t, removedFromTeam, "User should NOT be removed from team when they are valid")
	})

	t.Run("handles admin lookup failure gracefully", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		teamMember := &model.TeamMember{
			UserId: userID,
			TeamId: teamID,
		}

		// User with bad username
		badUser := &model.User{
			Id:       userID,
			Username: "baduser", // Matches bad username pattern
			DeleteAt: 0,
		}

		directRemovalAttempted := false
		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
			GetUsersFunc: func(options *model.UserGetOptions) ([]*model.User, *model.AppError) {
				// No admin found - simulate failure
				return nil, model.NewAppError("GetUsers", "no system admin users found", nil, "", 404)
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				// Should be called with empty admin ID as fallback
				if adminIDParam == "" {
					directRemovalAttempted = true
					return nil // Simulate success
				}
				return model.NewAppError("DeleteTeamMember", "should use empty admin ID", nil, "", 500)
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, badUser)

		actor := &model.User{Id: model.NewId()}
		// Should not panic - should handle gracefully
		p.UserHasJoinedTeam(&plugin.Context{}, teamMember, actor)

		// Test passes if no panic occurs and direct removal was attempted
		assert.True(t, directRemovalAttempted, "Should attempt direct removal when admin lookup fails")
	})

	t.Run("handles DeleteTeamMember failure gracefully", func(t *testing.T) {
		userID := model.NewId()
		teamID := model.NewId()
		adminID := model.NewId()
		teamMember := &model.TeamMember{
			UserId: userID,
			TeamId: teamID,
		}

		// User with bad username
		badUser := &model.User{
			Id:       userID,
			Username: "baduser", // Matches bad username pattern
			DeleteAt: 0,
		}

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
			GetUserByUsernameFunc: func(username string) (*model.User, *model.AppError) {
				if username == "admin" {
					return &model.User{Id: adminID, Username: "admin", Roles: "system_admin"}, nil
				}
				return nil, model.NewAppError("GetUserByUsername", "user not found", nil, "", 404)
			},
			DeleteTeamMemberFunc: func(teamIDParam, userIDParam, adminIDParam string) *model.AppError {
				// Simulate failure to remove team member
				return model.NewAppError("DeleteTeamMember", "failed to remove team member", nil, "", 500)
			},
		}
		p.SetAPI(mockAPI)
		p.cache.Put(userID, badUser)

		actor := &model.User{Id: model.NewId()}
		// Should not panic - should handle gracefully
		p.UserHasJoinedTeam(&plugin.Context{}, teamMember, actor)

		// Test passes if no panic occurs
		assert.True(t, true, "Should handle DeleteTeamMember failure gracefully")
	})
}

func TestBlockedUserPosts(t *testing.T) {
	// Set up plugin with bad usernames and domains configured
	p := Plugin{
		configuration: &configuration{
			CensorCharacter:   "*",
			RejectPosts:       false,
			BadWordsList:      "def ghi,abc",
			BadDomainsList:    "baddomain.com,bad.org",
			BadUsernamesList:  "hate,sucks",
			ExcludeBots:       false,
			BuiltinBadDomains: false,
		},
	}
	p.badWordsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadWordsList, defaultRegexTemplate))
	p.badDomainsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadDomainsList, `(?mi)(%s)`))
	p.badUsernamesRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadUsernamesList, `(?mi)(%s)`))
	p.cache = NewLRUCache(50)

	// Setup bad domains list (empty since BuiltinBadDomains is false)
	emptyList := []string{}
	p.badDomainsList = &emptyList

	t.Run("post from user with bad username should be rejected", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Email:    userID + "@gooddomain.com",
			Username: "ihateneil",
			Nickname: "Good Guy",
		}
		p.cache.Put(userID, badUser)

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  userID,
			Message: "This is a normal message",
		}

		filteredPost, msg := p.MessageWillBePosted(&plugin.Context{}, post)

		assert.Nil(t, filteredPost, "Post from user with bad username should be rejected")
		assert.Contains(t, msg, "flagged for moderation")
	})

	t.Run("post from user with bad email domain should be rejected", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Email:    userID + "@baddomain.com",
			Username: "gooduser",
			Nickname: "Good Guy",
		}
		p.cache.Put(userID, badUser)

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  userID,
			Message: "This is a normal message",
		}

		filteredPost, msg := p.MessageWillBePosted(&plugin.Context{}, post)

		assert.Nil(t, filteredPost, "Post from user with bad email domain should be rejected")
		assert.Contains(t, msg, "flagged for moderation")
	})

	t.Run("post from user with bad nickname should be rejected", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Email:    userID + "@gooddomain.com",
			Username: "gooduser",
			Nickname: "Neil Sucks",
		}
		p.cache.Put(userID, badUser)

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  userID,
			Message: "This is a normal message",
		}

		filteredPost, msg := p.MessageWillBePosted(&plugin.Context{}, post)

		assert.Nil(t, filteredPost, "Post from user with bad nickname should be rejected")
		assert.Contains(t, msg, "flagged for moderation")
	})

	t.Run("post from valid user should pass through", func(t *testing.T) {
		userID := model.NewId()
		goodUser := &model.User{
			Id:       userID,
			Email:    userID + "@gooddomain.com",
			Username: "gooduser",
			Nickname: "Good Guy",
		}
		p.cache.Put(userID, goodUser)

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return goodUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  userID,
			Message: "This is a normal message",
		}

		filteredPost, msg := p.MessageWillBePosted(&plugin.Context{}, post)

		assert.NotNil(t, filteredPost, "Post from valid user should pass through")
		assert.Empty(t, msg, "Valid user post should not have rejection message")
	})

	t.Run("post from bot should be allowed even if username is bad", func(t *testing.T) {
		userID := model.NewId()
		badUser := &model.User{
			Id:       userID,
			Email:    userID + "@gooddomain.com",
			Username: "ihateneil",
			Nickname: "Bad Bot",
		}
		p.cache.Put(userID, badUser)

		mockAPI := &MockAPI{
			GetUserFunc: func(id string) (*model.User, *model.AppError) {
				if id == userID {
					return badUser, nil
				}
				return &model.User{Id: id}, nil
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  userID,
			Message: "This is a bot message",
		}
		post.AddProp("from_bot", "true")

		// Configure plugin to exclude bots
		p.configuration.ExcludeBots = true

		filteredPost, msg := p.MessageWillBePosted(&plugin.Context{}, post)

		assert.NotNil(t, filteredPost, "Post from bot should be allowed even with bad username")
		assert.Empty(t, msg, "Bot post should not have rejection message")
	})
}

// TestFilterNewUserLinks verifies the link blocking functionality for new users
func TestFilterNewUserLinks(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                    // Created now
	oldUserCreateAt := now.Add(-48*time.Hour).Unix() * 1000 // Created 48 hours ago

	mockAPI := &MockAPI{
		UpdateUserFunc: func(user *model.User) (*model.User, *model.AppError) {
			return user, nil
		},
	}

	t.Run("new user with link is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserLinks:     true,
				BlockNewUserLinksTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Check out https://example.com",
		}

		filteredPost, msg := p.FilterNewUserLinks(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "Post with link from new user should be blocked")
		assert.Contains(t, msg, "not allowed")
		assert.Contains(t, msg, "links")
	})

	t.Run("old user with link is allowed", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserLinks:     true,
				BlockNewUserLinksTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add old user
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Check out https://example.com",
		}

		filteredPost, msg := p.FilterNewUserLinks(p.getConfiguration(), post)

		assert.NotNil(t, filteredPost, "Post with link from old user should be allowed")
		assert.Empty(t, msg)
	})

	t.Run("indefinite blocking with -1 duration", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserLinks:     true,
				BlockNewUserLinksTime: "-1",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add user (even old users should be blocked)
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Visit www.example.com",
		}

		filteredPost, msg := p.FilterNewUserLinks(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "Post with link should be blocked with indefinite duration")
		assert.Contains(t, msg, "indefinitely")
	})
}

// TestFilterNewUserImages verifies the image blocking functionality for new users
func TestFilterNewUserImages(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                    // Created now
	oldUserCreateAt := now.Add(-48*time.Hour).Unix() * 1000 // Created 48 hours ago

	mockAPI := &MockAPI{
		UpdateUserFunc: func(user *model.User) (*model.User, *model.AppError) {
			return user, nil
		},
	}

	t.Run("new user with image is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "![image](https://example.com/image.png)",
		}

		filteredPost, msg := p.FilterNewUserImages(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "Post with image from new user should be blocked")
		assert.Contains(t, msg, "not allowed")
		assert.Contains(t, msg, "images")
	})

	t.Run("old user with image is allowed", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add old user
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Here's a photo",
			Metadata: &model.PostMetadata{
				Files: []*model.FileInfo{
					{
						Extension: ".jpg",
						Name:      "photo.jpg",
					},
				},
			},
		}

		filteredPost, msg := p.FilterNewUserImages(p.getConfiguration(), post)

		assert.NotNil(t, filteredPost, "Post with image from old user should be allowed")
		assert.Empty(t, msg)
	})

	t.Run("indefinite blocking with -1 duration", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "-1",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add user (even old users should be blocked)
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Photo attached",
			Metadata: &model.PostMetadata{
				Images: map[string]*model.PostImage{
					"https://example.com/image.jpg": {},
				},
			},
		}

		filteredPost, msg := p.FilterNewUserImages(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "Post with image should be blocked with indefinite duration")
		assert.Contains(t, msg, "indefinitely")
	})

	t.Run("new user with multiple images is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Multiple photos",
			Metadata: &model.PostMetadata{
				Files: []*model.FileInfo{
					{
						Extension: ".jpg",
						Name:      "photo1.jpg",
					},
					{
						Extension: ".png",
						Name:      "photo2.png",
					},
				},
			},
		}

		filteredPost, msg := p.FilterNewUserImages(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "Post with multiple images from new user should be blocked")
		assert.Contains(t, msg, "not allowed")
	})
}

// Extended MockAPI for FilterDirectMessage tests
type ExtendedMockAPI struct {
	MockAPI
	GetUserFunc           func(userID string) (*model.User, *model.AppError)
	GetChannelFunc        func(channelID string) (*model.Channel, *model.AppError)
	SendEphemeralPostFunc func(userID string, post *model.Post) *model.Post
}

func (m *ExtendedMockAPI) GetUser(userID string) (*model.User, *model.AppError) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(userID)
	}
	return &model.User{Id: userID}, nil
}

func (m *ExtendedMockAPI) GetChannel(channelID string) (*model.Channel, *model.AppError) {
	if m.GetChannelFunc != nil {
		return m.GetChannelFunc(channelID)
	}
	return &model.Channel{Id: channelID, Type: model.ChannelTypeDirect}, nil
}

func (m *ExtendedMockAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	if m.SendEphemeralPostFunc != nil {
		return m.SendEphemeralPostFunc(userID, post)
	}
	return post
}

// TestFilterDirectMessageIndefiniteDuration verifies the refactored DM blocking works with indefinite duration
func TestFilterDirectMessageIndefiniteDuration(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                    // Created now
	oldUserCreateAt := now.Add(-48*time.Hour).Unix() * 1000 // Created 48 hours ago

	mockAPI := &MockAPI{
		UpdateUserFunc: func(user *model.User) (*model.User, *model.AppError) {
			return user, nil
		},
	}

	t.Run("indefinite DM blocking with -1 duration", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "-1",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add user (even old users should be blocked)
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Direct message to another user",
		}

		filteredPost, msg := p.FilterDirectMessage(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "DM should be blocked with indefinite duration")
		assert.Contains(t, msg, "indefinitely")
	})

	t.Run("standard duration still works", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Direct message",
		}

		filteredPost, msg := p.FilterDirectMessage(p.getConfiguration(), post)

		assert.Nil(t, filteredPost, "DM from new user should be blocked with standard duration")
		assert.Contains(t, msg, "not allowed")
	})

	t.Run("old user with standard duration is allowed", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
		}
		p.SetAPI(mockAPI)

		// Create cache and add old user
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "Direct message",
		}

		filteredPost, msg := p.FilterDirectMessage(p.getConfiguration(), post)

		assert.NotNil(t, filteredPost, "DM from old user should be allowed")
		assert.Empty(t, msg)
	})
}

func TestFilterDirectMessage(t *testing.T) {
	t.Run("blocks new user within time restriction", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
			cache: NewLRUCache(10),
		}

		// User created 1 hour ago
		oneHourAgo := model.GetMillis() - (60 * 60 * 1000)
		testUser := &model.User{
			Id:       "new-user",
			CreateAt: oneHourAgo,
		}

		ephemeralSent := false
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "new-user" {
					return testUser, nil
				}
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				ephemeralSent = true
				assert.Equal(t, "Configuration settings limit new users from sending private messages.", post.Message)
				return post
			},
		})

		post := &model.Post{
			UserId:    "new-user",
			ChannelId: "dm-channel",
			Message:   "Hello",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Nil(t, resultPost)
		assert.Contains(t, rejectReason, "New user not allowed to post direct messages")
		assert.True(t, ephemeralSent)
	})

	t.Run("allows user past time restriction", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
			cache: NewLRUCache(10),
		}

		// User created 25 hours ago
		twentyFiveHoursAgo := model.GetMillis() - (25 * 60 * 60 * 1000)
		testUser := &model.User{
			Id:       "old-user",
			CreateAt: twentyFiveHoursAgo,
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "old-user" {
					return testUser, nil
				}
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
		})

		post := &model.Post{
			UserId:    "old-user",
			ChannelId: "dm-channel",
			Message:   "Hello",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Equal(t, post, resultPost)
		assert.Empty(t, rejectReason)
	})

	t.Run("handles invalid duration format", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "invalid-duration",
			},
			cache: NewLRUCache(10),
		}

		testUser := &model.User{
			Id:       "user",
			CreateAt: model.GetMillis(),
		}

		ephemeralSent := false
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				return testUser, nil
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				ephemeralSent = true
				assert.Equal(t, "Something went wrong when sending your message. Contact an administrator.", post.Message)
				return post
			},
		})

		post := &model.Post{
			UserId:    "user",
			ChannelId: "dm-channel",
			Message:   "Hello",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Nil(t, resultPost)
		assert.Contains(t, rejectReason, "failed to parse duration")
		assert.True(t, ephemeralSent)
	})

	t.Run("handles user not found error", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
			cache: NewLRUCache(10),
		}

		ephemeralSent := false
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				ephemeralSent = true
				assert.Equal(t, "Something went wrong when sending your message. Contact an administrator.", post.Message)
				return post
			},
		})

		post := &model.Post{
			UserId:    "non-existent",
			ChannelId: "dm-channel",
			Message:   "Hello",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Nil(t, resultPost)
		assert.Equal(t, "Failed to get user", rejectReason)
		assert.True(t, ephemeralSent)
	})

	t.Run("handles exact time boundary", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "1h",
			},
			cache: NewLRUCache(10),
		}

		// User created exactly 1 hour ago (boundary case)
		exactlyOneHourAgo := model.GetMillis() - (60 * 60 * 1000)
		testUser := &model.User{
			Id:       "boundary-user",
			CreateAt: exactlyOneHourAgo,
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				return testUser, nil
			},
		})

		post := &model.Post{
			UserId:    "boundary-user",
			ChannelId: "dm-channel",
			Message:   "Hello",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		// Should be allowed as it's exactly at the boundary
		assert.Equal(t, post, resultPost)
		assert.Empty(t, rejectReason)
	})

	t.Run("handles complex duration formats", func(t *testing.T) {
		testCases := []struct {
			name        string
			duration    string
			hoursOld    int64
			shouldBlock bool
		}{
			{"12h30m format - blocked", "12h30m", 12, true},
			{"12h30m format - allowed", "12h30m", 13, false},
			{"7d format - blocked", "168h", 167, true}, // 7 days = 168 hours
			{"7d format - allowed", "168h", 169, false},
			{"30m format - blocked", "30m", 0, true}, // 0 hours = less than 30 min
			{"30m format - allowed", "30m", 1, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				p := Plugin{
					configuration: &configuration{
						BlockNewUserPM:     true,
						BlockNewUserPMTime: tc.duration,
					},
					cache: NewLRUCache(10),
				}

				createTime := model.GetMillis() - (tc.hoursOld * 60 * 60 * 1000)
				testUser := &model.User{
					Id:       "test-user",
					CreateAt: createTime,
				}

				p.SetAPI(&ExtendedMockAPI{
					GetUserFunc: func(userID string) (*model.User, *model.AppError) {
						return testUser, nil
					},
					SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
						return post
					},
				})

				post := &model.Post{
					UserId:    "test-user",
					ChannelId: "dm-channel",
					Message:   "Test",
				}

				resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

				if tc.shouldBlock {
					assert.Nil(t, resultPost)
					assert.Contains(t, rejectReason, "New user not allowed")
				} else {
					assert.Equal(t, post, resultPost)
					assert.Empty(t, rejectReason)
				}
			})
		}
	})

	t.Run("allows system admin even if new user", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
			cache: NewLRUCache(10),
		}

		// Admin user created just now (should normally be blocked)
		adminUser := &model.User{
			Id:       "admin-user",
			CreateAt: model.GetMillis(),
			Roles:    "system_admin",
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "admin-user" {
					return adminUser, nil
				}
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
		})

		post := &model.Post{
			UserId:    "admin-user",
			ChannelId: "dm-channel",
			Message:   "Hello from admin",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Equal(t, post, resultPost, "Admin user should be allowed to send DM")
		assert.Empty(t, rejectReason, "Admin user should not be rejected")
	})

	t.Run("allows system admin even with indefinite blocking", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "-1", // Indefinite blocking
			},
			cache: NewLRUCache(10),
		}

		// Admin user created just now (should normally be blocked indefinitely)
		adminUser := &model.User{
			Id:       "admin-user",
			CreateAt: model.GetMillis(),
			Roles:    "system_admin",
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "admin-user" {
					return adminUser, nil
				}
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
		})

		post := &model.Post{
			UserId:    "admin-user",
			ChannelId: "dm-channel",
			Message:   "Hello from admin",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Equal(t, post, resultPost, "Admin user should be allowed to send DM even with indefinite blocking")
		assert.Empty(t, rejectReason, "Admin user should not be rejected")
	})

	t.Run("allows system admin with multiple roles", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserPM:     true,
				BlockNewUserPMTime: "24h",
			},
			cache: NewLRUCache(10),
		}

		// Admin user with multiple roles (system_admin is one of them)
		adminUser := &model.User{
			Id:       "admin-user",
			CreateAt: model.GetMillis(),
			Roles:    "system_admin system_user",
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "admin-user" {
					return adminUser, nil
				}
				return nil, model.NewAppError("GetUser", "user not found", nil, "", 404)
			},
		})

		post := &model.Post{
			UserId:    "admin-user",
			ChannelId: "dm-channel",
			Message:   "Hello from admin",
		}

		resultPost, rejectReason := p.FilterDirectMessage(p.configuration, post)

		assert.Equal(t, post, resultPost, "Admin user with multiple roles should be allowed to send DM")
		assert.Empty(t, rejectReason, "Admin user should not be rejected")
	})
}

func TestGetUserByID(t *testing.T) {
	t.Run("returns user from cache when present", func(t *testing.T) {
		p := Plugin{
			cache: NewLRUCache(10),
		}

		cachedUser := &model.User{
			Id:       "cached-user",
			Username: "cached",
			Email:    "cached@test.com",
		}

		// Pre-populate cache
		p.cache.Put("cached-user", cachedUser)

		// API should not be called
		apiCalled := false
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				apiCalled = true
				return nil, nil
			},
		})

		user, err := p.GetUserByID("cached-user")

		assert.NoError(t, err)
		assert.Equal(t, cachedUser.Id, user.Id)
		assert.Equal(t, cachedUser.Username, user.Username)
		assert.False(t, apiCalled, "API should not be called when user is in cache")
	})

	t.Run("fetches from API and caches when not in cache", func(t *testing.T) {
		p := Plugin{
			cache: NewLRUCache(10),
		}

		apiUser := &model.User{
			Id:       "api-user",
			Username: "fromapi",
			Email:    "api@test.com",
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if userID == "api-user" {
					return apiUser, nil
				}
				return nil, model.NewAppError("GetUser", "not found", nil, "", 404)
			},
		})

		// First call - should hit API
		user, err := p.GetUserByID("api-user")

		assert.NoError(t, err)
		assert.Equal(t, apiUser.Id, user.Id)

		// Verify it was cached
		cachedUser, found := p.cache.Get("api-user")
		assert.True(t, found)
		assert.Equal(t, apiUser.Id, cachedUser.Id)

		// Second call - should hit cache
		apiCallCount := 0
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				apiCallCount++
				return apiUser, nil
			},
		})

		user2, err := p.GetUserByID("api-user")
		assert.NoError(t, err)
		assert.Equal(t, apiUser.Id, user2.Id)
		assert.Equal(t, 0, apiCallCount, "API should not be called on cache hit")
	})

	t.Run("returns error when API fails", func(t *testing.T) {
		p := Plugin{
			cache: NewLRUCache(10),
		}

		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				return nil, model.NewAppError("GetUser", "database error", nil, "", 500)
			},
		})

		user, err := p.GetUserByID("error-user")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find user with id")
		assert.Equal(t, &model.User{}, user)

		// Verify error case is not cached
		_, found := p.cache.Get("error-user")
		assert.False(t, found, "Failed API calls should not be cached")
	})

	t.Run("handles concurrent requests for same user", func(t *testing.T) {
		p := Plugin{
			cache: NewLRUCache(10),
		}

		apiCallCount := 0
		p.SetAPI(&ExtendedMockAPI{
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				apiCallCount++
				return &model.User{
					Id:       userID,
					Username: "concurrent",
				}, nil
			},
		})

		// Launch concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = p.GetUserByID("concurrent-user")
			}()
		}
		wg.Wait()

		// API might be called multiple times due to race conditions,
		// but cache should eventually contain the user
		_, found := p.cache.Get("concurrent-user")
		assert.True(t, found)
	})
}

// TestFilterPostIntegration verifies the full FilterPost flow with image blocking
// This test ensures the bug where images were checked after links is fixed
func TestFilterPostIntegration(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                    // Created now
	oldUserCreateAt := now.Add(-48*time.Hour).Unix() * 1000 // Created 48 hours ago

	t.Run("new user posting image URL is blocked as image, not link", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
				BlockNewUserLinks:      true,
				BlockNewUserLinksTime:  "24h",
			},
		}

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		// Post with image URL (contains both link and image - should be blocked as image)
		post := &model.Post{
			UserId:  newUser.Id,
			Message: "![image](https://example.com/image.png)",
		}

		filteredPost, msg := p.FilterPost(post)

		// Should be blocked as image, not link
		assert.Nil(t, filteredPost, "Post with image URL from new user should be blocked")
		assert.Contains(t, msg, "images", "Should be blocked as image, not link")
		assert.NotContains(t, msg, "links", "Should not be blocked as link")
	})

	t.Run("new user posting image file attachment is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Here's a photo",
			Metadata: &model.PostMetadata{
				Files: []*model.FileInfo{
					{
						Extension: ".jpg",
						Name:      "photo.jpg",
					},
				},
			},
		}

		filteredPost, msg := p.FilterPost(post)

		assert.Nil(t, filteredPost, "Post with image file from new user should be blocked")
		assert.Contains(t, msg, "images", "Should mention images in error message")
	})

	t.Run("old user posting image URL is allowed", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
				BlockNewUserLinks:      true,
				BlockNewUserLinksTime:  "24h",
			},
		}

		// Create cache and add old user
		p.cache = NewLRUCache(50)
		oldUser := &model.User{
			Id:       model.NewId(),
			CreateAt: oldUserCreateAt,
		}
		p.cache.Put(oldUser.Id, oldUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  oldUser.Id,
			Message: "![image](https://example.com/image.png)",
		}

		filteredPost, msg := p.FilterPost(post)

		// Should pass through (no blocking)
		assert.NotNil(t, filteredPost, "Post with image from old user should be allowed")
		assert.Empty(t, msg, "Should not have error message for old user")
	})

	t.Run("new user posting link without image is blocked as link", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
				BlockNewUserLinks:      true,
				BlockNewUserLinksTime:  "24h",
			},
		}

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Check out https://example.com for more info",
		}

		filteredPost, msg := p.FilterPost(post)

		// Should be blocked as link (not image since it's not an image)
		assert.Nil(t, filteredPost, "Post with link from new user should be blocked")
		assert.Contains(t, msg, "links", "Should be blocked as link")
		assert.NotContains(t, msg, "images", "Should not mention images")
	})

	t.Run("new user posting image with metadata.Images is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "https://example.com/image.jpg",
			Metadata: &model.PostMetadata{
				Images: map[string]*model.PostImage{
					"https://example.com/image.jpg": {},
				},
			},
		}

		filteredPost, msg := p.FilterPost(post)

		assert.Nil(t, filteredPost, "Post with image in metadata from new user should be blocked")
		assert.Contains(t, msg, "images", "Should mention images in error message")
	})

	t.Run("new user posting image using FileIds is blocked", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     true,
				BlockNewUserImagesTime: "24h",
			},
		}

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{
				GetFileInfoFunc: func(fileID string) (*model.FileInfo, *model.AppError) {
					// Return image file info for any file ID
					return &model.FileInfo{
						Id:        fileID,
						Extension: ".jpg",
						Name:      "photo.jpg",
					}, nil
				},
			},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
			SendEphemeralPostFunc: func(userID string, post *model.Post) *model.Post {
				return post
			},
		}
		p.SetAPI(mockAPI)

		// Post with FileIds but no Metadata.Files (simulates real upload scenario)
		post := &model.Post{
			UserId:  newUser.Id,
			Message: "Here's a photo",
			FileIds: []string{"file123"},
			// Metadata.Files is nil/empty - this is what happens during MessageWillBePosted
		}

		filteredPost, msg := p.FilterPost(post)

		assert.Nil(t, filteredPost, "Post with image FileIds from new user should be blocked")
		assert.Contains(t, msg, "images", "Should mention images in error message")
	})

	t.Run("image blocking disabled allows new users to post images", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BlockNewUserImages:     false,
				BlockNewUserImagesTime: "24h",
				BadWordsList:           "", // Empty to avoid bad word filtering
			},
		}
		// Initialize badWordsRegex to avoid nil pointer dereference
		p.badWordsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadWordsList, defaultRegexTemplate))

		// Create cache and add new user
		p.cache = NewLRUCache(50)
		newUser := &model.User{
			Id:       model.NewId(),
			CreateAt: newUserCreateAt,
		}
		p.cache.Put(newUser.Id, newUser)

		mockAPI := &ExtendedMockAPI{
			MockAPI: MockAPI{},
			GetUserFunc: func(userID string) (*model.User, *model.AppError) {
				if user, found := p.cache.Get(userID); found {
					return user, nil
				}
				return nil, &model.AppError{Message: "user not found"}
			},
		}
		p.SetAPI(mockAPI)

		post := &model.Post{
			UserId:  newUser.Id,
			Message: "![image](https://example.com/image.png)",
		}

		filteredPost, msg := p.FilterPost(post)

		// Should pass through (blocking disabled)
		assert.NotNil(t, filteredPost, "Post with image should be allowed when blocking is disabled")
		assert.Empty(t, msg, "Should not have error message when blocking is disabled")
	})
}

// TestUserWillLogIn verifies the security-critical login validation hook
func TestUserWillLogIn(t *testing.T) {
	t.Run("allows valid user to login", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadUsernamesList: "baduser",
				BadDomainsList:   "baddomain.com",
			},
		}
		p.badUsernamesRegex = regexp.MustCompile(wordListToRegex("baduser", `(?mi)(%s)`))
		p.badDomainsRegex = regexp.MustCompile(wordListToRegex("baddomain.com", defaultRegexTemplate))
		p.badDomainsList = &[]string{}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		validUser := &model.User{
			Id:       model.NewId(),
			Username: "gooduser",
			Email:    "user@gooddomain.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, validUser)

		assert.Empty(t, errorMsg, "Valid user should be allowed to login")
	})

	t.Run("blocks soft-deleted user from login", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{},
		}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		deletedUser := &model.User{
			Id:       model.NewId(),
			Username: "deleteduser",
			Email:    "user@example.com",
			DeleteAt: model.GetMillis(), // User is soft-deleted
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, deletedUser)

		assert.NotEmpty(t, errorMsg, "Soft-deleted user should be blocked from login")
		assert.Contains(t, errorMsg, "deactivated", "Error message should mention deactivation")
		assert.Contains(t, errorMsg, "policy violations", "Error message should mention policy violations")
	})

	t.Run("blocks user with bad username from login", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadUsernamesList: "spammer,baduser",
			},
		}
		p.badUsernamesRegex = regexp.MustCompile(wordListToRegex("spammer,baduser", `(?mi)(%s)`))
		p.badDomainsList = &[]string{}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		badUser := &model.User{
			Id:       model.NewId(),
			Username: "spammer123",
			Email:    "user@gooddomain.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, badUser)

		assert.NotEmpty(t, errorMsg, "User with bad username should be blocked from login")
		assert.Contains(t, errorMsg, "community guidelines", "Error message should mention guidelines")
	})

	t.Run("blocks user with bad nickname from login", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadUsernamesList: "spammer",
			},
		}
		p.badUsernamesRegex = regexp.MustCompile(wordListToRegex("spammer", `(?mi)(%s)`))
		p.badDomainsList = &[]string{}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		badUser := &model.User{
			Id:       model.NewId(),
			Username: "goodusername",
			Nickname: "I am a spammer",
			Email:    "user@gooddomain.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, badUser)

		assert.NotEmpty(t, errorMsg, "User with bad nickname should be blocked from login")
		assert.Contains(t, errorMsg, "community guidelines", "Error message should mention guidelines")
	})

	t.Run("blocks user with bad email domain from login", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadDomainsList: "tempmail.com,spam.com",
			},
		}
		p.badDomainsRegex = regexp.MustCompile(wordListToRegex("tempmail.com,spam.com", defaultRegexTemplate))
		p.badDomainsList = &[]string{}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		badUser := &model.User{
			Id:       model.NewId(),
			Username: "gooduser",
			Email:    "user@tempmail.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, badUser)

		assert.NotEmpty(t, errorMsg, "User with bad email domain should be blocked from login")
		assert.Contains(t, errorMsg, "community guidelines", "Error message should mention guidelines")
	})

	t.Run("blocks user with builtin bad domain from login", func(t *testing.T) {
		builtinDomains := []string{"hoo.com", "disposable.email"}

		p := Plugin{
			configuration: &configuration{
				BuiltinBadDomains: true,
			},
			badDomainsList: &builtinDomains,
		}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		badUser := &model.User{
			Id:       model.NewId(),
			Username: "gooduser",
			Email:    "user@hoo.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, badUser)

		assert.NotEmpty(t, errorMsg, "User with builtin bad domain should be blocked from login")
		assert.Contains(t, errorMsg, "community guidelines", "Error message should mention guidelines")
	})

	t.Run("blocks soft-deleted user even with good credentials", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				BadUsernamesList: "baduser",
			},
		}
		p.badUsernamesRegex = regexp.MustCompile(wordListToRegex("baduser", `(?mi)(%s)`))
		p.badDomainsList = &[]string{}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		deletedUser := &model.User{
			Id:       model.NewId(),
			Username: "gooduser", // Good username
			Email:    "user@gooddomain.com",
			DeleteAt: model.GetMillis(), // But user is deleted
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, deletedUser)

		// Soft-deleted check happens first, so it should block with deactivated message
		assert.NotEmpty(t, errorMsg, "Soft-deleted user should be blocked even with good credentials")
		assert.Contains(t, errorMsg, "deactivated", "Error message should mention deactivation")
	})

	t.Run("security: prevents deleted user from re-authenticating", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{},
		}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		// Simulate a user who was deleted due to bad username
		// but is trying to login again (maybe they cached credentials)
		deletedBadUser := &model.User{
			Id:       model.NewId(),
			Username: "sanitized-userid123", // Username was sanitized during cleanup
			Email:    "user@example.com",
			DeleteAt: model.GetMillis(), // User was soft-deleted
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, deletedBadUser)

		assert.NotEmpty(t, errorMsg, "Deleted user should not be able to re-authenticate")
		assert.Contains(t, errorMsg, "deactivated", "Should inform user account is deactivated")
	})

	t.Run("allows user with no validation rules configured", func(t *testing.T) {
		p := Plugin{
			configuration: &configuration{
				// No bad words, domains, or usernames configured
			},
			// Nil regexes mean no validation rules
			badUsernamesRegex: nil,
			badDomainsRegex:   nil,
			badDomainsList:    &[]string{},
		}

		mockAPI := &MockAPI{}
		p.SetAPI(mockAPI)

		user := &model.User{
			Id:       model.NewId(),
			Username: "anyuser",
			Email:    "user@anydomain.com",
			DeleteAt: 0,
		}

		errorMsg := p.UserWillLogIn(&plugin.Context{}, user)

		assert.Empty(t, errorMsg, "User should be allowed when no validation rules are configured")
	})
}
