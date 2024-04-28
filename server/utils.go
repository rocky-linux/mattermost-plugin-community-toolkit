package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, e := transform.String(t, s)
	if e != nil {
		return s
	}

	return output
}

func (p *Plugin) isDirectMessage(channelID string) bool {
	channel, err := p.API.GetChannel(channelID)
	if err != nil {
		panic("couldn't find channel")
	}
	return channel.Type == model.ChannelTypeDirect
}

func (p *Plugin) sendUserEphemeralMessageForPost(post *model.Post, message string) {
	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		Message:   message,
		RootId:    post.RootId,
	})
}

func EndsWith(search []string, email string) bool {
	// search is a slice of strings with WHOLE domains to match against
	// email is a string containing an email which we will compare the DOMAIN part only.
	domain := strings.SplitN(email, "@", 2)
	target := domain[1]
	for _, s := range search {
		// if the TARGET ends with search
		if target == s {
			fmt.Printf("FOUND: %s in %s\n", s, target)
			return true
		}
	}
	return false
}
