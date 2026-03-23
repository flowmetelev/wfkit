package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRewriteHTMLStripsManagedScriptsAndInjectsVite(t *testing.T) {
	input := `<!doctype html><html><head><script data-script-id="wf-loader">x</script></head><body><script data-script-id="global-script" src="https://cdn.example/app.js"></script><main>ok</main></body></html>`

	got := rewriteHTML(input, responseOptions{
		proxyOrigin:  "http://localhost:3000",
		scriptURL:    "http://localhost:5173/src/main.ts",
		targetOrigin: "https://site.webflow.io",
	})

	if strings.Contains(got, "wf-loader") {
		t.Fatalf("expected wf-loader to be removed: %s", got)
	}
	if strings.Contains(got, "cdn.example/app.js") {
		t.Fatalf("expected existing managed script to be removed: %s", got)
	}
	if !strings.Contains(got, `src="http://localhost:5173/@vite/client"`) {
		t.Fatalf("expected vite client injection: %s", got)
	}
	if !strings.Contains(got, `data-script-id="global-script"`) {
		t.Fatalf("expected injected global-script: %s", got)
	}
}

func TestBuildViteClientURL(t *testing.T) {
	got := buildViteClientURL("http://localhost:5173/src/main.ts")
	if got != "http://localhost:5173/@vite/client" {
		t.Fatalf("unexpected vite client URL: %s", got)
	}
}

func TestRewriteSetCookiesDropsRemoteDomainForLocalProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:3000", nil)
	req.Header.Set("X-WF-Proxy-Host", "localhost:3000")
	req.Header.Set("X-WF-Proxy-Scheme", "http")

	resp := &http.Response{
		Header: http.Header{
			"Set-Cookie": []string{
				"wf-test=1; Path=/; Domain=.webflow.io; Secure; HttpOnly",
			},
		},
		Request: req,
	}

	rewriteSetCookies(resp)

	got := resp.Header.Values("Set-Cookie")
	if len(got) != 1 {
		t.Fatalf("expected one Set-Cookie header, got %d", len(got))
	}
	if strings.Contains(strings.ToLower(got[0]), "domain=") {
		t.Fatalf("expected domain to be removed: %s", got[0])
	}
	if strings.Contains(strings.ToLower(got[0]), "secure") {
		t.Fatalf("expected secure to be removed for localhost http proxy: %s", got[0])
	}
}

func TestServeRewritesHTMLAndHeadersEndToEnd(t *testing.T) {
	var targetURLString string
	target := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("Content-Security-Policy", "default-src 'self'")
		http.SetCookie(rw, &http.Cookie{
			Name:     "wf-test",
			Value:    "1",
			Path:     "/",
			Domain:   ".webflow.io",
			Secure:   true,
			HttpOnly: true,
		})
		_, _ = rw.Write([]byte(`<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'self'"></head><body><a href="` + targetURLString + `/pricing">Pricing</a><script data-script-id="global-script" src="https://cdn.example/app.js"></script></body></html>`))
	}))
	defer target.Close()
	targetURLString = target.URL

	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	proxyPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{
			ListenHost: "127.0.0.1",
			ListenPort: proxyPort,
			ScriptURL:  "http://localhost:5173/src/main.ts",
			TargetURL:  target.URL,
		})
	}()

	proxyURL := "http://127.0.0.1:" + strconv.Itoa(proxyPort)
	waitForTestURL(t, proxyURL)

	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("get proxy response: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read proxy body: %v", err)
	}
	bodyText := string(body)

	if strings.Contains(bodyText, "cdn.example/app.js") {
		t.Fatalf("expected managed script to be removed: %s", bodyText)
	}
	if strings.Contains(bodyText, "Content-Security-Policy") {
		t.Fatalf("expected CSP meta tag to be removed: %s", bodyText)
	}
	if !strings.Contains(bodyText, `http://localhost:5173/@vite/client`) {
		t.Fatalf("expected vite client injection: %s", bodyText)
	}
	if !strings.Contains(bodyText, `http://localhost:5173/src/main.ts`) {
		t.Fatalf("expected local entry injection: %s", bodyText)
	}
	if strings.Contains(bodyText, targetURL.Host) {
		t.Fatalf("expected target host references to be rewritten: %s", bodyText)
	}
	if resp.Header.Get("Content-Security-Policy") != "" {
		t.Fatalf("expected CSP header to be removed")
	}

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) != 1 {
		t.Fatalf("expected one set-cookie header, got %d", len(cookies))
	}
	if strings.Contains(strings.ToLower(cookies[0]), "domain=") {
		t.Fatalf("expected rewritten cookie to be host-only: %s", cookies[0])
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("proxy serve returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy serve did not shut down after cancel")
	}
}

func waitForTestURL(t *testing.T, rawURL string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(rawURL)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", rawURL)
}
