package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
)

func TestT11aDev_CaptureLT_RecordsTheDefaultVersion(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "DescribeLaunchTemplates":
			return &ec2.DescribeLaunchTemplatesOutput{LaunchTemplates: []ec2types.LaunchTemplate{
				{LaunchTemplateId: aws.String("lt-0aaa"), LaunchTemplateName: aws.String("web"), DefaultVersionNumber: aws.Int64(2), LatestVersionNumber: aws.Int64(3)},
				{LaunchTemplateId: aws.String("lt-0bbb"), LaunchTemplateName: aws.String("denied")},
			}}, nil
		case "DescribeLaunchTemplateVersions":
			if aws.ToString(in.(*ec2.DescribeLaunchTemplateVersionsInput).LaunchTemplateId) == "lt-0bbb" {
				return nil, fmt.Errorf("denied")
			}
			return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{{
				CreateTime: aws.Time(time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)),
				LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
					ImageId:         aws.String("ami-0123456789abcdef0"),
					MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{HttpTokens: ec2types.LaunchTemplateHttpTokensStateOptional},
					BlockDeviceMappings: []ec2types.LaunchTemplateBlockDeviceMapping{
						{Ebs: &ec2types.LaunchTemplateEbsBlockDevice{Encrypted: aws.Bool(false)}},
					},
				},
			}}}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureLT(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureLT: %v", err)
	}
	tpls := got.(ltData).Templates
	if len(tpls) != 2 {
		t.Fatalf("templates = %d, want 2", len(tpls))
	}
	v := tpls[0].DefaultVersion
	if tpls[0].DefaultVersionNumber != 2 || tpls[0].LatestVersionNumber != 3 || v.Outcome != "ok" ||
		v.ImageId != "ami-0123456789abcdef0" || v.HttpTokens != "optional" || len(v.BlockDevices) != 1 || v.BlockDevices[0].Encrypted == nil || *v.BlockDevices[0].Encrypted {
		t.Errorf("web template = %+v", tpls[0])
	}
	if tpls[1].DefaultVersion.Outcome != "error" {
		t.Errorf("a denied version describe keeps the row with outcome error, got %+v", tpls[1].DefaultVersion)
	}
}

func TestT11aDev_CaptureVPCPeer_RecordsStatusAndBothSides(t *testing.T) {
	cfg := answeringConfig(func(_, op string, _ any) (any, error) {
		if op != "DescribeVpcPeeringConnections" {
			return nil, fmt.Errorf("unexpected %s", op)
		}
		return &ec2.DescribeVpcPeeringConnectionsOutput{VpcPeeringConnections: []ec2types.VpcPeeringConnection{{
			VpcPeeringConnectionId: aws.String("pcx-0aaa"),
			Status:                 &ec2types.VpcPeeringConnectionStateReason{Code: ec2types.VpcPeeringConnectionStateReasonCodePendingAcceptance, Message: aws.String("Pending Acceptance by 210987654321")},
			RequesterVpcInfo: &ec2types.VpcPeeringConnectionVpcInfo{
				VpcId: aws.String("vpc-0req"), OwnerId: aws.String("123456789012"),
				CidrBlockSet: []ec2types.CidrBlock{{CidrBlock: aws.String("10.0.0.0/16")}},
			},
			AccepterVpcInfo: &ec2types.VpcPeeringConnectionVpcInfo{VpcId: aws.String("vpc-0acc"), OwnerId: aws.String("210987654321")},
			Tags:            []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("hub")}},
		}}}, nil
	})

	got, err := captureVPCPeer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureVPCPeer: %v", err)
	}
	c := got.(vpcPeerData).Connections
	if len(c) != 1 || c[0].StatusCode != "pending-acceptance" || c[0].Requester.VpcId != "vpc-0req" ||
		fmt.Sprint(c[0].Requester.CidrBlocks) != "[10.0.0.0/16]" || c[0].Accepter.OwnerId != "210987654321" || c[0].Tags["Name"] != "hub" {
		t.Errorf("connections = %+v", c)
	}
}

func TestT11aDev_CaptureTransfer_RecordsDescribeOutcomePerServer(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListServers":
			return &transfer.ListServersOutput{Servers: []transfertypes.ListedServer{
				{ServerId: aws.String("s-0aaa"), State: transfertypes.StateOnline},
				{ServerId: aws.String("s-0bbb"), State: transfertypes.StateOffline},
			}}, nil
		case "DescribeServer":
			if aws.ToString(in.(*transfer.DescribeServerInput).ServerId) == "s-0bbb" {
				return nil, fmt.Errorf("denied")
			}
			return &transfer.DescribeServerOutput{Server: &transfertypes.DescribedServer{
				ServerId: aws.String("s-0aaa"), State: transfertypes.StateOnline,
				SecurityPolicyName: aws.String("TransferSecurityPolicy-2024-01"),
				UserCount:          aws.Int32(4),
			}}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureTransfer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureTransfer: %v", err)
	}
	s := got.(transferData).Servers
	if len(s) != 2 || s[0].Outcome != "ok" || s[0].SecurityPolicyName != "TransferSecurityPolicy-2024-01" || s[0].UserCount != 4 || s[0].LoggingRole != "" {
		t.Errorf("servers[0] = %+v", s)
	}
	if s[1].Outcome != "error" || s[1].State != "OFFLINE" {
		t.Errorf("a denied describe keeps the listed state with outcome error, got %+v", s[1])
	}
}

func TestT11aDev_CaptureMWAA_RecordsGetEnvironmentOutcomePerName(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListEnvironments":
			return &mwaa.ListEnvironmentsOutput{Environments: []string{"airflow-prod", "airflow-denied"}}, nil
		case "GetEnvironment":
			if aws.ToString(in.(*mwaa.GetEnvironmentInput).Name) == "airflow-denied" {
				return nil, fmt.Errorf("denied")
			}
			return &mwaa.GetEnvironmentOutput{Environment: &mwaatypes.Environment{
				Status:              mwaatypes.EnvironmentStatusAvailable,
				WebserverAccessMode: mwaatypes.WebserverAccessModePublicOnly,
				LastUpdate: &mwaatypes.LastUpdate{
					Status: mwaatypes.UpdateStatusFailed,
					Error:  &mwaatypes.UpdateError{ErrorCode: aws.String("INTERNAL_ERROR")},
				},
			}}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureMWAA(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureMWAA: %v", err)
	}
	e := got.(mwaaData).Environments
	if len(e) != 2 || e[0].Status != "AVAILABLE" || e[0].WebserverAccessMode != "PUBLIC_ONLY" || e[0].LastUpdateStatus != "FAILED" || e[0].LastUpdateError != "INTERNAL_ERROR" {
		t.Errorf("environments[0] = %+v", e)
	}
	if e[1].Outcome != "error" {
		t.Errorf("a denied GetEnvironment keeps the name with outcome error, got %+v", e[1])
	}
}
