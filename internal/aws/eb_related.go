package aws

import (
	"context"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("eb", []resource.RelatedDef{
		{TargetType: "cfn", DisplayName: "CloudFormation Stack", Checker: checkEbCFN, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEbLogs, NeedsTargetCache: true},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEbASG, NeedsTargetCache: true},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEbEC2, NeedsTargetCache: true},
	})
}

// checkEbCFN checks the CFN cache for a stack associated with this EB environment.
// Pattern C: match by stack name prefix "awseb-{envID}".
func checkEbCFN(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	envID := ""
	if eb.EnvironmentId != nil {
		envID = *eb.EnvironmentId
	}
	if envID == "" {
		envID = res.ID
	}
	if envID == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	envIDPrefix := "awseb-" + envID
	expectedName := envIDPrefix + "-stack"

	cfnList, truncated, err := ebRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	var ids []string
	for _, cfnRes := range cfnList {
		name := cfnRes.Fields["stack_name"]
		if name == expectedName || cfnRes.ID == expectedName || cfnRes.Name == expectedName ||
			strings.HasPrefix(name, envIDPrefix) {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkEbLogs checks the log groups cache for groups associated with this EB environment.
// Pattern C: match by log group prefix "/aws/elasticbeanstalk/{envName}/".
func checkEbLogs(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	prefix := "/aws/elasticbeanstalk/" + envName + "/"

	logList, truncated, err := ebRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, prefix) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkEbASG checks the ASG cache for groups tagged with this EB environment name.
// Pattern C: match by "elasticbeanstalk:environment-name" tag on each ASG.
func checkEbASG(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := ebRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var ids []string
	for _, asgRes := range asgList {
		raw, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		for _, tag := range raw.Tags {
			if tag.Key != nil && *tag.Key == "elasticbeanstalk:environment-name" &&
				tag.Value != nil && *tag.Value == envName {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// checkEbEC2 scans the EC2 instance cache for instances tagged with this EB
// environment name via the "elasticbeanstalk:environment-name" tag.
// Pattern C: tag-based cache scan.
func checkEbEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	envName := ""
	if eb.EnvironmentName != nil {
		envName = *eb.EnvironmentName
	}
	if envName == "" {
		envName = res.Name
	}
	if envName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := ebRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		if tagValue(inst.Tags, "elasticbeanstalk:environment-name") == envName {
			ids = append(ids, ec2Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	return relatedResult("ec2", ids)
}

// ebRelatedResources returns the resource list for target from cache or fetches it.
func ebRelatedResources(ctx context.Context, clients interface{}, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
