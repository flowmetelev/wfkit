package main

import (
	"flag"
	"testing"

	"wfkit/internal/config"

	"github.com/urfave/cli/v2"
)

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
