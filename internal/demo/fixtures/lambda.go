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
	lambdaProdRoleARN   = "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"
	lambdaProdVPCID     = "vpc-0abc123def456789a"
	lambdaProdSubnetA   = "subnet-0aaa111111111111a"
	lambdaProdALBSGID   = "sg-0aaa111111111111a"
	lambdaProcessOrders = "process-orders"
	lambdaRotateDocDB   = "rotate-docdb-credentials"
	lambdaRotateRDS     = "rotate-rds-credentials"
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
			FunctionName:     aws.String("api-gateway-authorizer"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimeNodejs20x,
			MemorySize:       aws.Int32(256),
			Timeout:          aws.Int32(10),
			Handler:          aws.String("index.handler"),
			Description:      aws.String("API Gateway custom authorizer"),
			LastModified:     aws.String("2026-03-15T08:22:14+00:00"),
			CodeSize:         1048576,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
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
			FunctionName:     aws.String("data-pipeline-transform"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-transform"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimePython312,
			MemorySize:       aws.Int32(512),
			Timeout:          aws.Int32(300),
			Handler:          aws.String("transform.lambda_handler"),
			Description:      aws.String("ETL data pipeline transformation function"),
			LastModified:     aws.String("2026-03-10T16:45:33+00:00"),
			CodeSize:         5242880,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/data-pipeline-transform"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String(lambdaProcessOrders),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaProcessOrders),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimeGo1x,
			MemorySize:       aws.Int32(128),
			Timeout:          aws.Int32(30),
			Handler:          aws.String("main"),
			Description:      aws.String("Order processing Lambda triggered by SQS"),
			LastModified:     aws.String("2026-02-28T11:03:47+00:00"),
			CodeSize:         8388608,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/" + lambdaProcessOrders),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String("image-thumbnail-gen"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimePython312,
			MemorySize:       aws.Int32(1024),
			Timeout:          aws.Int32(60),
			Handler:          aws.String("thumbnail.handler"),
			Description:      aws.String("S3-triggered image thumbnail generator"),
			LastModified:     aws.String("2026-03-01T09:18:55+00:00"),
			CodeSize:         15728640,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/image-thumbnail-gen"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String("payment-webhook"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:payment-webhook"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimeJava21,
			MemorySize:       aws.Int32(512),
			Timeout:          aws.Int32(15),
			Handler:          aws.String("com.example.PaymentHandler::handleRequest"),
			Description:      aws.String("Payment provider webhook handler"),
			LastModified:     aws.String("2026-03-12T20:11:09+00:00"),
			CodeSize:         31457280,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/payment-webhook"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String("cloudwatch-slack-notifier"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimeNodejs20x,
			MemorySize:       aws.Int32(128),
			Timeout:          aws.Int32(10),
			Handler:          aws.String("notify.handler"),
			Description:      aws.String("CloudWatch alarm to Slack notification relay"),
			LastModified:     aws.String("2026-01-20T13:42:00+00:00"),
			CodeSize:         524288,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/cloudwatch-slack-notifier"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String(lambdaRotateDocDB),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaRotateDocDB),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimePython312,
			MemorySize:       aws.Int32(128),
			Timeout:          aws.Int32(30),
			Handler:          aws.String("rotate.handler"),
			Description:      aws.String("Rotates DocDB credentials in Secrets Manager"),
			LastModified:     aws.String("2026-02-14T10:00:00+00:00"),
			CodeSize:         1048576,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/" + lambdaRotateDocDB),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String(lambdaRotateRDS),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + lambdaRotateRDS),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimePython312,
			MemorySize:       aws.Int32(128),
			Timeout:          aws.Int32(30),
			Handler:          aws.String("rotation.handler"),
			Description:      aws.String("Rotates Aurora PostgreSQL credentials for prod/database/primary"),
			LastModified:     aws.String("2026-03-10T08:00:00+00:00"),
			CodeSize:         524288,
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/" + lambdaRotateRDS),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		},
		{
			FunctionName:     aws.String("legacy-data-sync"),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:legacy-data-sync"),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          lambdatypes.RuntimePython312,
			MemorySize:       aws.Int32(256),
			Timeout:          aws.Int32(300),
			Handler:          aws.String("sync.lambda_handler"),
			Description:      aws.String("Legacy data sync function — failed during layer attachment update"),
			LastModified:     aws.String("2026-03-18T04:12:00+00:00"),
			CodeSize:         2097152,
			State:            lambdatypes.StateFailed,
			StateReason:      aws.String("Layer arn:aws:lambda:us-east-1:123456789012:layer:legacy-utils:3 could not be attached"),
			StateReasonCode:  lambdatypes.StateReasonCodeInvalidConfiguration,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
			LoggingConfig: &lambdatypes.LoggingConfig{
				LogGroup:  aws.String("/aws/lambda/legacy-data-sync"),
				LogFormat: lambdatypes.LogFormatText,
			},
			LastUpdateStatus:       lambdatypes.LastUpdateStatusFailed,
			LastUpdateStatusReason: aws.String("Layer attachment limit exceeded"),
		},
	}

	// Issue: State=Pending → Warning (deployment in progress)
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String("lambda-pending-deploy"),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:lambda-pending-deploy"),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimeNodejs20x,
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(30),
		Handler:          aws.String("index.handler"),
		Description:      aws.String("New function deployment in progress — not yet active"),
		LastModified:     aws.String("2026-04-18T08:00:00+00:00"),
		CodeSize:         1048576,
		State:            lambdatypes.StatePending,
		StateReason:      aws.String("The function is being created"),
		StateReasonCode:  lambdatypes.StateReasonCodeCreating,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/lambda-pending-deploy"),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusInProgress,
	})

	// Issue: State=Inactive → Dim (function has not been invoked in a long time)
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String("lambda-inactive-runtime"),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:lambda-inactive-runtime"),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(128),
		Timeout:          aws.Int32(15),
		Handler:          aws.String("handler.lambda_handler"),
		Description:      aws.String("Idle function — placed in Inactive state by Lambda after extended non-use"),
		LastModified:     aws.String("2025-10-01T10:00:00+00:00"),
		CodeSize:         524288,
		State:            lambdatypes.StateInactive,
		StateReason:      aws.String("The function has not been used for an extended period"),
		StateReasonCode:  lambdatypes.StateReasonCodeIdle,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/lambda-inactive-runtime"),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})

	// Add one container-image function to demonstrate ECR→Lambda relationship.
	// checkECRLambda matches any lambda with PackageType=Image as potentially using an ECR repo.
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:  aws.String("api-service-runner"),
		FunctionArn:   aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-service-runner"),
		Role:          aws.String(lambdaProdRoleARN),
		MemorySize:    aws.Int32(512),
		Timeout:       aws.Int32(30),
		Description:   aws.String("Container-image Lambda running the API service from ECR"),
		LastModified:  aws.String("2026-03-20T10:00:00+00:00"),
		CodeSize:      0,
		State:         lambdatypes.StateActive,
		PackageType:   lambdatypes.PackageTypeImage,
		Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/api-service-runner"),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})

	// orders-projector: triggered by orders-prod DynamoDB stream (DDB→lambda pivot).
	// checkDdbLambda calls ListEventSourceMappings(EventSourceArn=<LatestStreamArn>);
	// the fake filters ESMs by EventSourceArn so this ESM is returned only for that query.
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(OrdersProdLambdaName),
		FunctionArn:      aws.String(OrdersProdLambdaARN),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimeGo1x,
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(60),
		Handler:          aws.String("main"),
		Description:      aws.String("Projects orders-prod DynamoDB stream events to the read model"),
		LastModified:     aws.String("2026-01-15T08:00:00+00:00"),
		CodeSize:         4194304,
		State:            lambdatypes.StateActive,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModeActive},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/" + OrdersProdLambdaName),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})

	// S3 notifier: invoked by healthy-bucket S3 event notification (checkS3Lambda pivot).
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(S3NotifierLambdaName),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + S3NotifierLambdaName),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(128),
		Timeout:          aws.Int32(30),
		Handler:          aws.String("notifier.handler"),
		Description:      aws.String("Handles S3 event notifications from a9s-demo-healthy bucket"),
		LastModified:     aws.String("2026-01-20T10:00:00+00:00"),
		CodeSize:         524288,
		State:            lambdatypes.StateActive,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/" + S3NotifierLambdaName),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})

	// Generate 18 more functions to reach 26 total (including the image function above).
	for i := range 18 {
		name := lambdaNamePool[i]
		rt := lambdaRuntimePool[i]
		handler := lambdaHandlerPool[i]
		lastMod := fmt.Sprintf("2026-%02d-%02dT%02d:%02d:00+00:00", 1+(i%3), 1+i, 8+(i%14), (i*3)%60)
		fns = append(fns, lambdatypes.FunctionConfiguration{
			FunctionName:     aws.String(name),
			FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
			Role:             aws.String(lambdaProdRoleARN),
			Runtime:          rt,
			MemorySize:       aws.Int32(lambdaMemorySizes[i]),
			Timeout:          aws.Int32(lambdaTimeouts[i]),
			Handler:          aws.String(handler),
			LastModified:     aws.String(lastMod),
			CodeSize:         lambdaCodeSizes[i],
			State:            lambdatypes.StateActive,
			PackageType:      lambdatypes.PackageTypeZip,
			Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
			EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
			TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
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
	var mappings []lambdatypes.EventSourceMappingConfiguration
	for _, fn := range fns {
		switch aws.ToString(fn.FunctionName) {
		case lambdaProcessOrders:
			// process-orders is triggered by SQS.
			mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{
				UUID:                 aws.String("esm-process-orders-01"),
				FunctionArn:          fn.FunctionArn,
				EventSourceArn:       aws.String("arn:aws:sqs:us-east-1:123456789012:order-processing-queue"),
				State:                aws.String("Enabled"),
				BatchSize:            aws.Int32(10),
				LastProcessingResult: aws.String("OK"),
			})
		case OrdersProdLambdaName:
			// orders-projector is triggered by the orders-prod DynamoDB stream
			// (DDB→lambda pivot). checkDdbLambda filters by EventSourceArn =
			// table.LatestStreamArn, so this ESM must use OrdersProdStreamARN.
			mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{
				UUID:                 aws.String("esm-orders-projector-ddb-01"),
				FunctionArn:          fn.FunctionArn,
				EventSourceArn:       aws.String(OrdersProdStreamARN),
				State:                aws.String("Enabled"),
				BatchSize:            aws.Int32(100),
				StartingPosition:     lambdatypes.EventSourcePositionTrimHorizon,
				LastProcessingResult: aws.String("OK"),
			})
		}
	}
	return mappings
}
