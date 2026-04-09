package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
)

// SFNFixtures holds typed fixture data for Step Functions (SFN).
type SFNFixtures struct {
	StateMachines []sfntypes.StateMachineListItem
	Executions    map[string][]sfntypes.ExecutionListItem // key: state machine ARN
}

// NewSFNFixtures constructs SFNFixtures from the canonical demo data.
func NewSFNFixtures() *SFNFixtures {
	const smARNOrderFulfillment = "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"

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
			{
				Name:            aws.String("data-pipeline-orchestrator"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:data-pipeline-orchestrator"),
				Type:            sfntypes.StateMachineTypeStandard,
				CreationDate:    aws.Time(time.Date(2025, 8, 3, 14, 22, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String("payment-validation"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:payment-validation"),
				Type:            sfntypes.StateMachineTypeExpress,
				CreationDate:    aws.Time(time.Date(2025, 11, 20, 10, 45, 0, 0, time.UTC)),
			},
			{
				Name:            aws.String("user-onboarding-flow"),
				StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:user-onboarding-flow"),
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
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:order-fulfillment-workflow:exec-2026-0322-0200-b2c3d4e5"),
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
		},
	}
}
