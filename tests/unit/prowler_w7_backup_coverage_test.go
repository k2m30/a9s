package unit

// prowler_w7_backup_coverage_test.go — the backup-plan coverage join.
//
// The join answers "does any plan select this resource" from the cached backup
// list alone. That makes the list's completeness part of the contract: a list
// nobody fetched and a list cut short both mean the answer is unknown, and
// reporting a resource uncovered on either would invent a finding out of a gap
// in what was read. Only a list read to the end with no match can say so.
//
// Driven through each type's registered enricher rather than the join itself,
// because that is the path the app takes and the join is unexported. The
// enrichers run the coverage pass before their own client guard, so a nil
// service client is enough to reach it and no API is called.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	w7Account = "123456789012"
	w7Region  = "us-east-1"

	w7VolumeID  = "vol-0a1b2c3d4e5f60001"
	w7VolumeARN = "arn:aws:ec2:us-east-1:123456789012:volume/vol-0a1b2c3d4e5f60001"
	w7TableARN  = "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"
	w7DBIARN    = "arn:aws:rds:us-east-1:123456789012:db:acme-orders-db"
	w7DBCARN    = "arn:aws:rds:us-east-1:123456789012:cluster:acme-orders-cluster"
)

// w7Plan builds a backup plan row the way FetchBackupPlansPage does: the
// selection's ARNs, its exclusions and its tag conditions all ride on Fields.
func w7Plan(resources, notResources, selectionTags string) resource.Resource {
	return resource.Resource{
		ID:   "plan-0a1b2c3d",
		Name: "acme-nightly",
		Fields: map[string]string{
			"plan_name":      "acme-nightly",
			"plan_id":        "plan-0a1b2c3d",
			"resources":      resources,
			"not_resources":  notResources,
			"selection_tags": selectionTags,
		},
	}
}

// w7CacheWith returns a cache holding one whole backup list.
func w7CacheWith(plans ...resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: plans, IsTruncated: false},
	}
}

// w7Row is a resource carrying the ARN the join matches on.
func w7Row(id, arn string) resource.Resource {
	return resource.Resource{ID: id, Name: id, Fields: map[string]string{"arn": arn}}
}

// w7Enrich drives one type's registered enricher with no service client of its
// own, so only the cache-only coverage pass runs and no API is reachable.
func w7Enrich(
	t *testing.T,
	enrich func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error),
	clients *awsclient.ServiceClients,
	rows []resource.Resource,
	cache resource.ResourceCache,
) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := enrich(context.Background(), clients, rows, cache)
	if err != nil {
		t.Fatalf("enricher returned an error: %v", err)
	}
	return res
}

func w7EnrichDDB(t *testing.T, rows []resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	return w7Enrich(t, awsclient.EnrichDynamoDBPITR, &awsclient.ServiceClients{}, rows, cache)
}

func w7EnrichDBI(t *testing.T, rows []resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	return w7Enrich(t, awsclient.EnrichDBIMaintenance, &awsclient.ServiceClients{}, rows, cache)
}

func w7EnrichDBC(t *testing.T, rows []resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	return w7Enrich(t, awsclient.EnrichDBCMaintenance, &awsclient.ServiceClients{}, rows, cache)
}

// ── the join's three states ───────────────────────────────────────────────

// TestW7Coverage_WholeListWithNoMatchReportsUncovered is the only state that
// can produce the finding.
func TestW7Coverage_WholeListWithNoMatchReportsUncovered(t *testing.T) {
	res := w7EnrichDDB(t,
		[]resource.Resource{w7Row("acme-orders", w7TableARN)},
		w7CacheWith(w7Plan("arn:aws:dynamodb:us-east-1:123456789012:table/other-table", "", "")))

	w4AssertFinding(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2:ddb")
	w4AssertRows(t, res.AttentionDetails["acme-orders"], awsclient.CodeDDBNotInBackupPlan,
		[]domain.DetailRow{{Label: "Backup plans", Value: "0"}})
}

// TestW7Coverage_UnreadListReportsNothing pins the first unknown state. No
// cache entry means the backup list was never fetched, and a plan on a list
// nobody read can still cover this table.
func TestW7Coverage_UnreadListReportsNothing(t *testing.T) {
	res := w7EnrichDDB(t,
		[]resource.Resource{w7Row("acme-orders", w7TableARN)},
		resource.ResourceCache{})

	w4AssertNoCode(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan)
}

// TestW7Coverage_CutShortListReportsNothing pins the second. A truncated list
// is a lower bound on the plans that exist, so it cannot prove the absence of
// one that selects this table.
func TestW7Coverage_CutShortListReportsNothing(t *testing.T) {
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{w7Plan("arn:aws:dynamodb:us-east-1:123456789012:table/other-table", "", "")},
			IsTruncated: true,
		},
	}
	res := w7EnrichDDB(t, []resource.Resource{w7Row("acme-orders", w7TableARN)}, cache)

	w4AssertNoCode(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan)
}

// TestW7Coverage_EmptyWholeListReportsUncovered separates "no plans exist" from
// "the list was not read". An account with zero backup plans covers nothing,
// and that is a finding rather than an unknown.
func TestW7Coverage_EmptyWholeListReportsUncovered(t *testing.T) {
	res := w7EnrichDDB(t, []resource.Resource{w7Row("acme-orders", w7TableARN)}, w7CacheWith())

	w4AssertFinding(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2:ddb")
}

// ── the three matching modes, plus the exclusion ──────────────────────────

func TestW7Coverage_MatchingModes(t *testing.T) {
	tests := []struct {
		name          string
		plan          resource.Resource
		wantUncovered bool
	}{
		{
			name: "exact ARN in the selection",
			plan: w7Plan(w7TableARN, "", ""),
		},
		{
			name: "wildcard ARN covering every table",
			plan: w7Plan("arn:aws:dynamodb:*:*:table/*", "", ""),
		},
		{
			// A wildcard for another service must not swallow this table.
			name:          "wildcard ARN for a different service",
			plan:          w7Plan("arn:aws:ec2:*:*:volume/*", "", ""),
			wantUncovered: true,
		},
		{
			// NotResources excludes a resource the selection would otherwise
			// match, so the wildcard no longer covers it.
			name:          "excluded by NotResources",
			plan:          w7Plan("arn:aws:dynamodb:*:*:table/*", w7TableARN, ""),
			wantUncovered: true,
		},
		{
			// Beyond exact, wildcard and STRINGEQUALS the selection syntax is
			// matched conservatively as covered, so an unfamiliar shape never
			// invents a finding.
			name: "selection shape the join does not model",
			plan: w7Plan("arn:aws:dynamodb:us-east-1:123456789012:table/acme-*-v?", "", ""),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := w7EnrichDDB(t, []resource.Resource{w7Row("acme-orders", w7TableARN)}, w7CacheWith(tc.plan))
			if tc.wantUncovered {
				w4AssertFinding(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan,
					"not covered by a backup plan", domain.SevWarn, "wave2:ddb")
				return
			}
			w4AssertNoCode(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan)
		})
	}
}

// TestW7Coverage_SelectionTagsMatchTheVolumesTags pins the third mode on the
// one type in the batch whose row carries its tags: an EC2 volume. A plan that
// selects on a tag the volume has covers it without naming its ARN.
func TestW7Coverage_SelectionTagsMatchTheVolumesTags(t *testing.T) {
	volume := w7Volume("in-use")
	volume.Fields["arn"] = w7VolumeARN

	covered := w7EnrichEBS(t, []resource.Resource{volume}, w7CacheWith(w7Plan("", "", "backup=nightly")))
	w4AssertNoCode(t, covered.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)

	// A condition on a tag the volume does not carry leaves it uncovered.
	uncovered := w7EnrichEBS(t, []resource.Resource{volume}, w7CacheWith(w7Plan("", "", "backup=weekly")))
	w4AssertFinding(t, uncovered.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2:ebs")
}

// ── the four types ────────────────────────────────────────────────────────

// TestW7Coverage_EveryTypeReportsItsOwnCode pins that each type emits its own
// code and sentence rather than one shared finding, so a menu badge and a doc
// row exist per type.
func TestW7Coverage_EveryTypeReportsItsOwnCode(t *testing.T) {
	tests := []struct {
		short  string
		id     string
		arn    string
		code   domain.FindingCode
		enrich func(*testing.T, []resource.Resource, resource.ResourceCache) awsclient.IssueEnricherResult
	}{
		{"ddb", "acme-orders", w7TableARN, awsclient.CodeDDBNotInBackupPlan, w7EnrichDDB},
		{"dbi", "acme-orders-db", w7DBIARN, awsclient.CodeDBINotInBackupPlan, w7EnrichDBI},
		{"dbc", "acme-orders-cluster", w7DBCARN, awsclient.CodeDBCNotInBackupPlan, w7EnrichDBC},
	}

	for _, tc := range tests {
		t.Run(tc.short, func(t *testing.T) {
			res := tc.enrich(t, []resource.Resource{w7Row(tc.id, tc.arn)}, w7CacheWith())

			f := w4AssertFinding(t, res.Findings[tc.id], tc.code,
				"not covered by a backup plan", domain.SevWarn, "wave2:"+tc.short)
			if f.Detail == "" {
				t.Error("no Detail sentence")
			}
			w4AssertRows(t, res.AttentionDetails[tc.id], tc.code,
				[]domain.DetailRow{{Label: "Backup plans", Value: "0"}})

			// Covered by an exact ARN: the negative case for the same type.
			covered := tc.enrich(t, []resource.Resource{w7Row(tc.id, tc.arn)}, w7CacheWith(w7Plan(tc.arn, "", "")))
			w4AssertNoCode(t, covered.Findings[tc.id], tc.code)
		})
	}
}

// TestW7Coverage_ResourceWithNoARNReportsNothing pins the unknown case one
// level down: a row the fetcher gave no ARN cannot be matched against any
// selection, so it is not evidence of anything.
func TestW7Coverage_ResourceWithNoARNReportsNothing(t *testing.T) {
	res := w7EnrichDDB(t, []resource.Resource{{ID: "acme-orders", Fields: map[string]string{}}}, w7CacheWith())
	w4AssertNoCode(t, res.Findings["acme-orders"], awsclient.CodeDDBNotInBackupPlan)
}

// ── ebs: the ARN it has to build, and the snapshot join ───────────────────

// w7Volume builds an EC2 volume row the way FetchEBSVolumesPage does, tagged
// so the selection-tag mode has something to match.
func w7Volume(state string) resource.Resource {
	return resource.Resource{
		ID:   w7VolumeID,
		Name: "acme-data",
		Fields: map[string]string{
			"volume_id":   w7VolumeID,
			"name":        "acme-data",
			"state":       state,
			"attached_to": "i-0a1b2c3d4e5f60001",
			"az":          w7Region + "a",
		},
		RawStruct: ec2types.Volume{
			VolumeId:         aws.String(w7VolumeID),
			State:            ec2types.VolumeState(state),
			AvailabilityZone: aws.String(w7Region + "a"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("acme-data")},
				{Key: aws.String("backup"), Value: aws.String("nightly")},
			},
		},
	}
}

// w7EnrichEBS drives the ebs enricher with an account id already resolved, so
// the ARN it builds needs no STS call.
func w7EnrichEBS(t *testing.T, rows []resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{}
	store := session.NewIdentityStore()
	store.Set(w7Account, nil)
	clients.SetIdentityStore(store)
	return w7Enrich(t, awsclient.EnrichEBSVolumeStatus, clients, rows, cache)
}

// TestW7EBS_ARNIsBuiltFromTheAccountAndTheZone pins the one type whose row
// carries no ARN. The volume id and the availability zone are on the row; the
// account id comes from the clients, and the region is the zone without its
// trailing letter. Matching an exact ARN proves all three parts, because any
// one of them wrong yields a different string and the plan stops matching.
func TestW7EBS_ARNIsBuiltFromTheAccountAndTheZone(t *testing.T) {
	volume := w7Volume("in-use")

	covered := w7EnrichEBS(t, []resource.Resource{volume}, w7CacheWith(w7Plan(w7VolumeARN, "", "")))
	w4AssertNoCode(t, covered.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)

	// A plan naming the same volume in another account or region does not
	// cover it, which is what makes the assertion above about the whole ARN.
	for _, other := range []string{
		"arn:aws:ec2:us-east-1:210987654321:volume/" + w7VolumeID,
		"arn:aws:ec2:eu-west-1:123456789012:volume/" + w7VolumeID,
	} {
		res := w7EnrichEBS(t, []resource.Resource{volume}, w7CacheWith(w7Plan(other, "", "")))
		w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
			"not covered by a backup plan", domain.SevWarn, "wave2:ebs")
	}
}

// TestW7EBS_NoSnapshotJoin pins the second ebs row against the snapshot cache,
// including the states where the answer is unknown.
func TestW7EBS_NoSnapshotJoin(t *testing.T) {
	snapshotFor := func(volumeID string) resource.Resource {
		return resource.Resource{
			ID:     "snap-0a1b2c3d4e5f60001",
			Fields: map[string]string{"volume_id": volumeID},
		}
	}
	withSnapshots := func(entry resource.ResourceCacheEntry) resource.ResourceCache {
		c := w7CacheWith(w7Plan(w7VolumeARN, "", ""))
		c["ebs-snap"] = entry
		return c
	}

	tests := []struct {
		name          string
		state         string
		snapshots     resource.ResourceCacheEntry
		present       bool
		wantNoSnapNow bool
	}{
		{
			name:          "in-use volume with a whole snapshot list and no snapshot of it",
			state:         "in-use",
			snapshots:     resource.ResourceCacheEntry{Resources: []resource.Resource{snapshotFor("vol-other")}},
			present:       true,
			wantNoSnapNow: true,
		},
		{
			name:      "in-use volume with a snapshot of it",
			state:     "in-use",
			snapshots: resource.ResourceCacheEntry{Resources: []resource.Resource{snapshotFor(w7VolumeID)}},
			present:   true,
		},
		{
			// Not attached, so there is nothing running to lose.
			name:      "available volume with no snapshot",
			state:     "available",
			snapshots: resource.ResourceCacheEntry{Resources: []resource.Resource{snapshotFor("vol-other")}},
			present:   true,
		},
		{
			name:      "snapshot list cut short",
			state:     "in-use",
			snapshots: resource.ResourceCacheEntry{Resources: []resource.Resource{snapshotFor("vol-other")}, IsTruncated: true},
			present:   true,
		},
		{
			name:    "snapshot list never fetched",
			state:   "in-use",
			present: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := w7CacheWith(w7Plan(w7VolumeARN, "", ""))
			if tc.present {
				cache = withSnapshots(tc.snapshots)
			}
			res := w7EnrichEBS(t, []resource.Resource{w7Volume(tc.state)}, cache)

			if tc.wantNoSnapNow {
				w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNoSnapshot,
					"no snapshot exists", domain.SevWarn, "wave2:ebs")
				w4AssertRows(t, res.AttentionDetails[w7VolumeID], awsclient.CodeEBSNoSnapshot,
					[]domain.DetailRow{{Label: "Snapshots", Value: "0"}})
				return
			}
			w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNoSnapshot)
		})
	}
}

// TestW7EBS_BothConditionsOnOneVolume pins the independence rule: a volume that
// is neither in a plan nor snapshotted carries two findings, not the first one
// that happened to be evaluated.
func TestW7EBS_BothConditionsOnOneVolume(t *testing.T) {
	cache := w7CacheWith(w7Plan("arn:aws:ec2:us-east-1:123456789012:volume/vol-other", "", ""))
	cache["ebs-snap"] = resource.ResourceCacheEntry{Resources: []resource.Resource{}}

	res := w7EnrichEBS(t, []resource.Resource{w7Volume("in-use")}, cache)

	w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2:ebs")
	w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNoSnapshot,
		"no snapshot exists", domain.SevWarn, "wave2:ebs")
}

// ── the fetcher's nil-client guard ────────────────────────────────────────

// TestW7FetchBackupPlansPage_NilClientReturnsEmpty pins the guard every sibling
// fetcher has. A missing client is a session that never wired Backup, not a
// reason to crash the app; the coverage join then sees no cache entry and
// reports nothing, which is the same unknown state as an unfetched list.
func TestW7FetchBackupPlansPage_NilClientReturnsEmpty(t *testing.T) {
	// Recovered rather than left to crash: without the guard this call panics,
	// and a panic here takes the whole test binary down with it, hiding every
	// other failure in the package behind one missing nil check.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil client panicked instead of returning an empty page: %v", r)
		}
	}()

	got, err := awsclient.FetchBackupPlansPage(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("nil client returned an error: %v", err)
	}
	if len(got.Resources) != 0 {
		t.Errorf("nil client returned %d rows, want none", len(got.Resources))
	}
	if got.Pagination != nil && got.Pagination.IsTruncated {
		t.Error("nil client reported a truncated page, which reads as a lower bound rather than nothing")
	}
}
