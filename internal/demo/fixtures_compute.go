package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["ec2"] = ec2Instances
	demoData["lambda"] = lambdaFunctions
}

// ---------------------------------------------------------------------------
// EC2 Instances
// ---------------------------------------------------------------------------

// ec2Instances returns demo EC2 instance fixtures with populated RawStruct.
// Includes a mix of running/stopped/pending states and realistic naming for
// the demo scenario (filter /web must show results).
func ec2Instances() []resource.Resource {
	return []resource.Resource{
		makeEC2Instance(
			"i-0a1b2c3d4e5f60001", "web-prod-01", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.10", "54.210.33.112",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 30, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60002", "web-prod-02", "running",
			ec2types.InstanceTypeT3Large, "10.0.1.11", "54.210.33.113",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 11, 15, 8, 32, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60003", "api-staging-01", "running",
			ec2types.InstanceTypeM5Xlarge, "10.0.2.50", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 1, 20, 14, 15, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60004", "worker-batch-03", "stopped",
			ec2types.InstanceTypeC5Xlarge, "10.0.3.100", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2025, 9, 5, 11, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60005", "bastion-prod", "running",
			ec2types.InstanceTypeT3Micro, "10.0.0.5", "52.87.221.44",
			"vpc-0abc123def456789a", "subnet-0aaa111111111111a",
			time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60006", "db-proxy-01", "running",
			ec2types.InstanceTypeR5Large, "10.0.4.200", "",
			"vpc-0abc123def456789a", "subnet-0ddd444444444444d",
			time.Date(2025, 12, 10, 18, 45, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60007", "web-staging-01", "pending",
			ec2types.InstanceTypeT3Medium, "10.0.2.70", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60008", "ml-trainer-gpu", "stopping",
			ec2types.InstanceTypeG4dnXlarge, "10.0.5.30", "",
			"vpc-0abc123def456789a", "subnet-0eee555555555555e",
			time.Date(2026, 2, 14, 22, 0, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60009", "temp-load-test", "shutting-down",
			ec2types.InstanceTypeC5Large, "10.0.3.55", "",
			"vpc-0abc123def456789a", "subnet-0ccc333333333333c",
			time.Date(2026, 3, 20, 16, 30, 0, 0, time.UTC),
		),
		makeEC2Instance(
			"i-0a1b2c3d4e5f60010", "old-migration-worker", "terminated",
			ec2types.InstanceTypeT3Small, "", "",
			"vpc-0abc123def456789a", "subnet-0bbb222222222222b",
			time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC),
		),
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
) resource.Resource {
	stateName := ec2types.InstanceStateName(state)
	stateCode := stateNameToCode(stateName)

	inst := ec2types.Instance{
		InstanceId:       aws.String(instanceID),
		InstanceType:     instanceType,
		PrivateIpAddress: aws.String(privateIP),
		State: &ec2types.InstanceState{
			Name: stateName,
			Code: aws.Int32(stateCode),
		},
		VpcId:    aws.String(vpcID),
		SubnetId: aws.String(subnetID),
		Tags: []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String(name)},
			{Key: aws.String("Environment"), Value: aws.String(envFromName(name))},
		},
		LaunchTime: aws.Time(launchTime),
	}

	if publicIP != "" {
		inst.PublicIpAddress = aws.String(publicIP)
	}

	launchTimeStr := launchTime.Format("2006-01-02T15:04:05Z07:00")

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


// lambdaFunctions returns demo Lambda function fixtures.
func lambdaFunctions() []resource.Resource {
	return []resource.Resource{
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
				Runtime:      lambdatypes.RuntimeNodejs20x,
				MemorySize:   aws.Int32(256),
				Timeout:      aws.Int32(10),
				Handler:      aws.String("index.handler"),
				LastModified: aws.String("2026-03-15T08:22:14+00:00"),
				CodeSize:     1048576,
				State:        lambdatypes.StateActive,
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
				Runtime:      lambdatypes.RuntimePython312,
				MemorySize:   aws.Int32(512),
				Timeout:      aws.Int32(300),
				Handler:      aws.String("transform.lambda_handler"),
				LastModified: aws.String("2026-03-10T16:45:33+00:00"),
				CodeSize:     5242880,
				State:        lambdatypes.StateActive,
			},
		},
		{
			ID:     "order-processor",
			Name:   "order-processor",
			Status: "go1.x",
			Fields: map[string]string{
				"function_name": "order-processor",
				"runtime":       "go1.x",
				"memory":        "128",
				"timeout":       "30",
				"handler":       "main",
				"last_modified": "2026-02-28T11:03:47+00:00",
				"code_size":     "8388608",
				"log_group":     "/aws/lambda/order-processor",
				"package_type":  "Zip",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("order-processor"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:order-processor"),
				Runtime:      lambdatypes.RuntimeGo1x,
				MemorySize:   aws.Int32(128),
				Timeout:      aws.Int32(30),
				Handler:      aws.String("main"),
				LastModified: aws.String("2026-02-28T11:03:47+00:00"),
				CodeSize:     8388608,
				State:        lambdatypes.StateActive,
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
				Runtime:      lambdatypes.RuntimePython312,
				MemorySize:   aws.Int32(1024),
				Timeout:      aws.Int32(60),
				Handler:      aws.String("thumbnail.handler"),
				LastModified: aws.String("2026-03-01T09:18:55+00:00"),
				CodeSize:     15728640,
				State:        lambdatypes.StateActive,
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
				Runtime:      lambdatypes.RuntimeJava21,
				MemorySize:   aws.Int32(512),
				Timeout:      aws.Int32(15),
				Handler:      aws.String("com.example.PaymentHandler::handleRequest"),
				LastModified: aws.String("2026-03-12T20:11:09+00:00"),
				CodeSize:     31457280,
				State:        lambdatypes.StateActive,
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
				Runtime:      lambdatypes.RuntimeNodejs20x,
				MemorySize:   aws.Int32(128),
				Timeout:      aws.Int32(10),
				Handler:      aws.String("notify.handler"),
				LastModified: aws.String("2026-01-20T13:42:00+00:00"),
				CodeSize:     524288,
				State:        lambdatypes.StateActive,
			},
		},
	}
}

