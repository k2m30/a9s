// ec2_related.go contains EC2 related-resource checker functions and shared helpers.
package aws

import (
	"context"
	"sort"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// assertStruct extracts a value of type T from an interface that may hold
// either T or *T. Used for RawStruct type assertions across related checkers.
func assertStruct[T any](v any) (T, bool) {
	if val, ok := v.(T); ok {
		return val, true
	}
	if p, ok := v.(*T); ok && p != nil {
		return *p, true
	}
	var zero T
	return zero, false
}

// checkEC2TargetGroups checks the cache for target groups referencing this EC2 instance.
func checkEC2TargetGroups(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, vpcID, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}
	tgList, truncated, err := ec2RelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		targetType := tgRes.Fields["target_type"]
		tgVpcID := tgRes.Fields["vpc_id"]
		if ok {
			targetType = string(raw.TargetType)
			if raw.VpcId != nil {
				tgVpcID = *raw.VpcId
			}
		}
		if targetType != "instance" {
			continue
		}
		// Without target-health rows in cache, best available approximation is
		// VPC-level matching for instance target groups.
		if tgVpcID == vpcID {
			ids = append(ids, tgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	return relatedResult("tg", ids)
}

// checkEC2ASG checks the cache for ASGs containing this EC2 instance.
func checkEC2ASG(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	asgList, truncated, err := ec2RelatedResources(ctx, clients, cache, "asg")
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
		for _, inst := range raw.Instances {
			if inst.InstanceId != nil && *inst.InstanceId == instanceID {
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

// checkEC2Alarms checks the cache for CloudWatch alarms targeting this EC2 instance.
func checkEC2Alarms(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, truncated, err := ec2RelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Value != nil && *d.Value == instanceID {
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

// checkEC2CFN checks instance tags for aws:cloudformation:stack-name.
func checkEC2CFN(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_, _, stackName := ec2Identity(res)
	cfnList, truncated, err := ec2RelatedResources(ctx, clients, cache, "cfn")
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
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkEC2EIP checks the cache for Elastic IPs associated with this EC2 instance.
func checkEC2EIP(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	eipList, truncated, err := ec2RelatedResources(ctx, clients, cache, "eip")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1, Err: err}
	}
	if eipList == nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	var ids []string
	for _, eipRes := range eipList {
		raw, ok := assertStruct[ec2types.Address](eipRes.RawStruct)
		if ok && raw.InstanceId != nil && *raw.InstanceId == instanceID {
			ids = append(ids, eipRes.ID)
			continue
		}
		if eipRes.Fields["instance_id"] == instanceID {
			ids = append(ids, eipRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	return relatedResult("eip", ids)
}

func checkEC2EBS(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids := ec2VolumeIDs(res)
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	return relatedResult("ebs", ordered)
}

// checkEC2NodeGroups checks for EKS node groups associated with this EC2 instance.
// When the node group list is not loaded yet, we prefer a zero count over an
// "unknown" placeholder so the detail view stays stable.
func checkEC2NodeGroups(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
	}
	tags := ec2Tags(res)
	clusterName := tags["eks:cluster-name"]
	nodegroupName := tags["eks:nodegroup-name"]
	if clusterName == "" && nodegroupName == "" {
		return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
	}
	ngList, truncated, err := ec2RelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	var ids []string
	for _, ngRes := range ngList {
		raw, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		rawClusterName := ngRes.Fields["cluster_name"]
		rawNodegroupName := ngRes.Fields["nodegroup_name"]
		if ok {
			if raw.ClusterName != nil {
				rawClusterName = *raw.ClusterName
			}
			if raw.NodegroupName != nil {
				rawNodegroupName = *raw.NodegroupName
			}
		}
		if clusterName != "" && rawClusterName != "" && clusterName != rawClusterName {
			continue
		}
		if nodegroupName != "" && rawNodegroupName != "" && nodegroupName != rawNodegroupName {
			continue
		}
		if rawNodegroupName != "" {
			ids = append(ids, ngRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	return relatedResult("ng", ids)
}

// checkEC2CloudTrailEvents checks cached CloudTrail events for references to the
// instance. When ct-events has not been loaded we degrade to zero, not unknown.
func checkEC2CloudTrailEvents(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	eventList, truncated, err := ec2RelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if eventList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	var ids []string
	for _, eventRes := range eventList {
		raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct)
		if ok {
			if cloudTrailEventMentionsInstance(raw, instanceID) {
				ids = append(ids, eventRes.ID)
			}
			continue
		}
		if eventRes.Fields["resource_name"] == instanceID {
			ids = append(ids, eventRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

// checkEC2EBSSnap checks the cache for EBS snapshots belonging to this EC2 instance.
func checkEC2EBSSnap(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volumeIDs := ec2VolumeIDs(res)
	if len(volumeIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: 0}
	}
	snapList, truncated, err := ec2RelatedResources(ctx, clients, cache, "ebs-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}
	var ids []string
	for _, snapRes := range snapList {
		raw, ok := assertStruct[ec2types.Snapshot](snapRes.RawStruct)
		volumeID := snapRes.Fields["volume_id"]
		if ok && raw.VolumeId != nil {
			volumeID = *raw.VolumeId
		}
		if _, found := volumeIDs[volumeID]; found {
			ids = append(ids, snapRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}
	return relatedResult("ebs-snap", ids)
}

// ec2RelatedResources returns the resource list for target from cache or by
// fetching the first page. Returns (resources, isTruncated, error).
// isTruncated=true means the list is partial; callers should return Count=-1
// when 0 matches are found in a truncated list.
func ec2RelatedResources(ctx context.Context, clients interface{}, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := cache[target]; ok {
		return entry.Resources, entry.IsTruncated, nil
	}
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	// When AWS clients are not initialized (nil or wrong type), the registered
	// paginated fetchers return "AWS clients not initialized". Treat this as a
	// graceful no-op (no resources available) rather than a hard error, preserving
	// the same semantics as the old nil-client early-return.
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

func relatedResult(target string, ids []string) resource.RelatedCheckResult {
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: target, Count: 0}
	}
	set := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		uniq = append(uniq, id)
	}
	sort.Strings(uniq)
	return resource.RelatedCheckResult{
		TargetType:  target,
		Count:       len(uniq),
		ResourceIDs: uniq,
	}
}

func ec2Identity(res resource.Resource) (instanceID, vpcID, stackName string) {
	instanceID = res.ID
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return instanceID, res.Fields["vpc_id"], ""
	}
	if raw.InstanceId != nil {
		instanceID = *raw.InstanceId
	}
	if raw.VpcId != nil {
		vpcID = *raw.VpcId
	}
	for _, tag := range raw.Tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		if *tag.Key == "aws:cloudformation:stack-name" {
			stackName = *tag.Value
			break
		}
	}
	return instanceID, vpcID, stackName
}

func ec2Tags(res resource.Resource) map[string]string {
	tags := map[string]string{}
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return tags
	}
	for _, tag := range raw.Tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		tags[*tag.Key] = *tag.Value
	}
	return tags
}

func cloudTrailEventMentionsInstance(event cloudtrailtypes.Event, instanceID string) bool {
	for _, rr := range event.Resources {
		if rr.ResourceName != nil && *rr.ResourceName == instanceID {
			return true
		}
	}
	return false
}

func ec2VolumeIDs(res resource.Resource) map[string]struct{} {
	ids := map[string]struct{}{}
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return ids
	}
	for _, bdm := range raw.BlockDeviceMappings {
		if bdm.Ebs == nil || bdm.Ebs.VolumeId == nil {
			continue
		}
		ids[*bdm.Ebs.VolumeId] = struct{}{}
	}
	return ids
}

