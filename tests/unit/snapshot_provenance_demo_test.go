package unit_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

var spSnapTypes = []string{"dbi-snap", "dbc-snap", "ebs-snap"}

var spParentOf = map[string]string{"dbi-snap": "dbi", "dbc-snap": "dbc", "ebs-snap": "ebs"}

var spOrphanOf = map[string]domain.FindingCode{
	"dbi-snap": "dbi-snap.orphan",
	"dbc-snap": "dbc-snap.orphan",
	"ebs-snap": "ebs-snap.orphan",
}

var spOrphanFixture = map[string]string{
	"dbi-snap": fixtures.WarnDBISnapOrphanID,
	"dbc-snap": fixtures.WarnDBCSnapOrphanID,
}

func spHas(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

func spArnRegion(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// SourceDBClusterSnapshotArn is set on every cluster snapshot copy; only a
// source in another Region or account leaves the parent outside the local list.
func spForeignCopy(source, own *string) bool {
	src, self := strings.Split(aws.ToString(source), ":"), strings.Split(aws.ToString(own), ":")
	return len(src) >= 5 && len(self) >= 5 && (src[3] != self[3] || src[4] != self[4])
}

// SourceRegion is set on every RDS snapshot, so only a region differing from
// the snapshot's own ARN marks a copy; SourceDBSnapshotIdentifier is set only
// for cross-account or cross-region copies. A DbiResourceId, when present,
// is the only key to the parent instance.
func spCopiedWithParentAbsent(t *testing.T, raw any, parents []resource.Resource) bool {
	t.Helper()
	present := func(id string) bool {
		return slices.ContainsFunc(parents, func(p resource.Resource) bool { return p.ID == id })
	}
	switch s := raw.(type) {
	case rdstypes.DBSnapshot:
		copied := aws.ToString(s.SourceDBSnapshotIdentifier) != "" ||
			(aws.ToString(s.SourceRegion) != "" && aws.ToString(s.SourceRegion) != spArnRegion(aws.ToString(s.DBSnapshotArn)))
		if s.DbiResourceId == nil {
			return copied && !present(aws.ToString(s.DBInstanceIdentifier))
		}
		return copied && !slices.ContainsFunc(parents, func(p resource.Resource) bool {
			db, ok := p.RawStruct.(rdstypes.DBInstance)
			return ok && aws.ToString(db.DbiResourceId) == aws.ToString(s.DbiResourceId)
		})
	case docdbtypes.DBClusterSnapshot:
		return spForeignCopy(s.SourceDBClusterSnapshotArn, s.DBClusterSnapshotArn) && !present(aws.ToString(s.DBClusterIdentifier))
	case rdstypes.DBClusterSnapshot:
		return spForeignCopy(s.SourceDBClusterSnapshotArn, s.DBClusterSnapshotArn) && !present(aws.ToString(s.DBClusterIdentifier))
	case ec2types.Snapshot:
		return aws.ToString(s.VolumeId) == "vol-ffffffff"
	}
	t.Fatalf("unexpected snapshot RawStruct %T", raw)
	return false
}

func spDemoBench(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
	t.Helper()
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	out := map[string][]resource.Resource{}
	for _, s := range spSnapTypes {
		td := resource.FindResourceType(s)
		if td == nil {
			t.Fatalf("%s not registered", s)
		}
		out[s] = mergeWave2Findings(t, *td, byType[s], cache, clients)
	}
	return out, cache
}

func TestSnapshotProvenance_DemoCopiesAreNotOrphans(t *testing.T) {
	bench, cache := spDemoBench(t)

	copies := 0
	for _, s := range spSnapTypes {
		parents := cache[spParentOf[s]].Resources
		for _, r := range bench[s] {
			if !spCopiedWithParentAbsent(t, r.RawStruct, parents) {
				continue
			}
			copies++
			if spHas(r.Findings, spOrphanOf[s]) {
				t.Errorf("%s %q is a copy whose parent is not local, yet the demo shows it as orphan", s, r.ID)
			}
		}
	}
	if copies == 0 {
		t.Errorf("no demo dbi-snap/dbc-snap/ebs-snap row is a copy whose parent is absent from the local list; the demo cannot show the rule")
	}

	for s, id := range spOrphanFixture {
		i := slices.IndexFunc(bench[s], func(r resource.Resource) bool { return r.ID == id })
		if i < 0 {
			t.Errorf("%s orphan fixture %q is missing from the demo", s, id)
			continue
		}
		if !spHas(bench[s][i].Findings, spOrphanOf[s]) {
			t.Errorf("%s %q names a deleted local parent and lost its orphan finding: %+v", s, id, bench[s][i].Findings)
		}
	}
}

func TestSnapshotProvenance_OrphanStatusCellAndColour(t *testing.T) {
	bench, _ := spDemoBench(t)
	for s, id := range spOrphanFixture {
		td := resource.FindResourceType(s)
		i := slices.IndexFunc(bench[s], func(r resource.Resource) bool { return r.ID == id })
		if i < 0 {
			t.Fatalf("%s orphan fixture %q is missing from the demo", s, id)
		}
		r := bench[s][i]
		top, ok := domain.TopFinding(r.Findings)
		if !ok {
			t.Fatalf("%s %q carries no findings", s, id)
		}
		cell, ok := listStatusCellFor(t, *td, bench[s], id)
		if !ok {
			t.Fatalf("%s %q: no Status cell in the rendered list", s, id)
		}
		if phrase, _, _ := strings.Cut(cell, " (+"); phrase != top.Phrase {
			t.Errorf("%s %q Status cell = %q; want the top finding's phrase %q", s, id, cell, top.Phrase)
		}
		if got, want := td.ResolveColor(r), w5ColorOfSeverity(top.Severity); got != want {
			t.Errorf("%s %q colour %v; top finding %s wants %v", s, id, got, top.Code, want)
		}
	}
}

func TestSnapshotProvenance_AttentionRowsAreWords(t *testing.T) {
	bench, _ := spDemoBench(t)
	for s, rows := range bench {
		for _, r := range rows {
			for _, f := range r.Findings {
				ad, ok := r.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := w5Normalize(f.Phrase)
				for _, row := range ad.Rows {
					if w5Normalize(row.Value) == phrase || w5Normalize(row.Label+" "+row.Value) == phrase {
						t.Errorf("%s %q %s row %q=%q restates the phrase %q", s, r.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
					v := strings.TrimSpace(row.Value)
					if i := strings.LastIndex(v, " ("); i > 0 && strings.HasSuffix(v, ")") {
						v = v[:i]
					}
					if v == "true" || v == "false" {
						t.Errorf("%s %q %s row %q=%q is a Go bool literal", s, r.ID, f.Code, row.Label, row.Value)
					}
					if strings.Contains(v, "_") && v == strings.ToUpper(v) {
						t.Errorf("%s %q %s row %q=%q is SDK enum casing", s, r.ID, f.Code, row.Label, row.Value)
					}
				}
			}
		}
	}
}

// AWS creates an automated snapshot from its own instance or cluster; a copy
// of any snapshot is always manual, so an automated snapshot never names a
// copy source.
func TestSnapshotProvenance_DemoAutomatedSnapshotsNameNoCopySource(t *testing.T) {
	bench, _ := spDemoBench(t)
	for _, s := range []string{"dbi-snap", "dbc-snap"} {
		for _, r := range bench[s] {
			var snapType, source string
			switch raw := r.RawStruct.(type) {
			case rdstypes.DBSnapshot:
				snapType, source = aws.ToString(raw.SnapshotType), aws.ToString(raw.SourceDBSnapshotIdentifier)
			case docdbtypes.DBClusterSnapshot:
				snapType, source = aws.ToString(raw.SnapshotType), aws.ToString(raw.SourceDBClusterSnapshotArn)
			case rdstypes.DBClusterSnapshot:
				snapType, source = aws.ToString(raw.SnapshotType), aws.ToString(raw.SourceDBClusterSnapshotArn)
			}
			if snapType == "automated" && source != "" {
				t.Errorf("%s %q is automated yet names copy source %q", s, r.ID, source)
			}
		}
	}
}
