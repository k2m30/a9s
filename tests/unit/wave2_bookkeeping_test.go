package unit_test

// wave2_bookkeeping_test.go — what a Wave 2 enricher records when part of its
// work fails: the findings it did prove survive, a call that failed proves
// nothing, a walk that failed names the call rather than the cap, and the cap
// is spent only on rows the check can apply to.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	backupsdk "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cwlogssvc "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	bkNoTargets   domain.FindingCode = "eb-rule.no-targets"
	bkTargetIssue domain.FindingCode = "eb-rule.target-issue"
)

// bkOpErr is what the SDK hands an enricher when AWS refuses a call.
func bkOpErr(service, op, code string) error {
	return &smithy.OperationError{
		ServiceID:     service,
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    code,
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform this operation",
		},
	}
}

func bkHasCode(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

func bkCodes(fs []domain.Finding) []domain.FindingCode {
	out := make([]domain.FindingCode, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func bkEBTypeDef(t *testing.T) resource.ResourceTypeDef {
	t.Helper()
	td := resource.FindResourceType("eb-rule")
	if td == nil {
		t.Fatal("eb-rule is not registered")
	}
	return *td
}

// bkProvenButTagDenied is the witness result: the target walk proved the
// rule has no targets, and a second check on the same rule was refused.
func bkProvenButTagDenied(t *testing.T, rules []resource.Resource, proven string) awsclient.IssueEnricherResult {
	t.Helper()
	fake := &bkEBFake{targets: map[string][]eventbridgetypes.Target{proven: {}}}
	res, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), &awsclient.ServiceClients{EventBridge: fake}, rules, nil)
	if err != nil {
		t.Fatalf("EnrichEventBridgeRuleTargets: %v", err)
	}
	if !bkHasCode(res.Findings[proven], bkNoTargets) {
		t.Fatalf("setup: the target walk did not prove %q has no targets: %v", proven, bkCodes(res.Findings[proven]))
	}
	var failures []awsclient.Failure
	awsclient.MarkSkipped(&res, proven, &failures, bkOpErr("EventBridge", "ListTagsForResource", "AccessDeniedException"))
	return res
}

// ---------------------------------------------------------------------------
// Row 2 — a row marked not inspected keeps what the result proved for it
// ---------------------------------------------------------------------------

// TestFoldWave2Rows_UninspectedRowKeepsTheFindingItsResultProved folds a
// result into three rows that differ only in what the result says about them:
//   - the proven rule: a finding and a not-inspected mark — the finding lands;
//   - the refused rule: a mark and nothing proven — it keeps the finding it
//     already had, since nothing re-checked it;
//   - the healthy rule: answered, nothing found — its old finding is cleared.
func TestFoldWave2Rows_UninspectedRowKeepsTheFindingItsResultProved(t *testing.T) {
	const proven, refused, healthy = "acme-orders-rule", "acme-billing-rule", "acme-audit-rule"
	td := bkEBTypeDef(t)
	rules := []resource.Resource{
		bkEBRule(proven, "ENABLED"), bkEBRule(refused, "ENABLED"), bkEBRule(healthy, "ENABLED"),
	}

	noDLQ := func(rule string) []eventbridgetypes.Target {
		return []eventbridgetypes.Target{{Id: aws.String(rule + "-lambda"), Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + rule)}}
	}
	first, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), &awsclient.ServiceClients{EventBridge: &bkEBFake{
		targets: map[string][]eventbridgetypes.Target{refused: noDLQ(refused), healthy: noDLQ(healthy)},
	}}, rules, nil)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	runtime.FoldWave2Rows(rules, td, first.Findings, first.AttentionDetails, nil)
	for _, id := range []string{refused, healthy} {
		if !bkHasCode(rules[slices.IndexFunc(rules, func(r resource.Resource) bool { return r.ID == id })].Findings, bkTargetIssue) {
			t.Fatalf("setup: first sweep left %q without %s", id, bkTargetIssue)
		}
	}

	second, _ := awsclient.EnrichEventBridgeRuleTargets(context.Background(), &awsclient.ServiceClients{EventBridge: &bkEBFake{
		targets: map[string][]eventbridgetypes.Target{proven: {}},
		errs:    map[string]error{refused: bkOpErr("EventBridge", "ListTargetsByRule", "AccessDeniedException")},
	}}, rules, nil) //nolint:errcheck // the refused call's aggregate error is not under test
	var failures []awsclient.Failure
	awsclient.MarkSkipped(&second, proven, &failures, bkOpErr("EventBridge", "ListTagsForResource", "AccessDeniedException"))

	runtime.FoldWave2Rows(rules, td, second.Findings, second.AttentionDetails, second.TruncatedIDs)

	got := func(id string) resource.Resource {
		return rules[slices.IndexFunc(rules, func(r resource.Resource) bool { return r.ID == id })]
	}
	p := got(proven)
	if !bkHasCode(p.Findings, bkNoTargets) {
		t.Errorf("%s: row carries %v, want %s — a finding the result proved was dropped because another check on the row was refused",
			proven, bkCodes(p.Findings), bkNoTargets)
	} else {
		i := slices.IndexFunc(p.Findings, func(f domain.Finding) bool { return f.Code == bkNoTargets })
		if p.Findings[i].Source != "wave2:eb-rule" {
			t.Errorf("%s: finding Source = %q, want %q", proven, p.Findings[i].Source, "wave2:eb-rule")
		}
		if rows := p.AttentionDetails[bkNoTargets].Rows; len(rows) == 0 {
			t.Errorf("%s: the folded finding lost its supporting rows", proven)
		}
	}
	if r := got(refused); !bkHasCode(r.Findings, bkTargetIssue) || bkHasCode(r.Findings, bkNoTargets) {
		t.Errorf("%s: row carries %v, want exactly its earlier %s — nothing re-checked it", refused, bkCodes(r.Findings), bkTargetIssue)
	}
	if h := got(healthy); len(h.Findings) != 0 {
		t.Errorf("%s: row carries %v, want none — the result answered for it and found nothing", healthy, bkCodes(h.Findings))
	}
}

// TestUninspectedRowWithProvenFinding_ListBadgeAndDetailAgree drives the
// witness through the running app: the list row, the menu badge and the
// detail view all read one folded row, and the not-inspected mark still
// renders beside the finding.
func TestUninspectedRowWithProvenFinding_ListBadgeAndDetailAgree(t *testing.T) {
	const proven, healthy = "acme-orders-rule", "acme-audit-rule"
	rules := []resource.Resource{bkEBRule(proven, "ENABLED"), bkEBRule(healthy, "ENABLED")}
	res := bkProvenButTagDenied(t, rules, proven)

	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "eb-rule"})
	c.ApplyResourcesLoaded("eb-rule", rules, nil, false)
	core.ObserveRows("eb-rule", rules, nil, session.OriginFetch, false)

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType:     "eb-rule",
		Findings:         res.Findings,
		AttentionDetails: res.AttentionDetails,
		TruncatedIDs:     res.TruncatedIDs,
		FieldUpdates:     res.FieldUpdates,
	})
	c.ApplyIntents(intents)

	stored, _ := core.ProbeResources("eb-rule")
	var row resource.Resource
	for _, r := range stored {
		if r.ID == proven {
			row = r
		}
	}
	if !bkHasCode(row.Findings, bkNoTargets) {
		t.Errorf("stored row %q carries %v, want %s — the badge counts a finding no row holds", proven, bkCodes(row.Findings), bkNoTargets)
	}

	badge := -1
	for _, in := range intents {
		if pm, ok := in.(runtime.PatchMenu); ok && pm.ResourceType == "eb-rule" {
			badge = pm.Issues
		}
	}
	foldedIssues := 0
	for _, r := range stored {
		if slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return f.Severity == domain.SevBroken }) {
			foldedIssues++
		}
	}
	if badge != 1 || badge != foldedIssues {
		t.Errorf("menu badge = %d, folded rows with a broken finding = %d, want both 1", badge, foldedIssues)
	}

	body := *c.Snapshot().Body.List
	phrase := catalog.Phrase(bkNoTargets)
	if got := statusCellFor(t, body, proven); got != phrase {
		t.Errorf("Status cell = %q, want %q", got, phrase)
	}
	if got := colorFor(t, body, proven); got != "broken" {
		t.Errorf("row colour = %q, want %q — the colour of the row's top finding %s", got, "broken", bkNoTargets)
	}
	if got := colorFor(t, body, healthy); got == "broken" {
		t.Errorf("the healthy rule %q is coloured broken", healthy)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "eb-rule", ResourceID: proven},
	}})
	c.EnsureDetailState(row, "eb-rule")
	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body")
	}
	var attention []string
	inBlock := false
	for _, f := range detail.Fields {
		switch {
		case f.IsSection && strings.HasPrefix(f.Key, "Attention"):
			inBlock = true
		case f.IsSection, inBlock && f.IsSpacer:
			inBlock = false
		case inBlock:
			attention = append(attention, strings.TrimSpace(f.Key+" "+f.Value))
		}
	}
	joined := strings.Join(attention, " | ")
	if !strings.Contains(joined, phrase) {
		t.Errorf("detail Attention lacks the finding %q: %v", phrase, attention)
	}
	if want := domain.NotInspectedPhrase + ": ListTagsForResource"; !strings.Contains(joined, want) {
		t.Errorf("detail Attention lacks the mark %q beside the finding: %v", want, attention)
	}
	phraseLines := 0
	for _, line := range attention {
		if strings.EqualFold(strings.TrimSpace(line), phrase) {
			phraseLines++
		}
		for _, word := range strings.Fields(line) {
			if word == "true" || word == "false" {
				t.Errorf("Attention line %q carries a Go bool literal", line)
			}
		}
	}
	if phraseLines > 1 {
		t.Errorf("the phrase %q is stated %d times in the Attention block: %v", phrase, phraseLines, attention)
	}
}

// ---------------------------------------------------------------------------
// Row 3 — a failed call proves nothing
// ---------------------------------------------------------------------------

// TestEBRuleDeniedTargetCall_RaisesNoNoTargetsFinding: an empty target list
// is what the enricher holds when ListTargetsByRule was refused, and it is not
// an answer. The rule whose call succeeded with zero targets is the healthy
// counterpart: that one is proven.
func TestEBRuleDeniedTargetCall_RaisesNoNoTargetsFinding(t *testing.T) {
	const denied, empty, disabled = "acme-orders-rule", "acme-billing-rule", "acme-archive-rule"
	rules := []resource.Resource{
		bkEBRule(denied, "ENABLED"), bkEBRule(empty, "ENABLED"), bkEBRule(disabled, "DISABLED"),
	}
	fake := &bkEBFake{
		targets: map[string][]eventbridgetypes.Target{empty: {}},
		errs: map[string]error{
			denied:   bkOpErr("EventBridge", "ListTargetsByRule", "AccessDeniedException"),
			disabled: bkOpErr("EventBridge", "ListTargetsByRule", "AccessDeniedException"),
		},
	}

	res, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), &awsclient.ServiceClients{EventBridge: fake}, rules, nil)

	if err == nil {
		t.Error("two refused calls returned no error")
	}
	for _, id := range []string{denied, disabled} {
		if len(res.Findings[id]) != 0 {
			t.Errorf("%s: ListTargetsByRule was refused, yet the rule carries %v", id, bkCodes(res.Findings[id]))
		}
		if got := res.TruncatedIDs[id]; got != "ListTargetsByRule" {
			t.Errorf("%s: TruncatedIDs = %q, want %q", id, got, "ListTargetsByRule")
		}
	}
	if got := bkCodes(res.Findings[empty]); !slices.Equal(got, []domain.FindingCode{bkNoTargets}) {
		t.Errorf("%s: answered with zero targets, carries %v, want [%s]", empty, got, bkNoTargets)
	}
	if _, marked := res.TruncatedIDs[empty]; marked {
		t.Errorf("%s: answered, yet marked not inspected", empty)
	}
}

// TestEBRuleDeniedTargetCall_WritesNoTargetCount: a refused ListTargetsByRule
// counted nothing, so the Targets column stays blank. "0" is the answer of a
// call that succeeded with an empty list, and is only written for one.
func TestEBRuleDeniedTargetCall_WritesNoTargetCount(t *testing.T) {
	const denied, empty = "acme-orders-rule", "acme-billing-rule"
	rules := []resource.Resource{bkEBRule(denied, "ENABLED"), bkEBRule(empty, "ENABLED")}
	fake := &bkEBFake{
		targets: map[string][]eventbridgetypes.Target{empty: {}},
		errs:    map[string]error{denied: bkOpErr("EventBridge", "ListTargetsByRule", "AccessDeniedException")},
	}

	res, _ := awsclient.EnrichEventBridgeRuleTargets(context.Background(), &awsclient.ServiceClients{EventBridge: fake}, rules, nil) //nolint:errcheck // the refused call's aggregate error is not under test

	if got, ok := res.FieldUpdates[denied]["target_count"]; ok && got != "" {
		t.Errorf("%s: target_count = %q after a refused ListTargetsByRule, want blank — the call counted nothing", denied, got)
	}
	if got := res.FieldUpdates[empty]["target_count"]; got != "0" {
		t.Errorf("%s: target_count = %q after ListTargetsByRule answered with no targets, want %q", empty, got, "0")
	}
}

// bkDeniedTransport refuses every AWS call the way an SCP deny does.
type bkDeniedTransport struct{}

func (bkDeniedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const body = `{"__type":"AccessDeniedException","message":"User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized"}`
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
		Header: http.Header{
			"Content-Type":      {"application/x-amz-json-1.1"},
			"X-Amzn-Errortype":  {"AccessDeniedException"},
			"X-Amzn-Requestid":  {"5e0b6c1d-0000-4000-8000-000000000001"},
			"X-Amz-Request-Id":  {"5E0B6C1D00004000"},
			"X-Amzn-Error-Code": {"AccessDenied"},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

// TestWave2DeniedCalls_NoEnricherRaisesAFindingTheDataRefutes runs every
// registered enricher over the demo rows twice: against the demo account, and
// against the same rows with every AWS call refused. A refused call answers
// nothing, so the refused run may only raise findings the demo run also
// raises. A finding it raises that the real data refutes was manufactured
// from the absence of an answer.
func TestWave2DeniedCalls_NoEnricherRaisesAFindingTheDataRefutes(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	live := demo.NewServiceClients()
	denied := awsclient.CreateServiceClients(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "example-secret", ""),
		HTTPClient:  &http.Client{Transport: bkDeniedTransport{}},
	})
	denied.SetIAMPolicies(session.NewPolicyStore())
	denied.SetIdentityStore(session.NewIdentityStore())
	denied.SetRuleSets(session.NewRuleSetStore())

	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		if len(rows) == 0 {
			continue
		}
		t.Run(w.ShortName, func(t *testing.T) {
			truth, _ := w.Enricher.Fn(context.Background(), live, rows, cache)     //nolint:errcheck // judged by its findings
			refused, _ := w.Enricher.Fn(context.Background(), denied, rows, cache) //nolint:errcheck // judged by its findings
			for _, r := range rows {
				for _, f := range refused.Findings[r.ID] {
					if bkHasCode(truth.Findings[r.ID], f.Code) {
						continue
					}
					t.Errorf("%s/%s: every call was refused, yet it raises %s (%q), which the demo account's answers do not",
						w.ShortName, r.ID, f.Code, f.Phrase)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 4 — a walk that failed names the call
// ---------------------------------------------------------------------------

// bkEC2Fake serves DescribeInstanceStatus and DescribeSnapshots from ordered
// pages; a nil page with an error is a refused call.
type bkEC2Fake struct {
	awsclient.EC2API
	statusPages []*ec2sdk.DescribeInstanceStatusOutput
	statusErrAt int
	statusErr   error
	statusCalls int
	snapErr     error
}

func (f *bkEC2Fake) DescribeInstanceStatus(_ context.Context, _ *ec2sdk.DescribeInstanceStatusInput, _ ...func(*ec2sdk.Options)) (*ec2sdk.DescribeInstanceStatusOutput, error) {
	i := f.statusCalls
	f.statusCalls++
	if f.statusErr != nil && i == f.statusErrAt {
		return nil, f.statusErr
	}
	if i >= len(f.statusPages) {
		return &ec2sdk.DescribeInstanceStatusOutput{}, nil
	}
	return f.statusPages[i], nil
}

func (f *bkEC2Fake) DescribeSnapshots(_ context.Context, _ *ec2sdk.DescribeSnapshotsInput, _ ...func(*ec2sdk.Options)) (*ec2sdk.DescribeSnapshotsOutput, error) {
	if f.snapErr != nil {
		return nil, f.snapErr
	}
	return &ec2sdk.DescribeSnapshotsOutput{}, nil
}

func bkInstanceStatus(id string) ec2types.InstanceStatus {
	return ec2types.InstanceStatus{
		InstanceId:     aws.String(id),
		InstanceState:  &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		InstanceStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
		SystemStatus:   &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
	}
}

func bkInstances(ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{ID: id, Name: id, Type: "ec2",
			Fields: map[string]string{"instance_id": id, "state": "running"}})
	}
	return out
}

// TestEBSSnapPublicWalk_RefusedCallIsNotReportedAsTheCap reproduces the
// reviewer's scenario: DescribeSnapshots(RestorableByUserIds=all) is refused on
// its first page. Every snapshot is uninspected because AWS said no, and the
// row must name that call — "stopped at the inspection cap" tells the operator
// a9s chose to stop, and the detail view retries a call that will be refused
// again.
func TestEBSSnapPublicWalk_RefusedCallIsNotReportedAsTheCap(t *testing.T) {
	enricher, ok := awsclient.Wave2EnricherFor("ebs-snap")
	if !ok {
		t.Fatal("ebs-snap has no Wave 2 enricher")
	}
	snaps := []resource.Resource{
		{ID: "snap-0a1b2c3d4e5f60001", Name: "snap-0a1b2c3d4e5f60001", Type: "ebs-snap",
			RawStruct: ec2types.Snapshot{SnapshotId: aws.String("snap-0a1b2c3d4e5f60001"), VolumeId: aws.String("vol-0a1b2c3d4e5f60001")}},
		{ID: "snap-0a1b2c3d4e5f60002", Name: "snap-0a1b2c3d4e5f60002", Type: "ebs-snap",
			RawStruct: ec2types.Snapshot{SnapshotId: aws.String("snap-0a1b2c3d4e5f60002"), VolumeId: aws.String("vol-0a1b2c3d4e5f60002")}},
	}
	fake := &bkEC2Fake{snapErr: bkOpErr("EC2", "DescribeSnapshots", "UnauthorizedOperation")}

	res, err := enricher.Fn(context.Background(), &awsclient.ServiceClients{EC2: fake}, snaps, nil)

	if err == nil {
		t.Error("a refused DescribeSnapshots returned no error")
	}
	for _, s := range snaps {
		if got := res.TruncatedIDs[s.ID]; got != "DescribeSnapshots" {
			t.Errorf("%s: TruncatedIDs = %q, want %q — a refused call is reported as a9s's own bound", s.ID, got, "DescribeSnapshots")
		}
	}
}

// TestEC2StatusWalk_ThrottledPageNamesTheCall: page 1 answers for one
// instance, page 2 is throttled. The instance page 1 named keeps its answer;
// the one it did not is uninspected because of DescribeInstanceStatus.
func TestEC2StatusWalk_ThrottledPageNamesTheCall(t *testing.T) {
	const seen, unseen = "i-0aaa111111111111a", "i-0bbb222222222222b"
	fake := &bkEC2Fake{
		statusPages: []*ec2sdk.DescribeInstanceStatusOutput{{
			InstanceStatuses: []ec2types.InstanceStatus{bkInstanceStatus(seen)},
			NextToken:        aws.String("page-2"),
		}},
		statusErrAt: 1,
		statusErr:   bkOpErr("EC2", "DescribeInstanceStatus", "RequestLimitExceeded"),
	}

	res, _ := awsclient.EnrichEC2InstanceStatus(context.Background(), &awsclient.ServiceClients{EC2: fake}, bkInstances(seen, unseen), nil) //nolint:errcheck // the throttled page's error is not under test

	if got := res.TruncatedIDs[unseen]; got != "DescribeInstanceStatus" {
		t.Errorf("%s: TruncatedIDs = %q, want %q — a throttled walk reads as \"stopped at the inspection cap\"", unseen, got, "DescribeInstanceStatus")
	}
	if mark, marked := res.TruncatedIDs[seen]; marked {
		t.Errorf("%s: page 1 answered for it, yet it is marked %q", seen, mark)
	}
}

// TestEC2StatusWalk_CapStillReadsAsTheCap is the other half: a walk that
// stopped because a9s reached its page bound, with nothing refused, is the
// cap.
func TestEC2StatusWalk_CapStillReadsAsTheCap(t *testing.T) {
	pages := make([]*ec2sdk.DescribeInstanceStatusOutput, awsclient.EnrichmentCap+2)
	for i := range pages {
		pages[i] = &ec2sdk.DescribeInstanceStatusOutput{
			InstanceStatuses: []ec2types.InstanceStatus{bkInstanceStatus(fmt.Sprintf("i-page%010d", i))},
			NextToken:        aws.String(fmt.Sprintf("tok-%d", i+1)),
		}
	}
	unseen := fmt.Sprintf("i-page%010d", awsclient.EnrichmentCap+1)
	fake := &bkEC2Fake{statusPages: pages}

	res, err := awsclient.EnrichEC2InstanceStatus(context.Background(), &awsclient.ServiceClients{EC2: fake}, bkInstances("i-page0000000000", unseen), nil)
	if err != nil {
		t.Fatalf("EnrichEC2InstanceStatus: %v", err)
	}
	if got := res.TruncatedIDs[unseen]; got != awsclient.CheckCap {
		t.Errorf("%s: TruncatedIDs = %q, want %q", unseen, got, awsclient.CheckCap)
	}
}

type bkBackupFake struct {
	awsclient.BackupAPI
	calls int
}

func (f *bkBackupFake) ListBackupJobs(_ context.Context, _ *backupsdk.ListBackupJobsInput, _ ...func(*backupsdk.Options)) (*backupsdk.ListBackupJobsOutput, error) {
	f.calls++
	if f.calls > 1 {
		return nil, bkOpErr("Backup", "ListBackupJobs", "AccessDeniedException")
	}
	return &backupsdk.ListBackupJobsOutput{
		BackupJobs: []backuptypes.BackupJob{makeBkJob("acme-daily-plan", backuptypes.BackupJobStateCompleted)},
		NextToken:  aws.String("page-2"),
	}, nil
}

func makeBkJob(planID string, state backuptypes.BackupJobState) backuptypes.BackupJob {
	return backuptypes.BackupJob{
		BackupJobId:  aws.String("job-" + planID),
		State:        state,
		CreationDate: aws.Time(time.Now().Add(-2 * time.Hour)),
		CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String(planID)},
		ResourceArn:  aws.String("arn:aws:rds:us-east-1:123456789012:db:acme-orders"),
	}
}

// TestBackupJobWalk_RefusedPageNamesTheCall covers the walk whose rows are
// only answered by a complete walk: when page 2 is refused, every plan —
// including the one page 1 named — is uninspected because of ListBackupJobs.
func TestBackupJobWalk_RefusedPageNamesTheCall(t *testing.T) {
	plans := []resource.Resource{
		{ID: "acme-daily-plan", Name: "acme-daily", Type: "backup", Fields: map[string]string{"plan_id": "acme-daily-plan"}},
		{ID: "acme-weekly-plan", Name: "acme-weekly", Type: "backup", Fields: map[string]string{"plan_id": "acme-weekly-plan"}},
	}

	res, _ := awsclient.EnrichBackupJobs(context.Background(), &awsclient.ServiceClients{Backup: &bkBackupFake{}}, plans, nil) //nolint:errcheck // the refused page's error is not under test

	for _, p := range plans {
		if got := res.TruncatedIDs[p.ID]; got != "ListBackupJobs" {
			t.Errorf("%s: TruncatedIDs = %q, want %q", p.ID, got, "ListBackupJobs")
		}
	}
}

// ---------------------------------------------------------------------------
// Row 5 — the cap is spent on rows the check applies to
// ---------------------------------------------------------------------------

// bkAssertIneligibleUntouched: a row the check can never apply to is neither
// asked about nor marked — a mark there reads "unknown" for a question that
// has no answer to give.
func bkAssertIneligibleUntouched(t *testing.T, kind string, rows []resource.Resource, marks map[string]string, asked map[string]bool) {
	t.Helper()
	var marked, spent []string
	for _, r := range rows {
		if m, ok := marks[r.ID]; ok {
			marked = append(marked, r.ID+"="+m)
		}
		if asked[r.ID] {
			spent = append(spent, r.ID)
		}
	}
	if len(marked) > 0 {
		t.Errorf("%d %s row(s) are marked not inspected for a check that never applies to them: %v", len(marked), kind, marked)
	}
	if len(spent) > 0 {
		t.Errorf("%d call(s) of the capped check were spent on %s rows: %v", len(spent), kind, spent)
	}
}

type bkSESFake struct {
	awsclient.SESv2API
	asked map[string]bool
	mu    sync.Mutex
}

func newBkSESFake() *bkSESFake { return &bkSESFake{asked: map[string]bool{}} }

func (f *bkSESFake) GetAccount(_ context.Context, _ *sesv2.GetAccountInput, _ ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error) {
	return &sesv2.GetAccountOutput{
		EnforcementStatus: aws.String("HEALTHY"),
		SendQuota:         &sesv2types.SendQuota{Max24HourSend: 50000, SentLast24Hours: 1200, MaxSendRate: 14},
	}, nil
}

func (f *bkSESFake) GetEmailIdentity(_ context.Context, in *sesv2.GetEmailIdentityInput, _ ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error) {
	id := aws.ToString(in.EmailIdentity)
	f.mu.Lock()
	f.asked[id] = true
	f.mu.Unlock()
	if strings.Contains(id, "@") {
		return &sesv2.GetEmailIdentityOutput{IdentityType: sesv2types.IdentityTypeEmailAddress, VerifiedForSendingStatus: true}, nil
	}
	return &sesv2.GetEmailIdentityOutput{
		IdentityType:             sesv2types.IdentityTypeDomain,
		VerifiedForSendingStatus: true,
		DkimAttributes:           &sesv2types.DkimAttributes{SigningEnabled: false, Status: sesv2types.DkimStatusNotStarted},
	}, nil
}

func bkSESIdentity(id string) resource.Resource {
	kind := "domain"
	if strings.Contains(id, "@") {
		kind = "email address"
	}
	return resource.Resource{ID: id, Name: id, Type: "ses",
		Fields: map[string]string{"identity": id, "identity_type": kind, "verification_status": "success"}}
}

// TestSESDKIMCap_AddressesDoNotSpendTheCapOnDomains: only a domain can sign
// with DKIM. With more addresses than the cap listed ahead of the domains,
// every domain is still checked, and no address is asked about or marked.
func TestSESDKIMCap_AddressesDoNotSpendTheCapOnDomains(t *testing.T) {
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 5 {
		rows = append(rows, bkSESIdentity(fmt.Sprintf("alerts-%03d@acme.example", i)))
	}
	domains := []string{"acme.example", "mail.acme.example", "billing.acme.example"}
	for _, d := range domains {
		rows = append(rows, bkSESIdentity(d))
	}
	fake := newBkSESFake()

	res, err := awsclient.EnrichSESAccount(context.Background(), &awsclient.ServiceClients{SESv2: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichSESAccount: %v", err)
	}
	for _, d := range domains {
		if !bkHasCode(res.Findings[d], "ses.dkim-off") {
			t.Errorf("%s: carries %v, want ses.dkim-off — the domain was never checked", d, bkCodes(res.Findings[d]))
		}
		if mark, marked := res.TruncatedIDs[d]; marked {
			t.Errorf("%s: marked %q — the cap was spent on addresses", d, mark)
		}
	}
	bkAssertIneligibleUntouched(t, "email address", rows[:awsclient.EnrichmentCap+5], res.TruncatedIDs, fake.asked)
}

// TestSESDKIMCap_StillBitesOnEligibleRows: filtering is not dropping the cap.
// More domains than the cap leaves the domains past it marked.
func TestSESDKIMCap_StillBitesOnEligibleRows(t *testing.T) {
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 1 {
		rows = append(rows, bkSESIdentity(fmt.Sprintf("tenant-%03d.acme.example", i)))
	}
	res, _ := awsclient.EnrichSESAccount(context.Background(), &awsclient.ServiceClients{SESv2: newBkSESFake()}, rows, nil) //nolint:errcheck // judged by its marks
	last := rows[awsclient.EnrichmentCap].ID
	if got := res.TruncatedIDs[last]; got != awsclient.CheckCap {
		t.Errorf("%s: TruncatedIDs = %q, want %q", last, got, awsclient.CheckCap)
	}
}

type bkLogsFake struct {
	awsclient.CWLogsAPI
	mu    sync.Mutex
	asked map[string]bool
}

func (f *bkLogsFake) DescribeMetricFilters(_ context.Context, in *cwlogssvc.DescribeMetricFiltersInput, _ ...func(*cwlogssvc.Options)) (*cwlogssvc.DescribeMetricFiltersOutput, error) {
	f.mu.Lock()
	f.asked[aws.ToString(in.LogGroupName)] = true
	f.mu.Unlock()
	return &cwlogssvc.DescribeMetricFiltersOutput{MetricFilters: []cwlogstypes.MetricFilter{}}, nil
}

func (f *bkLogsFake) DescribeLogStreams(_ context.Context, _ *cwlogssvc.DescribeLogStreamsInput, _ ...func(*cwlogssvc.Options)) (*cwlogssvc.DescribeLogStreamsOutput, error) {
	return &cwlogssvc.DescribeLogStreamsOutput{}, nil
}

func bkLogGroup(name string) resource.Resource {
	return resource.Resource{ID: name, Name: name, Type: "logs",
		Fields: map[string]string{"log_group_name": name, "retention": "30 days", "stored_bytes": "1048576"}}
}

// TestLogsMetricFilterCap_NonAuditGroupsAreNeitherCheckedNorMarked: the
// metric-filter check applies to audit log groups only. Lambda groups listed
// ahead of them do not use up the cap, and are not marked for a check that
// never applied to them.
func TestLogsMetricFilterCap_NonAuditGroupsAreNeitherCheckedNorMarked(t *testing.T) {
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 5 {
		rows = append(rows, bkLogGroup(fmt.Sprintf("/aws/lambda/acme-fn-%03d", i)))
	}
	audit := []string{"/aws/cloudtrail/acme-org-trail", "/aws/cloudtrail/acme-data-trail"}
	for _, g := range audit {
		rows = append(rows, bkLogGroup(g))
	}
	fake := &bkLogsFake{asked: map[string]bool{}}

	res, err := awsclient.EnrichLogsMetricFilters(context.Background(), &awsclient.ServiceClients{CloudWatchLogs: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichLogsMetricFilters: %v", err)
	}
	for _, g := range audit {
		if !bkHasCode(res.Findings[g], "logs.missing-metric-filters") {
			t.Errorf("%s: carries %v, want logs.missing-metric-filters — the audit group was never checked", g, bkCodes(res.Findings[g]))
		}
		if mark, marked := res.TruncatedIDs[g]; marked {
			t.Errorf("%s: marked %q — the cap was spent on non-audit groups", g, mark)
		}
	}
	bkAssertIneligibleUntouched(t, "non-audit log group", rows[:awsclient.EnrichmentCap+5], res.TruncatedIDs, fake.asked)
}

type bkRDSFake struct {
	awsclient.RDSAPI
	mu    sync.Mutex
	asked map[string]bool
}

func (f *bkRDSFake) DescribeDBSnapshotAttributes(_ context.Context, in *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	id := aws.ToString(in.DBSnapshotIdentifier)
	f.mu.Lock()
	f.asked[id] = true
	f.mu.Unlock()
	return &rds.DescribeDBSnapshotAttributesOutput{DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
		DBSnapshotIdentifier: aws.String(id),
		DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{{AttributeName: aws.String("restore"), AttributeValues: []string{"all"}}},
	}}, nil
}

func bkDBISnap(id, kind string) resource.Resource {
	return resource.Resource{ID: id, Name: id, Type: "dbi-snap",
		Fields: map[string]string{"snapshot_id": id, "db_instance": "acme-orders-db", "snapshot_type": kind, "status": "available"},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String(id),
			DBInstanceIdentifier: aws.String("acme-orders-db"),
			SnapshotType:         aws.String(kind),
			Status:               aws.String("available"),
			SnapshotCreateTime:   aws.Time(time.Now().Add(-30 * time.Hour)),
		}}
}

// TestDBISnapShareCap_AutomatedSnapshotsDoNotSpendTheCap: only a manual
// snapshot can be shared. Automated snapshots listed ahead of the manual ones
// do not use up the attribute reads, and the publicly shared manual snapshots
// are reported.
func TestDBISnapShareCap_AutomatedSnapshotsDoNotSpendTheCap(t *testing.T) {
	enricher, ok := awsclient.Wave2EnricherFor("dbi-snap")
	if !ok {
		t.Fatal("dbi-snap has no Wave 2 enricher")
	}
	var rows []resource.Resource
	for i := range awsclient.EnrichmentCap + 5 {
		rows = append(rows, bkDBISnap(fmt.Sprintf("rds:acme-orders-db-2026-08-%03d", i), "automated"))
	}
	manual := []string{"acme-orders-db-pre-migration", "acme-orders-db-export"}
	for _, id := range manual {
		rows = append(rows, bkDBISnap(id, "manual"))
	}
	fake := &bkRDSFake{asked: map[string]bool{}}

	res, _ := enricher.Fn(context.Background(), &awsclient.ServiceClients{RDS: fake}, rows, nil) //nolint:errcheck // judged by its findings and marks
	td := resource.FindResourceType("dbi-snap")
	if td == nil {
		t.Fatal("dbi-snap is not registered")
	}
	folded := make(map[string]resource.Resource, len(manual))
	for _, r := range rows {
		if slices.Contains(manual, r.ID) {
			runtime.ApplyWave2ToRow(&r, *td, res.Findings, res.AttentionDetails)
			folded[r.ID] = r
		}
	}
	for _, id := range manual {
		if fs := folded[id].Findings; !bkHasCode(fs, "dbi-snap.public") {
			t.Errorf("%s: the folded row carries %v, want dbi-snap.public — the manual snapshot was never checked", id, bkCodes(fs))
		}
		if mark, marked := res.TruncatedIDs[id]; marked {
			t.Errorf("%s: marked %q — the cap was spent on automated snapshots", id, mark)
		}
	}
	bkAssertIneligibleUntouched(t, "automated snapshot", rows[:awsclient.EnrichmentCap+5], res.TruncatedIDs, fake.asked)
}
