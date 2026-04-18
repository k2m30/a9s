// redshift_related.go contains Redshift Cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("redshift", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedshiftAlarms, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedshiftSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedshiftVPC},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkRedshiftRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedshiftKMS},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedshiftCFN, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedshiftSecrets, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedshiftLogs},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkRedshiftS3},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedshiftSubnet},
	})

	// redshifttypes.Cluster: VpcId
	resource.RegisterNavigableFields("redshift", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
}

// checkRedshiftAlarms checks the cache for CloudWatch alarms with ClusterIdentifier dimension matching this cluster.
func checkRedshiftAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := redshiftRelatedResources(ctx, clients, cache, "alarm")
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
			if d.Name != nil && *d.Name == "ClusterIdentifier" && d.Value != nil && *d.Value == clusterID {
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

// checkRedshiftSG extracts security group IDs from the Redshift Cluster's
// VpcSecurityGroups slice.
// Pattern F — no cache needed.
func checkRedshiftSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, vsg := range cluster.VpcSecurityGroups {
		if vsg.VpcSecurityGroupId != nil && *vsg.VpcSecurityGroupId != "" {
			ids = append(ids, *vsg.VpcSecurityGroupId)
		}
	}
	return relatedResult("sg", ids)
}

// checkRedshiftVPC returns the VPC this Redshift cluster runs in (Pattern R).
// Reads Cluster.VpcId from the RawStruct.
func checkRedshiftVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if cluster.VpcId == nil || *cluster.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*cluster.VpcId})
}

// checkRedshiftRole extracts IAM role ARNs from the Redshift Cluster's IamRoles slice.
// Each ClusterIamRole has an IamRoleArn field; we extract the role name (last segment after "/").
// Pattern F — no cache needed.
func checkRedshiftRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if len(cluster.IamRoles) == 0 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	var ids []string
	for _, r := range cluster.IamRoles {
		if r.IamRoleArn == nil || *r.IamRoleArn == "" {
			continue
		}
		arn := *r.IamRoleArn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			ids = append(ids, arn[idx+1:])
		} else {
			ids = append(ids, arn)
		}
	}
	return relatedResult("role", ids)
}

// checkRedshiftKMS extracts the KMS key ID from the Redshift Cluster's KmsKeyId
// field. Pattern F — no cache needed.
func checkRedshiftKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok || cluster.KmsKeyId == nil || *cluster.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *cluster.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkRedshiftCFN checks the Cluster's Tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache.
func checkRedshiftCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := ""
	for _, tag := range cluster.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := redshiftRelatedResources(ctx, clients, cache, "cfn")
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
		raw, rawOK := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if rawOK && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cfn")
	}
	return relatedResult("cfn", ids)
}

// checkRedshiftSecrets resolves the admin-credentials secret managed for this
// Redshift cluster. Cluster.MasterPasswordSecretArn holds the full secret
// ARN; we match it against the secrets cache by ARN.
func checkRedshiftSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if cluster.MasterPasswordSecretArn == nil || *cluster.MasterPasswordSecretArn == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	secretARN := *cluster.MasterPasswordSecretArn

	secretList, truncated, err := redshiftRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	if secretList == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}

	var ids []string
	for _, secretRes := range secretList {
		if secretRes.Fields["arn"] == secretARN {
			ids = append(ids, secretRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("secrets")
	}
	return relatedResult("secrets", ids)
}

// checkRedshiftLogs resolves the cluster's audit-log target via a single
// redshift:DescribeLoggingStatus call (Pattern C). When LogDestinationType
// is cloudwatch, LogExports names the log-class subtypes and the actual log
// group follows the /aws/redshift/cluster/{clusterID} naming convention.
func checkRedshiftLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	status := redshiftLoggingStatus(ctx, clients, res)
	if status == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	if status.LoggingEnabled == nil || !*status.LoggingEnabled {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	if status.LogDestinationType != redshifttypes.LogDestinationTypeCloudwatch {
		// S3-only audit logging — no log group association.
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	// CloudWatch audit logs follow convention /aws/redshift/cluster/{clusterID}/{logtype}
	// We return the parent log-group prefix for display; the UI can scope.
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return relatedResult("logs", []string{"/aws/redshift/cluster/" + clusterID})
}

// checkRedshiftS3 resolves the audit-log S3 bucket via a single
// redshift:DescribeLoggingStatus call (Pattern C). BucketName is set only
// when the cluster logs to S3 (not CloudWatch).
func checkRedshiftS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	status := redshiftLoggingStatus(ctx, clients, res)
	if status == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	if status.LoggingEnabled == nil || !*status.LoggingEnabled {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	if status.BucketName == nil || *status.BucketName == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return relatedResult("s3", []string{*status.BucketName})
}

// checkRedshiftSubnet resolves the cluster's subnet-group members via a
// single redshift:DescribeClusterSubnetGroups call (Pattern C).
func checkRedshiftSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterSubnetGroupName == nil || *cluster.ClusterSubnetGroupName == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Redshift == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	name := *cluster.ClusterSubnetGroupName
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeClusterSubnetGroupsOutput, error) {
		return c.Redshift.DescribeClusterSubnetGroups(ctx, &redshift.DescribeClusterSubnetGroupsInput{
			ClusterSubnetGroupName: &name,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1, Err: err}
	}
	if out == nil || len(out.ClusterSubnetGroups) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	var ids []string
	for _, sn := range out.ClusterSubnetGroups[0].Subnets {
		if sn.SubnetIdentifier != nil && *sn.SubnetIdentifier != "" {
			ids = append(ids, *sn.SubnetIdentifier)
		}
	}
	return relatedResult("subnet", ids)
}

// redshiftLoggingStatus performs a single DescribeLoggingStatus call for this
// cluster's identifier, wrapped in RetryOnThrottle.
func redshiftLoggingStatus(ctx context.Context, clients any, res resource.Resource) *redshift.DescribeLoggingStatusOutput {
	clusterID := res.ID
	if clusterID == "" {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Redshift == nil {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeLoggingStatusOutput, error) {
		return c.Redshift.DescribeLoggingStatus(ctx, &redshift.DescribeLoggingStatusInput{
			ClusterIdentifier: &clusterID,
		})
	})
	if err != nil {
		return nil
	}
	return out
}

// redshiftRelatedResources returns the resource list for target from cache or by fetching the first page.
func redshiftRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
