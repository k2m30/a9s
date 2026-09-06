// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
)

// SFNLoggingOff, SFNNoCMK and SFNDefinitionSecret name the one demo state
// machine each for execution logging off (sfn.logging-off), AWS-owned-key
// encryption (sfn.no-cmk) and a credential in the definition
// (sfn.definition-secret). Every other state machine logs at ALL, uses a
// customer managed key, and has a clean definition.
const (
	SFNLoggingOff = "sfn-logging-off"
	SFNNoCMK      = "sfn-no-cmk"
	//nolint:gosec // G101 false positive: a fixture resource name, not a credential
	SFNDefinitionSecret = "sfn-definition-secret"

	sfnARNPrefix           = "arn:aws:states:us-east-1:123456789012:stateMachine:"
	sfnARNLoggingOff       = sfnARNPrefix + SFNLoggingOff
	sfnARNNoCMK            = sfnARNPrefix + SFNNoCMK
	sfnARNDefinitionSecret = sfnARNPrefix + SFNDefinitionSecret

	sfnDemoKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
)

// SFNHealthyLogLevel is the execution logging level every demo state machine
// runs at except the one SFNLoggingOff names.
const SFNHealthyLogLevel = sfntypes.LogLevelAll

// SFNFixtures holds typed fixture data for Step Functions (SFN).
type SFNFixtures struct {
	StateMachines []sfntypes.StateMachineListItem
	Executions    map[string][]sfntypes.ExecutionListItem // key: state machine ARN
	// Definitions maps state machine ARN -> ASL definition JSON, served by
	// DescribeStateMachine. Required for the ecs-svc:sfn related-panel pivot
	// witness (checkECSSvcSFN matches Task states whose Resource starts with
	// "arn:aws:states:::ecs:runTask" and whose Parameters.TaskDefinition
	// contains the ECS service's task-definition family name) and for the
	// sfn:lambda related-panel pivot witness (checkSFNLambda walks the
	// definition for Task states referencing a Lambda function ARN).
	Definitions map[string]string
	// RoleArns maps state machine ARN -> execution role ARN, served by
	// DescribeStateMachine. Required for the sfn:role related-panel pivot
	// witness (checkSFNRole). acme-lambda-execution is a real iam.go fixture.
	RoleArns map[string]string
	// EncryptionKeyIDs maps state machine ARN -> KMS key ID, served by
	// DescribeStateMachine. Required for the sfn:kms related-panel pivot
	// witness (checkSFNKMS). Shared prod KMS key used across fixtures.
	EncryptionKeyIDs map[string]string
	// LoggingLevels maps state machine ARN -> its execution logging level,
	// served by DescribeStateMachine. A machine absent from this map is
	// served SFNHealthyLogLevel, so exactly one demo row reads as unlogged.
	LoggingLevels map[string]sfntypes.LogLevel
	// History maps execution ARN -> GetExecutionHistory events, served by
	// SFNFake.GetExecutionHistory. Required for the sfn_execution_history
	// child view and its sfn-execution-history.broken.event_failed finding
	// witness (ClassifyEventStatus classifies *Failed/*TimedOut/
	// ExecutionAborted event types as "failed").
	History map[string][]sfntypes.HistoryEvent
}

// NewSFNFixtures constructs SFNFixtures from the canonical demo data.
var sharedSFNFixtures = sync.OnceValue(func() *SFNFixtures {
	const smARNOrderFulfillment = "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"
	const smARNPaymentValidation = "arn:aws:states:us-east-1:123456789012:stateMachine:payment-validation"
	const smARNUserOnboarding = "arn:aws:states:us-east-1:123456789012:stateMachine:user-onboarding-flow"
	const execArnOrderFulfillmentFailed = "arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0200-b2c3d4e5"

	redriveCount := int32(1)
	redriveDate := time.Date(2026, 3, 21, 19, 0, 0, 0, time.UTC)

	start1 := time.Date(2026, 3, 22, 3, 15, 0, 0, time.UTC)
	stop1 := time.Date(2026, 3, 22, 3, 17, 47, 0, time.UTC)
	start2 := time.Date(2026, 3, 22, 2, 0, 0, 0, time.UTC)
	stop2 := time.Date(2026, 3, 22, 2, 0, 12, 0, time.UTC)
	start3 := time.Date(2026, 3, 22, 1, 30, 0, 0, time.UTC)
	start4 := time.Date(2026, 3, 21, 22, 0, 0, 0, time.UTC)
	stop4 := time.Date(2026, 3, 22, 0, 30, 0, 0, time.UTC)
	start5 := time.Date(2026, 3, 21, 18, 0, 0, 0, time.UTC)
	stop5 := time.Date(2026, 3, 21, 18, 0, 3, 0, time.UTC)
	start6 := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	stop6 := time.Date(2026, 3, 21, 12, 5, 30, 0, time.UTC)
	start7 := time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)
	stop7 := time.Date(2026, 3, 20, 8, 45, 0, 0, time.UTC)

	return &SFNFixtures{
		StateMachines: []sfntypes.StateMachineListItem{
			{
				Name:            aws.String("order-fulfillment-workflow"),
				StateMachineArn: aws.String(smARNOrderFulfillment),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 5, 12, 9, 15, 0, 0, time.UTC)),
			},
			// SFNLoggingOff / SFNNoCMK / SFNDefinitionSecret: one machine per
			// configuration signal, each explicitly healthy on the other two.
			{
				Name:            aws.String(SFNLoggingOff),
				StateMachineArn: aws.String(sfnARNLoggingOff),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String(SFNNoCMK),
				StateMachineArn: aws.String(sfnARNNoCMK),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 9, 8, 10, 0, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String(SFNDefinitionSecret),
				StateMachineArn: aws.String(sfnARNDefinitionSecret),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 9, 15, 10, 0, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String("data-pipeline-orchestrator"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator"),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 8, 3, 14, 22, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String("payment-validation"),
				StateMachineArn: aws.String(smARNPaymentValidation),
				Type:            sfntypes.StateMachineTypeExpress,
				CreationDate:    aws.Time(time.Date(2025, 11, 20, 10, 45, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String("user-onboarding-flow"),
				StateMachineArn: aws.String(smARNUserOnboarding),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2026, 1, 8, 16, 30, 0, 0, time.UTC)),
			},
		},
		Executions: map[string][]sfntypes.ExecutionListItem{
			smARNOrderFulfillment: {
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0315-a1b2c3d4"),
					Name:            aws.String("exec-2026-0322-0315-a1b2c3d4"),
					StartDate:       &start1,
					StopDate:        &stop1,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusSucceeded,
				},
				{
					ExecutionArn:    aws.String(execArnOrderFulfillmentFailed),
					Name:            aws.String("exec-2026-0322-0200-b2c3d4e5"),
					StartDate:       &start2,
					StopDate:        &stop2,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusFailed,
				},
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0130-c3d4e5f6"),
					Name:            aws.String("exec-2026-0322-0130-c3d4e5f6"),
					StartDate:       &start3,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusRunning,
				},
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-2200-d4e5f6a7"),
					Name:            aws.String("exec-2026-0321-2200-d4e5f6a7"),
					StartDate:       &start4,
					StopDate:        &stop4,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusTimedOut,
				},
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1800-e5f6a7b8"),
					Name:            aws.String("exec-2026-0321-1800-e5f6a7b8"),
					StartDate:       &start5,
					StopDate:        &stop5,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusAborted,
					RedriveCount:    &redriveCount,
					RedriveDate:     &redriveDate,
				},
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0321-1200-f6a7b8c9"),
					Name:            aws.String("exec-2026-0321-1200-f6a7b8c9"),
					StartDate:       &start6,
					StopDate:        &stop6,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusPendingRedrive,
				},
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0320-0800-a7b8c9d0"),
					Name:            aws.String("exec-2026-0320-0800-a7b8c9d0"),
					StartDate:       &start7,
					StopDate:        &stop7,
					StateMachineArn: aws.String(smARNOrderFulfillment),
					Status:          sfntypes.ExecutionStatusSucceeded,
				},
			},
			// payment-validation's single (and therefore latest) execution
			// failed — required for EnrichStepFunctionsStatus's Wave-2 issue
			// check.
			smARNPaymentValidation: {
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:payment-validation:exec-2026-0322-0400-b1c2d3e4"),
					Name:            aws.String("exec-2026-0322-0400-b1c2d3e4"),
					StartDate:       aws.Time(time.Date(2026, 3, 22, 4, 0, 0, 0, time.UTC)),
					StopDate:        aws.Time(time.Date(2026, 3, 22, 4, 0, 8, 0, time.UTC)),
					StateMachineArn: aws.String(smARNPaymentValidation),
					Status:          sfntypes.ExecutionStatusFailed,
				},
			},
			// user-onboarding-flow's single (and therefore latest) execution
			// failed on a STANDARD-type state machine — required to keep
			// sfn.latest-execution-failed witnessed by a fixture ListExecutions
			// is actually called against (payment-validation is EXPRESS, which
			// EnrichStepFunctionsStatus now skips pre-call since AWS rejects
			// ListExecutions for that type with StateMachineTypeNotSupported).
			smARNUserOnboarding: {
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:user-onboarding-flow:exec-2026-0322-0500-c2d3e4f5"),
					Name:            aws.String("exec-2026-0322-0500-c2d3e4f5"),
					StartDate:       aws.Time(time.Date(2026, 3, 22, 5, 0, 0, 0, time.UTC)),
					StopDate:        aws.Time(time.Date(2026, 3, 22, 5, 0, 4, 0, time.UTC)),
					StateMachineArn: aws.String(smARNUserOnboarding),
					Status:          sfntypes.ExecutionStatusFailed,
				},
			},
		},
		// order-fulfillment-workflow's ASL definition runs an ECS task on
		// the acme-services cluster using the api-gateway task-definition
		// family (both real ecs.go fixtures) — required for the
		// ecs-svc:sfn related-panel pivot witness (checkECSSvcSFN) — then
		// invokes api-gateway-authorizer (real lambda.go fixture) — required
		// for the sfn:lambda related-panel pivot witness (checkSFNLambda).
		Definitions: map[string]string{
			// The only demo definition with a credential written into it.
			// The value is synthetic; secretscan reports the key and the
			// kind, never the value itself.
			sfnARNDefinitionSecret: `{"Comment":"Partner settlement","StartAt":"CallPartner","States":{"CallPartner":{"Type":"Task","Resource":"arn:aws:states:::http:invoke","Parameters":{"ApiEndpoint":"https://partner.example.com/settle","DB_PASSWORD":"hunter2-correct-horse-battery"},"End":true}}}`,
			smARNOrderFulfillment: `{
				"Comment": "Order fulfillment workflow",
				"StartAt": "RunFulfillmentTask",
				"States": {
					"RunFulfillmentTask": {
						"Type": "Task",
						"Resource": "arn:aws:states:::ecs:runTask.sync",
						"Parameters": {
							"Cluster": "` + ecsClusterArnServices + `",
							"TaskDefinition": "api-gateway"
						},
						"Next": "AuthorizeShipment"
					},
					"AuthorizeShipment": {
						"Type": "Task",
						"Resource": "arn:aws:states:::lambda:invoke",
						"Parameters": {
							"FunctionName": "arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer"
						},
						"End": true
					}
				}
			}`,
			// The remaining three state machines get a real ASL definition too
			// (rather than the fake's "{}" fallback) so the on-demand sfn detail
			// enrichment renders a populated Definition block. Pass/Choice/Wait/
			// Succeed/Fail states only — no Task states, so these definitions
			// carry no Lambda ARNs and no "states:::ecs:runTask" resource,
			// leaving the sfn:lambda and ecs-svc:sfn related-panel pivot counts
			// (which only order-fulfillment-workflow's definition feeds) unchanged.
			"arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator": `{
				"Comment": "Data pipeline orchestration workflow",
				"StartAt": "ValidateInput",
				"States": {
					"ValidateInput": {
						"Type": "Choice",
						"Choices": [
							{"Variable": "$.recordCount", "NumericGreaterThan": 0, "Next": "ProcessBatch"}
						],
						"Default": "NoRecords"
					},
					"ProcessBatch": {
						"Type": "Pass",
						"Result": {"status": "processed"},
						"Next": "WaitForDownstream"
					},
					"WaitForDownstream": {
						"Type": "Wait",
						"Seconds": 30,
						"Next": "Done"
					},
					"NoRecords": {
						"Type": "Succeed"
					},
					"Done": {
						"Type": "Succeed"
					}
				}
			}`,
			smARNPaymentValidation: `{
				"Comment": "Payment validation workflow",
				"StartAt": "CheckAmount",
				"States": {
					"CheckAmount": {
						"Type": "Choice",
						"Choices": [
							{"Variable": "$.amount", "NumericGreaterThan": 10000, "Next": "FlagForReview"}
						],
						"Default": "ApprovePayment"
					},
					"FlagForReview": {
						"Type": "Fail",
						"Error": "PaymentRequiresReview",
						"Cause": "Amount exceeds auto-approval threshold"
					},
					"ApprovePayment": {
						"Type": "Pass",
						"Result": {"approved": true},
						"End": true
					}
				}
			}`,
			smARNUserOnboarding: `{
				"Comment": "User onboarding workflow",
				"StartAt": "CreateProfile",
				"States": {
					"CreateProfile": {
						"Type": "Pass",
						"Result": {"profileCreated": true},
						"Next": "WaitForVerification"
					},
					"WaitForVerification": {
						"Type": "Wait",
						"Seconds": 60,
						"Next": "CheckVerificationStatus"
					},
					"CheckVerificationStatus": {
						"Type": "Choice",
						"Choices": [
							{"Variable": "$.verified", "BooleanEquals": true, "Next": "OnboardingComplete"}
						],
						"Default": "OnboardingIncomplete"
					},
					"OnboardingComplete": {
						"Type": "Succeed"
					},
					"OnboardingIncomplete": {
						"Type": "Fail",
						"Error": "VerificationTimeout",
						"Cause": "User did not complete verification in time"
					}
				}
			}`,
		},
		// order-fulfillment-workflow execution role — required for sfn:role.
		RoleArns: map[string]string{
			smARNOrderFulfillment: fixtIAMProdLambdaRoleARN,
		},
		// order-fulfillment-workflow encryption key — required for sfn:kms.
		// Every machine but SFNNoCMK carries one: an AWS-owned key is a
		// finding, so exactly one demo row may be without.
		EncryptionKeyIDs: map[string]string{
			smARNOrderFulfillment: sfnDemoKMSKeyARN,
			"arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator": sfnDemoKMSKeyARN,
			smARNPaymentValidation: sfnDemoKMSKeyARN,
			smARNUserOnboarding:    sfnDemoKMSKeyARN,
			sfnARNLoggingOff:       sfnDemoKMSKeyARN,
			sfnARNDefinitionSecret: sfnDemoKMSKeyARN,
		},
		// Only SFNLoggingOff records nothing about its executions.
		LoggingLevels: map[string]sfntypes.LogLevel{
			sfnARNLoggingOff: sfntypes.LogLevelOff,
		},
		// exec-2026-0322-0200-b2c3d4e5's history: RunFulfillmentTask's ECS
		// task fails to pull its container image, which fails the .sync
		// task integration and, with no Catch, the execution itself —
		// required for the sfn_execution_history.broken.event_failed
		// witness (both TaskFailed and ExecutionFailed classify "failed").
		History: map[string][]sfntypes.HistoryEvent{
			execArnOrderFulfillmentFailed: {
				{
					Id:        1,
					Timestamp: &start2,
					Type:      sfntypes.HistoryEventTypeExecutionStarted,
					ExecutionStartedEventDetails: &sfntypes.ExecutionStartedEventDetails{
						Input:   aws.String(`{"orderId":"ORD-88213","warehouseId":"WH-4"}`),
						RoleArn: aws.String(fixtIAMProdLambdaRoleARN),
					},
				},
				{
					Id:              2,
					PreviousEventId: 1,
					Timestamp:       &start2,
					Type:            sfntypes.HistoryEventTypeTaskStateEntered,
					StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
						Name:  aws.String("RunFulfillmentTask"),
						Input: aws.String(`{"orderId":"ORD-88213","warehouseId":"WH-4"}`),
					},
				},
				{
					Id:              3,
					PreviousEventId: 2,
					Timestamp:       aws.Time(time.Date(2026, 3, 22, 2, 0, 1, 0, time.UTC)),
					Type:            sfntypes.HistoryEventTypeTaskScheduled,
					TaskScheduledEventDetails: &sfntypes.TaskScheduledEventDetails{
						Resource:     aws.String("ecs:runTask.sync"),
						ResourceType: aws.String("ecs"),
						Region:       aws.String("us-east-1"),
						Parameters:   aws.String(`{"Cluster":"` + ecsClusterArnServices + `","TaskDefinition":"api-gateway"}`),
					},
				},
				{
					Id:              4,
					PreviousEventId: 3,
					Timestamp:       aws.Time(time.Date(2026, 3, 22, 2, 0, 2, 0, time.UTC)),
					Type:            sfntypes.HistoryEventTypeTaskStarted,
					TaskStartedEventDetails: &sfntypes.TaskStartedEventDetails{
						Resource:     aws.String("ecs:runTask.sync"),
						ResourceType: aws.String("ecs"),
					},
				},
				{
					Id:              5,
					PreviousEventId: 4,
					Timestamp:       &stop2,
					Type:            sfntypes.HistoryEventTypeTaskFailed,
					TaskFailedEventDetails: &sfntypes.TaskFailedEventDetails{
						Resource:     aws.String("ecs:runTask.sync"),
						ResourceType: aws.String("ecs"),
						Error:        aws.String("ECS.AmazonECSException"),
						Cause:        aws.String("CannotPullContainerError: pull image manifest has been retried 5 time(s): failed to resolve ref docker.io/acme/fulfillment-worker:2026.03.21: not found"),
					},
				},
				{
					Id:              6,
					PreviousEventId: 5,
					Timestamp:       &stop2,
					Type:            sfntypes.HistoryEventTypeExecutionFailed,
					ExecutionFailedEventDetails: &sfntypes.ExecutionFailedEventDetails{
						Error: aws.String("ECS.AmazonECSException"),
						Cause: aws.String("CannotPullContainerError: pull image manifest has been retried 5 time(s): failed to resolve ref docker.io/acme/fulfillment-worker:2026.03.21: not found"),
					},
				},
			},
		},
	}
})

func NewSFNFixtures() *SFNFixtures {
	return sharedSFNFixtures()
}

func init() {
	Register(Pin{ShortName: "sfn", Rows: 7, Issues: 0, CoverageGaps: []string{"dim"}})
}
