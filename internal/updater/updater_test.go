package updater

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckUsesFreshCacheWithoutNetwork(t *testing.T) {
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0"}`))
	}))
	defer server.Close()

	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	manager := NewUpdateManager("1.0.0")
	manager.baseURL = server.URL
	manager.cachePath = filepath.Join(t.TempDir(), "update-cache.json")
	manager.now = func() time.Time { return now }

	first, err := manager.Check(CheckOptions{})
	if err != nil {
		t.Fatalf("first Check: %v", err)
	}
	if !first.Available {
		t.Fatalf("expected update to be available")
	}

	second, err := manager.Check(CheckOptions{})
	if err != nil {
		t.Fatalf("second Check: %v", err)
	}
	if !second.Cached {
		t.Fatalf("expected second result to come from cache")
	}
	if hitCount != 1 {
		t.Fatalf("expected exactly one network request, got %d", hitCount)
	}
}

func TestCheckRevalidatesCacheWithETag(t *testing.T) {
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0"}`))
	}))
	defer server.Close()

	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	manager := NewUpdateManager("1.0.0")
	manager.baseURL = server.URL
	manager.cachePath = filepath.Join(t.TempDir(), "update-cache.json")
	manager.now = func() time.Time { return now }
	manager.cacheTTL = time.Hour

	if _, err := manager.Check(CheckOptions{}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	manager.now = func() time.Time { return now.Add(2 * time.Hour) }
	result, err := manager.Check(CheckOptions{})
	if err != nil {
		t.Fatalf("revalidate cache: %v", err)
	}
	if !result.Cached {
		t.Fatalf("expected 304 revalidation to use cached version")
	}
	if hitCount != 2 {
		t.Fatalf("expected two network requests, got %d", hitCount)
	}
}

func TestCheckFallsBackToStaleCacheOnNetworkFailure(t *testing.T) {
	manager := NewUpdateManager("1.0.0")
	manager.baseURL = "http://127.0.0.1:1"
	manager.cachePath = filepath.Join(t.TempDir(), "update-cache.json")
	manager.now = func() time.Time { return time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC) }

	if err := manager.saveCache(cacheEntry{
		LatestVersion: "v1.2.0",
		CheckedAt:     manager.now().Add(-24 * time.Hour),
		ETag:          `"abc"`,
	}); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	result, err := manager.Check(CheckOptions{AllowStale: true})
	if err != nil {
		t.Fatalf("Check with stale fallback: %v", err)
	}
	if !result.Stale || !result.Available {
		t.Fatalf("expected stale cached update result, got %+v", result)
	}
}
