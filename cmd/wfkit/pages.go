package main

import "github.com/urfave/cli/v2"

func pagesListMode(c *cli.Context) error {
	return newPagesFlow(c).runList()
}

func pagesCreateMode(c *cli.Context) error {
	return newPagesFlow(c).runCreate()
}

func pagesTypesMode(c *cli.Context) error {
	return newPagesFlow(c).runTypes()
}
