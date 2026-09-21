package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// describeVolumesStub answers DescribeVolumes with one fixed page.
type describeVolumesStub struct{ volumes []ec2types.Volume }

func (s *describeVolumesStub) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{Volumes: s.volumes}, nil
}

func ec2RelatedChecker(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == target {
			return def.Checker
		}
	}
	t.Fatalf("ec2 registers no %s pivot", target)
	return nil
}

// ebsRowFor runs the real EBS fetcher so the row carries the fields the
// volume's attachment produces, rather than a hand-written approximation.
func ebsRowFor(t *testing.T, vol ec2types.Volume) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchEBSVolumesPage(context.Background(), &describeVolumesStub{volumes: []ec2types.Volume{vol}}, "")
	if err != nil {
		t.Fatalf("FetchEBSVolumesPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("fetched %d rows, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

// TestRelatedOneFact_MultiAttachVolumeNamesEveryInstance pins that a volume
// names every instance it is attached to. EBS Multi-Attach attaches a single
// io1/io2 volume to up to 16 Nitro instances concurrently, which is why
// Volume carries MultiAttachEnabled beside a plural Attachments list; each of
// those instances carries the volume in its own BlockDeviceMappings, so the
// attachment is one fact readable from either end.
func TestRelatedOneFact_MultiAttachVolumeNamesEveryInstance(t *testing.T) {
	const volID = "vol-0a1b2c3d4e5f60001"
	instances := []string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003"}

	vol := ec2types.Volume{
		VolumeId:           aws.String(volID),
		State:              ec2types.VolumeStateInUse,
		VolumeType:         ec2types.VolumeTypeIo2,
		Size:               aws.Int32(500),
		Iops:               aws.Int32(10000),
		AvailabilityZone:   aws.String("us-east-1a"),
		MultiAttachEnabled: aws.Bool(true),
	}
	for _, id := range instances {
		vol.Attachments = append(vol.Attachments, ec2types.VolumeAttachment{
			VolumeId:   aws.String(volID),
			InstanceId: aws.String(id),
			Device:     aws.String("/dev/xvdf"),
			State:      ec2types.VolumeAttachmentStateAttached,
		})
	}

	row := ebsRowFor(t, vol)
	got := ebsCheckerByTarget(t, "ec2")(context.Background(), nil, row, resource.ResourceCache{}).ResourceIDs()
	slices.Sort(got)
	if !slices.Equal(got, instances) {
		t.Errorf("volume %s lists %v, want %v: a Multi-Attach volume is attached to all of them at once", volID, got, instances)
	}

	for _, id := range instances {
		inst := resource.Resource{
			ID: id,
			RawStruct: ec2types.Instance{
				InstanceId: aws.String(id),
				BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
					{DeviceName: aws.String("/dev/xvdf"), Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String(volID)}},
				},
			},
		}
		ids := ec2RelatedChecker(t, "ebs")(context.Background(), nil, inst, resource.ResourceCache{}).ResourceIDs()
		if !slices.Equal(ids, []string{volID}) {
			t.Errorf("instance %s lists %v, want [%s]", id, ids, volID)
		}
	}
}

// TestRelatedOneFact_SingleAttachVolumeNamesOneInstance pins that reading
// every attachment does not invent one: a volume attached to a single
// instance names that instance and no other.
func TestRelatedOneFact_SingleAttachVolumeNamesOneInstance(t *testing.T) {
	const volID, instID = "vol-0a1b2c3d4e5f60009", "i-0a1b2c3d4e5f60009"
	row := ebsRowFor(t, ec2types.Volume{
		VolumeId:         aws.String(volID),
		State:            ec2types.VolumeStateInUse,
		VolumeType:       ec2types.VolumeTypeGp3,
		Size:             aws.Int32(20),
		AvailabilityZone: aws.String("us-east-1a"),
		Attachments: []ec2types.VolumeAttachment{
			{VolumeId: aws.String(volID), InstanceId: aws.String(instID), State: ec2types.VolumeAttachmentStateAttached},
		},
	})
	got := ebsCheckerByTarget(t, "ec2")(context.Background(), nil, row, resource.ResourceCache{}).ResourceIDs()
	if !slices.Equal(got, []string{instID}) {
		t.Errorf("volume %s lists %v, want [%s]", volID, got, instID)
	}
}

// TestRelatedOneFact_UnattachedVolumeNamesNoInstance pins the empty end of
// the same fact: an available volume has no attachment to read.
func TestRelatedOneFact_UnattachedVolumeNamesNoInstance(t *testing.T) {
	const volID = "vol-0a1b2c3d4e5f60010"
	row := ebsRowFor(t, ec2types.Volume{
		VolumeId:         aws.String(volID),
		State:            ec2types.VolumeStateAvailable,
		VolumeType:       ec2types.VolumeTypeGp3,
		Size:             aws.Int32(20),
		AvailabilityZone: aws.String("us-east-1a"),
	})
	if got := ebsCheckerByTarget(t, "ec2")(context.Background(), nil, row, resource.ResourceCache{}).ResourceIDs(); len(got) != 0 {
		t.Errorf("volume %s lists %v, want none", volID, got)
	}
}

// mainRTBWorld is a VPC holding two subnets: one explicitly associated with a
// custom route table, one left alone. AWS implicitly associates a subnet with
// no explicit association to its VPC's main route table, and
// RouteTableAssociation returns no SubnetId for an implicit association.
func mainRTBWorld() (main, custom, explicitSubnet, implicitSubnet resource.Resource, cache resource.ResourceCache) {
	const vpcID = "vpc-0a1b2c3d4e5f60001"
	const explicitID, implicitID = "subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"

	main = resource.Resource{
		ID:     "rtb-0a1b2c3d4e5f60001",
		Fields: map[string]string{"vpc_id": vpcID},
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0a1b2c3d4e5f60001"),
			VpcId:        aws.String(vpcID),
			Associations: []ec2types.RouteTableAssociation{
				{RouteTableAssociationId: aws.String("rtbassoc-0a1b2c3d4e5f60001"), Main: aws.Bool(true)},
			},
		},
	}
	custom = resource.Resource{
		ID:     "rtb-0a1b2c3d4e5f60002",
		Fields: map[string]string{"vpc_id": vpcID},
		RawStruct: ec2types.RouteTable{
			RouteTableId: aws.String("rtb-0a1b2c3d4e5f60002"),
			VpcId:        aws.String(vpcID),
			Associations: []ec2types.RouteTableAssociation{
				{RouteTableAssociationId: aws.String("rtbassoc-0a1b2c3d4e5f60002"), SubnetId: aws.String(explicitID), Main: aws.Bool(false)},
			},
		},
	}
	explicitSubnet = resource.Resource{
		ID:        explicitID,
		Fields:    map[string]string{"vpc_id": vpcID},
		RawStruct: ec2types.Subnet{SubnetId: aws.String(explicitID), VpcId: aws.String(vpcID)},
	}
	implicitSubnet = resource.Resource{
		ID:        implicitID,
		Fields:    map[string]string{"vpc_id": vpcID},
		RawStruct: ec2types.Subnet{SubnetId: aws.String(implicitID), VpcId: aws.String(vpcID)},
	}
	cache = resource.ResourceCache{
		"rtb":    resource.ResourceCacheEntry{Resources: []resource.Resource{main, custom}},
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{explicitSubnet, implicitSubnet}},
	}
	return main, custom, explicitSubnet, implicitSubnet, cache
}

// TestRelatedOneFact_MainRouteTableNamesImplicitlyAssociatedSubnets pins that
// the main route table of a VPC names the subnets implicitly associated with
// it. A subnet with no explicit association routes through the main table, so
// the association is the same fact from either end even though the API
// returns no SubnetId for it.
func TestRelatedOneFact_MainRouteTableNamesImplicitlyAssociatedSubnets(t *testing.T) {
	main, custom, explicitSubnet, implicitSubnet, cache := mainRTBWorld()
	ctx := context.Background()

	rtbToSubnet := rtbCheckerByTarget(t, "subnet")
	subnetToRTB := subnetCheckerByTarget(t, "rtb")

	if got := subnetToRTB(ctx, nil, implicitSubnet, cache).ResourceIDs(); !slices.Equal(got, []string{main.ID}) {
		t.Errorf("subnet %s lists %v, want [%s]", implicitSubnet.ID, got, main.ID)
	}
	if got := rtbToSubnet(ctx, nil, main, cache).ResourceIDs(); !slices.Equal(got, []string{implicitSubnet.ID}) {
		t.Errorf("main route table %s lists %v, want [%s]: a subnet with no explicit association routes through its VPC's main table",
			main.ID, got, implicitSubnet.ID)
	}

	if got := subnetToRTB(ctx, nil, explicitSubnet, cache).ResourceIDs(); !slices.Equal(got, []string{custom.ID}) {
		t.Errorf("subnet %s lists %v, want [%s]", explicitSubnet.ID, got, custom.ID)
	}
	if got := rtbToSubnet(ctx, nil, custom, cache).ResourceIDs(); !slices.Equal(got, []string{explicitSubnet.ID}) {
		t.Errorf("route table %s lists %v, want [%s]", custom.ID, got, explicitSubnet.ID)
	}
}

// TestRelatedOneFact_MainRouteTableSkipsForeignVPCSubnets pins that the
// implicit association stops at the VPC boundary: a subnet of another VPC
// routes through that VPC's own main table.
func TestRelatedOneFact_MainRouteTableSkipsForeignVPCSubnets(t *testing.T) {
	main, _, _, _, cache := mainRTBWorld()
	const foreignID = "subnet-0a1b2c3d4e5f60003"
	foreign := resource.Resource{
		ID:        foreignID,
		Fields:    map[string]string{"vpc_id": "vpc-0a1b2c3d4e5f60002"},
		RawStruct: ec2types.Subnet{SubnetId: aws.String(foreignID), VpcId: aws.String("vpc-0a1b2c3d4e5f60002")},
	}
	entry := cache["subnet"]
	entry.Resources = append(slices.Clone(entry.Resources), foreign)
	cache["subnet"] = entry

	got := rtbCheckerByTarget(t, "subnet")(context.Background(), nil, main, cache).ResourceIDs()
	if slices.Contains(got, foreignID) {
		t.Errorf("main route table %s lists %v, which includes subnet %s of another VPC", main.ID, got, foreignID)
	}
}

// TestRelatedOneFact_ENINamesEIPOnSecondaryPrivateIP pins that a network
// interface names every Elastic IP associated with it. NetworkInterface
// .Association carries only the association of the primary private IPv4
// address; an EIP associated with a secondary private IP appears solely in
// that address's own entry under PrivateIpAddresses, while
// Address.NetworkInterfaceId names the interface either way.
func TestRelatedOneFact_ENINamesEIPOnSecondaryPrivateIP(t *testing.T) {
	const eniID = "eni-0a1b2c3d4e5f60001"
	const primaryAlloc, secondaryAlloc = "eipalloc-0a1b2c3d4e5f60001", "eipalloc-0a1b2c3d4e5f60002"

	eni := resource.Resource{
		ID: eniID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
			PrivateIpAddress:   aws.String("10.0.1.10"),
			Association: &ec2types.NetworkInterfaceAssociation{
				AllocationId: aws.String(primaryAlloc),
				PublicIp:     aws.String("198.51.100.10"),
			},
			PrivateIpAddresses: []ec2types.NetworkInterfacePrivateIpAddress{
				{
					Primary:          aws.Bool(true),
					PrivateIpAddress: aws.String("10.0.1.10"),
					Association: &ec2types.NetworkInterfaceAssociation{
						AllocationId: aws.String(primaryAlloc),
						PublicIp:     aws.String("198.51.100.10"),
					},
				},
				{
					Primary:          aws.Bool(false),
					PrivateIpAddress: aws.String("10.0.1.11"),
					Association: &ec2types.NetworkInterfaceAssociation{
						AllocationId: aws.String(secondaryAlloc),
						PublicIp:     aws.String("198.51.100.11"),
					},
				},
			},
		},
	}
	eips := []resource.Resource{
		{ID: primaryAlloc, RawStruct: ec2types.Address{
			AllocationId: aws.String(primaryAlloc), PublicIp: aws.String("198.51.100.10"),
			NetworkInterfaceId: aws.String(eniID), PrivateIpAddress: aws.String("10.0.1.10"),
		}},
		{ID: secondaryAlloc, RawStruct: ec2types.Address{
			AllocationId: aws.String(secondaryAlloc), PublicIp: aws.String("198.51.100.11"),
			NetworkInterfaceId: aws.String(eniID), PrivateIpAddress: aws.String("10.0.1.11"),
		}},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: eips},
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eni}},
	}
	ctx := context.Background()

	for _, eip := range eips {
		if got := eipCheckerByTarget(t, "eni")(ctx, nil, eip, cache).ResourceIDs(); !slices.Equal(got, []string{eniID}) {
			t.Errorf("EIP %s lists %v, want [%s]", eip.ID, got, eniID)
		}
	}

	got := eniCheckerByTarget(t, "eip")(ctx, nil, eni, cache).ResourceIDs()
	slices.Sort(got)
	want := []string{primaryAlloc, secondaryAlloc}
	if !slices.Equal(got, want) {
		t.Errorf("interface %s lists %v, want %v: an Elastic IP on a secondary private address is associated with the interface too",
			eniID, got, want)
	}
}

// TestRelatedOneFact_ENIWithNoAssociationNamesNoEIP pins the empty end: an
// interface whose addresses carry no association has no Elastic IP.
func TestRelatedOneFact_ENIWithNoAssociationNamesNoEIP(t *testing.T) {
	const eniID = "eni-0a1b2c3d4e5f60002"
	eni := resource.Resource{
		ID: eniID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
			PrivateIpAddress:   aws.String("10.0.2.10"),
			PrivateIpAddresses: []ec2types.NetworkInterfacePrivateIpAddress{
				{Primary: aws.Bool(true), PrivateIpAddress: aws.String("10.0.2.10")},
				{Primary: aws.Bool(false), PrivateIpAddress: aws.String("10.0.2.11")},
			},
		},
	}
	cache := resource.ResourceCache{
		"eip": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eni}},
	}
	if got := eniCheckerByTarget(t, "eip")(context.Background(), nil, eni, cache).ResourceIDs(); len(got) != 0 {
		t.Errorf("interface %s lists %v, want none", eniID, got)
	}
}
