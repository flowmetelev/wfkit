package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wfkit/internal/config"
	devproxy "wfkit/internal/proxy"
	"wfkit/internal/utils"

	"github.com/urfave/cli/v2"
)

func proxyMode(c *cli.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cfg.ValidateProxy(); err != nil {
		return err
	}

	devHost := resolveStringFlag(c, "dev-host", cfg.DevHost)
	proxyHost := resolveStringFlag(c, "proxy-host", cfg.ProxyHost)
	if sharedHost := resolveStringFlag(c, "host", ""); sharedHost != "" {
		devHost = sharedHost
		proxyHost = sharedHost
	}
	devPort := resolveIntFlag(c, "dev-port", cfg.DevPort)
	proxyPort := resolveIntFlag(c, "proxy-port", cfg.ProxyPort)
	openBrowserFlag := resolveBoolFlag(c, "open", cfg.OpenBrowser)
	scriptURL := resolveStringFlag(c, "script-url", "")
	if scriptURL == "" {
		scriptURL = buildLocalScriptURL(devHost, devPort, cfg.GlobalEntry)
	}

	targetURL := resolveStringFlag(c, "site-url", cfg.EffectiveSiteURL())
	if targetURL == "" {
		return fmt.Errorf("missing site URL: set --site-url or configure siteUrl/appName in wfkit.json")
	}
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return fmt.Errorf("invalid site URL: %w", err)
	}

	proxyListenHost := resolveListenHost(proxyHost)
	devListenHost := resolveListenHost(devHost)

	resolvedProxyPort, relocated, err := findAvailablePort(proxyListenHost, proxyPort)
	if err != nil {
		return fmt.Errorf("failed to reserve proxy port: %w", err)
	}
	if relocated {
		utils.CPrint(fmt.Sprintf("Proxy port %d is busy, using %d instead", proxyPort, resolvedProxyPort), "yellow")
	}
	proxyPort = resolvedProxyPort

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	devServer, err := ensureDevServerRunning(ctx, cfg.PackageManager, scriptURL, devListenHost, devPort)
	if err != nil {
		return err
	}
	defer func() {
		_ = devServer.Stop(5 * time.Second)
	}()

	proxyURL := fmt.Sprintf("http://%s:%d", displayHost(proxyHost), proxyPort)
	proxyErrCh := make(chan error, 1)
	go func() {
		proxyErrCh <- devproxy.Serve(ctx, devproxy.Options{
			ListenHost: proxyListenHost,
			ListenPort: proxyPort,
			ScriptURL:  scriptURL,
			TargetURL:  targetURL,
		})
	}()

	if err := waitForURL(ctx, proxyURL, 10*time.Second); err != nil {
		cancel()
		return fmt.Errorf("proxy did not become ready at %s: %w", proxyURL, err)
	}

	printProxyStatus(targetURL, scriptURL, proxyURL)

	if openBrowserFlag {
		if err := openURL(proxyURL); err != nil {
			utils.CPrint(fmt.Sprintf("Failed to open browser: %v", err), "yellow")
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var retErr error
	select {
	case <-sigCh:
		utils.CPrint("Shutting down dev proxy...", "yellow")
	case err := <-proxyErrCh:
		if err != nil {
			retErr = fmt.Errorf("proxy server failed: %w", err)
		}
	}

	cancel()

	if err := devServer.Stop(5 * time.Second); err != nil {
		return fmt.Errorf("development server error: %v", err)
	}

	return retErr
}
