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
		LaunchTime:        aws.Time(launchTime),
		InstanceLifecycle: lifecycle,
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
				Runtime:      runtimeMap[runtime],
				MemorySize:   aws.Int32(memorySizes[i]),
				Timeout:      aws.Int32(timeouts[i]),
				Handler:      aws.String(handler),
				LastModified: aws.String(lastMod),
				CodeSize:     codeSizes[i],
				State:        lambdatypes.StateActive,
			},
		})
	}

	return fns
}

