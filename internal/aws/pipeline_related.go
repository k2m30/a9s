// pipeline_related.go contains CodePipeline pipeline related-resource checker functions.
//
// All pipeline→* checkers here use Pattern C: a single GetPipeline call per checker
// (wrapped in RetryOnThrottle) resolves the full stage/action structure for the
// pipeline. Action configuration maps (key-value strings) are mined for the relevant
// target resource identifiers based on the action Provider. Bare names (not ARNs) are
// used where possible since CodePipeline stores action targets by name.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// pipelineGetDeclaration wraps GetPipeline in RetryOnThrottle. Returns the declaration
// or nil on any error / unsupported client.
func pipelineGetDeclaration(ctx context.Context, clients any, pipelineName string) *cptypes.PipelineDeclaration {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CodePipeline == nil {
		return nil
	}
	api, ok := c.CodePipeline.(CodePipelineGetPipelineAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*codepipeline.GetPipelineOutput, error) {
		return api.GetPipeline(ctx, &codepipeline.GetPipelineInput{Name: &pipelineName})
	})
	if err != nil || out == nil || out.Pipeline == nil {
		return nil
	}
	return out.Pipeline
}

// pipelineActions iterates every action across every stage and invokes fn. The
// stage name is passed for context.
func pipelineActions(p *cptypes.PipelineDeclaration, fn func(stage string, action cptypes.ActionDeclaration)) {
	if p == nil {
		return
	}
	for _, stg := range p.Stages {
		name := ""
		if stg.Name != nil {
			name = *stg.Name
		}
		for _, a := range stg.Actions {
			fn(name, a)
		}
	}
}

// actionProvider returns the action Provider string (e.g. "CodeBuild", "CloudFormation",
// "ECS", "Lambda", "S3", "CodeDeploy", "ManualApproval", ...).
func actionProvider(a cptypes.ActionDeclaration) string {
	if a.ActionTypeId == nil || a.ActionTypeId.Provider == nil {
		return ""
	}
	return *a.ActionTypeId.Provider
}

// checkPipelineCB resolves CodeBuild projects referenced by this pipeline's actions.
// Action Provider=CodeBuild → Configuration["ProjectName"] holds the project name.
func checkPipelineCB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "CodeBuild" {
			return
		}
		if name := a.Configuration["ProjectName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("cb", mapKeys(seen))
}

// checkPipelineRole returns the pipeline's service role by extracting the role name
// from Pipeline.RoleArn. Pattern C: GetPipeline + ARN last-segment extraction.
func checkPipelineRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	var names []string
	if p.RoleArn != nil && *p.RoleArn != "" {
		names = append(names, arnRoleName(*p.RoleArn))
	}
	// Also include any per-action RoleArn overrides.
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if a.RoleArn != nil && *a.RoleArn != "" {
			if n := arnRoleName(*a.RoleArn); n != "" {
				names = append(names, n)
			}
		}
	})
	return relatedResult("role", names)
}

// checkPipelineCFN resolves CloudFormation stacks deployed by this pipeline.
// Provider=CloudFormation → Configuration["StackName"].
func checkPipelineCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "CloudFormation" {
			return
		}
		if name := a.Configuration["StackName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("cfn", mapKeys(seen))
}

// checkPipelineCodeartifact resolves CodeArtifact repositories referenced in source actions.
// Provider=CodeStarSourceConnection with Repository/Owner pointing at CodeCommit/GitHub
// is common; CodeArtifact as a direct Source provider is rare but possible.
// Configuration["RepositoryName"] is inspected for CodeArtifact providers.
func checkPipelineCodeartifact(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "codeartifact", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "CodeArtifact" {
			return
		}
		if name := a.Configuration["RepositoryName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("codeartifact", mapKeys(seen))
}

// checkPipelineECR resolves ECR repositories referenced as Source action inputs.
// Provider=ECR (source action) → Configuration["RepositoryName"].
func checkPipelineECR(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "ECR" {
			return
		}
		if name := a.Configuration["RepositoryName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("ecr", mapKeys(seen))
}

// checkPipelineECSSvc resolves ECS services deployed by this pipeline.
// Provider=ECS → Configuration["ServiceName"] (ClusterName is also present but
// the ecs-svc type is keyed by service name).
func checkPipelineECSSvc(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		prov := actionProvider(a)
		if prov != "ECS" && prov != "ECSBlueGreen" {
			return
		}
		if name := a.Configuration["ServiceName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("ecs-svc", mapKeys(seen))
}

// checkPipelineKMS resolves the artifact-store KMS key. Pipeline.ArtifactStore.EncryptionKey
// (or per-region ArtifactStores) carries the key ARN/alias.
func checkPipelineKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	seen := map[string]struct{}{}
	addKey := func(st *cptypes.ArtifactStore) {
		if st == nil || st.EncryptionKey == nil {
			return
		}
		if st.EncryptionKey.Id != nil && *st.EncryptionKey.Id != "" {
			seen[arnLastSegment(*st.EncryptionKey.Id)] = struct{}{}
		}
	}
	addKey(p.ArtifactStore)
	for _, st := range p.ArtifactStores {
		addKey(&st)
	}
	return relatedResult("kms", mapKeys(seen))
}

// checkPipelineLambda resolves Lambda functions invoked by Lambda deploy/invoke actions.
// Provider=Lambda → Configuration["FunctionName"].
func checkPipelineLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "Lambda" {
			return
		}
		if name := a.Configuration["FunctionName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("lambda", mapKeys(seen))
}

// checkPipelineS3 resolves the S3 artifact bucket(s) for this pipeline.
// Pipeline.ArtifactStore.Location and per-region ArtifactStores[].Location hold
// bucket names.
func checkPipelineS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	seen := map[string]struct{}{}
	addBucket := func(st *cptypes.ArtifactStore) {
		if st == nil || st.Location == nil || *st.Location == "" {
			return
		}
		seen[*st.Location] = struct{}{}
	}
	addBucket(p.ArtifactStore)
	for _, st := range p.ArtifactStores {
		addBucket(&st)
	}
	// Also include S3 deploy action buckets.
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "S3" {
			return
		}
		if name := a.Configuration["BucketName"]; name != "" {
			seen[name] = struct{}{}
		}
	})
	return relatedResult("s3", mapKeys(seen))
}

// checkPipelineSNS resolves SNS approval topics configured on Approval actions.
// Provider=Manual (Category=Approval) → Configuration["NotificationArn"].
func checkPipelineSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p := pipelineGetDeclaration(ctx, clients, res.ID)
	if p == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if arn := a.Configuration["NotificationArn"]; arn != "" {
			seen[arn] = struct{}{}
		}
	})
	return relatedResult("sns", mapKeys(seen))
}

// arnRoleName extracts the role name from an IAM role ARN
// (arn:aws:iam::acct:role/path/Name → Name), or returns the input as-is.
func arnRoleName(a string) string {
	if idx := strings.LastIndex(a, "/"); idx >= 0 && idx < len(a)-1 {
		return a[idx+1:]
	}
	return a
}

// mapKeys returns the keys of a map[string]struct{} as a slice (order-independent —
// relatedResult sorts for stability).
func mapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// checkPipelineEbRule resolves EventBridge rules that target this CodePipeline pipeline.
// Pattern C: one events:ListRuleNamesByTarget call using the pipeline ARN from
// res.Fields["arn"]. Count = len(RuleNames).
func checkPipelineEbRule(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	pipelineARN := res.Fields["arn"]
	if pipelineARN == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	api, ok := c.EventBridge.(EventBridgeListRuleNamesByTargetAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eventbridge.ListRuleNamesByTargetOutput, error) {
		return api.ListRuleNamesByTarget(ctx, &eventbridge.ListRuleNamesByTargetInput{TargetArn: &pipelineARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	return relatedResult("eb-rule", out.RuleNames)
}
