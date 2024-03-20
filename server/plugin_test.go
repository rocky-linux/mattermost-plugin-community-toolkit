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

func TestUserHasBeenCreated(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			CensorCharacter:  "*",
			RejectPosts:      false,
			BadWordsList:     "def ghi,abc",
			BadDomainsList:   "baddomain.com,bad.org",
			BadUsernamesList: "hate",
			ExcludeBots:      true,
			BlockNewUserPM: true,
			BlockNewUserPMTime: "4h",
		},
	}
	p.SetAPI(&MockAPI{})
	p.badDomainsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadDomainsList, defaultRegexTemplate))
	p.badUsernamesRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().BadUsernamesList, `(?mi)(%s)`))

	t.Run("user matching word is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id: id,
			Email:       id + "@gooddomain.com",
			Nickname:    "Neil Sucks",
			Username:    "ihateneil-" + id,
			Password:    "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.NotEqual(t, user.Username, original.Username)
	})

	// NOTE(nhanlon): 2024-03-20 I'm not sure if this test is really necessary, but I'm including it to 
	// highlight that your badDomain list must be curated carefully and the combinations of potential word bits.
	t.Run("user matching word stub is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id: id,
			Email:       id + "@gooddomain.com",
			Nickname:    "Neil Sucks",
			Username:    "shakeoffthehaters-" + id,
			Password:    "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.Equal(t, user.Username, original.Username)
	})

	t.Run("user matching email is banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id: id,
			Email:       id + "@baddomain.com",
			Nickname:    "Neil Is Awesome",
			Username:    "neilfan-" + id,
			Password:    "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.NotEqual(t, user.Username, original.Username)
	})

	t.Run("user not matching email nor name is not banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id: id,
			Email:       id + "@gooddomain.com",
			Nickname:    "Neil Is Awesome",
			Username:    "neilfan-" + id,
			Password:    "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.Equal(t, user.Username, original.Username)
	})

	t.Run("user with email domain partial in bad list is not banned", func(_ *testing.T) {
		id := model.NewId()
		user := &model.User{
			Id: id,
			Email:       id + "@lookslikeabaddomain.com",
			Nickname:    "Neil Is Awesome",
			Username:    "neilfan-" + id,
			Password:    "passwd12345",
		}
		original := *user

		p.UserHasBeenCreated(&plugin.Context{}, user)

		time.Sleep(1 * time.Second)

		assert.Equal(t, user.Username, original.Username)
	})
}
