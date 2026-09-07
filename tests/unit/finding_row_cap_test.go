package unit

// finding_row_cap_test.go — one cap for every finding's supporting rows,
// applied where every emitter already hands rows over.
//
// A finding's supporting rows are unbounded in the data: a target group can
// report 400 unhealthy targets, a node group can report dozens of health
// issues. Rendering all of them pushes every other finding off the Attention
// section, so a reader loses the rest of the resource's posture to one noisy
// condition. The bound therefore belongs to the two sinks every emitter
// already routes through — awsclient.FindingRowCap beside them — and not to
// any individual builder, because a builder that caps only bounds itself and
// every unvisited site stays unbounded.
//
// The shape pinned here: the first FindingRowCap rows survive verbatim, then
// exactly one closing row carrying "… +K more", wearing the last kept row's
// label and tier so it reads as one more line of the same list rather than a
// foreign annotation.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// capOverflowValue is the closing row's value for K hidden rows. Built from
// the same format the sink must use, so a change to the wording is one edit.
func capOverflowValue(hidden int) string {
	return fmt.Sprintf("… +%d more", hidden)
}

// soleAttentionRows returns the rows of the single (resourceID, code) entry a
// one-condition enricher result must carry, failing when the result does not
// hold exactly one finding with rows for id.
func soleAttentionRows(t *testing.T, result awsclient.IssueEnricherResult, id string) []domain.DetailRow {
	t.Helper()
	byCode, ok := result.AttentionDetails[id]
	if !ok {
		t.Fatalf("no AttentionDetails entry for %q; AttentionDetails=%+v", id, result.AttentionDetails)
	}
	if len(byCode) != 1 {
		t.Fatalf("expected exactly one finding code with rows for %q, got %d: %+v", id, len(byCode), byCode)
	}
	for _, ad := range byCode {
		return ad.Rows
	}
	return nil
}

// assertCappedRows pins the whole rendered shape of a capped row list: cap
// rows of real content, then one unlabelled overflow row that inherits the
// last kept row's tier.
func assertCappedRows(t *testing.T, rows []domain.DetailRow, totalItems int) {
	t.Helper()
	capN := awsclient.FindingRowCap
	if totalItems <= capN {
		t.Fatalf("fixture defect: %d items does not exceed the cap of %d, so this assertion cannot bite", totalItems, capN)
	}
	if len(rows) != capN+1 {
		t.Fatalf("capped finding rendered %d rows, want %d (%d kept + 1 overflow); rows=%+v",
			len(rows), capN+1, capN, rows)
	}
	last := rows[capN-1]
	overflow := rows[capN]
	if got, want := overflow.Value, capOverflowValue(totalItems-capN); got != want {
		t.Errorf("overflow row Value = %q, want %q", got, want)
	}
	// Inverted for the cap batch's spec row 9. This asserted that the closing
	// row wore the last kept row's label, which rendered the backup detail as
	// "State  … +5 more" — a failed job whose state is that text. The closing
	// row is the count of what is not shown, not another member of the list,
	// so it carries no label. Do not restore the inherited form.
	if overflow.Label != "" {
		t.Errorf("overflow row Label = %q, want empty — a labelled closing row reads as one more %q row", overflow.Label, last.Label)
	}
	if got, want := overflow.Tier, last.Tier; got != want {
		t.Errorf("overflow row Tier = %q, want %q (the last kept row's tier)", got, want)
	}
	for i, r := range rows[:capN] {
		if strings.Contains(r.Value, "more") {
			t.Errorf("kept row %d (%q) reads as an overflow row; only the closing row may", i, r.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// Wave 2 sink — setWave2Finding, driven through the real tg enricher.
// ---------------------------------------------------------------------------

// tgCapResources builds one target group row shaped the way the tg fetcher
// leaves it: ID is the name, the ARN lives in Fields.
func tgCapResources(name, arn string) []resource.Resource {
	return []resource.Resource{{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"target_group_arn": arn},
	}}
}

// tgUnhealthyOutput is a DescribeTargetHealth answer where every one of n
// targets reports State=unhealthy with a real failure reason — the shape AWS
// returns for a target group behind a crashed deployment.
func tgUnhealthyOutput(n int) *elbv2.DescribeTargetHealthOutput {
	descs := make([]elbtypes.TargetHealthDescription, 0, n)
	for i := range n {
		descs = append(descs, elbtypes.TargetHealthDescription{
			Target: &elbtypes.TargetDescription{
				Id:   aws.String(fmt.Sprintf("i-0a1b2c3d4e5f%04d", i)),
				Port: aws.Int32(8080),
			},
			TargetHealth: &elbtypes.TargetHealth{
				State:  elbtypes.TargetHealthStateEnumUnhealthy,
				Reason: elbtypes.TargetHealthReasonEnumResponseCodeMismatch,
			},
		})
	}
	return &elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: descs}
}

func TestFindingRowCap_TargetGroupUnhealthyRowsCappedAtTheSink(t *testing.T) {
	const arn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/web-tg/73e2d6bc24d8a067"
	hidden := 7
	targets := awsclient.FindingRowCap + hidden

	fake := &tgHealthFake{outputs: map[string]*elbv2.DescribeTargetHealthOutput{
		arn: tgUnhealthyOutput(targets),
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake}, tgCapResources("web-tg", arn), nil)
	if err != nil {
		t.Fatalf("EnrichTargetGroupHealth: unexpected error: %v", err)
	}

	rows := soleAttentionRows(t, result, "web-tg")
	assertCappedRows(t, rows, targets)

	// The kept rows are the real per-target rows, unchanged by the cap: the
	// cap hides rows, it never rewrites the ones it keeps.
	if got, want := rows[0].Label, "Unhealthy target"; got != want {
		t.Errorf("first kept row Label = %q, want %q", got, want)
	}
	if got := rows[0].Value; !strings.Contains(got, "i-0a1b2c3d4e5f0000:8080") {
		t.Errorf("first kept row Value = %q, want it to name the first target i-0a1b2c3d4e5f0000:8080", got)
	}
	if got, want := rows[0].Tier, "!"; got != want {
		t.Errorf("first kept row Tier = %q, want %q (every target unhealthy is Broken)", got, want)
	}
}

// TestFindingRowCap_TargetGroupAtExactlyTheCapHasNoOverflowRow is the negative
// half: the closing row appears only when something is actually hidden.
func TestFindingRowCap_TargetGroupAtExactlyTheCapHasNoOverflowRow(t *testing.T) {
	const arn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/exact-tg/9f1c8e0b6a2d4713"
	targets := awsclient.FindingRowCap

	fake := &tgHealthFake{outputs: map[string]*elbv2.DescribeTargetHealthOutput{
		arn: tgUnhealthyOutput(targets),
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake}, tgCapResources("exact-tg", arn), nil)
	if err != nil {
		t.Fatalf("EnrichTargetGroupHealth: unexpected error: %v", err)
	}

	rows := soleAttentionRows(t, result, "exact-tg")
	if len(rows) != targets {
		t.Fatalf("target group with exactly %d unhealthy targets rendered %d rows, want %d and no overflow row; rows=%+v",
			targets, len(rows), targets, rows)
	}
	for i, r := range rows {
		if strings.Contains(r.Value, "more") {
			t.Errorf("row %d (%q) is an overflow row, but nothing was hidden", i, r.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// Wave 1 sink — addWave1Rows, driven through the real ng fetcher.
// ---------------------------------------------------------------------------

// ngIssueCodes cycles AWS's real NodegroupIssueCode values to build n issues.
// AWS repeats a code across issues that hit different instances, so a cycling
// fixture is the shape DescribeNodegroup actually returns.
func ngIssueCodes(n int) []ekstypes.Issue {
	codes := []ekstypes.NodegroupIssueCode{
		ekstypes.NodegroupIssueCodeAsgInstanceLaunchFailures,
		ekstypes.NodegroupIssueCodeInstanceLimitExceeded,
		ekstypes.NodegroupIssueCodeInsufficientFreeAddresses,
		ekstypes.NodegroupIssueCodeIamInstanceProfileNotFound,
		ekstypes.NodegroupIssueCodeEc2SubnetNotFound,
	}
	issues := make([]ekstypes.Issue, 0, n)
	for i := range n {
		issues = append(issues, ekstypes.Issue{
			Code:        codes[i%len(codes)],
			Message:     aws.String(fmt.Sprintf("node %d could not join the cluster", i)),
			ResourceIds: []string{fmt.Sprintf("i-0f9e8d7c6b5a%04d", i)},
		})
	}
	return issues
}

func ngCapDescribeOutput(cluster, name string, issues int) *eks.DescribeNodegroupOutput {
	return &eks.DescribeNodegroupOutput{Nodegroup: &ekstypes.Nodegroup{
		NodegroupName: aws.String(name),
		ClusterName:   aws.String(cluster),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"m5.large"},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{DesiredSize: aws.Int32(3)},
		Health:        &ekstypes.NodegroupHealth{Issues: ngIssueCodes(issues)},
	}}
}

// TestFindingRowCap_NodeGroupHealthIssueRowsCappedAtTheSink drives the
// registered "ng" fetcher, which is the only path that builds a node group's
// health-issue rows, so the cap is proven at the Wave 1 sink and not at a
// helper the fetcher happens to call.
func TestFindingRowCap_NodeGroupHealthIssueRowsCappedAtTheSink(t *testing.T) {
	const cluster = "prod-cluster"
	hidden := 3
	over := awsclient.FindingRowCap + hidden
	exact := awsclient.FindingRowCap

	clientsFake := &ngPaginatedFullFake{
		&mockEKSListClustersPaginatedClient{outputs: []*eks.ListClustersOutput{
			{Clusters: []string{cluster}},
		}},
		&mockEKSListNodegroupsPaginatedClient{outputs: map[string][]*eks.ListNodegroupsOutput{
			cluster: {{Nodegroups: []string{"ng-over", "ng-exact"}}},
		}},
		&mockEKSDescribeNodegroupPaginatedClient{nodegroups: map[string]*eks.DescribeNodegroupOutput{
			cluster + "/ng-over":  ngCapDescribeOutput(cluster, "ng-over", over),
			cluster + "/ng-exact": ngCapDescribeOutput(cluster, "ng-exact", exact),
		}},
	}

	pf := resource.GetPaginatedFetcher("ng")
	if pf == nil {
		t.Fatalf("no paginated fetcher registered for \"ng\"")
	}
	res, err := pf(context.Background(), &awsclient.ServiceClients{EKS: clientsFake}, "")
	if err != nil {
		t.Fatalf("ng fetcher: unexpected error: %v", err)
	}

	byID := make(map[string]resource.Resource, len(res.Resources))
	for _, r := range res.Resources {
		byID[r.ID] = r
	}

	// Two subtests over one fetch: the over-cap half aborts on its first
	// mismatch, and the negative half has to keep running when it does.
	t.Run("over_the_cap", func(t *testing.T) {
		overRow, ok := byID["ng-over"]
		if !ok {
			t.Fatalf("ng-over missing from the fetch result; got %+v", byID)
		}
		rows := soleWave1Rows(t, overRow)
		assertCappedRows(t, rows, over)
		if got, want := rows[0].Label, "Issue"; got != want {
			t.Errorf("first kept row Label = %q, want %q", got, want)
		}
		if got, want := rows[0].Tier, "~"; got != want {
			t.Errorf("first kept row Tier = %q, want %q (an ACTIVE node group with health issues is a Warning)", got, want)
		}
	})

	t.Run("exactly_the_cap", func(t *testing.T) {
		exactRow, ok := byID["ng-exact"]
		if !ok {
			t.Fatalf("ng-exact missing from the fetch result; got %+v", byID)
		}
		exactRows := soleWave1Rows(t, exactRow)
		if len(exactRows) != exact {
			t.Fatalf("node group with exactly %d health issues rendered %d rows, want %d and no overflow row; rows=%+v",
				exact, len(exactRows), exact, exactRows)
		}
		for i, r := range exactRows {
			if strings.Contains(r.Value, "more") {
				t.Errorf("row %d (%q) is an overflow row, but nothing was hidden", i, r.Value)
			}
		}
	})
}

// soleWave1Rows returns the rows of the single AttentionDetails entry a
// one-condition Wave 1 resource must carry.
func soleWave1Rows(t *testing.T, r resource.Resource) []domain.DetailRow {
	t.Helper()
	if len(r.AttentionDetails) != 1 {
		t.Fatalf("%s: expected exactly one AttentionDetails entry, got %d: %+v",
			r.ID, len(r.AttentionDetails), r.AttentionDetails)
	}
	for _, ad := range r.AttentionDetails {
		return ad.Rows
	}
	return nil
}

// ---------------------------------------------------------------------------
// privEscComboRows loses its private cap and inherits the shared one.
// ---------------------------------------------------------------------------

// privEscCapPolicyDocument allows every action of several services outright,
// which matches dozens of Prowler privilege-escalation combinations at once
// without being an admin policy (no Action "*" on Resource "*"), so the
// enricher takes the priv-esc branch rather than the admin-star one.
func privEscCapPolicyDocument() string {
	doc := map[string]any{
		"Version": "2012-10-17",
		"Statement": []any{map[string]any{
			"Effect": "Allow",
			"Action": []string{
				"iam:*", "sts:*", "lambda:*", "glue:*", "ec2:*",
				"cloudformation:*", "dynamodb:*", "ssm:*", "codebuild:*",
				"datapipeline:*", "sagemaker:*", "cloudtrail:*",
			},
			"Resource": "*",
		}},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestFindingRowCap_PrivEscCombosUseTheSharedCap(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:policy/escalation-prone"
	docJSON := privEscCapPolicyDocument()

	parsed, err := iampolicy.Parse(docJSON)
	if err != nil {
		t.Fatalf("fixture self-check: iampolicy.Parse: %v", err)
	}
	combos := parsed.PrivilegeEscalation()
	if len(combos) <= awsclient.FindingRowCap {
		t.Fatalf("fixture defect: policy matches only %d privilege-escalation combos, which does not exceed the cap of %d",
			len(combos), awsclient.FindingRowCap)
	}

	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			arn: {Policy: &iamtypes.Policy{Arn: aws.String(arn), DefaultVersionId: aws.String("v3")}},
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			arn: {PolicyVersion: &iamtypes.PolicyVersion{
				VersionId: aws.String("v3"),
				Document:  aws.String(pathEncodeDoc(docJSON)),
			}},
		},
	}

	resources := []resource.Resource{{
		ID:        arn,
		Name:      "escalation-prone",
		Fields:    map[string]string{"attachment_count": "2"},
		RawStruct: iamtypes.Policy{Arn: aws.String(arn), PolicyName: aws.String("escalation-prone")},
	}}

	result, err := awsclient.EnrichIAMPolicy(context.Background(),
		&awsclient.ServiceClients{IAM: fake}, resources, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: unexpected error: %v", err)
	}

	rows := soleAttentionRows(t, result, arn)
	assertCappedRows(t, rows, len(combos))
	if got, want := rows[0].Label, "Combo"; got != want {
		t.Errorf("first kept row Label = %q, want %q", got, want)
	}
	// PrivilegeEscalation returns its combos sorted, and the builder must hand
	// all of them over: the first kept row is the first combo, not whatever
	// survived a builder-side truncation with its own ordering.
	if got, want := rows[0].Value, combos[0]; got != want {
		t.Errorf("first kept row Value = %q, want %q", got, want)
	}
}

// TestFindingRowCap_PrivEscComboBuilderHasNoCapOfItsOwn pins the deletion of
// the one site that capped on its own. Two rules for one thing is how the
// shapes drift apart: the builder must hand every combo to the sink and let
// the sink decide. The constant is package-private, so the pin reads the
// source — the assertion the compiler would make if it were exported.
func TestFindingRowCap_PrivEscComboBuilderHasNoCapOfItsOwn(t *testing.T) {
	const src = "../../core/aws/iam_policy_issue_enrichment.go"
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	if strings.Contains(string(b), "privEscComboRowCap") {
		t.Errorf("%s still declares or uses privEscComboRowCap; the row cap now lives once beside the sinks as awsclient.FindingRowCap (%d)",
			src, awsclient.FindingRowCap)
	}
}

// ---------------------------------------------------------------------------
// The contract doc states the bound an operator will hit.
// ---------------------------------------------------------------------------

// TestFindingRowCap_StatedInAttentionSignalsS5 keeps the S5 mechanism cell
// honest: an operator who sees "… +3 more" and no way to reach the rest is
// owed the rule in the contract that governs that surface. The number is
// pinned from the constant, so raising the cap without touching the sentence
// is the failure.
func TestFindingRowCap_StatedInAttentionSignalsS5(t *testing.T) {
	const doc = "../../docs/attention-signals.md"
	b, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("reading %s: %v", doc, err)
	}
	var s5 string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| S5 ") {
			s5 = line
			break
		}
	}
	if s5 == "" {
		t.Fatalf("%s has no S5 row in the visualization-surfaces table", doc)
	}
	num := fmt.Sprintf("%d", awsclient.FindingRowCap)
	if !strings.Contains(s5, num) || !strings.Contains(s5, "more") {
		t.Errorf("S5 mechanism cell does not state the supporting-row cap of %s and the \"+K more\" closing row.\nS5 cell: %s",
			num, s5)
	}
}

// ---------------------------------------------------------------------------
// Accumulation: a code emitted once per item still shows one closing row.
// ---------------------------------------------------------------------------

// capKeyRows returns the rows of the entry whose first row is an access key,
// which is the "key too old" finding's own entry. The user also carries a
// two-active-keys finding with rows of its own.
func capKeyRows(t *testing.T, result awsclient.IssueEnricherResult, user string) []domain.DetailRow {
	t.Helper()
	for _, ad := range result.AttentionDetails[user] {
		if len(ad.Rows) > 0 && ad.Rows[0].Label == "Access key" {
			return ad.Rows
		}
	}
	t.Fatalf("no finding with \"Access key\" rows for %q; AttentionDetails=%+v", user, result.AttentionDetails[user])
	return nil
}

func capEnrichUser(t *testing.T, user string, keys int) awsclient.IssueEnricherResult {
	t.Helper()
	// Every key is old enough for the "key too old" condition, which emits
	// one row per key through a separate call for the same finding code —
	// the accumulating shape the cap has to survive.
	created := time.Now().Add(-400 * 24 * time.Hour)
	keyMeta := make([]iamtypes.AccessKeyMetadata, 0, keys)
	for i := range keys {
		keyMeta = append(keyMeta, iamtypes.AccessKeyMetadata{
			UserName:    aws.String(user),
			AccessKeyId: aws.String(fmt.Sprintf("AKIAEXAMPLEKEY%06d", i)),
			Status:      iamtypes.StatusTypeActive,
			CreateDate:  &created,
		})
	}
	fake := &iamUserMFAFake{
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{
			user: {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/" + user)}},
		},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{user: keyMeta},
	}
	result, err := awsclient.EnrichIAMUserMFA(context.Background(),
		&awsclient.ServiceClients{IAM: fake}, iamUserResources(user), nil)
	if err != nil {
		t.Fatalf("EnrichIAMUserMFA: unexpected error: %v", err)
	}
	return result
}

// TestFindingRowCap_RepeatedEmissionForOneCodeKeepsOneClosingRow drives the
// iam-user enricher, which emits the "key too old" code once per key. The cap
// holds over the combined rows, not per call: one closing row, K counting
// every key not shown, kept rows in call order.
//
// It is also the pin that a stored row whose value merely starts with the
// ellipsis — a masked key ID reads "…0009" — is content and not the stored
// count. lastFourOfKeyID is the only production row value that begins that
// way, so if the closing-row parse could misfire on real output it would
// misfire here, dropping the tenth key.
func TestFindingRowCap_RepeatedEmissionForOneCodeKeepsOneClosingRow(t *testing.T) {
	const user = "cap-many-keys"
	hidden := 2
	keys := awsclient.FindingRowCap + hidden

	rows := capKeyRows(t, capEnrichUser(t, user, keys), user)
	assertCappedRows(t, rows, keys)

	for i, r := range rows[:awsclient.FindingRowCap] {
		want := fmt.Sprintf("…%04d", i)
		if r.Value != want {
			t.Errorf("kept row %d Value = %q, want %q — kept rows are the first %d keys, in call order",
				i, r.Value, want, awsclient.FindingRowCap)
		}
	}
}

// TestFindingRowCap_RepeatedEmissionsReachingExactlyTheCapHaveNoClosingRow is
// the accumulating negative case: ten separate calls of one row each land on
// the cap and nothing is hidden, so nothing closes the list.
func TestFindingRowCap_RepeatedEmissionsReachingExactlyTheCapHaveNoClosingRow(t *testing.T) {
	const user = "cap-exact-keys"
	keys := awsclient.FindingRowCap

	rows := capKeyRows(t, capEnrichUser(t, user, keys), user)
	if len(rows) != keys {
		t.Fatalf("%d separate emissions of one row rendered %d rows, want %d and no closing row; rows=%+v",
			keys, len(rows), keys, rows)
	}
	for i, r := range rows {
		if strings.Contains(r.Value, "more") {
			t.Errorf("row %d (%q) closes the list, but nothing was hidden", i, r.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// The class: every finding that lists combos, not only the policy one.
// ---------------------------------------------------------------------------

// TestFindingRowCap_RoleInlinePrivEscRowsAreCapped pins the second caller of
// privEscComboRows. A role with an inline policy that matches the same
// escalation combinations lists them on the role row, and that list is as
// unbounded as the policy one — the builder no longer caps, so whatever the
// role's rows pass through has to.
func TestFindingRowCap_RoleInlinePrivEscRowsAreCapped(t *testing.T) {
	const roleName = "acme-inline-escalate-many"
	doc := privEscCapPolicyDocument()

	parsed, err := iampolicy.Parse(doc)
	if err != nil {
		t.Fatalf("fixture self-check: iampolicy.Parse: %v", err)
	}
	combos := parsed.PrivilegeEscalation()
	if len(combos) <= awsclient.FindingRowCap {
		t.Fatalf("fixture defect: %d combos does not exceed the cap of %d", len(combos), awsclient.FindingRowCap)
	}

	r := w4FetchRole(t, &w4RoleListFake{
		roles: []iamtypes.Role{w4Role(roleName, "/", w4TrustEC2Only)},
		inline: map[string]map[string]string{
			roleName: {"acme-break-glass": doc},
		},
	}, roleName)

	ad, ok := r.AttentionDetails[w4CodeRoleInlinePrivEsc]
	if !ok {
		t.Fatalf("role %q carries no inline privilege-escalation rows; AttentionDetails=%+v", roleName, r.AttentionDetails)
	}
	// One Policy row precedes the combos, so the list the cap sees is one
	// longer than the combo count.
	assertCappedRows(t, ad.Rows, len(combos)+1)

	// The role's own list, not a generic one: the Policy row survives at the
	// head, and the closing row carries the "!" tier the combo rows carry so
	// it is coloured with them.
	//
	// Inverted for the cap batch's spec row 9: this required the closing row's
	// label to be "Combo", the label of the row above it. A closing row is the
	// count of the combos not shown, not another combo. Do not restore.
	if got, want := ad.Rows[0].Label, "Policy"; got != want {
		t.Errorf("first kept row Label = %q, want %q", got, want)
	}
	closing := ad.Rows[awsclient.FindingRowCap]
	if closing.Label != "" {
		t.Errorf("closing row Label = %q, want empty", closing.Label)
	}
	if got, want := closing.Tier, "!"; got != want {
		t.Errorf("closing row Tier = %q, want %q", got, want)
	}
	if got, want := closing.Value, capOverflowValue(len(combos)+1-awsclient.FindingRowCap); got != want {
		t.Errorf("closing row Value = %q, want %q", got, want)
	}
}
