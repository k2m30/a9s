// Package fixtures provides Lambda fixture data for the Lambda fake.
package fixtures

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// LambdaFixtures holds all Lambda domain objects served by the fake.
type LambdaFixtures struct {
	// Functions is the full list returned by ListFunctions.
	Functions []lambdatypes.FunctionConfiguration
	// EventSourceMappings is the full list returned by ListEventSourceMappings.
	EventSourceMappings []lambdatypes.EventSourceMappingConfiguration
}

// NewLambdaFixtures builds and returns a fully-populated LambdaFixtures struct.
func NewLambdaFixtures() *LambdaFixtures {
	fns := buildLambdaFunctions()
	return &LambdaFixtures{
		Functions:           fns,
		EventSourceMappings: buildLambdaEventSourceMappings(fns),
	}
}

const (
	lambdaProdRoleARN  = "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"
	lambdaProdVPCID    = "vpc-0abc123def456789a"
	lambdaProdSubnetA  = "subnet-0aaa111111111111a"
	lambdaProdALBSGID  = "sg-0aaa111111111111a"
	lambdaProcessOrders = "process-orders"
	lambdaRotateDocDB   = "rotate-docdb-credentials"
)

var lambdaNamePool = []string{
	"user-signup-handler", "inventory-sync", "pdf-generator",
	"email-sender", "cache-warmer", "db-migrator",
	"report-scheduler", "webhook-processor", "file-cleanup",
	"audit-logger", "config-validator", "health-monitor",
	"rate-limiter", "token-refresher", "data-exporter",
	"schema-validator", "event-router", "log-archiver",
}

var lambdaRuntimePool = []lambdatypes.Runtime{
	lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimePython312, lambdatypes.RuntimeGo1x,
	lambdatypes.RuntimeJava21, lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimePython312,
	lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimeGo1x, lambdatypes.RuntimePython312,
	lambdatypes.RuntimeJava21, lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimePython312,
	lambdatypes.RuntimeGo1x, lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimePython312,
	lambdatypes.RuntimeJava21, lambdatypes.RuntimeNodejs20x, lambdatypes.RuntimePython312,
}

var lambdaHandlerPool = []string{
	"index.handler", "handler.lambda_handler", "main",
	"com.example.Handler::handleRequest", "index.handler",
	"app.lambda_handler", "handler.handler", "main",
	"process.lambda_handler", "com.example.Processor::handle",
	"index.handler", "checker.lambda_handler", "main",
	"index.handler", "export.lambda_handler",
	"com.example.Validator::handle", "index.handler", "archive.lambda_handler",
}

var lambdaMemorySizes = []int32{128, 256, 512, 1024, 256, 512, 128, 256, 512, 128, 256, 1024, 512, 128, 256, 512, 128, 256}
var lambdaTimeouts = []int32{10, 30, 60, 300, 15, 120, 10, 30, 60, 10, 30, 300, 60, 10, 15, 120, 30, 60}
var lambdaCodeSizes = []int64{524288, 1048576, 2097152, 5242880, 8388608, 1048576, 524288, 2097152, 15728640, 524288,
	1048576, 31457280, 5242880, 524288, 1048576, 2097152, 8388608, 1048576}

func buildLambdaFunctions() []lambdatypes.FunctionConfiguration {
	fns := []lambdatypes.FunctionConfiguration{
		{
			FunctionName: aws.String("api-gateway-authorizer"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
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
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId:            aws.String(lambdaProdVPCID),
				SubnetIds:        []string{lambdaProdSubnetA},
				SecurityGroupIds: []string{lambdaProdALBSGID},
			},
		},
		{
			FunctionName: aws.String("data-pipeline-transform"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/data-pipeline-transform"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName: aws.String(lambdaProcessOrders),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaProcessOrders),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/" + lambdaProcessOrders),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName: aws.String("image-thumbnail-gen"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen"),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/image-thumbnail-gen"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName: aws.String("payment-webhook"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:payment-webhook"),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/payment-webhook"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName: aws.String("cloudwatch-slack-notifier"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
			Role:         aws.String(lambdaProdRoleARN),
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
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/cloudwatch-slack-notifier"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName: aws.String(lambdaRotateDocDB),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaRotateDocDB),
			Role:         aws.String(lambdaProdRoleARN),
			Runtime:      lambdatypes.RuntimePython312,
			MemorySize:   aws.Int32(128),
			Timeout:      aws.Int32(30),
			Handler:      aws.String("rotate.handler"),
			Description:  aws.String("Rotates DocDB credentials in Secrets Manager"),
			LastModified: aws.String("2026-02-14T10:00:00+00:00"),
			CodeSize:     1048576,
			State:        lambdatypes.StateActive,
			PackageType:  lambdatypes.PackageTypeZip,
			Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/" + lambdaRotateDocDB),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
	}

	// Generate 18 more functions to reach 25 total.
	for i := 0; i < 18; i++ {
		name := lambdaNamePool[i]
		rt := lambdaRuntimePool[i]
		handler := lambdaHandlerPool[i]
		lastMod := fmt.Sprintf("2026-%02d-%02dT%02d:%02d:00+00:00", 1+(i%3), 1+i, 8+(i%14), (i*3)%60)
		fns = append(fns, lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(name),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
			Role:         aws.String(lambdaProdRoleARN),
			Runtime:      rt,
			MemorySize:   aws.Int32(lambdaMemorySizes[i]),
			Timeout:      aws.Int32(lambdaTimeouts[i]),
			Handler:      aws.String(handler),
			LastModified: aws.String(lastMod),
			CodeSize:     lambdaCodeSizes[i],
			State:        lambdatypes.StateActive,
			PackageType:  lambdatypes.PackageTypeZip,
			Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig: &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
			},
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{"ENV": "production", "LOG_LEVEL": "INFO"},
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId:            aws.String(lambdaProdVPCID),
				SubnetIds:        []string{lambdaProdSubnetA},
				SecurityGroupIds: []string{lambdaProdALBSGID},
			},
		})
	}

	return fns
}

func buildLambdaEventSourceMappings(fns []lambdatypes.FunctionConfiguration) []lambdatypes.EventSourceMappingConfiguration {
	// Only process-orders has an SQS trigger in the demo scenario.
	var mappings []lambdatypes.EventSourceMappingConfiguration
	for _, fn := range fns {
		if aws.ToString(fn.FunctionName) == lambdaProcessOrders {
			mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{
				UUID:            aws.String("esm-process-orders-01"),
				FunctionArn:     fn.FunctionArn,
				EventSourceArn:  aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
				State:           aws.String("Enabled"),
				BatchSize:       aws.Int32(10),
				LastProcessingResult: aws.String("OK"),
			})
			break
		}
	}
	return mappings
}
