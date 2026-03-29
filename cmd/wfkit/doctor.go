package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"wfkit/internal/config"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type doctorStatus string

const (
	doctorPass doctorStatus = "PASS"
	doctorWarn doctorStatus = "WARN"
	doctorFail doctorStatus = "FAIL"
)

type doctorCheck struct {
	Category string
	Name     string
	Status   doctorStatus
	Message  string
}

func doctorMode(c *cli.Context) error {
	return newDoctorFlow(c.Context, c.Bool("skip-auth")).run()
}

func printDoctorReport(checks []doctorCheck) {
	utils.PrintSection("Doctor Report")

	overview := doctorDashboardCards(checks)
	utils.PrintDashboardCards(overview...)

	passCount := 0
	warnCount := 0
	failCount := 0

	for _, check := range checks {
		switch check.Status {
		case doctorWarn:
			warnCount++
		case doctorFail:
			failCount++
		default:
			passCount++
		}

		utils.PrintStatus(string(check.Status), check.Name, check.Message)
	}

	utils.PrintSection("Summary")
	utils.PrintStatus(overallDoctorStatus(warnCount, failCount), "Overall", overallDoctorMessage(warnCount, failCount))
	utils.PrintSummary(
		utils.SummaryMetric{Label: "Passed", Value: fmt.Sprintf("%d", passCount), Tone: "success"},
		utils.SummaryMetric{Label: "Warnings", Value: fmt.Sprintf("%d", warnCount), Tone: "warning"},
		utils.SummaryMetric{Label: "Failures", Value: fmt.Sprintf("%d", failCount), Tone: "danger"},
	)
	fmt.Println()
}

func doctorDashboardCards(checks []doctorCheck) []utils.DashboardCard {
	type aggregate struct {
		pass int
		warn int
		fail int
	}

	categories := []struct {
		key   string
		title string
	}{
		{key: "project", title: "Project"},
		{key: "tooling", title: "Tooling"},
		{key: "runtime", title: "Runtime"},
		{key: "publish", title: "Publish"},
	}

	stats := map[string]*aggregate{
		"project": {},
		"tooling": {},
		"runtime": {},
		"publish": {},
	}

	for _, check := range checks {
		group := stats[check.Category]
		if group == nil {
			group = stats["runtime"]
		}
		switch check.Status {
		case doctorWarn:
			group.warn++
		case doctorFail:
			group.fail++
		default:
			group.pass++
		}
	}

	cards := make([]utils.DashboardCard, 0, len(categories))
	for _, category := range categories {
		group := stats[category.key]
		tone := "success"
		line := "Everything looks ready."
		switch {
		case group.fail > 0:
			tone = "danger"
			line = "Blocking issues need attention."
		case group.warn > 0:
			tone = "warning"
			line = "Usable, but worth checking."
		}

		cards = append(cards, utils.DashboardCard{
			Title: category.title,
			Tone:  tone,
			Metrics: []utils.SummaryMetric{
				{Label: "Pass", Value: fmt.Sprintf("%d", group.pass), Tone: "success"},
				{Label: "Warn", Value: fmt.Sprintf("%d", group.warn), Tone: "warning"},
				{Label: "Fail", Value: fmt.Sprintf("%d", group.fail), Tone: "danger"},
			},
			Lines: []string{line},
		})
	}

	return cards
}

func overallDoctorStatus(warnCount, failCount int) string {
	switch {
	case failCount > 0:
		return "FAIL"
	case warnCount > 0:
		return "WARN"
	default:
		return "OK"
	}
}

func overallDoctorMessage(warnCount, failCount int) string {
	switch {
	case failCount > 0:
		return "Resolve blocking issues before relying on this setup."
	case warnCount > 0:
		return "Core setup works, but a few things are still worth checking."
	default:
		return "Everything looks ready."
	}
}

func checkFileExists(name, path string) doctorCheck {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return doctorCheck{Category: "project", Name: name, Status: doctorFail, Message: fmt.Sprintf("%s is missing", path)}
		}
		return doctorCheck{Category: "project", Name: name, Status: doctorFail, Message: err.Error()}
	}
	return doctorCheck{Category: "project", Name: name, Status: doctorPass, Message: fmt.Sprintf("found %s", path)}
}

func checkConfigValues(cfg config.Config) doctorCheck {
	switch {
	case cfg.AppName == "":
		return doctorCheck{Category: "project", Name: "Config values", Status: doctorFail, Message: "appName is missing"}
	case cfg.EffectiveSiteURL() == "":
		return doctorCheck{Category: "project", Name: "Config values", Status: doctorFail, Message: "siteUrl is missing"}
	case cfg.GitHubUser == "", cfg.RepositoryName == "":
		return doctorCheck{Category: "project", Name: "Config values", Status: doctorWarn, Message: "ghUserName or repositoryName is missing for production publish"}
	default:
		return doctorCheck{Category: "project", Name: "Config values", Status: doctorPass, Message: fmt.Sprintf("site %s, repo %s/%s", cfg.AppName, cfg.GitHubUser, cfg.RepositoryName)}
	}
}

func checkCommandAvailable(name, command string) doctorCheck {
	if command == "" {
		return doctorCheck{Category: "tooling", Name: name, Status: doctorFail, Message: "command is not configured"}
	}
	if _, err := exec.LookPath(command); err != nil {
		return doctorCheck{Category: "tooling", Name: name, Status: doctorFail, Message: fmt.Sprintf("%s is not installed", command)}
	}
	return doctorCheck{Category: "tooling", Name: name, Status: doctorPass, Message: fmt.Sprintf("%s is available", command)}
}

func checkDevScript() doctorCheck {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return doctorCheck{Category: "tooling", Name: "Dev script", Status: doctorFail, Message: "package.json could not be read"}
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return doctorCheck{Category: "tooling", Name: "Dev script", Status: doctorFail, Message: "package.json scripts are invalid"}
	}

	devScript := detectDevScript()
	if _, ok := pkg.Scripts[devScript]; !ok {
		return doctorCheck{Category: "tooling", Name: "Dev script", Status: doctorFail, Message: "no dev or vite script found"}
	}

	return doctorCheck{Category: "tooling", Name: "Dev script", Status: doctorPass, Message: fmt.Sprintf("using `%s`", devScript)}
}

func checkBuildDirectory(buildDir string) doctorCheck {
	if buildDir == "" {
		return doctorCheck{Category: "runtime", Name: "Build directory", Status: doctorFail, Message: "buildDir is empty"}
	}

	absPath, err := filepath.Abs(buildDir)
	if err != nil {
		return doctorCheck{Category: "runtime", Name: "Build directory", Status: doctorWarn, Message: fmt.Sprintf("configured as %s", buildDir)}
	}

	if _, err := os.Stat(buildDir); err != nil {
		if os.IsNotExist(err) {
			return doctorCheck{Category: "runtime", Name: "Build directory", Status: doctorWarn, Message: fmt.Sprintf("%s does not exist yet", absPath)}
		}
		return doctorCheck{Category: "runtime", Name: "Build directory", Status: doctorWarn, Message: err.Error()}
	}

	return doctorCheck{Category: "runtime", Name: "Build directory", Status: doctorPass, Message: absPath}
}

func checkPortStatus(name, host string, port int) doctorCheck {
	if host == "" || port <= 0 {
		return doctorCheck{Category: "runtime", Name: name, Status: doctorWarn, Message: "host or port is not configured"}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		return doctorCheck{Category: "runtime", Name: name, Status: doctorWarn, Message: fmt.Sprintf("%s:%d is already in use", host, port)}
	}
	_ = listener.Close()
	return doctorCheck{Category: "runtime", Name: name, Status: doctorPass, Message: fmt.Sprintf("%s:%d is available", host, port)}
}

func checkDevServerStatus(cfg config.Config) doctorCheck {
	scriptURL := buildLocalScriptURL(cfg.DevHost, cfg.DevPort, cfg.GlobalEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()

	if err := waitForURL(ctx, buildViteClientURL(scriptURL), 750*time.Millisecond); err != nil {
		return doctorCheck{Category: "runtime", Name: "Dev server", Status: doctorWarn, Message: fmt.Sprintf("not running at %s yet", scriptURL)}
	}

	return doctorCheck{Category: "runtime", Name: "Dev server", Status: doctorPass, Message: fmt.Sprintf("reachable at %s", scriptURL)}
}

func checkWebflowAuth(ctx context.Context, cfg config.Config) doctorCheck {
	designURL := cfg.EffectiveDesignURL()
	if designURL == "" {
		return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorFail, Message: "cannot build design URL without appName"}
	}
	if _, err := url.ParseRequestURI(designURL); err != nil {
		return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorFail, Message: "design URL is invalid"}
	}

	authCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	if err := webflow.CheckAuthentication(authCtx, designURL); err != nil {
		return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorWarn, Message: err.Error()}
	}

	return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorPass, Message: fmt.Sprintf("authenticated against %s", designURL)}
}
