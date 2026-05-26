package unit

// qa_invariants_test.go — Structural invariants across the entire resource registry.
//
// These tests are the "missing invariant" guardrails added after Task #20/#21 found that
// newly added resource types (e.g. dbi) could silently ship without navigable fields or
// without populating Rows in their enrichment findings.
//
// T-INV-1: TestResourceTypeDef_AllHaveNavigableFields
//   Every resource type in AllResourceTypes() MUST have at least one NavigableField
//   registered, EXCEPT the types on the explicit allow-list below.  Adding a new type
//   without registering navigable fields will cause this test to fail, forcing the
//   engineer to either add the fields or justify the omission by adding to the list.
//
// T-INV-2: TestEnrichmentFinding_AllKeptEnrichersPopulateRows
//   Every enricher in the buildEnrichQueue order list MUST populate at least one
//   FindingRow when a matching issue exists.  An enricher that never populates Rows
//   produces a finding the detail view cannot render meaningfully.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	codepipeline "github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	glusvc "github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	rdssvc "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// T-INV-1: NavigableFields coverage invariant
// ─────────────────────────────────────────────────────────────────────────────

// noNavFieldsAllowList contains short names that legitimately have zero navigable
// fields.  These are either pure listing types (no cross-resource ARN references),
// identity/permission types, or DNS/CDN global services whose fields don't resolve
// to other cache-backed resource types.
//
// Rule: adding a type here MUST be accompanied by a comment explaining why it has
// no navigable fields.  Silently adding to this list defeats the purpose of the test.
var noNavFieldsAllowList = map[string]string{
	// Monitoring / audit — alarms reference metrics, not cache-backed resources
	"alarm": "CloudWatch alarms reference metrics, not navigable resource ARNs",
	// Analytics — Athena queries reference S3 paths and Glue catalogs by string
	"athena": "Athena query results reference S3 paths, no navigable ARN fields",
	// Messaging — SQS queues have no structural ARN references to other resources
	"sqs": "SQS queues have no navigable ARN fields in their list entry",
	// Messaging — SNS topic list entry has no ARN cross-references
	"sns": "SNS topic list entry has no navigable ARN cross-reference fields",
	// Messaging — Kinesis streams have no ARN references to other resources
	"kinesis": "Kinesis stream list entry has no navigable ARN fields",
	// Compute enrichment — SFN state machines: ARN navigates to execution, not a cached type
	"sfn": "SFN state machines reference execution ARNs, not cache-backed resource types",
	// Backup — AWS Backup plan entries have no ARN cross-references
	"backup": "AWS Backup plan entries have no navigable ARN fields",
	// SES — email identity records have no navigable cross-resource ARNs
	"ses": "SES identity entries have no navigable ARN fields",
	// KMS — key metadata has no ARN references to other resource types
	"kms": "KMS key metadata has no navigable cross-resource ARN fields",
	// Storage — S3 buckets have no navigable ARN references in bucket list entry
	"s3": "S3 bucket list entry has no navigable ARN cross-reference fields",
	// Database — Redshift cluster has no navigable ARN fields in cluster list
	"redshift": "Redshift cluster list entry has no navigable ARN cross-reference fields",
	// CI/CD — CodePipeline state has no navigable fields in the pipeline list entry
	"pipeline": "CodePipeline list entry has no navigable cross-resource ARN fields",
	// CI/CD — CodeArtifact domain list entry has no navigable ARN cross-references
	"codeartifact": "CodeArtifact domain list entry has no navigable ARN fields",
	// DNS/CDN — Route53 hosted zone has no ARN cross-references to cached resources
	"r53": "Route53 hosted zone has no navigable ARN cross-reference fields",
	// DNS/CDN — CloudFront distribution references S3/ALB origins as strings
	"cf": "CloudFront distribution references origins as strings, not cache-backed ARNs",
	// DNS/CDN — ACM certificate has no navigable ARN references to other resource types
	"acm": "ACM certificate list entry has no navigable ARN cross-reference fields",
	// DNS/CDN — API Gateway REST APIs have no navigable ARN references
	"apigw": "API Gateway list entry has no navigable ARN cross-reference fields",
	// Networking — VPCs are container resources; the list entry has no ARN cross-references
	"vpc": "VPC list entry has no navigable cross-resource ARN fields (subnets/RTBs reference VPC by ID, not vice versa)",
	// Networking — Transit Gateways have no navigable ARN cross-references in list entry
	"tgw": "Transit Gateway list entry has no navigable ARN cross-reference fields",
	// Compute — Elastic Beanstalk environment list entry has no navigable ARN fields
	"eb": "Elastic Beanstalk environment has no navigable cross-resource ARN fields",
	// IAM — role, policy, user, group have no navigable ARN fields in list entry
	"role":      "IAM role list entry has no navigable cross-resource ARN fields",
	"policy":    "IAM policy list entry has no navigable cross-resource ARN fields",
	"iam-user":  "IAM user list entry has no navigable cross-resource ARN fields",
	"iam-group": "IAM group list entry has no navigable cross-resource ARN fields",
	// Security — WAF web ACLs have no navigable ARN cross-references in list entry
	"waf": "WAF web ACL list entry has no navigable cross-resource ARN fields",
}

// TestResourceTypeDef_AllHaveNavigableFields verifies that every type returned by
// AllResourceTypes() has at least one NavigableField registered, OR is explicitly
// listed in noNavFieldsAllowList with a documented reason.
//
// Failure here means a new resource type was added without wiring navigable fields.
// Fix: either call resource.SetNavigableFieldsForTest in the type's aws/*.go init(),
// or add it to noNavFieldsAllowList with a rationale comment.
func TestResourceTypeDef_AllHaveNavigableFields(t *testing.T) {
	types := resource.AllResourceTypes()
	if len(types) == 0 {
		t.Fatal("AllResourceTypes() returned empty — registry not initialised (missing _ import?)")
	}

	for _, td := range types {
		fields := resource.GetNavigableFields(td.ShortName)
		if len(fields) > 0 {
			continue // registered — pass
		}
		if reason, ok := noNavFieldsAllowList[td.ShortName]; ok {
			_ = reason // documented exemption
			continue
		}
		t.Errorf("resource type %q has 0 NavigableFields and is not in the allow-list — "+
			"add SetNavigableFieldsForTest in internal/aws/<type>.go init() or document "+
			"why this type needs no navigable fields in noNavFieldsAllowList", td.ShortName)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T-INV-2: Enricher Rows population invariant
// ─────────────────────────────────────────────────────────────────────────────

// invRDSFake satisfies awsclient.RDSAPI for the invariant test (renamed to avoid
// collision with enrichRDSFake defined in enrichment_rds_findings_test.go).
type invRDSFake struct {
	awsclient.RDSAPI
	actions []rdstypes.ResourcePendingMaintenanceActions
}

func (f *invRDSFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rdssvc.DescribePendingMaintenanceActionsInput,
	_ ...func(*rdssvc.Options),
) (*rdssvc.DescribePendingMaintenanceActionsOutput, error) {
	return &rdssvc.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.actions,
	}, nil
}

// invEC2Fake satisfies awsclient.EC2API for the invariant test (renamed to avoid
// collision with ebsStatusFake).
type invEC2Fake struct {
	awsclient.EC2API
	volumeOutput *ec2.DescribeVolumeStatusOutput
}

func (f *invEC2Fake) DescribeVolumeStatus(
	_ context.Context,
	_ *ec2.DescribeVolumeStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeVolumeStatusOutput, error) {
	if f.volumeOutput != nil {
		return f.volumeOutput, nil
	}
	return &ec2.DescribeVolumeStatusOutput{}, nil
}

func (f *invEC2Fake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// invCodeBuildFake satisfies awsclient.CodeBuildAPI for the invariant test.
type invCodeBuildFake struct {
	awsclient.CodeBuildAPI
	projectBuilds map[string]string
	builds        map[string]cbtypes.Build
}

func (f *invCodeBuildFake) ListBuildsForProject(
	_ context.Context,
	params *codebuild.ListBuildsForProjectInput,
	_ ...func(*codebuild.Options),
) (*codebuild.ListBuildsForProjectOutput, error) {
	name := aws.ToString(params.ProjectName)
	if id, ok := f.projectBuilds[name]; ok {
		return &codebuild.ListBuildsForProjectOutput{Ids: []string{id}}, nil
	}
	return &codebuild.ListBuildsForProjectOutput{}, nil
}

func (f *invCodeBuildFake) BatchGetBuilds(
	_ context.Context,
	params *codebuild.BatchGetBuildsInput,
	_ ...func(*codebuild.Options),
) (*codebuild.BatchGetBuildsOutput, error) {
	var found []cbtypes.Build
	for _, id := range params.Ids {
		if b, ok := f.builds[id]; ok {
			found = append(found, b)
		}
	}
	return &codebuild.BatchGetBuildsOutput{Builds: found}, nil
}

// invELBv2Fake satisfies awsclient.ELBv2API for the invariant test.
type invELBv2Fake struct {
	awsclient.ELBv2API
	outputs map[string]*elbv2.DescribeTargetHealthOutput
}

func (f *invELBv2Fake) DescribeTargetHealth(
	_ context.Context,
	params *elbv2.DescribeTargetHealthInput,
	_ ...func(*elbv2.Options),
) (*elbv2.DescribeTargetHealthOutput, error) {
	arn := aws.ToString(params.TargetGroupArn)
	if out, ok := f.outputs[arn]; ok {
		return out, nil
	}
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

// invCodePipelineFake satisfies awsclient.CodePipelineAPI for the invariant test.
type invCodePipelineFake struct {
	awsclient.CodePipelineAPI
	states map[string]*codepipeline.GetPipelineStateOutput
}

func (f *invCodePipelineFake) GetPipelineState(
	_ context.Context,
	params *codepipeline.GetPipelineStateInput,
	_ ...func(*codepipeline.Options),
) (*codepipeline.GetPipelineStateOutput, error) {
	name := aws.ToString(params.Name)
	if out, ok := f.states[name]; ok {
		return out, nil
	}
	return &codepipeline.GetPipelineStateOutput{}, nil
}

// invSFNFake satisfies awsclient.SFNAPI for the invariant test.
type invSFNFake struct {
	awsclient.SFNAPI
	executions map[string]sfntypes.ExecutionStatus
}

func (f *invSFNFake) ListExecutions(
	_ context.Context,
	params *sfnsvc.ListExecutionsInput,
	_ ...func(*sfnsvc.Options),
) (*sfnsvc.ListExecutionsOutput, error) {
	arn := aws.ToString(params.StateMachineArn)
	if status, ok := f.executions[arn]; ok {
		return &sfnsvc.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{Status: status, StateMachineArn: &arn},
			},
		}, nil
	}
	return &sfnsvc.ListExecutionsOutput{}, nil
}

// invGlueFake satisfies awsclient.GlueAPI for the invariant test.
type invGlueFake struct {
	awsclient.GlueAPI
	jobRuns map[string]gluetypes.JobRunState
}

func (f *invGlueFake) GetJobRuns(
	_ context.Context,
	params *glusvc.GetJobRunsInput,
	_ ...func(*glusvc.Options),
) (*glusvc.GetJobRunsOutput, error) {
	name := aws.ToString(params.JobName)
	if state, ok := f.jobRuns[name]; ok {
		return &glusvc.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{JobName: &name, JobRunState: state},
			},
		}, nil
	}
	return &glusvc.GetJobRunsOutput{}, nil
}

// enricherInvariantCase pairs an enricher function with a pre-wired ServiceClients
// and a resource list that is guaranteed to produce at least one finding with Rows.
type enricherInvariantCase struct {
	name      string
	clients   *awsclient.ServiceClients
	resources []resource.Resource
	enrich    func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error)
}

// TestEnrichmentFinding_AllKeptEnrichersPopulateRows verifies that every enricher in
// the buildEnrichQueue order list populates at least one FindingRow when it detects
// a real issue.  An enricher that never produces rows makes the detail view render a
// finding header with no content — a silent rendering gap.
//
// The 7 enrichers tested are those in buildEnrichQueue's hardcoded order list:
//
//	["dbi", "ebs", "cb", "tg", "pipeline", "sfn", "glue"]
//
// Note: "rds" shares EnrichRDSDocDBMaintenance with "dbi" and is covered by the dbi case.
func TestEnrichmentFinding_AllKeptEnrichersPopulateRows(t *testing.T) {
	buildDate := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/inv-tg/abc"
	smARN := "arn:aws:states:us-east-1:000000000000:stateMachine:inv-sm"

	cases := []enricherInvariantCase{
		{
			name: "dbi (EnrichRDSDocDBMaintenance)",
			clients: &awsclient.ServiceClients{RDS: &invRDSFake{
				actions: []rdstypes.ResourcePendingMaintenanceActions{
					{
						ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:inv-db"),
						PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
							{Action: aws.String("system-update")},
						},
					},
				},
			}},
			resources: []resource.Resource{{ID: "inv-db"}},
			enrich:    awsclient.EnrichRDSDocDBMaintenance,
		},
		{
			name: "ebs (EnrichEBSVolumeStatus)",
			clients: &awsclient.ServiceClients{EC2: &invEC2Fake{
				volumeOutput: &ec2.DescribeVolumeStatusOutput{
					VolumeStatuses: []ec2types.VolumeStatusItem{
						{
							VolumeId: aws.String("vol-inv00000001"),
							VolumeStatus: &ec2types.VolumeStatusInfo{
								Status: "impaired",
							},
						},
					},
				},
			}},
			resources: []resource.Resource{{ID: "vol-inv00000001"}},
			enrich:    awsclient.EnrichEBSVolumeStatus,
		},
		{
			name: "cb (EnrichCodeBuildStatus)",
			clients: &awsclient.ServiceClients{CodeBuild: &invCodeBuildFake{
				projectBuilds: map[string]string{"inv-project": "inv-project:b001"},
				builds: map[string]cbtypes.Build{
					"inv-project:b001": {
						Id:          aws.String("inv-project:b001"),
						BuildStatus: cbtypes.StatusTypeFailed,
						EndTime:     &buildDate,
					},
				},
			}},
			resources: []resource.Resource{{ID: "inv-project"}},
			enrich:    awsclient.EnrichCodeBuildStatus,
		},
		{
			name: "tg (EnrichTargetGroupHealth)",
			clients: &awsclient.ServiceClients{ELBv2: &invELBv2Fake{
				outputs: map[string]*elbv2.DescribeTargetHealthOutput{
					tgARN: {
						TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
							{TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumUnhealthy}},
							{TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy}},
						},
					},
				},
			}},
			resources: []resource.Resource{{ID: "inv-tg", Fields: map[string]string{"target_group_arn": tgARN}}},
			enrich:    awsclient.EnrichTargetGroupHealth,
		},
		{
			name: "pipeline (EnrichCodePipelineStatus)",
			clients: &awsclient.ServiceClients{CodePipeline: &invCodePipelineFake{
				states: map[string]*codepipeline.GetPipelineStateOutput{
					"inv-pipeline": {
						StageStates: []cptypes.StageState{
							{
								StageName: aws.String("Deploy"),
								LatestExecution: &cptypes.StageExecution{
									Status: cptypes.StageExecutionStatusFailed,
								},
							},
						},
					},
				},
			}},
			resources: []resource.Resource{{ID: "inv-pipeline", Name: "inv-pipeline"}},
			enrich:    awsclient.EnrichCodePipelineStatus,
		},
		{
			name: "sfn (EnrichStepFunctionsStatus)",
			clients: &awsclient.ServiceClients{SFN: &invSFNFake{
				executions: map[string]sfntypes.ExecutionStatus{
					smARN: sfntypes.ExecutionStatusFailed,
				},
			}},
			resources: []resource.Resource{{ID: "inv-sm", Fields: map[string]string{"arn": smARN}}},
			enrich:    awsclient.EnrichStepFunctionsStatus,
		},
		{
			name: "glue (EnrichGlueJobStatus)",
			clients: &awsclient.ServiceClients{Glue: &invGlueFake{
				jobRuns: map[string]gluetypes.JobRunState{
					"inv-glue-job": gluetypes.JobRunStateFailed,
				},
			}},
			resources: []resource.Resource{{Name: "inv-glue-job"}},
			enrich:    awsclient.EnrichGlueJobStatus,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.enrich(context.Background(), tc.clients, tc.resources, nil)
			if err != nil {
				t.Fatalf("enricher returned unexpected error: %v", err)
			}
			if len(result.Findings) == 0 {
				t.Fatalf("enricher produced 0 findings — test setup must guarantee at least one issue")
			}
			for id := range result.Findings {
				if len(result.AttentionDetails[id].Rows) == 0 {
					t.Errorf("finding for resource %q has 0 Rows — enricher must populate at least one FindingRow", id)
				}
			}
		})
	}
}
