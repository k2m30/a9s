// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// asg_issue_enrichment.go — Wave 2 issue enrichment for the asg resource type.
package aws

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// asg canonical FindingCodes.
const (
	asgCodeScalingActivityFailed domain.FindingCode = "asg.scaling-activity-failed"
	asgCodeLaunchConfigIMDSv1    domain.FindingCode = "asg.launch-config.imdsv1"
	asgCodeLaunchConfigPublicIP  domain.FindingCode = "asg.launch-config.public-ip"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	asgCodeLaunchConfigSecret domain.FindingCode = "asg.launch-config.secret"
)

// EnrichASGScalingActivities calls DescribeScalingActivities(MaxRecords=1) for each ASG
// (cap EnrichmentCap) and returns a Finding when the latest activity StatusCode == Failed.
// Severity is "!" (broken/degraded). Summary: "latest scaling activity failed: <statusMessage>".
func EnrichASGScalingActivities(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.AutoScaling == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		name := r.ID
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return clients.AutoScaling.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
				AutoScalingGroupName: &name,
				MaxRecords:           aws.Int32(1),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			truncated = true
			return
		}
		if len(out.Activities) == 0 {
			return
		}
		act := out.Activities[0]
		if act.StatusCode != asgtypes.ScalingActivityStatusCodeFailed {
			return
		}
		statusMsg := ""
		if act.StatusMessage != nil {
			statusMsg = *act.StatusMessage
		}
		rows := []domain.DetailRow{
			{Label: "Status", Value: domain.HumanizeStatusPhrase(string(act.StatusCode)), Tier: "!"},
		}
		if statusMsg != "" {
			rows = append(rows, domain.DetailRow{Label: "Message", Value: statusMsg, Tier: "!"})
		}
		if act.Cause != nil && *act.Cause != "" {
			rows = append(rows, domain.DetailRow{Label: "Cause", Value: *act.Cause})
		}
		if act.StartTime != nil {
			rows = append(rows, domain.DetailRow{Label: "Started", Value: act.StartTime.Format("2006-01-02")})
		}
		setWave2Finding(&result, r.ID, asgCodeScalingActivityFailed, rows)
	})

	SetTruncated(&result, truncated)
	activitiesErr := AggregateFailures("DescribeScalingActivities", failures, total)
	lcErr := asgLaunchConfigurationPosture(ctx, clients, &result, resources)
	return result, errors.Join(activitiesErr, lcErr)
}

// asgLaunchConfigurationPosture describes the launch configurations the
// groups still reference — ONE batched DescribeLaunchConfigurations for all
// of them, not a call per group — and reports what those immutable
// configurations bake into every instance the group launches.
func asgLaunchConfigurationPosture(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, resources []resource.Resource) error {
	groupsByLC := make(map[string][]string)
	for _, r := range resources {
		group, ok := assertStruct[asgtypes.AutoScalingGroup](r.RawStruct)
		if !ok {
			continue
		}
		name := aws.ToString(group.LaunchConfigurationName)
		if name == "" || r.ID == "" || asgDeleting(r.Fields["status"]) {
			continue
		}
		groupsByLC[name] = append(groupsByLC[name], r.ID)
	}
	if len(groupsByLC) == 0 {
		return nil
	}
	names := make([]string, 0, len(groupsByLC))
	for name := range groupsByLC {
		names = append(names, name)
	}
	sort.Strings(names)
	names = capAtEnrichmentCap(result, names, func(n string) []string { return groupsByLC[n] })

	// DescribeLaunchConfigurations pages: asking for EnrichmentCap names fits
	// one page only because the API's default page size happens to match. Both
	// ways a configuration can go unread — dropped at the cap above, or left
	// behind a pending token below — mark the groups that referenced it, so
	// none of them reads "nothing to report" for a posture nobody looked at.
	const op = "DescribeLaunchConfigurations"
	var configs []asgtypes.LaunchConfiguration
	var nextToken *string
	for range PerParentPageCap {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			return clients.AutoScaling.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
				LaunchConfigurationNames: names,
				NextToken:                nextToken,
			})
		})
		if err != nil {
			// Any page failing leaves the whole batch uninspected: pages read
			// before it are dropped rather than applied, so no group reports a
			// clean posture on a partial read.
			var failures []Failure
			groups := 0
			for _, name := range names {
				for _, id := range groupsByLC[name] {
					MarkSkipped(result, id, &failures, err)
					groups++
				}
			}
			// One failure per group, so the aggregate's total counts groups
			// too: several groups can share a launch configuration, and
			// len(names) would read "8 of 5".
			return Finish(result, failures, groups, op)
		}
		configs = append(configs, out.LaunchConfigurations...)
		nextToken = out.NextToken
		if aws.ToString(nextToken) == "" {
			break
		}
	}

	// A token still pending means the cap fell through, not the pages: the
	// configurations behind it were never read, and a group referencing one is
	// as uninspected as a group behind a failed page. Only those groups are
	// marked — a group whose configuration DID arrive was inspected, and
	// hiding its finding behind a "?" would lose a real one.
	if aws.ToString(nextToken) != "" {
		SetTruncated(result, true)
		read := make(map[string]bool, len(configs))
		for _, lc := range configs {
			read[aws.ToString(lc.LaunchConfigurationName)] = true
		}
		for _, name := range names {
			if read[name] {
				continue
			}
			for _, id := range groupsByLC[name] {
				// The page cap stopped the walk; there is no error to record.
				result.TruncatedIDs[id] = true
			}
		}
	}

	for _, lc := range configs {
		name := aws.ToString(lc.LaunchConfigurationName)
		for _, id := range groupsByLC[name] {
			applyLaunchConfigurationFindings(result, id, lc)
		}
	}
	return nil
}

// applyLaunchConfigurationFindings evaluates the three launch-configuration
// posture rules independently; one configuration can trip all three.
func applyLaunchConfigurationFindings(result *IssueEnricherResult, groupID string, lc asgtypes.LaunchConfiguration) {
	// A launch configuration with no MetadataOptions defaults to optional —
	// unlike every other nil in this batch, absence IS the signal here.
	tokens := "unset"
	if lc.MetadataOptions != nil {
		tokens = string(lc.MetadataOptions.HttpTokens)
	}
	if lc.MetadataOptions == nil || lc.MetadataOptions.HttpTokens != asgtypes.InstanceMetadataHttpTokensStateRequired {
		setWave2Finding(result, groupID, asgCodeLaunchConfigIMDSv1, []domain.DetailRow{{Label: "Metadata tokens", Value: tokens, Tier: tierOf(asgCodeLaunchConfigIMDSv1)}})

	}
	if lc.AssociatePublicIpAddress != nil && *lc.AssociatePublicIpAddress {
		setWave2Finding(result, groupID, asgCodeLaunchConfigPublicIP, []domain.DetailRow{{Label: "Public address assignment", Value: "enabled", Tier: tierOf(asgCodeLaunchConfigPublicIP)}})

	}
	if userData := aws.ToString(lc.UserData); userData != "" {
		if rows := secretScanTextRows(decodeUserData(userData)); len(rows) > 0 {
			setWave2Finding(result, groupID, asgCodeLaunchConfigSecret, rows)
		}
	}
}
