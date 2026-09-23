// aws_eks_related_tagged_nodes_test.go — the EKS EC2 Instances, AMIs and
// Auto Scaling Groups pivots over a cluster whose nodes are not all in managed
// node groups.
//
// Self-managed nodes and Karpenter nodes are plain EC2 instances: EKS records
// no node group for them, and the only link AWS keeps between such an instance
// and its cluster is the instance's tags (kubernetes.io/cluster/<name>=owned,
// eks:cluster-name=<name>). A count built from node groups alone reports those
// clusters as having no instances, which is a proven zero nobody read.
package unit_test

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fakeEC2TaggedInstances answers DescribeInstances the way EC2 does: every
// filter must hold (AND), any value within a filter may match (OR), tag values
// and keys accept the * and ? wildcards, and InstanceIds narrows the set.
// Every other EC2API method is the empty stub of fakeEC2ForASG.
// refuseTagKey makes every query that filters on that tag key fail, the way
// a policy condition on the requested tag refuses one query among several.
type fakeEC2TaggedInstances struct {
	*fakeEC2ForASG
	instances    []ec2types.Instance
	err          error
	refuseTagKey string
}

func (f *fakeEC2TaggedInstances) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.refuseTagKey != "" {
		for _, flt := range in.Filters {
			name := aws.ToString(flt.Name)
			if name == "tag:"+f.refuseTagKey || (name == "tag-key" && slices.Contains(flt.Values, f.refuseTagKey)) {
				return nil, &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."}
			}
		}
	}
	var matched []ec2types.Instance
	for _, inst := range f.instances {
		if len(in.InstanceIds) > 0 && !slices.Contains(in.InstanceIds, aws.ToString(inst.InstanceId)) {
			continue
		}
		ok, err := instanceMatchesFilters(inst, in.Filters)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, inst)
		}
	}
	out := &ec2.DescribeInstancesOutput{}
	for _, inst := range matched {
		out.Reservations = append(out.Reservations, ec2types.Reservation{
			ReservationId: aws.String("r-" + strings.TrimPrefix(aws.ToString(inst.InstanceId), "i-")),
			OwnerId:       aws.String("123456789012"),
			Instances:     []ec2types.Instance{inst},
		})
	}
	return out, nil
}

func instanceMatchesFilters(inst ec2types.Instance, filters []ec2types.Filter) (bool, error) {
	for _, f := range filters {
		name := aws.ToString(f.Name)
		switch {
		case strings.HasPrefix(name, "tag:"):
			key := strings.TrimPrefix(name, "tag:")
			hit := false
			for _, tag := range inst.Tags {
				if aws.ToString(tag.Key) == key && anyGlob(f.Values, aws.ToString(tag.Value)) {
					hit = true
				}
			}
			if !hit {
				return false, nil
			}
		case name == "tag-key":
			hit := false
			for _, tag := range inst.Tags {
				if anyGlob(f.Values, aws.ToString(tag.Key)) {
					hit = true
				}
			}
			if !hit {
				return false, nil
			}
		case name == "instance-state-name":
			if inst.State == nil || !anyGlob(f.Values, string(inst.State.Name)) {
				return false, nil
			}
		case name == "instance-id":
			if !anyGlob(f.Values, aws.ToString(inst.InstanceId)) {
				return false, nil
			}
		default:
			return false, fmt.Errorf("fake DescribeInstances: filter %q is not modelled", name)
		}
	}
	return true, nil
}

func anyGlob(patterns []string, s string) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(p, s); ok {
			return true
		}
	}
	return false
}

func eksNode(id, ami string, tags map[string]string) ec2types.Instance {
	inst := ec2types.Instance{
		InstanceId:     aws.String(id),
		ImageId:        aws.String(ami),
		InstanceType:   ec2types.InstanceTypeM6iLarge,
		PrivateDnsName: aws.String("ip-10-0-1-10.ec2.internal"),
		State:          &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning, Code: aws.Int32(16)},
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		inst.Tags = append(inst.Tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(tags[k])})
	}
	return inst
}

// eksNodeWorld is one account's EKS/EC2/Auto Scaling state as the three
// pivots read it: the cluster's managed node groups (each backed by one ASG
// and one launch template) and every EC2 instance in the Region.
type eksNodeWorld struct {
	nodegroups   []*ekstypes.Nodegroup
	ltAMI        map[string]string
	asgMembers   map[string][]string
	instances    []ec2types.Instance
	describeErr  error
	refuseTagKey string
}

func (w eksNodeWorld) clients() *awsclient.ServiceClients {
	byName := map[string]*ekstypes.Nodegroup{}
	var names []string
	for _, ng := range w.nodegroups {
		byName[aws.ToString(ng.NodegroupName)] = ng
		names = append(names, aws.ToString(ng.NodegroupName))
	}
	eksFake := &fakeEKSForAMIEC2{
		listNodegroupsFn: func(*eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
		},
		describeNodegroupFn: describeNodegroupFromMap(byName),
	}
	ec2Fake := &fakeEC2TaggedInstances{
		fakeEC2ForASG: &fakeEC2ForASG{
			describeLaunchTemplateVersionsFn: func(in *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
				ami, ok := w.ltAMI[aws.ToString(in.LaunchTemplateId)]
				if !ok {
					return nil, &smithy.GenericAPIError{Code: "InvalidLaunchTemplateId.NotFound", Message: "not found"}
				}
				return &ec2.DescribeLaunchTemplateVersionsOutput{
					LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
						{LaunchTemplateId: in.LaunchTemplateId, VersionNumber: aws.Int64(1), LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{ImageId: aws.String(ami)}},
					},
				}, nil
			},
		},
		instances:    w.instances,
		err:          w.describeErr,
		refuseTagKey: w.refuseTagKey,
	}
	asgFake := &fakeASGForNG{
		describeAutoScalingGroupsFn: func(in *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			out := &autoscaling.DescribeAutoScalingGroupsOutput{}
			for _, name := range in.AutoScalingGroupNames {
				members, ok := w.asgMembers[name]
				if !ok {
					continue
				}
				g := asgtypes.AutoScalingGroup{AutoScalingGroupName: aws.String(name)}
				for _, id := range members {
					g.Instances = append(g.Instances, asgtypes.Instance{InstanceId: aws.String(id), LifecycleState: asgtypes.LifecycleStateInService})
				}
				out.AutoScalingGroups = append(out.AutoScalingGroups, g)
			}
			return out, nil
		},
	}
	return &awsclient.ServiceClients{EKS: eksFake, EC2: ec2Fake, AutoScaling: asgFake}
}

// cache is what the related executor seeds for the pivots that declare a
// target cache: the loaded node group list.
func (w eksNodeWorld) cache() resource.ResourceCache {
	var ngs []resource.Resource
	for _, ng := range w.nodegroups {
		ngs = append(ngs, resource.Resource{
			ID:        aws.ToString(ng.NodegroupName),
			Name:      aws.ToString(ng.NodegroupName),
			RawStruct: *ng,
		})
	}
	return resource.ResourceCache{"ng": resource.ResourceCacheEntry{Resources: ngs}}
}

func (w eksNodeWorld) check(t *testing.T, cluster *ekstypes.Cluster, target string) resource.RelatedCheckResult {
	t.Helper()
	src := resource.Resource{ID: aws.ToString(cluster.Name), Name: aws.ToString(cluster.Name), Fields: map[string]string{}, RawStruct: cluster}
	return eksCheckerByTarget(t, target)(context.Background(), w.clients(), src, w.cache())
}

func eksCluster(name string) *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:    aws.String(name),
		Arn:     aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
		Status:  ekstypes.ClusterStatusActive,
		Version: aws.String("1.31"),
	}
}

func isProvenZero(r resource.RelatedCheckResult) bool {
	return r.Err() == nil && r.EffectiveState() == domain.RelatedResolved &&
		r.Coverage() == domain.CoverageComplete && r.Count() == 0
}

func assertExactRelated(t *testing.T, target string, r resource.RelatedCheckResult, want []string) {
	t.Helper()
	if r.Err() != nil {
		t.Errorf("%s: Err = %v, want a resolved count", target, r.Err())
		return
	}
	got := slices.Clone(r.ResourceIDs())
	slices.Sort(got)
	want = slices.Clone(want)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s: ResourceIDs = %v, want %v", target, got, want)
	}
	if r.Count() != len(want) {
		t.Errorf("%s: Count = %d, want %d", target, r.Count(), len(want))
	}
	if r.EffectiveState() != domain.RelatedResolved || r.Truncated() {
		t.Errorf("%s: state %v truncated=%v, want a complete resolved count", target, r.EffectiveState(), r.Truncated())
	}
}

// assertLowerBound pins "found these, more may exist": every id is present and
// the count is marked as a lower bound.
func assertLowerBound(t *testing.T, target string, r resource.RelatedCheckResult, mustContain []string) {
	t.Helper()
	if r.Err() != nil || r.EffectiveState() != domain.RelatedResolved {
		t.Errorf("%s: state %v err %v, want a resolved lower bound carrying what was read", target, r.EffectiveState(), r.Err())
		return
	}
	for _, id := range mustContain {
		if !slices.Contains(r.ResourceIDs(), id) {
			t.Errorf("%s: ResourceIDs %v is missing %q", target, r.ResourceIDs(), id)
		}
	}
	if !r.Truncated() {
		t.Errorf("%s: Truncated = false, want a lower bound (a node source could not be read)", target)
	}
}

const (
	clusterTagPrefix = "kubernetes.io/cluster/"
	asgNameTag       = "aws:autoscaling:groupName"
)

// selfManagedWorld is a cluster with no managed node groups: two nodes in a
// self-managed ASG (eksctl / CloudFormation node template), one Karpenter
// node, and one node carrying only eks:cluster-name. The other instances
// belong elsewhere: a second cluster, a cluster whose name merely starts with
// this one's, and a bastion.
func selfManagedWorld() eksNodeWorld {
	return eksNodeWorld{
		instances: []ec2types.Instance{
			eksNode("i-0a1b2c3d4e5f60001", "ami-0aa1bb2cc3dd4ee01", map[string]string{
				clusterTagPrefix + "acme-prod": "owned",
				asgNameTag:                     "acme-prod-workers",
				"Name":                         "acme-prod-workers-node",
			}),
			eksNode("i-0a1b2c3d4e5f60002", "ami-0aa1bb2cc3dd4ee01", map[string]string{
				clusterTagPrefix + "acme-prod": "owned",
				asgNameTag:                     "acme-prod-workers",
				"Name":                         "acme-prod-workers-node",
			}),
			eksNode("i-0a1b2c3d4e5f60003", "ami-0aa1bb2cc3dd4ee02", map[string]string{
				clusterTagPrefix + "acme-prod":   "owned",
				"eks:eks-cluster-name":           "acme-prod",
				"karpenter.sh/nodepool":          "default",
				"karpenter.sh/nodeclaim":         "default-x7k2p",
				"karpenter.k8s.aws/ec2nodeclass": "default",
			}),
			eksNode("i-0a1b2c3d4e5f60004", "ami-0aa1bb2cc3dd4ee01", map[string]string{
				"eks:cluster-name": "acme-prod",
			}),
			eksNode("i-0a1b2c3d4e5f60010", "ami-0aa1bb2cc3dd4ee09", map[string]string{
				clusterTagPrefix + "acme-staging": "owned",
				"eks:cluster-name":                "acme-staging",
				asgNameTag:                        "acme-staging-workers",
			}),
			eksNode("i-0a1b2c3d4e5f60011", "ami-0aa1bb2cc3dd4ee08", map[string]string{
				clusterTagPrefix + "acme-prod-v2": "owned",
				asgNameTag:                        "acme-prod-v2-workers",
			}),
			eksNode("i-0a1b2c3d4e5f60012", "ami-0aa1bb2cc3dd4ee07", map[string]string{
				"Name": "acme-bastion",
			}),
		},
		asgMembers: map[string][]string{
			"acme-prod-workers":    {"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002"},
			"acme-staging-workers": {"i-0a1b2c3d4e5f60010"},
			"acme-prod-v2-workers": {"i-0a1b2c3d4e5f60011"},
		},
	}
}

// A cluster whose nodes are self-managed or Karpenter-launched has them as
// tagged EC2 instances; the three pivots count exactly those instances, their
// images, and the ASG their aws:autoscaling:groupName tag names — never an
// instance of another cluster, including one whose name shares a prefix.
func TestRelated_EKS_NodesFoundByTag_WithoutManagedNodegroups(t *testing.T) {
	w := selfManagedWorld()
	cluster := eksCluster("acme-prod")

	assertExactRelated(t, "ec2", w.check(t, cluster, "ec2"),
		[]string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003", "i-0a1b2c3d4e5f60004"})
	assertExactRelated(t, "ami", w.check(t, cluster, "ami"),
		[]string{"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02"})
	assertExactRelated(t, "asg", w.check(t, cluster, "asg"),
		[]string{"acme-prod-workers"})
}

// The tagged nodes join the managed node groups' own; a managed node carries
// the same cluster tags and is counted once.
func TestRelated_EKS_TaggedNodesJoinManagedNodegroups(t *testing.T) {
	w := selfManagedWorld()
	w.nodegroups = []*ekstypes.Nodegroup{{
		NodegroupName:  aws.String("general"),
		ClusterName:    aws.String("acme-prod"),
		NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/general/5ac9b2e4-0000-1111-2222-333344445555"),
		Status:         ekstypes.NodegroupStatusActive,
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String("lt-0a1b2c3d4e5f60001"), Version: aws.String("1")},
		Resources: &ekstypes.NodegroupResources{
			AutoScalingGroups: []ekstypes.AutoScalingGroup{{Name: aws.String("eks-general-5ac9b2e4-0000-1111-2222-333344445555")}},
		},
	}}
	w.ltAMI = map[string]string{"lt-0a1b2c3d4e5f60001": "ami-0aa1bb2cc3dd4ee05"}
	w.asgMembers["eks-general-5ac9b2e4-0000-1111-2222-333344445555"] = []string{"i-0a1b2c3d4e5f60020"}
	w.instances = append(w.instances, eksNode("i-0a1b2c3d4e5f60020", "ami-0aa1bb2cc3dd4ee05", map[string]string{
		clusterTagPrefix + "acme-prod": "owned",
		"eks:cluster-name":             "acme-prod",
		"eks:nodegroup-name":           "general",
		asgNameTag:                     "eks-general-5ac9b2e4-0000-1111-2222-333344445555",
	}))
	cluster := eksCluster("acme-prod")

	assertExactRelated(t, "ec2", w.check(t, cluster, "ec2"),
		[]string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003", "i-0a1b2c3d4e5f60004", "i-0a1b2c3d4e5f60020"})
	assertExactRelated(t, "ami", w.check(t, cluster, "ami"),
		[]string{"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02", "ami-0aa1bb2cc3dd4ee05"})
	assertExactRelated(t, "asg", w.check(t, cluster, "asg"),
		[]string{"acme-prod-workers", "eks-general-5ac9b2e4-0000-1111-2222-333344445555"})
}

// A cluster with neither node groups nor tagged instances has read every
// place a node is recorded and found none: a proven zero on all three rows.
func TestRelated_EKS_NoNodesAnywhere_ProvenZero(t *testing.T) {
	w := selfManagedWorld()
	cluster := eksCluster("acme-empty")

	for _, target := range []string{"ec2", "ami", "asg"} {
		r := w.check(t, cluster, target)
		if !isProvenZero(r) {
			t.Errorf("%s: state %v count %d truncated %v err %v ids %v, want a proven zero",
				target, r.EffectiveState(), r.Count(), r.Truncated(), r.Err(), r.ResourceIDs())
		}
	}
}

// When the tagged-instance read is refused, the self-managed and Karpenter
// nodes were never looked at: a cluster without node groups is not a proven
// zero, and one with node groups shows what they hold as a lower bound.
func TestRelated_EKS_TaggedInstanceReadFails_NeverProvenZero(t *testing.T) {
	refused := &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."}

	t.Run("no managed node groups", func(t *testing.T) {
		w := selfManagedWorld()
		w.describeErr = refused
		cluster := eksCluster("acme-prod")
		for _, target := range []string{"ec2", "ami", "asg"} {
			r := w.check(t, cluster, target)
			if isProvenZero(r) {
				t.Errorf("%s: reported a proven zero although the tagged-instance read failed", target)
			}
		}
	})

	t.Run("with a managed node group", func(t *testing.T) {
		w := selfManagedWorld()
		w.describeErr = refused
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
		cluster := eksCluster("acme-prod")

		assertLowerBound(t, "ec2", w.check(t, cluster, "ec2"), []string{"i-0a1b2c3d4e5f60020"})
		assertLowerBound(t, "ami", w.check(t, cluster, "ami"), []string{"ami-0aa1bb2cc3dd4ee05"})
		assertLowerBound(t, "asg", w.check(t, cluster, "asg"), []string{"eks-general-5ac9b2e4-0000-1111-2222-333344445555"})
	})
}

// EKS Auto Mode launches its nodes itself and tags them eks:eks-cluster-name,
// eks:kubernetes-node-class-name and eks:kubernetes-node-pool-name
// (docs.aws.amazon.com/eks/latest/userguide/automode-learn-instances.html).
// Either the pivot matches that tag and counts the node, or it cannot tell
// such a node apart and its count for an Auto Mode cluster is a lower bound —
// never a complete count that leaves the node out.
func TestRelated_EKS_AutoModeNode_CountedOrLowerBound(t *testing.T) {
	w := eksNodeWorld{instances: []ec2types.Instance{
		eksNode("i-0a1b2c3d4e5f60030", "ami-0aa1bb2cc3dd4ee06", map[string]string{
			"eks:eks-cluster-name":           "acme-auto",
			"eks:kubernetes-node-class-name": "default",
			"eks:kubernetes-node-pool-name":  "general-purpose",
		}),
	}}
	cluster := eksCluster("acme-auto")
	cluster.ComputeConfig = &ekstypes.ComputeConfigResponse{
		Enabled:     aws.Bool(true),
		NodePools:   []string{"general-purpose", "system"},
		NodeRoleArn: aws.String("arn:aws:iam::123456789012:role/AmazonEKSAutoNodeRole"),
	}

	for target, id := range map[string]string{"ec2": "i-0a1b2c3d4e5f60030", "ami": "ami-0aa1bb2cc3dd4ee06"} {
		r := w.check(t, cluster, target)
		if slices.Contains(r.ResourceIDs(), id) {
			continue
		}
		if r.Err() == nil && r.EffectiveState() == domain.RelatedResolved && !r.Truncated() {
			t.Errorf("%s: complete count %d (%v) leaves out the Auto Mode node %s; want it counted or the count marked a lower bound",
				target, r.Count(), r.ResourceIDs(), id)
		}
	}
}

// When one tag query fails and the others answer, the nodes the answering
// queries found are kept and the count is a lower bound — whichever query
// failed, so the answer does not depend on the order the queries run in.
func TestRelated_EKS_OneTagQueryFails_KeepsTheOthersAsLowerBound(t *testing.T) {
	cases := []struct {
		refused string
		ec2     []string
		ami     []string
		asg     []string
	}{
		{
			refused: clusterTagPrefix + "acme-prod",
			ec2:     []string{"i-0a1b2c3d4e5f60003", "i-0a1b2c3d4e5f60004"},
			ami:     []string{"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02"},
		},
		{
			refused: "eks:eks-cluster-name",
			ec2:     []string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003", "i-0a1b2c3d4e5f60004"},
			ami:     []string{"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02"},
			asg:     []string{"acme-prod-workers"},
		},
		{
			refused: "eks:cluster-name",
			ec2:     []string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003"},
			ami:     []string{"ami-0aa1bb2cc3dd4ee01", "ami-0aa1bb2cc3dd4ee02"},
			asg:     []string{"acme-prod-workers"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.refused, func(t *testing.T) {
			w := selfManagedWorld()
			w.refuseTagKey = tc.refused
			cluster := eksCluster("acme-prod")
			for target, want := range map[string][]string{"ec2": tc.ec2, "ami": tc.ami, "asg": tc.asg} {
				r := w.check(t, cluster, target)
				assertLowerBound(t, target, r, want)
				for _, id := range r.ResourceIDs() {
					if !slices.Contains(want, id) {
						t.Errorf("%s: ResourceIDs %v holds %q, which no answering query found", target, r.ResourceIDs(), id)
					}
				}
			}
		})
	}
}

// The EKS Auto Scaling Groups row reads the cluster's node groups and tagged
// instances; the loaded ASG list is not one of its sources. Its answer is the
// same whether that list is loaded, truncated, or absent, and the pivot does
// not ask the executor to load it, so a refused ASG list cannot turn the row
// unknown.
func TestRelated_EKS_ASG_DoesNotDependOnASGList(t *testing.T) {
	for _, def := range resource.GetRelated("eks") {
		if def.TargetType == "asg" && def.NeedsTargetCache {
			t.Errorf("eks→asg declares NeedsTargetCache: the executor loads the ASG list first, and a refused load makes the row unknown although the checker never reads it")
		}
	}

	w := selfManagedWorld()
	cluster := eksCluster("acme-prod")
	src := resource.Resource{ID: "acme-prod", Name: "acme-prod", Fields: map[string]string{}, RawStruct: cluster}
	checker := eksCheckerByTarget(t, "asg")

	base := checker(context.Background(), w.clients(), src, w.cache())
	assertExactRelated(t, "asg (no ASG list)", base, []string{"acme-prod-workers"})

	for name, entry := range map[string]resource.ResourceCacheEntry{
		"truncated empty": {Resources: []resource.Resource{}, IsTruncated: true},
		"unrelated groups": {Resources: []resource.Resource{
			{ID: "acme-batch-spot", Name: "acme-batch-spot", Fields: map[string]string{}},
		}},
	} {
		cache := w.cache()
		cache["asg"] = entry
		r := checker(context.Background(), w.clients(), src, cache)
		if !slices.Equal(r.ResourceIDs(), base.ResourceIDs()) || r.Truncated() != base.Truncated() || r.EffectiveState() != base.EffectiveState() {
			t.Errorf("with the ASG list %s: ids %v truncated %v state %v, want the answer without it: ids %v truncated %v state %v",
				name, r.ResourceIDs(), r.Truncated(), r.EffectiveState(), base.ResourceIDs(), base.Truncated(), base.EffectiveState())
		}
	}
}
