// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — lt.go: EC2 Launch Template fetcher.
//
// DescribeLaunchTemplates (the list call) carries identity/version-number/
// creator facts only — no LaunchTemplateData. Every related-panel field and
// every signal lives on the "$Default" version, so this fetcher does
// DescribeLaunchTemplates (paginated) +
// DescribeLaunchTemplateVersions(Versions=["$Default"]) per template
// (in-fetcher N+1, as transfer.go and mwaa.go do), with per-item failures
// aggregated. The details-denied
// finding is fetcher-written (Source: "wave1"). The IMDSv1 and unencrypted
// findings live in lt_issue_enrichment.go, because an unset metadata option
// and an explicit Encrypted=false both answer to account-level defaults that
// need their own reads, and so does the deprecated-ami finding, because the
// fetcher has no sibling caches to cross-reference against.
//
// RawStruct is LTRaw — a composite wrapper, not either bare SDK shape alone:
// the list LaunchTemplate has DefaultVersionNumber/LatestVersionNumber/Tags
// but no data; the LaunchTemplateVersion has LaunchTemplateData/CreatedBy/
// CreateTime but not the latest-version number. A denied
// DescribeLaunchTemplateVersions call keeps the row with all list fields,
// as transfer.go does: RawStruct is the same LTRaw type with a zero
// DefaultVersion, so every *_related.go checker sees empty fields and
// reports unknown, and the shared details-denied finding is appended with
// lt's own sentence.
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

// lt.* FindingCodes (docs/resources/lt.md). ltCodeDeprecatedAMI is declared
// here rather than in lt_issue_enrichment.go so every lt.* code lives in one
// place.
const (
	ltCodeIMDSv1        domain.FindingCode = "lt.warn.imdsv1"
	ltCodeUnencrypted   domain.FindingCode = "lt.warn.unencrypted"
	ltCodeDeprecatedAMI domain.FindingCode = "lt.warn.deprecated_ami"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	ltCodeUserDataSecret domain.FindingCode = "lt.user-data-secret"
)

// LTRaw is the composite RawStruct for Launch Templates: neither bare SDK
// shape alone carries the whole detail story. DefaultVersion is the zero
// value on a degraded row — the same type for every row, healthy or
// degraded, so fieldpath/YAML/detail rendering and every *_related.go
// checker walk one shape.
type LTRaw struct {
	Template       ec2types.LaunchTemplate
	DefaultVersion ec2types.LaunchTemplateVersion
}

// FetchLaunchTemplatesPage fetches a single page of Launch Templates.
// DescribeLaunchTemplates carries no LaunchTemplateData, so every row gets a
// per-id DescribeLaunchTemplateVersions("$Default") call (in-fetcher N+1).
// Per-id describe failures are aggregated rather than dropping the row: the
// row is kept, built
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

	listOutput, err := api.DescribeLaunchTemplates(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing launch templates: %w", err)
	}

	var resources []resource.Resource
	var tally describeTally
	for i := range listOutput.LaunchTemplates {
		tpl := listOutput.LaunchTemplates[i]
		id := aws.ToString(tpl.LaunchTemplateId)

		rows := tally.one("DescribeLaunchTemplateVersions", id, id, func() (*resource.Resource, error) {
			out, err := api.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: aws.String(id),
				Versions:         []string{"$Default"},
			})
			if err != nil {
				return nil, err
			}
			if len(out.LaunchTemplateVersions) == 0 {
				return nil, UnusableAnswerErr{Call: "DescribeLaunchTemplateVersions", Field: "default version"}
			}
			ver := out.LaunchTemplateVersions[0]
			r := ltResource(tpl, ver, nil)
			return &r, nil
		}, func(err error) resource.Resource {
			return ltResource(tpl, ec2types.LaunchTemplateVersion{}, []domain.Finding{degradedDetailsFinding("lt", err)})
		})
		resources = append(resources, rows...)
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, tally.err("lt: DescribeLaunchTemplateVersions")
}

// ltResource constructs a Resource from the list LaunchTemplate, its
// "$Default" LaunchTemplateVersion, and the caller-computed findings — the
// healthy path passes none, since its findings all come from Wave 2; the
// degraded path passes the shared details-denied finding alongside a
// zero-value ver. RawStruct is *LTRaw for both.
func ltResource(tpl ec2types.LaunchTemplate, ver ec2types.LaunchTemplateVersion, findings []domain.Finding) resource.Resource {
	id := aws.ToString(tpl.LaunchTemplateId)
	name := aws.ToString(tpl.LaunchTemplateName)

	raw := &LTRaw{Template: tpl, DefaultVersion: ver}
	return withWave1("lt", resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"name":            name,
			"default_version": strconv.FormatInt(aws.ToInt64(tpl.DefaultVersionNumber), 10),
			"latest_version":  strconv.FormatInt(aws.ToInt64(tpl.LatestVersionNumber), 10),
			"created_by":      aws.ToString(tpl.CreatedBy),
			"created":         ltFormatTime(tpl.CreateTime),
		},
		RawStruct: raw,
	}, findings...)
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
