package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	domainpkg "github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// openSearchUpdateForcedSoon reports whether AWS has scheduled a mandatory
// service-software update: UpdateAvailable=true AND AutomatedUpdateDate is a
// real timestamp in the past. A zero-value AutomatedUpdateDate (Go's
// time.Time{} zero, which formats as 0001-01-01, or the Unix epoch some AWS
// SDK paths substitute for "not scheduled") is NOT a past-due date — without
// this guard a domain that has never had an update scheduled reads as
// perpetually overdue. isZeroOrEpoch below rejects both zero-value forms.
func openSearchUpdateForcedSoon(d opensearchtypes.DomainStatus, now time.Time) bool {
	return d.ServiceSoftwareOptions != nil &&
		d.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*d.ServiceSoftwareOptions.UpdateAvailable &&
		d.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
		!isZeroOrEpoch(*d.ServiceSoftwareOptions.AutomatedUpdateDate) &&
		d.ServiceSoftwareOptions.AutomatedUpdateDate.Before(now)
}

// isZeroOrEpoch reports whether t is Go's zero time.Time or the Unix epoch —
// both are "no real timestamp was set" sentinels a caller might receive from
// an AWS SDK field instead of a nil pointer.
func isZeroOrEpoch(t time.Time) bool {
	return t.IsZero() || t.Unix() == 0
}

// openSearchSignals classifies a DomainStatus against the 5 spec signals.
// Returns hard-state findings (for Resource.Findings) and the total signal count
// (for computing the Fields["status"] display phrase with suffix).
// Background-check signals (UpdateForcedSoon, EncryptionOff) contribute to the
// display count but are NOT included in Findings — they are enricher territory
// (EnrichOpenSearchDomains emits each as its own Wave-2 Finding/Code so
// encryption-at-rest-off is never presented as an appendix of the unrelated
// update-forced finding; when both fire on one resource only one Wave-2
// Finding can attach per the IssueEnricherResult contract, so the enricher
// picks the worse one and surfaces the other as a supporting DetailRow).
func openSearchSignals(d opensearchtypes.DomainStatus, now time.Time) (hardFindings []domainpkg.Finding, totalCount int) {
	isDeleted := d.Deleted != nil && *d.Deleted
	isIsolated := d.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated
	isProcessing := (d.Processing != nil && *d.Processing) ||
		(d.UpgradeProcessing != nil && *d.UpgradeProcessing)
	isUpdateForcedSoon := openSearchUpdateForcedSoon(d, now)
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
		if openSearchUpdateForcedSoon(d, now) {
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

	descOutput, describeErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*opensearch.DescribeDomainsOutput, error) {
		return describeAPI.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{
			DomainNames: domainNames,
		})
	})

	var resources []resource.Resource
	described := make(map[string]bool, len(domainNames))

	if descOutput != nil {
		for _, domain := range descOutput.DomainStatusList {
			domainName := aws.ToString(domain.DomainName)
			described[domainName] = true

			engineVersion := aws.ToString(domain.EngineVersion)
			endpoint := aws.ToString(domain.Endpoint)

			instanceType := ""
			instanceCount := ""
			if domain.ClusterConfig != nil {
				instanceType = string(domain.ClusterConfig.InstanceType)
				if domain.ClusterConfig.InstanceCount != nil {
					instanceCount = fmt.Sprintf("%d", *domain.ClusterConfig.InstanceCount)
				}
			}

			// --- Signal flags ---
			deleted := strconv.FormatBool(aws.ToBool(domain.Deleted))
			processing := strconv.FormatBool(aws.ToBool(domain.Processing))
			upgradeProcessing := strconv.FormatBool(aws.ToBool(domain.UpgradeProcessing))

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
				if openSearchUpdateForcedSoon(domain, now) {
					updateAvailable = "true"
				}
				if sso.AutomatedUpdateDate != nil {
					updateDate = sso.AutomatedUpdateDate.Format(time.RFC3339)
				}
				currentVersion = aws.ToString(sso.CurrentVersion)
				newVersion = aws.ToString(sso.NewVersion)
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
	}

	// Domains the account listed but DescribeDomains did not return them for
	// — either the batch call itself failed (e.g. an es:DescribeDomains IAM
	// denial) or this domain was individually absent from its response —
	// are KEPT as name-only degraded rows; a listed domain must never
	// vanish from the list.
	var failures []string
	for _, name := range domainNames {
		if described[name] {
			continue
		}
		reason := "absent from DescribeDomains response"
		if describeErr != nil {
			reason = describeErr.Error()
		}
		failures = append(failures, fmt.Sprintf("%s: %s", name, reason))
		resources = append(resources, DegradedDetails("opensearch", name, opensearchtypes.DomainStatus{DomainName: &name}, describeErr))
	}

	return resources, AggregateFailures("opensearch: DescribeDomains", failures, len(domainNames))
}
