// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides Lambda fixture data for the Lambda fake.
package fixtures

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// LambdaFixtures holds all Lambda domain objects served by the fake.
type LambdaFixtures struct {
	// Functions is the full list returned by ListFunctions.
	Functions []lambdatypes.FunctionConfiguration
	// EventSourceMappings is the full list returned by ListEventSourceMappings.
	EventSourceMappings []lambdatypes.EventSourceMappingConfiguration
	// ImageURIs maps function name -> Code.ImageUri, served by GetFunction.
	// Real AWS only returns ImageUri via GetFunction (never ListFunctions),
	// so this is a GetFunction-only fixture — required for the lambda:ecr
	// related-panel pivot (checkLambdaECR).
	ImageURIs map[string]string
	// Tags maps function name -> tag map, served by ListTags. Required for
	// the lambda:cfn related-panel pivot (checkLambdaCFN reads
	// "aws:cloudformation:stack-name").
	Tags map[string]map[string]string
	// ReservedConcurrency maps function name -> GetFunction's
	// Concurrency.ReservedConcurrentExecutions. Real AWS omits this field
	// entirely unless PutFunctionConcurrency was called — most functions
	// have none, so only one fixture function carries it.
	ReservedConcurrency map[string]int32
	// Policies maps function name -> the resource policy GetPolicy returns.
	// A function absent from this map has no policy at all, which real
	// Lambda reports as ResourceNotFoundException.
	Policies map[string]string
	// FunctionURLConfigs maps function name -> the configs
	// ListFunctionUrlConfigs returns. Absent means the function has no URL.
	FunctionURLConfigs map[string][]lambdatypes.FunctionUrlConfig
}

// Lambda posture witnesses — one function per Prowler-derived finding.
const (
	// LambdaEnvSecret is the only function with a plaintext credential in
	// its environment; every other function stores references, not values.
	LambdaEnvSecret = "payment-webhook"
	// LambdaPublicPolicy is the only function whose resource policy allows a
	// wildcard principal.
	LambdaPublicPolicy = "image-thumbnail-gen"
	// LambdaFunctionURLPublic is the only function with a function URL whose
	// AuthType is NONE.
	LambdaFunctionURLPublic = "cloudwatch-slack-notifier"
)

// NewLambdaFixtures builds and returns a fully-populated LambdaFixtures struct.
var sharedLambdaFixtures = sync.OnceValue(func() *LambdaFixtures {
	fns := buildLambdaFunctions()
	return &LambdaFixtures{
		Functions:           fns,
		EventSourceMappings: buildLambdaEventSourceMappings(fns),
		ImageURIs: map[string]string{
			// api-service-runner is the container-image function (PackageType=Image)
			// declared below; acme/api-service is a real ecr.go repository fixture.
			"api-service-runner": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest",
		},
		Tags: map[string]map[string]string{
			// api-gateway-authorizer carries the CFN stack tag — required for
			// the lambda:cfn related-panel pivot witness. acme-eks-cluster is
			// a real stack fixture (cfn.go).
			"api-gateway-authorizer": {"aws:cloudformation:stack-name": "acme-eks-cluster"},
		},
		ReservedConcurrency: map[string]int32{
			// process-orders is throttled to protect the downstream SQS
			// consumer from over-scaling.
			lambdaProcessOrders: 10,
		},
		Policies: map[string]string{
			// The lambda.public-policy witness: lambda:InvokeFunction granted
			// to every principal with no condition narrowing it.
			LambdaPublicPolicy: `{"Version":"2012-10-17","Statement":[{"Sid":"AllowPublicInvoke","Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunction","Resource":"arn:aws:lambda:us-east-1:123456789012:function:image-thumbnail-gen"}]}`,
			// A healthy counterpart: the same grant scoped to one service
			// principal, so the enricher's Public verdict is exercised both ways.
			lambdaProcessOrders: `{"Version":"2012-10-17","Statement":[{"Sid":"AllowSQS","Effect":"Allow","Principal":{"Service":"sqs.amazonaws.com"},"Action":"lambda:InvokeFunction","Resource":"arn:aws:lambda:us-east-1:123456789012:function:process-orders"}]}`,
		},
		FunctionURLConfigs: map[string][]lambdatypes.FunctionUrlConfig{
			// The lambda.function-url-public witness: AuthType NONE with a
			// wildcard CORS origin.
			LambdaFunctionURLPublic: {{
				FunctionUrl:  aws.String("https://abcd1234efgh5678.lambda-url.us-east-1.on.aws/"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:cloudwatch-slack-notifier"),
				AuthType:     lambdatypes.FunctionUrlAuthTypeNone,
				Cors:         &lambdatypes.Cors{AllowOrigins: []string{"*"}},
				CreationTime: aws.String("2026-02-01T09:00:00.000000Z"),
			}},
			// A healthy counterpart: same feature, IAM-authorized.
			"api-gateway-authorizer": {{
				FunctionUrl:  aws.String("https://ijkl9012mnop3456.lambda-url.us-east-1.on.aws/"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"),
				AuthType:     lambdatypes.FunctionUrlAuthTypeAwsIam,
				CreationTime: aws.String("2026-02-01T09:05:00.000000Z"),
			}},
		},
	}
})

func NewLambdaFixtures() *LambdaFixtures {
	return sharedLambdaFixtures()
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
			// Environment variables carry a Secrets Manager ARN and an SSM
			// parameter-style path — required for lambda→secrets and
			// lambda→ssm related-panel pivots.
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"ENV":           "production",
					"LOG_LEVEL":     "INFO",
					"DB_SECRET_ARN": "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf",
					"CONFIG_PARAM":  "/acme/prod/app/config",
				},
			},
			LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
			VpcConfig: &lambdatypes.VpcConfigResponse{
				VpcId:            aws.String(lambdaProdVPCID),
				SubnetIds:        []string{lambdaProdSubnetA},
				SecurityGroupIds: []string{lambdaProdALBSGID},
			},
			// KMSKeyArn — required for lambda→kms related-panel pivot.
			KMSKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
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
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
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
			// The lambda.env-secret witness: the provider token pasted into
			// the environment instead of resolved from Secrets Manager.
			Environment: &lambdatypes.EnvironmentResponse{
				Variables: map[string]string{
					"ENV":               "production",
					"PROVIDER_ENDPOINT": "https://payments.example.com/hooks",
					"PROVIDER_API_KEY":  "pk-live-4c81b7e2af9d6035",
				},
			},
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
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
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:dead-letter-queue"),
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
			// DeadLetterConfig — required for the secrets:sns related-panel
			// pivot witness (checkSecretsSNS reads the rotation Lambda's
			// DLQ TargetArn when it points at an SNS topic). ops-alerts is
			// the shared prod SNS topic (relatedAlarmSNSARN in cloudwatch.go).
			DeadLetterConfig: &lambdatypes.DeadLetterConfig{
				TargetArn: aws.String("arn:aws:sns:us-east-1:123456789012:ops-alerts"),
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

	// Issue: State=Failed with LastUpdateStatus=Successful → Broken via the
	// lifecycle-state branch alone (lambda.state.failed). legacy-data-sync
	// above hits the higher-precedence last-update-failed branch instead
	// (LastUpdateStatus=Failed on that fixture), so this fixture is needed to
	// demonstrate the State=Failed branch in isolation.
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String("lambda-runtime-crash"),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:lambda-runtime-crash"),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(30),
		Handler:          aws.String("handler.lambda_handler"),
		Description:      aws.String("Function entered Failed state after an unrecoverable internal error"),
		LastModified:     aws.String("2026-04-02T11:20:00+00:00"),
		CodeSize:         1048576,
		State:            lambdatypes.StateFailed,
		StateReason:      aws.String("Function is unable to service requests due to an internal error"),
		StateReasonCode:  lambdatypes.StateReasonCodeInternalError,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/lambda-runtime-crash"),
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

	// SES inbound parser: invoked by SES v1 receipt rule (checkSESLambda pivot).
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(SESInboundLambdaName),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + SESInboundLambdaName),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(30),
		Handler:          aws.String("parser.lambda_handler"),
		Description:      aws.String("Parses inbound mail delivered via SES v1 receipt rule to support@acme-corp.com"),
		LastModified:     aws.String("2025-11-15T09:00:00+00:00"),
		CodeSize:         1048576,
		State:            lambdatypes.StateActive,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/" + SESInboundLambdaName),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})

	// EFS-mounted Lambda functions — required for efs→lambda related-panel pivot (Count = 2).
	// checkEFSLambda calls DescribeAccessPoints(FileSystemId=ProdEFSID), then scans lambda
	// cache matching FunctionConfiguration.FileSystemConfigs[].Arn against AP ARNs.
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(ProdEFSLambdaAName),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + ProdEFSLambdaAName),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(512),
		Timeout:          aws.Int32(300),
		Handler:          aws.String("processor.handler"),
		Description:      aws.String("Processes files written to EFS app-data filesystem"),
		LastModified:     aws.String("2026-02-10T09:00:00+00:00"),
		CodeSize:         2097152,
		State:            lambdatypes.StateActive,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
		FileSystemConfigs: []lambdatypes.FileSystemConfig{
			{Arn: aws.String(ProdEFSAccessPointAARN), LocalMountPath: aws.String("/mnt/app-data")},
		},
		VpcConfig: &lambdatypes.VpcConfigResponse{
			VpcId:            aws.String(ProdEFSVpcID),
			SubnetIds:        []string{ProdEFSSubnetAID},
			SecurityGroupIds: []string{ProdEFSSecurityGroupAID},
		},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/" + ProdEFSLambdaAName),
			LogFormat: lambdatypes.LogFormatText,
		},
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
	})
	fns = append(fns, lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(ProdEFSLambdaBName),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + ProdEFSLambdaBName),
		Role:             aws.String(lambdaProdRoleARN),
		Runtime:          lambdatypes.RuntimePython312,
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(120),
		Handler:          aws.String("report.handler"),
		Description:      aws.String("Generates reports from EFS app-data shared filesystem"),
		LastModified:     aws.String("2026-02-15T11:00:00+00:00"),
		CodeSize:         1048576,
		State:            lambdatypes.StateActive,
		PackageType:      lambdatypes.PackageTypeZip,
		Architectures:    []lambdatypes.Architecture{lambdatypes.ArchitectureX8664},
		EphemeralStorage: &lambdatypes.EphemeralStorage{Size: aws.Int32(512)},
		TracingConfig:    &lambdatypes.TracingConfigResponse{Mode: lambdatypes.TracingModePassThrough},
		FileSystemConfigs: []lambdatypes.FileSystemConfig{
			{Arn: aws.String(ProdEFSAccessPointBARN), LocalMountPath: aws.String("/mnt/reports")},
		},
		VpcConfig: &lambdatypes.VpcConfigResponse{
			VpcId:            aws.String(ProdEFSVpcID),
			SubnetIds:        []string{ProdEFSSubnetBID},
			SecurityGroupIds: []string{ProdEFSSecurityGroupAID},
		},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String("/aws/lambda/" + ProdEFSLambdaBName),
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
		case "data-pipeline-transform":
			// data-pipeline-transform is triggered by both a Kinesis stream and
			// an MSK cluster — required for lambda→kinesis and lambda→msk
			// related-panel pivots. clickstream-ingest and acme-events-prod are
			// real fixtures (kinesis.go, msk.go).
			mappings = append(mappings,
				lambdatypes.EventSourceMappingConfiguration{
					UUID:                 aws.String("esm-data-pipeline-kinesis-01"),
					FunctionArn:          fn.FunctionArn,
					EventSourceArn:       aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"),
					State:                aws.String("Enabled"),
					BatchSize:            aws.Int32(100),
					StartingPosition:     lambdatypes.EventSourcePositionLatest,
					LastProcessingResult: aws.String("OK"),
				},
				lambdatypes.EventSourceMappingConfiguration{
					UUID:                 aws.String("esm-data-pipeline-msk-01"),
					FunctionArn:          fn.FunctionArn,
					EventSourceArn:       aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/a1b2c3d4"),
					State:                aws.String("Enabled"),
					BatchSize:            aws.Int32(100),
					LastProcessingResult: aws.String("OK"),
				},
			)
		}
	}
	return mappings
}

func init() {
	Register(Pin{ShortName: "lambda", Rows: 36, Issues: 16})
}
