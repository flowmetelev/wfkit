package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	projectConfigFile  = "wfkit.json"
	defaultPkgManager  = "bun"
	defaultBuildDir    = "dist/assets"
	defaultAssetBranch = "wfkit-dist"
	defaultDelivery    = "cdn"
	defaultDevHost     = "localhost"
	defaultDevPort     = 5173
	defaultProxyHost   = "localhost"
	defaultProxyPort   = 3000
	defaultEntryFile   = "src/global/index.ts"
	defaultDocsEntry   = "docs/index.md"
	defaultDocsSlug    = "docs"
)

type Config struct {
	AppName        string
	SiteURL        string
	GitHubUser     string
	RepositoryName string
	PackageManager string
	BuildDir       string
	AssetBranch    string
	DeliveryMode   string
	DevHost        string
	DevPort        int
	ProxyHost      string
	ProxyPort      int
	OpenBrowser    bool
	GlobalEntry    string
	DocsEntry      string
	DocsPageSlug   string
}

type projectFileConfig struct {
	AppName        string `json:"appName"`
	SiteURL        string `json:"siteUrl"`
	GitHubUser     string `json:"ghUserName"`
	RepositoryName string `json:"repositoryName"`
	PackageManager string `json:"packageManager"`
	BuildDir       string `json:"buildDir"`
	AssetBranch    string `json:"assetBranch"`
	DeliveryMode   string `json:"deliveryMode"`
	Branch         string `json:"branch"`
	DevHost        string `json:"devHost"`
	DevPort        int    `json:"devPort"`
	ProxyHost      string `json:"proxyHost"`
	ProxyPort      int    `json:"proxyPort"`
	OpenBrowser    *bool  `json:"openBrowser"`
	GlobalEntry    string `json:"globalEntry"`
	DocsEntry      string `json:"docsEntry"`
	DocsPageSlug   string `json:"docsPageSlug"`
}

type packageJSON struct {
	Name           string `json:"name"`
	PackageManager string `json:"packageManager"`
	Config         struct {
		AppName        string `json:"appName"`
		SiteURL        string `json:"siteUrl"`
		GitHubUser     string `json:"ghUserName"`
		RepositoryName string `json:"repositoryName"`
		PackageManager string `json:"packageManager"`
		BuildDir       string `json:"buildDir"`
		AssetBranch    string `json:"assetBranch"`
		DeliveryMode   string `json:"deliveryMode"`
		Branch         string `json:"branch"`
		DevHost        string `json:"devHost"`
		DevPort        int    `json:"devPort"`
		ProxyHost      string `json:"proxyHost"`
		ProxyPort      int    `json:"proxyPort"`
		GlobalEntry    string `json:"globalEntry"`
		DocsEntry      string `json:"docsEntry"`
		DocsPageSlug   string `json:"docsPageSlug"`
	} `json:"config"`
}

func ReadConfig() (Config, error) {
	cfg := defaultConfig()

	pkg, err := readPackageJSON()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if err == nil {
		mergePackageConfig(&cfg, pkg)
	}

	fileCfg, err := readProjectConfigFile()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if err == nil {
		mergeProjectFileConfig(&cfg, fileCfg)
	}

	cfg.normalize()
	return cfg, nil
}

func (c Config) EffectiveSiteURL() string {
	if c.SiteURL != "" {
		return c.SiteURL
	}
	if c.AppName == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.webflow.io", c.AppName)
}

func (c Config) EffectiveDesignURL() string {
	if c.AppName == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.design.webflow.com", c.AppName)
}

func (c Config) ValidateProxy() error {
	if c.EffectiveSiteURL() == "" {
		return fmt.Errorf("missing site configuration: set appName or siteUrl in %s", projectConfigFile)
	}
	if c.PackageManager == "" {
		return errors.New("missing package manager configuration")
	}
	return nil
}

func (c Config) ValidatePublish() error {
	if c.AppName == "" {
		return fmt.Errorf("missing appName configuration in %s", projectConfigFile)
	}
	if c.GitHubUser == "" {
		return fmt.Errorf("missing ghUserName configuration in %s", projectConfigFile)
	}
	if c.RepositoryName == "" {
		return fmt.Errorf("missing repositoryName configuration in %s", projectConfigFile)
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		PackageManager: defaultPkgManager,
		BuildDir:       defaultBuildDir,
		AssetBranch:    defaultAssetBranch,
		DeliveryMode:   defaultDelivery,
		DevHost:        defaultDevHost,
		DevPort:        defaultDevPort,
		ProxyHost:      defaultProxyHost,
		ProxyPort:      defaultProxyPort,
		OpenBrowser:    true,
		GlobalEntry:    defaultEntryFile,
		DocsEntry:      defaultDocsEntry,
		DocsPageSlug:   defaultDocsSlug,
	}
}

func readPackageJSON() (packageJSON, error) {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return packageJSON{}, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return packageJSON{}, fmt.Errorf("failed to parse package.json: %w", err)
	}

	return pkg, nil
}

func readProjectConfigFile() (projectFileConfig, error) {
	data, err := os.ReadFile(projectConfigFile)
	if err != nil {
		return projectFileConfig{}, err
	}

	var cfg projectFileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return projectFileConfig{}, fmt.Errorf("failed to parse %s: %w", projectConfigFile, err)
	}

	return cfg, nil
}

func mergePackageConfig(cfg *Config, pkg packageJSON) {
	if pkg.Name != "" {
		cfg.AppName = pkg.Name
	}
	if pkg.PackageManager != "" {
		cfg.PackageManager = normalizePackageManager(pkg.PackageManager)
	}

	legacy := pkg.Config
	if legacy.AppName != "" {
		cfg.AppName = legacy.AppName
	}
	if legacy.SiteURL != "" {
		cfg.SiteURL = legacy.SiteURL
	}
	if legacy.GitHubUser != "" {
		cfg.GitHubUser = legacy.GitHubUser
	}
	if legacy.RepositoryName != "" {
		cfg.RepositoryName = legacy.RepositoryName
	}
	if legacy.PackageManager != "" {
		cfg.PackageManager = normalizePackageManager(legacy.PackageManager)
	}
	if legacy.BuildDir != "" {
		cfg.BuildDir = legacy.BuildDir
	}
	if legacy.Branch != "" {
		cfg.AssetBranch = legacy.Branch
	}
	if legacy.AssetBranch != "" {
		cfg.AssetBranch = legacy.AssetBranch
	}
	if legacy.DeliveryMode != "" {
		cfg.DeliveryMode = strings.ToLower(strings.TrimSpace(legacy.DeliveryMode))
	}
	if legacy.DevHost != "" {
		cfg.DevHost = legacy.DevHost
	}
	if legacy.DevPort > 0 {
		cfg.DevPort = legacy.DevPort
	}
	if legacy.ProxyHost != "" {
		cfg.ProxyHost = legacy.ProxyHost
	}
	if legacy.ProxyPort > 0 {
		cfg.ProxyPort = legacy.ProxyPort
	}
	if legacy.GlobalEntry != "" {
		cfg.GlobalEntry = legacy.GlobalEntry
	}
	if legacy.DocsEntry != "" {
		cfg.DocsEntry = legacy.DocsEntry
	}
	if legacy.DocsPageSlug != "" {
		cfg.DocsPageSlug = legacy.DocsPageSlug
	}
}

func mergeProjectFileConfig(cfg *Config, fileCfg projectFileConfig) {
	if fileCfg.AppName != "" {
		cfg.AppName = fileCfg.AppName
	}
	if fileCfg.SiteURL != "" {
		cfg.SiteURL = fileCfg.SiteURL
	}
	if fileCfg.GitHubUser != "" {
		cfg.GitHubUser = fileCfg.GitHubUser
	}
	if fileCfg.RepositoryName != "" {
		cfg.RepositoryName = fileCfg.RepositoryName
	}
	if fileCfg.PackageManager != "" {
		cfg.PackageManager = normalizePackageManager(fileCfg.PackageManager)
	}
	if fileCfg.BuildDir != "" {
		cfg.BuildDir = fileCfg.BuildDir
	}
	if fileCfg.Branch != "" {
		cfg.AssetBranch = fileCfg.Branch
	}
	if fileCfg.AssetBranch != "" {
		cfg.AssetBranch = fileCfg.AssetBranch
	}
	if fileCfg.DeliveryMode != "" {
		cfg.DeliveryMode = strings.ToLower(strings.TrimSpace(fileCfg.DeliveryMode))
	}
	if fileCfg.DevHost != "" {
		cfg.DevHost = fileCfg.DevHost
	}
	if fileCfg.DevPort > 0 {
		cfg.DevPort = fileCfg.DevPort
	}
	if fileCfg.ProxyHost != "" {
		cfg.ProxyHost = fileCfg.ProxyHost
	}
	if fileCfg.ProxyPort > 0 {
		cfg.ProxyPort = fileCfg.ProxyPort
	}
	if fileCfg.OpenBrowser != nil {
		cfg.OpenBrowser = *fileCfg.OpenBrowser
	}
	if fileCfg.GlobalEntry != "" {
		cfg.GlobalEntry = fileCfg.GlobalEntry
	}
	if fileCfg.DocsEntry != "" {
		cfg.DocsEntry = fileCfg.DocsEntry
	}
	if fileCfg.DocsPageSlug != "" {
		cfg.DocsPageSlug = fileCfg.DocsPageSlug
	}
}

func normalizePackageManager(value string) string {
	if value == "" {
		return ""
	}
	base, _, _ := strings.Cut(value, "@")
	if base == "" {
		return value
	}
	return base
}

func (c *Config) normalize() {
	if c.PackageManager == "" {
		c.PackageManager = defaultPkgManager
	}
	if c.BuildDir == "" {
		c.BuildDir = defaultBuildDir
	}
	if c.AssetBranch == "" {
		c.AssetBranch = defaultAssetBranch
	}
	if c.DeliveryMode != "inline" {
		c.DeliveryMode = defaultDelivery
	}
	if c.DevHost == "" {
		c.DevHost = defaultDevHost
	}
	if c.DevPort <= 0 {
		c.DevPort = defaultDevPort
	}
	if c.ProxyHost == "" {
		c.ProxyHost = defaultProxyHost
	}
	if c.ProxyPort <= 0 {
		c.ProxyPort = defaultProxyPort
	}
	if c.GlobalEntry == "" {
		c.GlobalEntry = defaultEntryFile
	}
	if c.DocsEntry == "" {
		c.DocsEntry = defaultDocsEntry
	}
	if c.DocsPageSlug == "" {
		c.DocsPageSlug = defaultDocsSlug
	}
	if c.SiteURL == "" && c.AppName == "" {
		return
	}
	if c.AppName == "" && c.SiteURL != "" {
		c.AppName = appNameFromURL(c.SiteURL)
	}
}

func appNameFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	host := strings.Split(rawURL, "/")[0]
	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
