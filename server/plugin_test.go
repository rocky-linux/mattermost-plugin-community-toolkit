package main

import (
	// "fmt"
	"regexp"
	"sync"
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
	_ = p.setupBadDomainList()

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
		assert.Contains(t, rejectReason, "New user not allowed to send DM")
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
		assert.Equal(t, "failed to parse duration", rejectReason)
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
