// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// pipeline_related.go contains CodePipeline pipeline related-resource checker functions.
//
// All pipeline→* checkers here make a single GetPipeline call per checker
// (wrapped in RetryOnThrottle) resolves the full stage/action structure for the
// pipeline. Action configuration maps (key-value strings) are mined for the relevant
// target resource identifiers based on the action Provider. Bare names (not ARNs) are
// used where possible since CodePipeline stores action targets by name.
package aws

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// errPipelineNotConfigured means no usable CodePipeline client was available
// to attempt the call at all.
var errPipelineNotConfigured = errors.New("codepipeline client not configured")

// pipelineGetDeclaration wraps GetPipeline in RetryOnThrottle. A nil error
// means the declaration was resolved; any non-nil error means it was not, and
// the caller must not treat that the same as a definitive "not found". Per
// docs/related-resources.md, the error must be
// classified — never collapsed wholesale — via pipelineRelatedOnErr:
// errPipelineNotConfigured means no CodePipeline client was even wired to
// attempt the call (structurally can't look, ever — UnknownRelated, still
// navigable); any other error means GetPipeline was actually attempted and
// failed (AccessDenied, exhausted RetryOnThrottle retries, or a malformed
// empty response) — a real operational failure that must surface via
// ErrorRelated (Flash + the "!" error log), never a silent Unknown. A checker
// looping over many pipelines must instead count failures across the loop:
// if every lookup in the loop failed, nothing could be determined at all; if
// only some failed, the count found so far is partial, not exact
// (relatedResultTrunc with truncated=true).
func pipelineGetDeclaration(ctx context.Context, clients any, pipelineName string) (*cptypes.PipelineDeclaration, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CodePipeline == nil {
		return nil, errPipelineNotConfigured
	}
	api, ok := c.CodePipeline.(CodePipelineGetPipelineAPI)
	if !ok {
		return nil, errPipelineNotConfigured
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*codepipeline.GetPipelineOutput, error) {
		return api.GetPipeline(ctx, &codepipeline.GetPipelineInput{Name: &pipelineName})
	})
	if err != nil {
		return nil, err
	}
	if out == nil || out.Pipeline == nil {
		return nil, errors.New("codepipeline GetPipeline returned no pipeline declaration")
	}
	return out.Pipeline, nil
}

// pipelineRelatedOnErr classifies a pipelineGetDeclaration failure into the
// correct RelatedCheckResult, per the error rule in
// docs/related-resources.md: errPipelineNotConfigured is the one
// structural "we never even attempted the call" case — no client was wired,
// retrying changes nothing — and stays UnknownRelated. Every other error
// means GetPipeline was actually invoked and failed (AccessDenied, exhausted
// throttle retries, or a malformed empty response), which is a real failure
// the operator must be able to see and retry (ErrorRelated: surfaced via
// Flash + the "!" error log, never silently collapsed to Unknown).
func pipelineRelatedOnErr(targetType string, err error) resource.RelatedCheckResult {
	if errors.Is(err, errPipelineNotConfigured) {
		return NotRead(targetType)
	}
	return ReadFailed(targetType, err)
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
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("cb", err)
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
	return relatedResultTrunc("cb", mapKeys(seen), false)
}

// checkPipelineRole returns the pipeline's service role (Pipeline.RoleArn) and
// any per-action role overrides.
func checkPipelineRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("role", err)
	}
	var names []string
	if p.RoleArn != nil && *p.RoleArn != "" {
		names = append(names, *p.RoleArn)
	}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if a.RoleArn != nil {
			names = append(names, *a.RoleArn)
		}
	})
	return relatedRefs("role", names, refContext(clients, cache, "role"))
}

// checkPipelineCFN resolves CloudFormation stacks deployed by this pipeline.
// Provider=CloudFormation → Configuration["StackName"].
func checkPipelineCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("cfn", err)
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
	return relatedResultTrunc("cfn", mapKeys(seen), false)
}

// checkPipelineECR resolves ECR repositories referenced as Source action inputs.
// Provider=ECR (source action) → Configuration["RepositoryName"].
func checkPipelineECR(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("ecr", err)
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
	return relatedResultTrunc("ecr", mapKeys(seen), false)
}

// checkPipelineECSSvc resolves ECS services deployed by this pipeline.
// Provider=ECS → Configuration["ServiceName"], which names a service inside
// the deploy action's own ClusterName. A blue/green action (provider
// CodeDeployToECS) names a CodeDeploy application and deployment group
// (https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-ECSbluegreen.html),
// and the service is on the deployment group, which a9s does not read: the
// answer is short by it.
func checkPipelineECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("ecs-svc", err)
	}
	seen := map[string]struct{}{}
	blueGreen := false
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		switch actionProvider(a) {
		case "CodeDeployToECS":
			blueGreen = true
		case "ECS":
			if name := a.Configuration["ServiceName"]; name != "" {
				seen[ecsSvcID(a.Configuration["ClusterName"], name)] = struct{}{}
			}
		}
	})
	ids, dropped := resolveRefs("ecs-svc", mapKeys(seen), refContext(clients, cache, "ecs-svc"))
	return relatedAnswer("ecs-svc", relatedRead{ids: ids, partial: dropped, unread: blueGreen})
}

// checkPipelineKMS resolves the artifact-store KMS key. Pipeline.ArtifactStore.EncryptionKey
// (or per-region ArtifactStores) carries the key ARN/alias.
func checkPipelineKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("kms", err)
	}
	var refs []string
	addKey := func(st *cptypes.ArtifactStore) {
		if st != nil && st.EncryptionKey != nil && st.EncryptionKey.Id != nil {
			refs = append(refs, *st.EncryptionKey.Id)
		}
	}
	addKey(p.ArtifactStore)
	for _, st := range p.ArtifactStores {
		addKey(&st)
	}
	return kmsRelated(ctx, clients, cache, refs)
}

// checkPipelineLambda resolves Lambda functions invoked by Lambda deploy/invoke actions.
// Provider=Lambda → Configuration["FunctionName"].
func checkPipelineLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("lambda", err)
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
	return relatedResultTrunc("lambda", mapKeys(seen), false)
}

// checkPipelineS3 resolves the S3 artifact bucket(s) for this pipeline.
// Pipeline.ArtifactStore.Location and per-region ArtifactStores[].Location hold
// bucket names.
func checkPipelineS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("s3", err)
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
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if actionProvider(a) != "S3" {
			return
		}
		// A deploy action names its target under BucketName; a source action
		// names the bucket it polls under S3Bucket.
		for _, key := range []string{"BucketName", "S3Bucket"} {
			if name := a.Configuration[key]; name != "" {
				seen[name] = struct{}{}
			}
		}
	})
	return relatedResultTrunc("s3", mapKeys(seen), false)
}

// checkPipelineSNS resolves SNS approval topics configured on Approval actions.
// Provider=Manual (Category=Approval) → Configuration["NotificationArn"].
func checkPipelineSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	p, err := pipelineGetDeclaration(ctx, clients, res.ID)
	if err != nil {
		return pipelineRelatedOnErr("sns", err)
	}
	seen := map[string]struct{}{}
	pipelineActions(p, func(_ string, a cptypes.ActionDeclaration) {
		if arn := a.Configuration["NotificationArn"]; arn != "" {
			seen[arn] = struct{}{}
		}
	})
	return relatedResultTrunc("sns", mapKeys(seen), false)
}

// mapKeys returns the keys of a map[string]struct{} as a slice (order-independent —
// relatedResult sorts for stability).
func mapKeys(m map[string]struct{}) []string {
	return slices.Collect(maps.Keys(m))
}

// checkPipelineEbRule resolves EventBridge rules that target this CodePipeline pipeline.
// One events:ListRuleNamesByTarget call using the pipeline ARN from
// res.Fields["arn"]. Count = len(RuleNames).
func checkPipelineEbRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRulesTargeting(ctx, clients, cache, res.Fields["arn"])
}
