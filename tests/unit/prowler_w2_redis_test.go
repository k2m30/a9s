package unit

// prowler_w2_redis_test.go — redis posture rows 7–10 of the w2 Prowler batch:
// encryption at rest off, encryption in transit off, no authentication token,
// automatic backups off.
//
// All four are Wave-1 signals derived from the ReplicationGroup the fetcher
// already holds, so the tests drive FetchRedisPage and read the findings off
// the produced resource. That also pins that the fetcher passes the whole
// replication group to the classifier rather than the four scalars it used to.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2RedisCodeAtRestOff  = "redis.encryption-at-rest-off"
	w2RedisCodeTransitOff = "redis.encryption-in-transit-off"
	w2RedisCodeNoAuth     = "redis.no-auth"
	w2RedisCodeNoBackup   = "redis.no-backup"
)

type w2RedisFake struct {
	groups []ectypes.ReplicationGroup
}

func (f *w2RedisFake) DescribeReplicationGroups(_ context.Context, _ *elasticache.DescribeReplicationGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return &elasticache.DescribeReplicationGroupsOutput{ReplicationGroups: f.groups}, nil
}

// w2RedisGroup returns an otherwise-healthy available redis replication group:
// encrypted at rest and in transit, AUTH on, seven days of snapshots. Each
// test switches off exactly the setting it is about.
func w2RedisGroup(id string) ectypes.ReplicationGroup {
	return ectypes.ReplicationGroup{
		ReplicationGroupId:       aws.String(id),
		Engine:                   aws.String("redis"),
		Status:                   aws.String("available"),
		CacheNodeType:            aws.String("cache.t4g.small"),
		ARN:                      aws.String("arn:aws:elasticache:eu-central-1:123456789012:replicationgroup:" + id),
		MemberClusters:           []string{id + "-001", id + "-002"},
		MultiAZ:                  ectypes.MultiAZStatusEnabled,
		AutomaticFailover:        ectypes.AutomaticFailoverStatusEnabled,
		AtRestEncryptionEnabled:  aws.Bool(true),
		TransitEncryptionEnabled: aws.Bool(true),
		AuthTokenEnabled:         aws.Bool(true),
		SnapshotRetentionLimit:   aws.Int32(7),
		ConfigurationEndpoint:    &ectypes.Endpoint{Address: aws.String(id + ".cache.amazonaws.com"), Port: aws.Int32(6379)},
		ClusterEnabled:           aws.Bool(true),
	}
}

func w2RedisFetch(t *testing.T, groups ...ectypes.ReplicationGroup) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchRedisPage(context.Background(), &w2RedisFake{groups: groups}, "")
	if err != nil {
		t.Fatalf("FetchRedisPage: %v", err)
	}
	byID := make(map[string]resource.Resource, len(out.Resources))
	for _, r := range out.Resources {
		byID[r.ID] = r
	}
	return byID
}

// ---------------------------------------------------------------------------
// row 7 — encryption at rest
// ---------------------------------------------------------------------------

func TestW2RedisAtRestEncryptionOff(t *testing.T) {
	off := w2RedisGroup("acme-cache-atrest-off")
	off.AtRestEncryptionEnabled = aws.Bool(false)

	// A group that predates the flag reports nil. AWS never enabled encryption
	// on those, so nil is a definite "off", not an unknown.
	absent := w2RedisGroup("acme-cache-atrest-nil")
	absent.AtRestEncryptionEnabled = nil

	got := w2RedisFetch(t, off, absent, w2RedisGroup("acme-cache-healthy"))

	for _, id := range []string{"acme-cache-atrest-off", "acme-cache-atrest-nil"} {
		w2AssertFinding(t, got[id].Findings, w2RedisCodeAtRestOff, "encryption at rest off", domain.SevWarn, "wave1")
	}
	w2AssertNoCode(t, got["acme-cache-healthy"].Findings, w2RedisCodeAtRestOff)
	w2AssertFindingDef(t, "redis", w2RedisCodeAtRestOff, "encryption at rest off", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// rows 8 & 9 — in-transit encryption and AUTH
// ---------------------------------------------------------------------------

func TestW2RedisTransitEncryptionOff(t *testing.T) {
	off := w2RedisGroup("acme-cache-transit-off")
	off.TransitEncryptionEnabled = aws.Bool(false)
	off.AuthTokenEnabled = aws.Bool(false)

	got := w2RedisFetch(t, off, w2RedisGroup("acme-cache-healthy"))

	w2AssertFinding(t, got["acme-cache-transit-off"].Findings, w2RedisCodeTransitOff, "encryption in transit off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-cache-healthy"].Findings, w2RedisCodeTransitOff)
	w2AssertFindingDef(t, "redis", w2RedisCodeTransitOff, "encryption in transit off", domain.SevWarn, "wave1")
}

func TestW2RedisNoAuthToken(t *testing.T) {
	noAuth := w2RedisGroup("acme-cache-noauth")
	noAuth.AuthTokenEnabled = aws.Bool(false)

	got := w2RedisFetch(t, noAuth, w2RedisGroup("acme-cache-healthy"))

	w2AssertFinding(t, got["acme-cache-noauth"].Findings, w2RedisCodeNoAuth, "no authentication token", domain.SevBroken, "wave1")
	w2AssertNoCode(t, got["acme-cache-healthy"].Findings, w2RedisCodeNoAuth)
	w2AssertFindingDef(t, "redis", w2RedisCodeNoAuth, "no authentication token", domain.SevBroken, "wave1")
}

// AWS only accepts an AUTH token on a group that has in-transit encryption on.
// Reporting a missing authentication token on a plaintext group would send
// the operator to fix a setting AWS refuses; the in-transit row is the one
// actionable signal.
func TestW2RedisNoAuthSuppressedWhenTransitEncryptionOff(t *testing.T) {
	plain := w2RedisGroup("acme-cache-plain")
	plain.TransitEncryptionEnabled = aws.Bool(false)
	plain.AuthTokenEnabled = aws.Bool(false)

	got := w2RedisFetch(t, plain)

	w2AssertFinding(t, got["acme-cache-plain"].Findings, w2RedisCodeTransitOff, "encryption in transit off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-cache-plain"].Findings, w2RedisCodeNoAuth)
}

// ---------------------------------------------------------------------------
// row 10 — automatic backups
// ---------------------------------------------------------------------------

func TestW2RedisNoBackup(t *testing.T) {
	zero := w2RedisGroup("acme-cache-nobackup")
	zero.SnapshotRetentionLimit = aws.Int32(0)

	absent := w2RedisGroup("acme-cache-nobackup-nil")
	absent.SnapshotRetentionLimit = nil

	got := w2RedisFetch(t, zero, absent, w2RedisGroup("acme-cache-healthy"))

	for _, id := range []string{"acme-cache-nobackup", "acme-cache-nobackup-nil"} {
		w2AssertFinding(t, got[id].Findings, w2RedisCodeNoBackup, "automatic backups off", domain.SevWarn, "wave1")
	}
	w2AssertNoCode(t, got["acme-cache-healthy"].Findings, w2RedisCodeNoBackup)
	w2AssertFindingDef(t, "redis", w2RedisCodeNoBackup, "automatic backups off", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// independence
// ---------------------------------------------------------------------------

// Three independent problems on one group produce three findings. The Status
// cell shows only the first plus a "(+N)" suffix, so the detail view is the
// only surface where the rest are visible — they must all be in Findings.
func TestW2RedisMultipleConditionsAllSurvive(t *testing.T) {
	bad := w2RedisGroup("acme-cache-bad")
	bad.AtRestEncryptionEnabled = aws.Bool(false)
	bad.TransitEncryptionEnabled = aws.Bool(false)
	bad.AuthTokenEnabled = aws.Bool(false)
	bad.SnapshotRetentionLimit = aws.Int32(0)

	got := w2RedisFetch(t, bad)

	w2AssertFinding(t, got["acme-cache-bad"].Findings, w2RedisCodeAtRestOff, "encryption at rest off", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-cache-bad"].Findings, w2RedisCodeTransitOff, "encryption in transit off", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-cache-bad"].Findings, w2RedisCodeNoBackup, "automatic backups off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-cache-bad"].Findings, w2RedisCodeNoAuth)
}

// A group being torn down is not a posture problem — flagging it would put a
// row in the badge count that nobody can or should act on.
func TestW2RedisDeletingGroupEmitsNoPostureFinding(t *testing.T) {
	deleting := w2RedisGroup("acme-cache-deleting")
	deleting.Status = aws.String("deleting")
	deleting.AtRestEncryptionEnabled = aws.Bool(false)
	deleting.SnapshotRetentionLimit = aws.Int32(0)

	got := w2RedisFetch(t, deleting)

	w2AssertNoCode(t, got["acme-cache-deleting"].Findings, w2RedisCodeAtRestOff)
	w2AssertNoCode(t, got["acme-cache-deleting"].Findings, w2RedisCodeNoBackup)
}
