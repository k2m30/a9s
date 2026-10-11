package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t576Run3 runs shortName's list enricher on its demo rows with clients
// whatever wrap returns.
func t576Run3(t *testing.T, shortName string, rows []resource.Resource, cache resource.ResourceCache, wrap func(*awsclient.ServiceClients)) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	clients := demo.NewServiceClients()
	wrap(clients)
	return t576Enricher(t, shortName).Fn(context.Background(), clients, rows, cache)
}

// GetHostedZone answers the private-zone orphan check alone: a public zone
// is not asked, so refusing the call leaves every public zone inspected.
func TestT576Dev_R53PublicZoneNeedsNoGetHostedZone(t *testing.T) {
	_, rows, cache := t576Bench(t, "r53")
	got := t576EnrichType(t, "r53", rows, cache, t576DenyOp("GetHostedZone"))
	public := 0
	for _, r := range rows {
		if r.Fields["private_zone"] != "false" {
			continue
		}
		public++
		if check, ok := got.res.TruncatedIDs[r.ID]; ok && check == "GetHostedZone" {
			t.Errorf("public zone %s marked %q, though no public-zone check reads GetHostedZone", r.ID, check)
		}
	}
	for _, c := range got.calls {
		if c.Op == "GetHostedZone" {
			for _, r := range rows {
				if r.Fields["private_zone"] == "false" && t576Names(t576Leaves(c.In), r) {
					t.Errorf("GetHostedZone sent for public zone %s", r.ID)
				}
			}
		}
	}
	if public == 0 {
		t.Fatal("the r53 bench has no public zone")
	}
}

// capGroupIAM answers GetGroup with one member and a next page, forever.
type capGroupIAM struct{ awsclient.IAMAPI }

func (capGroupIAM) GetGroup(_ context.Context, in *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	return &iam.GetGroupOutput{
		Group:       &iamtypes.Group{GroupName: in.GroupName},
		Users:       []iamtypes.User{{UserName: aws.String("member-" + aws.ToString(in.Marker))}},
		IsTruncated: true,
		Marker:      aws.String(aws.ToString(in.Marker) + "x"),
	}, nil
}

// A member walk stopped at the page cap has still read a member, which
// decides the orphan check; no other check reads the member list.
func TestT576Dev_IAMGroupCappedMemberWalkWithMembersIsInspected(t *testing.T) {
	_, rows, cache := t576Bench(t, "iam-group")
	res, err := t576Run3(t, "iam-group", rows, cache, func(c *awsclient.ServiceClients) { c.IAM = capGroupIAM{c.IAM} })
	if err != nil {
		t.Fatalf("a page cap is not a failure: %v", err)
	}
	for _, r := range rows {
		if check, ok := res.TruncatedIDs[r.ID]; ok {
			t.Errorf("%s marked %q, though its capped member walk read members and decided every check", r.ID, check)
		}
		if v := res.FieldUpdates[r.ID]["member_count"]; !strings.HasSuffix(v, "+") {
			t.Errorf("%s: member_count = %q, want a lower bound", r.ID, v)
		}
	}
}

// Each read below is retried on a throttle like its sibling reads: one
// throttle answers on the retry, with no failure and no mark.
func TestT576Dev_ReadsRetryAThrottle(t *testing.T) {
	t576FastRetries(t)
	for _, c := range []struct{ short, op string }{
		{"athena", "GetWorkGroup"},
		{"cb", "BatchGetBuilds"},
		{"codeartifact", "GetRepositoryPermissionsPolicy"},
		{"ddb", "DescribeContinuousBackups"},
		{"ecs", "DescribeClusters"},
		{"role", "GetRole"},
	} {
		t.Run(c.short+"/"+c.op, func(t *testing.T) {
			_, rows, cache := t576Bench(t, c.short)
			clean := t576EnrichType(t, c.short, rows, cache, nil)
			got := t576EnrichType(t, c.short, rows, cache, t576ThrottleFirst(c.op))
			sent := 0
			for _, call := range got.calls {
				if call.Op == c.op {
					sent++
				}
			}
			if sent == 0 {
				t.Fatalf("the %s bench never calls %s", c.short, c.op)
			}
			if got.err != nil {
				t.Errorf("one throttle surfaced as a failure: %v", got.err)
			}
			for id, check := range got.res.TruncatedIDs {
				if _, ok := clean.res.TruncatedIDs[id]; !ok {
					t.Errorf("%s marked %q after a retried throttle", id, check)
				}
			}
		})
	}
}

// capKeysIAM answers ListAccessKeys with a next page, forever.
type capKeysIAM struct{ awsclient.IAMAPI }

func (capKeysIAM) ListAccessKeys(_ context.Context, in *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{IsTruncated: true, Marker: aws.String(aws.ToString(in.Marker) + "x")}, nil
}

// a9s's own page cap is a bound, not a refused call: the row reads the cap
// check and nothing is reported as failed.
func TestT576Dev_PageCapIsNotAFailure(t *testing.T) {
	_, rows, cache := t576Bench(t, "iam-user")
	res, err := t576Run3(t, "iam-user", rows, cache, func(c *awsclient.ServiceClients) { c.IAM = capKeysIAM{c.IAM} })
	if err != nil {
		t.Errorf("a page cap reported as a failure: %v", err)
	}
	for _, r := range rows {
		if check := res.TruncatedIDs[r.ID]; check != awsclient.CheckCap {
			t.Errorf("%s: not-inspected check = %q, want %q", r.ID, check, awsclient.CheckCap)
		}
	}
}

// stuckRecordsR53 answers ListResourceRecordSets truncated with no next record.
type stuckRecordsR53 struct{ awsclient.Route53API }

func (stuckRecordsR53) ListResourceRecordSets(context.Context, *route53.ListResourceRecordSetsInput, ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return &route53.ListResourceRecordSetsOutput{IsTruncated: true}, nil
}

// A truncated record page with no next record is an answer that came back
// short: the zone names the call and the missing field, and the failure is
// reported. It is not a9s's own page cap.
func TestT576Dev_R53StuckRecordPageIsAnUnusableAnswer(t *testing.T) {
	_, rows, cache := t576Bench(t, "r53")
	res, err := t576Run3(t, "r53", rows, cache, func(c *awsclient.ServiceClients) { c.Route53 = stuckRecordsR53{c.Route53} })
	if err == nil {
		t.Error("a record page with no next record reported no failure")
	}
	const want = "ListResourceRecordSets returned no next record name"
	for _, r := range rows {
		if r.Fields["private_zone"] != "false" {
			continue
		}
		if check := res.TruncatedIDs[r.ID]; check != want {
			t.Errorf("public zone %s: not-inspected check = %q, want %q", r.ID, check, want)
		}
	}
}

// An aggregate names every call it carries.
func TestT576Dev_AggregateLabelNamesEveryCall(t *testing.T) {
	for _, c := range []struct{ short, op, label string }{
		{"codeartifact", "ListPackages", "repository packages and policy"},
		{"cf", "GetDistributionConfig", "distribution config and origin buckets"},
	} {
		_, rows, cache := t576Bench(t, c.short)
		got := t576EnrichType(t, c.short, rows, cache, t576DenyOp(c.op))
		if got.err == nil || !strings.Contains(got.err.Error(), c.label) {
			t.Errorf("%s with %s refused: error %v, want it labelled %q", c.short, c.op, got.err, c.label)
		}
	}
}

// Two reads refused for two different actions are two failures: granting
// the first one alone would leave the function failing again.
func TestT576Dev_LambdaRecordsEveryFailedRead(t *testing.T) {
	_, rows, cache := t576Bench(t, "lambda")
	got := t576EnrichType(t, "lambda", rows, cache, func(service, op string, _ any) error {
		if op == "GetFunction" || op == "GetPolicy" {
			return t576Denied(service, op)
		}
		return nil
	})
	if got.err == nil {
		t.Fatal("GetFunction and GetPolicy refused, enricher returned no error")
	}
	for _, op := range []string{"GetFunction", "GetPolicy"} {
		if !strings.Contains(got.err.Error(), op) {
			t.Errorf("the failure report does not name %s: %v", op, got.err)
		}
	}
}

// A queue or table deleted between the list call and the per-row read is a
// race, not a failure, in the code the SDK really reports.
func TestT576Dev_GoneQueueAndTableAreNotFound(t *testing.T) {
	for _, code := range []string{"AWS.SimpleQueueService.NonExistentQueue", "TableNotFoundException"} {
		err := &smithy.OperationError{ServiceID: "x", OperationName: "y", Err: &smithy.GenericAPIError{Code: code, Fault: smithy.FaultClient}}
		if !awsclient.IsNotFoundErr(err) {
			t.Errorf("IsNotFoundErr(%s) = false", code)
		}
	}
}

// A trail delivering to a deleted bucket is answered: there is no bucket
// posture to report, so the trail is neither marked nor counted as failed.
func TestT576Dev_TrailLogBucketGoneIsAnAnswer(t *testing.T) {
	gone := func(service, op string, _ any) error {
		switch op {
		case "GetPublicAccessBlock", "GetBucketPolicyStatus", "GetBucketAcl", "GetBucketLogging":
			return t576APIError(service, op, "NoSuchBucket", "The specified bucket does not exist")
		}
		return nil
	}
	got := t576LoadTrails(t, gone)
	if got.run.err != nil {
		t.Errorf("a deleted log bucket reported as a failure: %v", got.run.err)
	}
	for _, r := range got.rows {
		if check, ok := got.run.res.TruncatedIDs[r.ID]; ok {
			t.Errorf("%s marked %q for a log bucket that does not exist", r.ID, check)
		}
	}
}

// A refused GetStages leaves stages_count blank beside the row's mark,
// never a count of the pages read before the refusal.
func TestT576Dev_APIGWRefusedStagesLeaveNoCount(t *testing.T) {
	_, rows, cache := t576Bench(t, "apigw")
	clean := t576EnrichType(t, "apigw", rows, cache, nil)
	got := t576EnrichType(t, "apigw", rows, cache, t576DenyOp("GetStages"))
	checked := 0
	for id := range clean.res.FieldUpdates {
		if _, ok := clean.res.FieldUpdates[id]["stages_count"]; !ok {
			continue
		}
		checked++
		if v, ok := got.res.FieldUpdates[id]["stages_count"]; ok {
			t.Errorf("%s: stages_count = %q after GetStages was refused", id, v)
		}
		if _, ok := got.res.TruncatedIDs[id]; !ok {
			t.Errorf("%s: GetStages refused, row not marked", id)
		}
	}
	if checked == 0 {
		t.Fatal("the apigw bench writes no stages_count")
	}
}

// A VPC whose subnet list was not read is still asked about its own id: an
// ACTIVE flow log there proves flow logs on; finding none proves nothing,
// since a subnet-level log may cover it.
func TestT576Dev_VPCUnreadSubnetsStillAskTheVPC(t *testing.T) {
	_, rows, cache := t576Bench(t, "vpc")
	unread := make([]resource.Resource, len(rows))
	for i, r := range rows {
		r.Fields = map[string]string{}
		for k, v := range rows[i].Fields {
			r.Fields[k] = v
		}
		r.Fields["subnet_ids_unread"] = "true"
		unread[i] = r
	}
	got := t576EnrichType(t, "vpc", unread, cache, nil)
	yes := 0
	for _, r := range unread {
		v := got.res.FieldUpdates[r.ID]["flow_logs"]
		check := got.res.TruncatedIDs[r.ID]
		switch {
		case v == "yes":
			yes++
		case v == "" && check == "subnet list incomplete":
		default:
			t.Errorf("%s: flow_logs = %q, mark %q; want yes, or no value and the subnet mark", r.ID, v, check)
		}
	}
	if yes == 0 {
		t.Error("no VPC proved flow logs from its own id")
	}
}
