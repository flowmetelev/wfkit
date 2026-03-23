package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const defaultCacheTTL = 6 * time.Hour

type UpdateManager struct {
	repoOwner string
	repoName  string
	version   string
	baseURL   string
	client    *http.Client
	cachePath string
	cacheTTL  time.Duration
	now       func() time.Time
}

type CheckOptions struct {
	Force      bool
	AllowStale bool
}

type CheckResult struct {
	Available     bool
	LatestVersion string
	Cached        bool
	Stale         bool
	CheckedAt     time.Time
}

type cacheEntry struct {
	LatestVersion string    `json:"latestVersion"`
	CheckedAt     time.Time `json:"checkedAt"`
	ETag          string    `json:"etag,omitempty"`
	LastModified  string    `json:"lastModified,omitempty"`
}

func NewUpdateManager(version string) *UpdateManager {
	return &UpdateManager{
		repoOwner: "yndmitry",
		repoName:  "wfkit",
		version:   version,
		baseURL:   "https://api.github.com",
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		cachePath: defaultCachePath(),
		cacheTTL:  defaultCacheTTL,
		now:       time.Now,
	}
}

func formatVersion(v string) string {
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func (um *UpdateManager) CheckUpdateAvailable() (bool, string, error) {
	result, err := um.Check(CheckOptions{AllowStale: true})
	if err != nil {
		return false, "", err
	}
	return result.Available, result.LatestVersion, nil
}

func (um *UpdateManager) Check(options CheckOptions) (CheckResult, error) {
	cache, _ := um.loadCache()
	if cache != nil && !options.Force && um.now().Sub(cache.CheckedAt) < um.cacheTTL {
		return um.resultFromVersion(cache.LatestVersion, true, false, cache.CheckedAt)
	}

	latest, nextCache, err := um.fetchLatestVersion(cache)
	if err != nil {
		if options.AllowStale && cache != nil && cache.LatestVersion != "" {
			return um.resultFromVersion(cache.LatestVersion, true, true, cache.CheckedAt)
		}
		return CheckResult{}, err
	}

	if nextCache != nil {
		if err := um.saveCache(*nextCache); err != nil && !options.AllowStale {
			return CheckResult{}, err
		}
		usedCache := cache != nil && nextCache == cache
		return um.resultFromVersion(latest, usedCache, false, nextCache.CheckedAt)
	}

	return um.resultFromVersion(latest, false, false, um.now())
}

func (um *UpdateManager) GetCurrentVersionNumber() string {
	return um.version
}

func (um *UpdateManager) resultFromVersion(latest string, cached, stale bool, checkedAt time.Time) (CheckResult, error) {
	if latest == "" {
		return CheckResult{}, fmt.Errorf("latest version is empty")
	}

	currentSemver := formatVersion(um.version)
	latestSemver := formatVersion(latest)
	available := false

	switch {
	case semver.IsValid(currentSemver) && semver.IsValid(latestSemver):
		available = semver.Compare(currentSemver, latestSemver) < 0
	default:
		available = currentSemver != latestSemver
	}

	return CheckResult{
		Available:     available,
		LatestVersion: latest,
		Cached:        cached,
		Stale:         stale,
		CheckedAt:     checkedAt,
	}, nil
}

func (um *UpdateManager) fetchLatestVersion(cache *cacheEntry) (string, *cacheEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(um.baseURL, "/"), um.repoOwner, um.repoName)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "wfkit-cli")
	if cache != nil {
		if cache.ETag != "" {
			req.Header.Set("If-None-Match", cache.ETag)
		}
		if cache.LastModified != "" {
			req.Header.Set("If-Modified-Since", cache.LastModified)
		}
	}

	resp, err := um.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if cache == nil {
			return "", nil, fmt.Errorf("received 304 without cached release data")
		}
		cache.CheckedAt = um.now()
		return cache.LatestVersion, cache, nil
	case http.StatusForbidden:
		return "", nil, fmt.Errorf("github api rate limit exceeded")
	case http.StatusOK:
	default:
		return "", nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", nil, fmt.Errorf("failed to parse github response: %w", err)
	}

	nextCache := &cacheEntry{
		LatestVersion: release.TagName,
		CheckedAt:     um.now(),
		ETag:          resp.Header.Get("ETag"),
		LastModified:  resp.Header.Get("Last-Modified"),
	}

	return release.TagName, nextCache, nil
}

func (um *UpdateManager) loadCache() (*cacheEntry, error) {
	if strings.TrimSpace(um.cachePath) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(um.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.LatestVersion == "" {
		return nil, nil
	}

	return &entry, nil
}

func (um *UpdateManager) saveCache(entry cacheEntry) error {
	if strings.TrimSpace(um.cachePath) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(um.cachePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(um.cachePath, data, 0o644)
}

func defaultCachePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".wfkit", "update-cache.json")
}
