// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// FetchCodeBuildProjectsPage fetches one page of project names from ListProjects
// using the continuationToken, then calls BatchGetProjects for that page's names.
// IsTruncated reflects whether ListProjects has more pages beyond this one.
func FetchCodeBuildProjectsPage(
	ctx context.Context,
	listAPI CodeBuildListProjectsAPI,
	batchAPI CodeBuildBatchGetProjectsAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &codebuild.ListProjectsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listAPI.ListProjects(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing CodeBuild projects: %w", err)
	}

	if len(listOutput.Projects) == 0 {
		nextToken := ""
		isTruncated := false
		if listOutput.NextToken != nil {
			nextToken = *listOutput.NextToken
			isTruncated = true
		}
		return resource.FetchResult{
			Resources: []resource.Resource{},
			Pagination: &resource.PaginationMeta{
				IsTruncated: isTruncated,
				NextToken:   nextToken,
				PageSize:    0,
				TotalHint:   -1,
			},
		}, nil
	}

	batchOutput, err := batchAPI.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
		Names: listOutput.Projects,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("batch getting CodeBuild projects: %w", err)
	}

	var resources []resource.Resource

	for _, project := range batchOutput.Projects {
		name := ""
		if project.Name != nil {
			name = *project.Name
		}

		description := ""
		if project.Description != nil {
			description = *project.Description
		}

		sourceType := ""
		if project.Source != nil {
			sourceType = string(project.Source.Type)
		}

		lastModified := ""
		if project.LastModified != nil {
			lastModified = project.LastModified.Format("2006-01-02 15:04")
		}

		arn := ""
		if project.Arn != nil {
			arn = *project.Arn
		}

		r := resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"name":          name,
				"source_type":   sourceType,
				"description":   description,
				"last_modified": lastModified,
				"arn":           arn,
			},
			RawStruct: project,
		}

		addCBPostureFindings(&r, project)
		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}

// cbContributorSourceTypes are the source types whose contents a pull-request
// author can change. The buildspec finding is about who controls the build
// commands, so an S3 archive or a NO_SOURCE project — which nobody can open a
// pull request against — is out of scope however its buildspec is set.
var cbContributorSourceTypes = map[cbtypes.SourceType]bool{ //nolint:gochecknoglobals // static SDK enum set
	cbtypes.SourceTypeGithub:           true,
	cbtypes.SourceTypeGithubEnterprise: true,
	cbtypes.SourceTypeBitbucket:        true,
	cbtypes.SourceTypeCodecommit:       true,
}

// cbBuildspecFromSource reports whether the buildspec lives in the source
// repository, and what to show the operator. CodeBuild treats an empty
// Buildspec as "buildspec.yml at the repository root", and a value that is a
// file path (rather than inline YAML, which always spans lines) as a path in
// that same repository. Either way a contributor controls the commands.
func cbBuildspecFromSource(src *cbtypes.ProjectSource) (string, bool) {
	if src == nil || !cbContributorSourceTypes[src.Type] {
		return "", false
	}
	spec := strings.TrimSpace(aws.ToString(src.Buildspec))
	switch {
	case spec == "":
		return "repo default", true
	case strings.ContainsAny(spec, "\n\r"):
		return "", false
	case strings.HasSuffix(spec, ".yml"), strings.HasSuffix(spec, ".yaml"):
		return spec, true
	}
	return "", false
}

// cbSourceLocationCredential returns the source address with its userinfo
// removed when the address embeds a credential. A CodeCommit or S3 location is
// not a URL and never carries one, so a parse failure is a healthy answer
// rather than a reason to guess.
func cbSourceLocationCredential(src *cbtypes.ProjectSource) (string, bool) {
	if src == nil {
		return "", false
	}
	loc := aws.ToString(src.Location)
	u, err := url.Parse(loc)
	if err != nil || u.User == nil || u.Scheme == "" {
		return "", false
	}
	if _, hasPassword := u.User.Password(); !hasPassword && u.User.Username() == "" {
		return "", false
	}
	u.User = nil
	return u.String(), true
}

// addCBPostureFindings evaluates the four w6b posture signals against the
// BatchGetProjects response the fetcher already holds. Each is independent
// (contract rule 4), so a project that is wrong four ways carries four
// findings.
func addCBPostureFindings(r *resource.Resource, project cbtypes.Project) {
	if project.ProjectVisibility == cbtypes.ProjectVisibilityTypePublicRead {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeCBPublicBuilds, Phrase: "build results publicly visible",
			Detail: cbPublicBuildsDetail, Severity: domain.SevBroken, Source: "wave1",
		})
	}

	if spec, ok := cbBuildspecFromSource(project.Source); ok {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeCBBuildspecFromSource, Phrase: "buildspec taken from the source repository",
			Detail: cbBuildspecFromSourceDetail, Severity: domain.SevWarn, Source: "wave1",
		})
		addWave1Rows(r, CodeCBBuildspecFromSource, domain.DetailRow{Label: "Buildspec", Value: spec, Tier: "~"})
	}

	if redacted, ok := cbSourceLocationCredential(project.Source); ok {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeCBSourceURLCredential, Phrase: "credential in the source repository address",
			Detail: cbSourceURLCredentialDetail, Severity: domain.SevBroken, Source: "wave1",
		})
		addWave1Rows(r, CodeCBSourceURLCredential, domain.DetailRow{Label: "Repository", Value: redacted, Tier: "!"})
	}

	if project.Environment == nil {
		return
	}
	// PARAMETER_STORE and SECRETS_MANAGER variables hold the name of a
	// secret, not the secret; scanning them reports the recommended fix as
	// the defect.
	plaintext := make(map[string]string, len(project.Environment.EnvironmentVariables))
	for _, ev := range project.Environment.EnvironmentVariables {
		if ev.Type == cbtypes.EnvironmentVariableTypePlaintext {
			plaintext[aws.ToString(ev.Name)] = aws.ToString(ev.Value)
		}
	}
	if hits := secretscan.ScanKV(plaintext); len(hits) > 0 {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeCBEnvSecret, Phrase: "credential in environment variables",
			Detail: cbEnvSecretDetail, Severity: domain.SevBroken, Source: "wave1",
		})
		for _, h := range hits {
			addWave1Rows(r, CodeCBEnvSecret, domain.DetailRow{Label: h.Where, Value: h.Kind, Tier: "!"})
		}
	}
}
