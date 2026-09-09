package unit

// prowler_w2_dbi_test.go — dbi posture rows 11–16 of the w2 Prowler batch:
// single-AZ, auto minor version upgrade off, IAM database authentication off,
// default master username, CA certificate expiring, deprecated engine version.
//
// Rows 11–15 are Wave-1 signals off the DBInstance the fetcher already holds.
// Row 15 needs a clock, so those cases drive the time-injectable fetcher
// variant rather than wall-clock time — a boundary test that depends on
// time.Now() is a test that fails on a slow machine at midnight.
// Row 16 is Wave 2 and reaches the enricher through the catalog registry.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2DBICodeSingleAZ         = "dbi.single-az"
	w2DBICodeMinorUpgradeOff  = "dbi.minor-upgrade-off"
	w2DBICodeIAMAuthOff       = "dbi.iam-auth-off"
	w2DBICodeDefaultMaster    = "dbi.default-master-user"
	w2DBICodeCACertExpiring   = "dbi.ca-cert-expiring"
	w2DBICodeCACertUrgent     = "dbi.ca-cert-expiring-urgent"
	w2DBICodeEngineDeprecated = "dbi.engine-deprecated"
)

// w2DBINow is the fixed clock every dated dbi case is evaluated against.
var w2DBINow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

type w2RDSInstancesFake struct {
	instances []rdstypes.DBInstance
}

func (f *w2RDSInstancesFake) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.instances}, nil
}

// w2DBIInstance returns an otherwise-healthy available PostgreSQL instance:
// multi-AZ, auto minor upgrade on, IAM auth on, a non-default master user, a
// CA certificate two years out. Each test switches off exactly one setting.
func w2DBIInstance(id string) rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier:             aws.String(id),
		DBInstanceArn:                    aws.String("arn:aws:rds:eu-central-1:123456789012:db:" + id),
		DBInstanceStatus:                 aws.String("available"),
		Engine:                           aws.String("postgres"),
		EngineVersion:                    aws.String("16.3"),
		DBInstanceClass:                  aws.String("db.t4g.medium"),
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		MasterUsername:                   aws.String("acme_app"),
		BackupRetentionPeriod:            aws.Int32(7),
		StorageEncrypted:                 aws.Bool(true),
		DeletionProtection:               aws.Bool(true),
		PubliclyAccessible:               aws.Bool(false),
		CertificateDetails: &rdstypes.CertificateDetails{
			CAIdentifier: aws.String("rds-ca-rsa2048-g1"),
			ValidTill:    aws.Time(w2DBINow.AddDate(2, 0, 0)),
		},
	}
}

func w2DBIFetch(t *testing.T, now time.Time, instances ...rdstypes.DBInstance) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchRDSInstancesPageAt(context.Background(), &w2RDSInstancesFake{instances: instances}, "", now)
	if err != nil {
		t.Fatalf("FetchRDSInstancesPageAt: %v", err)
	}
	byID := make(map[string]resource.Resource, len(out.Resources))
	for _, r := range out.Resources {
		byID[r.ID] = r
	}
	return byID
}

// ---------------------------------------------------------------------------
// row 11 — single-AZ
// ---------------------------------------------------------------------------

func TestW2DBISingleAZ(t *testing.T) {
	single := w2DBIInstance("acme-orders-db")
	single.MultiAZ = aws.Bool(false)

	got := w2DBIFetch(t, w2DBINow, single, w2DBIInstance("acme-billing-db"))

	w2AssertFinding(t, got["acme-orders-db"].Findings, w2DBICodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-billing-db"].Findings, w2DBICodeSingleAZ)
	w2AssertFindingDef(t, "dbi", w2DBICodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
}

// A read replica is single-AZ by construction and an Aurora instance gets its
// availability from the cluster. Flagging either is noise on a design that is
// already correct.
func TestW2DBISingleAZSkipsReplicasAndAurora(t *testing.T) {
	replica := w2DBIInstance("acme-orders-db-replica")
	replica.MultiAZ = aws.Bool(false)
	replica.ReadReplicaSourceDBInstanceIdentifier = aws.String("acme-orders-db")

	aurora := w2DBIInstance("acme-aurora-writer")
	aurora.MultiAZ = aws.Bool(false)
	aurora.Engine = aws.String("aurora-postgresql")
	aurora.DBClusterIdentifier = aws.String("acme-aurora")

	got := w2DBIFetch(t, w2DBINow, replica, aurora)

	w2AssertNoCode(t, got["acme-orders-db-replica"].Findings, w2DBICodeSingleAZ)
	w2AssertNoCode(t, got["acme-aurora-writer"].Findings, w2DBICodeSingleAZ)
}

// ---------------------------------------------------------------------------
// row 12 — auto minor version upgrade
// ---------------------------------------------------------------------------

func TestW2DBIMinorUpgradeOff(t *testing.T) {
	off := w2DBIInstance("acme-legacy-db")
	off.AutoMinorVersionUpgrade = aws.Bool(false)

	got := w2DBIFetch(t, w2DBINow, off, w2DBIInstance("acme-billing-db"))

	w2AssertFinding(t, got["acme-legacy-db"].Findings, w2DBICodeMinorUpgradeOff, "auto minor version upgrade off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-billing-db"].Findings, w2DBICodeMinorUpgradeOff)
	w2AssertFindingDef(t, "dbi", w2DBICodeMinorUpgradeOff, "auto minor version upgrade off", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// row 13 — IAM database authentication
// ---------------------------------------------------------------------------

func TestW2DBIIAMAuthOff(t *testing.T) {
	off := w2DBIInstance("acme-orders-db")
	off.IAMDatabaseAuthenticationEnabled = aws.Bool(false)

	got := w2DBIFetch(t, w2DBINow, off, w2DBIInstance("acme-billing-db"))

	w2AssertFinding(t, got["acme-orders-db"].Findings, w2DBICodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-billing-db"].Findings, w2DBICodeIAMAuthOff)
	w2AssertFindingDef(t, "dbi", w2DBICodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
}

// Oracle and SQL Server instances report the flag false because the engine has
// no IAM authentication to turn on. Flagging them asks for the impossible.
func TestW2DBIIAMAuthSkipsUnsupportedEngines(t *testing.T) {
	oracle := w2DBIInstance("acme-oracle-db")
	oracle.Engine = aws.String("oracle-se2")
	oracle.EngineVersion = aws.String("19.0.0.0.ru-2024-01.rur-2024-01.r1")
	oracle.IAMDatabaseAuthenticationEnabled = aws.Bool(false)

	sqlserver := w2DBIInstance("acme-mssql-db")
	sqlserver.Engine = aws.String("sqlserver-se")
	sqlserver.IAMDatabaseAuthenticationEnabled = aws.Bool(false)

	got := w2DBIFetch(t, w2DBINow, oracle, sqlserver)

	w2AssertNoCode(t, got["acme-oracle-db"].Findings, w2DBICodeIAMAuthOff)
	w2AssertNoCode(t, got["acme-mssql-db"].Findings, w2DBICodeIAMAuthOff)
}

func TestW2DBIIAMAuthCoversEverySupportedEngine(t *testing.T) {
	var instances []rdstypes.DBInstance
	for _, eng := range []string{"mysql", "mariadb", "postgres", "aurora-mysql", "aurora-postgresql"} {
		db := w2DBIInstance("acme-" + eng + "-db")
		db.Engine = aws.String(eng)
		db.IAMDatabaseAuthenticationEnabled = aws.Bool(false)
		instances = append(instances, db)
	}
	got := w2DBIFetch(t, w2DBINow, instances...)

	for _, eng := range []string{"mysql", "mariadb", "postgres", "aurora-mysql", "aurora-postgresql"} {
		id := "acme-" + eng + "-db"
		w2AssertFinding(t, got[id].Findings, w2DBICodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
	}
}

// ---------------------------------------------------------------------------
// row 14 — default master username
// ---------------------------------------------------------------------------

func TestW2DBIDefaultMasterUser(t *testing.T) {
	// Case is not significant: RDS stores whatever the operator typed.
	for _, name := range []string{"admin", "postgres", "root", "mysql", "master", "awsuser", "Admin", "POSTGRES"} {
		db := w2DBIInstance("acme-db-" + name)
		db.MasterUsername = aws.String(name)

		got := w2DBIFetch(t, w2DBINow, db)
		f := w2AssertFinding(t, got["acme-db-"+name].Findings, w2DBICodeDefaultMaster, "default master username", domain.SevWarn, "wave1")

		// U11: the phrase is stable and never carries the value, which the
		// detail row is responsible for.
		if f.Phrase != "default master username" {
			t.Errorf("%s: phrase drifted to %q", name, f.Phrase)
		}
	}
}

func TestW2DBINonDefaultMasterUserIsClean(t *testing.T) {
	got := w2DBIFetch(t, w2DBINow, w2DBIInstance("acme-billing-db"))
	w2AssertNoCode(t, got["acme-billing-db"].Findings, w2DBICodeDefaultMaster)
	w2AssertFindingDef(t, "dbi", w2DBICodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// row 15 — CA certificate expiry
// ---------------------------------------------------------------------------

func TestW2DBICACertExpiringWarn(t *testing.T) {
	db := w2DBIInstance("acme-orders-db")
	db.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 60)),
	}

	got := w2DBIFetch(t, w2DBINow, db)
	w2AssertFinding(t, got["acme-orders-db"].Findings, w2DBICodeCACertExpiring, "server certificate expires in 60 days", domain.SevWarn, "wave1")
}

// Inside 30 days the operator has a rotation window measured in weeks, not
// months — the row escalates so it sorts above the ordinary warnings.
//
// Inverted for the spec row that gave severity one owner: the escalation is
// now a second code carrying the catalog's broken severity, not the same code
// emitted at a severity its declaration does not have. Do not restore the
// dbi.ca-cert-expiring assertion here.
func TestW2DBICACertExpiringBroken(t *testing.T) {
	db := w2DBIInstance("acme-orders-db")
	db.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 15)),
	}

	got := w2DBIFetch(t, w2DBINow, db)
	w2AssertFinding(t, got["acme-orders-db"].Findings, w2DBICodeCACertUrgent, "server certificate expires in 15 days — rotate now", domain.SevBroken, "wave1")
}

// Exactly 30 days out is still the escalated tier; exactly 90 is still inside
// the window. Both boundaries are the ones an off-by-one lands on.
func TestW2DBICACertBoundaries(t *testing.T) {
	at30 := w2DBIInstance("acme-db-30")
	at30.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-rsa2048-g1"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 30)),
	}
	at90 := w2DBIInstance("acme-db-90")
	at90.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-rsa2048-g1"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 90)),
	}
	at91 := w2DBIInstance("acme-db-91")
	at91.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-rsa2048-g1"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 91)),
	}

	got := w2DBIFetch(t, w2DBINow, at30, at90, at91)

	w2AssertFinding(t, got["acme-db-30"].Findings, w2DBICodeCACertUrgent, "server certificate expires in 30 days — rotate now", domain.SevBroken, "wave1")
	w2AssertFinding(t, got["acme-db-90"].Findings, w2DBICodeCACertExpiring, "server certificate expires in 90 days", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-db-91"].Findings, w2DBICodeCACertExpiring)
	w2AssertNoCode(t, got["acme-db-91"].Findings, w2DBICodeCACertUrgent)
}

// No CertificateDetails means the expiry is unknown, which is not the same as
// expiring — an unknown must never colour a row.
func TestW2DBICACertNilDetailsEmitsNothing(t *testing.T) {
	noDetails := w2DBIInstance("acme-db-nodetails")
	noDetails.CertificateDetails = nil

	noValidTill := w2DBIInstance("acme-db-novalidtill")
	noValidTill.CertificateDetails = &rdstypes.CertificateDetails{CAIdentifier: aws.String("rds-ca-rsa2048-g1")}

	got := w2DBIFetch(t, w2DBINow, noDetails, noValidTill)

	w2AssertNoCode(t, got["acme-db-nodetails"].Findings, w2DBICodeCACertExpiring)
	w2AssertNoCode(t, got["acme-db-novalidtill"].Findings, w2DBICodeCACertExpiring)
}

// ---------------------------------------------------------------------------
// row 16 — deprecated engine version (Wave 2)
// ---------------------------------------------------------------------------

// w2RDSEngineVersionsFake answers DescribeDBEngineVersions from a
// (engine, version) → status map and counts calls so the per-run cache can be
// checked. Unknown pairs return an empty result, which AWS does for a version
// it no longer publishes.
type w2RDSEngineVersionsFake struct {
	awsclient.RDSAPI

	status map[string]string // "engine|version" → Status
	calls  map[string]int
	errs   map[string]error
}

func (f *w2RDSEngineVersionsFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	key := aws.ToString(in.Engine) + "|" + aws.ToString(in.EngineVersion)
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[key]++
	if err := f.errs[key]; err != nil {
		return nil, err
	}
	st, ok := f.status[key]
	if !ok {
		return &rds.DescribeDBEngineVersionsOutput{}, nil
	}
	return &rds.DescribeDBEngineVersionsOutput{
		DBEngineVersions: []rdstypes.DBEngineVersion{{
			Engine:        in.Engine,
			EngineVersion: in.EngineVersion,
			Status:        aws.String(st),
		}},
	}, nil
}

// The dbi enricher also carries the pre-existing pending-maintenance signal;
// answering it with an empty list keeps these cases about the engine version.
func (f *w2RDSEngineVersionsFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{}, nil
}

func w2DBIEnrich(t *testing.T, fake *w2RDSEngineVersionsFake, instances ...rdstypes.DBInstance) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(instances))
	for _, db := range instances {
		r := w2Res(aws.ToString(db.DBInstanceIdentifier), db)
		r.Fields["engine"] = aws.ToString(db.Engine)
		r.Fields["engine_version"] = aws.ToString(db.EngineVersion)
		rs = append(rs, r)
	}
	res, err := w2Enricher(t, "dbi")(context.Background(), &awsclient.ServiceClients{RDS: fake}, rs, nil)
	w2AssertEnricherInvariants(t, res, err)
	return res
}

func TestW2DBIEngineDeprecated(t *testing.T) {
	old := w2DBIInstance("acme-legacy-db")
	old.Engine = aws.String("mysql")
	old.EngineVersion = aws.String("5.7.44")

	current := w2DBIInstance("acme-billing-db")
	current.Engine = aws.String("postgres")
	current.EngineVersion = aws.String("16.3")

	fake := &w2RDSEngineVersionsFake{status: map[string]string{
		"mysql|5.7.44":  "deprecated",
		"postgres|16.3": "available",
	}}
	res := w2DBIEnrich(t, fake, old, current)

	w2AssertFinding(t, res.Findings["acme-legacy-db"], w2DBICodeEngineDeprecated, "engine version deprecated", domain.SevBroken, "wave2")
	w2AssertRow(t, w2Rows(t, res, "acme-legacy-db", w2DBICodeEngineDeprecated), "Engine", "mysql 5.7.44")
	w2AssertNoCode(t, res.Findings["acme-billing-db"], w2DBICodeEngineDeprecated)
	w2AssertFindingDef(t, "dbi", w2DBICodeEngineDeprecated, "engine version deprecated", domain.SevBroken, "wave2")
}

// AWS stops publishing a version once it is fully retired, so an empty result
// is the strongest deprecation signal there is — not an absence of data.
func TestW2DBIEngineUnpublishedCountsAsDeprecated(t *testing.T) {
	gone := w2DBIInstance("acme-ancient-db")
	gone.Engine = aws.String("mysql")
	gone.EngineVersion = aws.String("5.6.51")

	res := w2DBIEnrich(t, &w2RDSEngineVersionsFake{status: map[string]string{}}, gone)
	w2AssertFinding(t, res.Findings["acme-ancient-db"], w2DBICodeEngineDeprecated, "engine version deprecated", domain.SevBroken, "wave2")
}

// Ten instances on one engine version must cost one call, not ten — the
// per-run cache is the reason this row is affordable at all.
func TestW2DBIEngineVersionLookupCachedPerPair(t *testing.T) {
	var instances []rdstypes.DBInstance
	for i := range 10 {
		db := w2DBIInstance("acme-shard-" + string(rune('a'+i)))
		db.Engine = aws.String("mysql")
		db.EngineVersion = aws.String("5.7.44")
		instances = append(instances, db)
	}
	fake := &w2RDSEngineVersionsFake{status: map[string]string{"mysql|5.7.44": "deprecated"}}
	res := w2DBIEnrich(t, fake, instances...)

	if got := fake.calls["mysql|5.7.44"]; got != 1 {
		t.Errorf("DescribeDBEngineVersions called %d times for one (engine, version) pair; want 1", got)
	}
	for _, db := range instances {
		id := aws.ToString(db.DBInstanceIdentifier)
		w2AssertFinding(t, res.Findings[id], w2DBICodeEngineDeprecated, "engine version deprecated", domain.SevBroken, "wave2")
	}
}

// A denied lookup leaves the version unknown. The row goes unknown rather than
// claiming a deprecation nobody verified, and the other instance still resolves.
func TestW2DBIEngineLookupErrorMarksTruncated(t *testing.T) {
	denied := w2DBIInstance("acme-denied-db")
	denied.Engine = aws.String("oracle-se2")
	denied.EngineVersion = aws.String("19.0.0.0")

	ok := w2DBIInstance("acme-legacy-db")
	ok.Engine = aws.String("mysql")
	ok.EngineVersion = aws.String("5.7.44")

	fake := &w2RDSEngineVersionsFake{
		status: map[string]string{"mysql|5.7.44": "deprecated"},
		errs:   map[string]error{"oracle-se2|19.0.0.0": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}},
	}
	// Not w2DBIEnrich: that helper fatals on any error, and a denied lookup
	// legitimately returns the composite one (codex2 round 5 — the pass used
	// to record the failure and drop it). w2AssertEnricherShape is the map
	// half of the invariants, documented for exactly this case.
	res, err := w2Enricher(t, "dbi")(context.Background(),
		&awsclient.ServiceClients{RDS: fake}, codex2DBIRows(t, denied, ok), nil)
	if err == nil {
		t.Error("a denied engine lookup must reach the operator through the composite error")
	}
	w2AssertEnricherShape(t, res)

	if _, marked := res.TruncatedIDs["acme-denied-db"]; !marked {
		t.Error("instance whose engine lookup failed was not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["acme-denied-db"], w2DBICodeEngineDeprecated)
	w2AssertFinding(t, res.Findings["acme-legacy-db"], w2DBICodeEngineDeprecated, "engine version deprecated", domain.SevBroken, "wave2")
}

func TestW2DBIEngineNilClientIsSafe(t *testing.T) {
	db := w2DBIInstance("acme-legacy-db")
	r := w2Res("acme-legacy-db", db)
	res, err := w2Enricher(t, "dbi")(context.Background(), &awsclient.ServiceClients{}, []resource.Resource{r}, nil)
	w2AssertEnricherInvariants(t, res, err)
	w2AssertNoCode(t, res.Findings["acme-legacy-db"], w2DBICodeEngineDeprecated)
}

// ---------------------------------------------------------------------------
// independence and lifecycle
// ---------------------------------------------------------------------------

func TestW2DBIFourPostureConditionsOnOneInstance(t *testing.T) {
	bad := w2DBIInstance("acme-worst-db")
	bad.MultiAZ = aws.Bool(false)
	bad.AutoMinorVersionUpgrade = aws.Bool(false)
	bad.IAMDatabaseAuthenticationEnabled = aws.Bool(false)
	bad.MasterUsername = aws.String("postgres")

	got := w2DBIFetch(t, w2DBINow, bad)

	w2AssertFinding(t, got["acme-worst-db"].Findings, w2DBICodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-worst-db"].Findings, w2DBICodeMinorUpgradeOff, "auto minor version upgrade off", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-worst-db"].Findings, w2DBICodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["acme-worst-db"].Findings, w2DBICodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
}

// A deleting instance is on its way out; posture advice on it is noise the
// operator cannot act on and the badge should not count.
func TestW2DBIDeletingInstanceEmitsNoPostureFinding(t *testing.T) {
	deleting := w2DBIInstance("acme-old-db")
	deleting.DBInstanceStatus = aws.String("deleting")
	deleting.MultiAZ = aws.Bool(false)
	deleting.AutoMinorVersionUpgrade = aws.Bool(false)
	deleting.MasterUsername = aws.String("admin")

	got := w2DBIFetch(t, w2DBINow, deleting)

	w2AssertNoCode(t, got["acme-old-db"].Findings, w2DBICodeSingleAZ)
	w2AssertNoCode(t, got["acme-old-db"].Findings, w2DBICodeMinorUpgradeOff)
	w2AssertNoCode(t, got["acme-old-db"].Findings, w2DBICodeDefaultMaster)
}

// The Wave-1 guard alone is not enough: the Wave-2 enricher runs over the same
// rows and reaches its own conclusion. A deleting instance must be silent on
// both paths, or the badge still counts a row nobody can act on.
func TestW2DBIDeletingInstanceEmitsNoWave2Finding(t *testing.T) {
	deleting := w2DBIInstance("acme-old-db")
	deleting.DBInstanceStatus = aws.String("deleting")
	deleting.Engine = aws.String("mysql")
	deleting.EngineVersion = aws.String("5.7.44")

	out, err := awsclient.FetchRDSInstancesPageAt(context.Background(), &w2RDSInstancesFake{instances: []rdstypes.DBInstance{deleting}}, "", w2DBINow)
	if err != nil {
		t.Fatalf("FetchRDSInstancesPageAt: %v", err)
	}
	fake := &w2RDSEngineVersionsFake{status: map[string]string{"mysql|5.7.44": "deprecated"}}
	res, err := w2Enricher(t, "dbi")(context.Background(), &awsclient.ServiceClients{RDS: fake}, out.Resources, nil)
	w2AssertEnricherInvariants(t, res, err)

	w2AssertNoCode(t, res.Findings["acme-old-db"], w2DBICodeEngineDeprecated)
}

// TestW2DBICACertSingularDay pins spec row 3 (task aws3) at this emit site:
// number agreement in the wording is the slot filler's, read off the declared
// phrase, so a certificate one day out reads "1 day" and not "1 days". The
// site passes the number and nothing else.
func TestW2DBICACertSingularDay(t *testing.T) {
	one := w2DBIInstance("acme-db-1")
	one.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 1)),
	}
	two := w2DBIInstance("acme-db-2")
	two.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(w2DBINow.AddDate(0, 0, 2)),
	}

	got := w2DBIFetch(t, w2DBINow, one, two)
	w2AssertFinding(t, got["acme-db-1"].Findings, w2DBICodeCACertUrgent, "server certificate expires in 1 day — rotate now", domain.SevBroken, "wave1")
	w2AssertFinding(t, got["acme-db-2"].Findings, w2DBICodeCACertUrgent, "server certificate expires in 2 days — rotate now", domain.SevBroken, "wave1")
}
