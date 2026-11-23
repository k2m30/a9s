package unit_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

func t6DevMenuBadge(t *testing.T, a *t6App) (int, bool) {
	t.Helper()
	entry := menuEntryFor(a.c.Snapshot().Body.Menu, a.short)
	if entry == nil {
		t.Fatalf("the menu has no entry for %s", a.short)
	}
	return entry.IssueBadge.Count, entry.IssueBadge.Truncated
}

func t6DevRaisesBroken(t *testing.T, short string) bool {
	t.Helper()
	td := resource.FindResourceType(short)
	if td == nil {
		t.Fatalf("%s not registered", short)
	}
	for _, f := range td.Findings {
		if f.Source == "wave2" && f.Severity == domain.SevBroken {
			return true
		}
	}
	return false
}

// A menu refresh re-reads the rows and carries their Wave 2 findings and
// marks over: the badge it leaves is the badge the Wave 2 answer left, count
// and lower bound alike, because both are read from the rows the store holds.
func TestT6Dev_Wave1RefreshKeepsTheWave2Badge(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	for _, short := range []string{"sqs", "s3", "ecr", "dbi"} {
		t.Run(short, func(t *testing.T) {
			a := t6NewApp(t, short, demo.NewServiceClients(), byType[short], cache, false)
			a.wave2(t)
			issues, lower := t6DevMenuBadge(t, a)
			a.wave1(t)
			gotIssues, gotLower := t6DevMenuBadge(t, a)
			if gotIssues != issues || gotLower != lower {
				t.Errorf("badge after the Wave 2 answer = (%d, lower bound %v); after a menu refresh = (%d, lower bound %v)", issues, lower, gotIssues, gotLower)
			}
		})
	}
}

type t6DevEndlessDocDB struct{ awsclient.DocDBAPI }

func (t6DevEndlessDocDB) DescribePendingMaintenanceActions(context.Context, *docdb.DescribePendingMaintenanceActionsInput, ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	return &docdb.DescribePendingMaintenanceActionsOutput{Marker: aws.String("more")}, nil
}

type t6DevEndlessRDS struct{ awsclient.RDSAPI }

func (t6DevEndlessRDS) DescribePendingMaintenanceActions(context.Context, *rds.DescribePendingMaintenanceActionsInput, ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{Marker: aws.String("more")}, nil
}

type t6DevEndlessEC2 struct{ awsclient.EC2API }

func (t6DevEndlessEC2) DescribeVolumeStatus(context.Context, *ec2svc.DescribeVolumeStatusInput, ...func(*ec2svc.Options)) (*ec2svc.DescribeVolumeStatusOutput, error) {
	return &ec2svc.DescribeVolumeStatusOutput{NextToken: aws.String("more")}, nil
}

func (t6DevEndlessEC2) DescribeSnapshots(context.Context, *ec2svc.DescribeSnapshotsInput, ...func(*ec2svc.Options)) (*ec2svc.DescribeSnapshotsOutput, error) {
	return &ec2svc.DescribeSnapshotsOutput{NextToken: aws.String("more")}, nil
}

// An account-wide walk cut at its page budget leaves every row no walked page
// named marked at the cap; the type can raise "!", so its badge is a lower
// bound and its scan partial.
func TestT6Dev_AccountWalkCutMakesTheBadgeALowerBound(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	for short, wrap := range map[string]func(*awsclient.ServiceClients){
		"dbc":      func(c *awsclient.ServiceClients) { c.DocDB = t6DevEndlessDocDB{c.DocDB} },
		"dbi":      func(c *awsclient.ServiceClients) { c.RDS = t6DevEndlessRDS{c.RDS} },
		"ebs":      func(c *awsclient.ServiceClients) { c.EC2 = t6DevEndlessEC2{c.EC2} },
		"ebs-snap": func(c *awsclient.ServiceClients) { c.EC2 = t6DevEndlessEC2{c.EC2} },
	} {
		t.Run(short, func(t *testing.T) {
			if !t6DevRaisesBroken(t, short) {
				t.Fatalf("precondition: %s raises no \"!\" Wave-2 finding", short)
			}
			clients := demo.NewServiceClients()
			wrap(clients)
			rows := byType[short]
			a := t6NewApp(t, short, clients, rows, cache, false)
			ev := a.wave2(t)
			capped := 0
			for _, check := range ev.TruncatedIDs {
				if check == awsclient.CheckCap {
					capped++
				}
			}
			if capped == 0 {
				t.Fatalf("no row marked at the cap after a walk that never ended: %v", ev.TruncatedIDs)
			}
			if _, lower := t6DevMenuBadge(t, a); !lower {
				t.Error("badge is exact after rows were left at the cap")
			}
			if got := a.scanOutcome(t); got != runtime.ProbePartial {
				t.Errorf("scan status = %q, want %q", got, runtime.ProbePartial)
			}
		})
	}
}

type t6DevCodeBuild struct {
	awsclient.CodeBuildAPI
	quiet string
}

func (f t6DevCodeBuild) ListBuildsForProject(_ context.Context, in *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	if aws.ToString(in.ProjectName) == f.quiet {
		return &codebuild.ListBuildsForProjectOutput{}, nil
	}
	return &codebuild.ListBuildsForProjectOutput{Ids: []string{aws.ToString(in.ProjectName) + ":a1b2c3d4-1111-2222-3333-444455556666"}}, nil
}

func (t6DevCodeBuild) BatchGetBuilds(context.Context, *codebuild.BatchGetBuildsInput, ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	return nil, t576Denied("CodeBuild", "BatchGetBuilds")
}

// A refused batch read after a project that has no builds was answered: that
// project is inspected and its column cleared, the projects whose builds the
// batch held are marked and keep their column, the error is returned, and
// the badge is a lower bound with the scan partial.
func TestT6Dev_RefusedBuildBatchMarksOnlyTheProjectsItHeld(t *testing.T) {
	rows := []resource.Resource{
		{ID: "acme-quiet", Name: "acme-quiet", Fields: map[string]string{"last_build": "OK"}},
		{ID: "acme-busy", Name: "acme-busy", Fields: map[string]string{"last_build": "OK"}},
	}
	a := t6NewApp(t, "cb", &awsclient.ServiceClients{CodeBuild: t6DevCodeBuild{quiet: "acme-quiet"}}, rows, nil, false)
	ev := a.wave2(t)
	if ev.Err == nil {
		t.Error("the refused batch read returned no error")
	}
	if _, marked := ev.TruncatedIDs["acme-quiet"]; marked {
		t.Errorf("acme-quiet, which has no builds, is marked: %v", ev.TruncatedIDs)
	}
	if got := ev.TruncatedIDs["acme-busy"]; got == "" {
		t.Errorf("acme-busy, whose build the refused batch held, is not marked: %v", ev.TruncatedIDs)
	}
	if v, _ := t6DevStoredField(t, a, "acme-quiet", "last_build"); v != "" {
		t.Errorf("acme-quiet last_build = %q, want it cleared by the answer", v)
	}
	if v, _ := t6DevStoredField(t, a, "acme-busy", "last_build"); v != "OK" {
		t.Errorf("acme-busy last_build = %q, want the earlier value kept", v)
	}
	if _, lower := t6DevMenuBadge(t, a); !lower {
		t.Error("badge is exact after a project went uninspected")
	}
	if got := a.scanOutcome(t); got != runtime.ProbePartial {
		t.Errorf("scan status = %q, want %q", got, runtime.ProbePartial)
	}
}

type t6DevDynamoDB struct {
	awsclient.DynamoDBAPI
	refuse string
}

func (f t6DevDynamoDB) DescribeContinuousBackups(ctx context.Context, in *dynamodb.DescribeContinuousBackupsInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	if aws.ToString(in.TableName) == f.refuse {
		return nil, t576Denied("DynamoDB", "DescribeContinuousBackups")
	}
	return f.DynamoDBAPI.DescribeContinuousBackups(ctx, in, opts...)
}

// A table whose continuous-backups read is refused after another answered is
// marked and the error returned; the scan is partial.
func TestT6Dev_RefusedTableReadMarksThatTable(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	rows := byType["ddb"]
	if len(rows) < 2 {
		t.Fatalf("precondition: %d demo ddb rows", len(rows))
	}
	refused := rows[len(rows)-1]
	clients := demo.NewServiceClients()
	clients.DynamoDB = t6DevDynamoDB{DynamoDBAPI: clients.DynamoDB, refuse: refused.Name}
	a := t6NewApp(t, "ddb", clients, rows, cache, false)
	ev := a.wave2(t)
	if ev.Err == nil {
		t.Error("the refused read returned no error")
	}
	if _, marked := ev.TruncatedIDs[refused.ID]; !marked {
		t.Errorf("%s, whose read was refused, is not marked: %v", refused.ID, ev.TruncatedIDs)
	}
	if _, marked := ev.TruncatedIDs[rows[0].ID]; marked && ev.TruncatedIDs[rows[0].ID] == "DescribeContinuousBackups" {
		t.Errorf("%s, whose read answered, is marked for it", rows[0].ID)
	}
	if got := a.scanOutcome(t); got != runtime.ProbePartial {
		t.Errorf("scan status = %q, want %q", got, runtime.ProbePartial)
	}
}

// The log-stream pass fills last_event_at and decides no finding. The rows it
// does not reach — past its cap — keep the column an earlier answer wrote,
// and are not marked not inspected.
func TestT6Dev_FieldOnlyRowsPastTheCapKeepTheirColumn(t *testing.T) {
	rows := t6DemoRows(t, "logs")
	if len(rows) <= awsclient.EnrichmentCap {
		t.Fatalf("precondition: %d demo log groups, want more than the cap %d", len(rows), awsclient.EnrichmentCap)
	}
	rows = slices.Clone(rows)
	last := rows[len(rows)-1]
	last.Fields = maps.Clone(last.Fields)
	last.Fields["last_event_at"] = "seeded"
	rows[len(rows)-1] = last
	a := t6NewApp(t, "logs", demo.NewServiceClients(), rows, nil, false)
	ev := a.wave2(t)
	if check, marked := ev.TruncatedIDs[last.ID]; marked {
		t.Errorf("%s past the field-only cap is marked %q", last.ID, check)
	}
	if v, _ := t6DevStoredField(t, a, last.ID, "last_event_at"); v != "seeded" {
		t.Errorf("%s last_event_at = %q, want the earlier value kept", last.ID, v)
	}
}

type t6DevLogStreams struct {
	awsclient.CWLogsAPI
	refused string
}

func (f t6DevLogStreams) DescribeLogStreams(_ context.Context, in *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	if aws.ToString(in.LogGroupName) == f.refused {
		return nil, t576Denied("CloudWatchLogs", "DescribeLogStreams")
	}
	return &cloudwatchlogs.DescribeLogStreamsOutput{}, nil
}

// A group whose stream read was refused keeps its Last Event; a group the
// read answered with no stream has it cleared.
func TestT6Dev_LogsLastEventIsClearedOnlyByAnAnswer(t *testing.T) {
	rows := slices.Clone(t6DemoRows(t, "logs")[:2])
	for i := range rows {
		rows[i].Fields = maps.Clone(rows[i].Fields)
		rows[i].Fields["last_event_at"] = "seeded"
	}
	refused, answered := rows[0], rows[1]
	clients := demo.NewServiceClients()
	clients.CloudWatchLogs = t6DevLogStreams{CWLogsAPI: clients.CloudWatchLogs, refused: refused.Name}
	a := t6NewApp(t, "logs", clients, rows, nil, false)
	a.wave2(t)
	if v, _ := t6DevStoredField(t, a, refused.ID, "last_event_at"); v != "seeded" {
		t.Errorf("%s last_event_at = %q after a refused read, want the earlier value kept", refused.ID, v)
	}
	if v, _ := t6DevStoredField(t, a, answered.ID, "last_event_at"); v != "" {
		t.Errorf("%s last_event_at = %q after an answer with no stream, want it cleared", answered.ID, v)
	}
}

// A row's status stays partial only while a marked row or the enrichment's
// error is behind it: the rows a sweep leaves at its cap that an on-demand
// check had already answered are not marks, and answering the last marked
// row on demand clears the partial.
func TestT6Dev_PartialFollowsTheMarksThatRemain(t *testing.T) {
	rows := []resource.Resource{
		{ID: "acme-web-alb", Name: "acme-web-alb", Fields: map[string]string{}},
		{ID: "acme-api-alb", Name: "acme-api-alb", Fields: map[string]string{}},
	}
	scan := func(core *runtime.Core) runtime.ProbeOutcome {
		for _, st := range core.ScanStatus() {
			if st.ShortName == "elb" {
				return st.Outcome
			}
		}
		t.Fatal("elb has no scan status")
		return ""
	}
	onDemand := func(core *runtime.Core, id string) {
		core.HandleEvent(messages.RowEnriched{ResourceType: "elb", ResourceID: id, Findings: map[string][]domain.Finding{}, Gen: core.Session().EnrichmentGen, TypeGen: core.EnrichmentTypeGen("elb")})
	}

	t.Run("answered before the sweep", func(t *testing.T) {
		_, core := newTestControllerAndCore(t)
		core.ObserveRows("elb", rows, nil, session.OriginFetch, false)
		core.HandleEvent(messages.EnrichmentChecked{ResourceType: "elb", Findings: map[string][]domain.Finding{}, TruncatedIDs: map[string]string{rows[1].ID: awsclient.CheckCap}})
		onDemand(core, rows[1].ID)
		core.HandleEvent(messages.EnrichmentChecked{ResourceType: "elb", Findings: map[string][]domain.Finding{}, TruncatedIDs: map[string]string{rows[1].ID: awsclient.CheckCap}})
		if got := scan(core); got == runtime.ProbePartial {
			t.Errorf("scan status = %q with no row left marked", got)
		}
	})
	t.Run("answered after the sweep", func(t *testing.T) {
		_, core := newTestControllerAndCore(t)
		core.ObserveRows("elb", rows, nil, session.OriginFetch, false)
		core.HandleEvent(messages.EnrichmentChecked{ResourceType: "elb", Findings: map[string][]domain.Finding{}, TruncatedIDs: map[string]string{rows[1].ID: awsclient.CheckCap}})
		if got := scan(core); got != runtime.ProbePartial {
			t.Fatalf("precondition: scan status = %q with a marked row", got)
		}
		onDemand(core, rows[1].ID)
		if got := scan(core); got == runtime.ProbePartial {
			t.Errorf("scan status = %q after the last marked row was answered", got)
		}
	})
	t.Run("an errored sweep stays partial", func(t *testing.T) {
		_, core := newTestControllerAndCore(t)
		core.ObserveRows("elb", rows, nil, session.OriginFetch, false)
		core.HandleEvent(messages.EnrichmentChecked{ResourceType: "elb", Findings: map[string][]domain.Finding{}, TruncatedIDs: map[string]string{rows[1].ID: awsclient.CheckCap}, Err: errors.New("elb: DescribeLoadBalancerAttributes failed for 1 of 2 IDs")})
		onDemand(core, rows[1].ID)
		if got := scan(core); got != runtime.ProbePartial {
			t.Errorf("scan status = %q after an errored sweep, want %q", got, runtime.ProbePartial)
		}
	})
}

// The headless lane holds one fact for "this type's count is a lower bound":
// a badge seeded from the type file reaches the list title the menu shows.
func TestT6Dev_SeededLowerBoundReachesTheHeadlessTitle(t *testing.T) {
	c := newMenuController(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"sqs": 0},
			Truncated: map[string]bool{"sqs": true},
			Known:     map[string]bool{"sqs": true},
			Origin:    runtime.OriginCache,
		},
	})
	c.PushChildListScreen("sqs")
	c.ApplyResourcesLoaded("sqs", []resource.Resource{
		{ID: "https://sqs.us-east-1.amazonaws.com/123456789012/acme-orders", Name: "acme-orders", Fields: map[string]string{}},
		{ID: "https://sqs.us-east-1.amazonaws.com/123456789012/acme-billing", Name: "acme-billing", Fields: map[string]string{}},
	}, nil, false)
	if got := c.ListFrameTitle(); got != "sqs(2) !0+" {
		t.Errorf("list title = %q, want %q", got, "sqs(2) !0+")
	}
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	if got := c.ListFrameTitle(); got != "sqs(0+ of 2) [!]" {
		t.Errorf("issues-only title = %q, want %q", got, "sqs(0+ of 2) [!]")
	}
}

// A Fields key has one writer: the fetcher (FieldKeys) or the Wave 2
// enricher (IssueEnricherFieldKeys). A key in both lists has its Wave 1 value
// blanked whenever an answer leaves it unset.
func TestT6Dev_FetcherAndEnricherFieldKeysAreDisjoint(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		for _, k := range td.IssueEnricherFieldKeys {
			if slices.Contains(td.FieldKeys, k) {
				t.Errorf("%s: %q is in both FieldKeys and IssueEnricherFieldKeys", td.ShortName, k)
			}
		}
	}
}
