package unit

// enrichment_cap_ownership_test.go — every cap has one owner, and a row past a
// cap says so.
//
// Three caps live in core/aws and each one used to be written out by hand at
// every site that needed it: the per-item work-list cap (capAtEnrichmentCap,
// already owned), the account-wide page cap, and the supporting-row cap
// (capRows, already owned). A cap written by hand is a cap that forgets the
// half nobody sees: the rows it dropped. A row whose answer sat on a page
// nobody read must render "not inspected", never inspected-and-healthy.
//
// The pins here hold the page cap to one owner, hold IssueEnricherResult's
// Truncated flag to one writer that cannot lower a flag another pass raised,
// and hold capRows' stored-count parse to the exact shape it writes.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Row 1 — the account-wide page cap
// ---------------------------------------------------------------------------

// alwaysPagingInstanceStatus returns n pages that each name one instance and
// each carry a NextToken, so the walk only ever ends because its own bound
// stopped it.
func alwaysPagingInstanceStatus(n int) []*ec2sdk.DescribeInstanceStatusOutput {
	pages := make([]*ec2sdk.DescribeInstanceStatusOutput, n)
	for i := range pages {
		pages[i] = &ec2sdk.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{
				makeInstanceStatus(fmt.Sprintf("i-page%010d", i), ec2types.SummaryStatusOk),
			},
			NextToken: aws.String(fmt.Sprintf("tok-%d", i+1)),
		}
	}
	return pages
}

// TestPageCap_RowBeyondTheLastWalkedPageIsNotInspected drives an account-wide
// walk of more pages than the cap allows and reads the row the walk never
// reached. DescribeInstanceStatus answers for every instance in the account,
// so an instance whose answer sits past the last walked page was not looked
// at — and a check that did not answer must not render as one that answered
// "healthy".
func TestPageCap_RowBeyondTheLastWalkedPageIsNotInspected(t *testing.T) {
	fake := newEC2PaginatedFake()
	fake.instanceStatusPages = alwaysPagingInstanceStatus(awsclient.EnrichmentCap + 2)

	// The row on page 0 is answered for; the row whose answer would sit on the
	// page past the cap is not.
	seen := "i-page0000000000"
	unseen := fmt.Sprintf("i-page%010d", awsclient.EnrichmentCap+1)

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, ec2InstanceResources(seen, unseen), nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus returned error: %v", err)
	}

	if fake.instanceStatusCallCount > awsclient.EnrichmentCap {
		t.Errorf("DescribeInstanceStatus called %d times, want at most %d — the walk is not bounded",
			fake.instanceStatusCallCount, awsclient.EnrichmentCap)
	}
	if _, marked := result.TruncatedIDs[unseen]; !marked {
		t.Errorf("row %q is not in TruncatedIDs — its answer sat past the last walked page, so it renders as inspected-and-healthy", unseen)
	}
	if _, marked := result.TruncatedIDs[seen]; marked {
		t.Errorf("row %q is marked uninspected although page 1 answered for it", seen)
	}
	if !result.Truncated {
		t.Error("result.Truncated = false, want true — the walk was cut short and this enricher emits \"!\" findings")
	}
}

// TestPageCap_CompletedWalkMarksNoRowUninspected is the negative half: a walk
// that reaches the last page answered for every row, including the rows it
// named on no page at all — those are healthy, not unknown.
func TestPageCap_CompletedWalkMarksNoRowUninspected(t *testing.T) {
	fake := newEC2PaginatedFake()
	fake.instanceStatusPages = []*ec2sdk.DescribeInstanceStatusOutput{
		{
			InstanceStatuses: []ec2types.InstanceStatus{makeInstanceStatus("i-aaa001", ec2types.SummaryStatusOk)},
			NextToken:        aws.String("p1"),
		},
		{
			InstanceStatuses: []ec2types.InstanceStatus{makeInstanceStatus("i-aaa002", ec2types.SummaryStatusOk)},
		},
	}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, ec2InstanceResources("i-aaa001", "i-aaa002", "i-aaa003"), nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus returned error: %v", err)
	}
	if len(result.TruncatedIDs) != 0 {
		t.Errorf("a complete walk marked %d row(s) uninspected: %v", len(result.TruncatedIDs), result.TruncatedIDs)
	}
	if result.Truncated {
		t.Error("result.Truncated = true after a walk that reached the last page")
	}
}

// ---------------------------------------------------------------------------
// Row 3 — one writer for Truncated
// ---------------------------------------------------------------------------

// TestSetTruncated_NeverLowersAFlagAlreadyRaised pins what the writer is for.
// An enricher that caps its work list and then finishes its remaining batches
// without a failure must still report the cut: Finish's zero-failure case, and
// every later pass, compose with the flag rather than overwrite it.
func TestSetTruncated_NeverLowersAFlagAlreadyRaised(t *testing.T) {
	result := awsclient.IssueEnricherResult{TruncatedIDs: map[string]string{}}
	awsclient.SetTruncated(&result, true)
	awsclient.SetTruncated(&result, false)
	if !result.Truncated {
		t.Error("SetTruncated(result, false) lowered a flag an earlier pass raised")
	}

	fresh := awsclient.IssueEnricherResult{TruncatedIDs: map[string]string{}}
	awsclient.SetTruncated(&fresh, false)
	if fresh.Truncated {
		t.Error("SetTruncated(result, false) raised the flag on an untouched result")
	}
}

// ---------------------------------------------------------------------------
// Row 5 — capRows' stored-count parse
// ---------------------------------------------------------------------------

// scheduledEventStatus returns an instance status carrying one scheduled event
// whose code is eventCode, due tomorrow so the enricher reports it.
func scheduledEventStatus(instanceID, eventCode string) ec2types.InstanceStatus {
	due := time.Now().Add(24 * time.Hour)
	return ec2types.InstanceStatus{
		InstanceId:     aws.String(instanceID),
		InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
		SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
		Events: []ec2types.InstanceStatusEvent{{
			Code:      ec2types.EventCode(eventCode),
			NotBefore: &due,
		}},
	}
}

// TestCapRows_TrailingInputIsNotAStoredCount feeds the row cap a stored row
// that opens with the closing row's wording and carries text after it. Only
// the row capRows itself writes is a stored count; anything else is one more
// row of content and must survive verbatim.
//
// The path is real: an instance named on two pages of DescribeInstanceStatus
// emits the scheduled-event finding twice for one (resource, code) pair, so
// the second emission re-reads the first's last row. Its value is built from
// the event code AWS returned, which is free text this pin sets to the shape
// that misparses.
func TestCapRows_TrailingInputIsNotAStoredCount(t *testing.T) {
	const id = "i-0cap111111111111c"
	fake := newEC2PaginatedFake()
	fake.instanceStatusPages = []*ec2sdk.DescribeInstanceStatusOutput{
		{
			InstanceStatuses: []ec2types.InstanceStatus{scheduledEventStatus(id, "… +3 more")},
			NextToken:        aws.String("p1"),
		},
		{
			InstanceStatuses: []ec2types.InstanceStatus{scheduledEventStatus(id, "system-reboot")},
		},
	}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, ec2InstanceResources(id), nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus returned error: %v", err)
	}

	rows := soleAttentionRows(t, result, id)
	if len(rows) != 2 {
		t.Fatalf("finding carries %d rows, want 2 (one scheduled event per page, neither read as a stored count); rows=%+v", len(rows), rows)
	}
	if !strings.HasPrefix(rows[0].Value, "… +3 more at ") {
		t.Errorf("row 0 Value = %q, want the first page's event verbatim — a row with text after the closing wording was read as a stored count", rows[0].Value)
	}
	for _, r := range rows {
		if r.Value == capOverflowValue(3) {
			t.Errorf("the finding gained a closing row for 3 hidden rows out of 2 real ones: %+v", rows)
		}
	}
}

// TestCapRows_ZeroAndNegativeCountsAreContent covers the other two values that
// round-trip through the closing row's wording but that capRows never writes:
// it emits a closing row only for a count above zero, so a row reading
// "… +0 more" or "… +-2 more" is one more line of the list.
func TestCapRows_ZeroAndNegativeCountsAreContent(t *testing.T) {
	for _, eventCode := range []string{"… +0 more", "… +-2 more"} {
		t.Run(eventCode, func(t *testing.T) {
			const id = "i-0cap222222222222c"
			fake := newEC2PaginatedFake()
			fake.instanceStatusPages = []*ec2sdk.DescribeInstanceStatusOutput{
				{
					InstanceStatuses: []ec2types.InstanceStatus{scheduledEventStatus(id, eventCode)},
					NextToken:        aws.String("p1"),
				},
				{
					InstanceStatuses: []ec2types.InstanceStatus{scheduledEventStatus(id, "system-reboot")},
				},
			}

			result, err := awsclient.EnrichEC2InstanceStatus(context.Background(),
				&awsclient.ServiceClients{EC2: fake}, ec2InstanceResources(id), nil)
			if err != nil {
				t.Fatalf("EnrichEC2InstanceStatus returned error: %v", err)
			}
			rows := soleAttentionRows(t, result, id)
			if len(rows) != 2 {
				t.Fatalf("finding carries %d rows, want 2 — %q was read as a stored count; rows=%+v", len(rows), eventCode, rows)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 2 — the ECS task cap bites at the advertised count
// ---------------------------------------------------------------------------

// ecsTaskCapFake answers DescribeTasks for whatever it is asked and records
// every task ID the enricher asked about, so the pin reads the work the cap
// was supposed to bound rather than the calls it took to do it.
type ecsTaskCapFake struct {
	awsclient.ECSAPI
	mu      sync.Mutex
	askedAt []string
}

func (f *ecsTaskCapFake) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	f.mu.Lock()
	f.askedAt = append(f.askedAt, in.Tasks...)
	f.mu.Unlock()
	out := make([]ecstypes.Task, 0, len(in.Tasks))
	for _, id := range in.Tasks {
		out = append(out, ecstypes.Task{
			TaskArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + id),
			ClusterArn: aws.String(ecsCapClusterARN),
			LastStatus: aws.String("RUNNING"),
		})
	}
	return &ecs.DescribeTasksOutput{Tasks: out}, nil
}

const ecsCapClusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster"

// TestECSTasks_CapBitesBeforeTheBatchIsSized pins that one cluster holding
// more tasks than the cap does not describe them all. DescribeTasks takes 100
// task IDs per call, so a cap evaluated once per call rather than once per
// task lets a single cluster of 51 tasks through in one batch and the
// advertised bound never bites.
func TestECSTasks_CapBitesBeforeTheBatchIsSized(t *testing.T) {
	total := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, 0, total)
	for i := range total {
		id := fmt.Sprintf("task-%04d", i)
		resources = append(resources, resource.Resource{
			ID: id, Name: id, Type: "ecs-task",
			Fields: map[string]string{"cluster": ecsCapClusterARN, "task_id": id},
		})
	}

	fake := &ecsTaskCapFake{}
	result, err := awsclient.EnrichECSTasks(context.Background(),
		&awsclient.ServiceClients{ECS: fake}, resources, nil)
	if err != nil {
		t.Fatalf("EnrichECSTasks returned error: %v", err)
	}

	if len(fake.askedAt) > awsclient.EnrichmentCap {
		t.Errorf("DescribeTasks was asked about %d tasks, want at most %d — the cap was evaluated after the batch was sized",
			len(fake.askedAt), awsclient.EnrichmentCap)
	}
	dropped := resources[awsclient.EnrichmentCap].ID
	if _, marked := result.TruncatedIDs[dropped]; !marked {
		t.Errorf("task %q fell outside the cap but is not in TruncatedIDs — it renders as inspected-and-healthy", dropped)
	}
	if _, marked := result.TruncatedIDs[resources[0].ID]; marked {
		t.Errorf("task %q is inside the cap but was marked uninspected", resources[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Row 6 — the demo bench carries a witness for the closing row
// ---------------------------------------------------------------------------

// TestDemo_TargetGroupOverTheRowCapRendersTheClosingRow reads the demo
// fixtures through the demo fake and the real tg enricher. Without a fixture
// whose finding carries more supporting rows than FindingRowCap, `a9s --demo`
// never renders the closing "… +K more" row, so the one surface an operator
// can look at is the one surface never exercised.
func TestDemo_TargetGroupOverTheRowCapRendersTheClosingRow(t *testing.T) {
	f := fixtures.NewELBFixtures()
	targets := f.TargetHealth[fixtures.GRPCTargetGroupARN]
	if len(targets) <= awsclient.FindingRowCap {
		t.Fatalf("the demo bench carries %d targets on %s, which is at most the row cap of %d — no demo finding can render the closing row",
			len(targets), fixtures.GRPCTargetGroupARN, awsclient.FindingRowCap)
	}

	clients := &awsclient.ServiceClients{ELBv2: fakes.NewELB()}
	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients,
		tgCapResources("acme-grpc-tg", fixtures.GRPCTargetGroupARN), nil)
	if err != nil {
		t.Fatalf("EnrichTargetGroupHealth returned error: %v", err)
	}
	assertCappedRows(t, soleAttentionRows(t, result, "acme-grpc-tg"), len(targets))
}
