package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	ebridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// t1aDBIStatuses is every status in the DB instance status tables of the
// Amazon RDS and Aurora User Guides, as docs/resources/dbi.md §3/§4 list them.
var t1aDBIStatuses = []string{
	"available", "stopped",
	"backing-up", "backtracking", "configuring-enhanced-monitoring",
	"configuring-iam-database-auth", "configuring-log-exports",
	"converting-to-vpc", "creating", "delete-precheck", "deleting",
	"maintenance", "modifying", "moving-to-vpc", "rebooting",
	"renaming", "resetting-master-credentials", "starting",
	"stopping", "storage-config-upgrade", "storage-initialization",
	"storage-optimization", "upgrading",
	"failed", "storage-full", "incompatible-network", "incompatible-option-group",
	"incompatible-parameters", "incompatible-restore", "restore-error",
	"inaccessible-encryption-credentials", "inaccessible-encryption-credentials-recoverable",
	"incompatible-create", "insufficient-capacity", "upgrade-failed",
}

// t1aRedshiftStatuses is the ClusterStatus list of the SDK's
// redshift/types.Cluster doc comment, which docs/resources/redshift.md cites.
var t1aRedshiftStatuses = []string{
	"available", "available, prep-for-resize", "available, resize-cleanup",
	"cancelling-resize", "creating", "deleting", "final-snapshot",
	"hardware-failure", "incompatible-hsm", "incompatible-network",
	"incompatible-parameters", "incompatible-restore", "modifying", "paused",
	"rebooting", "renaming", "resizing", "rotating-keys", "storage-full",
	"updating-hsm",
}

// t1aDomain is the set of values AWS can report for a type's lifecycle
// field; enum is the SDK type that carries them, nil for a documented list.
type t1aDomain struct {
	enum   reflect.Type
	values []string
}

func t1aEnum[T interface {
	~string
	Values() []T
}]() t1aDomain {
	var zero T
	d := t1aDomain{enum: reflect.TypeOf(zero)}
	for _, v := range zero.Values() {
		d.values = append(d.values, string(v))
	}
	return d
}

var t1aStateDomains = map[string]t1aDomain{
	"cfn":      t1aEnum[cfntypes.StackStatus](),
	"eb":       t1aEnum[ebtypes.EnvironmentStatus](),
	"efs":      t1aEnum[efstypes.LifeCycleState](),
	"kms":      t1aEnum[kmstypes.KeyState](),
	"ddb":      t1aEnum[ddbtypes.TableStatus](),
	"lambda":   t1aEnum[lambdatypes.State](),
	"kinesis":  t1aEnum[kinesistypes.StreamStatus](),
	"msk":      t1aEnum[kafkatypes.ClusterState](),
	"eks":      t1aEnum[ekstypes.ClusterStatus](),
	"ng":       t1aEnum[ekstypes.NodegroupStatus](),
	"nat":      t1aEnum[ec2types.NatGatewayState](),
	"vpc":      t1aEnum[ec2types.VpcState](),
	"subnet":   t1aEnum[ec2types.SubnetState](),
	"tgw":      t1aEnum[ec2types.TransitGatewayState](),
	"eni":      t1aEnum[ec2types.NetworkInterfaceStatus](),
	"ec2":      t1aEnum[ec2types.InstanceStateName](),
	"ebs":      t1aEnum[ec2types.VolumeState](),
	"ami":      t1aEnum[ec2types.ImageState](),
	"elb":      t1aEnum[elbv2types.LoadBalancerStateEnum](),
	"alarm":    t1aEnum[cwtypes.StateValue](),
	"acm":      t1aEnum[acmtypes.CertificateStatus](),
	"mwaa":     t1aEnum[mwaatypes.EnvironmentStatus](),
	"transfer": t1aEnum[transfertypes.State](),
	"eb-rule":  t1aEnum[ebridgetypes.RuleState](),
	"athena":   t1aEnum[athenatypes.WorkGroupState](),
	"dbi":      {values: t1aDBIStatuses},
	"redshift": {values: t1aRedshiftStatuses},
}

// t1aStateScope is every top-level type whose AWS answer carries a lifecycle
// status.
var t1aStateScope = []string{
	"acm", "alarm", "ami", "asg", "athena", "cf", "cfn", "dbc", "dbi", "ddb",
	"eb", "eb-rule", "ebs", "ec2", "ecs", "ecs-svc", "ecs-task", "efs", "eks",
	"elb", "eni", "igw", "kinesis", "kms", "lambda", "msk", "mwaa", "nat", "ng",
	"opensearch", "redis", "redshift", "secrets", "subnet", "tgw", "transfer",
	"vpc", "vpce",
}

func t1aNonLive(c catalog.LifecycleClass) bool {
	return c == catalog.LifecycleTearingDown || c == catalog.LifecycleEnded || c == catalog.LifecyclePendingDeletion
}

func t1aTables(t *testing.T) []catalog.ResourceTypeDef {
	t.Helper()
	var out []catalog.ResourceTypeDef
	for _, td := range catalog.All() {
		if len(td.StateEnum) > 0 {
			out = append(out, td)
		}
	}
	if len(out) == 0 {
		t.Fatal("no type declares a state table")
	}
	return out
}

func t1aStateCodes(td catalog.ResourceTypeDef) map[domain.FindingCode]catalog.FindingDef {
	out := map[domain.FindingCode]catalog.FindingDef{}
	for _, def := range td.Findings {
		if len(def.States) > 0 {
			out[def.Code] = def
		}
	}
	return out
}

func t1aHas(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

// Each value AWS can report for a type's lifecycle field is decided in one
// place: it is a normal value or it raises exactly one state finding, and the
// table declares no value AWS never reports.
func TestT1aGuard_StateTablePartitionsTheEnum(t *testing.T) {
	declared := map[string]bool{}
	for _, td := range catalog.All() {
		declared[td.ShortName] = len(td.StateEnum) > 0
	}
	for _, s := range t1aStateScope {
		if !declared[s] {
			t.Errorf("%s reports a lifecycle status and declares no state table", s)
		}
	}

	for _, td := range t1aTables(t) {
		t.Run(td.ShortName, func(t *testing.T) {
			enum := map[string]bool{}
			for _, v := range td.StateEnum {
				enum[v] = true
			}
			if d, ok := t1aStateDomains[td.ShortName]; ok {
				want := slices.Sorted(slices.Values(d.values))
				got := slices.Sorted(slices.Values(td.StateEnum))
				if !slices.Equal(got, want) {
					t.Errorf("StateEnum = %q, want the AWS domain %q", got, want)
				}
			}

			owners := map[string][]string{}
			for _, v := range td.NormalStates {
				owners[v] = append(owners[v], "normal")
			}
			for _, def := range td.Findings {
				for _, v := range def.States {
					owners[v] = append(owners[v], string(def.Code))
				}
				if len(def.States) > 0 && def.Class == catalog.LifecycleNormal {
					t.Errorf("%s raises a finding for the Normal class", def.Code)
				}
			}
			for _, v := range td.StateEnum {
				if len(owners[v]) != 1 {
					t.Errorf("value %q is owned by %v, want exactly one of the normal values or one finding", v, owners[v])
				}
			}
			for v, by := range owners {
				if !enum[v] {
					t.Errorf("%v declares %q, which is not in the enum", by, v)
				}
			}
			if td.UnknownState == "" || !slices.ContainsFunc(td.Findings, func(d catalog.FindingDef) bool { return d.Code == td.UnknownState }) {
				t.Errorf("UnknownState %q is not one of the type's FindingDefs", td.UnknownState)
			}

			for _, v := range td.NormalStates {
				if class, code := catalog.Lifecycle(td, v); class != catalog.LifecycleNormal || code != "" {
					t.Errorf("Lifecycle(%q) = (%v, %q), want (Normal, \"\")", v, class, code)
				}
				if f, ok := awsclient.StateFinding(td.ShortName, v); ok {
					t.Errorf("StateFinding(%q) = %s, want none for a normal value", v, f.Code)
				}
			}
			for _, def := range t1aStateCodes(td) {
				for _, v := range def.States {
					if class, code := catalog.Lifecycle(td, v); class != def.Class || code != def.Code {
						t.Errorf("Lifecycle(%q) = (%v, %q), want (%v, %q)", v, class, code, def.Class, def.Code)
					}
					if got := awsclient.LifecycleClassOf(td.ShortName, v); got != def.Class {
						t.Errorf("LifecycleClassOf(%q) = %v, want %v", v, got, def.Class)
					}
					f, ok := awsclient.StateFinding(td.ShortName, v)
					if !ok || f.Code != def.Code || f.Severity != def.Severity || f.Phrase == "" {
						t.Errorf("StateFinding(%q) = (%+v, %v), want %s at %v with a phrase", v, f, ok, def.Code, def.Severity)
					}
				}
			}
		})
	}
}

// A value outside the enum is a state a9s does not know; the row says so with
// AWS's own word and keeps every other signal, because nothing says the
// resource is on its way out.
func TestT1aState_UnrecognisedValueRaisesTheTypesUnknownFinding(t *testing.T) {
	const raw = "quiescing"
	for _, td := range t1aTables(t) {
		t.Run(td.ShortName, func(t *testing.T) {
			if slices.Contains(td.StateEnum, raw) {
				t.Fatalf("%q is in the %s enum; pick another unknown value", raw, td.ShortName)
			}
			class, code := catalog.Lifecycle(td, raw)
			if code != td.UnknownState {
				t.Errorf("Lifecycle(%q) code = %q, want UnknownState %q", raw, code, td.UnknownState)
			}
			if t1aNonLive(class) {
				t.Errorf("Lifecycle(%q) class = %v; an unknown state must not drop the row's other signals", raw, class)
			}
			f, ok := awsclient.StateFinding(td.ShortName, raw)
			if !ok {
				t.Fatalf("StateFinding(%q) raised nothing", raw)
			}
			if f.Code != td.UnknownState {
				t.Errorf("StateFinding(%q).Code = %q, want %q", raw, f.Code, td.UnknownState)
			}
			if !strings.Contains(strings.ToLower(f.Phrase), raw) {
				t.Errorf("StateFinding(%q).Phrase = %q, want it to name %q", raw, f.Phrase, raw)
			}
			if !f.Severity.IsIssue() {
				t.Errorf("StateFinding(%q).Severity = %v, want Warning or Broken", raw, f.Severity)
			}
		})
	}
}

// Each lifecycle class has one default severity, so one state reads the same
// colour on every type; a FindingDef that departs from it says so, and the
// departures are listed here.
func TestT1aGuard_SeverityDeviationsAreDeclared(t *testing.T) {
	var deviations []string
	n := 0
	for _, td := range t1aTables(t) {
		for _, def := range t1aStateCodes(td) {
			n++
			dflt := def.Class.DefaultSeverity()
			deviates := def.Severity != dflt
			if deviates != def.SeverityDeviates {
				t.Errorf("%s: severity %v, class %v defaults to %v, SeverityDeviates = %v", def.Code, def.Severity, def.Class, dflt, def.SeverityDeviates)
			}
			if deviates {
				deviations = append(deviations, fmt.Sprintf("%s: %v instead of %v", def.Code, def.Severity, dflt))
			}
		}
	}
	if n == 0 {
		t.Fatal("no state FindingDef found")
	}
	sort.Strings(deviations)
	for _, d := range deviations {
		t.Logf("declared deviation %s", d)
	}
}

// The lifecycle field of every demo row holds the value AWS reported, and the
// row carries the state finding the table names for it. A value outside the
// enum is one the table does not know; such a row carries the type's
// unrecognised-state finding.
func TestT1aState_LifecycleFieldHoldsTheAWSValue(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	checked := 0
	for _, td := range t1aTables(t) {
		codes := t1aStateCodes(td)
		for _, r := range byType[td.ShortName] {
			if r.Fields[awsclient.DegradedFindingField] != "" {
				continue
			}
			raw := r.Fields[td.StatusKey()]
			if raw == "" {
				continue
			}
			checked++
			if !slices.Contains(td.StateEnum, raw) && !t1aHas(r.Findings, td.UnknownState) {
				t.Errorf("%s/%s Fields[%q] = %q, not a value AWS reports, and the row lacks %s", td.ShortName, r.ID, td.StatusKey(), raw, td.UnknownState)
				continue
			}
			_, want := catalog.Lifecycle(td, raw)
			for _, f := range r.Findings {
				if _, isState := codes[f.Code]; isState && f.Code != want {
					t.Errorf("%s/%s (%s) carries state finding %s, want %q", td.ShortName, r.ID, raw, f.Code, want)
				}
			}
			if want != "" && !t1aHas(r.Findings, want) {
				t.Errorf("%s/%s (%s) lacks its state finding %s", td.ShortName, r.ID, raw, want)
			}
		}
	}
	if checked < 100 {
		t.Fatalf("checked %d demo rows, want at least 100", checked)
	}
}

func t1aWave2(t *testing.T, short string, rows []resource.Resource, fail func(service, op string, in any) error) (runtime.ProbeEnrichmentResult, []t576Call) {
	t.Helper()
	clients := demo.NewServiceClients()
	inj := &t576Injector{fail: fail}
	t576WrapClients(clients, inj)
	res := unit.ProbeWave2(t, short, clients, rows)
	return res, inj.calls
}

func t1aCallsNaming(calls []t576Call, r resource.Resource) []string {
	var out []string
	for _, c := range calls {
		if t576Names(t576Leaves(c.In), r) {
			out = append(out, t576CallKey(c))
		}
	}
	return out
}

// A row tearing down, ended or pending deletion has nothing left for Wave 2 to
// find. The runner drops it before any enricher and before any read the
// enricher declares, so a sweep over such a row alone makes no AWS call. The
// row is shaped as the disk cache restores it: Fields only.
func TestT1aGuard_NonLiveRowsMakeNoWave2Calls(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	denyAll := func(service, op string, _ any) error { return t576Denied(service, op) }
	falsifiable, pairs := 0, 0
	for _, td := range t1aTables(t) {
		enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
		if !ok {
			continue
		}
		rows := byType[td.ShortName]
		idx := slices.IndexFunc(rows, func(r resource.Resource) bool {
			class, _ := catalog.Lifecycle(td, r.Fields[td.StatusKey()])
			return !t1aNonLive(class) && r.Fields[awsclient.DegradedFindingField] == ""
		})
		if idx < 0 {
			t.Errorf("%s: no live demo row to build from", td.ShortName)
			continue
		}
		if _, calls := t1aWave2(t, td.ShortName, rows[idx:idx+1], nil); len(calls) > 0 {
			falsifiable++
		}
		for _, def := range t1aStateCodes(td) {
			if !t1aNonLive(def.Class) {
				continue
			}
			for _, v := range def.States {
				pairs++
				row := rows[idx]
				row.Fields = map[string]string{}
				for k, val := range rows[idx].Fields {
					row.Fields[k] = val
				}
				row.Fields[td.StatusKey()] = v
				row.RawStruct = nil
				row.Findings = []domain.Finding{{Code: def.Code, Phrase: def.Phrase, Severity: def.Severity, Source: def.Source}}
				_, calls := t1aWave2(t, td.ShortName, []resource.Resource{row}, denyAll)
				// A pass that declares ReadsNonLive answers only once a
				// resource stops, so the runner hands it the row and it reads.
				if enricher.ReadsNonLive {
					if len(calls) == 0 {
						t.Errorf("%s row in %q (%v): the pass declares ReadsNonLive and made no call", td.ShortName, v, def.Class)
					}
					continue
				}
				if len(calls) > 0 {
					keys := make([]string, 0, len(calls))
					for _, c := range calls {
						keys = append(keys, t576CallKey(c))
					}
					t.Errorf("%s row in %q (%v): Wave 2 made %d calls: %v", td.ShortName, v, def.Class, len(calls), keys)
				}
			}
		}
	}
	if pairs == 0 {
		t.Fatal("no non-live state value declared on any type with a Wave 2 enricher")
	}
	if falsifiable < 10 {
		t.Fatalf("only %d types make a Wave 2 call for a live row; the guard cannot fail", falsifiable)
	}
	t.Logf("%d non-live values over %d falsifiable types", pairs, falsifiable)
}

// DescribeContinuousBackups and the backup-coverage join are about a table
// that will keep its data; a table being deleted or archived gets neither,
// and is not marked as uninspected for it. A live table is still read.
func TestT1aDDB_EndedTablesGetNoWave2Read(t *testing.T) {
	td := resource.FindResourceType("ddb")
	if td == nil {
		t.Fatal("ddb is not registered")
	}
	rows, ok := unit.DrainFixtures(t, *td, demo.NewServiceClients())
	if !ok {
		t.Fatal("the ddb demo fixtures drained no rows")
	}
	res, calls := t1aWave2(t, "ddb", rows, nil)
	seen := map[ddbtypes.TableStatus]bool{}
	liveRead := false
	for _, r := range rows {
		tb, ok := r.RawStruct.(*ddbtypes.TableDescription)
		if !ok {
			continue
		}
		switch tb.TableStatus {
		case ddbtypes.TableStatusDeleting, ddbtypes.TableStatusArchived:
			seen[tb.TableStatus] = true
			if named := t1aCallsNaming(calls, r); len(named) > 0 {
				t.Errorf("%s (%s): Wave 2 calls %v", r.ID, tb.TableStatus, named)
			}
			if got := res.Findings[r.ID]; len(got) != 0 {
				t.Errorf("%s (%s): Wave 2 findings %v, want none", r.ID, tb.TableStatus, got)
			}
			if reason, marked := res.TruncatedIDs[r.ID]; marked {
				t.Errorf("%s (%s): marked not inspected (%q)", r.ID, tb.TableStatus, reason)
			}
		case ddbtypes.TableStatusActive:
			for _, c := range calls {
				if c.Op == "DescribeContinuousBackups" && t576Names(t576Leaves(c.In), r) {
					liveRead = true
				}
			}
		}
	}
	if !seen[ddbtypes.TableStatusDeleting] || !seen[ddbtypes.TableStatusArchived] {
		t.Fatalf("demo ddb rows cover %v, want DELETING and ARCHIVED", seen)
	}
	if !liveRead {
		t.Errorf("no ACTIVE demo table was asked DescribeContinuousBackups; the comparison proves nothing")
	}
}

type t1aRDSInstances struct{ db rdstypes.DBInstance }

func (f t1aRDSInstances) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{f.db}}, nil
}

type t1aRDSCluster struct{ c rdstypes.DBCluster }

func (f t1aRDSCluster) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: []rdstypes.DBCluster{f.c}}, nil
}

type t1aDocDBCluster struct{ c docdbtypes.DBCluster }

func (f t1aDocDBCluster) DescribeDBClusters(context.Context, *docdb.DescribeDBClustersInput, ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: []docdbtypes.DBCluster{f.c}}, nil
}

type t1aDDBTable struct{ tb *ddbtypes.TableDescription }

func (f t1aDDBTable) ListTables(context.Context, *dynamodb.ListTablesInput, ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return &dynamodb.ListTablesOutput{TableNames: []string{aws.ToString(f.tb.TableName)}}, nil
}

func (f t1aDDBTable) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{Table: f.tb}, nil
}

type t1aVolume struct{ v ec2types.Volume }

func (f t1aVolume) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{f.v}}, nil
}

// t1aRefetch lists a demo row again through its fetcher with only its
// lifecycle field changed, as AWS would answer once the resource moved on.
func t1aRefetch(t *testing.T, r resource.Resource, status string) resource.Resource {
	t.Helper()
	ctx := context.Background()
	var res resource.FetchResult
	var err error
	switch raw := r.RawStruct.(type) {
	case rdstypes.DBInstance:
		raw.DBInstanceStatus = aws.String(status)
		res, err = awsclient.FetchRDSInstancesPage(ctx, t1aRDSInstances{raw}, "")
	case rdstypes.DBCluster:
		raw.Status = aws.String(status)
		res, err = awsclient.FetchRDSDBClustersPage(ctx, t1aRDSCluster{raw}, "")
	case docdbtypes.DBCluster:
		raw.Status = aws.String(status)
		res, err = awsclient.FetchDocDBClustersPage(ctx, t1aDocDBCluster{raw}, "")
	case *ddbtypes.TableDescription:
		tb := *raw
		tb.TableStatus = ddbtypes.TableStatus(status)
		res, err = awsclient.FetchDynamoDBTablesPage(ctx, t1aDDBTable{&tb}, t1aDDBTable{&tb}, "")
	case ec2types.Volume:
		raw.State = ec2types.VolumeState(status)
		res, err = awsclient.FetchEBSVolumesPage(ctx, t1aVolume{raw}, "")
	default:
		t.Fatalf("%s: no refetch for RawStruct %T", r.ID, r.RawStruct)
	}
	if err != nil {
		t.Fatalf("%s: refetch: %v", r.ID, err)
	}
	for _, got := range res.Resources {
		if got.ID == r.ID {
			return got
		}
	}
	t.Fatalf("%s: refetch returned %d rows, none with its ID", r.ID, len(res.Resources))
	return resource.Resource{}
}

// A resource being deleted has no backup plan to join; the coverage finding
// and the per-row tag read it needs are skipped for it, while the same row
// live is judged.
func TestT1aBackupCoverage_TearingDownRowsAreNotJudged(t *testing.T) {
	for _, tc := range []struct {
		short, status string
		code          domain.FindingCode
	}{
		{"dbi", "deleting", awsclient.CodeDBINotInBackupPlan},
		{"dbc", "deleting", awsclient.CodeDBCNotInBackupPlan},
		{"ddb", string(ddbtypes.TableStatusDeleting), awsclient.CodeDDBNotInBackupPlan},
		{"ebs", string(ec2types.VolumeStateDeleting), awsclient.CodeEBSNotInBackupPlan},
	} {
		t.Run(tc.short, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s is not registered", tc.short)
			}
			rows, ok := unit.DrainFixtures(t, *td, demo.NewServiceClients())
			if !ok {
				t.Fatalf("the %s demo fixtures drained no rows", tc.short)
			}
			base, _ := t1aWave2(t, tc.short, rows, nil)
			idx := slices.IndexFunc(rows, func(r resource.Resource) bool { return t1aHas(base.Findings[r.ID], tc.code) })
			if idx < 0 {
				t.Fatalf("no live demo %s row raises %s; the comparison proves nothing", tc.short, tc.code)
			}
			gone := t1aRefetch(t, rows[idx], tc.status)
			if got := gone.Fields[td.StatusKey()]; got != tc.status {
				t.Errorf("%s refetched in %q: Fields[%q] = %q", gone.ID, tc.status, td.StatusKey(), got)
			}
			swapped := slices.Clone(rows)
			swapped[idx] = gone
			res, calls := t1aWave2(t, tc.short, swapped, nil)
			if t1aHas(res.Findings[gone.ID], tc.code) {
				t.Errorf("%s in %q carries %s", gone.ID, tc.status, tc.code)
			}
			if named := t1aCallsNaming(calls, gone); len(named) > 0 {
				t.Errorf("%s in %q: Wave 2 calls %v", gone.ID, tc.status, named)
			}
		})
	}
}

// Lambda reports eight function states; each one other than Active is a
// state the operator has to see, and a healthy Active function is blank.
func TestT1aLambda_EveryNonActiveStateIsVisible(t *testing.T) {
	const healthy = "acme-orders-api"
	fns := []lambdatypes.FunctionConfiguration{t561Fn(healthy, lambdatypes.RuntimePython312)}
	states := []lambdatypes.State{
		lambdatypes.StateDeactivating, lambdatypes.StateDeactivated,
		lambdatypes.StateActiveNonInvocable, lambdatypes.StateDeleting,
	}
	for _, s := range states {
		fn := t561Fn("acme-fn-"+strings.ToLower(string(s)), lambdatypes.RuntimePython312)
		fn.State = s
		fns = append(fns, fn)
	}
	rows, _ := t561LambdaRows(t, &t561LambdaFake{fns: fns})
	td := resource.FindResourceType("lambda")
	for _, s := range states {
		id := "acme-fn-" + strings.ToLower(string(s))
		r := t561Row(t, rows, id)
		_, code := catalog.Lifecycle(*td, string(s))
		if code == "" {
			t.Errorf("%s: the lambda state table raises no finding for %s", id, s)
			continue
		}
		if !t1aHas(r.Findings, code) {
			t.Errorf("%s (%s): findings %v lack %s", id, s, r.Findings, code)
		}
		cell, ok := listStatusCellFor(t, *td, rows, id)
		if !ok {
			t.Fatalf("%s: no list row", id)
		}
		if top, _ := domain.TopFinding(r.Findings); cell == "" || !strings.HasPrefix(cell, top.Phrase) {
			t.Errorf("%s (%s): Status cell = %q, want the phrase %q", id, s, cell, top.Phrase)
		}
	}
	if r := t561Row(t, rows, healthy); len(r.Findings) != 0 {
		t.Errorf("%s: findings %v, want none", healthy, r.Findings)
	}
	if cell, _ := listStatusCellFor(t, *td, rows, healthy); cell != "" {
		t.Errorf("%s (Active): Status cell = %q, want blank", healthy, cell)
	}
}

func t1aNormalRaw(r resource.Resource) (string, bool) {
	switch v := r.RawStruct.(type) {
	case elasticachetypes.ReplicationGroup:
		return aws.ToString(v.Status), aws.ToString(v.Status) == "available"
	case rdstypes.DBCluster:
		return aws.ToString(v.Status), aws.ToString(v.Status) == "available"
	case docdbtypes.DBCluster:
		return aws.ToString(v.Status), aws.ToString(v.Status) == "available"
	case *mwaatypes.Environment:
		return string(v.Status), v.Status == mwaatypes.EnvironmentStatusAvailable
	case *ec2types.VpcPeeringConnection:
		if v.Status == nil {
			return "", false
		}
		return string(v.Status.Code), v.Status.Code == ec2types.VpcPeeringConnectionStateReasonCodeActive
	}
	return "", false
}

func t1aDetailValue(t *testing.T, r resource.Resource, short, key string) (string, bool) {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(r, short)
	body := c.Snapshot().Body.Detail
	if body == nil {
		return "", false
	}
	for _, f := range body.Fields {
		if f.Key == key && f.Path != "Attention" && !f.IsSection && !f.IsHeader {
			return f.Value, true
		}
	}
	return "", false
}

// The detail Status row is the AWS state of the resource; the finding phrase
// belongs to the Attention section and the list's Status cell.
func TestT1aDetail_StatusRowShowsTheAWSValue(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	for _, short := range []string{"redis", "dbc", "mwaa", "vpc-peer"} {
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Fatalf("%s is not registered", short)
			}
			rows := mergeWave2Findings(t, *td, byType[short], cache, clients)
			idx := slices.IndexFunc(rows, func(r resource.Resource) bool {
				_, normal := t1aNormalRaw(r)
				return normal && len(r.Findings) > 0
			})
			if idx < 0 {
				t.Fatalf("no demo %s row is in its normal state with a posture finding", short)
			}
			r := rows[idx]
			raw, _ := t1aNormalRaw(r)
			got, ok := t1aDetailValue(t, r, short, "Status")
			if !ok {
				t.Fatalf("%s: detail has no Status row", r.ID)
			}
			if got != raw && got != domain.HumanizeStatusPhrase(raw) {
				top, _ := domain.TopFinding(r.Findings)
				t.Errorf("%s: detail Status = %q, want the AWS value %q (top finding phrase %q)", r.ID, got, raw, top.Phrase)
			}
		})
	}
}

// A row in a live state other than normal is where posture can be dropped;
// the demo carries one such dbi, dbc and redshift row, and it renders its top
// finding with the posture finding beside the state.
func TestT1aDemo_LiveNonNormalRowKeepsPostureBesideItsState(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	for _, short := range []string{"dbi", "dbc", "redshift"} {
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Fatalf("%s is not registered", short)
			}
			codes := t1aStateCodes(*td)
			rows := mergeWave2Findings(t, *td, byType[short], cache, clients)
			idx := slices.IndexFunc(rows, func(r resource.Resource) bool {
				live, posture := false, false
				for _, f := range r.Findings {
					def, isState := codes[f.Code]
					switch {
					case isState && def.Class != catalog.LifecycleNormal && !t1aNonLive(def.Class):
						live = true
					case !isState && f.Code != td.UnknownState:
						posture = true
					}
				}
				return live && posture
			})
			if idx < 0 {
				t.Fatalf("no demo %s row carries a live non-normal state finding and a posture finding", short)
			}
			t563AssertWitnessRendered(t, *td, rows, rows[idx])
		})
	}
}

// The Status cell text comes from Findings through the one render cascade,
// so no fetcher in core/aws stores a finding phrase in Fields; which rows
// Wave 2 skips is the runner's answer from the state table alone.
func TestT1aGuard_NoPhraseInFieldsAndNoTeardownFile(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	if _, err := os.Stat(filepath.Join(dir, "teardown.go")); err == nil {
		t.Errorf("core/aws/teardown.go still exists")
	}
	fset := token.NewFileSet()
	// transfer_children.go builds the rows of the transfer_agreements child
	// kind; the guard covers the fetchers of top-level types.
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") && fi.Name() != "transfer_children.go"
	}, 0)
	if err != nil {
		t.Fatalf("parse core/aws: %v", err)
	}
	var sites []string
	for _, p := range pkgs {
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "StatusPhrase" {
					pos := fset.Position(call.Pos())
					sites = append(sites, fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line))
				}
				return true
			})
		}
	}
	sort.Strings(sites)
	if len(sites) > 0 {
		t.Errorf("core/aws stores a finding phrase at %d sites: %v", len(sites), sites)
	}
}

// Only the state table decides what a lifecycle value means. A switch case or
// an equality test against a constant of a lifecycle enum outside it is a
// second answer to the same question.
func TestT1aGuard_NoLifecycleEnumComparedOutsideTheTable(t *testing.T) {
	enums := map[string]string{}
	for short, d := range t1aStateDomains {
		if d.enum != nil {
			enums[d.enum.PkgPath()+"."+d.enum.Name()] = short
		}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := packages.Load(cfg, "./core/aws", "./core/runtime", "./core/app")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sites := map[string]bool{}
	files := 0
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("%s: %v", p.PkgPath, e)
		}
		files += len(p.Syntax)
		enumConst := func(e ast.Expr) (string, bool) {
			tv, ok := p.TypesInfo.Types[e]
			if !ok || tv.Value == nil {
				return "", false
			}
			named, ok := tv.Type.(*types.Named)
			if !ok || named.Obj().Pkg() == nil {
				return "", false
			}
			short, ok := enums[named.Obj().Pkg().Path()+"."+named.Obj().Name()]
			return short, ok
		}
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				var hits []ast.Expr
				switch x := n.(type) {
				case *ast.CaseClause:
					hits = x.List
				case *ast.BinaryExpr:
					if x.Op == token.EQL || x.Op == token.NEQ {
						hits = []ast.Expr{x.X, x.Y}
					}
				}
				for _, e := range hits {
					if short, ok := enumConst(e); ok {
						pos := p.Fset.Position(e.Pos())
						sites[fmt.Sprintf("%s:%d (%s)", filepath.Base(pos.Filename), pos.Line, short)] = true
					}
				}
				return true
			})
		}
	}
	if files < 300 {
		t.Fatalf("scanned %d files; the load lost core/aws", files)
	}
	if len(sites) > 0 {
		list := slices.Sorted(maps.Keys(sites))
		t.Errorf("%d lines compare against a lifecycle enum outside the state table: %v", len(list), list)
	}
}
