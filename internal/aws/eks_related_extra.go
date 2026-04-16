// eks_related_extra.go — additional EKS related-resource checkers.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEKSSubnet extracts subnet IDs from ResourcesVpcConfig.SubnetIds.
func checkEKSSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if cluster.ResourcesVpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	var ids []string
	for _, s := range cluster.ResourcesVpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResult("subnet", ids)
}

// checkEKSACM returns Count:0 — ACM cert associations aren't in the Cluster struct.
func checkEKSACM(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
}

// checkEKSAMI returns Count:0 — AMI is on NodeGroup's launch template, not cluster.
func checkEKSAMI(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
}

// checkEKSASG — ASGs are owned by NodeGroups; derive by scanning ng cache.
func checkEKSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	ngList, truncated, err := eksRelatedResourcesExtra(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	seen := make(map[string]struct{})
	for _, ngRes := range ngList {
		ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		if !ok {
			continue
		}
		if ng.ClusterName == nil || *ng.ClusterName != clusterName {
			continue
		}
		if ng.Resources != nil {
			for _, a := range ng.Resources.AutoScalingGroups {
				if a.Name != nil && *a.Name != "" {
					seen[*a.Name] = struct{}{}
				}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// checkEKSEC2 — EC2 instances backing this cluster's node groups. Not
// cache-resolvable without launching-instance awareness.
func checkEKSEC2(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
}

// checkEKSECR — image repos used by pods; not in Cluster struct.
func checkEKSECR(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
}

// checkEKSIAMUser — cluster's access entries / aws-auth configmap; not in Cluster list.
func checkEKSIAMUser(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
}

// checkEKSCTEvents scans ct-events for events involving this cluster.
func checkEKSCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := eksRelatedResourcesExtra(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, clusterName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

// eksRelatedResourcesExtra — companion helper so we don't duplicate the
// pattern from eks_related.go.
func eksRelatedResourcesExtra(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
