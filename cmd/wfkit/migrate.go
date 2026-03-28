package main

import "github.com/urfave/cli/v2"

func migrateMode(c *cli.Context) error {
	return newMigrateFlow(c).run()
}
