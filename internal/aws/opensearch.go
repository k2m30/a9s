package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	domainpkg "github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("opensearch", []string{
		"domain_name", "engine_version", "instance_type", "instance_count", "endpoint",
		"status", "domain_processing_status",
		"deleted", "processing", "upgrade_processing",
		"service_software_update_available", "encryption_at_rest_enabled",
		"automated_update_date", "current_version", "new_version",
	})

	resource.RegisterPaginated("opensearch", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		resources, err := FetchOpenSearchDomains(ctx, c.OpenSearch, c.OpenSearch)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return resource.FetchResult{
			Resources:  resources,
			Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
		}, nil
	})
}

// openSearchSignals classifies a DomainStatus against the 5 spec signals.
// Returns hard-state findings (for Resource.Findings) and the total signal count
// (for computing the Fields["status"] display phrase with suffix).
// Background-check signals (UpdateForcedSoon, EncryptionOff) contribute to the
// display count but are NOT included in Findings — they are enricher territory.
func openSearchSignals(d opensearchtypes.DomainStatus, now time.Time) (hardFindings []domainpkg.Finding, totalCount int) {
	isDeleted := d.Deleted != nil && *d.Deleted
	isIsolated := d.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated
	isProcessing := (d.Processing != nil && *d.Processing) ||
		(d.UpgradeProcessing != nil && *d.UpgradeProcessing)
	isUpdateForcedSoon := d.ServiceSoftwareOptions != nil &&
		d.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*d.ServiceSoftwareOptions.UpdateAvailable &&
		d.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
		d.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)
	isEncOff := d.EncryptionAtRestOptions != nil &&
		d.EncryptionAtRestOptions.Enabled != nil &&
		!*d.EncryptionAtRestOptions.Enabled

	if isDeleted {
		hardFindings = append(hardFindings, domainpkg.Finding{Code: CodeOpenSearchDeleting, Phrase: "deleting: removal in progress", Severity: domainpkg.SevDim, Source: "wave1"})
	}
	if isIsolated {
		hardFindings = append(hardFindings, domainpkg.Finding{Code: CodeOpenSearchIsolated, Phrase: "isolated: quarantined by AWS", Severity: domainpkg.SevBroken, Source: "wave1"})
	}
	if isProcessing {
		hardFindings = append(hardFindings, domainpkg.Finding{Code: CodeOpenSearchProcessing, Phrase: "processing: config change in flight", Severity: domainpkg.SevWarn, Source: "wave1"})
	}

	totalCount = len(hardFindings)
	if isUpdateForcedSoon {
		totalCount++
	}
	if isEncOff {
		totalCount++
	}
	return hardFindings, totalCount
}

// computeOpenSearchFindings returns only the hard-state findings for a domain.
// Background-check signals are not included (enricher territory).
func computeOpenSearchFindings(d opensearchtypes.DomainStatus, now time.Time) []domainpkg.Finding {
	findings, _ := openSearchSignals(d, now)
	return findings
}

// openSearchStatusPhrase computes the display phrase for Fields["status"],
// including background-check signals in the suffix count.
func openSearchStatusPhrase(d opensearchtypes.DomainStatus, now time.Time) string {
	findings, totalCount := openSearchSignals(d, now)
	if totalCount == 0 {
		return ""
	}
	if len(findings) == 0 {
		// Only background checks active — use first background phrase
		// (this path means totalCount > 0 but no hard findings)
		// Determine which background came first
		isUpdateForcedSoon := d.ServiceSoftwareOptions != nil &&
			d.ServiceSoftwareOptions.UpdateAvailable != nil &&
			*d.ServiceSoftwareOptions.UpdateAvailable &&
			d.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
			d.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)
		if isUpdateForcedSoon {
			top := "software update forced soon"
			if totalCount > 1 {
				return fmt.Sprintf("%s (+%d)", top, totalCount-1)
			}
			return top
		}
		top := "encryption at rest off"
		return top
	}
	top := findings[0].Phrase
	if totalCount > 1 {
		return fmt.Sprintf("%s (+%d)", top, totalCount-1)
	}
	return top
}

// FetchOpenSearchDomains performs a two-step fetch:
// 1. ListDomainNames to get domain names
// 2. DescribeDomains to get full domain status details
func FetchOpenSearchDomains(
	ctx context.Context,
	listAPI OpenSearchListDomainNamesAPI,
	describeAPI OpenSearchDescribeDomainsAPI,
) ([]resource.Resource, error) {
	return FetchOpenSearchDomainsAt(ctx, listAPI, describeAPI, time.Now())
}

// FetchOpenSearchDomainsAt is the time-injectable implementation used by FetchOpenSearchDomains
// and directly by tests.
func FetchOpenSearchDomainsAt(
	ctx context.Context,
	listAPI OpenSearchListDomainNamesAPI,
	describeAPI OpenSearchDescribeDomainsAPI,
	now time.Time,
) ([]resource.Resource, error) {
	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.ListDomainNamesOutput, error) {
		return listAPI.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	})
	if err != nil {
		return nil, fmt.Errorf("listing OpenSearch domains: %w", err)
	}

	if len(listOutput.DomainNames) == 0 {
		return []resource.Resource{}, nil
	}

	// Collect domain names for the DescribeDomains call.
	domainNames := make([]string, 0, len(listOutput.DomainNames))
	for _, d := range listOutput.DomainNames {
		if d.DomainName != nil {
			domainNames = append(domainNames, *d.DomainName)
		}
	}

	descOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.DescribeDomainsOutput, error) {
		return describeAPI.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{
			DomainNames: domainNames,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("describing OpenSearch domains: %w", err)
	}

	var resources []resource.Resource

	for _, domain := range descOutput.DomainStatusList {
		domainName := ""
		if domain.DomainName != nil {
			domainName = *domain.DomainName
		}

		engineVersion := ""
		if domain.EngineVersion != nil {
			engineVersion = *domain.EngineVersion
		}

		endpoint := ""
		if domain.Endpoint != nil {
			endpoint = *domain.Endpoint
		}

		instanceType := ""
		instanceCount := ""
		if domain.ClusterConfig != nil {
			instanceType = string(domain.ClusterConfig.InstanceType)
			if domain.ClusterConfig.InstanceCount != nil {
				instanceCount = fmt.Sprintf("%d", *domain.ClusterConfig.InstanceCount)
			}
		}

		// --- Signal flags ---
		deleted := "false"
		if domain.Deleted != nil && *domain.Deleted {
			deleted = "true"
		}

		processing := "false"
		if domain.Processing != nil && *domain.Processing {
			processing = "true"
		}

		upgradeProcessing := "false"
		if domain.UpgradeProcessing != nil && *domain.UpgradeProcessing {
			upgradeProcessing = "true"
		}

		// DomainProcessingStatus: always emit at least "Active" so the Color func's
		// Isolated branch is deterministic even when the AWS field is zero-value.
		processingStatus := "Active"
		if domain.DomainProcessingStatus != "" {
			processingStatus = string(domain.DomainProcessingStatus)
		}

		// Software update forced soon: UpdateAvailable AND AutomatedUpdateDate in the past.
		updateAvailable := "false"
		updateDate := ""
		currentVersion := ""
		newVersion := ""
		if domain.ServiceSoftwareOptions != nil {
			sso := domain.ServiceSoftwareOptions
			if sso.UpdateAvailable != nil && *sso.UpdateAvailable &&
				sso.AutomatedUpdateDate != nil &&
				sso.AutomatedUpdateDate.Before(now) {
				updateAvailable = "true"
			}
			if sso.AutomatedUpdateDate != nil {
				updateDate = sso.AutomatedUpdateDate.Format(time.RFC3339)
			}
			if sso.CurrentVersion != nil {
				currentVersion = *sso.CurrentVersion
			}
			if sso.NewVersion != nil {
				newVersion = *sso.NewVersion
			}
		}

		// Encryption at rest: non-nil pointer with value false.
		encEnabled := "true"
		if domain.EncryptionAtRestOptions != nil &&
			domain.EncryptionAtRestOptions.Enabled != nil &&
			!*domain.EncryptionAtRestOptions.Enabled {
			encEnabled = "false"
		}

		findings := computeOpenSearchFindings(domain, now)
		statusPhrase := openSearchStatusPhrase(domain, now)

		r := resource.Resource{
			ID:       domainName,
			Name:     domainName,
			Findings: findings,
			Fields: map[string]string{
				"domain_name":                       domainName,
				"engine_version":                    engineVersion,
				"instance_type":                     instanceType,
				"instance_count":                    instanceCount,
				"endpoint":                          endpoint,
				"status":                            statusPhrase,
				"deleted":                           deleted,
				"processing":                        processing,
				"upgrade_processing":                upgradeProcessing,
				"domain_processing_status":          processingStatus,
				"service_software_update_available": updateAvailable,
				"encryption_at_rest_enabled":        encEnabled,
				"automated_update_date":             updateDate,
				"current_version":                   currentVersion,
				"new_version":                       newVersion,
			},
			RawStruct: domain,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
