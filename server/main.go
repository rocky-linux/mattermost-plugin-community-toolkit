// Package main provides the entry point for the Mattermost community toolkit plugin server.
package main

import (
	"github.com/mattermost/mattermost/server/public/plugin"
)

func main() {
	plugin.ClientMain(&Plugin{})
}
