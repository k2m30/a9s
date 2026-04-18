package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// Issue #23: Name column must be first in all default list views
//
// These tests verify that for 14 resource types, the human-readable name
// column comes first in BOTH:
//   1. config.DefaultViewDef(shortName).List[0] — the config default
//   2. resource.FindResourceType(shortName).Columns[0] — the type definition
//
// The tests are written to FAIL against the current code (TDD) and will
// pass once the column order is swapped in defaults and type definitions.
// ===========================================================================

// resourceColumnSpec defines the expected first column for a resource type
// after the fix is applied.
type resourceColumnSpec struct {
	shortName         string
	configFirstTitle  string // expected config.DefaultViewDef(...).List[0].Title
	typeDefFirstTitle string // expected resource.FindResourceType(...).Columns[0].Title
	typeDefFirstKey   string // expected resource.FindResourceType(...).Columns[0].Key
}

// affectedResources lists all 14 resource types that need name-first columns.
// sg and vpc already have name-first in config defaults but NOT in type defs.
var affectedResources = []resourceColumnSpec{
	// Networking (10 resources)
	{shortName: "sg", configFirstTitle: "Group Name", typeDefFirstTitle: "Group Name", typeDefFirstKey: "group_name"},
	{shortName: "vpc", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "subnet", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "rtb", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "nat", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "igw", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "eip", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "vpce", configFirstTitle: "Service Name", typeDefFirstTitle: "Service Name", typeDefFirstKey: "service_name"},
	{shortName: "tgw", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "eni", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	// DNS/CDN (3 resources)
	{shortName: "r53", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	{shortName: "cf", configFirstTitle: "Domain Name", typeDefFirstTitle: "Domain Name", typeDefFirstKey: "domain_name"},
	{shortName: "apigw", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
	// Databases (1 resource)
	{shortName: "efs", configFirstTitle: "Name", typeDefFirstTitle: "Name", typeDefFirstKey: "name"},
}

// ---------------------------------------------------------------------------
// Config Defaults Tests: DefaultViewDef(...).List[0].Title must be name column
// ---------------------------------------------------------------------------

func TestConfigDefaultViewDef_NameColumnFirst(t *testing.T) {
	for _, spec := range affectedResources {
		t.Run(spec.shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(spec.shortName)
			if len(vd.List) == 0 {
				t.Fatalf("config.DefaultViewDef(%q) returned empty List", spec.shortName)
			}
			if vd.List[0].Title != spec.configFirstTitle {
				t.Errorf("config.DefaultViewDef(%q).List[0].Title = %q, want %q",
					spec.shortName, vd.List[0].Title, spec.configFirstTitle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Resource Type Tests: FindResourceType(...).Columns[0] must be name column
// ---------------------------------------------------------------------------

func TestResourceTypeDef_NameColumnFirst(t *testing.T) {
	for _, spec := range affectedResources {
		t.Run(spec.shortName, func(t *testing.T) {
			rt := resource.FindResourceType(spec.shortName)
			if rt == nil {
				t.Fatalf("resource.FindResourceType(%q) returned nil", spec.shortName)
			}
			if len(rt.Columns) == 0 {
				t.Fatalf("resource type %q has no columns", spec.shortName)
			}
			if rt.Columns[0].Title != spec.typeDefFirstTitle {
				t.Errorf("resource type %q Columns[0].Title = %q, want %q",
					spec.shortName, rt.Columns[0].Title, spec.typeDefFirstTitle)
			}
			if rt.Columns[0].Key != spec.typeDefFirstKey {
				t.Errorf("resource type %q Columns[0].Key = %q, want %q",
					spec.shortName, rt.Columns[0].Key, spec.typeDefFirstKey)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Column count preservation: swapping columns must not add or remove columns
// ---------------------------------------------------------------------------

// expectedConfigColumnCounts lists the number of columns each resource type
// should have in its YAML-driven config default view. These counts include
// attention columns added in Round 1 (nat +1 Failure, sg +1 Open).
var expectedConfigColumnCounts = map[string]int{
	"sg":     5,
	"vpc":    5,
	"subnet": 8,
	"rtb":    5,
	"nat":    7,
	"igw":    4,
	"eip":    6,
	"vpce":   6,
	"tgw":    5,
	"eni":    6,
	"r53":    5,
	"cf":     8,
	"apigw":  5,
	"efs":    6,
}

// expectedTypeDefColumnCounts lists the number of columns in the Go type
// definition (resource.FindResourceType). These may differ from config counts
// when YAML views have been updated ahead of the type definitions.
var expectedTypeDefColumnCounts = map[string]int{
	"sg":     4,
	"vpc":    5,
	"subnet": 7,
	"rtb":    5,
	"nat":    6,
	"igw":    4,
	"eip":    6,
	"vpce":   5,
	"tgw":    5,
	"eni":    6,
	"r53":    5,
	"cf":     6,
	"apigw":  5,
	"efs":    6,
}

func TestConfigDefaultViewDef_ColumnCountPreserved(t *testing.T) {
	for shortName, wantCount := range expectedConfigColumnCounts {
		t.Run(shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(shortName)
			if len(vd.List) != wantCount {
				t.Errorf("config.DefaultViewDef(%q) has %d columns, want %d",
					shortName, len(vd.List), wantCount)
			}
		})
	}
}

func TestResourceTypeDef_ColumnCountPreserved(t *testing.T) {
	for shortName, wantCount := range expectedTypeDefColumnCounts {
		t.Run(shortName, func(t *testing.T) {
			rt := resource.FindResourceType(shortName)
			if rt == nil {
				t.Fatalf("resource.FindResourceType(%q) returned nil", shortName)
			}
			if len(rt.Columns) != wantCount {
				t.Errorf("resource type %q has %d columns, want %d",
					shortName, len(rt.Columns), wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Second column tests: the old first column should now be second
// ---------------------------------------------------------------------------

// expectedSecondColumn lists what the second column should be after the swap.
// This is the column that was previously first (the ID column).
var expectedSecondColumn = map[string]struct {
	configTitle  string
	typeDefTitle string
	typeDefKey   string
}{
	"sg":    {configTitle: "Group ID", typeDefTitle: "Group ID", typeDefKey: "group_id"},
	"vpc":   {configTitle: "VPC ID", typeDefTitle: "VPC ID", typeDefKey: "vpc_id"},
	"subnet": {configTitle: "Subnet ID", typeDefTitle: "Subnet ID", typeDefKey: "subnet_id"},
	"rtb":   {configTitle: "Route Table ID", typeDefTitle: "Route Table ID", typeDefKey: "route_table_id"},
	"nat":   {configTitle: "NAT Gateway ID", typeDefTitle: "NAT Gateway ID", typeDefKey: "nat_gateway_id"},
	"igw":   {configTitle: "IGW ID", typeDefTitle: "IGW ID", typeDefKey: "igw_id"},
	"eip":   {configTitle: "Allocation ID", typeDefTitle: "Allocation ID", typeDefKey: "allocation_id"},
	"vpce":  {configTitle: "Endpoint ID", typeDefTitle: "Endpoint ID", typeDefKey: "vpce_id"},
	"tgw":   {configTitle: "TGW ID", typeDefTitle: "TGW ID", typeDefKey: "tgw_id"},
	"eni":   {configTitle: "ENI ID", typeDefTitle: "ENI ID", typeDefKey: "eni_id"},
	"r53":   {configTitle: "Zone ID", typeDefTitle: "Zone ID", typeDefKey: "zone_id"},
	"cf":    {configTitle: "Distribution ID", typeDefTitle: "Distribution ID", typeDefKey: "distribution_id"},
	"apigw": {configTitle: "API ID", typeDefTitle: "API ID", typeDefKey: "api_id"},
	"efs":   {configTitle: "File System ID", typeDefTitle: "File System ID", typeDefKey: "file_system_id"},
}

func TestConfigDefaultViewDef_IDColumnSecond(t *testing.T) {
	for shortName, want := range expectedSecondColumn {
		t.Run(shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(shortName)
			if len(vd.List) < 2 {
				t.Fatalf("config.DefaultViewDef(%q) has fewer than 2 columns", shortName)
			}
			if vd.List[1].Title != want.configTitle {
				t.Errorf("config.DefaultViewDef(%q).List[1].Title = %q, want %q",
					shortName, vd.List[1].Title, want.configTitle)
			}
		})
	}
}

func TestResourceTypeDef_IDColumnSecond(t *testing.T) {
	for shortName, want := range expectedSecondColumn {
		t.Run(shortName, func(t *testing.T) {
			rt := resource.FindResourceType(shortName)
			if rt == nil {
				t.Fatalf("resource.FindResourceType(%q) returned nil", shortName)
			}
			if len(rt.Columns) < 2 {
				t.Fatalf("resource type %q has fewer than 2 columns", shortName)
			}
			if rt.Columns[1].Title != want.typeDefTitle {
				t.Errorf("resource type %q Columns[1].Title = %q, want %q",
					shortName, rt.Columns[1].Title, want.typeDefTitle)
			}
			if rt.Columns[1].Key != want.typeDefKey {
				t.Errorf("resource type %q Columns[1].Key = %q, want %q",
					shortName, rt.Columns[1].Key, want.typeDefKey)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Documented exceptions: these resources correctly have ID-first because the
// ID IS the human-readable name. Verify they are NOT affected by the fix.
// ---------------------------------------------------------------------------

var idFirstExceptions = []struct {
	shortName  string
	firstTitle string
}{
	{"dbi", "DB Identifier"},
	{"redis", "Cluster ID"},
	{"dbc", "Cluster ID"},
	{"redshift", "Cluster ID"},
	{"rds-snap", "Snapshot ID"},
	{"docdb-snap", "Snapshot ID"},
}

func TestDocumentedExceptions_IDFirstIsCorrect(t *testing.T) {
	for _, exc := range idFirstExceptions {
		t.Run(exc.shortName, func(t *testing.T) {
			// Verify config defaults still have ID first
			vd := config.DefaultViewDef(exc.shortName)
			if len(vd.List) == 0 {
				t.Fatalf("config.DefaultViewDef(%q) returned empty List", exc.shortName)
			}
			if vd.List[0].Title != exc.firstTitle {
				t.Errorf("exception %q: config.DefaultViewDef.List[0].Title = %q, want %q (ID-first is correct for this type)",
					exc.shortName, vd.List[0].Title, exc.firstTitle)
			}
		})
	}
}
