package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

const issueFormBaseURL = "https://github.com/yndmitry/wfkit/issues/new"

func openBugReport(c *cli.Context) error {
	target := bugReportURL(nil, c.App.Version, os.Args[1:])
	if err := openURL(target); err != nil {
		return fmt.Errorf("failed to open bug report form: %w", err)
	}
	return nil
}

func openFeatureRequest(c *cli.Context) error {
	target := featureRequestURL()
	if err := openURL(target); err != nil {
		return fmt.Errorf("failed to open feature request form: %w", err)
	}
	return nil
}

func featureRequestURL() string {
	return issueFormURL("feature_request.yml", "[Feature]: ")
}

func issueFormURL(template, title string) string {
	params := url.Values{}
	params.Set("template", template)
	if strings.TrimSpace(title) != "" {
		params.Set("title", title)
	}
	return issueFormBaseURL + "?" + params.Encode()
}
