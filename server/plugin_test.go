package main

import (
	// "fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	// "github.com/mattermost/mattermost/server/public/shared/request"
)

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
	UpdateUserFunc func(user *model.User) (*model.User, *model.AppError)
}

func (m *MockAPI) UpdateUser(user *model.User) (*model.User, *model.AppError) {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(user)
	}
	return user, nil // Default behavior
}

func (m *MockAPI) DeleteUser(userID string) *model.AppError {
	return nil
}

func (m *MockAPI) GetTeamsForUser(userID string) ([]*model.Team, *model.AppError) {
	return nil, nil
}

func (m *MockAPI) GetUserByUsername(userName string) (*model.User, *model.AppError) {
	return nil, nil
}

func (m *MockAPI) DeleteTeamMember(teamID string, userID string, adminID string) *model.AppError {
	return nil
}

func (m *MockAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	return post
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
	p.badDomainsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadDomainsList, defaultRegexTemplate))
	p.badUsernamesRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadUsernamesList, `(?mi)(%s)`))
	p.setupBadDomainList()

	t.Run("username matching word is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "ihateneil-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.NotEqual(t, user.Username, original.Username)
	})

	t.Run("nickname matching word is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "reasonable-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.NotEqual(t, user.Username, original.Username)
	})

	// NOTE(nhanlon): 2024-03-20 I'm not sure if this test is really necessary, but I'm including it to
	// highlight that your badDomain list must be curated carefully and the combinations of potential word bits.
	t.Run("user matching word stub is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@gooddomain.com",
			Nickname: "Neil Sucks",
			Username: "shakeoffthehaters-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.NotEqual(t, user.Username, original.Username)
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
		user := &model.User{
			Id:       id,
			Email:    id + "@hoo.com",
			Nickname: "Neil Is Alright",
			Username: "alright-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		assert.NotEqual(t, user.Username, original.Username)
	})

	t.Run("user matching email is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id:       id,
			Email:    id + "@baddomain.com",
			Nickname: "Neil Is Awesome",
			Username: "neilfan-" + id,
			Password: "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.NotEqual(t, user.Username, original.Username)
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

// TestFilterNewUserLinks verifies the link blocking functionality for new users
func TestFilterNewUserLinks(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                      // Created now
	oldUserCreateAt := now.Add(-48 * time.Hour).Unix() * 1000 // Created 48 hours ago

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
	newUserCreateAt := now.Unix() * 1000                      // Created now
	oldUserCreateAt := now.Add(-48 * time.Hour).Unix() * 1000 // Created 48 hours ago

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

// TestFilterDirectMessageIndefiniteDuration verifies the refactored DM blocking works with indefinite duration
func TestFilterDirectMessageIndefiniteDuration(t *testing.T) {
	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                      // Created now
	oldUserCreateAt := now.Add(-48 * time.Hour).Unix() * 1000 // Created 48 hours ago

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
