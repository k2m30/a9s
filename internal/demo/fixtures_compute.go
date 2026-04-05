package demo

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["ec2"] = ec2Instances
	demoData["lambda"] = lambdaFunctions
	demoData["ebs"] = ebsVolumeFixtures
	demoData["ebs-snap"] = ebsSnapshotFixtures
	demoData["ami"] = amiFixtures
}

// ---------------------------------------------------------------------------
// EC2 Instances
// ---------------------------------------------------------------------------

// ec2Instances returns demo EC2 instance fixtures with populated RawStruct.
// Includes a mix of running/stopped/pending states and realistic naming for
// the demo scenario (filter /web must show results).
func ec2Instances() []resource.Resource {
	instances := []resource.Resource{
		makeEC2Instance(
			"i-0a1b2c3d4e5f60001", "web-prod-01", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.10", "54.210.33.112",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC),
			"",
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60002", "web-prod-02", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.11", "54.210.33.113",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 32, 0, 0, time.UTC),
			"",
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60003", "api-staging-01", "running",
			ec2types.InstanceTypeM5Xlarge, "10.0.2.50", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC),
			ec2types.InstanceLifecycleTypeSpot,
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60004", "worker-batch-03", "stopped",
			ec2types.InstanceTypeC5Xlarge, "10.0.3.100", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2025, 9, 5, 11, 0, 0, 0, time.UTC),
			ec2types.InstanceLifecycleTypeSpot,
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60005", "bastion-prod", "running",
			ec2types.InstanceTypeT3Micro, "10.0.0.5", "52.87.221.44",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC),
			"",
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60006", "db-proxy-01", "running",
			ec2types.InstanceTypeR5Large, "10.0.4.200", "",
			"vpc-0abc123def456789a", "subnet-0ddd444444444444d",
			time.Date(2025, 12, 10, 18, 45, 0, 0, time.UTC),
			"",
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60007", "web-staging-01", "pending",
			ec2types.InstanceTypeT3Medium, "10.0.2.70", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
			ec2types.InstanceLifecycleTypeSpot,
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60008", "ml-trainer-gpu", "stopping",
			ec2types.InstanceTypeG4dnXlarge, "10.0.5.30", "",
			"vpc-0abc123def456789a", "subnet-0eee555555555555e",
			time.Date(2026, 2, 14, 22, 0, 0, 0, time.UTC),
			ec2types.InstanceLifecycleTypeSpot,
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60009", "temp-load-test", "shutting-down",
			ec2types.InstanceTypeC5Large, "10.0.3.55", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2026, 3, 20, 16, 30, 0, 0, time.UTC),
			"",
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60010", "old-migration-worker", "terminated",
			ec2types.InstanceTypeT3Small, "", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC),
			"",
		),
	}

	// Generate 15 more instances to reach 25 total
	ec2Types := []ec2types.InstanceType{
		ec2types.InstanceTypeT3Medium, ec2types.InstanceTypeM5Large,
		ec2types.InstanceTypeC5Large, ec2types.InstanceTypeR5Large,
		ec2types.InstanceTypeT3Large, ec2types.InstanceTypeM5Xlarge,
		ec2types.InstanceTypeC5Xlarge, ec2types.InstanceTypeT3Small,
		ec2types.InstanceTypeR5Xlarge, ec2types.InstanceTypeT3Micro,
		ec2types.InstanceTypeM5Large, ec2types.InstanceTypeC5Large,
		ec2types.InstanceTypeT3Medium, ec2types.InstanceTypeR5Large,
		ec2types.InstanceTypeT3Large,
	}
	subnets := []string{
		prodPublicSubnetA, prodPublicSubnetB, prodPrivateSubnetA,
		prodPrivateSubnetB, stagingSubnetA, stagingSubnetB,
	}
	for i := 0; i < 15; i++ {
		idx := i + 11
		name := ec2NamePool[i%len(ec2NamePool)]
		state := ec2StatePool[i%len(ec2StatePool)]
		ip := fmt.Sprintf("10.0.%d.%d", (idx/10)+1, 10+idx)
		publicIP := ""
		if i%5 == 0 {
			publicIP = fmt.Sprintf("54.210.%d.%d", 34+i, 100+i)
		}
		instances = append(instances, makeEC2Instance(
			fmt.Sprintf("i-0a1b2c3d4e5f6%04d", idx),
			fmt.Sprintf("%s-%02d", name, idx),
			state,
			ec2Types[i],
			ip, publicIP,
			prodVPCID,
			subnets[i%len(subnets)],
			time.Date(2025, time.Month(6+(i%7)), 1+i, 8+i, 0, 0, 0, time.UTC),
			"",
		))
	}

	return instances
}

// ec2InstanceExtras holds the per-instance fields that are not part of the
// original makeEC2Instance signature but are required by navigable-field tests.
type ec2InstanceExtras struct {
	imageID        string
	keyName        string
	architecture   ec2types.ArchitectureValues
	az             string
	securityGroups []ec2types.GroupIdentifier
}

// ec2Extras maps instance IDs to their extra fixture data.
var ec2Extras = map[string]ec2InstanceExtras{
	"i-0a1b2c3d4e5f60001": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60002": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60003": {
		imageID:      prodAMIID2,
		keyName:      "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64,
		az:           "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60004": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0a1b2c3d4e5f60005": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	},
	"i-0a1b2c3d4e5f60006": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1c",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodDBProxySGID), GroupName: aws.String("acme-db-proxy-sg")},
		},
	},
	"i-0a1b2c3d4e5f60007": {
		imageID:      prodAMIID2,
		keyName:      "acme-staging-keypair",
		architecture: ec2types.ArchitectureValuesArm64,
		az:           "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
	"i-0a1b2c3d4e5f60008": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-api-internal-sg")},
		},
	},
	"i-0a1b2c3d4e5f60009": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1a",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodRDSSGID), GroupName: aws.String("acme-worker-sg")},
		},
	},
	"i-0a1b2c3d4e5f60010": {
		imageID:      prodAMIID1,
		keyName:      "acme-prod-keypair",
		architecture: ec2types.ArchitectureValuesX8664,
		az:           "us-east-1b",
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodAPIInternalSGID), GroupName: aws.String("acme-web-app-sg")},
		},
	},
}

// ec2DefaultExtras returns a fallback ec2InstanceExtras for generated instances.
func ec2DefaultExtras(instanceID string) ec2InstanceExtras {
	if ex, ok := ec2Extras[instanceID]; ok {
		return ex
	}
	// Derive deterministic values from the instance ID suffix.
	suffix := instanceID[len(instanceID)-4:]
	amiIDs := []string{prodAMIID1, prodAMIID2, prodAMIID3}
	keyNames := []string{"acme-prod-keypair", "acme-staging-keypair", "acme-dev-keypair"}
	archs := []ec2types.ArchitectureValues{ec2types.ArchitectureValuesX8664, ec2types.ArchitectureValuesArm64}
	azs := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	idx := 0
	for _, ch := range suffix {
		idx += int(ch)
	}
	return ec2InstanceExtras{
		imageID:      amiIDs[idx%len(amiIDs)],
		keyName:      keyNames[idx%len(keyNames)],
		architecture: archs[idx%len(archs)],
		az:           azs[idx%len(azs)],
		securityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String(prodWebALBSGID), GroupName: aws.String("acme-web-alb-sg")},
		},
	}
}

// makeEC2Instance constructs a resource.Resource with a fully populated
// ec2types.Instance as RawStruct. This enables both detail and YAML views
// in demo mode.
func makeEC2Instance(
	instanceID, name, state string,
	instanceType ec2types.InstanceType,
	privateIP, publicIP string,
	vpcID, subnetID string,
	launchTime time.Time,
	lifecycle ec2types.InstanceLifecycleType,
) resource.Resource {
	stateName := ec2types.InstanceStateName(state)
	stateCode := stateNameToCode(stateName)
	extras := ec2DefaultExtras(instanceID)

	inst := ec2types.Instance{
		InstanceId:       aws.String(instanceID),
		InstanceType:     instanceType,
		PrivateIpAddress: aws.String(privateIP),
		State: &ec2types.InstanceState{
			Name: stateName,
			Code: aws.Int32(stateCode),
		},
		VpcId:          aws.String(vpcID),
		SubnetId:       aws.String(subnetID),
		ImageId:        aws.String(extras.imageID),
		KeyName:        aws.String(extras.keyName),
		Architecture:   extras.architecture,
		Placement:      &ec2types.Placement{AvailabilityZone: aws.String(extras.az)},
		SecurityGroups: extras.securityGroups,
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: aws.String(prodInstanceProfileARN),
		},
		BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/xvda"),
				Ebs: &ec2types.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeIDForInstance(instanceID)),
					Status:   ec2types.AttachmentStatusAttached,
				},
			},
		},
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String(name)},
			{Key: aws.String("Environment"), Value: aws.String(envFromName(name))},
		},
		LaunchTime:        aws.Time(launchTime),
		InstanceLifecycle: lifecycle,
		EbsOptimized:      aws.Bool(true),
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			State:                   ec2types.InstanceMetadataOptionsStateApplied,
			HttpEndpoint:            ec2types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:              ec2types.HttpTokensStateRequired,
			HttpPutResponseHopLimit: aws.Int32(2),
		},
		PrivateDnsName: aws.String(privateDNSFromIP(privateIP)),
	}
	if instanceID == "i-0a1b2c3d4e5f60001" {
		inst.Platform = ec2types.PlatformValuesWindows
		inst.Tags = append(inst.Tags, ec2types.Tag{
			Key:   aws.String("kubernetes.io/cluster/" + prodEKSClusterName),
			Value: aws.String("owned"),
		})
	}
	if instanceID == "i-0a1b2c3d4e5f60003" {
		inst.Tags = append(inst.Tags,
			ec2types.Tag{Key: aws.String("eks:cluster-name"), Value: aws.String(prodEKSClusterName)},
			ec2types.Tag{Key: aws.String("eks:nodegroup-name"), Value: aws.String(relatedEC2NGNodeGroupID)},
		)
	}

	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}

	launchTimeStr := launchTime.Format("2006-01-02 15:04")

	lifecycleStr := "on-demand"
	if lifecycle != "" {
		lifecycleStr = string(lifecycle)
	}

	return resource.Resource{
		ID:     instanceID,
		Name:   name,
		Status: state,
		Fields: map[string]string{
			"instance_id": instanceID,
			"name":        name,
			"state":       state,
			"type":        string(instanceType),
			"private_ip":  privateIP,
			"public_ip":   publicIP,
			"launch_time": launchTimeStr,
			"lifecycle":   lifecycleStr,
			"image_id":    extras.imageID,
			"vpc_id":      vpcID,
		},
		RawStruct: inst,
	}
}

// stateNameToCode maps EC2 instance state names to their numeric codes.
func stateNameToCode(name ec2types.InstanceStateName) int32 {
	switch name {
	case ec2types.InstanceStateNamePending:
		return 0
	case ec2types.InstanceStateNameRunning:
		return 16
	case ec2types.InstanceStateNameShuttingDown:
		return 32
	case ec2types.InstanceStateNameTerminated:
		return 48
	case ec2types.InstanceStateNameStopping:
		return 64
	case ec2types.InstanceStateNameStopped:
		return 80
	default:
		return -1
	}
}

// envFromName infers an environment tag from the instance name.
func envFromName(name string) string {
	for _, prefix := range []string{"prod", "staging", "dev"} {
		for i := 0; i <= len(name)-len(prefix); i++ {
			if name[i:i+len(prefix)] == prefix {
				return prefix
			}
		}
	}
	return "prod"
}

// privateDNSFromIP converts a private IP to EC2 internal DNS format.
// e.g. "10.0.1.10" -> "ip-10-0-1-10.ec2.internal"
func privateDNSFromIP(ip string) string {
	dashed := ""
	for _, ch := range ip {
		if ch == '.' {
			dashed += "-"
		} else {
			dashed += string(ch)
		}
	}
	return "ip-" + dashed + ".ec2.internal"
}

func volumeIDForInstance(instanceID string) string {
	// Keep volume references deterministic and aligned with snapshot fixtures.
	// This powers EC2 -> EBS Snapshot demo dependencies.
	volIDs := []string{
		"vol-0a1b2c3d4e5f60001",
		"vol-0a1b2c3d4e5f60002",
		"vol-0a1b2c3d4e5f60003",
		"vol-0a1b2c3d4e5f60005",
	}
	idx := 0
	for _, ch := range instanceID {
		idx += int(ch)
	}
	return volIDs[idx%len(volIDs)]
}

// lambdaFunctions returns demo Lambda function fixtures.
func lambdaFunctions() []resource.Resource {
	fns := []resource.Resource{
		{
			ID:     "api-gateway-authorizer",
			Name:   "api-gateway-authorizer",
			Status: "nodejs20.x",
			Fields: map[string]string{
				"function_name": "api-gateway-authorizer",
				"runtime":       "nodejs20.x",
				"memory":        "256",
				"timeout":       "10",
				"handler":       "index.handler",
				"last_modified": "2026-03-15T08:22:14+00:00",
				"code_size":     "1048576",
				"log_group":     "/aws/lambda/api-gateway-authorizer",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("api-gateway-authorizer"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimeNodejs20x,
				MemorySize:   aws.Int32(256),
				Timeout:      aws.Int32(10),
				Handler:      aws.String("index.handler"),
				Description:  aws.String("API Gateway custom authorizer"),
				LastModified: aws.String("2026-03-15T08:22:14+00:00"),
				CodeSize:     1048576,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/api-gateway-authorizer"),
					LogFormat: lambdatypes.LogFormatText,
				},
				DeadLetterConfig: &lambdatypes.DeadLetterConfig{
					TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
				},
				Environment: &lambdatypes.EnvironmentResponse{
					Variables: map[string]string{"ENV": "production", "LOG_LEVEL": "INFO"},
				},
				LastUpdateStatus:       lambdatypes.LastUpdateStatusSuccessful,
				LastUpdateStatusReason: aws.String("Function update succeeded"),
				Layers: []lambdatypes.Layer{
					{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:layer:acme-common:3"), CodeSize: 1048576},
				},
				VpcConfig: &lambdatypes.VpcConfigResponse{
					VpcId:            aws.String(prodVPCID),
					SubnetIds:        []string{prodPublicSubnetA},
					SecurityGroupIds: []string{prodWebALBSGID},
				},
			},
		},
		{
			ID:     "data-pipeline-transform",
			Name:   "data-pipeline-transform",
			Status: "python3.12",
			Fields: map[string]string{
				"function_name": "data-pipeline-transform",
				"runtime":       "python3.12",
				"memory":        "512",
				"timeout":       "300",
				"handler":       "transform.lambda_handler",
				"last_modified": "2026-03-10T16:45:33+00:00",
				"code_size":     "5242880",
				"log_group":     "/aws/lambda/data-pipeline-transform",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("data-pipeline-transform"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimePython312,
				MemorySize:   aws.Int32(512),
				Timeout:      aws.Int32(300),
				Handler:      aws.String("transform.lambda_handler"),
				Description:  aws.String("ETL data pipeline transformation function"),
				LastModified: aws.String("2026-03-10T16:45:33+00:00"),
				CodeSize:     5242880,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/data-pipeline-transform"),
					LogFormat: lambdatypes.LogFormatText,
				},
			},
		},
		{
			ID:     lambdaProcessOrdersFnName,
			Name:   lambdaProcessOrdersFnName,
			Status: "go1.x",
			Fields: map[string]string{
				"function_name":    lambdaProcessOrdersFnName,
				"runtime":          "go1.x",
				"memory":           "128",
				"timeout":          "30",
				"handler":          "main",
				"last_modified":    "2026-02-28T11:03:47+00:00",
				"code_size":        "8388608",
				"log_group":        "/aws/lambda/" + lambdaProcessOrdersFnName,
				"package_type":     "Zip",
				"event_source_arn": "arn:aws:sqs:us-east-1:123456789012:order-processing-queue",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String(lambdaProcessOrdersFnName),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaProcessOrdersFnName),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimeGo1x,
				MemorySize:   aws.Int32(128),
				Timeout:      aws.Int32(30),
				Handler:      aws.String("main"),
				Description:  aws.String("Order processing Lambda triggered by SQS"),
				LastModified: aws.String("2026-02-28T11:03:47+00:00"),
				CodeSize:     8388608,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/" + lambdaProcessOrdersFnName),
					LogFormat: lambdatypes.LogFormatText,
				},
			},
		},
		{
			ID:     "image-thumbnail-gen",
			Name:   "image-thumbnail-gen",
			Status: "python3.12",
			Fields: map[string]string{
				"function_name": "image-thumbnail-gen",
				"runtime":       "python3.12",
				"memory":        "1024",
				"timeout":       "60",
				"handler":       "thumbnail.handler",
				"last_modified": "2026-03-01T09:18:55+00:00",
				"code_size":     "15728640",
				"log_group":     "/aws/lambda/image-thumbnail-gen",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("image-thumbnail-gen"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen"),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimePython312,
				MemorySize:   aws.Int32(1024),
				Timeout:      aws.Int32(60),
				Handler:      aws.String("thumbnail.handler"),
				Description:  aws.String("S3-triggered image thumbnail generator"),
				LastModified: aws.String("2026-03-01T09:18:55+00:00"),
				CodeSize:     15728640,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/image-thumbnail-gen"),
					LogFormat: lambdatypes.LogFormatText,
				},
			},
		},
		{
			ID:     "payment-webhook",
			Name:   "payment-webhook",
			Status: "java21",
			Fields: map[string]string{
				"function_name": "payment-webhook",
				"runtime":       "java21",
				"memory":        "512",
				"timeout":       "15",
				"handler":       "com.example.PaymentHandler::handleRequest",
				"last_modified": "2026-03-12T20:11:09+00:00",
				"code_size":     "31457280",
				"log_group":     "/aws/lambda/payment-webhook",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("payment-webhook"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:payment-webhook"),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimeJava21,
				MemorySize:   aws.Int32(512),
				Timeout:      aws.Int32(15),
				Handler:      aws.String("com.example.PaymentHandler::handleRequest"),
				Description:  aws.String("Payment provider webhook handler"),
				LastModified: aws.String("2026-03-12T20:11:09+00:00"),
				CodeSize:     31457280,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/payment-webhook"),
					LogFormat: lambdatypes.LogFormatText,
				},
			},
		},
		{
			ID:     "cloudwatch-slack-notifier",
			Name:   "cloudwatch-slack-notifier",
			Status: "nodejs20.x",
			Fields: map[string]string{
				"function_name": "cloudwatch-slack-notifier",
				"runtime":       "nodejs20.x",
				"memory":        "128",
				"timeout":       "10",
				"handler":       "notify.handler",
				"last_modified": "2026-01-20T13:42:00+00:00",
				"code_size":     "524288",
				"log_group":     "/aws/lambda/cloudwatch-slack-notifier",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("cloudwatch-slack-notifier"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      lambdatypes.RuntimeNodejs20x,
				MemorySize:   aws.Int32(128),
				Timeout:      aws.Int32(10),
				Handler:      aws.String("notify.handler"),
				Description:  aws.String("CloudWatch alarm to Slack notification relay"),
				LastModified: aws.String("2026-01-20T13:42:00+00:00"),
				CodeSize:     524288,
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				LoggingConfig: &lambdatypes.LoggingConfig{
					LogGroup:  aws.String("/aws/lambda/cloudwatch-slack-notifier"),
					LogFormat: lambdatypes.LogFormatText,
				},
			},
		},
	}

	// Generate 19 more functions to reach 25 total
	runtimeMap := map[string]lambdatypes.Runtime{
		"nodejs20.x": lambdatypes.RuntimeNodejs20x,
		"python3.12": lambdatypes.RuntimePython312,
		"go1.x":      lambdatypes.RuntimeGo1x,
		"java21":     lambdatypes.RuntimeJava21,
	}
	memorySizes := []int32{128, 256, 512, 1024, 256, 512, 128, 256, 512, 128, 256, 1024, 512, 128, 256, 512, 128, 256, 512}
	timeouts := []int32{10, 30, 60, 300, 15, 120, 10, 30, 60, 10, 30, 300, 60, 10, 15, 120, 30, 60, 10}
	codeSizes := []int64{524288, 1048576, 2097152, 5242880, 8388608, 1048576, 524288, 2097152, 15728640, 524288,
		1048576, 31457280, 5242880, 524288, 1048576, 2097152, 8388608, 1048576, 524288}

	for i := 0; i < 19; i++ {
		name := lambdaNamePool[i]
		runtime := lambdaRuntimePool[i]
		handler := lambdaHandlerPool[i]
		lastMod := fmt.Sprintf("2026-%02d-%02dT%02d:%02d:00+00:00", 1+(i%3), 1+i, 8+(i%14), i*3%60)

		fns = append(fns, resource.Resource{
			ID:     name,
			Name:   name,
			Status: runtime,
			Fields: map[string]string{
				"function_name": name,
				"runtime":       runtime,
				"memory":        fmt.Sprintf("%d", memorySizes[i]),
				"timeout":       fmt.Sprintf("%d", timeouts[i]),
				"handler":       handler,
				"last_modified": lastMod,
				"code_size":     fmt.Sprintf("%d", codeSizes[i]),
				"log_group":     "/aws/lambda/" + name,
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String(name),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
				Role:         aws.String(prodLambdaRoleARN),
				Runtime:      runtimeMap[runtime],
				MemorySize:   aws.Int32(memorySizes[i]),
				Timeout:      aws.Int32(timeouts[i]),
				Handler:      aws.String(handler),
				LastModified: aws.String(lastMod),
				CodeSize:     codeSizes[i],
				State:        lambdatypes.StateActive,
				PackageType:  lambdatypes.PackageTypeZip,
				Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
				EphemeralStorage: &lambdatypes.EphemeralStorage{
					Size: aws.Int32(512),
				},
				TracingConfig: &lambdatypes.TracingConfigResponse{
					Mode: lambdatypes.TracingModePassThrough,
				},
				DeadLetterConfig: &lambdatypes.DeadLetterConfig{
					TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
				},
				Environment: &lambdatypes.EnvironmentResponse{
					Variables: map[string]string{"ENV": "production", "LOG_LEVEL": "INFO"},
				},
				LastUpdateStatus:       lambdatypes.LastUpdateStatusSuccessful,
				LastUpdateStatusReason: aws.String("Function update succeeded"),
				Layers: []lambdatypes.Layer{
					{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:layer:acme-common:3"), CodeSize: 1048576},
				},
				VpcConfig: &lambdatypes.VpcConfigResponse{
					VpcId:            aws.String(prodVPCID),
					SubnetIds:        []string{prodPublicSubnetA},
					SecurityGroupIds: []string{prodWebALBSGID},
				},
			},
		})
	}

	for i := range fns {
		if _, ok := fns[i].Fields["event_source_arn"]; !ok {
			fns[i].Fields["event_source_arn"] = ""
		}
	}

	return fns
}

// ---------------------------------------------------------------------------
// EBS Volumes
// ---------------------------------------------------------------------------

// ebsVolumeFixtures returns demo EBS Volume fixtures with populated RawStruct.
// Includes a mix of in-use, available, and creating states.
func ebsVolumeFixtures() []resource.Resource {
	t1 := time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	t5 := time.Date(2026, 3, 28, 9, 30, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "vol-0a1b2c3d4e5f60001",
			Name:   "web-prod-01-root",
			Status: "in-use",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60001",
				"name":        "web-prod-01-root",
				"state":       "in-use",
				"size":        "50",
				"type":        "gp3",
				"iops":        "3000",
				"encrypted":   "true",
				"attached_to": "i-0a1b2c3d4e5f60001",
				"az":          "us-east-1a",
				"created":     t2.Format("2006-01-02 15:04"),
			},
			RawStruct: ec2types.Volume{
				VolumeId:           aws.String("vol-0a1b2c3d4e5f60001"),
				State:              ec2types.VolumeStateInUse,
				Size:               aws.Int32(50),
				VolumeType:         ec2types.VolumeTypeGp3,
				Iops:               aws.Int32(3000),
				Throughput:         aws.Int32(125),
				Encrypted:          aws.Bool(true),
				AvailabilityZone:   aws.String("us-east-1a"),
				CreateTime:         aws.Time(t2),
				KmsKeyId:           aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				MultiAttachEnabled: aws.Bool(false),
				Attachments: []ec2types.VolumeAttachment{
					{InstanceId: aws.String("i-0a1b2c3d4e5f60001")},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("web-prod-01-root")},
				},
			},
		},
		{
			ID:     "vol-0a1b2c3d4e5f60002",
			Name:   "api-staging-data",
			Status: "in-use",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60002",
				"name":        "api-staging-data",
				"state":       "in-use",
				"size":        "200",
				"type":        "io1",
				"iops":        "6000",
				"encrypted":   "true",
				"attached_to": "i-0a1b2c3d4e5f60003",
				"az":          "us-east-1b",
				"created":     t1.Format("2006-01-02 15:04"),
			},
			RawStruct: ec2types.Volume{
				VolumeId:         aws.String("vol-0a1b2c3d4e5f60002"),
				State:            ec2types.VolumeStateInUse,
				Size:             aws.Int32(200),
				VolumeType:       ec2types.VolumeTypeIo1,
				Iops:             aws.Int32(6000),
				Encrypted:        aws.Bool(true),
				AvailabilityZone: aws.String("us-east-1b"),
				CreateTime:       aws.Time(t1),
				Attachments: []ec2types.VolumeAttachment{
					{InstanceId: aws.String("i-0a1b2c3d4e5f60003")},
				},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("api-staging-data")},
				},
			},
		},
		{
			ID:     "vol-0a1b2c3d4e5f60003",
			Name:   "orphaned-backup-vol",
			Status: "available",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60003",
				"name":        "orphaned-backup-vol",
				"state":       "available",
				"size":        "100",
				"type":        "gp2",
				"iops":        "300",
				"encrypted":   "false",
				"attached_to": "",
				"az":          "us-east-1a",
				"created":     t3.Format("2006-01-02 15:04"),
			},
			RawStruct: ec2types.Volume{
				VolumeId:         aws.String("vol-0a1b2c3d4e5f60003"),
				State:            ec2types.VolumeStateAvailable,
				Size:             aws.Int32(100),
				VolumeType:       ec2types.VolumeTypeGp2,
				Iops:             aws.Int32(300),
				Encrypted:        aws.Bool(false),
				AvailabilityZone: aws.String("us-east-1a"),
				CreateTime:       aws.Time(t3),
				Attachments:      []ec2types.VolumeAttachment{},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("orphaned-backup-vol")},
				},
			},
		},
		{
			ID:     "vol-0a1b2c3d4e5f60004",
			Name:   "",
			Status: "available",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60004",
				"name":        "",
				"state":       "available",
				"size":        "500",
				"type":        "gp3",
				"iops":        "3000",
				"encrypted":   "true",
				"attached_to": "",
				"az":          "us-east-1c",
				"created":     t4.Format("2006-01-02 15:04"),
			},
			RawStruct: ec2types.Volume{
				VolumeId:         aws.String("vol-0a1b2c3d4e5f60004"),
				State:            ec2types.VolumeStateAvailable,
				Size:             aws.Int32(500),
				VolumeType:       ec2types.VolumeTypeGp3,
				Iops:             aws.Int32(3000),
				Encrypted:        aws.Bool(true),
				AvailabilityZone: aws.String("us-east-1c"),
				CreateTime:       aws.Time(t4),
				Attachments:      []ec2types.VolumeAttachment{},
				Tags:             []ec2types.Tag{},
			},
		},
		{
			ID:     "vol-0a1b2c3d4e5f60005",
			Name:   "new-db-volume",
			Status: "creating",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60005",
				"name":        "new-db-volume",
				"state":       "creating",
				"size":        "1000",
				"type":        "io2",
				"iops":        "16000",
				"encrypted":   "true",
				"attached_to": "",
				"az":          "us-east-1b",
				"created":     t5.Format("2006-01-02 15:04"),
			},
			RawStruct: ec2types.Volume{
				VolumeId:         aws.String("vol-0a1b2c3d4e5f60005"),
				State:            ec2types.VolumeStateCreating,
				Size:             aws.Int32(1000),
				VolumeType:       ec2types.VolumeTypeIo2,
				Iops:             aws.Int32(16000),
				Encrypted:        aws.Bool(true),
				AvailabilityZone: aws.String("us-east-1b"),
				CreateTime:       aws.Time(t5),
				Attachments:      []ec2types.VolumeAttachment{},
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("new-db-volume")},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// EBS Snapshots
// ---------------------------------------------------------------------------

// ebsSnapshotFixtures returns demo EBS Snapshot fixtures with populated RawStruct.
// Includes a mix of completed and pending states.
func ebsSnapshotFixtures() []resource.Resource {
	t1 := time.Date(2025, 9, 1, 2, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 21, 4, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 3, 28, 4, 0, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "snap-0a1b2c3d4e5f60001",
			Name:   "web-prod-snapshot-2025q3",
			Status: "completed",
			Fields: map[string]string{
				"snapshot_id": "snap-0a1b2c3d4e5f60001",
				"name":        "web-prod-snapshot-2025q3",
				"state":       "completed",
				"volume_id":   "vol-0a1b2c3d4e5f60001",
				"size":        "50",
				"encrypted":   "true",
				"description": "Quarterly backup of web-prod-01 root volume",
				"started":     t1.Format("2006-01-02 15:04"),
				"progress":    "100%",
			},
			RawStruct: ec2types.Snapshot{
				SnapshotId:  aws.String("snap-0a1b2c3d4e5f60001"),
				State:       ec2types.SnapshotStateCompleted,
				VolumeId:    aws.String("vol-0a1b2c3d4e5f60001"),
				VolumeSize:  aws.Int32(50),
				Encrypted:   aws.Bool(true),
				Description: aws.String("Quarterly backup of web-prod-01 root volume"),
				StartTime:   aws.Time(t1),
				Progress:    aws.String("100%"),
				OwnerId:     aws.String("123456789012"),
				KmsKeyId:    aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("web-prod-snapshot-2025q3")},
				},
			},
		},
		{
			ID:     "snap-0a1b2c3d4e5f60002",
			Name:   "api-data-snapshot-2026q1",
			Status: "completed",
			Fields: map[string]string{
				"snapshot_id": "snap-0a1b2c3d4e5f60002",
				"name":        "api-data-snapshot-2026q1",
				"state":       "completed",
				"volume_id":   "vol-0a1b2c3d4e5f60002",
				"size":        "200",
				"encrypted":   "true",
				"description": "Q1 2026 backup of api-staging-data volume",
				"started":     t2.Format("2006-01-02 15:04"),
				"progress":    "100%",
			},
			RawStruct: ec2types.Snapshot{
				SnapshotId:  aws.String("snap-0a1b2c3d4e5f60002"),
				State:       ec2types.SnapshotStateCompleted,
				VolumeId:    aws.String("vol-0a1b2c3d4e5f60002"),
				VolumeSize:  aws.Int32(200),
				Encrypted:   aws.Bool(true),
				Description: aws.String("Q1 2026 backup of api-staging-data volume"),
				StartTime:   aws.Time(t2),
				Progress:    aws.String("100%"),
				OwnerId:     aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("api-data-snapshot-2026q1")},
				},
			},
		},
		{
			ID:     "snap-0a1b2c3d4e5f60003",
			Name:   "orphaned-vol-snapshot",
			Status: "completed",
			Fields: map[string]string{
				"snapshot_id": "snap-0a1b2c3d4e5f60003",
				"name":        "orphaned-vol-snapshot",
				"state":       "completed",
				"volume_id":   "vol-0a1b2c3d4e5f60003",
				"size":        "100",
				"encrypted":   "false",
				"description": "Pre-deletion backup",
				"started":     t3.Format("2006-01-02 15:04"),
				"progress":    "100%",
			},
			RawStruct: ec2types.Snapshot{
				SnapshotId:  aws.String("snap-0a1b2c3d4e5f60003"),
				State:       ec2types.SnapshotStateCompleted,
				VolumeId:    aws.String("vol-0a1b2c3d4e5f60003"),
				VolumeSize:  aws.Int32(100),
				Encrypted:   aws.Bool(false),
				Description: aws.String("Pre-deletion backup"),
				StartTime:   aws.Time(t3),
				Progress:    aws.String("100%"),
				OwnerId:     aws.String("123456789012"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("orphaned-vol-snapshot")},
				},
			},
		},
		{
			ID:     "snap-0a1b2c3d4e5f60004",
			Name:   "",
			Status: "pending",
			Fields: map[string]string{
				"snapshot_id": "snap-0a1b2c3d4e5f60004",
				"name":        "",
				"state":       "pending",
				"volume_id":   "vol-0a1b2c3d4e5f60005",
				"size":        "1000",
				"encrypted":   "true",
				"description": "Initial snapshot of new-db-volume",
				"started":     t4.Format("2006-01-02 15:04"),
				"progress":    "23%",
			},
			RawStruct: ec2types.Snapshot{
				SnapshotId:  aws.String("snap-0a1b2c3d4e5f60004"),
				State:       ec2types.SnapshotStatePending,
				VolumeId:    aws.String("vol-0a1b2c3d4e5f60005"),
				VolumeSize:  aws.Int32(1000),
				Encrypted:   aws.Bool(true),
				Description: aws.String("Initial snapshot of new-db-volume"),
				StartTime:   aws.Time(t4),
				Progress:    aws.String("23%"),
				OwnerId:     aws.String("123456789012"),
				Tags:        []ec2types.Tag{},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// AMIs
// ---------------------------------------------------------------------------

// amiFixtures returns demo AMI fixtures with populated RawStruct.
// Includes a mix of x86_64 and arm64 architectures, varying ages.
func amiFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     prodAMIID1,
			Name:   "acme-app-server-x86-v2.3.1",
			Status: "available",
			Fields: map[string]string{
				"image_id":         prodAMIID1,
				"name":             "acme-app-server-x86-v2.3.1",
				"state":            "available",
				"architecture":     "x86_64",
				"platform":         "Linux/UNIX",
				"root_device_type": "ebs",
				"creation_date":    "2026-02-15T10:30:00.000Z",
				"public":           "false",
			},
			RawStruct: ec2types.Image{
				ImageId:            aws.String(prodAMIID1),
				Name:               aws.String("acme-app-server-x86-v2.3.1"),
				State:              ec2types.ImageStateAvailable,
				Architecture:       ec2types.ArchitectureValuesX8664,
				PlatformDetails:    aws.String("Linux/UNIX"),
				RootDeviceType:     ec2types.DeviceTypeEbs,
				RootDeviceName:     aws.String("/dev/xvda"),
				Hypervisor:         ec2types.HypervisorTypeXen,
				VirtualizationType: ec2types.VirtualizationTypeHvm,
				ImageType:          ec2types.ImageTypeValuesMachine,
				CreationDate:       aws.String("2026-02-15T10:30:00.000Z"),
				Public:             aws.Bool(false),
				OwnerId:            aws.String("123456789012"),
				Description:        aws.String("Production app server image x86_64 v2.3.1"),
				EnaSupport:         aws.Bool(true),
				BlockDeviceMappings: []ec2types.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &ec2types.EbsBlockDevice{
							VolumeSize:          aws.Int32(20),
							VolumeType:          ec2types.VolumeTypeGp3,
							DeleteOnTermination: aws.Bool(true),
						},
					},
				},
				BootMode:        ec2types.BootModeValuesUefi,
				DeprecationTime: aws.String("2028-01-01T00:00:00Z"),
				ImageLocation:   aws.String("123456789012/amazon-linux-2023-x86_64"),
				ImageOwnerAlias: aws.String("amazon"),
				SriovNetSupport: aws.String("simple"),
				UsageOperation:  aws.String("RunInstances"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-app-server-x86-v2.3.1")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     prodAMIID2,
			Name:   "acme-app-server-arm64-v2.3.1",
			Status: "available",
			Fields: map[string]string{
				"image_id":         prodAMIID2,
				"name":             "acme-app-server-arm64-v2.3.1",
				"state":            "available",
				"architecture":     "arm64",
				"platform":         "Linux/UNIX",
				"root_device_type": "ebs",
				"creation_date":    "2026-02-15T10:35:00.000Z",
				"public":           "false",
			},
			RawStruct: ec2types.Image{
				ImageId:            aws.String(prodAMIID2),
				Name:               aws.String("acme-app-server-arm64-v2.3.1"),
				State:              ec2types.ImageStateAvailable,
				Architecture:       ec2types.ArchitectureValuesArm64,
				PlatformDetails:    aws.String("Linux/UNIX"),
				RootDeviceType:     ec2types.DeviceTypeEbs,
				RootDeviceName:     aws.String("/dev/xvda"),
				Hypervisor:         ec2types.HypervisorTypeXen,
				VirtualizationType: ec2types.VirtualizationTypeHvm,
				ImageType:          ec2types.ImageTypeValuesMachine,
				CreationDate:       aws.String("2026-02-15T10:35:00.000Z"),
				Public:             aws.Bool(false),
				OwnerId:            aws.String("123456789012"),
				Description:        aws.String("Production app server image arm64 v2.3.1"),
				EnaSupport:         aws.Bool(true),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-app-server-arm64-v2.3.1")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     prodAMIID3,
			Name:   "acme-worker-x86-v1.8.0",
			Status: "available",
			Fields: map[string]string{
				"image_id":         prodAMIID3,
				"name":             "acme-worker-x86-v1.8.0",
				"state":            "available",
				"architecture":     "x86_64",
				"platform":         "Linux/UNIX",
				"root_device_type": "ebs",
				"creation_date":    "2025-09-10T08:00:00.000Z",
				"public":           "false",
			},
			RawStruct: ec2types.Image{
				ImageId:            aws.String(prodAMIID3),
				Name:               aws.String("acme-worker-x86-v1.8.0"),
				State:              ec2types.ImageStateAvailable,
				Architecture:       ec2types.ArchitectureValuesX8664,
				PlatformDetails:    aws.String("Linux/UNIX"),
				RootDeviceType:     ec2types.DeviceTypeEbs,
				RootDeviceName:     aws.String("/dev/xvda"),
				Hypervisor:         ec2types.HypervisorTypeXen,
				VirtualizationType: ec2types.VirtualizationTypeHvm,
				ImageType:          ec2types.ImageTypeValuesMachine,
				CreationDate:       aws.String("2025-09-10T08:00:00.000Z"),
				Public:             aws.Bool(false),
				OwnerId:            aws.String("123456789012"),
				Description:        aws.String("Batch worker image x86_64 v1.8.0"),
				EnaSupport:         aws.Bool(true),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-worker-x86-v1.8.0")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
		{
			ID:     "ami-0a1b2c3d4e5f60004",
			Name:   "acme-bastion-x86-v3.0.0-deprecated",
			Status: "deregistered",
			Fields: map[string]string{
				"image_id":         "ami-0a1b2c3d4e5f60004",
				"name":             "acme-bastion-x86-v3.0.0-deprecated",
				"state":            "deregistered",
				"architecture":     "x86_64",
				"platform":         "Linux/UNIX",
				"root_device_type": "ebs",
				"creation_date":    "2024-06-01T12:00:00.000Z",
				"public":           "false",
			},
			RawStruct: ec2types.Image{
				ImageId:            aws.String("ami-0a1b2c3d4e5f60004"),
				Name:               aws.String("acme-bastion-x86-v3.0.0-deprecated"),
				State:              ec2types.ImageStateDeregistered,
				Architecture:       ec2types.ArchitectureValuesX8664,
				PlatformDetails:    aws.String("Linux/UNIX"),
				RootDeviceType:     ec2types.DeviceTypeEbs,
				RootDeviceName:     aws.String("/dev/xvda"),
				Hypervisor:         ec2types.HypervisorTypeXen,
				VirtualizationType: ec2types.VirtualizationTypeHvm,
				ImageType:          ec2types.ImageTypeValuesMachine,
				CreationDate:       aws.String("2024-06-01T12:00:00.000Z"),
				Public:             aws.Bool(false),
				OwnerId:            aws.String("123456789012"),
				Description:        aws.String("Deprecated bastion host image"),
				EnaSupport:         aws.Bool(true),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-bastion-x86-v3.0.0-deprecated")},
				},
			},
		},
	}
}
