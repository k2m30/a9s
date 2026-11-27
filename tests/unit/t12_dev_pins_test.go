package unit_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// t12ListCallFields are the fields a list fetcher reads with a second call
// the by-ID lane does not send: an EC2 drill by ID is one DescribeInstances
// call, and the status checks come from DescribeInstanceStatus.
var t12ListCallFields = map[string][]string{"ec2": {"instance_status", "system_status"}} //nolint:gochecknoglobals // static per-type contract

// A resource reached by its ID is the one the list shows: every type that
// can fetch rows by ID builds them with its list's own builder, so the two
// lanes state the same fields for one resource.
func TestT12Dev_ByIDRowsCarryTheListRowsFields(t *testing.T) {
	clients := demo.NewServiceClients()
	compared := 0
	for _, td := range catalog.All() {
		byIDs := resource.GetFetchByIDs(td.ShortName)
		list := resource.GetPaginatedFetcher(td.ShortName)
		if byIDs == nil || list == nil || td.FetchByIDs == nil {
			continue
		}
		page, _ := list(context.Background(), clients, "")
		listed := map[string]resource.Resource{}
		var ids []string
		for _, r := range page.Resources {
			listed[r.ID] = r
			ids = append(ids, r.ID)
		}
		rows, _ := byIDs(context.Background(), clients, ids)
		for _, r := range rows {
			l, ok := listed[r.ID]
			if !ok {
				continue
			}
			compared++
			// A value the by-ID answer did not carry, such as the ARN of a key
			// whose describe was refused, is empty there rather than different.
			for k, v := range l.Fields {
				if slices.Contains(t12ListCallFields[td.ShortName], k) {
					continue
				}
				got, ok := r.Fields[k]
				if !ok || (got != "" && got != v) {
					t.Errorf("%s %s: %s by ID %q (present %v), listed %q", td.ShortName, r.ID, k, got, ok, v)
				}
			}
			for k := range r.Fields {
				if _, ok := l.Fields[k]; !ok {
					t.Errorf("%s %s: by ID writes %s, the list does not", td.ShortName, r.ID, k)
				}
			}
		}
	}
	if compared == 0 {
		t.Fatal("no row was fetched both ways; the pin reads nothing")
	}
}

// The creating replica reached by its ID reads as the replica the list shows.
func TestT12Dev_KMSReplicaByIDStatesItsKeyType(t *testing.T) {
	rows, err := resource.GetFetchByIDs("kms")(context.Background(), demo.NewServiceClients(), []string{demofixtures.KMSCreatingReplicaKeyID})
	if err != nil || len(rows) != 1 {
		t.Fatalf("kms by ID: %d rows, err %v", len(rows), err)
	}
	if got := rows[0].Fields["multi_region_key_type"]; got != "REPLICA" {
		t.Errorf("multi_region_key_type = %q, want REPLICA", got)
	}
}

// A cache file another build wrote can carry a code this build does not
// declare; the load drops it and keeps the row and its declared findings.
func TestT12Dev_CachedFindingTheCatalogDoesNotDeclareIsDroppedAtLoad(t *testing.T) {
	root := t.TempDir()
	const retired = domain.FindingCode("ec2.retired_by_an_earlier_build")
	declared := domain.Finding{Code: awsclient.CodeEC2PublicIP, Phrase: "public IP", Severity: domain.SevWarn, Source: "wave1"}
	if !catalog.Declares("ec2", declared.Code) || catalog.Declares("ec2", retired) {
		t.Fatal("precondition: ec2 declares the kept code and not the retired one")
	}
	store := cache.LoadDirIn(root, "example-readonly", "us-east-1")
	store.Put("ec2", cache.TypeFile{
		Count: 1, HasResources: true,
		Rows: []cache.Row{{ID: "i-0aaa", Name: "i-0aaa", Findings: []domain.Finding{declared, {Code: retired, Phrase: "retired", Severity: domain.SevBroken}}}},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatal(err)
	}
	tf, ok := cache.LoadDirIn(root, "example-readonly", "us-east-1").Type("ec2")
	if !ok || len(tf.Rows) != 1 {
		t.Fatalf("reloaded ec2: ok %v, %d rows", ok, len(tf.Rows))
	}
	if got := tf.Rows[0].Findings; len(got) != 1 || got[0].Code != declared.Code {
		t.Errorf("reloaded findings = %v, want only %s", got, declared.Code)
	}
}

// A detail enricher judges only what it adds: a row that comes in carrying a
// finding its type does not declare, as a row restored from an older cache
// file can, opens its detail and keeps that finding.
func TestT12Dev_DetailEnrichJudgesOnlyTheFindingsItAdds(t *testing.T) {
	clients := demo.NewServiceClients()
	page, err := resource.GetPaginatedChildFetcher("transfer_agreements")(context.Background(), clients, resource.ParentContext{"server_id": demofixtures.ProdAS2GatewayID}, "")
	if err != nil || len(page.Resources) == 0 {
		t.Fatalf("transfer agreements: %d rows, err %v", len(page.Resources), err)
	}
	row := page.Resources[0]
	carried := domain.Finding{Code: "transfer_agreements.retired_by_an_earlier_build", Phrase: "retired", Severity: domain.SevWarn, Source: "wave1"}
	row.Findings = append(slices.Clone(row.Findings), carried)
	before := slices.Clone(row.Findings)
	enrich := catalog.ChildOnly("transfer_agreements").DetailEnrich
	out, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, row)
	if err != nil && !domain.DetailResultApplies(err) {
		t.Fatalf("enrich: %v", err)
	}
	if !slices.Contains(out.Findings, carried) {
		t.Errorf("findings %v lost the finding the row came in with", out.Findings)
	}
	for _, f := range out.Findings {
		if !slices.Contains(before, f) && !catalog.Declares("transfer_agreements", f.Code) {
			t.Errorf("the enricher added %s, which transfer_agreements does not declare", f.Code)
		}
	}
}

type t12AthenaFake struct{ getErr error }

func (f t12AthenaFake) ListWorkGroups(context.Context, *athena.ListWorkGroupsInput, ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	return &athena.ListWorkGroupsOutput{WorkGroups: []athenatypes.WorkGroupSummary{{Name: aws.String("acme-paused"), State: athenatypes.WorkGroupStateDisabled}}}, nil
}

func (f t12AthenaFake) GetWorkGroup(context.Context, *athena.GetWorkGroupInput, ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	return nil, f.getErr
}

// A disabled workgroup whose configuration read failed is both: its row
// states the disabled state and that its configuration was not read.
func TestT12Dev_DisabledWorkgroupWhoseConfigFailsKeepsItsStateAndIsDegraded(t *testing.T) {
	state, ok := awsclient.StateFinding("athena", string(athenatypes.WorkGroupStateDisabled))
	if !ok {
		t.Fatal("precondition: athena raises a state finding for DISABLED")
	}
	page, _ := unit.BindPage("athena")(awsclient.FetchAthenaWorkgroupsPage(context.Background(), t12AthenaFake{getErr: errors.New("throttled")}, ""))
	if len(page.Resources) != 1 {
		t.Fatalf("rows = %d, want 1", len(page.Resources))
	}
	codes := map[domain.FindingCode]bool{}
	for _, f := range page.Resources[0].Findings {
		codes[f.Code] = true
	}
	for _, want := range []domain.FindingCode{state.Code, awsclient.DetailsUnavailableCode("athena")} {
		if !codes[want] {
			t.Errorf("findings %v lack %s", page.Resources[0].Findings, want)
		}
	}
}

type t12EBPages struct{ awsclient.EventBridgeAPI }

func (t12EBPages) ListTargetsByRule(_ context.Context, in *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	if in.NextToken == nil {
		return &eventbridge.ListTargetsByRuleOutput{
			Targets:   []ebtypes.Target{{Id: aws.String("queue"), Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-queue")}},
			NextToken: aws.String("page-2"),
		}, nil
	}
	return &eventbridge.ListTargetsByRuleOutput{
		Targets: []ebtypes.Target{{Id: aws.String("fn"), Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:acme-fn")}},
	}, nil
}

// A rule's targets page; a function that is the rule's target on its second
// page is found as surely as one on its first.
func TestT12Dev_LambdaRulePivotReadsEveryTargetPage(t *testing.T) {
	var check domain.RelatedChecker
	for _, def := range resource.GetRelated("lambda") {
		if def.TargetType == "eb-rule" {
			check = def.Checker
		}
	}
	if check == nil {
		t.Fatal("lambda has no EventBridge rule pivot")
	}
	fn := resource.Resource{ID: "acme-fn", Name: "acme-fn", Type: "lambda", RawStruct: lambdatypes.FunctionConfiguration{
		FunctionName: aws.String("acme-fn"), FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:acme-fn"),
	}}
	rules := resource.ResourceCache{"eb-rule": {Resources: []resource.Resource{{ID: "acme-rule", Name: "acme-rule", Type: "eb-rule", Fields: map[string]string{"name": "acme-rule", "event_bus": "default"}}}}}
	got := check(context.Background(), &awsclient.ServiceClients{EventBridge: t12EBPages{}, Region: "us-east-1"}, fn, rules)
	if !slices.Equal(got.ResourceIDs(), []string{"acme-rule"}) || got.Truncated() {
		t.Errorf("rules = %v (truncated %v), want [acme-rule] exact", got.ResourceIDs(), got.Truncated())
	}
}
