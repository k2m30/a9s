// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — lt.go: EC2 Launch Template fetcher.
//
// docs/resources/lt.md §1: DescribeLaunchTemplates (the list call) carries
// identity/version-number/creator facts only — NO LaunchTemplateData. Every
// §2 related-panel field and every §3.2 signal lives on the "$Default"
// version, so this fetcher does DescribeLaunchTemplates (paginated) +
// DescribeLaunchTemplateVersions(Versions=["$Default"]) per template
// (in-fetcher N+1, the transfer.go/mwaa.go pattern) — both RetryOnThrottle,
// E3/E5 aggregation. IMDSv1/unencrypted/details-denied findings are
// fetcher-written (Source: "wave1"); the deprecated-ami finding alone lives
// in lt_issue_enrichment.go's cache-scan enricher, because the fetcher has
// no sibling caches to cross-reference against.
//
// RawStruct is LTRaw — a composite wrapper, not either bare SDK shape alone:
// the list LaunchTemplate has DefaultVersionNumber/LatestVersionNumber/Tags
// but no data; the LaunchTemplateVersion has LaunchTemplateData/CreatedBy/
// CreateTime but not the latest-version number. A denied
// DescribeLaunchTemplateVersions call keeps the row (rich degradation, the
// transfer.go convention, not mwaa's name-only degradation): all list fields
// survive, RawStruct is the SAME LTRaw type with a zero DefaultVersion (no
// dual-shape fallback — every *_related.go Pattern F checker just sees
// empty fields and reports unknown), and the shared details-denied finding
// is appended with lt's own §4 sentence.
package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// lt.* FindingCodes — docs/resources/lt.md §3/§4. ltCodeDeprecatedAMI is
// declared here (not in lt_issue_enrichment.go) so every lt.* code lives in
// one place, mirroring transfer.go's single const block.
const (
	ltCodeIMDSv1        domain.FindingCode = "lt.warn.imdsv1"
	ltCodeUnencrypted   domain.FindingCode = "lt.warn.unencrypted"
	ltCodeDeprecatedAMI domain.FindingCode = "lt.warn.deprecated_ami"
)

// ltDetailsDeniedDetail is lt's own §4 S5 sentence for the details-denied
// finding — deliberately NOT the generic degraded_resource.go text, matching
// transferDetailsDeniedDetail's precedent (a rich degraded row keeps far
// more than just the name, so the sentence names what's specifically
// missing: the default version).
const ltDetailsDeniedDetail = "Access to the default version was denied; only the listed fields are visible."

// LTRaw is the composite RawStruct for Launch Templates: neither bare SDK
// shape alone carries the whole detail story (docs/resources/lt-impl-plan.md
// §0). DefaultVersion is the zero value on a rich degraded row — the SAME
// type for every row, healthy or degraded, so fieldpath/YAML/detail
// rendering and every *_related.go checker walk one shape.
type LTRaw struct {
	Template       ec2types.LaunchTemplate
	DefaultVersion ec2types.LaunchTemplateVersion
}

// FetchLaunchTemplatesPage fetches a single page of Launch Templates.
// DescribeLaunchTemplates carries no LaunchTemplateData, so every row gets a
// per-id DescribeLaunchTemplateVersions("$Default") call (in-fetcher N+1, no
// separate lt Wave-2-API-calling enrichment). Per-id describe failures are
// aggregated (E3/E5) rather than dropping the row: the row is kept, built
// from the list fields, with the details-denied finding appended. A
// DescribeLaunchTemplates failure returns an error, never an empty success
// (AccessDenied contract).
func FetchLaunchTemplatesPage(ctx context.Context, api EC2FetchLaunchTemplatesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeLaunchTemplatesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplatesOutput, error) {
		return api.DescribeLaunchTemplates(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing launch templates: %w", err)
	}

	total := len(listOutput.LaunchTemplates)
	var resources []resource.Resource
	var failures []string
	for i := range listOutput.LaunchTemplates {
		tpl := listOutput.LaunchTemplates[i]
		id := aws.ToString(tpl.LaunchTemplateId)

		versionOutput, versionErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return api.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: aws.String(id),
				Versions:         []string{"$Default"},
			})
		})
		switch {
		case versionErr != nil:
			failures = append(failures, fmt.Sprintf("%s: %s", id, versionErr.Error()))
			resources = append(resources, ltResource(tpl, ec2types.LaunchTemplateVersion{}, []domain.Finding{degradedDetailsFinding("lt", versionErr, ltDetailsDeniedDetail, detailsUnavailableDetail)}))
		case len(versionOutput.LaunchTemplateVersions) == 0:
			failures = append(failures, fmt.Sprintf("%s: no $Default version in DescribeLaunchTemplateVersions response", id))
			resources = append(resources, ltResource(tpl, ec2types.LaunchTemplateVersion{}, []domain.Finding{degradedDetailsFinding("lt", nil, ltDetailsDeniedDetail, detailsUnavailableDetail)}))
		default:
			ver := versionOutput.LaunchTemplateVersions[0]
			resources = append(resources, ltResource(tpl, ver, computeLTFindings(ver)))
		}
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("lt: DescribeLaunchTemplateVersions", failures, total)
}

// ltResource constructs a Resource from the list LaunchTemplate, its
// "$Default" LaunchTemplateVersion, and the caller-computed findings — the
// healthy path passes computeLTFindings(ver); the degraded path passes the
// shared details-denied finding alongside a zero-value ver (the SAME *LTRaw
// shape for both rows, docs/resources/lt-impl-plan.md §0). RawStruct is
// *LTRaw (the composite wrapper's pointer, mwaa/transfer's value/pointer
// convention).
func ltResource(tpl ec2types.LaunchTemplate, ver ec2types.LaunchTemplateVersion, findings []domain.Finding) resource.Resource {
	id := aws.ToString(tpl.LaunchTemplateId)
	name := aws.ToString(tpl.LaunchTemplateName)

	raw := &LTRaw{Template: tpl, DefaultVersion: ver}
	return resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"name":            name,
			"status":          phraseFromFindings(findings),
			"default_version": strconv.FormatInt(aws.ToInt64(tpl.DefaultVersionNumber), 10),
			"latest_version":  strconv.FormatInt(aws.ToInt64(tpl.LatestVersionNumber), 10),
			"created_by":      aws.ToString(tpl.CreatedBy),
			"created":         ltFormatTime(tpl.CreateTime),
		},
		RawStruct: raw,
		Findings:  findings,
	}
}

// computeLTFindings builds the ordered Finding slice for one "$Default"
// version: IMDSv1 first, then unencrypted-EBS — docs/resources/lt.md §4
// precedence order (deprecated-ami and details-denied are appended later, by
// the Wave 2 enricher and the degraded-row builder respectively).
func computeLTFindings(ver ec2types.LaunchTemplateVersion) []domain.Finding {
	data := ver.LaunchTemplateData
	if data == nil {
		return nil
	}

	var findings []domain.Finding

	// HttpEndpoint == disabled means the metadata service is unreachable
	// entirely; HttpTokens is moot and produces no signal regardless of its
	// value (docs/resources/lt.md §3.2).
	endpointDisabled := data.MetadataOptions != nil && data.MetadataOptions.HttpEndpoint == ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled

	// Unset defaults to optional (SDK-confirmed) — absence of
	// MetadataOptions IS the signal, not its negation.
	if !endpointDisabled && (data.MetadataOptions == nil || data.MetadataOptions.HttpTokens != ec2types.LaunchTemplateHttpTokensStateRequired) {
		findings = append(findings, domain.Finding{
			Code:     ltCodeIMDSv1,
			Phrase:   "IMDSv1 allowed",
			Detail:   "Instance metadata does not require session tokens; IMDSv1 credentials are exposed to SSRF.",
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	// nil Encrypted is UNKNOWN, not unencrypted — never flag nil. Only an
	// explicit Encrypted=false fires; one mapping is enough to flag the row.
	for _, bdm := range data.BlockDeviceMappings {
		if bdm.Ebs != nil && bdm.Ebs.Encrypted != nil && !*bdm.Ebs.Encrypted {
			findings = append(findings, domain.Finding{
				Code:     ltCodeUnencrypted,
				Phrase:   "EBS encryption disabled",
				Detail:   "A block device explicitly sets Encrypted=false; launched instances get unencrypted volumes.",
				Severity: domain.SevWarn,
				Source:   "wave1",
			})
			break
		}
	}

	return findings
}

// ltFormatTime formats *time.Time as the house "Created" column format
// ("2006-01-02 15:04" — the ebs.go/dbi_snap.go/pipeline.go convention),
// returning "" for nil.
func ltFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
