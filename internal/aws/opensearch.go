package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

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

// computeOpenSearchStatusAndIssues classifies a DomainStatus against the 5 spec signals
// and returns the top Status phrase (with optional (+N) suffix) and the Issues slice
// (hard-state phrases only). now is injected so tests can use a fixed instant.
//
// Signal precedence (top-first):
//  1. Deleted → "deleting: removal in progress" (Dim / hard-state)
//  2. DomainProcessingStatus=="Isolated" → "isolated: quarantined by AWS" (Broken / hard-state)
//  3. Processing || UpgradeProcessing → "processing: config change in flight" (Warning / hard-state)
//  4. UpdateAvailable && AutomatedUpdateDate in the past → "software update forced soon" (! background)
//  5. EncryptionAtRestOptions.Enabled==false → "encryption at rest off" (~ background)
//
// Hard-state phrases go into Issues; background-check phrases do NOT.
// topPhrase is "" when all signals are absent (Healthy silence — rule 1).
func computeOpenSearchStatusAndIssues(domain opensearchtypes.DomainStatus, now time.Time) (topPhrase string, issues []string) {
	const (
		phraseDeleting    = "deleting: removal in progress"
		phraseIsolated    = "isolated: quarantined by AWS"
		phraseProcessing  = "processing: config change in flight"
		phraseUpdateSoon  = "software update forced soon"
		phraseEncOff      = "encryption at rest off"
	)

	// Classify each signal.
	isDeleted := domain.Deleted != nil && *domain.Deleted
	isIsolated := domain.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated
	isProcessing := (domain.Processing != nil && *domain.Processing) ||
		(domain.UpgradeProcessing != nil && *domain.UpgradeProcessing)

	isUpdateForcedSoon := domain.ServiceSoftwareOptions != nil &&
		domain.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*domain.ServiceSoftwareOptions.UpdateAvailable &&
		domain.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
		domain.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)

	isEncOff := domain.EncryptionAtRestOptions != nil &&
		domain.EncryptionAtRestOptions.Enabled != nil &&
		!*domain.EncryptionAtRestOptions.Enabled

	// Build ordered phrase list (precedence order).
	var phrases []string
	var hardPhrases []string

	if isDeleted {
		phrases = append(phrases, phraseDeleting)
		hardPhrases = append(hardPhrases, phraseDeleting)
	}
	if isIsolated {
		phrases = append(phrases, phraseIsolated)
		hardPhrases = append(hardPhrases, phraseIsolated)
	}
	if isProcessing {
		phrases = append(phrases, phraseProcessing)
		hardPhrases = append(hardPhrases, phraseProcessing)
	}
	if isUpdateForcedSoon {
		phrases = append(phrases, phraseUpdateSoon)
		// background-check: NOT added to hardPhrases
	}
	if isEncOff {
		phrases = append(phrases, phraseEncOff)
		// background-check: NOT added to hardPhrases
	}

	if len(phrases) == 0 {
		return "", nil
	}

	top := phrases[0]
	hidden := len(phrases) - 1
	if hidden > 0 {
		top = fmt.Sprintf("%s (+%d)", top, hidden)
	}

	// Issues carries only hard-state phrases (no suffix — raw phrase per signal).
	var issueList []string
	if len(hardPhrases) > 0 {
		issueList = hardPhrases
	}

	return top, issueList
}

// FetchOpenSearchDomains performs a two-step fetch:
// 1. ListDomainNames to get domain names
// 2. DescribeDomains to get full domain status details
func FetchOpenSearchDomains(
	ctx context.Context,
	listAPI OpenSearchListDomainNamesAPI,
	describeAPI OpenSearchDescribeDomainsAPI,
) ([]resource.Resource, error) {
	return fetchOpenSearchDomainsAt(ctx, listAPI, describeAPI, time.Now())
}

// fetchOpenSearchDomainsAt is the time-injectable implementation used by FetchOpenSearchDomains
// and directly by tests.
func fetchOpenSearchDomainsAt(
	ctx context.Context,
	listAPI OpenSearchListDomainNamesAPI,
	describeAPI OpenSearchDescribeDomainsAPI,
	now time.Time,
) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
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

	descOutput, err := describeAPI.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{
		DomainNames: domainNames,
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

		// Classify status and issues using the shared classifier (injectable now).
		statusPhrase, issues := computeOpenSearchStatusAndIssues(domain, now)

		r := resource.Resource{
			ID:     domainName,
			Name:   domainName,
			Status: statusPhrase,
			Issues: issues,
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
				"automated_update_date":                       updateDate,
				"current_version":                   currentVersion,
				"new_version":                       newVersion,
			},
			RawStruct: domain,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
