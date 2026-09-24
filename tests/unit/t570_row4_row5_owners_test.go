package unit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t570ECS struct {
	awsclient.ECSAPI
	family string
	edit   func(*ecstypes.TaskDefinition)
}

func (f *t570ECS) DescribeTaskDefinition(ctx context.Context, in *ecs.DescribeTaskDefinitionInput, opt ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	out, err := f.ECSAPI.DescribeTaskDefinition(ctx, in, opt...)
	if err != nil || out.TaskDefinition == nil || aws.ToString(out.TaskDefinition.Family) != f.family {
		return out, err
	}
	td := *out.TaskDefinition
	td.ContainerDefinitions = append([]ecstypes.ContainerDefinition(nil), td.ContainerDefinitions...)
	f.edit(&td)
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &td, Tags: out.Tags}, nil
}

// t570SecretRefs are the demo secrets and parameters one task definition
// injects, each in a shape ECS accepts.
type t570SecretRefs struct {
	envSecret, logSecret resource.Resource
	byName, byARN        resource.Resource
}

func t570SSMARN(name string) string {
	return "arn:aws:ssm:us-east-1:123456789012:parameter/" + strings.TrimPrefix(name, "/")
}

// t570ECSWithSecrets rewrites the api-gateway task definition's references:
// a Secrets Manager secret by full ARN with a JSON-key suffix, a log-driver
// secret (LogConfiguration.SecretOptions), an SSM parameter by name (same
// Region) and one by ARN. Secret.valueFrom takes a Secrets Manager secret by
// full ARN only; a bare name is an SSM parameter (ECS API_Secret).
func t570ECSWithSecrets(t *testing.T) (*awsclient.ServiceClients, t570SecretRefs) {
	t.Helper()
	c := t570Demo()
	secrets := t570List(t, c, "secrets")
	params := t570List(t, c, "ssm")
	refs := t570SecretRefs{envSecret: secrets[1], logSecret: secrets[2], byName: params[0], byARN: params[1]}
	c.ECS = &t570ECS{ECSAPI: c.ECS, family: "api-gateway", edit: func(td *ecstypes.TaskDefinition) {
		cd := td.ContainerDefinitions[0]
		cd.Secrets = []ecstypes.Secret{
			{Name: aws.String("DB_PASSWORD"), ValueFrom: aws.String(refs.envSecret.Fields["arn"] + ":password::")},
			{Name: aws.String("APP_CONFIG"), ValueFrom: aws.String(refs.byName.ID)},
			{Name: aws.String("FEATURE_FLAGS"), ValueFrom: aws.String(t570SSMARN(refs.byARN.ID))},
		}
		cd.LogConfiguration = &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverSplunk,
			Options:   map[string]string{"splunk-url": "https://splunk.example.com:8088"},
			SecretOptions: []ecstypes.Secret{
				{Name: aws.String("splunk-token"), ValueFrom: aws.String(refs.logSecret.Fields["arn"])},
			},
		}
		cd.RepositoryCredentials = nil
		td.ContainerDefinitions = []ecstypes.ContainerDefinition{cd}
	}}
	return c, refs
}

func t570TaskRunning(t *testing.T, c *awsclient.ServiceClients, family string) resource.Resource {
	t.Helper()
	for _, r := range t570List(t, c, "ecs-task") {
		if task, ok := r.RawStruct.(ecstypes.Task); ok && strings.Contains(aws.ToString(task.TaskDefinitionArn), "task-definition/"+family+":") {
			return r
		}
	}
	t.Fatalf("no demo task runs %s", family)
	return resource.Resource{}
}

// Both service and task read the same classification: the environment
// secret (JSON-key suffix stripped) and the log-driver secret are Secrets
// Manager secrets; the bare name and the parameter ARN are SSM parameters.
func TestT570_ECSSecretRefs_ServiceAndTaskAgree(t *testing.T) {
	c, refs := t570ECSWithSecrets(t)
	svc := t570Row(t, c, "ecs-svc", "acme-services/api-gateway")
	t570Exact(t, t570Pivot(t, c, svc, "ecs-svc", "secrets"), refs.envSecret.ID, refs.logSecret.ID)

	task := t570TaskRunning(t, c, "api-gateway")
	t570Exact(t, t570Pivot(t, c, task, "ecs-task", "secrets"), refs.envSecret.ID, refs.logSecret.ID)
	t570Exact(t, t570Pivot(t, c, task, "ecs-task", "ssm"), refs.byName.ID, refs.byARN.ID)
}

// The reverse pivot reads the same classifier: a secret injected into a log
// driver, or through a JSON key, is used by the task.
func TestT570_SecretsECSTask_ReadsLogSecretsAndJSONKeyRefs(t *testing.T) {
	c, refs := t570ECSWithSecrets(t)
	task := t570TaskRunning(t, c, "api-gateway")
	for _, s := range []resource.Resource{refs.envSecret, refs.logSecret} {
		t570Contains(t, t570Pivot(t, c, s, "secrets", "ecs-task"), []string{task.ID}, nil)
	}
}

const (
	t570LTA        = "lt-0e570aaaaaaaaaaa1"
	t570LTB        = "lt-0e570bbbbbbbbbbb2"
	t570LTBName    = "acme-web-arm"
	t570AMIA       = "ami-0a1b2c3d4e5f60001"
	t570AMIB       = "ami-0a1b2c3d4e5f60002"
	t570SGA        = "sg-0aaa111111111111a"
	t570SGB        = "sg-0bbb222222222222b"
	t570SGLC       = "sg-0ddd444444444444d"
	t570RoleA      = "acme-eks-node-role"
	t570RoleB      = "acme-ci-deploy-role"
	t570ASGLT      = "acme-web-lt-asg"
	t570ASGLC      = "acme-web-lc-asg"
	t570ASGMixed   = "acme-web-mixed-asg"
	t570LCName     = "acme-web-legacy-lc"
	t570ProfileA   = "arn:aws:iam::123456789012:instance-profile/acme-web-profile"
	t570ProfileBNm = "acme-web-arm-profile"
)

type t570LTData struct {
	id, name string
	data     ec2types.ResponseLaunchTemplateData
}

var t570LaunchTemplates = []t570LTData{
	{id: t570LTA, name: "acme-web", data: ec2types.ResponseLaunchTemplateData{
		ImageId:            aws.String(t570AMIA),
		InstanceType:       ec2types.InstanceTypeM6iLarge,
		IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecification{Arn: aws.String(t570ProfileA)},
		SecurityGroupIds:   []string{t570SGA},
	}},
	{id: t570LTB, name: t570LTBName, data: ec2types.ResponseLaunchTemplateData{
		ImageId:            aws.String(t570AMIB),
		InstanceType:       ec2types.InstanceTypeM7gLarge,
		IamInstanceProfile: &ec2types.LaunchTemplateIamInstanceProfileSpecification{Name: aws.String(t570ProfileBNm)},
		SecurityGroupIds:   []string{t570SGB},
	}},
}

type t570EC2LT struct {
	awsclient.EC2API
}

// DescribeLaunchTemplateVersions finds a template by id or by name, as EC2
// does.
func (f *t570EC2LT) DescribeLaunchTemplateVersions(ctx context.Context, in *ec2.DescribeLaunchTemplateVersionsInput, opt ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	for _, lt := range t570LaunchTemplates {
		if aws.ToString(in.LaunchTemplateId) == lt.id || aws.ToString(in.LaunchTemplateName) == lt.name {
			data := lt.data
			return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{{
				LaunchTemplateId:   aws.String(lt.id),
				LaunchTemplateName: aws.String(lt.name),
				VersionNumber:      aws.Int64(3),
				DefaultVersion:     aws.Bool(true),
				LaunchTemplateData: &data,
			}}}, nil
		}
	}
	return f.EC2API.DescribeLaunchTemplateVersions(ctx, in, opt...)
}

type t570ASG struct {
	awsclient.ASGAPI
}

func t570Group(name string, edit func(*asgtypes.AutoScalingGroup)) asgtypes.AutoScalingGroup {
	g := asgtypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String(name),
		AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:0e570000-0000-4000-8000-000000000000:autoScalingGroupName/" + name),
		MinSize:              aws.Int32(0),
		MaxSize:              aws.Int32(6),
		DesiredCapacity:      aws.Int32(0),
		DefaultCooldown:      aws.Int32(300),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		HealthCheckType:      aws.String("EC2"),
		CreatedTime:          aws.Time(time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)),
		VPCZoneIdentifier:    aws.String("subnet-0f10a1b2c3d4e5f60"),
		ServiceLinkedRoleARN: aws.String("arn:aws:iam::123456789012:role/aws-service-role/autoscaling.amazonaws.com/AWSServiceRoleForAutoScaling"),
	}
	edit(&g)
	return g
}

// Three groups scaled to zero, one per launch source: a launch template, a
// launch configuration, and a mixed-instances policy whose override names a
// second template by name.
var t570Groups = []asgtypes.AutoScalingGroup{
	t570Group(t570ASGLT, func(g *asgtypes.AutoScalingGroup) {
		g.LaunchTemplate = &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String(t570LTA), Version: aws.String("$Latest")}
	}),
	t570Group(t570ASGLC, func(g *asgtypes.AutoScalingGroup) {
		g.LaunchConfigurationName = aws.String(t570LCName)
	}),
	t570Group(t570ASGMixed, func(g *asgtypes.AutoScalingGroup) {
		g.MixedInstancesPolicy = &asgtypes.MixedInstancesPolicy{
			LaunchTemplate: &asgtypes.LaunchTemplate{
				LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{LaunchTemplateId: aws.String(t570LTA), Version: aws.String("$Default")},
				Overrides: []asgtypes.LaunchTemplateOverrides{
					{InstanceType: aws.String("m6i.large")},
					{InstanceType: aws.String("m7g.large"), LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{LaunchTemplateName: aws.String(t570LTBName), Version: aws.String("$Latest")}},
				},
			},
			InstancesDistribution: &asgtypes.InstancesDistribution{OnDemandBaseCapacity: aws.Int32(0), SpotAllocationStrategy: aws.String("price-capacity-optimized")},
		}
	}),
}

func (f *t570ASG) DescribeAutoScalingGroups(ctx context.Context, in *autoscaling.DescribeAutoScalingGroupsInput, opt ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	var mine []asgtypes.AutoScalingGroup
	for _, g := range t570Groups {
		if len(in.AutoScalingGroupNames) == 0 || t570In(aws.ToString(g.AutoScalingGroupName), in.AutoScalingGroupNames) {
			mine = append(mine, g)
		}
	}
	out, err := f.ASGAPI.DescribeAutoScalingGroups(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	if in.NextToken == nil {
		out.AutoScalingGroups = append(out.AutoScalingGroups, mine...)
	}
	return out, nil
}

func (f *t570ASG) DescribeLaunchConfigurations(ctx context.Context, in *autoscaling.DescribeLaunchConfigurationsInput, opt ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	if t570In(t570LCName, in.LaunchConfigurationNames) {
		return &autoscaling.DescribeLaunchConfigurationsOutput{LaunchConfigurations: []asgtypes.LaunchConfiguration{{
			LaunchConfigurationName: aws.String(t570LCName),
			ImageId:                 aws.String(t570AMIB),
			InstanceType:            aws.String("m5.large"),
			IamInstanceProfile:      aws.String(t570ProfileBNm),
			SecurityGroups:          []string{t570SGLC},
			CreatedTime:             aws.Time(time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)),
		}}}, nil
	}
	return f.ASGAPI.DescribeLaunchConfigurations(ctx, in, opt...)
}

type t570IAM struct {
	awsclient.IAMAPI
}

func (f *t570IAM) GetInstanceProfile(ctx context.Context, in *iam.GetInstanceProfileInput, opt ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	role := map[string]string{"acme-web-profile": t570RoleA, t570ProfileBNm: t570RoleB}[aws.ToString(in.InstanceProfileName)]
	if role == "" {
		return f.IAMAPI.GetInstanceProfile(ctx, in, opt...)
	}
	return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{
		InstanceProfileName: in.InstanceProfileName,
		InstanceProfileId:   aws.String("AIPAEXAMPLE570000000"),
		Arn:                 aws.String("arn:aws:iam::123456789012:instance-profile/" + aws.ToString(in.InstanceProfileName)),
		Path:                aws.String("/"),
		CreateDate:          aws.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		Roles: []iamtypes.Role{{
			RoleName:   aws.String(role),
			RoleId:     aws.String("AROAEXAMPLE570000000"),
			Arn:        aws.String("arn:aws:iam::123456789012:role/" + role),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		}},
	}}, nil
}

func t570ASGClients() *awsclient.ServiceClients {
	c := t570Demo()
	c.EC2 = &t570EC2LT{EC2API: c.EC2}
	c.AutoScaling = &t570ASG{ASGAPI: c.AutoScaling}
	c.IAM = &t570IAM{IAMAPI: c.IAM}
	return c
}

// An ASG's AMI, security groups and role come from every launch source it
// names: its launch template, its launch configuration, and each template a
// mixed-instances override names (Overrides[].LaunchTemplateSpecification,
// by id or by name).
func TestT570_ASGLaunchSources_AMIsSGsAndRoles(t *testing.T) {
	c := t570ASGClients()
	for _, tc := range []struct {
		asg              string
		amis, sgs, roles []string
	}{
		{t570ASGLT, []string{t570AMIA}, []string{t570SGA}, []string{t570RoleA}},
		{t570ASGLC, []string{t570AMIB}, []string{t570SGLC}, []string{t570RoleB}},
		{t570ASGMixed, []string{t570AMIA, t570AMIB}, []string{t570SGA, t570SGB}, []string{t570RoleA, t570RoleB}},
	} {
		t.Run(tc.asg, func(t *testing.T) {
			g := t570Row(t, c, "asg", tc.asg)
			t570Exact(t, t570Pivot(t, c, g, "asg", "ami"), tc.amis...)
			t570Exact(t, t570Pivot(t, c, g, "asg", "sg"), tc.sgs...)
			t570Contains(t, t570Pivot(t, c, g, "asg", "role"), tc.roles, nil)
		})
	}
}

// An AMI's ASGs are the groups whose launch sources name it, whether or not
// they run instances: a group at desired capacity 0 launches from the AMI the
// moment it scales out.
func TestT570_AMIASG_MatchesLaunchSourcesAtDesiredZero(t *testing.T) {
	c := t570ASGClients()
	a := t570Row(t, c, "ami", t570AMIA)
	t570Contains(t, t570Pivot(t, c, a, "ami", "asg"), []string{t570ASGLT, t570ASGMixed}, []string{t570ASGLC})
	b := t570Row(t, c, "ami", t570AMIB)
	t570Contains(t, t570Pivot(t, c, b, "ami", "asg"), []string{t570ASGLC, t570ASGMixed}, []string{t570ASGLT})
}
