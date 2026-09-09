package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
)

// ===========================================================================
// The name column comes first in every default list view.
//
// One list is read here. The built-in view is derived from the type's own
// columns (core/config/defaults.go), and that the two agree whole — set,
// order, width, key, path — is TestDefaultConfigColumnsAreTheCatalogs. So
// asserting the order twice, once per declaration, would be asserting the
// derivation rather than the order; the titles asserted here are the
// property, and the whole list is pinned in the file named above.
// ===========================================================================

// nameFirstColumn is the name column each of these 14 types must open with,
// and the key its cell reads.
var nameFirstColumn = []struct {
	shortName string
	title     string
	key       string
}{
	// Networking
	{"sg", "Group Name", "group_name"},
	{"vpc", "Name", "name"},
	{"subnet", "Name", "name"},
	{"rtb", "Name", "name"},
	{"nat", "Name", "name"},
	{"igw", "Name", "name"},
	{"eip", "Name", "name"},
	{"vpce", "Service Name", "service_name"},
	{"tgw", "Name", "name"},
	{"eni", "Name", "name"},
	// DNS/CDN
	{"r53", "Name", "name"},
	{"cf", "Domain Name", "domain_name"},
	{"apigw", "Name", "name"},
	// Databases
	{"efs", "Name", "name"},
}

func TestDefaultViewDef_NameColumnFirst(t *testing.T) {
	for _, spec := range nameFirstColumn {
		t.Run(spec.shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(spec.shortName)
			if len(vd.List) == 0 {
				t.Fatalf("config.DefaultViewDef(%q) returned empty List", spec.shortName)
			}
			if vd.List[0].Title != spec.title || vd.List[0].Key != spec.key {
				t.Errorf("%s opens with {Title:%q Key:%q}, want {Title:%q Key:%q} — the column an "+
					"operator scans for is the first one",
					spec.shortName, vd.List[0].Title, vd.List[0].Key, spec.title, spec.key)
			}
		})
	}
}

// idColumn is the identifier column that follows the name, at the index the
// type's Status column leaves it. r53 and apigw carry a Status column ahead of
// their ID (the title-based Status cascade, a56dc887), so theirs is third.
var idColumn = []struct {
	shortName string
	index     int
	title     string
	key       string
}{
	{"sg", 1, "Group ID", "group_id"},
	{"vpc", 1, "VPC ID", "vpc_id"},
	{"subnet", 1, "Subnet ID", "subnet_id"},
	{"rtb", 1, "Route Table ID", "route_table_id"},
	{"nat", 1, "NAT Gateway ID", "nat_gateway_id"},
	{"igw", 1, "IGW ID", "igw_id"},
	{"eip", 1, "Allocation ID", "allocation_id"},
	{"vpce", 1, "Endpoint ID", "vpce_id"},
	{"tgw", 1, "TGW ID", "tgw_id"},
	{"eni", 1, "ENI ID", "eni_id"},
	{"r53", 2, "Zone ID", "zone_id"},
	{"cf", 1, "Distribution ID", "distribution_id"},
	{"apigw", 2, "API ID", "api_id"},
	{"efs", 1, "File System ID", "file_system_id"},
}

func TestDefaultViewDef_IDColumnFollowsTheName(t *testing.T) {
	for _, spec := range idColumn {
		t.Run(spec.shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(spec.shortName)
			if len(vd.List) <= spec.index {
				t.Fatalf("config.DefaultViewDef(%q) has %d columns, want more than %d",
					spec.shortName, len(vd.List), spec.index)
			}
			got := vd.List[spec.index]
			if got.Title != spec.title || got.Key != spec.key {
				t.Errorf("%s column %d is {Title:%q Key:%q}, want {Title:%q Key:%q} — the id the name "+
					"replaced at the front is still on the screen",
					spec.shortName, spec.index, got.Title, got.Key, spec.title, spec.key)
			}
		})
	}
}

// idFirstExceptions are the types whose identifier IS the name an operator
// reads, so the rule above does not apply to them and must not be "fixed" onto
// them by a later sweep.
var idFirstExceptions = []struct {
	shortName  string
	firstTitle string
}{
	{"dbi", "DB Identifier"},
	{"redis", "Cluster ID"},
	{"dbc", "Cluster ID"},
	{"redshift", "Cluster ID"},
	{"dbi-snap", "Snapshot ID"},
	{"dbc-snap", "Snapshot ID"},
}

func TestDocumentedExceptions_IDFirstIsCorrect(t *testing.T) {
	for _, exc := range idFirstExceptions {
		t.Run(exc.shortName, func(t *testing.T) {
			vd := config.DefaultViewDef(exc.shortName)
			if len(vd.List) == 0 {
				t.Fatalf("config.DefaultViewDef(%q) returned empty List", exc.shortName)
			}
			if vd.List[0].Title != exc.firstTitle {
				t.Errorf("exception %q opens with %q, want %q — the identifier is this type's name",
					exc.shortName, vd.List[0].Title, exc.firstTitle)
			}
		})
	}
}
