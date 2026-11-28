package unit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
	"gopkg.in/yaml.v3"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
)

// t9cListRowTypes is, per type, every RawStruct type a fetcher or a child
// fetcher stores on a demo row, before any detail enricher runs.
func t9cListRowTypes(t *testing.T) map[string]map[reflect.Type]int {
	t.Helper()
	rows, enrichedIdx, _ := t9bDevTreeRowsEnriched(t)
	out := map[string]map[reflect.Type]int{}
	for i, r := range rows {
		if enrichedIdx[i] || r.res.RawStruct == nil {
			continue
		}
		if out[r.typ] == nil {
			out[r.typ] = map[reflect.Type]int{}
		}
		out[r.typ][reflect.TypeOf(r.res.RawStruct)]++
	}
	return out
}

func t9cDeclares(td catalog.ResourceTypeDef, rt reflect.Type) bool {
	for _, d := range t9cRowTypes(td) {
		if t9cDeref(d) == t9cDeref(rt) {
			return true
		}
	}
	return false
}

// Every catalog type and child kind declares the struct types its rows
// carry, and every row a fetcher stores carries one of them: a detail or
// column path can only be checked against a struct the type names.
func TestT9c_Row1_EveryRowStructTypeIsDeclared(t *testing.T) {
	stored := t9cListRowTypes(t)
	var problems []string
	for _, td := range t9cAllDefs() {
		declared := t9cRowTypes(td)
		if len(declared) == 0 {
			problems = append(problems, fmt.Sprintf("type %s declares no row struct types", td.ShortName))
			continue
		}
		for rt, n := range stored[td.ShortName] {
			if !t9cDeclares(td, rt) {
				problems = append(problems, fmt.Sprintf("type %s: %d demo rows carry %v, which RowTypes %v does not name", td.ShortName, n, rt, declared))
			}
		}
	}
	if len(stored) < 90 {
		t.Fatalf("the demo walk reached %d types; the gate needs the whole bench", len(stored))
	}
	t9bReport(t, "row struct types not declared", problems)
}

func t9cPathProblems(td catalog.ResourceTypeDef, types []reflect.Type) []string {
	onAny := func(path string) bool {
		for _, rt := range types {
			if t9cResolves(rt, path) {
				return true
			}
		}
		return t9cComputed(td, path)
	}
	vd := config.DefaultViewDef(td.ShortName)
	var out []string
	for _, c := range vd.List {
		if c.Path != "" && !onAny(c.Path) {
			out = append(out, fmt.Sprintf("%s: column %q path %s names no field of any of %v", td.ShortName, c.Title, c.Path, types))
		}
	}
	for _, d := range vd.Detail {
		if d.Path != "" && !onAny(d.Path) {
			out = append(out, fmt.Sprintf("%s: detail path %s names no field of any of %v", td.ShortName, d.Path, types))
		}
	}
	return out
}

// Every default column path and detail path names a field of at least one
// struct the type declares, or the type declares it computed. A field one
// lane lacks is blank on that lane's rows, which is true; a path no declared
// struct has is blank on every row.
func TestT9c_Row2_DefaultPathsResolveOnADeclaredStruct(t *testing.T) {
	var problems []string
	for _, td := range t9cAllDefs() {
		declared := t9cRowTypes(td)
		if len(declared) == 0 {
			problems = append(problems, fmt.Sprintf("type %s declares no row struct types", td.ShortName))
			continue
		}
		problems = append(problems, t9cPathProblems(td, declared)...)
	}
	t9bReport(t, "default paths that resolve on no declared struct", problems)
}

// A wrapper row is a struct of its own, and a type with two lanes stores one
// struct per lane: each is declared, and the named path, a field of one of
// them, is read from the struct rather than declared computed.
func TestT9c_Row2_ReviewDisagreementsAreGone(t *testing.T) {
	cases := []struct {
		name, typ string
		stored    []reflect.Type
		path      string
	}{
		{"sqs attributes row", "sqs", []reflect.Type{reflect.TypeFor[awsclient.SQSQueueAttributesRow]()}, "QueueUrl"},
		{"apigw REST and HTTP", "apigw", []reflect.Type{reflect.TypeFor[apigwv1types.RestApi](), reflect.TypeFor[apigwv2types.Api]()}, "RouteSelectionExpression"},
		{"dbc docdb and rds", "dbc", []reflect.Type{reflect.TypeFor[docdbtypes.DBCluster](), reflect.TypeFor[rdstypes.DBCluster]()}, "DBClusterMembers"},
		{"dbc-snap docdb and rds", "dbc-snap", []reflect.Type{reflect.TypeFor[docdbtypes.DBClusterSnapshot](), reflect.TypeFor[rdstypes.DBClusterSnapshot]()}, "StorageType"},
		{"transfer described and listed", "transfer", []reflect.Type{reflect.TypeFor[*transfertypes.DescribedServer](), reflect.TypeFor[*transfertypes.ListedServer]()}, "Protocols"},
		{"transfer_agreements described and listed", "transfer_agreements", []reflect.Type{reflect.TypeFor[*transfertypes.DescribedAgreement](), reflect.TypeFor[*transfertypes.ListedAgreement]()}, "BaseDirectory"},
		{"athena wrapper row", "athena", []reflect.Type{reflect.TypeFor[awsclient.AthenaWorkGroupRow]()}, "EngineVersion"},
		{"backup wrapper row", "backup", []reflect.Type{reflect.TypeFor[awsclient.BackupPlanRow]()}, "AdvancedBackupSettings"},
		{"s3_objects object and prefix", "s3_objects", []reflect.Type{reflect.TypeFor[s3types.Object](), reflect.TypeFor[s3types.CommonPrefix]()}, "LastModified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := catalog.FindAny(tc.typ)
			if td == nil {
				t.Fatalf("no catalog type %s", tc.typ)
			}
			for _, rt := range tc.stored {
				if !t9cDeclares(*td, rt) {
					t.Errorf("type %s does not declare %v, the struct its fetcher stores (RowTypes %v)", tc.typ, rt, t9cRowTypes(*td))
				}
			}
			resolves := false
			for _, rt := range tc.stored {
				resolves = resolves || t9cResolves(rt, tc.path)
			}
			if !resolves {
				t.Errorf("%s: path %s names no field of any stored struct %v", tc.typ, tc.path, tc.stored)
			}
			if t9cComputed(*td, tc.path) {
				t.Errorf("%s declares %s computed, but it is a field of a stored struct", tc.typ, tc.path)
			}
			if declared := t9cRowTypes(*td); len(declared) > 0 {
				for _, p := range t9cPathProblems(*td, declared) {
					t.Error(p)
				}
			}
		})
	}
}

// The sqs row struct carries QueueURL and Attributes as fields, so the type
// does not declare those two paths computed: the path reads the struct.
func TestT9c_Row2_SQSDeclaresNoComputedPathTheStructHas(t *testing.T) {
	td := catalog.FindAny("sqs")
	if td == nil {
		t.Fatal("no catalog type sqs")
	}
	rt := reflect.TypeFor[awsclient.SQSQueueAttributesRow]()
	for _, p := range td.ComputedDetailPaths {
		if t9cResolves(rt, p) {
			t.Errorf("sqs declares %s computed, but it is a field of %v", p, rt)
		}
	}
}

// views_reference.yaml lists, for sqs, only paths an operator can name and
// see filled: paths of the struct the sqs fetcher stores. An attribute name
// such as ApproximateNumberOfMessages is a key of the Attributes map, which
// a path cannot enter.
func TestT9c_Row3_SQSReferenceListsOnlyPathsOfTheStoredRow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(t9bRepoRoot(t), "core", "config", "views_reference.yaml"))
	if err != nil {
		t.Fatalf("reading views_reference.yaml: %v", err)
	}
	var ref map[string][]string
	if err := yaml.Unmarshal(data, &ref); err != nil {
		t.Fatalf("parsing views_reference.yaml: %v", err)
	}
	paths, ok := ref["sqs"]
	if !ok || len(paths) == 0 {
		t.Fatal("views_reference.yaml has no sqs section")
	}
	rt := reflect.TypeFor[awsclient.SQSQueueAttributesRow]()
	var bad []string
	for _, p := range paths {
		if !t9cResolves(rt, strings.ReplaceAll(p, "[]", "")) {
			bad = append(bad, p)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("views_reference.yaml offers %d sqs paths that name no field of %v: %s", len(bad), rt, strings.Join(bad, ", "))
	}
	for _, want := range []string{"QueueURL", "QueueName", "Attributes"} {
		found := false
		for _, p := range paths {
			first, _, _ := strings.Cut(p, ".")
			if strings.EqualFold(strings.TrimRight(first, "[]{}"), want) {
				found = true
			}
		}
		if !found {
			t.Errorf("views_reference.yaml sqs section lacks %s, a field of the stored row", want)
		}
	}
}
