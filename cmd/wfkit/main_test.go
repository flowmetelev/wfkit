package main

import "testing"

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
