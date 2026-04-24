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

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ecs-svc", EnrichECSServices, 100)
}

// EnrichECSServices is a Wave 2 enricher for ECS services.
// It groups services by cluster name, batches DescribeServices calls (up to 10 per
// cluster per call — the ECS API maximum), and raises findings for:
//   - Any deployment with RolloutState == FAILED → "!" finding
//   - deployment circuit-breaker triggered → "!" finding
//   - runningCount < desiredCount with no IN_PROGRESS deployment → "!" finding
//   - Recent events (last 10m) containing "unable to place" or "ELB health checks failed" → "!" finding
func EnrichECSServices(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.ECS == nil || len(resources) == 0 {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}

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

	truncated := len(resources) > EnrichmentCap
	checked := 0
	var failures []string
	total := 0

	for clusterName, svcNames := range clusterServices {
		// ECS DescribeServices accepts up to 10 services per call.
		const descBatch = 10
		for i := 0; i < len(svcNames); i += descBatch {
			if checked >= EnrichmentCap {
				truncated = true
				break
			}
			end := min(i+descBatch, len(svcNames))
			batch := svcNames[i:end]
			checked += len(batch)
			total += len(batch)

			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeServicesOutput, error) {
				return clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
					Cluster:  aws.String(clusterName),
					Services: batch,
				})
			})
			if err != nil {
				for _, svcName := range batch {
					failures = append(failures, fmt.Sprintf("%s/%s: %v", clusterName, svcName, err))
					if r, ok := resourceByService[svcName]; ok {
						truncatedIDs[r.ID] = true
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
					if now.Sub(*ev.CreatedAt) > 10*time.Minute {
						break // Events are newest-first; stop once outside the 10m window.
					}
					msg := strings.ToLower(*ev.Message)
					if strings.Contains(msg, "unable to place") {
						eventIssues = append(eventIssues, "unable to place task")
					} else if strings.Contains(msg, "elb health checks failed") || strings.Contains(msg, "health checks failed") {
						eventIssues = append(eventIssues, "ELB health checks failed")
					}
				}

				if len(deploymentIssues) == 0 && !serviceStuck && len(eventIssues) == 0 {
					continue
				}

				var rows []resource.FindingRow
				for _, issue := range deploymentIssues {
					rows = append(rows, resource.FindingRow{Label: "Deployment", Value: issue, Tier: "!"})
				}
				if serviceStuck {
					rows = append(rows, resource.FindingRow{
						Label: "Tasks",
						Value: fmt.Sprintf("running %d / desired %d (stuck)", svc.RunningCount, svc.DesiredCount),
						Tier:  "!",
					})
				}
				for _, issue := range eventIssues {
					rows = append(rows, resource.FindingRow{Label: "Event", Value: issue, Tier: "!"})
				}

				summary := "deployment failed"
				if len(deploymentIssues) == 0 && serviceStuck {
					summary = fmt.Sprintf("service stuck: running %d / desired %d", svc.RunningCount, svc.DesiredCount)
				} else if len(deploymentIssues) == 0 && len(eventIssues) > 0 {
					summary = eventIssues[0]
				}

				findings[svcName] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
					Rows:     rows,
				}
			}
		}
	}

	return IssueEnricherResult{IssueCount: len(findings), Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings},
		AggregateFailures("ecs-svc-enrich: DescribeServices", failures, total)
}
