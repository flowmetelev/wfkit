package main

import (
	"context"
	"fmt"

	"wfkit/internal/config"
	"wfkit/internal/webflow"
)

type doctorFlow struct {
	context  context.Context
	skipAuth bool
	config   config.Config
	checks   []doctorCheck
}

func newDoctorFlow(ctx context.Context, skipAuth bool) *doctorFlow {
	return &doctorFlow{
		context:  ctx,
		skipAuth: skipAuth,
	}
}

func (f *doctorFlow) run() error {
	if err := f.loadConfig(); err != nil {
		return err
	}

	f.collectChecks()
	printDoctorReport(f.checks)

	if f.hasBlockingIssues() {
		return fmt.Errorf("doctor found blocking issues")
	}

	return nil
}

func (f *doctorFlow) loadConfig() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	f.config = cfg
	return nil
}

func (f *doctorFlow) collectChecks() {
	f.checks = append(f.checks,
		checkFileExists("Project config", "wfkit.json"),
		checkFileExists("Package file", "package.json"),
		checkConfigValues(f.config),
		checkCommandAvailable("Package manager", f.config.PackageManager),
		checkCommandAvailable("Git", "git"),
		checkDevScript(),
		checkBuildDirectory(f.config.BuildDir),
		checkPortStatus("Proxy port", f.config.ProxyHost, f.config.ProxyPort),
		checkDevServerStatus(f.config),
	)

	if f.skipAuth {
		f.checks = append(f.checks, doctorCheck{
			Category: "runtime",
			Name:     "Webflow auth",
			Status:   doctorWarn,
			Message:  "skipped by flag",
		})
		f.checks = append(f.checks, doctorCheck{
			Category: "publish",
			Name:     "Publish readiness",
			Status:   doctorWarn,
			Message:  "skipped because auth checks are disabled",
		})
		return
	}

	authCheck, token, cookies := f.checkWebflowSession()
	f.checks = append(f.checks, authCheck)

	if authCheck.Status != doctorPass {
		f.checks = append(f.checks, doctorCheck{
			Category: "publish",
			Name:     "Publish readiness",
			Status:   doctorWarn,
			Message:  "skipped because Webflow authentication is not ready",
		})
		return
	}

	preflight, err := webflow.GetPublishPreflight(f.context, f.config.AppName, token, cookies)
	if err != nil {
		f.checks = append(f.checks, doctorCheck{
			Category: "publish",
			Name:     "Publish readiness",
			Status:   doctorWarn,
			Message:  err.Error(),
		})
		return
	}

	f.checks = append(f.checks, doctorChecksFromPublishPreflight(preflight)...)
}

func (f *doctorFlow) hasBlockingIssues() bool {
	for _, check := range f.checks {
		if check.Status == doctorFail {
			return true
		}
	}

	return false
}

func (f *doctorFlow) checkWebflowSession() (doctorCheck, string, string) {
	designURL := f.config.EffectiveDesignURL()
	if designURL == "" {
		return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorFail, Message: "cannot build design URL without appName"}, "", ""
	}

	token, cookies, err := webflow.GetCsrfTokenAndCookies(f.context, designURL)
	if err != nil {
		return doctorCheck{Category: "runtime", Name: "Webflow auth", Status: doctorWarn, Message: err.Error()}, "", ""
	}

	return doctorCheck{
		Category: "runtime",
		Name:     "Webflow auth",
		Status:   doctorPass,
		Message:  fmt.Sprintf("authenticated against %s", designURL),
	}, token, cookies
}
