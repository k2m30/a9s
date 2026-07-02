package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ec2FormatTime formats a *time.Time the same way s3.go's formatTime does,
// under a distinct name to avoid a symbol collision in this package.
func ec2FormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// ---------------------------------------------------------------------------
// ec2 — DescribeInstances (list) + DescribeInstanceStatus (Wave 2, account-wide)
// ---------------------------------------------------------------------------

type ec2Data struct {
	Instances []ec2Instance `json:"instances"`
	Statuses  []ec2Status   `json:"instance_statuses"`
}

type ec2Instance struct {
	InstanceId            string            `json:"instance_id"`
	ImageId               string            `json:"image_id,omitempty"`
	State                 string            `json:"state,omitempty"`
	StateReasonCode       string            `json:"state_reason_code,omitempty"`
	StateReasonMessage    string            `json:"state_reason_message,omitempty"`
	StateTransitionReason string            `json:"state_transition_reason,omitempty"`
	SubnetId              string            `json:"subnet_id,omitempty"`
	VpcId                 string            `json:"vpc_id,omitempty"`
	LaunchTime            string            `json:"launch_time,omitempty"`
	IamInstanceProfileArn string            `json:"iam_instance_profile_arn,omitempty"`
	SecurityGroups        []ec2GroupRef     `json:"security_groups,omitempty"`
	NetworkInterfaceIds   []string          `json:"network_interface_ids,omitempty"`
	BlockDeviceMappings   []ec2BlockDevice  `json:"block_device_mappings,omitempty"`
	Tags                  map[string]string `json:"tags,omitempty"`
}

type ec2GroupRef struct {
	GroupId   string `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
}

type ec2BlockDevice struct {
	DeviceName string `json:"device_name,omitempty"`
	VolumeId   string `json:"volume_id,omitempty"`
}

// ec2Status captures one DescribeInstanceStatus(IncludeAllInstances=true) row.
type ec2Status struct {
	InstanceId           string           `json:"instance_id"`
	InstanceStateName    string           `json:"instance_state_name,omitempty"`
	SystemStatus         string           `json:"system_status,omitempty"`
	InstanceStatusStatus string           `json:"instance_status_status,omitempty"`
	Events               []ec2StatusEvent `json:"events,omitempty"`
}

type ec2StatusEvent struct {
	Code      string `json:"code,omitempty"`
	NotBefore string `json:"not_before,omitempty"`
	NotAfter  string `json:"not_after,omitempty"`
}

func captureEC2(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var instances []ec2Instance
	pager := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range out.Reservations {
			for _, i := range r.Instances {
				inst := ec2Instance{
					InstanceId:            aws.ToString(i.InstanceId),
					ImageId:               aws.ToString(i.ImageId),
					SubnetId:              aws.ToString(i.SubnetId),
					VpcId:                 aws.ToString(i.VpcId),
					LaunchTime:            ec2FormatTime(i.LaunchTime),
					StateTransitionReason: aws.ToString(i.StateTransitionReason),
				}
				if i.State != nil {
					inst.State = string(i.State.Name)
				}
				if i.StateReason != nil {
					inst.StateReasonCode = aws.ToString(i.StateReason.Code)
					inst.StateReasonMessage = aws.ToString(i.StateReason.Message)
				}
				if i.IamInstanceProfile != nil {
					inst.IamInstanceProfileArn = aws.ToString(i.IamInstanceProfile.Arn)
				}
				for _, sg := range i.SecurityGroups {
					inst.SecurityGroups = append(inst.SecurityGroups, ec2GroupRef{
						GroupId:   aws.ToString(sg.GroupId),
						GroupName: aws.ToString(sg.GroupName),
					})
				}
				for _, ni := range i.NetworkInterfaces {
					inst.NetworkInterfaceIds = append(inst.NetworkInterfaceIds, aws.ToString(ni.NetworkInterfaceId))
				}
				for _, bdm := range i.BlockDeviceMappings {
					bd := ec2BlockDevice{DeviceName: aws.ToString(bdm.DeviceName)}
					if bdm.Ebs != nil {
						bd.VolumeId = aws.ToString(bdm.Ebs.VolumeId)
					}
					inst.BlockDeviceMappings = append(inst.BlockDeviceMappings, bd)
				}
				if len(i.Tags) > 0 {
					inst.Tags = make(map[string]string, len(i.Tags))
					for _, t := range i.Tags {
						inst.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
					}
				}
				instances = append(instances, inst)
			}
		}
	}

	var statuses []ec2Status
	statusPager := ec2.NewDescribeInstanceStatusPaginator(client, &ec2.DescribeInstanceStatusInput{
		IncludeAllInstances: aws.Bool(true),
	})
	for statusPager.HasMorePages() {
		out, err := statusPager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.InstanceStatuses {
			st := ec2Status{InstanceId: aws.ToString(s.InstanceId)}
			if s.InstanceState != nil {
				st.InstanceStateName = string(s.InstanceState.Name)
			}
			if s.SystemStatus != nil {
				st.SystemStatus = string(s.SystemStatus.Status)
			}
			if s.InstanceStatus != nil {
				st.InstanceStatusStatus = string(s.InstanceStatus.Status)
			}
			for _, e := range s.Events {
				st.Events = append(st.Events, ec2StatusEvent{
					Code:      string(e.Code),
					NotBefore: ec2FormatTime(e.NotBefore),
					NotAfter:  ec2FormatTime(e.NotAfter),
				})
			}
			statuses = append(statuses, st)
		}
	}

	return ec2Data{Instances: instances, Statuses: statuses}, nil
}

// ---------------------------------------------------------------------------
// ami — DescribeImages (self-owned)
// ---------------------------------------------------------------------------

type amiData struct {
	Images []amiImage `json:"images"`
}

type amiImage struct {
	ImageId             string           `json:"image_id"`
	Name                string           `json:"name,omitempty"`
	State               string           `json:"state,omitempty"`
	StateReasonMessage  string           `json:"state_reason_message,omitempty"`
	ImageOwnerAlias     string           `json:"image_owner_alias,omitempty"`
	OwnerId             string           `json:"owner_id,omitempty"`
	Public              bool             `json:"public"`
	CreationDate        string           `json:"creation_date,omitempty"`
	DeprecationTime     string           `json:"deprecation_time,omitempty"`
	BlockDeviceMappings []amiBlockDevice `json:"block_device_mappings,omitempty"`
}

type amiBlockDevice struct {
	SnapshotId string `json:"snapshot_id,omitempty"`
	KmsKeyId   string `json:"kms_key_id,omitempty"`
}

func captureAMI(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var images []amiImage
	pager := ec2.NewDescribeImagesPaginator(client, &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, img := range out.Images {
			im := amiImage{
				ImageId:         aws.ToString(img.ImageId),
				Name:            aws.ToString(img.Name),
				State:           string(img.State),
				ImageOwnerAlias: aws.ToString(img.ImageOwnerAlias),
				OwnerId:         aws.ToString(img.OwnerId),
				Public:          aws.ToBool(img.Public),
				CreationDate:    aws.ToString(img.CreationDate),
				DeprecationTime: aws.ToString(img.DeprecationTime),
			}
			if img.StateReason != nil {
				im.StateReasonMessage = aws.ToString(img.StateReason.Message)
			}
			for _, bdm := range img.BlockDeviceMappings {
				if bdm.Ebs == nil {
					continue
				}
				im.BlockDeviceMappings = append(im.BlockDeviceMappings, amiBlockDevice{
					SnapshotId: aws.ToString(bdm.Ebs.SnapshotId),
					KmsKeyId:   aws.ToString(bdm.Ebs.KmsKeyId),
				})
			}
			images = append(images, im)
		}
	}

	return amiData{Images: images}, nil
}

// ---------------------------------------------------------------------------
// ebs — DescribeVolumes (list) + DescribeVolumeStatus (Wave 2, account-wide)
// ---------------------------------------------------------------------------

type ebsData struct {
	Volumes  []ebsVolume       `json:"volumes"`
	Statuses []ebsVolumeStatus `json:"volume_statuses"`
}

type ebsVolume struct {
	VolumeId    string            `json:"volume_id"`
	State       string            `json:"state,omitempty"`
	Encrypted   bool              `json:"encrypted"`
	CreateTime  string            `json:"create_time,omitempty"`
	KmsKeyId    string            `json:"kms_key_id,omitempty"`
	Size        int32             `json:"size_gib,omitempty"`
	VolumeType  string            `json:"volume_type,omitempty"`
	Attachments []ebsAttachment   `json:"attachments,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

type ebsAttachment struct {
	InstanceId string `json:"instance_id,omitempty"`
	State      string `json:"state,omitempty"`
}

type ebsVolumeStatus struct {
	VolumeId string           `json:"volume_id"`
	Status   string           `json:"status,omitempty"`
	Events   []ebsVolumeEvent `json:"events,omitempty"`
}

type ebsVolumeEvent struct {
	EventType   string `json:"event_type,omitempty"`
	Description string `json:"description,omitempty"`
	NotBefore   string `json:"not_before,omitempty"`
	NotAfter    string `json:"not_after,omitempty"`
}

func captureEBS(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var volumes []ebsVolume
	pager := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range out.Volumes {
			vol := ebsVolume{
				VolumeId:   aws.ToString(v.VolumeId),
				State:      string(v.State),
				Encrypted:  aws.ToBool(v.Encrypted),
				CreateTime: ec2FormatTime(v.CreateTime),
				KmsKeyId:   aws.ToString(v.KmsKeyId),
				Size:       aws.ToInt32(v.Size),
				VolumeType: string(v.VolumeType),
			}
			for _, a := range v.Attachments {
				vol.Attachments = append(vol.Attachments, ebsAttachment{
					InstanceId: aws.ToString(a.InstanceId),
					State:      string(a.State),
				})
			}
			if len(v.Tags) > 0 {
				vol.Tags = make(map[string]string, len(v.Tags))
				for _, t := range v.Tags {
					vol.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			volumes = append(volumes, vol)
		}
	}

	var statuses []ebsVolumeStatus
	statusPager := ec2.NewDescribeVolumeStatusPaginator(client, &ec2.DescribeVolumeStatusInput{})
	for statusPager.HasMorePages() {
		out, err := statusPager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.VolumeStatuses {
			vs := ebsVolumeStatus{VolumeId: aws.ToString(s.VolumeId)}
			if s.VolumeStatus != nil {
				vs.Status = string(s.VolumeStatus.Status)
			}
			for _, e := range s.Events {
				vs.Events = append(vs.Events, ebsVolumeEvent{
					EventType:   aws.ToString(e.EventType),
					Description: aws.ToString(e.Description),
					NotBefore:   ec2FormatTime(e.NotBefore),
					NotAfter:    ec2FormatTime(e.NotAfter),
				})
			}
			statuses = append(statuses, vs)
		}
	}

	return ebsData{Volumes: volumes, Statuses: statuses}, nil
}

// ---------------------------------------------------------------------------
// ebs-snap — DescribeSnapshots (self-owned)
// ---------------------------------------------------------------------------

type ebsSnapData struct {
	Snapshots []ebsSnapshot `json:"snapshots"`
}

type ebsSnapshot struct {
	SnapshotId   string `json:"snapshot_id"`
	VolumeId     string `json:"volume_id,omitempty"`
	State        string `json:"state,omitempty"`
	StateMessage string `json:"state_message,omitempty"`
	Progress     string `json:"progress,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	Encrypted    bool   `json:"encrypted"`
	KmsKeyId     string `json:"kms_key_id,omitempty"`
	Description  string `json:"description,omitempty"`
	OwnerId      string `json:"owner_id,omitempty"`
}

func captureEBSSnap(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var snapshots []ebsSnapshot
	pager := ec2.NewDescribeSnapshotsPaginator(client, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.Snapshots {
			snapshots = append(snapshots, ebsSnapshot{
				SnapshotId:   aws.ToString(s.SnapshotId),
				VolumeId:     aws.ToString(s.VolumeId),
				State:        string(s.State),
				StateMessage: aws.ToString(s.StateMessage),
				Progress:     aws.ToString(s.Progress),
				StartTime:    ec2FormatTime(s.StartTime),
				Encrypted:    aws.ToBool(s.Encrypted),
				KmsKeyId:     aws.ToString(s.KmsKeyId),
				Description:  aws.ToString(s.Description),
				OwnerId:      aws.ToString(s.OwnerId),
			})
		}
	}

	return ebsSnapData{Snapshots: snapshots}, nil
}

// ---------------------------------------------------------------------------
// eni — DescribeNetworkInterfaces
// ---------------------------------------------------------------------------

type eniData struct {
	NetworkInterfaces []eniInterface `json:"network_interfaces"`
}

type eniInterface struct {
	NetworkInterfaceId string          `json:"network_interface_id"`
	Status             string          `json:"status,omitempty"`
	InterfaceType      string          `json:"interface_type,omitempty"`
	Description        string          `json:"description,omitempty"`
	RequesterManaged   bool            `json:"requester_managed"`
	SubnetId           string          `json:"subnet_id,omitempty"`
	VpcId              string          `json:"vpc_id,omitempty"`
	Groups             []ec2GroupRef   `json:"groups,omitempty"`
	Attachment         *eniAttachment  `json:"attachment,omitempty"`
	Association        *eniAssociation `json:"association,omitempty"`
}

type eniAttachment struct {
	InstanceId string `json:"instance_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

type eniAssociation struct {
	AllocationId string `json:"allocation_id,omitempty"`
	PublicIp     string `json:"public_ip,omitempty"`
}

func captureENI(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var enis []eniInterface
	pager := ec2.NewDescribeNetworkInterfacesPaginator(client, &ec2.DescribeNetworkInterfacesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, ni := range out.NetworkInterfaces {
			e := eniInterface{
				NetworkInterfaceId: aws.ToString(ni.NetworkInterfaceId),
				Status:             string(ni.Status),
				InterfaceType:      string(ni.InterfaceType),
				Description:        aws.ToString(ni.Description),
				RequesterManaged:   aws.ToBool(ni.RequesterManaged),
				SubnetId:           aws.ToString(ni.SubnetId),
				VpcId:              aws.ToString(ni.VpcId),
			}
			for _, g := range ni.Groups {
				e.Groups = append(e.Groups, ec2GroupRef{
					GroupId:   aws.ToString(g.GroupId),
					GroupName: aws.ToString(g.GroupName),
				})
			}
			if ni.Attachment != nil {
				e.Attachment = &eniAttachment{
					InstanceId: aws.ToString(ni.Attachment.InstanceId),
					Status:     string(ni.Attachment.Status),
				}
			}
			if ni.Association != nil {
				e.Association = &eniAssociation{
					AllocationId: aws.ToString(ni.Association.AllocationId),
					PublicIp:     aws.ToString(ni.Association.PublicIp),
				}
			}
			enis = append(enis, e)
		}
	}

	return eniData{NetworkInterfaces: enis}, nil
}

// ---------------------------------------------------------------------------
// eip — DescribeAddresses
// ---------------------------------------------------------------------------

type eipData struct {
	Addresses []eipAddress `json:"addresses"`
}

type eipAddress struct {
	AllocationId       string `json:"allocation_id,omitempty"`
	AssociationId      string `json:"association_id,omitempty"`
	InstanceId         string `json:"instance_id,omitempty"`
	NetworkInterfaceId string `json:"network_interface_id,omitempty"`
	PublicIp           string `json:"public_ip,omitempty"`
	Domain             string `json:"domain,omitempty"`
}

func captureEIP(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	out, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, err
	}

	addresses := make([]eipAddress, 0, len(out.Addresses))
	for _, a := range out.Addresses {
		addresses = append(addresses, eipAddress{
			AllocationId:       aws.ToString(a.AllocationId),
			AssociationId:      aws.ToString(a.AssociationId),
			InstanceId:         aws.ToString(a.InstanceId),
			NetworkInterfaceId: aws.ToString(a.NetworkInterfaceId),
			PublicIp:           aws.ToString(a.PublicIp),
			Domain:             string(a.Domain),
		})
	}

	return eipData{Addresses: addresses}, nil
}

// ---------------------------------------------------------------------------
// sg — DescribeSecurityGroups
// ---------------------------------------------------------------------------

type sgData struct {
	SecurityGroups []sgGroup `json:"security_groups"`
}

type sgGroup struct {
	GroupId             string            `json:"group_id"`
	GroupName           string            `json:"group_name,omitempty"`
	VpcId               string            `json:"vpc_id,omitempty"`
	IpPermissions       []sgPermission    `json:"ip_permissions,omitempty"`
	IpPermissionsEgress []sgPermission    `json:"ip_permissions_egress,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

type sgPermission struct {
	IpProtocol string   `json:"ip_protocol,omitempty"`
	FromPort   *int32   `json:"from_port,omitempty"`
	ToPort     *int32   `json:"to_port,omitempty"`
	IpRanges   []string `json:"ip_ranges,omitempty"`
	Ipv6Ranges []string `json:"ipv6_ranges,omitempty"`
	GroupPairs []string `json:"group_pairs,omitempty"`
}

func captureSG(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var groups []sgGroup
	pager := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range out.SecurityGroups {
			sg := sgGroup{
				GroupId:   aws.ToString(g.GroupId),
				GroupName: aws.ToString(g.GroupName),
				VpcId:     aws.ToString(g.VpcId),
			}
			sg.IpPermissions = capturePermissions(g.IpPermissions)
			sg.IpPermissionsEgress = capturePermissions(g.IpPermissionsEgress)
			if len(g.Tags) > 0 {
				sg.Tags = make(map[string]string, len(g.Tags))
				for _, t := range g.Tags {
					sg.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			groups = append(groups, sg)
		}
	}

	return sgData{SecurityGroups: groups}, nil
}

func capturePermissions(perms []types.IpPermission) []sgPermission {
	out := make([]sgPermission, 0, len(perms))
	for _, p := range perms {
		perm := sgPermission{
			IpProtocol: aws.ToString(p.IpProtocol),
			FromPort:   p.FromPort,
			ToPort:     p.ToPort,
		}
		for _, r := range p.IpRanges {
			perm.IpRanges = append(perm.IpRanges, aws.ToString(r.CidrIp))
		}
		for _, r := range p.Ipv6Ranges {
			perm.Ipv6Ranges = append(perm.Ipv6Ranges, aws.ToString(r.CidrIpv6))
		}
		for _, gp := range p.UserIdGroupPairs {
			perm.GroupPairs = append(perm.GroupPairs, aws.ToString(gp.GroupId))
		}
		out = append(out, perm)
	}
	return out
}

// ---------------------------------------------------------------------------
// vpc — DescribeVpcs + DescribeFlowLogs (Wave 2, account-wide)
// ---------------------------------------------------------------------------

type vpcData struct {
	Vpcs     []vpcVpc     `json:"vpcs"`
	FlowLogs []vpcFlowLog `json:"flow_logs"`
}

type vpcVpc struct {
	VpcId     string            `json:"vpc_id"`
	State     string            `json:"state,omitempty"`
	CidrBlock string            `json:"cidr_block,omitempty"`
	IsDefault bool              `json:"is_default"`
	OwnerId   string            `json:"owner_id,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

type vpcFlowLog struct {
	FlowLogId     string `json:"flow_log_id"`
	ResourceId    string `json:"resource_id,omitempty"`
	FlowLogStatus string `json:"flow_log_status,omitempty"`
}

func captureVPC(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var vpcs []vpcVpc
	pager := ec2.NewDescribeVpcsPaginator(client, &ec2.DescribeVpcsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range out.Vpcs {
			vv := vpcVpc{
				VpcId:     aws.ToString(v.VpcId),
				State:     string(v.State),
				CidrBlock: aws.ToString(v.CidrBlock),
				IsDefault: aws.ToBool(v.IsDefault),
				OwnerId:   aws.ToString(v.OwnerId),
			}
			if len(v.Tags) > 0 {
				vv.Tags = make(map[string]string, len(v.Tags))
				for _, t := range v.Tags {
					vv.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			vpcs = append(vpcs, vv)
		}
	}

	var flowLogs []vpcFlowLog
	flPager := ec2.NewDescribeFlowLogsPaginator(client, &ec2.DescribeFlowLogsInput{})
	for flPager.HasMorePages() {
		out, err := flPager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, fl := range out.FlowLogs {
			flowLogs = append(flowLogs, vpcFlowLog{
				FlowLogId:     aws.ToString(fl.FlowLogId),
				ResourceId:    aws.ToString(fl.ResourceId),
				FlowLogStatus: aws.ToString(fl.FlowLogStatus),
			})
		}
	}

	return vpcData{Vpcs: vpcs, FlowLogs: flowLogs}, nil
}

// ---------------------------------------------------------------------------
// subnet — DescribeSubnets
// ---------------------------------------------------------------------------

type subnetData struct {
	Subnets []subnetSubnet `json:"subnets"`
}

type subnetSubnet struct {
	SubnetId                string            `json:"subnet_id"`
	VpcId                   string            `json:"vpc_id,omitempty"`
	State                   string            `json:"state,omitempty"`
	CidrBlock               string            `json:"cidr_block,omitempty"`
	AvailableIpAddressCount int32             `json:"available_ip_address_count"`
	MapPublicIpOnLaunch     bool              `json:"map_public_ip_on_launch"`
	AvailabilityZone        string            `json:"availability_zone,omitempty"`
	Tags                    map[string]string `json:"tags,omitempty"`
}

func captureSubnet(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var subnets []subnetSubnet
	pager := ec2.NewDescribeSubnetsPaginator(client, &ec2.DescribeSubnetsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.Subnets {
			sn := subnetSubnet{
				SubnetId:                aws.ToString(s.SubnetId),
				VpcId:                   aws.ToString(s.VpcId),
				State:                   string(s.State),
				CidrBlock:               aws.ToString(s.CidrBlock),
				AvailableIpAddressCount: aws.ToInt32(s.AvailableIpAddressCount),
				MapPublicIpOnLaunch:     aws.ToBool(s.MapPublicIpOnLaunch),
				AvailabilityZone:        aws.ToString(s.AvailabilityZone),
			}
			if len(s.Tags) > 0 {
				sn.Tags = make(map[string]string, len(s.Tags))
				for _, t := range s.Tags {
					sn.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			subnets = append(subnets, sn)
		}
	}

	return subnetData{Subnets: subnets}, nil
}

// ---------------------------------------------------------------------------
// rtb — DescribeRouteTables
// ---------------------------------------------------------------------------

type rtbData struct {
	RouteTables []rtbRouteTable `json:"route_tables"`
}

type rtbRouteTable struct {
	RouteTableId string            `json:"route_table_id"`
	VpcId        string            `json:"vpc_id,omitempty"`
	Routes       []rtbRoute        `json:"routes,omitempty"`
	Associations []rtbAssociation  `json:"associations,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

type rtbRoute struct {
	DestinationCidrBlock string `json:"destination_cidr_block,omitempty"`
	State                string `json:"state,omitempty"`
	GatewayId            string `json:"gateway_id,omitempty"`
	NatGatewayId         string `json:"nat_gateway_id,omitempty"`
	TransitGatewayId     string `json:"transit_gateway_id,omitempty"`
	NetworkInterfaceId   string `json:"network_interface_id,omitempty"`
}

type rtbAssociation struct {
	SubnetId string `json:"subnet_id,omitempty"`
	Main     bool   `json:"main"`
}

func captureRTB(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var tables []rtbRouteTable
	pager := ec2.NewDescribeRouteTablesPaginator(client, &ec2.DescribeRouteTablesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, rt := range out.RouteTables {
			t := rtbRouteTable{
				RouteTableId: aws.ToString(rt.RouteTableId),
				VpcId:        aws.ToString(rt.VpcId),
			}
			for _, r := range rt.Routes {
				t.Routes = append(t.Routes, rtbRoute{
					DestinationCidrBlock: aws.ToString(r.DestinationCidrBlock),
					State:                string(r.State),
					GatewayId:            aws.ToString(r.GatewayId),
					NatGatewayId:         aws.ToString(r.NatGatewayId),
					TransitGatewayId:     aws.ToString(r.TransitGatewayId),
					NetworkInterfaceId:   aws.ToString(r.NetworkInterfaceId),
				})
			}
			for _, a := range rt.Associations {
				t.Associations = append(t.Associations, rtbAssociation{
					SubnetId: aws.ToString(a.SubnetId),
					Main:     aws.ToBool(a.Main),
				})
			}
			if len(rt.Tags) > 0 {
				t.Tags = make(map[string]string, len(rt.Tags))
				for _, tg := range rt.Tags {
					t.Tags[aws.ToString(tg.Key)] = aws.ToString(tg.Value)
				}
			}
			tables = append(tables, t)
		}
	}

	return rtbData{RouteTables: tables}, nil
}

// ---------------------------------------------------------------------------
// igw — DescribeInternetGateways
// ---------------------------------------------------------------------------

type igwData struct {
	InternetGateways []igwGateway `json:"internet_gateways"`
}

type igwGateway struct {
	InternetGatewayId string            `json:"internet_gateway_id"`
	Attachments       []igwAttachment   `json:"attachments,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
}

type igwAttachment struct {
	VpcId string `json:"vpc_id,omitempty"`
	State string `json:"state,omitempty"`
}

func captureIGW(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var gateways []igwGateway
	pager := ec2.NewDescribeInternetGatewaysPaginator(client, &ec2.DescribeInternetGatewaysInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range out.InternetGateways {
			igw := igwGateway{InternetGatewayId: aws.ToString(g.InternetGatewayId)}
			for _, a := range g.Attachments {
				igw.Attachments = append(igw.Attachments, igwAttachment{
					VpcId: aws.ToString(a.VpcId),
					State: string(a.State),
				})
			}
			if len(g.Tags) > 0 {
				igw.Tags = make(map[string]string, len(g.Tags))
				for _, t := range g.Tags {
					igw.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			gateways = append(gateways, igw)
		}
	}

	return igwData{InternetGateways: gateways}, nil
}

// ---------------------------------------------------------------------------
// nat — DescribeNatGateways
// ---------------------------------------------------------------------------

type natData struct {
	NatGateways []natGateway `json:"nat_gateways"`
}

type natGateway struct {
	NatGatewayId        string            `json:"nat_gateway_id"`
	State               string            `json:"state,omitempty"`
	FailureCode         string            `json:"failure_code,omitempty"`
	FailureMessage      string            `json:"failure_message,omitempty"`
	SubnetId            string            `json:"subnet_id,omitempty"`
	VpcId               string            `json:"vpc_id,omitempty"`
	CreateTime          string            `json:"create_time,omitempty"`
	NatGatewayAddresses []natAddress      `json:"nat_gateway_addresses,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

type natAddress struct {
	AllocationId       string `json:"allocation_id,omitempty"`
	NetworkInterfaceId string `json:"network_interface_id,omitempty"`
}

func captureNAT(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var gateways []natGateway
	pager := ec2.NewDescribeNatGatewaysPaginator(client, &ec2.DescribeNatGatewaysInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, n := range out.NatGateways {
			nat := natGateway{
				NatGatewayId:   aws.ToString(n.NatGatewayId),
				State:          string(n.State),
				FailureCode:    aws.ToString(n.FailureCode),
				FailureMessage: aws.ToString(n.FailureMessage),
				SubnetId:       aws.ToString(n.SubnetId),
				VpcId:          aws.ToString(n.VpcId),
				CreateTime:     ec2FormatTime(n.CreateTime),
			}
			for _, a := range n.NatGatewayAddresses {
				nat.NatGatewayAddresses = append(nat.NatGatewayAddresses, natAddress{
					AllocationId:       aws.ToString(a.AllocationId),
					NetworkInterfaceId: aws.ToString(a.NetworkInterfaceId),
				})
			}
			if len(n.Tags) > 0 {
				nat.Tags = make(map[string]string, len(n.Tags))
				for _, t := range n.Tags {
					nat.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
			}
			gateways = append(gateways, nat)
		}
	}

	return natData{NatGateways: gateways}, nil
}

// ---------------------------------------------------------------------------
// tgw — DescribeTransitGateways + DescribeTransitGatewayAttachments (Wave 2, per-TGW)
// ---------------------------------------------------------------------------

type tgwData struct {
	TransitGateways []tgwGateway `json:"transit_gateways"`
}

type tgwGateway struct {
	TransitGatewayId string              `json:"transit_gateway_id"`
	State            string              `json:"state,omitempty"`
	CreationTime     string              `json:"creation_time,omitempty"`
	Attachments      tgwAttachmentResult `json:"attachments"`
	Tags             map[string]string   `json:"tags,omitempty"`
}

// tgwAttachmentResult carries the DescribeTransitGatewayAttachments outcome
// per TGW so a per-resource describe error doesn't fail the whole capture.
type tgwAttachmentResult struct {
	Outcome   string          `json:"outcome"`
	ErrorCode string          `json:"error_code,omitempty"`
	Items     []tgwAttachment `json:"items,omitempty"`
}

type tgwAttachment struct {
	TransitGatewayAttachmentId string `json:"transit_gateway_attachment_id"`
	ResourceType               string `json:"resource_type,omitempty"`
	ResourceId                 string `json:"resource_id,omitempty"`
	State                      string `json:"state,omitempty"`
	CreationTime               string `json:"creation_time,omitempty"`
}

func captureTGW(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var gateways []tgwGateway
	pager := ec2.NewDescribeTransitGatewaysPaginator(client, &ec2.DescribeTransitGatewaysInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range out.TransitGateways {
			tg := tgwGateway{
				TransitGatewayId: aws.ToString(t.TransitGatewayId),
				State:            string(t.State),
				CreationTime:     ec2FormatTime(t.CreationTime),
			}
			if len(t.Tags) > 0 {
				tg.Tags = make(map[string]string, len(t.Tags))
				for _, tag := range t.Tags {
					tg.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}
			}
			tg.Attachments = captureTGWAttachments(ctx, client, tg.TransitGatewayId)
			gateways = append(gateways, tg)
		}
	}

	return tgwData{TransitGateways: gateways}, nil
}

func captureTGWAttachments(ctx context.Context, client *ec2.Client, transitGatewayId string) tgwAttachmentResult {
	var items []tgwAttachment
	pager := ec2.NewDescribeTransitGatewayAttachmentsPaginator(client, &ec2.DescribeTransitGatewayAttachmentsInput{
		Filters: []types.Filter{
			{Name: aws.String("transit-gateway-id"), Values: []string{transitGatewayId}},
		},
	})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return tgwAttachmentResult{Outcome: "error", ErrorCode: err.Error()}
		}
		for _, a := range out.TransitGatewayAttachments {
			items = append(items, tgwAttachment{
				TransitGatewayAttachmentId: aws.ToString(a.TransitGatewayAttachmentId),
				ResourceType:               string(a.ResourceType),
				ResourceId:                 aws.ToString(a.ResourceId),
				State:                      string(a.State),
				CreationTime:               ec2FormatTime(a.CreationTime),
			})
		}
	}
	return tgwAttachmentResult{Outcome: "ok", Items: items}
}

// ---------------------------------------------------------------------------
// vpce — DescribeVpcEndpoints
// ---------------------------------------------------------------------------

type vpceData struct {
	VpcEndpoints []vpceEndpoint `json:"vpc_endpoints"`
}

type vpceEndpoint struct {
	VpcEndpointId       string        `json:"vpc_endpoint_id"`
	VpcEndpointType     string        `json:"vpc_endpoint_type,omitempty"`
	State               string        `json:"state,omitempty"`
	VpcId               string        `json:"vpc_id,omitempty"`
	ServiceName         string        `json:"service_name,omitempty"`
	RouteTableIds       []string      `json:"route_table_ids,omitempty"`
	SubnetIds           []string      `json:"subnet_ids,omitempty"`
	NetworkInterfaceIds []string      `json:"network_interface_ids,omitempty"`
	Groups              []ec2GroupRef `json:"groups,omitempty"`
	PrivateDnsEnabled   bool          `json:"private_dns_enabled"`
	LastErrorCode       string        `json:"last_error_code,omitempty"`
	LastErrorMessage    string        `json:"last_error_message,omitempty"`
	DnsHostedZoneIds    []string      `json:"dns_hosted_zone_ids,omitempty"`
}

func captureVPCE(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var endpoints []vpceEndpoint
	pager := ec2.NewDescribeVpcEndpointsPaginator(client, &ec2.DescribeVpcEndpointsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range out.VpcEndpoints {
			ve := vpceEndpoint{
				VpcEndpointId:       aws.ToString(e.VpcEndpointId),
				VpcEndpointType:     string(e.VpcEndpointType),
				State:               string(e.State),
				VpcId:               aws.ToString(e.VpcId),
				ServiceName:         aws.ToString(e.ServiceName),
				RouteTableIds:       e.RouteTableIds,
				SubnetIds:           e.SubnetIds,
				NetworkInterfaceIds: e.NetworkInterfaceIds,
				PrivateDnsEnabled:   aws.ToBool(e.PrivateDnsEnabled),
			}
			for _, g := range e.Groups {
				ve.Groups = append(ve.Groups, ec2GroupRef{
					GroupId:   aws.ToString(g.GroupId),
					GroupName: aws.ToString(g.GroupName),
				})
			}
			if e.LastError != nil {
				ve.LastErrorCode = aws.ToString(e.LastError.Code)
				ve.LastErrorMessage = aws.ToString(e.LastError.Message)
			}
			for _, d := range e.DnsEntries {
				if hz := aws.ToString(d.HostedZoneId); hz != "" {
					ve.DnsHostedZoneIds = append(ve.DnsHostedZoneIds, hz)
				}
			}
			endpoints = append(endpoints, ve)
		}
	}

	return vpceData{VpcEndpoints: endpoints}, nil
}
