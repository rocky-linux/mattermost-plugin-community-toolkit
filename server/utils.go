package main

import (
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

func (p *Plugin) isDirectMessage(channelId string) bool { 
	channel, err := p.API.GetChannel(channelId)
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