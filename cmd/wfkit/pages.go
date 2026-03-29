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

func pagesInspectMode(c *cli.Context) error {
	return newPagesFlow(c).runInspect()
}

func pagesDeleteMode(c *cli.Context) error {
	return newPagesFlow(c).runDelete()
}

func pagesOpenMode(c *cli.Context) error {
	return newPagesFlow(c).runOpen()
}
