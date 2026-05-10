package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/internal/domain"
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

// computeOpenSearchFindings classifies a DomainStatus against the 5 spec signals
// and returns ordered Wave-1 findings. now is injected so tests can use a fixed instant.
//
// Signal precedence (top-first):
//  1. Deleted → "deleting: removal in progress" (Dim / hard-state)
//  2. DomainProcessingStatus=="Isolated" → "isolated: quarantined by AWS" (Broken / hard-state)
//  3. Processing || UpgradeProcessing → "processing: config change in flight" (Warning / hard-state)
//  4. UpdateAvailable && AutomatedUpdateDate in the past → "software update forced soon" (Warning / background)
//  5. EncryptionAtRestOptions.Enabled==false → "encryption at rest off" (Warning / background)
func computeOpenSearchFindings(dom opensearchtypes.DomainStatus, now time.Time) []domain.Finding {
	// Classify each signal.
	isDeleted := dom.Deleted != nil && *dom.Deleted
	isIsolated := dom.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated
	isProcessing := (dom.Processing != nil && *dom.Processing) ||
		(dom.UpgradeProcessing != nil && *dom.UpgradeProcessing)

	isUpdateForcedSoon := dom.ServiceSoftwareOptions != nil &&
		dom.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*dom.ServiceSoftwareOptions.UpdateAvailable &&
		dom.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
		dom.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)

	isEncOff := dom.EncryptionAtRestOptions != nil &&
		dom.EncryptionAtRestOptions.Enabled != nil &&
		!*dom.EncryptionAtRestOptions.Enabled

	var findings []domain.Finding

	if isDeleted {
		findings = append(findings, domain.Finding{Code: CodeOpenSearchDeleting, Phrase: "deleting: removal in progress", Severity: domain.SevDim, Source: "wave1"})
	}
	if isIsolated {
		findings = append(findings, domain.Finding{Code: CodeOpenSearchIsolated, Phrase: "isolated: quarantined by AWS", Severity: domain.SevBroken, Source: "wave1"})
	}
	if isProcessing {
		findings = append(findings, domain.Finding{Code: CodeOpenSearchProcessing, Phrase: "processing: config change in flight", Severity: domain.SevWarn, Source: "wave1"})
	}
	// Background-check signals (UpdateForcedSoon, EncOff) are owned by the
	// EnrichOpenSearchDomains Wave 2 enricher — emitting them as wave1 here
	// would double-render in the detail view and incorrectly drive Color.
	// They appear in Fields["status"] as display phrases via the suffix below.
	_ = isUpdateForcedSoon
	_ = isEncOff

	return findings
}

// computeOpenSearchDisplayPhrases mirrors computeOpenSearchFindings but returns
// the full ordered phrase list including background-check signals (used to
// build Fields["status"] with (+N) suffix).
func computeOpenSearchDisplayPhrases(dom opensearchtypes.DomainStatus, now time.Time) []string {
	isDeleted := dom.Deleted != nil && *dom.Deleted
	isIsolated := dom.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated
	isProcessing := (dom.Processing != nil && *dom.Processing) ||
		(dom.UpgradeProcessing != nil && *dom.UpgradeProcessing)
	isUpdateForcedSoon := dom.ServiceSoftwareOptions != nil &&
		dom.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*dom.ServiceSoftwareOptions.UpdateAvailable &&
		dom.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
		dom.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)
	isEncOff := dom.EncryptionAtRestOptions != nil &&
		dom.EncryptionAtRestOptions.Enabled != nil &&
		!*dom.EncryptionAtRestOptions.Enabled

	var phrases []string
	if isDeleted {
		phrases = append(phrases, "deleting: removal in progress")
	}
	if isIsolated {
		phrases = append(phrases, "isolated: quarantined by AWS")
	}
	if isProcessing {
		phrases = append(phrases, "processing: config change in flight")
	}
	if isUpdateForcedSoon {
		phrases = append(phrases, "software update forced soon")
	}
	if isEncOff {
		phrases = append(phrases, "encryption at rest off")
	}
	return phrases
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

	for _, dom := range descOutput.DomainStatusList {
		domainName := ""
		if dom.DomainName != nil {
			domainName = *dom.DomainName
		}

		engineVersion := ""
		if dom.EngineVersion != nil {
			engineVersion = *dom.EngineVersion
		}

		endpoint := ""
		if dom.Endpoint != nil {
			endpoint = *dom.Endpoint
		}

		instanceType := ""
		instanceCount := ""
		if dom.ClusterConfig != nil {
			instanceType = string(dom.ClusterConfig.InstanceType)
			if dom.ClusterConfig.InstanceCount != nil {
				instanceCount = fmt.Sprintf("%d", *dom.ClusterConfig.InstanceCount)
			}
		}

		// --- Signal flags ---
		deleted := "false"
		if dom.Deleted != nil && *dom.Deleted {
			deleted = "true"
		}

		processing := "false"
		if dom.Processing != nil && *dom.Processing {
			processing = "true"
		}

		upgradeProcessing := "false"
		if dom.UpgradeProcessing != nil && *dom.UpgradeProcessing {
			upgradeProcessing = "true"
		}

		// DomainProcessingStatus: always emit at least "Active" so the Color func's
		// Isolated branch is deterministic even when the AWS field is zero-value.
		processingStatus := "Active"
		if dom.DomainProcessingStatus != "" {
			processingStatus = string(dom.DomainProcessingStatus)
		}

		// Software update forced soon: UpdateAvailable AND AutomatedUpdateDate in the past.
		updateAvailable := "false"
		updateDate := ""
		currentVersion := ""
		newVersion := ""
		if dom.ServiceSoftwareOptions != nil {
			sso := dom.ServiceSoftwareOptions
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
		if dom.EncryptionAtRestOptions != nil &&
			dom.EncryptionAtRestOptions.Enabled != nil &&
			!*dom.EncryptionAtRestOptions.Enabled {
			encEnabled = "false"
		}

		findings := computeOpenSearchFindings(dom, now)
		// Display phrase covers background-check signals (UpdateForcedSoon,
		// EncOff) that are deliberately not in Findings — those are Wave 2
		// territory but still surface in the Status column for visibility.
		displayPhrases := computeOpenSearchDisplayPhrases(dom, now)
		statusPhrase := ""
		if len(displayPhrases) > 0 {
			statusPhrase = displayPhrases[0]
			if len(displayPhrases) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(displayPhrases)-1)
			}
		}

		r := resource.Resource{
			ID:   domainName,
			Name: domainName,
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
			Findings:  findings,
			RawStruct: dom,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
