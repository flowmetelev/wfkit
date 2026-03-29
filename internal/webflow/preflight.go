package webflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type PublishPreflight struct {
	Permissions       PublishPermissions
	Domains           PublishDomains
	Credentials       PublishCredentials
	NumberOfPublishes int
}

type PublishPermissions struct {
	CanDesign           bool
	CanPublish          bool
	CanManageCustomCode bool
	CanManageDomains    bool
}

type PublishDomains struct {
	StagingDomain     PublishDomain
	ProductionDomains []PublishDomain
	BranchDomains     []PublishDomain
	Statuses          []DomainStatus
	IsStagingPrivate  bool
}

type PublishDomain struct {
	Name          string `json:"name"`
	Stage         string `json:"stage"`
	Default       bool   `json:"default"`
	HasValidSSL   bool   `json:"hasValidSSL"`
	LastPublished string `json:"lastPublished"`
}

type DomainStatus struct {
	Name    string
	Status  string
	Message string
}

type PublishCredentials struct {
	SecretCount int
}

func GetPublishPreflight(ctx context.Context, siteName, token, cookies string) (PublishPreflight, error) {
	permissions, err := getPublishPermissions(ctx, siteName, token, cookies)
	if err != nil {
		return PublishPreflight{}, err
	}

	domains, err := getPublishDomains(ctx, siteName, token, cookies)
	if err != nil {
		return PublishPreflight{}, err
	}

	credentials, err := getPublishCredentials(ctx, siteName, token, cookies)
	if err != nil {
		return PublishPreflight{}, err
	}

	publishCount, err := getPublishCount(ctx, siteName, token, cookies)
	if err != nil {
		return PublishPreflight{}, err
	}

	return PublishPreflight{
		Permissions:       permissions,
		Domains:           domains,
		Credentials:       credentials,
		NumberOfPublishes: publishCount,
	}, nil
}

func getPublishPermissions(ctx context.Context, siteName, token, cookies string) (PublishPermissions, error) {
	var payload struct {
		Site struct {
			Design  bool `json:"design"`
			Publish bool `json:"publish"`
		} `json:"site"`
		SiteCustomCode struct {
			Manage bool `json:"manage"`
		} `json:"siteCustomCode"`
		SiteCustomDomain struct {
			Manage bool `json:"manage"`
		} `json:"siteCustomDomain"`
	}

	if err := getJSON(ctx, siteAPIURL(siteName, "permissions"), cookies, token, &payload); err != nil {
		return PublishPermissions{}, fmt.Errorf("failed to load publish permissions: %w", err)
	}

	return PublishPermissions{
		CanDesign:           payload.Site.Design,
		CanPublish:          payload.Site.Publish,
		CanManageCustomCode: payload.SiteCustomCode.Manage,
		CanManageDomains:    payload.SiteCustomDomain.Manage,
	}, nil
}

func getPublishDomains(ctx context.Context, siteName, token, cookies string) (PublishDomains, error) {
	var payload struct {
		Domains       []PublishDomain `json:"domains"`
		BranchDomains []PublishDomain `json:"branchDomains"`
		Subdomain     PublishDomain   `json:"subdomain"`
		Site          struct {
			IsStagingPrivate bool `json:"isStagingPrivate"`
		} `json:"site"`
	}

	if err := getJSON(ctx, siteAPIURL(siteName, "domains"), cookies, token, &payload); err != nil {
		return PublishDomains{}, fmt.Errorf("failed to load publish domains: %w", err)
	}

	statuses, err := getDomainStatuses(ctx, siteName, token, cookies)
	if err != nil {
		return PublishDomains{}, err
	}

	return PublishDomains{
		StagingDomain:     payload.Subdomain,
		ProductionDomains: payload.Domains,
		BranchDomains:     payload.BranchDomains,
		Statuses:          statuses,
		IsStagingPrivate:  payload.Site.IsStagingPrivate,
	}, nil
}

func getDomainStatuses(ctx context.Context, siteName, token, cookies string) ([]DomainStatus, error) {
	resp, err := doGet(ctx, siteAPIURL(siteName, "domain_statuses"), cookies, token)
	if err != nil {
		return nil, fmt.Errorf("failed to load domain statuses: %w", err)
	}
	defer resp.Body.Close()

	var payload []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode domain statuses response: %w", err)
	}

	statuses := make([]DomainStatus, 0, len(payload))
	for _, item := range payload {
		statuses = append(statuses, DomainStatus{
			Name:    firstString(item, "name", "domain", "host"),
			Status:  firstString(item, "status", "state"),
			Message: firstString(item, "message", "reason", "error"),
		})
	}

	return statuses, nil
}

func getPublishCredentials(ctx context.Context, siteName, token, cookies string) (PublishCredentials, error) {
	var payload struct {
		Secrets []json.RawMessage `json:"secrets"`
	}

	if err := getJSON(ctx, siteAPIURL(siteName, "credentials"), cookies, token, &payload); err != nil {
		return PublishCredentials{}, fmt.Errorf("failed to load credentials: %w", err)
	}

	return PublishCredentials{SecretCount: len(payload.Secrets)}, nil
}

func getPublishCount(ctx context.Context, siteName, token, cookies string) (int, error) {
	var payload struct {
		NumberOfPublishes int `json:"numberOfPublishes"`
	}

	if err := getJSON(ctx, siteAPIURL(siteName, "numberOfPublishes"), cookies, token, &payload); err != nil {
		return 0, fmt.Errorf("failed to load publish history: %w", err)
	}

	return payload.NumberOfPublishes, nil
}

func getJSON(ctx context.Context, url, cookies, token string, target interface{}) error {
	resp, err := doGet(ctx, url, cookies, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode %s: %w", url, err)
	}

	return nil
}

func siteAPIURL(siteName, path string) string {
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	return fmt.Sprintf(baseApiUrl, siteName, siteName+"/"+path)
}

func firstString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			str = strings.TrimSpace(str)
			if str != "" {
				return str
			}
		}
	}

	return ""
}
