// aws_eks_related_nodegroup_paging_test.go — eks→ami and eks→ec2 over a
// cluster whose node groups do not fit in one ListNodegroups page, and over
// more ASG names than one DescribeAutoScalingGroups request accepts.
//
// A cluster large enough to page is exactly the cluster whose related panel
// matters; a single unpaged call silently reports the first page as the whole
// truth.
package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// pagedListNodegroups answers page one with pageOne plus a continuation
// token, and the token'd call with pageTwo. It records every NextToken it saw
// so a test can tell "never paged" from "paged wrongly".
func pagedListNodegroups(pageOne, pageTwo []string, seenTokens *[]string) func(*eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
	const token = "page-two-token"
	return func(in *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
		got := aws.ToString(in.NextToken)
		*seenTokens = append(*seenTokens, got)
		if got == "" {
			return &eks.ListNodegroupsOutput{Nodegroups: pageOne, NextToken: aws.String(token)}, nil
		}
		if got == token {
			return &eks.ListNodegroupsOutput{Nodegroups: pageTwo}, nil
		}
		return nil, fmt.Errorf("ListNodegroups called with an unexpected NextToken %q", got)
	}
}

func nodegroupWithLaunchTemplate(cluster, name, ltID string) *ekstypes.Nodegroup {
	return &ekstypes.Nodegroup{
		NodegroupName:  aws.String(name),
		ClusterName:    aws.String(cluster),
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String(ltID), Version: aws.String("1")},
	}
}

func nodegroupWithASG(cluster, name, asgName string) *ekstypes.Nodegroup {
	return &ekstypes.Nodegroup{
		NodegroupName: aws.String(name),
		ClusterName:   aws.String(cluster),
		Resources: &ekstypes.NodegroupResources{
			AutoScalingGroups: []ekstypes.AutoScalingGroup{{Name: aws.String(asgName)}},
		},
	}
}

func describeNodegroupFromMap(byName map[string]*ekstypes.Nodegroup) func(*eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	return func(in *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
		return &eks.DescribeNodegroupOutput{Nodegroup: byName[aws.ToString(in.NodegroupName)]}, nil
	}
}

// TestRelated_EKS_AMI_ReadsEveryNodegroupPage pins that the AMI of a node
// group listed on the second page is in the result.
func TestRelated_EKS_AMI_ReadsEveryNodegroupPage(t *testing.T) {
	const amiOne = "ami-0a1b2c3d4e5f60001"
	const amiTwo = "ami-0a1b2c3d4e5f60002"

	var tokens []string
	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: pagedListNodegroups([]string{"ng-a"}, []string{"ng-b"}, &tokens),
		describeNodegroupFn: describeNodegroupFromMap(map[string]*ekstypes.Nodegroup{
			"ng-a": nodegroupWithLaunchTemplate("acme-services", "ng-a", "lt-aaa"),
			"ng-b": nodegroupWithLaunchTemplate("acme-services", "ng-b", "lt-bbb"),
		}),
	}
	ec2Fake := &fakeEC2ForASG{
		describeLaunchTemplateVersionsFn: func(in *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			ami := amiOne
			if aws.ToString(in.LaunchTemplateId) == "lt-bbb" {
				ami = amiTwo
			}
			return &ec2.DescribeLaunchTemplateVersionsOutput{
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{ImageId: aws.String(ami)}},
				},
			}, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake, EC2: ec2Fake}
	result := eksCheckerByTarget(t, "ami")(context.Background(), clients, eksClusterSrcResource(), nil)

	if result.Err() != nil {
		t.Fatalf("unexpected Err: %v", result.Err())
	}
	if len(tokens) != 2 {
		t.Errorf("ListNodegroups called %d time(s) with tokens %v, want 2 (the second page was never read)", len(tokens), tokens)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, want := range []string{amiOne, amiTwo} {
		if !seen[want] {
			t.Errorf("AMI %q missing from ResourceIDs %v", want, result.ResourceIDs())
		}
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
}

// TestRelated_EKS_AMI_SinglePageDoesNotPageAgain is the negative half: a
// cluster whose node groups fit in one page makes exactly one call, so paging
// cannot turn into an unterminated loop.
func TestRelated_EKS_AMI_SinglePageDoesNotPageAgain(t *testing.T) {
	const ami = "ami-0a1b2c3d4e5f60001"

	calls := 0
	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: func(_ *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			calls++
			return &eks.ListNodegroupsOutput{Nodegroups: []string{"ng-only"}}, nil
		},
		describeNodegroupFn: describeNodegroupFromMap(map[string]*ekstypes.Nodegroup{
			"ng-only": nodegroupWithLaunchTemplate("acme-services", "ng-only", "lt-aaa"),
		}),
	}
	ec2Fake := &fakeEC2ForASG{
		describeLaunchTemplateVersionsFn: func(_ *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return &ec2.DescribeLaunchTemplateVersionsOutput{
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{ImageId: aws.String(ami)}},
				},
			}, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake, EC2: ec2Fake}
	result := eksCheckerByTarget(t, "ami")(context.Background(), clients, eksClusterSrcResource(), nil)

	if calls != 1 {
		t.Errorf("ListNodegroups called %d times, want 1 (no continuation token was returned)", calls)
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

// TestRelated_EKS_EC2_ReadsEveryNodegroupPage pins that instances belonging
// to a node group listed on the second page are in the result.
func TestRelated_EKS_EC2_ReadsEveryNodegroupPage(t *testing.T) {
	const instanceOne = "i-0a1b2c3d4e5f60001"
	const instanceTwo = "i-0a1b2c3d4e5f60002"

	var tokens []string
	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: pagedListNodegroups([]string{"ng-a"}, []string{"ng-b"}, &tokens),
		describeNodegroupFn: describeNodegroupFromMap(map[string]*ekstypes.Nodegroup{
			"ng-a": nodegroupWithASG("acme-services", "ng-a", "asg-a"),
			"ng-b": nodegroupWithASG("acme-services", "ng-b", "asg-b"),
		}),
	}
	asgFake := &fakeASGForNG{
		describeAutoScalingGroupsFn: func(in *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			out := &autoscaling.DescribeAutoScalingGroupsOutput{}
			for _, name := range in.AutoScalingGroupNames {
				id := instanceOne
				if name == "asg-b" {
					id = instanceTwo
				}
				out.AutoScalingGroups = append(out.AutoScalingGroups, asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String(name),
					Instances:            []asgtypes.Instance{{InstanceId: aws.String(id)}},
				})
			}
			return out, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake, AutoScaling: asgFake}
	result := eksCheckerByTarget(t, "ec2")(context.Background(), clients, eksClusterSrcResource(), nil)

	if result.Err() != nil {
		t.Fatalf("unexpected Err: %v", result.Err())
	}
	if len(tokens) != 2 {
		t.Errorf("ListNodegroups called %d time(s) with tokens %v, want 2 (the second page was never read)", len(tokens), tokens)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, want := range []string{instanceOne, instanceTwo} {
		if !seen[want] {
			t.Errorf("instance %q missing from ResourceIDs %v", want, result.ResourceIDs())
		}
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
}

// TestRelated_EKS_EC2_BatchesASGNamesUnderTheAPILimit pins that 51 ASG names
// go out as two DescribeAutoScalingGroups requests of at most 50 names each,
// and that every instance is collected. One request holding 51 names is
// refused by the API, which turns the whole EC2 row into an error.
func TestRelated_EKS_EC2_BatchesASGNamesUnderTheAPILimit(t *testing.T) {
	const total = 51
	const apiLimit = 50

	names := make([]string, 0, total)
	details := make(map[string]*ekstypes.Nodegroup, total)
	wantInstances := make(map[string]bool, total)
	for i := range total {
		ngName := fmt.Sprintf("ng-%02d", i)
		asgName := fmt.Sprintf("eks-asg-%02d", i)
		names = append(names, ngName)
		details[ngName] = nodegroupWithASG("acme-services", ngName, asgName)
		wantInstances[fmt.Sprintf("i-0a1b2c3d4e5f6%04d", i)] = true
	}

	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: func(_ *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
		},
		describeNodegroupFn: describeNodegroupFromMap(details),
	}

	var batchSizes []int
	asgFake := &fakeASGForNG{
		describeAutoScalingGroupsFn: func(in *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			batchSizes = append(batchSizes, len(in.AutoScalingGroupNames))
			if len(in.AutoScalingGroupNames) > apiLimit {
				return nil, fmt.Errorf("ValidationError: AutoScalingGroupNames holds %d names, the maximum is %d",
					len(in.AutoScalingGroupNames), apiLimit)
			}
			out := &autoscaling.DescribeAutoScalingGroupsOutput{}
			for _, name := range in.AutoScalingGroupNames {
				var idx int
				if _, err := fmt.Sscanf(name, "eks-asg-%d", &idx); err != nil {
					return nil, fmt.Errorf("unexpected ASG name %q: %w", name, err)
				}
				out.AutoScalingGroups = append(out.AutoScalingGroups, asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String(name),
					Instances:            []asgtypes.Instance{{InstanceId: aws.String(fmt.Sprintf("i-0a1b2c3d4e5f6%04d", idx))}},
				})
			}
			return out, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake, AutoScaling: asgFake}
	result := eksCheckerByTarget(t, "ec2")(context.Background(), clients, eksClusterSrcResource(), nil)

	if result.Err() != nil {
		t.Fatalf("unexpected Err: %v", result.Err())
	}
	if len(batchSizes) != 2 || batchSizes[0] != apiLimit || batchSizes[1] != total-apiLimit {
		t.Errorf("DescribeAutoScalingGroups batch sizes = %v, want [%d %d]", batchSizes, apiLimit, total-apiLimit)
	}
	if result.Count() != total {
		t.Errorf("Count = %d, want %d", result.Count(), total)
	}
	for _, id := range result.ResourceIDs() {
		delete(wantInstances, id)
	}
	if len(wantInstances) != 0 {
		t.Errorf("%d instance(s) missing from the result: %v", len(wantInstances), wantInstances)
	}
}

// TestRelated_EKS_EC2_ExactlyTheLimitIsOneRequest is the boundary's other
// side: 50 names fit in one request and must not be split.
func TestRelated_EKS_EC2_ExactlyTheLimitIsOneRequest(t *testing.T) {
	const total = 50

	names := make([]string, 0, total)
	details := make(map[string]*ekstypes.Nodegroup, total)
	for i := range total {
		ngName := fmt.Sprintf("ng-%02d", i)
		names = append(names, ngName)
		details[ngName] = nodegroupWithASG("acme-services", ngName, fmt.Sprintf("eks-asg-%02d", i))
	}

	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: func(_ *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
		},
		describeNodegroupFn: describeNodegroupFromMap(details),
	}

	var batchSizes []int
	asgFake := &fakeASGForNG{
		describeAutoScalingGroupsFn: func(in *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			batchSizes = append(batchSizes, len(in.AutoScalingGroupNames))
			out := &autoscaling.DescribeAutoScalingGroupsOutput{}
			for i, name := range in.AutoScalingGroupNames {
				out.AutoScalingGroups = append(out.AutoScalingGroups, asgtypes.AutoScalingGroup{
					AutoScalingGroupName: aws.String(name),
					Instances:            []asgtypes.Instance{{InstanceId: aws.String(fmt.Sprintf("i-0a1b2c3d4e5f6%04d", i))}},
				})
			}
			return out, nil
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake, AutoScaling: asgFake}
	result := eksCheckerByTarget(t, "ec2")(context.Background(), clients, eksClusterSrcResource(), nil)

	if len(batchSizes) != 1 || batchSizes[0] != total {
		t.Errorf("DescribeAutoScalingGroups batch sizes = %v, want [%d]", batchSizes, total)
	}
	if result.Count() != total {
		t.Errorf("Count = %d, want %d", result.Count(), total)
	}
}
