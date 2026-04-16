// eks_related.go contains EKS cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	// Cluster.ResourcesVpcConfig: VpcId, ClusterSecurityGroupId, SubnetIds, SecurityGroupIds
	// Cluster: RoleArn (IAM role for cluster operations)
	resource.RegisterNavigableFields("eks", []resource.NavigableField{
		{FieldPath: "ResourcesVpcConfig.VpcId", TargetType: "vpc"},
		{FieldPath: "ResourcesVpcConfig.ClusterSecurityGroupId", TargetType: "sg"},
		{FieldPath: "ResourcesVpcConfig.SubnetIds", TargetType: "subnet"},
		{FieldPath: "ResourcesVpcConfig.SecurityGroupIds", TargetType: "sg"},
		{FieldPath: "RoleArn", TargetType: "role"},
	})

	resource.RegisterRelated("eks", []resource.RelatedDef{
		{TargetType: "ng", DisplayName: "Node Groups", Checker: checkEKSNodeGroups, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEKSAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEKSCFN, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEKSLogs, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEKSSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkEKSVPC},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkEKSRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEKSKMS},
	})
}

// checkEKSNodeGroups checks the cache for node groups belonging to this EKS cluster.
func checkEKSNodeGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		clusterName = res.Fields["cluster_name"]
	}
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
	}

	ngList, truncated, err := eksRelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}

	var ids []string
	for _, ngRes := range ngList {
		rawNG, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		ngCluster := ngRes.Fields["cluster_name"]
		if ok && rawNG.ClusterName != nil {
			ngCluster = *rawNG.ClusterName
		}
		if ngCluster == clusterName {
			ids = append(ids, ngRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	return relatedResult("ng", ids)
}

// checkEKSAlarms checks the cache for CloudWatch alarms with ClusterName dimension matching this cluster.
func checkEKSAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := eksRelatedResources(ctx, clients, cache, "alarm")
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
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkEKSCFN checks the EKS cluster's tags for aws:cloudformation:stack-name and finds the matching CFN stack.
// EKS Cluster Tags is map[string]string (not a slice of Tag structs).
func checkEKSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ""
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if ok {
		stackName = raw.Tags["aws:cloudformation:stack-name"]
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := eksRelatedResources(ctx, clients, cache, "cfn")
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
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkEKSLogs searches the logs cache for the EKS control-plane log group.
// Pattern N — naming convention: /aws/eks/{cluster-name}/cluster
func checkEKSLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	expectedLogGroup := "/aws/eks/" + clusterName + "/cluster"

	logList, truncated, err := eksRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkEKSSG extracts security group IDs from the EKS Cluster's
// ResourcesVpcConfig (ClusterSecurityGroupId + SecurityGroupIds).
// Pattern F — no cache needed.
func checkEKSSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if raw.ResourcesVpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	var ids []string
	if raw.ResourcesVpcConfig.ClusterSecurityGroupId != nil && *raw.ResourcesVpcConfig.ClusterSecurityGroupId != "" {
		ids = append(ids, *raw.ResourcesVpcConfig.ClusterSecurityGroupId)
	}
	for _, sgID := range raw.ResourcesVpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkEKSVPC returns the VPC this EKS cluster runs in (Pattern R).
// Reads ResourcesVpcConfig.VpcId from the Cluster RawStruct.
func checkEKSVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if raw.ResourcesVpcConfig == nil || raw.ResourcesVpcConfig.VpcId == nil || *raw.ResourcesVpcConfig.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*raw.ResourcesVpcConfig.VpcId})
}

// checkEKSKMS extracts the KMS key ID from the EKS Cluster's EncryptionConfig.
// The KeyArn has the form arn:aws:kms::ACCOUNT:key/KEY-ID; the key ID is the
// last segment after "/". Pattern F — no cache needed.
func checkEKSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok || len(raw.EncryptionConfig) == 0 ||
		raw.EncryptionConfig[0].Provider == nil ||
		raw.EncryptionConfig[0].Provider.KeyArn == nil ||
		*raw.EncryptionConfig[0].Provider.KeyArn == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *raw.EncryptionConfig[0].Provider.KeyArn
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkEKSRole extracts the IAM role name from the EKS Cluster's RoleArn field.
// The RoleArn has the form arn:aws:iam::ACCOUNT:role/ROLE-NAME; the role name is
// the last segment after "/".
func checkEKSRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok || raw.RoleArn == nil || *raw.RoleArn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	arn := *raw.RoleArn
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
		return relatedResult("role", []string{arn[idx+1:]})
	}
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}








// eksRelatedResources returns the resource list for target from cache or by fetching the first page.
func eksRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
