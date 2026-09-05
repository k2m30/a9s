// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	"github.com/k2m30/a9s/v3/core/iampolicy"
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

// computeOpenSearchFindings classifies a DomainStatus against every signal the
// spec names. DescribeDomains is the fetcher's own call and DomainStatus
// carries all of them, so there is no second pass reading these same fields
// back out of Fields: one slice holds every finding and domain.StatusPhrase
// counts each once.
func computeOpenSearchFindings(d opensearchtypes.DomainStatus, now time.Time) []domainpkg.Finding {
	var findings []domainpkg.Finding
	if d.Deleted != nil && *d.Deleted {
		// A domain being torn down has no actionable posture left; the
		// background signals would only add issue-severity noise to a row
		// that is on its way out.
		return []domainpkg.Finding{{Code: CodeOpenSearchDeleting, Phrase: "deleting: removal in progress", Severity: domainpkg.SevDim, Source: "wave1"}}
	}
	if d.DomainProcessingStatus == opensearchtypes.DomainProcessingStatusTypeIsolated {
		findings = append(findings, domainpkg.Finding{Code: CodeOpenSearchIsolated, Phrase: "isolated: quarantined by AWS", Severity: domainpkg.SevBroken, Source: "wave1"})
	}
	if (d.Processing != nil && *d.Processing) || (d.UpgradeProcessing != nil && *d.UpgradeProcessing) {
		findings = append(findings, domainpkg.Finding{Code: CodeOpenSearchProcessing, Phrase: "processing: config change in flight", Severity: domainpkg.SevWarn, Source: "wave1"})
	}
	if openSearchUpdateForcedSoon(d, now) {
		findings = append(findings, domainpkg.Finding{
			Code: opensearchCodeUpdateForced, Phrase: "software update forced soon",
			Detail: opensearchUpdateForcedDetail, Severity: domainpkg.SevWarn, Source: "wave1",
		})
	}
	if d.EncryptionAtRestOptions != nil && d.EncryptionAtRestOptions.Enabled != nil && !*d.EncryptionAtRestOptions.Enabled {
		findings = append(findings, domainpkg.Finding{
			Code: opensearchCodeEncryptionOff, Phrase: "encryption at rest off",
			Detail: opensearchEncryptionOffDetail, Severity: domainpkg.SevWarn, Source: "wave1",
		})
	}
	// Reachable outside a VPC only counts when the access policy also lets
	// anyone in: a public endpoint fronted by a scoped policy is a deliberate,
	// defended design.
	if d.VPCOptions == nil && openSearchPolicyIsPublic(d) {
		findings = append(findings, domainpkg.Finding{
			Code: opensearchCodePublic, Phrase: "reachable outside a VPC",
			Detail: opensearchPublicDetail, Severity: domainpkg.SevBroken, Source: "wave1",
		})
	}
	if d.DomainEndpointOptions == nil || !aws.ToBool(d.DomainEndpointOptions.EnforceHTTPS) {
		findings = append(findings, domainpkg.Finding{
			Code: opensearchCodeHTTPSNotForced, Phrase: "HTTPS not enforced",
			Detail: opensearchHTTPSNotForcedDetail, Severity: domainpkg.SevWarn, Source: "wave1",
		})
	}
	if d.NodeToNodeEncryptionOptions == nil || !aws.ToBool(d.NodeToNodeEncryptionOptions.Enabled) {
		findings = append(findings, domainpkg.Finding{
			Code: opensearchCodeN2NOff, Phrase: "node-to-node encryption off",
			Detail: opensearchN2NOffDetail, Severity: domainpkg.SevWarn, Source: "wave1",
		})
	}
	return findings
}

// openSearchPolicyIsPublic reports whether the domain's access policy lets any
// principal in. The fetcher also writes the verdict to Fields for the list
// column; both read it from here so the two can never disagree.
func openSearchPolicyIsPublic(d opensearchtypes.DomainStatus) bool {
	doc, err := iampolicy.Parse(aws.ToString(d.AccessPolicies))
	return err == nil && iampolicy.Evaluate(doc, "").Public
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

			// VPC placement and the access policy verdict are read here, at
			// the one point that holds the DomainStatus, and handed to the
			// wave-2 enricher as Fields — the enricher makes no AWS calls of
			// its own and must not re-derive them from RawStruct.
			vpcEnabled := strconv.FormatBool(domain.VPCOptions != nil)
			accessPolicyPublic := strconv.FormatBool(openSearchPolicyIsPublic(domain))
			enforceHTTPS := "true"
			if domain.DomainEndpointOptions == nil || !aws.ToBool(domain.DomainEndpointOptions.EnforceHTTPS) {
				enforceHTTPS = "false"
			}
			nodeToNode := "true"
			if domain.NodeToNodeEncryptionOptions == nil || !aws.ToBool(domain.NodeToNodeEncryptionOptions.Enabled) {
				nodeToNode = "false"
			}

			findings := computeOpenSearchFindings(domain, now)
			statusPhrase := domainpkg.StatusPhrase(findings)

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
					"vpc_enabled":                       vpcEnabled,
					"access_policy_public":              accessPolicyPublic,
					"enforce_https":                     enforceHTTPS,
					"node_to_node_encryption_enabled":   nodeToNode,
				},
				RawStruct: domain,
			}

			if updateAvailable == "true" {
				var rows []domainpkg.DetailRow
				if updateDate != "" {
					rows = append(rows, domainpkg.DetailRow{Label: "Automated Update", Value: updateDate, Tier: "~"})
				}
				if currentVersion != "" {
					rows = append(rows, domainpkg.DetailRow{Label: "Current Version", Value: currentVersion})
				}
				if newVersion != "" {
					rows = append(rows, domainpkg.DetailRow{Label: "New Version", Value: newVersion})
				}
				addWave1Rows(&r, opensearchCodeUpdateForced, rows...)
			}

			if vpcEnabled == "false" && accessPolicyPublic == "true" {
				addWave1Rows(&r, opensearchCodePublic,
					domainpkg.DetailRow{Label: "Endpoint", Value: "public", Tier: "!"},
					domainpkg.DetailRow{Label: "Access policy", Value: "open", Tier: "!"},
				)
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
