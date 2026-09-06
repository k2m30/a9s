// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEC2InstancesPage calls the EC2 DescribeInstances API and returns
// a single page of instances. Pass an empty continuationToken for the first page.
func FetchEC2InstancesPage(ctx context.Context, api EC2FetchInstancesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeInstances(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EC2 instances: %w", err)
	}

	var resources []resource.Resource
	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			resources = append(resources, ec2InstanceToResource(inst))
		}
	}

	// Enrich with status check data (graceful degradation on error).
	if len(resources) > 0 {
		enrichEC2StatusChecks(ctx, api, resources)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// ec2InstanceGone reports an instance no operator can still reconfigure.
// It is the single lifecycle guard for every ec2 posture finding — the
// fetcher's Wave-1 rules and the Wave-2 enricher both call it, so the two
// waves can never disagree about which instances are still worth reporting.
func ec2InstanceGone(state string) bool {
	return state == "terminated" || state == "shutting-down"
}

// ec2InstanceToResource builds the canonical EC2 instance Resource (same
// Fields keys, Findings rules) from one SDK Instance — shared by
// FetchEC2InstancesPage and FetchEC2InstancesByIDs so the two paths can
// never drift in shape (mirrors ami.go's imageResource /
// FetchAMIsPage+FetchAMIsByIDs convention).
func ec2InstanceToResource(inst ec2types.Instance) resource.Resource {
	instanceID := ""
	if inst.InstanceId != nil {
		instanceID = *inst.InstanceId
	}

	name := ""
	for _, tag := range inst.Tags {
		if tag.Key != nil && *tag.Key == "Name" {
			if tag.Value != nil {
				name = *tag.Value
			}
			break
		}
	}

	state := ""
	if inst.State != nil {
		state = string(inst.State.Name)
	}
	instanceType := string(inst.InstanceType)

	privateIP := ""
	if inst.PrivateIpAddress != nil {
		privateIP = *inst.PrivateIpAddress
	}

	publicIP := ""
	if inst.PublicIpAddress != nil {
		publicIP = *inst.PublicIpAddress
	}

	launchTime := ""
	if inst.LaunchTime != nil {
		launchTime = inst.LaunchTime.Format("2006-01-02 15:04")
	}

	lifecycle := "on-demand"
	if inst.InstanceLifecycle != "" {
		lifecycle = string(inst.InstanceLifecycle)
	}

	imageID := ""
	if inst.ImageId != nil {
		imageID = *inst.ImageId
	}
	vpcID := ""
	if inst.VpcId != nil {
		vpcID = *inst.VpcId
	}

	stateReasonCode := ""
	if inst.StateReason != nil && inst.StateReason.Code != nil {
		stateReasonCode = *inst.StateReason.Code
	}

	r := resource.Resource{
		ID:   instanceID,
		Name: name,
		Type: "ec2",
		// Status intentionally unset — lifecycle state is emitted as a Finding.
		Fields: map[string]string{
			"instance_id":       instanceID,
			"name":              name,
			"state":             state,
			"type":              instanceType,
			"private_ip":        privateIP,
			"public_ip":         publicIP,
			"launch_time":       launchTime,
			"lifecycle":         lifecycle,
			"image_id":          imageID,
			"vpc_id":            vpcID,
			"state_reason_code": stateReasonCode,
		},
		RawStruct: inst,
	}

	// emit canonical Findings for every non-healthy lifecycle state.
	// Healthy ("running") has no Finding.
	switch state {
	case "pending":
		r.Findings = []domain.Finding{{
			Code: CodeEC2StatePending, Phrase: "pending",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case "shutting-down":
		r.Findings = []domain.Finding{{
			Code: CodeEC2StateShuttingDown, Phrase: "shutting down",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case "stopping":
		r.Findings = []domain.Finding{{
			Code: CodeEC2StateStopping, Phrase: "stopping",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case "stopped":
		if strings.HasPrefix(stateReasonCode, "Server.") {
			r.Findings = []domain.Finding{{
				Code: CodeEC2StateStoppedServer, Phrase: "stopped",
				Severity: domain.SevBroken, Source: "wave1",
			}}
		} else {
			r.Findings = []domain.Finding{{
				Code: CodeEC2StateStopped, Phrase: "stopped",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}
	case "terminated":
		r.Findings = []domain.Finding{{
			Code: CodeEC2StateTerminated, Phrase: "terminated",
			Severity: domain.SevDim, Source: "wave1",
		}}
	}

	// Posture signals. Independently evaluated and appended, so an instance
	// that is both IMDSv1-permissive and publicly addressed carries both.
	if !ec2InstanceGone(state) {
		if inst.MetadataOptions != nil && inst.MetadataOptions.HttpTokens == ec2types.HttpTokensStateOptional {
			r.Findings = append(r.Findings, domain.Finding{
				Code: CodeEC2IMDSv1Allowed, Phrase: "IMDSv1 allowed",
				Detail:   catalog.Detail(CodeEC2IMDSv1Allowed),
				Severity: domain.SevWarn, Source: "wave1",
			})
			addWave1Rows(&r, CodeEC2IMDSv1Allowed, domain.DetailRow{
				Label: "Metadata tokens", Value: "optional", Tier: "~",
			})
		}
		if publicIP != "" {
			r.Findings = append(r.Findings, domain.Finding{
				Code: CodeEC2PublicIP, Phrase: "public address",
				Detail:   catalog.Detail(CodeEC2PublicIP),
				Severity: domain.SevWarn, Source: "wave1",
			})
			addWave1Rows(&r, CodeEC2PublicIP, domain.DetailRow{
				Label: "Public address", Value: publicIP, Tier: "~",
			})
		}
	}

	return r
}

// enrichEC2StatusChecks calls DescribeInstanceStatus for the page's resources
// and merges system_status/instance_status into each resource's Fields map.
// Errors are silently ignored (graceful degradation per design spec).
func enrichEC2StatusChecks(ctx context.Context, api EC2DescribeInstanceStatusAPI, resources []resource.Resource) {
	// Collect instance IDs.
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return
	}

	// Build a map from instance ID to (systemStatus, instanceStatus).
	statusMap := make(map[string][2]string, len(ids))

	// DescribeInstanceStatus accepts max 100 IDs per call.
	const batchSize = 100
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		batch := ids[start:end]

		out, err := api.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
			InstanceIds:         batch,
			IncludeAllInstances: aws.Bool(true),
		})
		if err != nil {
			// Non-fatal: skip enrichment for this batch.
			continue
		}

		for _, s := range out.InstanceStatuses {
			if s.InstanceId == nil {
				continue
			}
			sysStatus := ""
			instStatus := ""
			if s.SystemStatus != nil {
				sysStatus = string(s.SystemStatus.Status)
			}
			if s.InstanceStatus != nil {
				instStatus = string(s.InstanceStatus.Status)
			}
			statusMap[*s.InstanceId] = [2]string{sysStatus, instStatus}
		}
	}

	// Merge status fields into resources and promote Status for running instances.
	for i, r := range resources {
		if pair, ok := statusMap[r.ID]; ok {
			if resources[i].Fields == nil {
				resources[i].Fields = make(map[string]string)
			}
			sysStatus := pair[0]
			instStatus := pair[1]
			if sysStatus != "" {
				resources[i].Fields["system_status"] = sysStatus
			}
			if instStatus != "" {
				resources[i].Fields["instance_status"] = instStatus
			}
		}
	}
}
