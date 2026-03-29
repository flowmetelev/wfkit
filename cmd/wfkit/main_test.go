package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"wfkit/internal/config"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

func TestInteractiveActionOptionsIncludePagesManagement(t *testing.T) {
	options := interactiveActionOptions()
	foundCMS := false
	for _, option := range options {
		if option.Key == "Manage pages" && option.Value == "pages" {
			foundCMS = true
		}
		if option.Key == "Manage CMS" && option.Value == "cms" {
			return
		}
	}

	if !foundCMS {
		t.Fatal("expected interactive action options to include Manage pages")
	}

	t.Fatal("expected interactive action options to include Manage CMS")
}

func TestPreferredDevScriptPrefersDedicatedViteScript(t *testing.T) {
	script := preferredDevScript(map[string]string{
		"dev":      "wfkit proxy",
		"dev:vite": "vite",
	})

	if script != "dev:vite" {
		t.Fatalf("expected dev:vite, got %q", script)
	}
}

func TestPreferredDevScriptFallsBackToViteThenDev(t *testing.T) {
	if got := preferredDevScript(map[string]string{"vite": "vite"}); got != "vite" {
		t.Fatalf("expected vite fallback, got %q", got)
	}

	if got := preferredDevScript(map[string]string{"dev": "vite"}); got != "dev" {
		t.Fatalf("expected dev fallback, got %q", got)
	}
}

func TestNewPublishRequestDefaultsToProdEnvWithoutPublishFlags(t *testing.T) {
	app := &cli.App{}
	set := flag.NewFlagSet("wfkit", flag.ContinueOnError)
	ctx := cli.NewContext(app, set, nil)

	request := newPublishRequest(ctx, config.Config{
		AssetBranch: "wfkit-dist",
		BuildDir:    "dist/assets",
		DevHost:     "localhost",
		DevPort:     5173,
	})

	if request.env() != "prod" {
		t.Fatalf("expected default publish env prod, got %q", request.env())
	}
}

func TestMigrateLoadConfigSupportsInlinePublishBeforeArgsInit(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	if err := os.WriteFile(filepath.Join(tmpDir, "wfkit.json"), []byte(`{
		"appName": "demo-site",
		"packageManager": "bun",
		"deliveryMode": "cdn"
	}`), 0o644); err != nil {
		t.Fatalf("write wfkit.json: %v", err)
	}

	app := &cli.App{}
	set := flag.NewFlagSet("wfkit", flag.ContinueOnError)
	_ = set.Bool("publish", false, "")
	_ = set.String("delivery", "", "")
	_ = set.String("pages-dir", "", "")
	_ = set.String("asset-branch", "", "")
	_ = set.String("branch", "", "")
	_ = set.String("build-dir", "", "")
	_ = set.String("custom-commit", "", "")
	_ = set.Bool("notify", false, "")
	if err := set.Parse([]string{"--publish", "--delivery", "inline"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	ctx := cli.NewContext(app, set, nil)
	flow := newMigrateFlow(ctx)
	if err := flow.loadConfig(); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if got := flow.delivery(); got != "inline" {
		t.Fatalf("expected inline delivery, got %q", got)
	}
}

func TestValidatePublishReadinessRequiresPublishAndCustomCode(t *testing.T) {
	err := validatePublishReadiness(webflow.PublishPreflight{
		Permissions: webflow.PublishPermissions{
			CanPublish:          true,
			CanManageCustomCode: false,
		},
		Domains: webflow.PublishDomains{
			StagingDomain: webflow.PublishDomain{Name: "demo.webflow.io"},
		},
	}, "staging")
	if err == nil {
		t.Fatal("expected publish readiness validation to fail")
	}
}

func TestDoctorChecksFromPublishPreflightWarnsWithoutProductionDomains(t *testing.T) {
	checks := doctorChecksFromPublishPreflight(webflow.PublishPreflight{
		Permissions: webflow.PublishPermissions{
			CanPublish:          true,
			CanManageCustomCode: true,
			CanDesign:           true,
		},
		Domains: webflow.PublishDomains{
			StagingDomain: webflow.PublishDomain{Name: "demo.webflow.io"},
		},
		Credentials:       webflow.PublishCredentials{SecretCount: 0},
		NumberOfPublishes: 12,
	})

	foundProduction := false
	for _, check := range checks {
		if check.Name != "Production destinations" {
			continue
		}
		foundProduction = true
		if check.Status != doctorWarn {
			t.Fatalf("expected production destinations warning, got %s", check.Status)
		}
	}

	if !foundProduction {
		t.Fatal("expected production destinations check to be present")
	}
}

func TestResolvePublishTargetsSupportsAllModes(t *testing.T) {
	preflight := webflow.PublishPreflight{
		Domains: webflow.PublishDomains{
			StagingDomain: webflow.PublishDomain{Name: "demo.webflow.io"},
			ProductionDomains: []webflow.PublishDomain{
				{Name: "example.com"},
				{Name: "www.example.com"},
			},
		},
	}

	if got := resolvePublishTargets(preflight, "staging"); len(got) != 1 || got[0] != "demo.webflow.io" {
		t.Fatalf("unexpected staging targets: %#v", got)
	}

	if got := resolvePublishTargets(preflight, "production"); len(got) != 2 || got[0] != "example.com" || got[1] != "www.example.com" {
		t.Fatalf("unexpected production targets: %#v", got)
	}

	if got := resolvePublishTargets(preflight, "all"); len(got) != 3 {
		t.Fatalf("unexpected all targets: %#v", got)
	}
}

func TestValidatePublishReadinessRequiresProductionDomainsForProductionTarget(t *testing.T) {
	err := validatePublishReadiness(webflow.PublishPreflight{
		Permissions: webflow.PublishPermissions{
			CanPublish:          true,
			CanManageCustomCode: true,
		},
		Domains: webflow.PublishDomains{
			StagingDomain: webflow.PublishDomain{Name: "demo.webflow.io"},
		},
	}, "production")
	if err == nil {
		t.Fatal("expected production target validation to fail without production domains")
	}
}
