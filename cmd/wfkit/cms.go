package main

import "github.com/urfave/cli/v2"

func cmsCollectionsMode(c *cli.Context) error {
	return newCMSFlow(c).runCollections()
}

func cmsPullMode(c *cli.Context) error {
	return newCMSFlow(c).runPull()
}

func cmsDiffMode(c *cli.Context) error {
	return newCMSFlow(c).runDiff()
}

func cmsPushMode(c *cli.Context) error {
	return newCMSFlow(c).runPush()
}
