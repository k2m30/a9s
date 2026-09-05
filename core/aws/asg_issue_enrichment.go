// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// asg_issue_enrichment.go — Wave 2 issue enrichment for the asg resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// asg canonical FindingCodes.
const (
	asgCodeScalingActivityFailed domain.FindingCode = "asg.scaling-activity-failed"
	asgCodeLaunchConfigIMDSv1    domain.FindingCode = "asg.launch-config.imdsv1"
	asgCodeLaunchConfigPublicIP  domain.FindingCode = "asg.launch-config.public-ip"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	asgCodeLaunchConfigSecret domain.FindingCode = "asg.launch-config.secret"
)

// S5 operator sentences for the launch-configuration posture codes above.
const (
	asgLaunchConfigIMDSv1Detail   = "Instances this group launches answer metadata requests without a session token, so an SSRF bug on any of them leaks the attached role's credentials. Launch configurations cannot be edited — copy this one to a launch template that requires session tokens and repoint the group."
	asgLaunchConfigPublicIPDetail = "Every instance this group launches gets a routable public address, so each new instance is reachable from the internet on whatever its security groups leave open. Copy the launch configuration to a launch template with public address assignment off."
	//nolint:gosec // G101 false positive: operator prose about a credential, not one
	asgLaunchConfigSecretDetail = "A credential is pasted into the launch configuration's user data, so it is readable by anyone who can call autoscaling:DescribeLaunchConfigurations and lands on every instance the group starts. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it."
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
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
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
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
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
		summary := "latest scaling activity failed"
		if statusMsg != "" {
			summary = fmt.Sprintf("latest scaling activity failed: %s", statusMsg)
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
		setWave2Finding(&result, r.ID, asgCodeScalingActivityFailed, summary, "!", "asg", rows, "")
	})
	sort.Strings(failures)
	result.Truncated = truncated
	activitiesErr := AggregateFailures("asg-enrich: DescribeScalingActivities", failures, total)
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
	if len(names) > EnrichmentCap {
		result.Truncated = true
		names = names[:EnrichmentCap]
	}

	const op = "asg-enrich: DescribeLaunchConfigurations"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
		return clients.AutoScaling.DescribeLaunchConfigurations(ctx, &autoscaling.DescribeLaunchConfigurationsInput{
			LaunchConfigurationNames: names,
		})
	})
	if err != nil {
		var failures []string
		for _, name := range names {
			for _, id := range groupsByLC[name] {
				MarkSkipped(result, id, &failures, op, err)
			}
		}
		return Finish(result, failures, len(names), op)
	}

	for _, lc := range out.LaunchConfigurations {
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
		setWave2Finding(result, groupID, asgCodeLaunchConfigIMDSv1, "launch configuration allows IMDSv1", "~", "asg",
			[]domain.DetailRow{{Label: "Metadata tokens", Value: tokens, Tier: "~"}}, asgLaunchConfigIMDSv1Detail)
	}
	if lc.AssociatePublicIpAddress != nil && *lc.AssociatePublicIpAddress {
		setWave2Finding(result, groupID, asgCodeLaunchConfigPublicIP, "launch configuration assigns public IPs", "~", "asg",
			[]domain.DetailRow{{Label: "Public address assignment", Value: "true", Tier: "~"}}, asgLaunchConfigPublicIPDetail)
	}
	if userData := aws.ToString(lc.UserData); userData != "" {
		if hits := secretscan.ScanText(decodeUserData(userData)); len(hits) > 0 {
			rows := make([]domain.DetailRow, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, domain.DetailRow{Label: h.Where, Value: h.Kind, Tier: "!"})
			}
			setWave2Finding(result, groupID, asgCodeLaunchConfigSecret, "credential in launch configuration user data", "!", "asg",
				rows, asgLaunchConfigSecretDetail)
		}
	}
}
