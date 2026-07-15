// ec2_issue_enrichment.go — Wave 2 issue enrichment for the ec2 resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ec2 canonical FindingCodes.
const (
	ec2CodeInstanceStatusImpaired domain.FindingCode = "ec2.instance-status-impaired"
	// Scheduled-event findings intentionally reuse the code above, distinguished
	// by tier ("~" vs "!"); a separate code is unnecessary while the ec2 status
	// and scheduled-event checks live in one enricher function.
)

// ec2StatusFinding is the (tier, phrase, detail) triple for a single
// SystemStatus/InstanceStatus value, per docs/resources/ec2.md §4 rows
// "impaired" / "initializing" / "insufficient-data" (lines 225-227).
type ec2StatusFinding struct {
	tier   string
	phrase string
	detail string
}

// classifyEC2Status maps an AWS instance/system status-check value to the
// severity, list phrase (S4), and detail sentence (S5) mandated by
// docs/resources/ec2.md §4. Only "impaired" is Broken; "initializing" and
// "insufficient-data" are Warning and must never carry the "impaired"
// wording. "ok" and "not-applicable" produce no finding — "not-applicable"
// is explicitly out of scope per docs/resources/ec2.md §5 (line 244): AWS
// classifies it as Healthy/informational, not surfaced.
func classifyEC2Status(status ec2types.SummaryStatus) (ec2StatusFinding, bool) {
	switch status {
	case ec2types.SummaryStatusImpaired:
		return ec2StatusFinding{
			tier:   "!",
			phrase: "impaired: system checks failing",
			detail: "AWS reports this instance is impaired — system or instance status checks are failing.",
		}, true
	case ec2types.SummaryStatusInitializing:
		return ec2StatusFinding{
			tier:   "~",
			phrase: "initializing: checks in progress",
			detail: "Instance status checks have not yet passed since start.",
		}, true
	case ec2types.SummaryStatusInsufficientData:
		return ec2StatusFinding{
			tier:   "~",
			phrase: "status unknown: AWS insufficient-data",
			detail: "AWS cannot determine status — insufficient data from the hypervisor.",
		}, true
	default:
		// "ok" and "not-applicable" — no finding.
		return ec2StatusFinding{}, false
	}
}

// EnrichEC2InstanceStatus calls DescribeInstanceStatus(IncludeAllInstances=true) (account-wide,
// paginated) and returns a Finding for every instance whose system or instance status is not "ok".
// Scheduled events with NotBeforeDeadline within the next 7 days also produce a Finding.
// Severity "!" for status != ok; "~" for scheduled events. IssueCount counts "!" findings only.
// Pagination uses NextToken; walks up to EnrichmentCap pages.
func EnrichEC2InstanceStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.EC2 == nil {
		return result, nil
	}
	// Build a set of known resource IDs so we can detect unmatched API returns.
	knownIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			knownIDs[r.ID] = true
		}
	}
	var allInstanceStatuses []ec2types.InstanceStatus
	var nextToken *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2svc.DescribeInstanceStatusInput{
			IncludeAllInstances: aws.Bool(true),
			NextToken:           nextToken,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: result.TruncatedIDs}, err
		}
		allInstanceStatuses = append(allInstanceStatuses, out.InstanceStatuses...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	now := time.Now()
	cutoff := now.Add(7 * 24 * time.Hour)

	for _, is := range allInstanceStatuses {
		if is.InstanceId == nil {
			continue
		}
		id := *is.InstanceId
		// Track unmatched: API returned an instance not in the input resources slice.
		if len(knownIDs) > 0 && !knownIDs[id] {
			continue
		}

		// Collect rows for this instance.
		var rows []domain.DetailRow
		severity := "~" // start informational; upgrade to "!" only for a real "impaired" status
		var statusFinding ec2StatusFinding
		haveStatusFinding := false

		// Check instance status (docs/resources/ec2.md §4 lines 225-227).
		if is.InstanceStatus != nil {
			if sf, ok := classifyEC2Status(is.InstanceStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.InstanceStatus.Status))
				rows = append(rows, domain.DetailRow{Label: "Instance Status", Value: statusStr, Tier: sf.tier})
				if sf.tier == "!" {
					severity = "!"
				}
				if !haveStatusFinding || sf.tier == "!" {
					statusFinding = sf
					haveStatusFinding = true
				}
			}
		}

		// Check system status (docs/resources/ec2.md §4 lines 225-227).
		if is.SystemStatus != nil {
			if sf, ok := classifyEC2Status(is.SystemStatus.Status); ok {
				statusStr := domain.HumanizeStatusPhrase(string(is.SystemStatus.Status))
				rows = append(rows, domain.DetailRow{Label: "System Status", Value: statusStr, Tier: sf.tier})
				if sf.tier == "!" {
					severity = "!"
				}
				if !haveStatusFinding || sf.tier == "!" {
					statusFinding = sf
					haveStatusFinding = true
				}
			}
		}

		// Check scheduled events within 7 days.
		// NotBeforeDeadline is the hard deadline (forced retirement/reboot).
		// NotBefore is the earliest scheduled start — also within 7d is actionable.
		for _, ev := range is.Events {
			var eventDate *time.Time
			if ev.NotBeforeDeadline != nil && ev.NotBeforeDeadline.Before(cutoff) {
				eventDate = ev.NotBeforeDeadline
			} else if ev.NotBefore != nil && ev.NotBefore.Before(cutoff) {
				eventDate = ev.NotBefore
			}
			if eventDate == nil {
				continue
			}
			code := string(ev.Code)
			dateStr := eventDate.Format("2006-01-02")
			rows = append(rows, domain.DetailRow{
				Label: "Scheduled Event",
				Value: fmt.Sprintf("%s at %s", code, dateStr),
				Tier:  "~",
			})
		}

		if len(rows) == 0 {
			continue
		}

		// Build summary from the status-check classification (docs/resources/ec2.md
		// §4 lines 225-227); an "impaired" status-check row takes priority over a
		// scheduled-event summary since it is the most severe signal.
		summary := ""
		detail := ""
		if haveStatusFinding {
			summary = statusFinding.phrase
			detail = statusFinding.detail
		}
		if summary == "" && len(rows) > 0 {
			summary = fmt.Sprintf("scheduled event: %s", rows[0].Value)
		}

		setWave2Finding(&result, id, ec2CodeInstanceStatusImpaired, summary, severity, "ec2", rows, detail)
	}

	issueCount := 0
	for _, fs := range result.Findings {
		for _, f := range fs {
			if f.Severity == domain.SevBroken {
				issueCount++
				break
			}
		}
	}
	result.IssueCount = issueCount
	result.Truncated = truncated
	return result, nil
}
