package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorCFN(r domain.Resource) domain.Color          { return cfnStackColor(r.Fields["status"]) }
func colorPipeline(_ domain.Resource) domain.Color     { return domain.ColorHealthy }
func colorCB(_ domain.Resource) domain.Color           { return domain.ColorHealthy }
func colorECR(_ domain.Resource) domain.Color          { return domain.ColorHealthy }
func colorCodeArtifact(_ domain.Resource) domain.Color { return domain.ColorHealthy }

var cicdTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "CloudFormation Stacks",
		ShortName:     "cfn",
		Aliases:       []string{"cfn", "cloudformation", "stacks"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "stack_name", Title: "Stack Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
			{Key: "last_updated", Title: "Updated", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Children: []domain.ChildViewDef{
			{ChildType: "cfn_events", Key: "enter", ContextKeys: map[string]string{"stack_name": "ID"}, DisplayNameKey: "Name"},
			{ChildType: "cfn_resources", Key: "R", ContextKeys: map[string]string{"stack_name": "ID"}, DisplayNameKey: "Name"},
		},
		Color: colorCFN,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudFormationStacksPage(ctx, c.CloudFormation, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichCFNCombined, Priority: 100},
		FieldKeys: []string{"stack_name", "status", "creation_time", "last_updated", "description"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCfnRole, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "Related Stacks", Checker: checkCFNCFN, NeedsTargetCache: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkCfnSNS},
			{TargetType: "s3", DisplayName: "S3 (stack resources)", Checker: checkCfnS3},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkCfnEBRule},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("cfn")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "RoleARN", TargetType: "role"},
			{FieldPath: "NotificationARNs", TargetType: "sns"},
		},
		IssueEnricherFieldKeys: []string{"drift_status"},
	},
	{
		Name:          "CodePipelines",
		ShortName:     "pipeline",
		Aliases:       []string{"pipeline", "codepipeline", "pipelines"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Pipeline Name", Width: 30, Sortable: true},
			{Key: "pipeline_type", Title: "Type", Width: 6, Sortable: true},
			{Key: "version", Title: "Version", Width: 9, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
			{Key: "updated", Title: "Updated", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "pipeline_stages",
			Key:            "enter",
			ContextKeys:    map[string]string{"pipeline_name": "ID"},
			DisplayNameKey: "Name",
		}},
		Color: colorPipeline,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCodePipelinesPage(ctx, c.CodePipeline, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichCodePipelineStatus, Priority: 10},
		FieldKeys:              []string{"name", "pipeline_type", "version", "created", "updated"},
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
	},
	{
		Name:          "CodeBuild Projects",
		ShortName:     "cb",
		Aliases:       []string{"cb", "codebuild"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Project Name", Width: 32, Sortable: true},
			{Key: "source_type", Title: "Source Type", Width: 14, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "cb_builds",
			Key:            "enter",
			ContextKeys:    map[string]string{"project_name": "ID"},
			DisplayNameKey: "project_name",
		}},
		Color: colorCB,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCodeBuildProjectsPage(ctx, c.CodeBuild, c.CodeBuild, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichCodeBuildStatus, Priority: 10},
		FieldKeys:              []string{"name", "source_type", "description", "last_modified"},
		IssueEnricherFieldKeys: []string{"last_build"},
		Related: []domain.RelatedDef{
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCbLogs, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCbRole, NeedsTargetCache: true},
			{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkCbPipeline, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCbSG},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkCbSubnet, NeedsTargetCache: false},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkCbVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkCbKMS},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkCbAlarm, NeedsTargetCache: true},
			{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkCbECR, NeedsTargetCache: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkCbS3, NeedsTargetCache: true},
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
	},
	{
		Name:          "ECR Repositories",
		ShortName:     "ecr",
		Aliases:       []string{"ecr", "container-registry"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "repository_name", Title: "Repository", Width: 36, Sortable: true},
			{Key: "uri", Title: "URI", Width: 60, Sortable: false},
			{Key: "tag_mutability", Title: "Tag Mutability", Width: 16, Sortable: true},
			{Key: "scan_on_push", Title: "Scan", Width: 6, Sortable: true},
			{Key: "created_at", Title: "Created", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "ecr_images",
			Key:            "enter",
			ContextKeys:    map[string]string{"repository_name": "ID", "repository_uri": "uri"},
			DisplayNameKey: "repository_name",
		}},
		Color: colorECR,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchECRRepositoriesPage(ctx, c.ECR, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichECRRepository, Priority: 100},
		FieldKeys: []string{
			"repository_name", "uri", "tag_mutability", "scan_on_push", "created_at",
		},
		IssueEnricherFieldKeys: []string{"critical_vulns", "high_vulns", "images_scanned"},
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkECRLambda, NeedsTargetCache: true},
			{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkECRCodeBuild, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECRCFN, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECRKMS},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkECRCTEvents, NeedsTargetCache: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkECREbRule},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkECRECSTask, NeedsTargetCache: true},
			{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkECRPipeline},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkECRRole},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "EncryptionConfiguration.KmsKey", TargetType: "kms"},
		},
	},
	{
		Name:          "CodeArtifact Repos",
		ShortName:     "codeartifact",
		Aliases:       []string{"codeartifact", "artifact", "ca"},
		Category:      "CI/CD",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "repo_name", Title: "Repository", Width: 28, Sortable: true},
			{Key: "domain_name", Title: "Domain", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "domain_owner", Title: "Owner", Width: 14, Sortable: true},
		},
		Color: colorCodeArtifact,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCodeArtifactReposPage(ctx, c.CodeArtifact, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichCodeArtifactRepository, Priority: 100},
		FieldKeys:              []string{"repo_name", "domain_name", "description", "domain_owner"},
		IssueEnricherFieldKeys: []string{"package_count"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkCodeartifactKMS, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("codeartifact"), NeedsTargetCache: false},
		},
	},
}

// cicdChildTypes is the declarative child-type catalog for the CI/CD category.
// Migrated under AS-816 / AS-795k per spec docs/refactor/AS-795-init-cycle-break.md §3 + §5.2.
// Sibling category PRs (AS-795b/c/d–m) own their own `<cat>ChildTypes` slice
// appended into allChildTypes() in install.go without merge conflicts.
//
// Each entry carries Name/ShortName/Columns/CopyField (verbatim from the deleted
// resource.RegisterChildType calls) plus FieldKeys and a ChildFetcher closure
// (verbatim from the deleted resource.RegisterPaginatedChild closures).
var cicdChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "CodeBuild Builds",
		ShortName: "cb_builds",
		Columns:   resource.CBBuildColumns(),
		CopyField: "build_id",
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
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCBBuilds(ctx, c.CodeBuild, c.CodeBuild, parentCtx, continuationToken)
		},
	},
	{
		Name:      "Build Logs",
		ShortName: "cb_build_logs",
		Columns:   resource.CBBuildLogColumns(),
		CopyField: "message",
		FieldKeys: []string{"timestamp", "message", "ingestion_time", "event_id"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCBBuildLogs(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], parentCtx["log_stream_name"], continuationToken)
		},
	},
	{
		Name:      "Pipeline Stages",
		ShortName: "pipeline_stages",
		Columns:   resource.PipelineStageColumns(),
		CopyField: "external_url",
		FieldKeys: []string{
			"stage_name", "stage_status", "action_name", "action_status",
			"last_change_time", "external_url", "action_token",
			"action_error_details", "revision_id", "revision_summary",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchPipelineStages(ctx, c.CodePipeline, parentCtx, continuationToken)
		},
	},
}
