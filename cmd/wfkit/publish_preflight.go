package main

import (
	"fmt"
	"strings"

	"wfkit/internal/utils"
	"wfkit/internal/webflow"
)

func printPublishReadiness(preflight webflow.PublishPreflight) {
	utils.PrintSection("Publish Readiness")
	utils.PrintStatus(statusForBool(preflight.Permissions.CanPublish && preflight.Permissions.CanManageCustomCode), "Permissions", publishPermissionsMessage(preflight))
	utils.PrintStatus(statusForBool(preflight.Domains.StagingDomain.Name != ""), "Staging", describeStagingDomain(preflight.Domains))
	utils.PrintStatus(productionDomainsStatus(preflight.Domains), "Production", describeProductionDomains(preflight.Domains))
	utils.PrintStatus(domainStatusesStatus(preflight.Domains), "Domain status", describeDomainStatuses(preflight.Domains))
	utils.PrintStatus("INFO", "Credentials", describeCredentials(preflight.Credentials))
	utils.PrintStatus("INFO", "Publish history", fmt.Sprintf("%d publish(es) recorded", preflight.NumberOfPublishes))
	fmt.Println()
}

func validatePublishReadiness(preflight webflow.PublishPreflight) error {
	if !preflight.Permissions.CanPublish {
		return fmt.Errorf("Webflow permissions do not allow site publishing")
	}
	if !preflight.Permissions.CanManageCustomCode {
		return fmt.Errorf("Webflow permissions do not allow custom code updates")
	}
	if strings.TrimSpace(preflight.Domains.StagingDomain.Name) == "" {
		return fmt.Errorf("Webflow staging domain is missing, so publish cannot proceed")
	}
	return nil
}

func doctorChecksFromPublishPreflight(preflight webflow.PublishPreflight) []doctorCheck {
	return []doctorCheck{
		{
			Category: "publish",
			Name:     "Publish permissions",
			Status:   doctorStatusFromBool(preflight.Permissions.CanPublish && preflight.Permissions.CanManageCustomCode, doctorFail),
			Message:  publishPermissionsMessage(preflight),
		},
		{
			Category: "publish",
			Name:     "Staging destination",
			Status:   doctorStatusFromBool(strings.TrimSpace(preflight.Domains.StagingDomain.Name) != "", doctorFail),
			Message:  describeStagingDomain(preflight.Domains),
		},
		{
			Category: "publish",
			Name:     "Production destinations",
			Status:   doctorStatus(productionDomainsStatus(preflight.Domains)),
			Message:  describeProductionDomains(preflight.Domains),
		},
		{
			Category: "publish",
			Name:     "Domain status",
			Status:   doctorStatus(domainStatusesStatus(preflight.Domains)),
			Message:  describeDomainStatuses(preflight.Domains),
		},
		{
			Category: "publish",
			Name:     "Credentials",
			Status:   doctorPass,
			Message:  describeCredentials(preflight.Credentials),
		},
		{
			Category: "publish",
			Name:     "Publish history",
			Status:   doctorPass,
			Message:  fmt.Sprintf("%d publish(es) recorded", preflight.NumberOfPublishes),
		},
	}
}

func publishPermissionsMessage(preflight webflow.PublishPreflight) string {
	parts := make([]string, 0, 3)
	parts = append(parts, booleanLabel("publish", preflight.Permissions.CanPublish))
	parts = append(parts, booleanLabel("custom code", preflight.Permissions.CanManageCustomCode))
	parts = append(parts, booleanLabel("design", preflight.Permissions.CanDesign))
	return strings.Join(parts, " · ")
}

func describeStagingDomain(domains webflow.PublishDomains) string {
	if strings.TrimSpace(domains.StagingDomain.Name) == "" {
		return "no staging subdomain is configured"
	}

	parts := []string{domains.StagingDomain.Name}
	if domains.IsStagingPrivate {
		parts = append(parts, "private")
	} else {
		parts = append(parts, "public")
	}
	if domains.StagingDomain.LastPublished != "" {
		parts = append(parts, "published before")
	}
	return strings.Join(parts, " · ")
}

func describeProductionDomains(domains webflow.PublishDomains) string {
	if len(domains.ProductionDomains) == 0 {
		return "no custom production domains configured"
	}

	names := make([]string, 0, len(domains.ProductionDomains))
	for _, domain := range domains.ProductionDomains {
		label := domain.Name
		if label == "" {
			continue
		}
		if domain.HasValidSSL {
			label += " (SSL)"
		}
		names = append(names, label)
	}
	if len(names) == 0 {
		return fmt.Sprintf("%d production domain(s) configured", len(domains.ProductionDomains))
	}
	return strings.Join(names, ", ")
}

func describeDomainStatuses(domains webflow.PublishDomains) string {
	if len(domains.Statuses) == 0 {
		if len(domains.ProductionDomains) == 0 {
			return "no production domain statuses to check"
		}
		return "no explicit domain status issues reported"
	}

	parts := make([]string, 0, len(domains.Statuses))
	for _, status := range domains.Statuses {
		label := strings.TrimSpace(status.Name)
		if label == "" {
			label = "domain"
		}
		state := strings.TrimSpace(status.Status)
		if state == "" {
			state = "unknown"
		}
		message := strings.TrimSpace(status.Message)
		if message != "" {
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", label, state, message))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", label, state))
	}

	return strings.Join(parts, ", ")
}

func describeCredentials(credentials webflow.PublishCredentials) string {
	if credentials.SecretCount == 0 {
		return "0 secrets configured"
	}
	return fmt.Sprintf("%d secret(s) configured", credentials.SecretCount)
}

func productionDomainsStatus(domains webflow.PublishDomains) string {
	if len(domains.ProductionDomains) == 0 {
		return string(doctorWarn)
	}
	return string(doctorPass)
}

func domainStatusesStatus(domains webflow.PublishDomains) string {
	if len(domains.Statuses) == 0 {
		return string(doctorPass)
	}

	for _, status := range domains.Statuses {
		state := strings.ToLower(strings.TrimSpace(status.Status))
		message := strings.ToLower(strings.TrimSpace(status.Message))
		if strings.Contains(state, "error") || strings.Contains(state, "fail") || strings.Contains(message, "error") || strings.Contains(message, "fail") {
			return string(doctorWarn)
		}
	}

	return string(doctorPass)
}

func doctorStatusFromBool(ok bool, failStatus doctorStatus) doctorStatus {
	if ok {
		return doctorPass
	}
	return failStatus
}

func statusForBool(ok bool) string {
	if ok {
		return "OK"
	}
	return "WARN"
}

func booleanLabel(label string, ok bool) string {
	if ok {
		return fmt.Sprintf("%s ok", label)
	}
	return fmt.Sprintf("%s missing", label)
}
