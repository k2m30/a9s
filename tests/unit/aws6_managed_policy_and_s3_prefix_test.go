package unit_test

// aws6_managed_policy_and_s3_prefix_test.go — two decisions taken with a
// witness on the demo bench.
//
// The first: a pivot naming an AWS-managed policy the account cannot read. The
// name came from the role's OWN attachment list, so a not-found there is not
// the deleted-between-list-and-read race MarkSkipped exists for — it is a
// navigation that points at something unreadable, and the aggregate keeps
// saying so.
//
// The second: an S3 "directory" row. A CommonPrefixes entry is API data, so it
// stays in the fetcher, but a prefix has no size, and "no size" is not the
// empty string.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// unreadableManagedPolicyName is a policy name no account can resolve: AWS
// retired it, so a role attachment naming it survives while GetPolicy does not.
// It is the shape row 8 is decided on.
const unreadableManagedPolicyName = "AmazonElasticTranscoder_FullAccess"

// TestManagedPolicyPivotToAnUnreadableNameStaysAFailure locks row 8's decision.
//
// A name that cannot be resolved came from the role's OWN attachment list, so
// it is not the deleted-between-list-and-read race MarkSkipped exists for — it
// is a navigation pointing at something unreadable, and the batch says so while
// still returning every policy it could read. Silently dropping the name would
// render the role's attachments one row short with no sign anything was
// missing.
//
// This pin is green today and stays that way: the decision is that the current
// behaviour is right, so the test exists to stop a later "tidy-up" from routing
// it through MarkSkipped.
func TestManagedPolicyPivotToAnUnreadableNameStaysAFailure(t *testing.T) {
	clients := demo.NewServiceClients()
	td := resource.FindResourceType("policy")
	if td == nil {
		t.Fatal("no policy resource type registered")
	}
	if td.FetchByIDs == nil {
		t.Fatal("policy has no FetchByIDs")
	}

	readable := demoReadablePolicyName(t, clients)

	// Negative case first: a batch of names the account can read succeeds
	// outright, so the failure below is about the one name, not about the call.
	rows, err := td.FetchByIDs(context.Background(), clients, []string{readable})
	if err != nil {
		t.Fatalf("FetchByIDs([%q]) = %v, want no error for a name the account holds", readable, err)
	}
	if len(rows) != 1 {
		t.Fatalf("FetchByIDs([%q]) returned %d rows, want 1", readable, len(rows))
	}

	rows, err = td.FetchByIDs(context.Background(), clients, []string{readable, unreadableManagedPolicyName})
	if err == nil {
		t.Fatalf("FetchByIDs([%q %q]) returned no error; a name the account cannot read must be reported, "+
			"not dropped", readable, unreadableManagedPolicyName)
	}
	if !strings.Contains(err.Error(), unreadableManagedPolicyName) {
		t.Errorf("FetchByIDs error = %q, want it to name %q — an aggregate that does not say which id failed "+
			"leaves the operator nothing to act on", err.Error(), unreadableManagedPolicyName)
	}
	if !strings.Contains(err.Error(), "NoSuchEntity") {
		t.Errorf("FetchByIDs error = %q, want it to carry the not-found cause; the operator needs to know the "+
			"policy is gone, not merely that something failed", err.Error())
	}

	// The readable policy still comes back: one unreadable name does not cost
	// the operator the rows the account could answer.
	if len(rows) != 1 || rows[0].ID != readable {
		t.Errorf("FetchByIDs returned %d rows %v, want just the readable policy %q", len(rows), policyIDs(rows), readable)
	}
}

// TestDemoBenchHasARoleAttachedToAnUnreadablePolicy is the witness half of row
// 8. The decision above is only visible to someone opening the app if the demo
// account actually contains a role whose attachment list names a policy that
// cannot be read — otherwise the failure path is a branch nobody ever sees, and
// the next person to read it has no way to tell it apart from dead code.
func TestDemoBenchHasARoleAttachedToAnUnreadablePolicy(t *testing.T) {
	clients := demo.NewServiceClients()
	td := resource.FindResourceType("policy")
	if td == nil {
		t.Fatal("no policy resource type registered")
	}

	attached := demoAttachedPolicyNames(t, clients)
	if len(attached) == 0 {
		t.Fatal("no demo role attaches any policy")
	}

	var unreadable []string
	for _, name := range attached {
		rows, err := td.FetchByIDs(context.Background(), clients, []string{name})
		if err != nil || len(rows) == 0 {
			unreadable = append(unreadable, name)
		}
	}
	if len(unreadable) == 0 {
		t.Errorf("all %d policy names attached to demo roles (%v) resolve; no demo role names a policy that "+
			"cannot be read, so the aggregate's navigation-defect failure never appears on the bench",
			len(attached), attached)
	}
}

// demoAttachedPolicyNames returns every managed-policy name a demo role
// attaches, read through the same two IAM calls the role-to-policy pivot makes.
func demoAttachedPolicyNames(t *testing.T, clients *awsclient.ServiceClients) []string {
	t.Helper()
	ctx := context.Background()
	roles, err := clients.IAM.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}

	seen := map[string]bool{}
	var out []string
	for i := range roles.Roles {
		attached, err := clients.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: roles.Roles[i].RoleName,
		})
		if err != nil {
			continue
		}
		for _, ap := range attached.AttachedPolicies {
			name := aws.ToString(ap.PolicyName)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// demoReadablePolicyName returns one policy name the demo account holds.
func demoReadablePolicyName(t *testing.T, clients *awsclient.ServiceClients) string {
	t.Helper()
	td := resource.FindResourceType("policy")
	if td == nil {
		t.Fatal("no policy resource type registered")
	}
	rows, ok := drainVisibilityFixtures(t, *td, clients)
	if !ok || len(rows) == 0 {
		t.Fatal("no demo policy rows")
	}
	return rows[0].ID
}

func policyIDs(rows []resource.Resource) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// TestS3PrefixRowCarriesTheSameKeysAsAnObjectRow pins the first half of row 9.
// A prefix row and an object row are rendered by the same columns, so a prefix
// row that carries fewer keys renders cells no formatter ever saw.
func TestS3PrefixRowCarriesTheSameKeysAsAnObjectRow(t *testing.T) {
	folders, files := demoS3ObjectRows(t, demo.NewServiceClients())
	if len(folders) == 0 || len(files) == 0 {
		t.Fatalf("demo bench needs both prefix and object rows; got %d folders, %d files", len(folders), len(files))
	}

	want := fieldKeySet(files[0].Fields)
	for _, f := range folders {
		got := fieldKeySet(f.Fields)
		if strings.Join(got, ",") == strings.Join(want, ",") {
			continue
		}
		t.Errorf("s3 prefix row %q carries keys %v, want the object row's key set %v — "+
			"both are rendered by the same columns", f.ID, got, want)
	}
}

// TestS3PrefixRowWritesNoEmptyString pins the second half. The fetcher does not
// know how "no size" should read; the cell formatter does. Writing "" here is
// the fetcher deciding, and deciding on a blank.
func TestS3PrefixRowWritesNoEmptyString(t *testing.T) {
	folders, _ := demoS3ObjectRows(t, demo.NewServiceClients())
	if len(folders) == 0 {
		t.Fatal("no demo s3 prefix rows")
	}
	for _, f := range folders {
		keys := fieldKeySet(f.Fields)
		for _, k := range keys {
			if f.Fields[k] != "" {
				continue
			}
			t.Errorf("s3 prefix row %q writes Fields[%q] = \"\" — a prefix has no size, no last-modified "+
				"and no storage class, and \"not applicable\" is the cell formatter's word to choose, "+
				"not an empty string the fetcher writes", f.ID, k)
		}
	}
}

// TestS3PrefixSizeReadsTheSameOnEveryPrefixRow pins the wording: whatever the
// not-applicable size reads as, every prefix says it the same way, and no
// object row borrows it.
//
// The assertion is on the value the fetcher hands the column, not on a rendered
// child-list cell: a child view has no top-level list screen to drive, and the
// Size column reads this value verbatim.
func TestS3PrefixSizeReadsTheSameOnEveryPrefixRow(t *testing.T) {
	folders, files := demoS3ObjectRows(t, demo.NewServiceClients())
	if len(folders) < 2 || len(files) == 0 {
		t.Fatalf("need at least two prefix rows and one object row; got %d and %d", len(folders), len(files))
	}

	want := folders[0].Fields["size"]
	if want == "" {
		t.Errorf("prefix row %q has an empty size; a prefix's size is not applicable and the row says so",
			folders[0].ID)
	}
	for _, f := range folders[1:] {
		if f.Fields["size"] != want {
			t.Errorf("prefix rows disagree on the not-applicable size: %q says %q, %q says %q — "+
				"one fact, one wording", folders[0].ID, want, f.ID, f.Fields["size"])
		}
	}

	// The negative case: a row that HAS a size still shows it.
	for _, f := range files {
		if f.Fields["size_raw"] == "" || f.Fields["size_raw"] == "0" {
			continue
		}
		if f.Fields["size"] == want {
			t.Errorf("object row %q carries the prefix's not-applicable size %q despite holding %s bytes",
				f.ID, want, f.Fields["size_raw"])
		}
	}
}

// TestS3PrefixRowsSortFirstAscending pins what must NOT change: within one
// bucket listing, prefixes lead, the way a file browser puts folders first.
func TestS3PrefixRowsSortFirstAscending(t *testing.T) {
	listings := demoS3ObjectListings(t, demo.NewServiceClients())
	if len(listings) == 0 {
		t.Fatal("no demo s3 object listings")
	}

	checked := 0
	for bucket, rows := range listings {
		seenFile := -1
		for i, r := range rows {
			if r.Fields["kind"] != "folder" {
				if seenFile < 0 {
					seenFile = i
				}
				continue
			}
			if seenFile >= 0 {
				t.Errorf("bucket %s: prefix row %q at index %d follows an object row at index %d; prefixes sort first",
					bucket, r.ID, i, seenFile)
			}
		}
		if seenFile > 0 {
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no demo bucket listing mixes prefixes and objects, so the ordering rule was never exercised")
	}
}

// fieldKeySet returns the sorted keys of a Fields map.
func fieldKeySet(fields map[string]string) []string {
	out := make([]string, 0, len(fields))
	for k := range fields {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// demoS3ObjectRows drains the s3_objects child view over every demo bucket and
// splits the result into prefix rows and object rows.
func demoS3ObjectRows(t *testing.T, clients *awsclient.ServiceClients) (folders, files []resource.Resource) {
	t.Helper()
	for _, r := range demoS3ObjectRowsInFetchOrder(t, clients) {
		if r.Fields["kind"] == "folder" {
			folders = append(folders, r)
		} else {
			files = append(files, r)
		}
	}
	return folders, files
}

// demoS3ObjectRowsInFetchOrder returns every s3_objects row across all demo
// buckets.
func demoS3ObjectRowsInFetchOrder(t *testing.T, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	var out []resource.Resource
	for _, rows := range demoS3ObjectListings(t, clients) {
		out = append(out, rows...)
	}
	return out
}

// demoS3ObjectListings returns one listing per demo bucket, in the order the
// fetcher produced it, so per-listing ordering can be pinned. Ordering across
// buckets is not a thing the app ever renders.
func demoS3ObjectListings(t *testing.T, clients *awsclient.ServiceClients) map[string][]resource.Resource {
	t.Helper()
	parent := resource.FindResourceType("s3")
	if parent == nil {
		t.Fatal("no s3 resource type registered")
	}
	buckets, ok := drainVisibilityFixtures(t, *parent, clients)
	if !ok {
		t.Fatal("s3 has no fetcher")
	}
	ctd := resource.GetChildType("s3_objects")
	if ctd == nil || ctd.ChildFetcher == nil {
		t.Fatal("s3_objects has no child fetcher")
	}

	out := map[string][]resource.Resource{}
	for _, child := range parent.Children {
		if child.ChildType != "s3_objects" {
			continue
		}
		for i := range buckets {
			dr := domain.Resource(buckets[i])
			pctx := resource.ResolveChildContext(child, &dr, nil)
			res, err := ctd.ChildFetcher(context.Background(), clients, pctx, "")
			if err != nil || len(res.Resources) == 0 {
				continue
			}
			out[buckets[i].ID] = res.Resources
		}
	}
	return out
}
