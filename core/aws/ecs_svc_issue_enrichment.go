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

// ecsEventReason is AWS's own explanation out of a service event message: the
// clause after the words AWS introduces it with, without the documentation
// pointer AWS appends, which is not a reason and is long enough to push the
// cause off a one-line cell.
//
// A message a9s finds no marker in keeps all of its words. AWS wording this
// code has not seen must not lose its reason the way the fixed phrase did,
// and an event a9s cannot parse still said something.
func ecsEventReason(message string) string {
	reason := message
	for _, marker := range []string{" because ", " due to "} {
		if _, tail, ok := strings.Cut(reason, marker); ok {
			reason = tail
			break
		}
	}
	if i := strings.Index(reason, "For more information"); i >= 0 {
		reason = reason[:i]
	}
	reason = strings.TrimSpace(reason)
	// AWS parenthesises the load-balancer reason as "(reason …).", which under
	// a row already labelled Reason says the word twice.
	if rest, ok := strings.CutPrefix(reason, "(reason "); ok {
		reason = strings.TrimSuffix(strings.TrimSuffix(rest, "."), ")")
	}
	return strings.TrimSpace(reason)
}

// EnrichECSServices is a Wave 2 enricher for ECS services.
// It groups services by cluster name, batches DescribeServices calls (up to 10 per
// cluster per call — the ECS API maximum), and raises findings for:
//   - Any deployment with RolloutState == FAILED → "!" finding
//   - deployment circuit-breaker triggered → "!" finding
//   - runningCount < desiredCount with no IN_PROGRESS deployment → "!" finding
//   - Recent events (last 10m) containing "unable to place" or "ELB health checks failed" → "!" finding,
//     each carrying AWS's own reason for the event as a row under a9s's phrase
func EnrichECSServices(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
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
				// Same shape as the event scan below: a9s's words on one row,
				// AWS's reason on the row under it, never behind a colon in the
				// value, which would shape one fact two ways and put AWS's FAILED
				// on the screen.
				hasInProgress := false
				var deploymentRows []domain.DetailRow
				for _, dep := range svc.Deployments {
					if dep.RolloutState == ecstypes.DeploymentRolloutStateInProgress {
						hasInProgress = true
					}
					if dep.RolloutState != ecstypes.DeploymentRolloutStateFailed {
						continue
					}
					reason := aws.ToString(dep.RolloutStateReason)
					deploymentRows = append(deploymentRows, domain.DetailRow{Label: "Deployment", Value: "rollout failed", Tier: "!"})
					if reason != "" {
						deploymentRows = append(deploymentRows, domain.DetailRow{Label: "Reason", Value: reason, Tier: "!"})
					}
					// The circuit breaker is a second thing to say about the
					// same deployment, and AWS only says it inside the reason.
					if strings.Contains(strings.ToLower(reason), "circuit breaker") {
						deploymentRows = append(deploymentRows, domain.DetailRow{Label: "Deployment", Value: "circuit breaker triggered", Tier: "!"})
					}
				}

				// runningCount < desiredCount with no IN_PROGRESS deployment → stuck.
				serviceStuck := svc.DesiredCount > 0 &&
					svc.RunningCount < svc.DesiredCount &&
					!hasInProgress

				// Check recent events for placement/ELB failures.
				var eventRows []domain.DetailRow
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
					phrase := ""
					switch {
					case strings.Contains(msg, "unable to place"):
						phrase = "unable to place task"
					case strings.Contains(msg, "elb health checks failed"), strings.Contains(msg, "health checks failed"):
						phrase = "load balancer health checks failed"
					default:
						continue
					}
					// The phrase is a9s's, so it reads the same however AWS
					// worded the event; the reason under it is AWS's, because
					// only AWS knows whether it was memory, ports or capacity,
					// and that is the difference between a scheduler problem
					// and a code problem.
					eventRows = append(eventRows, domain.DetailRow{Label: "Event", Value: phrase, Tier: "!"})
					if reason := ecsEventReason(*ev.Message); reason != "" {
						eventRows = append(eventRows, domain.DetailRow{Label: "Reason", Value: reason, Tier: "!"})
					}
				}

				// Independent of the deployment health below: a service that
				// hands its tasks public addresses is a posture signal even
				// when every deployment is green.
				if nc := svc.NetworkConfiguration; ecsServiceScheduling(aws.ToString(svc.Status)) &&
					nc != nil && nc.AwsvpcConfiguration != nil &&
					nc.AwsvpcConfiguration.AssignPublicIp == ecstypes.AssignPublicIpEnabled {
					setWave2Finding(&result, svcName, ecsSvcCodePublicIP, []domain.DetailRow{{Label: "Public address assignment", Value: "enabled", Tier: tierOf(ecsSvcCodePublicIP)}})

				}

				if len(deploymentRows) == 0 && !serviceStuck && len(eventRows) == 0 {
					continue
				}

				rows := deploymentRows
				if serviceStuck {
					rows = append(rows, domain.DetailRow{
						Label: "Tasks",
						Value: fmt.Sprintf("running %d / desired %d (stuck)", svc.RunningCount, svc.DesiredCount),
						Tier:  "!",
					})
				}
				rows = append(rows, eventRows...)

				setWave2Finding(&result, svcName, ecsSvcCodeDeploymentFailed, rows)
			}
		}
	}

	SetTruncated(&result, truncated)
	err := Finish(&result, failures, total, op)
	return result, err
}
