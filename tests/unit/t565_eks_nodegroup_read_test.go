// t565_eks_nodegroup_read_test.go — the EKS EC2 Instances, AMIs and Auto
// Scaling Groups pivots read the cluster's managed node groups the same way,
// so one is exact whenever the others are.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// The session's node group list is partial (another cluster's group could not
// be described), while this cluster's own ListNodegroups/DescribeNodegroup
// reads answer in full: all three pivots are exact.
func TestRelated_EKS_NodePivotsShareOneNodegroupRead(t *testing.T) {
	w := selfManagedWorld()
	w.nodegroups = []*ekstypes.Nodegroup{{
		NodegroupName:  aws.String("general"),
		ClusterName:    aws.String("acme-prod"),
		Status:         ekstypes.NodegroupStatusActive,
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String("lt-0a1b2c3d4e5f60001"), Version: aws.String("1")},
		Resources: &ekstypes.NodegroupResources{
			AutoScalingGroups: []ekstypes.AutoScalingGroup{{Name: aws.String("eks-general-5ac9b2e4-0000-1111-2222-333344445555")}},
		},
	}}
	w.ltAMI = map[string]string{"lt-0a1b2c3d4e5f60001": "ami-0aa1bb2cc3dd4ee05"}
	w.asgMembers["eks-general-5ac9b2e4-0000-1111-2222-333344445555"] = []string{"i-0a1b2c3d4e5f60020"}

	cache := w.cache()
	entry := cache["ng"]
	entry.IsTruncated = true
	cache["ng"] = entry

	cluster := eksCluster("acme-prod")
	src := resource.Resource{ID: "acme-prod", Name: "acme-prod", Fields: map[string]string{}, RawStruct: cluster}
	for target, want := range map[string][]string{
		"ec2": {"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003", "i-0a1b2c3d4e5f60004", "i-0a1b2c3d4e5f60020"},
		"ami": {"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02", "ami-0aa1bb2cc3dd4ee05"},
		"asg": {"acme-prod-workers", "eks-general-5ac9b2e4-0000-1111-2222-333344445555"},
	} {
		r := eksCheckerByTarget(t, target)(context.Background(), w.clients(), src, cache)
		assertExactRelated(t, target, r, want)
	}
}
