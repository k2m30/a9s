// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// asgDeleting reports a group AWS is tearing down. Its capacity is on its
// way to zero and nothing about it can be reconfigured. The single place that
// fact is spelled: the fetcher's lifecycle switch and posture rules and the
// launch-configuration pass in the Wave-2 enricher all call it.
func asgDeleting(status string) bool {
	return status == "Delete in progress"
}

// FetchAutoScalingGroupsPage fetches a single page of Auto Scaling groups.
func FetchAutoScalingGroupsPage(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Auto Scaling groups: %w", err)
	}

	var resources []resource.Resource

	for _, asg := range output.AutoScalingGroups {
		asgName := ""
		if asg.AutoScalingGroupName != nil {
			asgName = *asg.AutoScalingGroupName
		}

		minSize := ""
		if asg.MinSize != nil {
			minSize = fmt.Sprintf("%d", *asg.MinSize)
		}

		maxSize := ""
		if asg.MaxSize != nil {
			maxSize = fmt.Sprintf("%d", *asg.MaxSize)
		}

		desired := ""
		if asg.DesiredCapacity != nil {
			desired = fmt.Sprintf("%d", *asg.DesiredCapacity)
		}

		instances := fmt.Sprintf("%d", len(asg.Instances))

		status := ""
		if asg.Status != nil {
			status = *asg.Status
		}

		unhealthyCount := 0
		inServiceCount := 0
		for _, inst := range asg.Instances {
			if inst.HealthStatus != nil && *inst.HealthStatus == "Unhealthy" {
				unhealthyCount++
			}
			if inst.LifecycleState == autoscalingtypes.LifecycleStateInService {
				inServiceCount++
			}
		}

		var suspendedNames []string
		for _, sp := range asg.SuspendedProcesses {
			if sp.ProcessName != nil {
				suspendedNames = append(suspendedNames, *sp.ProcessName)
			}
		}
		suspendedProcesses := strings.Join(suspendedNames, ",")

		vpcZoneIdentifier := ""
		if asg.VPCZoneIdentifier != nil {
			vpcZoneIdentifier = *asg.VPCZoneIdentifier
		}

		r := resource.Resource{
			ID:   asgName,
			Name: asgName,
			// Status intentionally unset — lifecycle state is emitted as a Finding.
			Fields: map[string]string{
				"asg_name":                  asgName,
				"min_size":                  minSize,
				"max_size":                  maxSize,
				"desired":                   desired,
				"instances":                 instances,
				"status":                    status,
				"instances_unhealthy_count": fmt.Sprintf("%d", unhealthyCount),
				"in_service_count":          fmt.Sprintf("%d", inServiceCount),
				"suspended_processes":       suspendedProcesses,
				"vpc_zone_identifier":       vpcZoneIdentifier,
			},
			RawStruct: asg,
		}

		minSizeInt := 0
		if asg.MinSize != nil {
			minSizeInt = int(*asg.MinSize)
		}
		r.Findings = asgHealthFindings(status, inServiceCount, unhealthyCount, minSizeInt, suspendedProcesses)

		// Posture signals, each evaluated independently of the health switch
		// above and of one another.
		if asgDeleting(status) {
			resources = append(resources, r)
			continue
		}
		if asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != "" {
			r.Findings = append(r.Findings, wave1Finding(CodeASGLegacyLaunchConfig))
			addWave1Rows(&r, CodeASGLegacyLaunchConfig, domain.DetailRow{
				Label: "Launch configuration", Value: *asg.LaunchConfigurationName, Tier: "~",
			})
		}
		if len(asg.AvailabilityZones) < 2 {
			r.Findings = append(r.Findings, wave1Finding(CodeASGSingleAZ))
			addWave1Rows(&r, CodeASGSingleAZ, domain.DetailRow{
				Label: "Availability zones", Value: strings.Join(asg.AvailabilityZones, ", "), Tier: "~",
			})
		}
		if (len(asg.LoadBalancerNames) > 0 || len(asg.TargetGroupARNs) > 0) &&
			aws.ToString(asg.HealthCheckType) != "ELB" {
			r.Findings = append(r.Findings, wave1Finding(CodeASGNoELBHealthCheck))
			// The API spells the type "EC2"/"ELB"; the row says which check the
			// group runs, not how the SDK spells it.
			addWave1Rows(&r, CodeASGNoELBHealthCheck, domain.DetailRow{
				Label: "Health check type", Value: strings.ToLower(aws.ToString(asg.HealthCheckType)), Tier: "~",
			})
		}

		resources = append(resources, r)
	}

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

// asgHealthFindings is the one predicate for a group's health, in precedence
// order: a group being deleted reports nothing else, then too few instances in
// service, then unhealthy instances, then scaling processes an operator
// suspended. colorASG runs it over Fields for rows built outside the fetcher.
func asgHealthFindings(status string, inServiceCount, unhealthyCount, minSize int, suspendedProcesses string) []domain.Finding {
	switch {
	case asgDeleting(status):
		return []domain.Finding{wave1Finding(CodeASGStateDeleting)}
	case inServiceCount < minSize:
		return []domain.Finding{wave1Finding(CodeASGUnderprovisioned, strconv.Itoa(inServiceCount), strconv.Itoa(minSize))}
	case unhealthyCount > 0:
		return []domain.Finding{wave1Finding(CodeASGUnhealthyInstances, strconv.Itoa(unhealthyCount))}
	case strings.Contains(suspendedProcesses, "Launch") ||
		strings.Contains(suspendedProcesses, "Terminate") ||
		strings.Contains(suspendedProcesses, "HealthCheck"):
		return []domain.Finding{wave1Finding(CodeASGScalingSuspended)}
	}
	return nil
}
