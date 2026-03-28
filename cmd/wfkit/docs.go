package main

import "github.com/urfave/cli/v2"

func docsMode(c *cli.Context) error {
	return newDocsFlow(c).run()
}
