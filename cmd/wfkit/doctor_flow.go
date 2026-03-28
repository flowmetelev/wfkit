package main

import (
	"context"
	"fmt"

	"wfkit/internal/config"
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
		return
	}

	f.checks = append(f.checks, checkWebflowAuth(f.context, f.config))
}

func (f *doctorFlow) hasBlockingIssues() bool {
	for _, check := range f.checks {
		if check.Status == doctorFail {
			return true
		}
	}

	return false
}
