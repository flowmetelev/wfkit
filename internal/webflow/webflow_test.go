package webflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func TestCreateStaticPageInitializesSecrets(t *testing.T) {
	t.Helper()

	var createBody map[string]string
	var secretsBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sites/wfkit/pages":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"page":{"_id":"page-123","title":"Docs","slug":"docs"}}`)
		case r.Method == http.MethodPut && r.URL.Path == "/api/pages/page-123/secrets":
			if err := json.NewDecoder(r.Body).Decode(&secretsBody); err != nil {
				t.Fatalf("decode secrets body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"page":"page-123","pagePassword":false}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	originalClient := httpClient
	httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: rewriteTransport{target: targetURL, base: server.Client().Transport},
	}
	defer func() { httpClient = originalClient }()

	page, err := CreateStaticPage(context.Background(), "wfkit", server.URL, "token", "cookie=value", "Docs", "docs")
	if err != nil {
		t.Fatalf("CreateStaticPage returned error: %v", err)
	}

	if page.ID != "page-123" {
		t.Fatalf("expected page id page-123, got %q", page.ID)
	}
	if createBody["title"] != "Docs" || createBody["slug"] != "docs" {
		t.Fatalf("unexpected create body: %#v", createBody)
	}
	if secretsBody["head"] != "" || secretsBody["postBody"] != "" {
		t.Fatalf("expected empty secrets init body, got %#v", secretsBody)
	}
}

func TestInitializePageSecretsTrimsBaseURLSlash(t *testing.T) {
	t.Helper()

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"page":"page-123","pagePassword":false}`)
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	originalClient := httpClient
	httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: rewriteTransport{target: targetURL, base: server.Client().Transport},
	}
	defer func() { httpClient = originalClient }()

	if err := InitializePageSecrets(context.Background(), server.URL+"/", "token", "cookie=value", "page-123", "", ""); err != nil {
		t.Fatalf("InitializePageSecrets returned error: %v", err)
	}

	if requestedPath != "/api/pages/page-123/secrets" {
		t.Fatalf("expected trimmed secrets path, got %q", requestedPath)
	}
}

func TestCreateStaticPageReturnsHelpfulDecodeError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/secrets") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"page":"page-123","pagePassword":false}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"page":`)
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	originalClient := httpClient
	httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: rewriteTransport{target: targetURL, base: server.Client().Transport},
	}
	defer func() { httpClient = originalClient }()

	_, err = CreateStaticPage(context.Background(), "wfkit", server.URL, "token", "cookie=value", "Docs", "docs")
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), "failed to decode create page response") {
		t.Fatalf("unexpected error: %v", err)
	}
}
