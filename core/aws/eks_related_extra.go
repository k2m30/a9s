// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eks_related_extra.go — additional EKS related-resource checkers.
package aws

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingPkg "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEKSSubnet extracts subnet IDs from ResourcesVpcConfig.SubnetIds.
func checkEKSSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if cluster.ResourcesVpcConfig == nil {
		return foundNone("subnet", "cluster.ResourcesVpcConfig")
	}
	var ids []string
	for _, s := range cluster.ResourcesVpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkEKSASG resolves the cluster's Auto Scaling groups: those of its
// managed node groups, and the one each tagged node names in
// aws:autoscaling:groupName. With an EKS client the node groups are read as
// the EC2 and AMI pivots read them, so the three agree on completeness;
// without one, the loaded ng list is all there is to go on.
func checkEKSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return keyMissing("asg", "clusterName")
	}
	seen := make(map[string]struct{})
	nodes, partial, err := clusterTaggedNodes(ctx, clients, clusterName)
	for _, n := range nodes {
		if name := tagValue(n.Tags, "aws:autoscaling:groupName"); name != "" {
			seen[name] = struct{}{}
		}
	}

	var ngs []ekstypes.Nodegroup
	var ngPartial bool
	var ngErr error
	if c, ok := clients.(*ServiceClients); ok && c != nil && c.EKS != nil {
		ngs, ngPartial, ngErr = clusterNodegroups(ctx, c, clusterName)
	} else {
		ngList, truncated, listErr := relatedResourcesFor(ctx, clients, cache, "ng")
		if ngList == nil && listErr == nil && len(seen) == 0 {
			return NotRead("asg")
		}
		ngPartial, ngErr = truncated || ngList == nil, listErr
		for _, ngRes := range ngList {
			if ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct); ok && aws.ToString(ng.ClusterName) == clusterName {
				ngs = append(ngs, ng)
			}
		}
	}
	for _, name := range nodegroupASGNames(ngs) {
		seen[name] = struct{}{}
	}
	return nodeSourcesResult("asg", seen, partial || ngPartial, errors.Join(err, ngErr))
}

// clusterTaggedNodes returns the EC2 instances whose tags name clusterName —
// self-managed, Karpenter and Auto Mode nodes, and managed node group nodes
// too. Each tag is read as an exact key/value match:
//   - kubernetes.io/cluster/<name>=owned: propagated at launch by the
//     self-managed node group CloudFormation template (amazon-eks-nodegroup.yaml,
//     docs.aws.amazon.com/eks/latest/userguide/launch-workers.html) and a
//     Karpenter default tag (karpenter.sh/docs/concepts/nodeclasses/).
//   - eks:eks-cluster-name=<name>: a Karpenter default tag (same page), and the
//     tag EKS Auto Mode must set on every instance it launches — the Auto Mode
//     cluster role allows ec2:RunInstances only with
//     aws:RequestTag/eks:eks-cluster-name
//     (docs.aws.amazon.com/eks/latest/userguide/auto-cluster-iam-role.html).
//   - eks:cluster-name=<name>: set by EKS on managed node group instances next
//     to eks:nodegroup-name. The EKS user guide does not list it; it is read
//     from DescribeInstances on live accounts.
//
// partial is true when a query stopped at the page cap or was refused, or
// there is no EC2 client to ask; the nodes the other queries found are
// returned with it. err is set only when every query was refused.
// IncludeManagedResources lists Auto Mode instances, which EC2 hides from
// DescribeInstances by default for instances launched from 2026-04-22
// (docs.aws.amazon.com/eks/latest/userguide/automode-learn-instances.html).
func clusterTaggedNodes(ctx context.Context, clients any, clusterName string) (nodes []ec2types.Instance, partial bool, err error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return nil, true, nil
	}
	seen := make(map[string]struct{})
	tags := map[string]string{
		"kubernetes.io/cluster/" + clusterName: "owned",
		"eks:eks-cluster-name":                 clusterName,
		"eks:cluster-name":                     clusterName,
	}
	var refused []error
	for key, value := range tags {
		insts, complete, qErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.Instance, *string, error) {
			out, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:" + key), Values: []string{value}},
					{Name: aws.String("instance-state-name"), Values: liveInstanceStates()},
				},
				IncludeManagedResources: aws.Bool(true),
				NextToken:               token,
			})
			if err != nil {
				return nil, nil, err
			}
			var page []ec2types.Instance
			for _, r := range out.Reservations {
				page = append(page, r.Instances...)
			}
			return page, out.NextToken, nil
		})
		if qErr != nil {
			refused = append(refused, qErr)
		}
		partial = partial || !complete || qErr != nil
		for _, inst := range insts {
			id := aws.ToString(inst.InstanceId)
			if _, dup := seen[id]; id == "" || dup {
				continue
			}
			seen[id] = struct{}{}
			nodes = append(nodes, inst)
		}
	}
	if len(refused) == len(tags) {
		return nil, false, errors.Join(refused...)
	}
	return nodes, partial, nil
}

// nodeSourcesResult is the related row over every place a cluster's nodes are
// recorded. err is a source that could not be read at all: with nothing found
// elsewhere the row is that error, otherwise what was found is a lower bound.
func nodeSourcesResult(target string, ids map[string]struct{}, partial bool, err error) resource.RelatedCheckResult {
	if len(ids) == 0 && err != nil {
		return ReadFailed(target, err)
	}
	resolved, dropped := resolveRefs(target, slices.Collect(maps.Keys(ids)), domain.RefContext{})
	return relatedResultTrunc(target, resolved, partial || dropped || err != nil)
}

// listClusterNodegroups walks the ListNodegroups pages of one cluster.
// complete is false when the walk stopped at the page cap.
func listClusterNodegroups(ctx context.Context, api EKSAPI, clusterName string) (names []string, complete bool, err error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
		out, err := api.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
			NextToken:   token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Nodegroups, out.NextToken, nil
	})
}

// checkEKSAMI resolves the AMIs the cluster's nodes run: the ImageId of every
// tagged node, and for each managed node group with a launch template, the
// LaunchTemplateData.ImageId of its versions. A managed node group without a
// launch template records AmiType+ReleaseVersion only; its AMI is found
// through its nodes' tags.
func checkEKSAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("ami")
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return keyMissing("ami", "clusterName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return NotRead("ami")
	}

	amiSet := make(map[string]struct{})
	nodes, partial, nodesErr := clusterTaggedNodes(ctx, c, clusterName)
	for _, n := range nodes {
		if id := aws.ToString(n.ImageId); id != "" {
			amiSet[id] = struct{}{}
		}
	}
	ngs, ngPartial, ngErr := clusterNodegroups(ctx, c, clusterName)
	ltPartial, ltErr := nodegroupLaunchTemplateAMIs(ctx, c, ngs, amiSet)
	return nodeSourcesResult("ami", amiSet, partial || ngPartial || ltPartial, errors.Join(nodesErr, ngErr, ltErr))
}

// clusterNodegroups reads the cluster's managed node groups:
// ListNodegroups, then DescribeNodegroup for each. The three node pivots all
// read them here, so each is as complete as the others. partial is true when
// the list stopped at the page cap or some DescribeNodegroup was refused;
// err when the list or every DescribeNodegroup was refused.
func clusterNodegroups(ctx context.Context, c *ServiceClients, clusterName string) (ngs []ekstypes.Nodegroup, partial bool, err error) {
	ngNames, ngComplete, err := listClusterNodegroups(ctx, c.EKS, clusterName)
	if err != nil && len(ngNames) == 0 {
		return nil, false, err
	}
	var failures []Failure
	for _, ngName := range ngNames {
		descOut, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
			return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
		})
		if descErr != nil {
			failures = append(failures, FailedCall(ngName, descErr))
			continue
		}
		if descOut.Nodegroup != nil {
			ngs = append(ngs, *descOut.Nodegroup)
		}
	}
	aggErr := AggregateFailures("eks-related: DescribeNodegroup", failures, len(ngNames))
	if aggErr != nil && len(failures) == len(ngNames) {
		return nil, false, errors.Join(err, aggErr)
	}
	return ngs, aggErr != nil || !ngComplete || err != nil, err
}

// nodegroupASGNames lists the Auto Scaling groups behind the node groups.
func nodegroupASGNames(ngs []ekstypes.Nodegroup) []string {
	var names []string
	for _, ng := range ngs {
		if ng.Resources == nil {
			continue
		}
		for _, asg := range ng.Resources.AutoScalingGroups {
			if name := aws.ToString(asg.Name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// nodegroupLaunchTemplateAMIs adds the launch-template AMIs of the node
// groups to amiSet. partial is true when some launch template could not be
// read; err when every one read was refused.
func nodegroupLaunchTemplateAMIs(ctx context.Context, c *ServiceClients, ngs []ekstypes.Nodegroup, amiSet map[string]struct{}) (partial bool, err error) {
	var failures []Failure
	read := 0
	ec2API, ec2OK := serviceClient(c, func(c *ServiceClients) EC2API { return c.EC2 })
	for _, ng := range ngs {
		if ng.LaunchTemplate == nil || aws.ToString(ng.LaunchTemplate.Id) == "" {
			continue
		}
		if !ec2OK {
			partial = true
			continue
		}
		read++
		versions, ltErr := launchTemplateVersions(ctx, ec2API, ng.LaunchTemplate.Id, ng.LaunchTemplate.Version)
		if ltErr != nil {
			// Soft-skip when the launch template has been deleted upstream:
			// AWS returns InvalidLaunchTemplateId.NotFound, which is a true
			// "no AMI to relate to" rather than a fetch failure.
			if !ErrCodeIs(ltErr, "InvalidLaunchTemplateId.NotFound") {
				failures = append(failures, FailedCall(aws.ToString(ng.NodegroupName)+"/lt", ltErr))
			}
			continue
		}
		for _, v := range versions {
			if v.LaunchTemplateData != nil && aws.ToString(v.LaunchTemplateData.ImageId) != "" {
				amiSet[*v.LaunchTemplateData.ImageId] = struct{}{}
			}
		}
	}
	aggErr := AggregateFailures("eks-related: DescribeLaunchTemplateVersions", failures, read)
	if aggErr != nil && len(failures) == read {
		return partial, aggErr
	}
	return partial || aggErr != nil, nil
}

// asgNamesPerDescribe is the maximum number of names DescribeAutoScalingGroups
// accepts in one request; a longer list is refused outright, which would turn a
// large cluster's whole EC2 row into an error.
const asgNamesPerDescribe = 50

// checkEKSEC2 resolves the cluster's EC2 nodes: every tagged node, and the
// members of each managed node group's Auto Scaling groups.
func checkEKSEC2(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("ec2")
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return keyMissing("ec2", "clusterName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return NotRead("ec2")
	}

	seen := make(map[string]struct{})
	nodes, partial, nodesErr := clusterTaggedNodes(ctx, c, clusterName)
	for _, n := range nodes {
		seen[aws.ToString(n.InstanceId)] = struct{}{}
	}
	ngs, ngPartial, ngErr := clusterNodegroups(ctx, c, clusterName)
	asgPartial, asgErr := asgInstances(ctx, c, nodegroupASGNames(ngs), seen)
	return nodeSourcesResult("ec2", seen, partial || ngPartial || asgPartial, errors.Join(nodesErr, ngErr, asgErr))
}

// asgInstances adds the members of the named Auto Scaling groups to seen.
// partial is true when an ASG page could not be read or there is no
// AutoScaling client; err when an ASG read was refused.
func asgInstances(ctx context.Context, c *ServiceClients, asgNames []string, seen map[string]struct{}) (partial bool, err error) {
	if len(asgNames) == 0 {
		return false, nil
	}
	if c.AutoScaling == nil {
		return true, nil
	}

	for batch := range slices.Chunk(asgNames, asgNamesPerDescribe) {
		asgs, batchComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]asgtypes.AutoScalingGroup, *string, error) {
			out, err := c.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscalingPkg.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: batch,
				NextToken:             token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.AutoScalingGroups, out.NextToken, nil
		})
		partial = partial || !batchComplete
		for _, asg := range asgs {
			for _, id := range asgMembers(asg) {
				seen[id] = struct{}{}
			}
		}
		if err != nil {
			return true, err
		}
	}
	return partial, nil
}
