// ecs_related.go contains ECS cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSServices checks the cache for ECS services belonging to this cluster.
func checkECSServices(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}

	clusterArn := ""
	raw, ok := assertStruct[ecstypes.Cluster](res.RawStruct)
	if ok && raw.ClusterArn != nil {
		clusterArn = *raw.ClusterArn
	}

	svcList, truncated, err := ecsRelatedResources(ctx, clients, cache, "ecs-svc")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1, Err: err}
	}
	if svcList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
	}

	var ids []string
	for _, svcRes := range svcList {
		rawSvc, svcOk := assertStruct[ecstypes.Service](svcRes.RawStruct)
		if svcOk && rawSvc.ClusterArn != nil {
			arnVal := *rawSvc.ClusterArn
			if (clusterArn != "" && arnVal == clusterArn) || strings.HasSuffix(arnVal, "/"+clusterName) {
				ids = append(ids, svcRes.ID)
				continue
			}
		}
		if svcRes.Fields["cluster"] == clusterName {
			ids = append(ids, svcRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ecs-svc")
	}
	return relatedResult("ecs-svc", ids)
}

// checkECSAlarms checks the cache for CloudWatch alarms with ClusterName dimension matching this cluster.
func checkECSAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := ecsRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "ClusterName" && d.Value != nil && *d.Value == clusterName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkECSCFN checks the ECS cluster's tags for aws:cloudformation:stack-name and finds the matching CFN stack.
func checkECSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ""
	raw, ok := assertStruct[ecstypes.Cluster](res.RawStruct)
	if ok {
		for _, tag := range raw.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
				stackName = *tag.Value
				break
			}
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := ecsRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
}

// checkECSKMS extracts the KMS key from the ECS Cluster's
// Configuration.ExecuteCommandConfiguration.KmsKeyId field.
// Returns the key ID (last segment after "/"). Pattern F — no cache needed.
func checkECSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ecstypes.Cluster](res.RawStruct)
	if !ok || cluster.Configuration == nil ||
		cluster.Configuration.ExecuteCommandConfiguration == nil ||
		cluster.Configuration.ExecuteCommandConfiguration.KmsKeyId == nil ||
		*cluster.Configuration.ExecuteCommandConfiguration.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *cluster.Configuration.ExecuteCommandConfiguration.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// ecsRelatedResources returns the resource list for target from cache or by fetching the first page.
func ecsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
