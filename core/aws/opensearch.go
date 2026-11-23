// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
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

// openSearchLifecycle is the domain's DomainProcessingStatus. Deleted is
// AWS's own statement that a deletion is in progress ("True if the domain is
// in the process of being deleted", DomainStatus in the OpenSearch API
// Reference), so it reads as Deleting whatever the processing status says.
func openSearchLifecycle(d opensearchtypes.DomainStatus) string {
	if aws.ToBool(d.Deleted) {
		return string(opensearchtypes.DomainProcessingStatusTypeDeleting)
	}
	return string(d.DomainProcessingStatus)
}

// computeOpenSearchFindings classifies a DomainStatus against every
// opensearch signal. DescribeDomains is the fetcher's own call and
// DomainStatus carries all of them, so one slice holds every finding and
// domain.StatusPhrase counts each once.
func computeOpenSearchFindings(d opensearchtypes.DomainStatus, now time.Time) []domainpkg.Finding {
	var findings []domainpkg.Finding
	if (d.Processing != nil && *d.Processing) || (d.UpgradeProcessing != nil && *d.UpgradeProcessing) {
		findings = append(findings, wave1Finding(CodeOpenSearchProcessing))
	}
	if openSearchUpdateForcedSoon(d, now) {
		findings = append(findings, wave1Finding(opensearchCodeUpdateForced))
	}
	if openSearchEncryptionAtRestOff(d) {
		findings = append(findings, wave1Finding(opensearchCodeEncryptionOff))
	}
	// Reachable outside a VPC only counts when the access policy also lets
	// anyone in: a public endpoint fronted by a scoped policy is a deliberate,
	// defended design.
	if d.VPCOptions == nil && openSearchPolicyIsPublic(d) {
		findings = append(findings, wave1Finding(opensearchCodePublic))
	}
	if openSearchHTTPSNotEnforced(d) {
		findings = append(findings, wave1Finding(opensearchCodeHTTPSNotForced))
	}
	if openSearchNodeToNodeOff(d) {
		findings = append(findings, wave1Finding(opensearchCodeN2NOff))
	}
	return withState("opensearch", openSearchLifecycle(d), findings)
}

// openSearchEncryptionAtRestOff, openSearchHTTPSNotEnforced and
// openSearchNodeToNodeOff are the one reading of each setting: the findings
// and the Fields the list and the Wave 2 enricher show both come from here.
// Encryption at rest counts as off only when AWS states it is; HTTPS and
// node-to-node encryption count as off unless AWS states they are on.
func openSearchEncryptionAtRestOff(d opensearchtypes.DomainStatus) bool {
	return d.EncryptionAtRestOptions != nil && d.EncryptionAtRestOptions.Enabled != nil && !*d.EncryptionAtRestOptions.Enabled
}

func openSearchHTTPSNotEnforced(d opensearchtypes.DomainStatus) bool {
	return d.DomainEndpointOptions == nil || !aws.ToBool(d.DomainEndpointOptions.EnforceHTTPS)
}

func openSearchNodeToNodeOff(d opensearchtypes.DomainStatus) bool {
	return d.NodeToNodeEncryptionOptions == nil || !aws.ToBool(d.NodeToNodeEncryptionOptions.Enabled)
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
	listOutput, err := listAPI.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	if err != nil {
		return nil, fmt.Errorf("listing OpenSearch domains: %w", err)
	}

	if len(listOutput.DomainNames) == 0 {
		return []resource.Resource{}, nil
	}

	domainNames := make([]string, 0, len(listOutput.DomainNames))
	for _, d := range listOutput.DomainNames {
		if d.DomainName != nil {
			domainNames = append(domainNames, *d.DomainName)
		}
	}

	// DescribeDomains accepts at most five names per call (AWS API reference,
	// DomainNames: "Maximum number of 5 items").
	var resources []resource.Resource
	var failures []Failure
	for batch := range slices.Chunk(domainNames, 5) {
		var ans listedAnswer
		out, err := describeAPI.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{DomainNames: batch})
		if err == nil {
			for _, domain := range out.DomainStatusList {
				ans.rows = append(ans.rows, openSearchDomainRow(domain, now))
			}
		}
		rows, batchFailures := keepListed("DescribeDomains", batch, listedNameIsID, ans, err, func(name string, err error) resource.Resource {
			return DegradedDetails("opensearch", name, err)
		})
		resources = append(resources, rows...)
		failures = append(failures, batchFailures...)
	}
	return resources, AggregateFailures("opensearch: DescribeDomains", failures, len(domainNames))
}

func openSearchDomainRow(domain opensearchtypes.DomainStatus, now time.Time) resource.Resource {
	domainName := aws.ToString(domain.DomainName)

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

	deleted := strconv.FormatBool(aws.ToBool(domain.Deleted))
	processing := strconv.FormatBool(aws.ToBool(domain.Processing))
	upgradeProcessing := strconv.FormatBool(aws.ToBool(domain.UpgradeProcessing))

	// The detail field reads an absent DomainProcessingStatus as Active; the
	// lifecycle field keeps the value as AWS sent it.
	processingStatus := "Active"
	if domain.DomainProcessingStatus != "" {
		processingStatus = string(domain.DomainProcessingStatus)
	}

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

	encEnabled := strconv.FormatBool(!openSearchEncryptionAtRestOff(domain))

	// VPC placement and the access policy verdict are read here, at
	// the one point that holds the DomainStatus, and handed to the
	// wave-2 enricher as Fields — the enricher makes no AWS calls of
	// its own and must not re-derive them from RawStruct.
	vpcEnabled := strconv.FormatBool(domain.VPCOptions != nil)
	accessPolicyPublic := strconv.FormatBool(openSearchPolicyIsPublic(domain))
	enforceHTTPS := strconv.FormatBool(!openSearchHTTPSNotEnforced(domain))
	nodeToNode := strconv.FormatBool(!openSearchNodeToNodeOff(domain))

	r := resource.Resource{
		ID:       domainName,
		Name:     domainName,
		Findings: computeOpenSearchFindings(domain, now),
		Fields: map[string]string{
			"domain_name":                       domainName,
			"engine_version":                    engineVersion,
			"instance_type":                     instanceType,
			"instance_count":                    instanceCount,
			"endpoint":                          endpoint,
			"status":                            openSearchLifecycle(domain),
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

	return r
}
