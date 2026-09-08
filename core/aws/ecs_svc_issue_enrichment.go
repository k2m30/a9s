// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_issue_enrichment.go — Wave 2 issue enrichment for the ecs-svc resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecs-svc canonical FindingCodes.
const (
	ecsSvcCodeDeploymentFailed domain.FindingCode = "ecs-svc.deployment-failed"
	ecsSvcCodePublicIP         domain.FindingCode = "ecs-svc.public-ip"
)

// ecsServiceScheduling reports a service that still launches tasks. An
// inactive or draining service schedules nothing, so its network settings
// hand out nothing. The single lifecycle guard for every ecs-svc posture
// finding.
func ecsServiceScheduling(status string) bool {
	return status != "INACTIVE" && status != "DRAINING"
}

// EnrichECSServices is a Wave 2 enricher for ECS services.
// It groups services by cluster name, batches DescribeServices calls (up to 10 per
// cluster per call — the ECS API maximum), and raises findings for:
//   - Any deployment with RolloutState == FAILED → "!" finding
//   - deployment circuit-breaker triggered → "!" finding
//   - runningCount < desiredCount with no IN_PROGRESS deployment → "!" finding
//   - Recent events (last 10m) containing "unable to place" or "ELB health checks failed" → "!" finding
func EnrichECSServices(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ECS == nil || len(resources) == 0 {
		return result, nil
	}

	// Cap the work list BEFORE grouping: the cap is a limit on how many
	// services a9s looked at, and capping the input keeps which services those
	// are deterministic (grouping first made it depend on map order) while
	// recording every dropped row as uninspected.
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)

	// Group service names by cluster name. Both fields are populated by FetchECSServicesPage.
	clusterServices := make(map[string][]string)
	resourceByService := make(map[string]resource.Resource)
	for _, r := range resources {
		cluster := r.Fields["cluster"]
		svcName := r.Fields["service_name"]
		if cluster == "" || svcName == "" {
			continue
		}
		clusterServices[cluster] = append(clusterServices[cluster], svcName)
		resourceByService[svcName] = r
	}

	truncated := false
	var failures []Failure
	total := 0
	const op = "DescribeServices"

	for clusterName, svcNames := range clusterServices {
		// ECS DescribeServices accepts up to 10 services per call.
		const descBatch = 10
		for i := 0; i < len(svcNames); i += descBatch {
			end := min(i+descBatch, len(svcNames))
			batch := svcNames[i:end]
			total += len(batch)

			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeServicesOutput, error) {
				return clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
					Cluster:  aws.String(clusterName),
					Services: batch,
				})
			})
			if err != nil {
				for _, svcName := range batch {
					if r, ok := resourceByService[svcName]; ok {
						MarkSkipped(&result, r.ID, &failures, err)
					} else {
						failures = append(failures, FailedCall(svcName, err))
					}
				}
				truncated = true
				continue
			}

			now := time.Now()
			for _, svc := range out.Services {
				svcName := ""
				if svc.ServiceName != nil {
					svcName = *svc.ServiceName
				}
				if svcName == "" {
					continue
				}

				// Check deployments for rollout failures and circuit-breaker.
				hasInProgress := false
				var deploymentIssues []string
				for _, dep := range svc.Deployments {
					if dep.RolloutState == ecstypes.DeploymentRolloutStateInProgress {
						hasInProgress = true
					}
					if dep.RolloutState == ecstypes.DeploymentRolloutStateFailed {
						reason := ""
						if dep.RolloutStateReason != nil {
							reason = *dep.RolloutStateReason
						}
						if reason != "" {
							deploymentIssues = append(deploymentIssues, fmt.Sprintf("deployment rollout FAILED: %s", reason))
						} else {
							deploymentIssues = append(deploymentIssues, "deployment rollout FAILED")
						}
						// Detect circuit-breaker in the rollout-state reason.
						if strings.Contains(strings.ToLower(reason), "circuit breaker") {
							deploymentIssues = append(deploymentIssues, "deployment circuit-breaker triggered")
						}
					}
				}

				// runningCount < desiredCount with no IN_PROGRESS deployment → stuck.
				serviceStuck := svc.DesiredCount > 0 &&
					svc.RunningCount < svc.DesiredCount &&
					!hasInProgress

				// Check recent events for placement/ELB failures.
				var eventIssues []string
				for _, ev := range svc.Events {
					if ev.CreatedAt == nil || ev.Message == nil {
						continue
					}
					// The service event list is capped at 100 entries and its
					// order is not part of the API contract, so an old event
					// ends this event, not the scan.
					if now.Sub(*ev.CreatedAt) > 10*time.Minute {
						continue
					}
					msg := strings.ToLower(*ev.Message)
					if strings.Contains(msg, "unable to place") {
						eventIssues = append(eventIssues, "unable to place task")
					} else if strings.Contains(msg, "elb health checks failed") || strings.Contains(msg, "health checks failed") {
						eventIssues = append(eventIssues, "ELB health checks failed")
					}
				}

				// Independent of the deployment health below: a service that
				// hands its tasks public addresses is a posture signal even
				// when every deployment is green.
				if nc := svc.NetworkConfiguration; ecsServiceScheduling(aws.ToString(svc.Status)) &&
					nc != nil && nc.AwsvpcConfiguration != nil &&
					nc.AwsvpcConfiguration.AssignPublicIp == ecstypes.AssignPublicIpEnabled {
					setWave2Finding(&result, svcName, ecsSvcCodePublicIP, "~", "ecs-svc", []domain.DetailRow{{Label: "Public address assignment", Value: "enabled", Tier: "~"}})

				}

				if len(deploymentIssues) == 0 && !serviceStuck && len(eventIssues) == 0 {
					continue
				}

				var rows []domain.DetailRow
				for _, issue := range deploymentIssues {
					rows = append(rows, domain.DetailRow{Label: "Deployment", Value: issue, Tier: "!"})
				}
				if serviceStuck {
					rows = append(rows, domain.DetailRow{
						Label: "Tasks",
						Value: fmt.Sprintf("running %d / desired %d (stuck)", svc.RunningCount, svc.DesiredCount),
						Tier:  "!",
					})
				}
				for _, issue := range eventIssues {
					rows = append(rows, domain.DetailRow{Label: "Event", Value: issue, Tier: "!"})
				}

				setWave2Finding(&result, svcName, ecsSvcCodeDeploymentFailed, "!", "ecs-svc", rows)
			}
		}
	}

	SetTruncated(&result, truncated)
	err := Finish(&result, failures, total, op)
	return result, err
}
