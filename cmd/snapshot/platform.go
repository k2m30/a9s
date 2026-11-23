package main

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
)

type ltData struct {
	Templates []ltTemplate `json:"launch_templates"`
	// The Region and AMI settings a template's own data leaves to them, read
	// only when a template defers to them.
	EbsEncryptionByDefault   *ltEbsEncryptionByDefault `json:"ebs_encryption_by_default,omitempty"`
	InstanceMetadataDefaults *ltMetadataDefaults       `json:"instance_metadata_defaults,omitempty"`
	AMIs                     []ltAMI                   `json:"amis,omitempty"`
}

type ltTemplate struct {
	LaunchTemplateId     string    `json:"launch_template_id"`
	LaunchTemplateName   string    `json:"launch_template_name"`
	CreatedBy            string    `json:"created_by,omitempty"`
	CreateTime           string    `json:"create_time,omitempty"`
	DefaultVersionNumber int64     `json:"default_version_number"`
	LatestVersionNumber  int64     `json:"latest_version_number"`
	DefaultVersion       ltVersion `json:"default_version"`
}

// ltVersion is the DescribeLaunchTemplateVersions("$Default") answer.
type ltVersion struct {
	readOutcome
	VersionNumber      int64           `json:"version_number,omitempty"`
	ImageId            string          `json:"image_id,omitempty"`
	HttpTokens         string          `json:"http_tokens,omitempty"`
	HttpEndpoint       string          `json:"http_endpoint,omitempty"`
	BlockDevices       []ltBlockDevice `json:"block_devices,omitempty"`
	HasUserData        bool            `json:"has_user_data,omitempty"`
	DefaultVersionTime string          `json:"version_create_time,omitempty"`
}

// ltBlockDevice keeps Encrypted as AWS states it: unset leaves the volume to
// the Region's encryption-by-default setting.
type ltBlockDevice struct {
	DeviceName string `json:"device_name,omitempty"`
	Encrypted  *bool  `json:"encrypted,omitempty"`
}

type ltEbsEncryptionByDefault struct {
	readOutcome
	Enabled bool `json:"enabled"`
}

type ltMetadataDefaults struct {
	readOutcome
	HttpTokens         string `json:"http_tokens,omitempty"`
	HttpEndpoint       string `json:"http_endpoint,omitempty"`
	HttpTokensEnforced string `json:"http_tokens_enforced,omitempty"`
}

type ltAMI struct {
	ImageId string `json:"image_id"`
	readOutcome
	ImdsSupport string `json:"imds_support,omitempty"`
}

func captureLT(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var ids []string
	listed := map[string]ec2types.LaunchTemplate{}
	pager := ec2.NewDescribeLaunchTemplatesPaginator(client, &ec2.DescribeLaunchTemplatesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range out.LaunchTemplates {
			ids = append(ids, aws.ToString(t.LaunchTemplateId))
			listed[aws.ToString(t.LaunchTemplateId)] = t
		}
	}

	var data ltData
	for _, row := range describeListed(ids, 1, captureLTDefaultVersion(ctx, client)) {
		t := listed[row.id]
		tpl := ltTemplate{
			LaunchTemplateId:     row.id,
			LaunchTemplateName:   aws.ToString(t.LaunchTemplateName),
			CreatedBy:            aws.ToString(t.CreatedBy),
			CreateTime:           snapFormatTime(t.CreateTime),
			DefaultVersionNumber: aws.ToInt64(t.DefaultVersionNumber),
			LatestVersionNumber:  aws.ToInt64(t.LatestVersionNumber),
			DefaultVersion:       ltVersion{readOutcome: row.read},
		}
		if v := row.item; v != nil {
			tpl.DefaultVersion.VersionNumber = aws.ToInt64(v.VersionNumber)
			tpl.DefaultVersion.DefaultVersionTime = snapFormatTime(v.CreateTime)
			if d := v.LaunchTemplateData; d != nil {
				tpl.DefaultVersion.ImageId = aws.ToString(d.ImageId)
				tpl.DefaultVersion.HasUserData = aws.ToString(d.UserData) != ""
				if md := d.MetadataOptions; md != nil {
					tpl.DefaultVersion.HttpTokens = string(md.HttpTokens)
					tpl.DefaultVersion.HttpEndpoint = string(md.HttpEndpoint)
				}
				for _, bdm := range d.BlockDeviceMappings {
					if bdm.Ebs != nil {
						tpl.DefaultVersion.BlockDevices = append(tpl.DefaultVersion.BlockDevices, ltBlockDevice{DeviceName: aws.ToString(bdm.DeviceName), Encrypted: bdm.Ebs.Encrypted})
					}
				}
			}
		}
		data.Templates = append(data.Templates, tpl)
	}
	captureLTDeferredSettings(ctx, client, &data)
	return data, nil
}

func captureLTDefaultVersion(ctx context.Context, client *ec2.Client) func([]string) (map[string]ec2types.LaunchTemplateVersion, []string, error) {
	return func(batch []string) (map[string]ec2types.LaunchTemplateVersion, []string, error) {
		versions, err := pageAll(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: aws.String(batch[0]),
			Versions:         []string{"$Default"},
		},
			func(ctx context.Context, in *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
				return client.DescribeLaunchTemplateVersions(ctx, in)
			},
			func(out *ec2.DescribeLaunchTemplateVersionsOutput) []ec2types.LaunchTemplateVersion {
				return out.LaunchTemplateVersions
			}, onePage)
		if err != nil || len(versions) == 0 {
			return nil, nil, err
		}
		return describedAs(batch[0], &versions[0]), nil, nil
	}
}

// captureLTDeferredSettings reads what a template leaves to the Region or the
// AMI, as the template checks do: the Region's EBS encryption by default when
// a template sets Encrypted=false (the Region setting overrides it), and,
// when a template leaves the metadata service reachable without required
// tokens, the account's instance metadata defaults and the ImdsSupport of the
// AMIs of templates that set no HttpTokens.
func captureLTDeferredSettings(ctx context.Context, client *ec2.Client, data *ltData) {
	var off, open bool
	var amis []string
	for _, t := range data.Templates {
		v := t.DefaultVersion
		if slices.ContainsFunc(v.BlockDevices, func(b ltBlockDevice) bool { return b.Encrypted != nil && !*b.Encrypted }) {
			off = true
		}
		if v.Outcome != "ok" || v.HttpEndpoint == string(ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled) || v.HttpTokens == string(ec2types.LaunchTemplateHttpTokensStateRequired) {
			continue
		}
		open = true
		if v.HttpTokens == "" && strings.HasPrefix(v.ImageId, "ami-") && !slices.Contains(amis, v.ImageId) {
			amis = append(amis, v.ImageId)
		}
	}
	if off {
		out, err := client.GetEbsEncryptionByDefault(ctx, &ec2.GetEbsEncryptionByDefaultInput{})
		data.EbsEncryptionByDefault = &ltEbsEncryptionByDefault{readOutcome: readOf(err)}
		if out != nil {
			data.EbsEncryptionByDefault.Enabled = aws.ToBool(out.EbsEncryptionByDefault)
		}
	}
	if !open {
		return
	}
	out, err := client.GetInstanceMetadataDefaults(ctx, &ec2.GetInstanceMetadataDefaultsInput{})
	data.InstanceMetadataDefaults = &ltMetadataDefaults{readOutcome: readOf(err)}
	if out != nil && out.AccountLevel != nil {
		data.InstanceMetadataDefaults.HttpTokens = string(out.AccountLevel.HttpTokens)
		data.InstanceMetadataDefaults.HttpEndpoint = string(out.AccountLevel.HttpEndpoint)
		data.InstanceMetadataDefaults.HttpTokensEnforced = string(out.AccountLevel.HttpTokensEnforced)
	}
	if len(amis) == 0 {
		return
	}
	slices.Sort(amis)
	// 200 ids per request, as the template check asks. The image-id filter
	// answers an id it cannot find with nothing, where ImageIds fails the
	// whole call.
	rows := describeListed(amis, 200, func(chunk []string) (map[string]ec2types.Image, []string, error) {
		images, err := pageAll(ctx, &ec2.DescribeImagesInput{
			Filters:           []ec2types.Filter{{Name: aws.String("image-id"), Values: chunk}},
			IncludeDeprecated: aws.Bool(true),
			IncludeDisabled:   aws.Bool(true),
		},
			func(ctx context.Context, in *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
				return client.DescribeImages(ctx, in)
			},
			func(out *ec2.DescribeImagesOutput) []ec2types.Image { return out.Images }, allPages)
		if err != nil {
			return nil, nil, err
		}
		found := map[string]ec2types.Image{}
		for _, img := range images {
			found[aws.ToString(img.ImageId)] = img
		}
		return found, nil, nil
	})
	for _, row := range rows {
		ami := ltAMI{ImageId: row.id, readOutcome: row.read}
		if row.item != nil {
			ami.ImdsSupport = string(row.item.ImdsSupport)
		}
		data.AMIs = append(data.AMIs, ami)
	}
}

type vpcPeerData struct {
	Connections []vpcPeerConnection `json:"vpc_peering_connections"`
}

type vpcPeerConnection struct {
	VpcPeeringConnectionId string            `json:"vpc_peering_connection_id"`
	StatusCode             string            `json:"status_code,omitempty"`
	StatusMessage          string            `json:"status_message,omitempty"`
	ExpirationTime         string            `json:"expiration_time,omitempty"`
	Requester              vpcPeerSide       `json:"requester"`
	Accepter               vpcPeerSide       `json:"accepter"`
	Tags                   map[string]string `json:"tags,omitempty"`
}

type vpcPeerSide struct {
	VpcId      string   `json:"vpc_id,omitempty"`
	OwnerId    string   `json:"owner_id,omitempty"`
	CidrBlocks []string `json:"cidr_blocks,omitempty"`
}

func captureVPCPeer(ctx context.Context, cfg aws.Config) (any, error) {
	client := ec2.NewFromConfig(cfg)

	var conns []vpcPeerConnection
	pager := ec2.NewDescribeVpcPeeringConnectionsPaginator(client, &ec2.DescribeVpcPeeringConnectionsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, pc := range out.VpcPeeringConnections {
			c := vpcPeerConnection{
				VpcPeeringConnectionId: aws.ToString(pc.VpcPeeringConnectionId),
				ExpirationTime:         snapFormatTime(pc.ExpirationTime),
			}
			if pc.Status != nil {
				c.StatusCode = string(pc.Status.Code)
				c.StatusMessage = aws.ToString(pc.Status.Message)
			}
			if i := pc.RequesterVpcInfo; i != nil {
				c.Requester = vpcPeerSide{VpcId: aws.ToString(i.VpcId), OwnerId: aws.ToString(i.OwnerId)}
				for _, b := range i.CidrBlockSet {
					c.Requester.CidrBlocks = append(c.Requester.CidrBlocks, aws.ToString(b.CidrBlock))
				}
			}
			if i := pc.AccepterVpcInfo; i != nil {
				c.Accepter = vpcPeerSide{VpcId: aws.ToString(i.VpcId), OwnerId: aws.ToString(i.OwnerId)}
				for _, b := range i.CidrBlockSet {
					c.Accepter.CidrBlocks = append(c.Accepter.CidrBlocks, aws.ToString(b.CidrBlock))
				}
			}
			if len(pc.Tags) > 0 {
				c.Tags = make(map[string]string, len(pc.Tags))
				for _, tag := range pc.Tags {
					c.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}
			}
			conns = append(conns, c)
		}
	}

	return vpcPeerData{Connections: conns}, nil
}

type transferData struct {
	Servers []transferServer `json:"servers"`
}

type transferServer struct {
	ServerId string `json:"server_id"`
	State    string `json:"state,omitempty"`
	readOutcome
	Domain               string `json:"domain,omitempty"`
	EndpointType         string `json:"endpoint_type,omitempty"`
	IdentityProviderType string `json:"identity_provider_type,omitempty"`
	SecurityPolicyName   string `json:"security_policy_name,omitempty"`
	LoggingRole          string `json:"logging_role,omitempty"`
	StructuredLogDests   int    `json:"structured_log_destinations,omitempty"`
	UserCount            int32  `json:"user_count,omitempty"`
}

func captureTransfer(ctx context.Context, cfg aws.Config) (any, error) {
	client := transfer.NewFromConfig(cfg)

	var ids []string
	listedState := map[string]string{}
	pager := transfer.NewListServersPaginator(client, &transfer.ListServersInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.Servers {
			ids = append(ids, aws.ToString(s.ServerId))
			listedState[aws.ToString(s.ServerId)] = string(s.State)
		}
	}

	rows := describeListed(ids, 1, func(batch []string) (map[string]transfertypes.DescribedServer, []string, error) {
		out, err := client.DescribeServer(ctx, &transfer.DescribeServerInput{ServerId: aws.String(batch[0])})
		if err != nil {
			return nil, nil, err
		}
		return describedAs(batch[0], out.Server), nil, nil
	})
	servers := make([]transferServer, 0, len(rows))
	for _, row := range rows {
		srv := transferServer{ServerId: row.id, State: listedState[row.id], readOutcome: row.read}
		if d := row.item; d != nil {
			srv.State = string(d.State)
			srv.Domain = string(d.Domain)
			srv.EndpointType = string(d.EndpointType)
			srv.IdentityProviderType = string(d.IdentityProviderType)
			srv.SecurityPolicyName = aws.ToString(d.SecurityPolicyName)
			srv.LoggingRole = aws.ToString(d.LoggingRole)
			srv.StructuredLogDests = len(d.StructuredLogDestinations)
			srv.UserCount = aws.ToInt32(d.UserCount)
		}
		servers = append(servers, srv)
	}

	return transferData{Servers: servers}, nil
}

type mwaaData struct {
	Environments []mwaaEnvironment `json:"environments"`
}

// mwaaEnvironment carries the GetEnvironment outcome per listed name.
type mwaaEnvironment struct {
	Name string `json:"name"`
	readOutcome
	Status              string `json:"status,omitempty"`
	AirflowVersion      string `json:"airflow_version,omitempty"`
	EnvironmentClass    string `json:"environment_class,omitempty"`
	MaxWorkers          int32  `json:"max_workers,omitempty"`
	Schedulers          int32  `json:"schedulers,omitempty"`
	WebserverAccessMode string `json:"webserver_access_mode,omitempty"`
	LastUpdateStatus    string `json:"last_update_status,omitempty"`
	LastUpdateError     string `json:"last_update_error_code,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
}

func captureMWAA(ctx context.Context, cfg aws.Config) (any, error) {
	client := mwaa.NewFromConfig(cfg)

	var names []string
	pager := mwaa.NewListEnvironmentsPaginator(client, &mwaa.ListEnvironmentsInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		names = append(names, out.Environments...)
	}

	rows := describeListed(names, 1, func(batch []string) (map[string]mwaatypes.Environment, []string, error) {
		out, err := client.GetEnvironment(ctx, &mwaa.GetEnvironmentInput{Name: aws.String(batch[0])})
		if err != nil {
			return nil, nil, err
		}
		return describedAs(batch[0], out.Environment), nil, nil
	})
	envs := make([]mwaaEnvironment, 0, len(rows))
	for _, row := range rows {
		env := mwaaEnvironment{Name: row.id, readOutcome: row.read}
		if e := row.item; e != nil {
			env.Status = string(e.Status)
			env.AirflowVersion = aws.ToString(e.AirflowVersion)
			env.EnvironmentClass = aws.ToString(e.EnvironmentClass)
			env.MaxWorkers = aws.ToInt32(e.MaxWorkers)
			env.Schedulers = aws.ToInt32(e.Schedulers)
			env.WebserverAccessMode = string(e.WebserverAccessMode)
			env.CreatedAt = snapFormatTime(e.CreatedAt)
			if e.LastUpdate != nil {
				env.LastUpdateStatus = string(e.LastUpdate.Status)
				if e.LastUpdate.Error != nil {
					env.LastUpdateError = aws.ToString(e.LastUpdate.Error.ErrorCode)
				}
			}
		}
		envs = append(envs, env)
	}

	return mwaaData{Environments: envs}, nil
}
