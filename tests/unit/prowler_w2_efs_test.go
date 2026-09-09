package unit

// prowler_w2_efs_test.go — efs rows 27–29 of the w2 Prowler batch:
// unencrypted file system (Wave 1), a file system policy open to anyone and
// automatic backups off (both Wave 2, added to EnrichEFSMountTargets).
//
// The two Wave-2 rows land in the existing mount-target enricher rather than a
// second one, so the tests keep a healthy mount target on every file system:
// the mount-target finding must not appear or disappear because a policy or a
// backup policy was read alongside it.

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2EFSCodeUnencrypted    = "efs.unencrypted"
	w2EFSCodePublicPolicy   = "efs.public-policy"
	w2EFSCodeNoBackupPolicy = "efs.no-backup-policy"
	w2EFSSource             = "wave2"
)

// ---------------------------------------------------------------------------
// row 27 — encryption at rest (Wave 1)
// ---------------------------------------------------------------------------

type w2EFSListFake struct {
	fileSystems []efstypes.FileSystemDescription
}

func (f *w2EFSListFake) DescribeFileSystems(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	return &efs.DescribeFileSystemsOutput{FileSystems: f.fileSystems}, nil
}

func w2EFSFileSystem(id, name string) efstypes.FileSystemDescription {
	return efstypes.FileSystemDescription{
		FileSystemId:         aws.String(id),
		FileSystemArn:        aws.String("arn:aws:elasticfilesystem:eu-central-1:123456789012:file-system/" + id),
		Name:                 aws.String(name),
		LifeCycleState:       efstypes.LifeCycleStateAvailable,
		NumberOfMountTargets: 2,
		Encrypted:            aws.Bool(true),
		PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
		ThroughputMode:       efstypes.ThroughputModeElastic,
		OwnerId:              aws.String("123456789012"),
		CreationToken:        aws.String(name),
	}
}

func w2EFSFetch(t *testing.T, fss ...efstypes.FileSystemDescription) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchEFSFileSystemsPage(context.Background(), &w2EFSListFake{fileSystems: fss}, "")
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: %v", err)
	}
	return w2ByID(out.Resources)
}

func TestW2EFSUnencrypted(t *testing.T) {
	off := w2EFSFileSystem("fs-0acme00000000001", "acme-shared")
	off.Encrypted = aws.Bool(false)

	// File systems created before encryption existed report nil; AWS never
	// encrypted those, so nil is a definite off.
	absent := w2EFSFileSystem("fs-0acme00000000002", "acme-legacy")
	absent.Encrypted = nil

	got := w2EFSFetch(t, off, absent, w2EFSFileSystem("fs-0acme00000000003", "acme-safe"))

	w2AssertFinding(t, got["fs-0acme00000000001"].Findings, w2EFSCodeUnencrypted, "not encrypted", domain.SevWarn, "wave1")
	w2AssertFinding(t, got["fs-0acme00000000002"].Findings, w2EFSCodeUnencrypted, "not encrypted", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["fs-0acme00000000003"].Findings, w2EFSCodeUnencrypted)
	w2AssertFindingDef(t, "efs", w2EFSCodeUnencrypted, "not encrypted", domain.SevWarn, "wave1")
}

func TestW2EFSDeletingFileSystemEmitsNoEncryptionFinding(t *testing.T) {
	deleting := w2EFSFileSystem("fs-0acme00000000009", "acme-old")
	deleting.LifeCycleState = efstypes.LifeCycleStateDeleting
	deleting.Encrypted = aws.Bool(false)

	got := w2EFSFetch(t, deleting)
	w2AssertNoCode(t, got["fs-0acme00000000009"].Findings, w2EFSCodeUnencrypted)
}

// ---------------------------------------------------------------------------
// rows 28 & 29 — file system policy and backup policy (Wave 2)
// ---------------------------------------------------------------------------

// w2EFSPolicyFake answers mount targets, the file system policy and the backup
// policy. Absent map entries are healthy: two available mount targets, no
// file system policy, backups enabled.
type w2EFSPolicyFake struct {
	awsclient.EFSAPI

	policies   map[string]string
	backupOff  map[string]string // fsID → non-ENABLED status
	noBackup   map[string]bool   // fsID → PolicyNotFound from DescribeBackupPolicy
	mtErrs     map[string]error
	policyErrs map[string]error
}

func (f *w2EFSPolicyFake) DescribeMountTargets(_ context.Context, in *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
	id := aws.ToString(in.FileSystemId)
	if err := f.mtErrs[id]; err != nil {
		return nil, err
	}
	return &efs.DescribeMountTargetsOutput{
		MountTargets: []efstypes.MountTargetDescription{
			{
				MountTargetId:  aws.String("fsmt-0acme00000000001"),
				FileSystemId:   in.FileSystemId,
				SubnetId:       aws.String("subnet-0acme0000000001"),
				LifeCycleState: efstypes.LifeCycleStateAvailable,
			},
		},
	}, nil
}

func (f *w2EFSPolicyFake) DescribeFileSystemPolicy(_ context.Context, in *efs.DescribeFileSystemPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemPolicyOutput, error) {
	id := aws.ToString(in.FileSystemId)
	if err := f.policyErrs[id]; err != nil {
		return nil, err
	}
	doc, ok := f.policies[id]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "PolicyNotFound", Message: "no policy"}
	}
	return &efs.DescribeFileSystemPolicyOutput{FileSystemId: in.FileSystemId, Policy: aws.String(doc)}, nil
}

func (f *w2EFSPolicyFake) DescribeBackupPolicy(_ context.Context, in *efs.DescribeBackupPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeBackupPolicyOutput, error) {
	id := aws.ToString(in.FileSystemId)
	if f.noBackup[id] {
		return nil, &smithy.GenericAPIError{Code: "PolicyNotFound", Message: "no backup policy"}
	}
	status := efstypes.StatusEnabled
	if s, ok := f.backupOff[id]; ok {
		status = efstypes.Status(s)
	}
	return &efs.DescribeBackupPolicyOutput{BackupPolicy: &efstypes.BackupPolicy{Status: status}}, nil
}

func w2EFSRun(t *testing.T, fake *w2EFSPolicyFake, ids ...string) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	rs := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		rs = append(rs, w2Res(id, w2EFSFileSystem(id, "acme-"+id)))
	}
	res, err := w2Enricher(t, "efs")(context.Background(), &awsclient.ServiceClients{EFS: fake}, rs, nil)
	w2AssertEnricherShape(t, res)
	return res, err
}

// w2EFSEnrich is for the cases where nothing genuinely failed. A file system
// with no policy and no backup policy answers PolicyNotFound on both calls,
// which is the normal shape of an unshared, unbacked-up file system — not a
// call the run failed to make.
func w2EFSEnrich(t *testing.T, fake *w2EFSPolicyFake, ids ...string) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := w2EFSRun(t, fake, ids...)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	return res
}

const w2EFSPublicPolicyDoc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AnyoneMount",
      "Effect": "Allow",
      "Principal": {"AWS": "*"},
      "Action": ["elasticfilesystem:ClientMount", "elasticfilesystem:ClientWrite"],
      "Resource": "arn:aws:elasticfilesystem:eu-central-1:123456789012:file-system/fs-0acme00000000001"
    }
  ]
}`

func TestW2EFSPublicPolicy(t *testing.T) {
	fake := &w2EFSPolicyFake{policies: map[string]string{
		"fs-0acme00000000001": w2EFSPublicPolicyDoc,
	}}
	res := w2EFSEnrich(t, fake, "fs-0acme00000000001", "fs-0acme00000000003")

	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodePublicPolicy, "file system policy open to anyone", domain.SevBroken, w2EFSSource)
	rows := w2Rows(t, res, "fs-0acme00000000001", w2EFSCodePublicPolicy)
	w2AssertRow(t, rows, "Principal", "*")
	w2AssertRow(t, rows, "Actions", "elasticfilesystem:ClientMount, elasticfilesystem:ClientWrite")
	w2AssertFindingDef(t, "efs", w2EFSCodePublicPolicy, "file system policy open to anyone", domain.SevBroken, "wave2")

	w2AssertNoCode(t, res.Findings["fs-0acme00000000003"], w2EFSCodePublicPolicy)
	if _, marked := res.TruncatedIDs["fs-0acme00000000003"]; marked {
		t.Error("PolicyNotFound marked the file system unknown; it is a definite no-policy answer")
	}
}

// A policy scoped to this account's roles is the intended configuration.
func TestW2EFSScopedPolicyIsClean(t *testing.T) {
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/app"},"Action":"elasticfilesystem:ClientMount","Resource":"*"}]}`
	fake := &w2EFSPolicyFake{policies: map[string]string{"fs-0acme00000000004": doc}}
	res := w2EFSEnrich(t, fake, "fs-0acme00000000004")
	w2AssertNoCode(t, res.Findings["fs-0acme00000000004"], w2EFSCodePublicPolicy)
}

// A wildcard principal fenced by an access-point condition is a scoped grant.
func TestW2EFSConditionedWildcardIsNotPublic(t *testing.T) {
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"elasticfilesystem:ClientMount","Resource":"*","Condition":{"StringEquals":{"aws:SourceVpce":["vpce-0acme00000000001"]}}}]}`
	fake := &w2EFSPolicyFake{policies: map[string]string{"fs-0acme00000000005": doc}}
	res := w2EFSEnrich(t, fake, "fs-0acme00000000005")
	w2AssertNoCode(t, res.Findings["fs-0acme00000000005"], w2EFSCodePublicPolicy)
}

func TestW2EFSNoBackupPolicy(t *testing.T) {
	fake := &w2EFSPolicyFake{
		backupOff: map[string]string{"fs-0acme00000000001": string(efstypes.StatusDisabled)},
		noBackup:  map[string]bool{"fs-0acme00000000002": true},
	}
	res := w2EFSEnrich(t, fake, "fs-0acme00000000001", "fs-0acme00000000002", "fs-0acme00000000003")

	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodeNoBackupPolicy, "automatic backups off", domain.SevWarn, w2EFSSource)
	// The row does not read the raw EFS enum "DISABLED": an operator never
	// sees an SDK constant, and the phrase already says backups are off, so
	// the row has nothing to add.
	w2AssertNoRows(t, res, "fs-0acme00000000001", w2EFSCodeNoBackupPolicy)

	// No backup policy at all is the same operational fact as a disabled one.
	w2AssertFinding(t, res.Findings["fs-0acme00000000002"], w2EFSCodeNoBackupPolicy, "automatic backups off", domain.SevWarn, w2EFSSource)

	w2AssertNoCode(t, res.Findings["fs-0acme00000000003"], w2EFSCodeNoBackupPolicy)
	w2AssertFindingDef(t, "efs", w2EFSCodeNoBackupPolicy, "automatic backups off", domain.SevWarn, "wave2")
}

// ---------------------------------------------------------------------------
// independence and error handling inside the shared enricher
// ---------------------------------------------------------------------------

func TestW2EFSPolicyAndBackupConditionsAreIndependent(t *testing.T) {
	fake := &w2EFSPolicyFake{
		policies:  map[string]string{"fs-0acme00000000001": w2EFSPublicPolicyDoc},
		backupOff: map[string]string{"fs-0acme00000000001": string(efstypes.StatusDisabled)},
	}
	res := w2EFSEnrich(t, fake, "fs-0acme00000000001")

	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodePublicPolicy, "file system policy open to anyone", domain.SevBroken, w2EFSSource)
	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodeNoBackupPolicy, "automatic backups off", domain.SevWarn, w2EFSSource)
	w2AssertRow(t, w2Rows(t, res, "fs-0acme00000000001", w2EFSCodePublicPolicy), "Principal", "*")
	// Same reason as above: the backup-policy finding stands on its phrase.
	w2AssertNoRows(t, res, "fs-0acme00000000001", w2EFSCodeNoBackupPolicy)
}

// The mount-target signal this enricher already owns must keep working once
// the two policy calls join it in the same per-file-system body.
func TestW2EFSMountTargetSignalSurvivesTheNewCalls(t *testing.T) {
	fake := &w2EFSPolicyFake{
		policies: map[string]string{"fs-0acme00000000001": w2EFSPublicPolicyDoc},
		mtErrs: map[string]error{
			"fs-0acme00000000002": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2EFSRun(t, fake, "fs-0acme00000000001", "fs-0acme00000000002")
	if err == nil {
		t.Error("a denied mount-target read was not folded into the composite error")
	}

	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodePublicPolicy, "file system policy open to anyone", domain.SevBroken, w2EFSSource)
	if _, marked := res.TruncatedIDs["fs-0acme00000000002"]; !marked {
		t.Error("file system whose mount targets could not be read was not marked in TruncatedIDs")
	}
}

func TestW2EFSPolicyErrorMarksTruncatedAndSparesTheRest(t *testing.T) {
	fake := &w2EFSPolicyFake{
		policies: map[string]string{"fs-0acme00000000001": w2EFSPublicPolicyDoc},
		policyErrs: map[string]error{
			"fs-0acme00000000002": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"},
		},
	}
	res, err := w2EFSRun(t, fake, "fs-0acme00000000002", "fs-0acme00000000001")
	if err == nil {
		t.Error("a denied policy read was not folded into the composite error")
	}

	if _, marked := res.TruncatedIDs["fs-0acme00000000002"]; !marked {
		t.Error("file system whose policy could not be read was not marked in TruncatedIDs")
	}
	w2AssertNoCode(t, res.Findings["fs-0acme00000000002"], w2EFSCodePublicPolicy)
	w2AssertFinding(t, res.Findings["fs-0acme00000000001"], w2EFSCodePublicPolicy, "file system policy open to anyone", domain.SevBroken, w2EFSSource)
}

func TestW2EFSNilClientIsSafe(t *testing.T) {
	res, err := w2Enricher(t, "efs")(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w2Res("fs-0acme00000000001", w2EFSFileSystem("fs-0acme00000000001", "acme-shared"))}, nil)
	w2AssertEnricherInvariants(t, res, err)
	if len(res.Findings) != 0 {
		t.Errorf("nil EFS client produced findings %v", res.Findings)
	}
}

func TestW2EFSCapBoundary(t *testing.T) {
	ids := make([]string, 0, awsclient.EnrichmentCap+1)
	backupOff := map[string]string{}
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		id := "fs-0acme" + strconv.Itoa(100000000+i)
		ids = append(ids, id)
		backupOff[id] = string(efstypes.StatusDisabled)
	}

	atCap := w2EFSEnrich(t, &w2EFSPolicyFake{backupOff: backupOff}, ids[:awsclient.EnrichmentCap]...)
	if atCap.Truncated {
		t.Error("exactly EnrichmentCap file systems reported Truncated")
	}

	overCap := w2EFSEnrich(t, &w2EFSPolicyFake{backupOff: backupOff}, ids...)
	if !overCap.Truncated {
		t.Error("EnrichmentCap+1 file systems did not report Truncated")
	}
}

// Same contract on the Wave-2 side for efs: a file system being deleted needs
// no backup policy and no policy review.
func TestW2EFSDeletingFileSystemEmitsNoWave2Finding(t *testing.T) {
	deleting := w2EFSFileSystem("fs-0acme00000000009", "acme-old")
	deleting.LifeCycleState = efstypes.LifeCycleStateDeleting

	out, err := awsclient.FetchEFSFileSystemsPage(context.Background(), &w2EFSListFake{fileSystems: []efstypes.FileSystemDescription{deleting}}, "")
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: %v", err)
	}

	fake := &w2EFSPolicyFake{
		policies:  map[string]string{"fs-0acme00000000009": w2EFSPublicPolicyDoc},
		backupOff: map[string]string{"fs-0acme00000000009": string(efstypes.StatusDisabled)},
	}
	res, err := w2Enricher(t, "efs")(context.Background(), &awsclient.ServiceClients{EFS: fake}, out.Resources, nil)
	w2AssertEnricherShape(t, res)
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}

	w2AssertNoCode(t, res.Findings["fs-0acme00000000009"], w2EFSCodePublicPolicy)
	w2AssertNoCode(t, res.Findings["fs-0acme00000000009"], w2EFSCodeNoBackupPolicy)
}
