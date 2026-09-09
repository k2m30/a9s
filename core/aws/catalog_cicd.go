// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorCFN derives the row colour from the stack's findings, and for a row that
// carries none from the same predicates the fetcher used: its status, and the
// two posture words it wrote into Fields. Nothing reads a rendered phrase back.
func colorCFN(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	findings := cfnStackFindings(r.Fields["status"])
	findings = append(findings, cfnPostureFindings(
		r.Fields["termination_protection"], r.Fields["output_secret"])...)
	return colorFromFindings(findings)
}

func colorPipeline(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorCB(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorECR(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorCodeArtifact(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

var cicdTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:           "CloudFormation Stacks",
		ShortName:      "cfn",
		HumanizeFields: []string{"status", "StackStatus"},
		Aliases:        []string{"cfn", "cloudformation", "stacks"},
		Category:       "CI/CD",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "cloudformation/home?region="+region+"#/stacks/stackinfo?stackId="+arn)
		},
		Columns: []domain.Column{
			{Key: "stack_name", Title: "Stack Name", Path: "StackName", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Path: "StackStatus", Width: 24, Sortable: true},
			{Key: "drift_status", Title: "Drift", Width: 14},
			{Title: "Reason", Path: "StackStatusReason", Width: 32},
			{Key: "creation_time", Title: "Created", Path: "CreationTime", Width: 22, Sortable: true},
			{Key: "last_updated", Title: "Updated", Path: "LastUpdatedTime", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
		},
		Children: []domain.ChildViewDef{
			{ChildType: "cfn_events", Key: "enter", ContextKeys: map[string]string{"stack_name": "ID"}, DisplayNameKey: "Name"},
			{ChildType: "cfn_resources", Key: "R", ContextKeys: map[string]string{"stack_name": "ID", "stack_arn": "arn"}, DisplayNameKey: "Name"},
		},
		Color: colorCFN,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudFormationStacksPage(ctx, c.CloudFormation, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichCFNCombined, Priority: 100},
		FieldKeys: []string{"stack_name", "status", "creation_time", "last_updated", "description", "arn"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCfnRole},
			{TargetType: "cfn", DisplayName: "Related Stacks", Checker: checkCFNCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkCfnSNS},
			{TargetType: "s3", DisplayName: "S3 (stack resources)", Checker: checkCfnS3},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkCfnEBRule},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("cfn")},
		},
		DetailEnrich: enrichCfn,
		Navigable: []domain.NavigableField{
			{FieldPath: "RoleARN", TargetType: "role"},
			{FieldPath: "NotificationARNs", TargetType: "sns"},
		},
		IssueEnricherFieldKeys: []string{"drift_status"},
		Findings: []catalog.FindingDef{
			{Code: CodeCFNStackFailed, Phrase: "<status, in words>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCFNStackRollback, Phrase: "<status, in words>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCFNStackInProgress, Phrase: "<status, in words>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeCFNStackDeleted, Phrase: "delete complete", Severity: domain.SevDim, Source: "wave1"},
			{Code: cfnCodeRecentResourceFailure, Phrase: "recent resource failure", Severity: domain.SevBroken, Source: "wave2"},
			{Code: cfnCodeStackDrifted, Phrase: "stack drifted from template", Severity: domain.SevWarn, Source: "wave2"},
			{Code: CodeCFNTerminationProtectionOff, Phrase: "termination protection off", Severity: domain.SevWarn, Source: "wave1", Detail: "A single delete call removes this stack and every resource it owns, with no second step to stop an accidental or scripted deletion. Turn on termination protection so the stack must be unprotected deliberately before it can be deleted."},
			{Code: CodeCFNOutputSecret, Phrase: "credential in stack outputs", Severity: domain.SevBroken, Source: "wave1", Detail: "A stack output holds what looks like a credential, and outputs are readable by anyone who can describe the stack and importable by any other stack in the account. Move the value into Secrets Manager, export only its name, and rotate the exposed credential."},
		},
	},
	{
		Name:           "CodePipelines",
		ShortName:      "pipeline",
		HumanizeFields: []string{"pipeline_type", "ExecutionMode"},
		Aliases:        []string{"pipeline", "codepipeline", "pipelines"},
		Category:       "CI/CD",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "codesuite/codepipeline/pipelines/"+url.PathEscape(r.ID)+"/view?region="+region)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Pipeline Name", Path: "Name", Width: 30, Sortable: true},
			{Key: "pipeline_type", Title: "Type", Path: "PipelineType", Width: 6, Sortable: true},
			{Key: "last_status", Title: "Status", Width: 18, Sortable: true},
			{Key: "version", Title: "Version", Path: "Version", Width: 9, Sortable: true},
			{Key: "created", Title: "Created", Path: "Created", Width: 22, Sortable: true},
			{Key: "updated", Title: "Updated", Path: "Updated", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "pipeline_stages",
			Key:            "enter",
			ContextKeys:    map[string]string{"pipeline_name": "ID"},
			DisplayNameKey: "Name",
		}},
		Color:                  colorPipeline,
		Fetcher:                fetcherWithClients(FetchCodePipelinesPageWithClients),
		Wave2:                  IssueEnricher{Fn: EnrichCodePipelineStatus, Priority: 10},
		FieldKeys:              []string{"name", "pipeline_type", "version", "created", "updated", "arn"},
		IssueEnricherFieldKeys: []string{"last_status"},
		Related: []domain.RelatedDef{
			{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkPipelineCB, NeedsTargetCache: false},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkPipelineRole, NeedsTargetCache: false},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkPipelineCFN, NeedsTargetCache: false},
			{TargetType: "codeartifact", DisplayName: "CodeArtifact", Checker: checkPipelineCodeartifact, NeedsTargetCache: false},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkPipelineEbRule, NeedsTargetCache: false},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkPipelineECR, NeedsTargetCache: false},
			{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkPipelineECSSvc, NeedsTargetCache: false},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkPipelineKMS, NeedsTargetCache: false},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkPipelineLambda, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets (artifacts)", Checker: checkPipelineS3, NeedsTargetCache: false},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkPipelineSNS, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("pipeline"), NeedsTargetCache: false},
		},
		Findings: []catalog.FindingDef{
			{Code: pipelineCodeStageFailed, Phrase: "stage failed", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
	{
		Name:           "CodeBuild Projects",
		ShortName:      "cb",
		HumanizeFields: []string{"source_type"},
		Aliases:        []string{"cb", "codebuild"},
		Category:       "CI/CD",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, accountID string) string {
			acct := consolelink.AccountFromARN(r.Fields["arn"])
			if acct == "" {
				acct = accountID
			}
			if acct == "" {
				return ""
			}
			return consolelink.Regional(region, "codesuite/codebuild/"+acct+"/projects/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Project Name", Path: "Name", Width: 32, Sortable: true},
			{Key: "source_type", Title: "Source Type", Path: "Source.Type", Width: 14, Sortable: true},
			{Key: "last_build", Title: "Status", Width: 18, Sortable: true},
			{Key: "description", Title: "Description", Path: "Description", Width: 36},
			{Key: "last_modified", Title: "Last Modified", Path: "LastModified", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "cb_builds",
			Key:            "enter",
			ContextKeys:    map[string]string{"project_name": "ID"},
			DisplayNameKey: "project_name",
		}},
		Color: colorCB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCodeBuildProjectsPage(ctx, c.CodeBuild, c.CodeBuild, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichCodeBuildStatus, Priority: 10},
		FieldKeys:              []string{"name", "source_type", "description", "last_modified", "arn"},
		IssueEnricherFieldKeys: []string{"last_build"},
		Related: []domain.RelatedDef{
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCbLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCbRole},
			{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkCbPipeline, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCbSG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkCbSubnet, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkCbVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkCbKMS},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkCbAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkCbECR, NeedsTargetCache: true, Truncated: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkCbS3, NeedsTargetCache: true, Truncated: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkCbSecrets, NeedsTargetCache: false},
			{TargetType: "ssm", DisplayName: "SSM Parameters", Checker: checkCbSSM, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("cb"), NeedsTargetCache: false},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ServiceRole", TargetType: "role"},
			{FieldPath: "EncryptionKey", TargetType: "kms"},
			{FieldPath: "VpcConfig.VpcId", TargetType: "vpc"},
			{FieldPath: "VpcConfig.Subnets", TargetType: "subnet"},
			{FieldPath: "VpcConfig.SecurityGroupIds", TargetType: "sg"},
		},
		Findings: []catalog.FindingDef{
			{Code: cbCodeLatestBuildFailed, Phrase: "latest build <status>", Severity: domain.SevBroken, Source: "wave2"},
			{Code: CodeCBPublicBuilds, Phrase: "build results publicly visible", Severity: domain.SevBroken, Source: "wave1", Detail: "Build logs, environment variables and artifacts for this project are readable by anyone on the internet without an AWS account, so any credential or internal hostname a build prints is public. Set the project's visibility back to private and rotate anything the logs have already exposed."},
			{Code: CodeCBBuildspecFromSource, Phrase: "buildspec taken from the source repository", Severity: domain.SevWarn, Source: "wave1", Detail: "The build instructions come from a file in the source repository, so anyone who can open a pull request can change what runs inside the build role. Move the buildspec inline into the project definition, or restrict who can trigger builds from unmerged branches."},
			{Code: CodeCBSourceURLCredential, Phrase: "credential in the source repository address", Severity: domain.SevBroken, Source: "wave1", Detail: "The source repository address embeds a username and password or token, which is stored in the project definition and printed in build logs in clear text. Move the credential into a CodeBuild source credential or Secrets Manager entry and rotate it, because it must be assumed leaked."},
			{Code: CodeCBEnvSecret, Phrase: "credential in environment variables", Severity: domain.SevBroken, Source: "wave1", Detail: "A plaintext environment variable on this project holds what looks like a credential; every build log and anyone who can read the project definition sees its value. Move it to Secrets Manager or Parameter Store, reference it by type, and rotate the exposed value."},
		},
	},
	{
		Name:           "ECR Repositories",
		ShortName:      "ecr",
		HumanizeFields: []string{"tag_mutability", "ImageTagMutability"},
		Aliases:        []string{"ecr", "container-registry"},
		Category:       "CI/CD",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "ecr/repositories/"+url.PathEscape(r.ID)+"/?region="+region)
		},
		Columns: []domain.Column{
			{Key: "repository_name", Title: "Repository", Path: "RepositoryName", Width: 36, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "uri", Title: "URI", Path: "RepositoryUri", Width: 60},
			{Key: "tag_mutability", Title: "Tag Mutability", Path: "ImageTagMutability", Width: 16, Sortable: true},
			{Key: "scan_on_push", Title: "Scan", Path: "ImageScanningConfiguration.ScanOnPush", Width: 6, Sortable: true},
			{Key: "critical_vulns", Title: "Critical", Width: 8},
			{Key: "high_vulns", Title: "High", Width: 6},
			{Key: "images_scanned", Title: "Scanned", Width: 8},
			{Key: "created_at", Title: "Created", Path: "CreatedAt", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "ecr_images",
			Key:            "enter",
			ContextKeys:    map[string]string{"repository_name": "ID", "repository_uri": "uri"},
			DisplayNameKey: "repository_name",
		}},
		Color: colorECR,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchECRRepositoriesPage(ctx, c.ECR, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichECRRepository, Priority: 100},
		FieldKeys: []string{
			"repository_name", "uri", "tag_mutability", "scan_on_push", "created_at",
		},
		IssueEnricherFieldKeys: []string{"critical_vulns", "high_vulns", "images_scanned"},
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkECRLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkECRCodeBuild, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECRCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECRKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkECRCTEvents, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkECREbRule, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECRECSTask, NeedsTargetCache: true, Truncated: true},
			{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkECRPipeline, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkECRRole},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "EncryptionConfiguration.KmsKey", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: ecrCodeVulnerabilities, Phrase: "<N> critical, <M> high vulnerabilities", Severity: domain.SevBroken, Source: "wave2"},
			{Code: ecrCodeHighVulnerabilities, Phrase: "<N> high vulnerabilities", Severity: domain.SevWarn, Source: "wave2"},
			{Code: ecrCodePublicPolicy, Phrase: "repository policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The repository policy grants a wildcard principal, so any AWS account can pull the images this repository holds and read whatever is baked into their layers. Replace the wildcard principal with the accounts or roles that need the images, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing."},
			{Code: ecrCodeNoLifecyclePolicy, Phrase: "no lifecycle policy", Severity: domain.SevWarn, Source: "wave2", Detail: "No lifecycle policy is set, so every image ever pushed is kept forever: storage cost grows without limit and long-superseded, vulnerable images stay pullable by tag or digest. Add a lifecycle policy that expires untagged images and caps how many versions of each tag are retained."},
			{Code: CodeECRScanOnPushOff, Phrase: "scan on push off", Severity: domain.SevWarn, Source: "wave1", Detail: "Images pushed to this repository are never scanned, so a known vulnerability in a base layer reaches production without anyone being told. Turn on scan on push for the repository so every new image is checked as it arrives."},
			{Code: CodeECRMutableTags, Phrase: "tags are mutable", Severity: domain.SevWarn, Source: "wave1", Detail: "An existing tag in this repository can be moved to different image content, so the digest behind a deployed tag can change without any deployment. Set the repository to immutable tags so a tag always names the image it was built from."},
		},
	},
	{
		Name:          "CodeArtifact Repos",
		ShortName:     "codeartifact",
		Aliases:       []string{"codeartifact", "artifact", "ca"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, accountID string) string {
			acct := r.Fields["domain_owner"]
			if acct == "" {
				acct = consolelink.AccountFromARN(r.Fields["arn"])
			}
			if acct == "" {
				acct = accountID
			}
			domainName := r.Fields["domain_name"]
			if acct == "" || domainName == "" {
				return ""
			}
			return consolelink.Regional(region, "codesuite/codeartifact/d/"+acct+"/"+url.PathEscape(domainName)+"/r/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "repo_name", Title: "Repository", Path: "Name", Width: 28, Sortable: true},
			{Title: "Status", Width: 12},
			{Key: "domain_name", Title: "Domain", Path: "DomainName", Width: 24, Sortable: true},
			{Key: "package_count", Title: "Packages", Width: 8},
			{Key: "description", Title: "Description", Path: "Description", Width: 30},
			{Key: "domain_owner", Title: "Owner", Path: "DomainOwner", Width: 14, Sortable: true},
		},
		Color: colorCodeArtifact,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCodeArtifactReposPage(ctx, c.CodeArtifact, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichCodeArtifactRepository, Priority: 100},
		FieldKeys:              []string{"repo_name", "domain_name", "description", "domain_owner", "arn"},
		IssueEnricherFieldKeys: []string{"package_count"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkCodeartifactKMS, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("codeartifact"), NeedsTargetCache: false},
		},
		Findings: []catalog.FindingDef{
			{Code: codeartifactCodeNoPermissionsPolicy, Phrase: "no permissions policy", Severity: domain.SevWarn, Source: "wave2"},
			{Code: codeartifactCodePublicAccessPolicy, Phrase: "public access policy", Severity: domain.SevBroken, Source: "wave2", Detail: "The repository's resource policy grants a wildcard principal, so any AWS account can read the packages it holds and, depending on the actions allowed, publish into it. Replace the \"*\" principal with the accounts or roles that need the repository, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing."},
		},
	},
}

// cicdChildTypes is the declarative child-type catalog for the CI/CD category.
// Each per-category `<cat>ChildTypes` slice is appended into allChildTypes()
// in install.go.
//
// Each entry carries Name/ShortName/Columns/CopyField plus FieldKeys and a
// ChildFetcher closure.
var cicdChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "CodeBuild Builds",
		ShortName: "cb_builds",
		ConsoleURL: func(r domain.Resource, region, accountID string) string {
			buildID := r.Fields["build_id"]
			if buildID == "" {
				return ""
			}
			proj, _, ok := strings.Cut(buildID, ":")
			if !ok || proj == "" {
				return ""
			}
			acct := consolelink.AccountFromARN(r.Fields["build_arn"])
			if acct == "" {
				acct = accountID
			}
			if acct == "" {
				return ""
			}
			return consolelink.Regional(region, "codesuite/codebuild/"+acct+"/projects/"+url.PathEscape(proj)+"/build/"+url.PathEscape(buildID))
		},
		Columns:   resource.CBBuildColumns(),
		CopyField: "build_id",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"build_number", "build_status", "start_time", "end_time",
			"duration", "source_version_short", "initiator", "build_id",
			"build_arn", "current_phase", "source_version",
			"resolved_source_version", "log_group_name", "log_stream_name",
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "cb_build_logs",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "log_group_name", "log_stream_name": "log_stream_name", "build_number": "build_number"},
			DisplayNameKey: "build_number",
			DrillCondition: func(r domain.Resource) bool {
				return r.Fields["log_group_name"] != ""
			},
			DrillBlockMessage: "Build logs not available in CloudWatch",
		}},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchCBBuilds(ctx, c.CodeBuild, c.CodeBuild, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCBBuildFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCBBuildFault, Phrase: "fault", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCBBuildTimedOut, Phrase: "timed out", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCBBuildInProgress, Phrase: "in progress", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeCBBuildStopped, Phrase: "stopped", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:      "Build Logs",
		ShortName: "cb_build_logs",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group_name"], r.Fields["log_stream_name"])
		},
		Columns:   resource.CBBuildLogColumns(),
		CopyField: "message",
		FieldKeys: []string{"timestamp", "message", "ingestion_time", "event_id", "log_group_name", "log_stream_name"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchCBBuildLogs(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], parentCtx["log_stream_name"], continuationToken)
		}),
	},
	{
		Name:      "Pipeline Stages",
		ShortName: "pipeline_stages",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			name := r.Fields["pipeline_name"]
			if name == "" {
				return ""
			}
			return consolelink.Regional(region, "codesuite/codepipeline/pipelines/"+url.PathEscape(name)+"/view?region="+region)
		},
		Columns:   resource.PipelineStageColumns(),
		CopyField: "external_url",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"stage_name", "stage_status", "action_name", "action_status",
			"last_change_time", "external_url", "action_token",
			"action_error_details", "revision_id", "revision_summary", "pipeline_name",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchPipelineStages(ctx, c.CodePipeline, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodePipelineActionFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
	},
}
