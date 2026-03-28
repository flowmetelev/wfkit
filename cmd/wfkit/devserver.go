package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"wfkit/internal/utils"
)

type managedProcess struct {
	cmd    *exec.Cmd
	doneCh chan error
	reused bool
}

func ensureDevServerRunning(ctx context.Context, packageManager, scriptURL, listenHost string, port int) (*managedProcess, error) {
	devURL := buildViteClientURL(scriptURL)

	shortCtx, shortCancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer shortCancel()
	if waitForURL(shortCtx, devURL, 750*time.Millisecond) == nil {
		utils.CPrint(fmt.Sprintf("Using existing development server at %s", devURL), "cyan")
		return &managedProcess{reused: true}, nil
	}

	devScript := detectDevScript()
	utils.CPrint(fmt.Sprintf("Starting development server using '%s run %s'...", packageManager, devScript), "cyan")

	cmd := buildDevCommand(ctx, packageManager, devScript, listenHost, port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dev server: %w", err)
	}

	proc := &managedProcess{
		cmd:    cmd,
		doneCh: make(chan error, 1),
	}
	go func() {
		proc.doneCh <- cmd.Wait()
		close(proc.doneCh)
	}()

	if err := waitForProcessURL(ctx, devURL, 20*time.Second, proc.doneCh); err != nil {
		_ = proc.Stop(2 * time.Second)
		return nil, fmt.Errorf("development server did not become ready at %s: %w", devURL, err)
	}

	return proc, nil
}

func (p *managedProcess) Stop(timeout time.Duration) error {
	if p == nil || p.reused || p.cmd == nil {
		return nil
	}

	select {
	case err := <-p.doneCh:
		return normalizeProcessExit(err)
	default:
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(os.Interrupt)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-p.doneCh:
		return normalizeProcessExit(err)
	case <-timer.C:
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}

	return normalizeProcessExit(<-p.doneCh)
}

func printProxyStatus(targetURL, scriptURL, proxyURL string) {
	utils.PrintSection("Proxy Ready")
	utils.PrintKeyValue("Local", proxyURL)
	utils.PrintKeyValue("Target", targetURL)
	utils.PrintKeyValue("Script", scriptURL)
	fmt.Println()
}

func openURL(rawURL string) error {
	var cmd string
	var args []string

	switch {
	case isDarwin():
		cmd = "open"
		args = []string{rawURL}
	case isWindows():
		cmd = "cmd"
		args = []string{"/c", "start", rawURL}
	default:
		cmd = "xdg-open"
		args = []string{rawURL}
	}

	return exec.Command(cmd, args...).Start()
}

func isDarwin() bool  { return runtime.GOOS == "darwin" }
func isWindows() bool { return runtime.GOOS == "windows" }

func detectDevScript() string {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return "dev"
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "dev"
	}

	return preferredDevScript(pkg.Scripts)
}

func preferredDevScript(scripts map[string]string) string {
	for _, script := range []string{"dev:vite", "vite", "dev"} {
		if _, ok := scripts[script]; ok {
			return script
		}
	}

	return "dev"
}

func buildLocalScriptURL(host string, port int, entry string) string {
	host = displayHost(host)
	entry = strings.TrimPrefix(entry, "/")
	if entry == "" {
		entry = "src/global/index.ts"
	}
	return fmt.Sprintf("http://%s:%d/%s", host, port, entry)
}

func buildViteClientURL(scriptURL string) string {
	parsed, err := url.Parse(scriptURL)
	if err != nil {
		return strings.TrimRight(scriptURL, "/") + "/@vite/client"
	}
	parsed.Path = "/@vite/client"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func waitForURL(ctx context.Context, rawURL string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &http.Client{Timeout: 750 * time.Millisecond}
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	try := func() error {
		req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	for {
		if err := try(); err == nil {
			return nil
		}

		select {
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()
		case <-ticker.C:
		}
	}
}

func waitForProcessURL(ctx context.Context, rawURL string, timeout time.Duration, doneCh <-chan error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	client := &http.Client{Timeout: 750 * time.Millisecond}

	for {
		req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, rawURL, nil)
		if err == nil {
			resp, reqErr := client.Do(req)
			if reqErr == nil {
				resp.Body.Close()
				return nil
			}
		}

		select {
		case err := <-doneCh:
			if normalized := normalizeProcessExit(err); normalized != nil {
				return fmt.Errorf("process exited before becoming ready: %w", normalized)
			}
			return fmt.Errorf("process exited before becoming ready")
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()
		case <-ticker.C:
		}
	}
}

func findAvailablePort(host string, preferred int) (int, bool, error) {
	if preferred <= 0 {
		preferred = 0
	}

	if listener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", preferred))); err == nil {
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		return port, port != preferred, nil
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, false, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port, true, nil
}

func normalizeProcessExit(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return nil
	}
	return err
}

func buildDevCommand(ctx context.Context, packageManager, devScript, listenHost string, port int) *exec.Cmd {
	args := []string{"run", devScript}
	extra := []string{}
	if listenHost != "" {
		extra = append(extra, "--host", listenHost)
	}
	if port > 0 {
		extra = append(extra, "--port", fmt.Sprintf("%d", port))
	}

	switch packageManager {
	case "yarn":
		args = append(args, extra...)
	default:
		if len(extra) > 0 {
			args = append(args, "--")
			args = append(args, extra...)
		}
	}

	return exec.CommandContext(ctx, packageManager, args...)
}

func resolveListenHost(publicHost string) string {
	host := strings.TrimSpace(publicHost)
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		if host == "" {
			return "localhost"
		}
		return host
	case "0.0.0.0", "::":
		return host
	default:
		return "0.0.0.0"
	}
}

func displayHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::":
		return "localhost"
	default:
		return host
	}
}
