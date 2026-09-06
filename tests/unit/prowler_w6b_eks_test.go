package unit

// prowler_w6b_eks_test.go — behavioural pins for the four eks posture signals
// of batch w6b.
//
// All four are wave 1: FetchEKSClustersPage already calls DescribeCluster per
// cluster, so the endpoint configuration, the control-plane log setup, the
// encryption configuration and the version all arrive with the row.
//
// eks.version-unsupported is the exception worth spelling out. AWS publishes
// the support state of every Kubernetes minor through DescribeClusterVersions,
// so the check asks AWS rather than comparing against a floor constant
// compiled into the binary. A constant is wrong the day AWS moves the
// calendar, and wrong silently — the binary keeps answering with confidence.
// These tests therefore never assert which numbers are supported; they assert
// that whatever AWS calls extended support or unsupported is flagged and
// whatever AWS calls standard support is not.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6bEKSCodePublicEndpoint = domain.FindingCode("eks.public-endpoint")
	w6bEKSCodeLoggingOff     = domain.FindingCode("eks.control-plane-logging-off")
	w6bEKSCodeSecretsNotKMS  = domain.FindingCode("eks.secrets-not-kms")
	w6bEKSCodeVersionOld     = domain.FindingCode("eks.version-unsupported")
)

// Registered phrases. eks.public-endpoint does not read "API endpoint …" as
// the batch table first wrote it: "API" is a bare uppercase token the
// rendered-surface ruling bans.
const (
	w6bEKSPhrasePublicEndpoint = "cluster endpoint reachable from the internet"
	w6bEKSPhraseLoggingOff     = "control plane logging incomplete"
	w6bEKSPhraseSecretsNotKMS  = "secrets not encrypted with KMS"
)

// w6bEKSFake serves the fetcher's ListClusters/DescribeCluster pair plus the
// version catalogue. Embedding EKSAPI keeps the nodegroup methods honest: if
// the posture code ever reaches for one, the fake panics instead of quietly
// answering zero.
type w6bEKSFake struct {
	awsclient.EKSAPI
	clusters map[string]*ekstypes.Cluster
	order    []string
	// versions maps a Kubernetes minor to what AWS reports about it. A version
	// absent from this map is absent from the catalogue, which is what AWS
	// does for a minor it no longer publishes at all.
	versions map[string]w6bEKSVersionInfo
	// versionsErr fails the catalogue lookup.
	versionsErr error
	// versionCalls counts catalogue lookups so a per-cluster fan-out is
	// visible rather than merely slow.
	versionCalls int
}

func (f *w6bEKSFake) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: f.order}, nil
}

func (f *w6bEKSFake) DescribeCluster(_ context.Context, in *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return &eks.DescribeClusterOutput{Cluster: f.clusters[aws.ToString(in.Name)]}, nil
}

func (f *w6bEKSFake) DescribeClusterVersions(_ context.Context, _ *eks.DescribeClusterVersionsInput, _ ...func(*eks.Options)) (*eks.DescribeClusterVersionsOutput, error) {
	f.versionCalls++
	if f.versionsErr != nil {
		return nil, f.versionsErr
	}
	out := &eks.DescribeClusterVersionsOutput{}
	for v, info := range f.versions {
		// Status (the deprecated lowercase field) is deliberately left unset.
		// AWS's own docs say it is replaced by VersionStatus, and a reader
		// that still consults it sees "" for every version — which means no
		// cluster is ever flagged and the row silently stops working.
		out.ClusterVersions = append(out.ClusterVersions, ekstypes.ClusterVersionInformation{
			ClusterVersion:           aws.String(v),
			ClusterType:              aws.String("eks"),
			VersionStatus:            info.status,
			EndOfStandardSupportDate: info.endOfStandardSupport,
		})
	}
	return out, nil
}

// w6bEKSVersionInfo is what the catalogue reports for one Kubernetes minor.
type w6bEKSVersionInfo struct {
	status               ekstypes.VersionStatus
	endOfStandardSupport *time.Time
}

// w6bEKSCluster builds a healthy active cluster: private endpoint, all five
// control-plane log types on, secrets under a customer KMS key.
func w6bEKSCluster(name, version string) *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:            aws.String(name),
		Arn:             aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
		Version:         aws.String(version),
		Status:          ekstypes.ClusterStatusActive,
		Endpoint:        aws.String("https://ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com"),
		PlatformVersion: aws.String("eks.12"),
		CreatedAt:       aws.Time(time.Now().Add(-400 * 24 * time.Hour)),
		RoleArn:         aws.String("arn:aws:iam::123456789012:role/acme-eks-cluster"),
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			VpcId:                 aws.String("vpc-0aaaa1111bbbb2222"),
			SubnetIds:             []string{"subnet-0aaaa1111bbbb2222", "subnet-0cccc3333dddd4444"},
			EndpointPrivateAccess: true,
			EndpointPublicAccess:  false,
		},
		Logging: &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{{
			Enabled: aws.Bool(true),
			Types: []ekstypes.LogType{
				ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator,
				ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler,
			},
		}}},
		EncryptionConfig: []ekstypes.EncryptionConfig{{
			Resources: []string{"secrets"},
			Provider:  &ekstypes.Provider{KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/aaaa1111-bbbb-2222-cccc-333344445555")},
		}},
	}
}

// w6bEKSEndOfStandardSupport is the date the catalogue reports for 1.28. It is
// referenced by the row assertions rather than repeated, so the expected
// rendering and the fake cannot drift apart.
var w6bEKSEndOfStandardSupport = time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)

// w6bEKSSupportedVersions is the catalogue every test that is not about the
// version row uses, so those clusters never trip row 18 by accident.
func w6bEKSSupportedVersions() map[string]w6bEKSVersionInfo {
	return map[string]w6bEKSVersionInfo{
		"1.33": {status: ekstypes.VersionStatusStandardSupport},
		"1.32": {status: ekstypes.VersionStatusStandardSupport},
		"1.31": {status: ekstypes.VersionStatusStandardSupport},
		"1.28": {status: ekstypes.VersionStatusExtendedSupport, endOfStandardSupport: aws.Time(w6bEKSEndOfStandardSupport)},
		"1.26": {status: ekstypes.VersionStatusUnsupported, endOfStandardSupport: aws.Time(time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC))},
	}
}

func w6bFetchEKS(t *testing.T, fake *w6bEKSFake) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchEKSClustersPage(context.Background(),
		&awsclient.ServiceClients{EKS: fake}, "")
	if err != nil {
		t.Fatalf("FetchEKSClustersPage: %v", err)
	}
	return out.Resources
}

// w6bEKSFetchOne is the common single-cluster path: one cluster, the standard
// support catalogue.
func w6bEKSFetchOne(t *testing.T, c *ekstypes.Cluster) resource.Resource {
	t.Helper()
	name := aws.ToString(c.Name)
	rs := w6bFetchEKS(t, &w6bEKSFake{
		order:    []string{name},
		clusters: map[string]*ekstypes.Cluster{name: c},
		versions: w6bEKSSupportedVersions(),
	})
	return pw1ResourceByID(t, rs, name)
}

// ─── row 15: eks.public-endpoint ────────────────────────────────────────────

// A public endpoint open to 0.0.0.0/0 puts the Kubernetes API on the internet,
// where every credential-stuffing bot can reach it.
func TestW6BEKS_PublicEndpoint_OpenToTheWorld(t *testing.T) {
	c := w6bEKSCluster("acme-dev", "1.33")
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = []string{"0.0.0.0/0"}
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodePublicEndpoint,
		w6bEKSPhrasePublicEndpoint, domain.SevBroken, "wave1")
	// The row exists to show the ranges, which the phrase does not carry.
	w6bRequireRowValue(t, w6bWave1Rows(r, w6bEKSCodePublicEndpoint), "0.0.0.0/0")
}

// Public but fenced to named ranges is a weaker exposure than open to the
// world, and the severity has to say so or the two are indistinguishable in
// the list.
func TestW6BEKS_PublicEndpoint_ScopedToNamedRanges_IsWarn(t *testing.T) {
	c := w6bEKSCluster("acme-staging", "1.33")
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = []string{"203.0.113.0/24"}
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodePublicEndpoint,
		w6bEKSPhrasePublicEndpoint, domain.SevWarn, "wave1")
	w6bRequireRowValue(t, w6bWave1Rows(r, w6bEKSCodePublicEndpoint), "203.0.113.0/24")
}

// An open range anywhere in the list is an open endpoint; a scoped range
// alongside it does not narrow anything.
func TestW6BEKS_PublicEndpoint_OpenRangeAmongScopedOnes_IsBroken(t *testing.T) {
	c := w6bEKSCluster("acme-mixed", "1.33")
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = []string{"203.0.113.0/24", "0.0.0.0/0"}
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodePublicEndpoint,
		w6bEKSPhrasePublicEndpoint, domain.SevBroken, "wave1")
}

// AWS defaults PublicAccessCidrs to 0.0.0.0/0 and omits it when it was never
// narrowed, so an empty list on a public endpoint is the open case, not the
// unknown one.
func TestW6BEKS_PublicEndpoint_NoCidrsListed_IsBroken(t *testing.T) {
	c := w6bEKSCluster("acme-default-cidrs", "1.33")
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = nil
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodePublicEndpoint,
		w6bEKSPhrasePublicEndpoint, domain.SevBroken, "wave1")
}

func TestW6BEKS_PrivateEndpoint_IsHealthy(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-prod", "1.33"))
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodePublicEndpoint)
}

// A cluster with no VPC configuration in the response must not panic and must
// not be guessed at.
func TestW6BEKS_NilVpcConfig_IsHealthy(t *testing.T) {
	c := w6bEKSCluster("acme-novpc", "1.33")
	c.ResourcesVpcConfig = nil
	r := w6bEKSFetchOne(t, c)
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodePublicEndpoint)
}

// ─── row 16: eks.control-plane-logging-off ──────────────────────────────────

// Missing audit and authenticator logs mean an intrusion leaves no record.
func TestW6BEKS_ControlPlaneLoggingOff_SomeTypesDisabled(t *testing.T) {
	c := w6bEKSCluster("acme-degraded-prod", "1.33")
	c.Logging = &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{
		{Enabled: aws.Bool(true), Types: []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAuthenticator, ekstypes.LogTypeControllerManager}},
		{Enabled: aws.Bool(false), Types: []ekstypes.LogType{ekstypes.LogTypeAudit, ekstypes.LogTypeScheduler}},
	}}
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodeLoggingOff,
		w6bEKSPhraseLoggingOff, domain.SevWarn, "wave1")
	// The row names which types to turn on — the phrase only says the set is
	// incomplete.
	rows := w6bWave1Rows(r, w6bEKSCodeLoggingOff)
	w6bRequireRowValueContains(t, rows, "audit")
	w6bRequireRowValueContains(t, rows, "scheduler")
}

// A cluster with logging never configured has all five off.
func TestW6BEKS_ControlPlaneLoggingOff_NothingConfigured(t *testing.T) {
	c := w6bEKSCluster("acme-nolog", "1.33")
	c.Logging = nil
	r := w6bEKSFetchOne(t, c)
	pw1RequireFinding(t, r.Findings, w6bEKSCodeLoggingOff,
		w6bEKSPhraseLoggingOff, domain.SevWarn, "wave1")
}

// A LogSetup entry that lists a type with Enabled=false does not enable it.
// Reading Types without reading Enabled reports a fully-logged cluster.
func TestW6BEKS_ControlPlaneLogging_DisabledEntryDoesNotCount(t *testing.T) {
	c := w6bEKSCluster("acme-disabled-entry", "1.33")
	c.Logging = &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{{
		Enabled: aws.Bool(false),
		Types: []ekstypes.LogType{
			ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator,
			ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler,
		},
	}}}
	r := w6bEKSFetchOne(t, c)
	pw1RequireFinding(t, r.Findings, w6bEKSCodeLoggingOff,
		w6bEKSPhraseLoggingOff, domain.SevWarn, "wave1")
}

func TestW6BEKS_AllLogTypesEnabled_IsHealthy(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-prod", "1.33"))
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodeLoggingOff)
}

// The five types may arrive spread across several enabled entries; the check
// is about the union, not about one entry carrying them all.
func TestW6BEKS_LogTypesSplitAcrossEntries_IsHealthy(t *testing.T) {
	c := w6bEKSCluster("acme-split-logging", "1.33")
	c.Logging = &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{
		{Enabled: aws.Bool(true), Types: []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAudit}},
		{Enabled: aws.Bool(true), Types: []ekstypes.LogType{ekstypes.LogTypeAuthenticator}},
		{Enabled: aws.Bool(true), Types: []ekstypes.LogType{ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler}},
	}}
	r := w6bEKSFetchOne(t, c)
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodeLoggingOff)
}

// ─── row 17: eks.secrets-not-kms ────────────────────────────────────────────

func TestW6BEKS_SecretsNotKMS_NoEncryptionConfig(t *testing.T) {
	c := w6bEKSCluster("acme-prod-updating", "1.33")
	c.EncryptionConfig = nil
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodeSecretsNotKMS,
		w6bEKSPhraseSecretsNotKMS, domain.SevWarn, "wave1")
	w6bRequireNoRows(t, w6bWave1Rows(r, w6bEKSCodeSecretsNotKMS))
}

// An encryption configuration that covers something other than secrets does
// not cover secrets. Testing only for a non-empty slice reports this cluster
// as protected.
func TestW6BEKS_SecretsNotKMS_ConfigDoesNotCoverSecrets(t *testing.T) {
	c := w6bEKSCluster("acme-other-resources", "1.33")
	c.EncryptionConfig = []ekstypes.EncryptionConfig{{
		Resources: []string{"configmaps"},
		Provider:  &ekstypes.Provider{KeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/aaaa1111-bbbb-2222-cccc-333344445555")},
	}}
	r := w6bEKSFetchOne(t, c)
	pw1RequireFinding(t, r.Findings, w6bEKSCodeSecretsNotKMS,
		w6bEKSPhraseSecretsNotKMS, domain.SevWarn, "wave1")
}

func TestW6BEKS_SecretsUnderKMS_IsHealthy(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-prod", "1.33"))
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodeSecretsNotKMS)
}

// ─── row 18: eks.version-unsupported ────────────────────────────────────────

// w6bEKSVersionPhrase is the phrase with the cluster's own version spliced in.
// The FindingDef registers it with a <version> placeholder.
func w6bEKSVersionPhrase(version string) string {
	return "Kubernetes " + version + " is out of standard support"
}

// A minor in extended support still receives patches, but only on a paid clock
// that ends — and AWS, not a constant in this binary, is the authority on when
// that clock started.
func TestW6BEKS_VersionUnsupported_ExtendedSupportIsFlagged(t *testing.T) {
	c := w6bEKSCluster("acme-staging-failed", "1.28")
	r := w6bEKSFetchOne(t, c)

	f := pw1RequireFinding(t, r.Findings, w6bEKSCodeVersionOld,
		w6bEKSVersionPhrase("1.28"), domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bEKSCodeVersionOld)
	// Two facts the phrase leaves out. "Out of standard support" does not say
	// whether the cluster still receives patches at all, and it does not say
	// when the clock ran out — which is what tells an operator whether this is
	// a plan-it-this-quarter or a fix-it-now.
	w6bRequireRowValueContains(t, rows, "extended support")
	w6bRequireRowValueContains(t, rows, w6bEKSEndOfStandardSupport.Format("2006-01-02"))
	w6bRequireNoRawEnum(t, f, rows)
	if strings.Contains(f.Detail, "1.31") {
		t.Errorf("Detail names a hardcoded floor version: %q", f.Detail)
	}
}

// The support word is the row's whole point, so it has to be a word. AWS
// spells the state EXTENDED_SUPPORT; handing string(VersionStatus) to the row
// puts the SDK's enum on a surface an operator reads.
func TestW6BEKS_VersionUnsupported_UnsupportedRowIsWordsAndDate(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-ancient", "1.26"))

	f := pw1RequireFinding(t, r.Findings, w6bEKSCodeVersionOld,
		w6bEKSVersionPhrase("1.26"), domain.SevBroken, "wave1")
	rows := w6bWave1Rows(r, w6bEKSCodeVersionOld)
	w6bRequireRowValueContains(t, rows, "unsupported")
	w6bRequireRowValueContains(t, rows, "2024-06-11")
	w6bRequireNoRawEnum(t, f, rows)
}

// The catalogue answers through VersionStatus. AWS deprecated the lowercase
// Status field in favour of it, and this fake leaves Status unset on every
// entry, so a reader still consulting the deprecated field sees "" for every
// version and flags nothing. Without this test that regression looks exactly
// like a clean run: no findings, no errors, no clue.
func TestW6BEKS_VersionCatalogue_ReadFromVersionStatusNotDeprecatedStatus(t *testing.T) {
	const name = "acme-deprecated-field-probe"
	fake := &w6bEKSFake{
		order:    []string{name},
		clusters: map[string]*ekstypes.Cluster{name: w6bEKSCluster(name, "1.28")},
		versions: w6bEKSSupportedVersions(),
	}
	rs := w6bFetchEKS(t, fake)

	for _, entry := range w6bCatalogueEntries(t, fake) {
		if entry.Status != "" {
			t.Fatalf("the fake must leave the deprecated Status field unset for %s; this test proves nothing otherwise",
				aws.ToString(entry.ClusterVersion))
		}
	}
	pw1RequireFinding(t, pw1ResourceByID(t, rs, name).Findings, w6bEKSCodeVersionOld,
		w6bEKSVersionPhrase("1.28"), domain.SevBroken, "wave1")
}

// w6bCatalogueEntries returns what the fake would answer, so the guard above
// checks the fixture it actually served rather than a second copy of it.
func w6bCatalogueEntries(t *testing.T, fake *w6bEKSFake) []ekstypes.ClusterVersionInformation {
	t.Helper()
	out, err := fake.DescribeClusterVersions(context.Background(), &eks.DescribeClusterVersionsInput{})
	if err != nil {
		t.Fatalf("fake catalogue: %v", err)
	}
	return out.ClusterVersions
}

// A minor AWS has dropped entirely receives no patches at all.
func TestW6BEKS_VersionUnsupported_UnsupportedIsFlagged(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-ancient", "1.26"))
	pw1RequireFinding(t, r.Findings, w6bEKSCodeVersionOld,
		w6bEKSVersionPhrase("1.26"), domain.SevBroken, "wave1")
}

// The negative case is the whole point of asking AWS: a version in standard
// support is clean whatever its number, so this must stay green when the
// calendar moves and nothing in a9s changes.
func TestW6BEKS_VersionInStandardSupport_IsHealthy(t *testing.T) {
	for _, v := range []string{"1.31", "1.32", "1.33"} {
		t.Run(v, func(t *testing.T) {
			r := w6bEKSFetchOne(t, w6bEKSCluster("acme-current-"+v, v))
			pw1RequireNoFinding(t, r.Findings, w6bEKSCodeVersionOld)
		})
	}
}

// The support state is a property of the version, not of the cluster, so the
// same catalogue answers for every cluster in the page. One lookup per page,
// not one per cluster.
func TestW6BEKS_VersionCatalogue_ReadOncePerPage(t *testing.T) {
	fake := &w6bEKSFake{
		order:    []string{"acme-a", "acme-b", "acme-c"},
		clusters: map[string]*ekstypes.Cluster{},
		versions: w6bEKSSupportedVersions(),
	}
	for _, n := range fake.order {
		fake.clusters[n] = w6bEKSCluster(n, "1.28")
	}
	rs := w6bFetchEKS(t, fake)
	for _, n := range fake.order {
		pw1RequireFinding(t, pw1ResourceByID(t, rs, n).Findings, w6bEKSCodeVersionOld,
			w6bEKSVersionPhrase("1.28"), domain.SevBroken, "wave1")
	}
	if fake.versionCalls > 1 {
		t.Errorf("DescribeClusterVersions called %d times for one page of %d clusters; the catalogue is per-version, not per-cluster",
			fake.versionCalls, len(fake.order))
	}
}

// When AWS will not answer, the version is unknown. Guessing from a constant
// is exactly what this row exists to avoid, so an unreadable catalogue means
// no finding rather than a confident one.
func TestW6BEKS_VersionCatalogueUnavailable_EmitsNoVersionFinding(t *testing.T) {
	fake := &w6bEKSFake{
		order:       []string{"acme-unknown-version"},
		clusters:    map[string]*ekstypes.Cluster{"acme-unknown-version": w6bEKSCluster("acme-unknown-version", "1.28")},
		versionsErr: errors.New("AccessDeniedException: not authorized to perform eks:DescribeClusterVersions"),
	}
	rs := w6bFetchEKS(t, fake)
	r := pw1ResourceByID(t, rs, "acme-unknown-version")
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodeVersionOld)
	// The other three rows read the DescribeCluster response and are
	// unaffected by the catalogue being unavailable.
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodePublicEndpoint)
}

// A version AWS does not publish at all is unknown, not old.
func TestW6BEKS_VersionAbsentFromCatalogue_EmitsNoVersionFinding(t *testing.T) {
	r := w6bEKSFetchOne(t, w6bEKSCluster("acme-unpublished", "1.99"))
	pw1RequireNoFinding(t, r.Findings, w6bEKSCodeVersionOld)
}

// ─── independence ───────────────────────────────────────────────────────────

// Contract rule 4: four conditions on one cluster are four findings, and the
// lifecycle finding the fetcher already emits survives alongside them.
func TestW6BEKS_AllFourConditions_ProduceFourFindings(t *testing.T) {
	c := w6bEKSCluster("acme-worst-cluster", "1.28")
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = []string{"0.0.0.0/0"}
	c.Logging = nil
	c.EncryptionConfig = nil
	r := w6bEKSFetchOne(t, c)

	pw1RequireFinding(t, r.Findings, w6bEKSCodePublicEndpoint, w6bEKSPhrasePublicEndpoint, domain.SevBroken, "wave1")
	pw1RequireFinding(t, r.Findings, w6bEKSCodeLoggingOff, w6bEKSPhraseLoggingOff, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bEKSCodeSecretsNotKMS, w6bEKSPhraseSecretsNotKMS, domain.SevWarn, "wave1")
	pw1RequireFinding(t, r.Findings, w6bEKSCodeVersionOld, w6bEKSVersionPhrase("1.28"), domain.SevBroken, "wave1")
}

// A cluster being deleted has no posture to fix — contract rule 4.
func TestW6BEKS_DeletingCluster_EmitsNoPostureFinding(t *testing.T) {
	c := w6bEKSCluster("acme-going-away", "1.28")
	c.Status = ekstypes.ClusterStatusDeleting
	c.ResourcesVpcConfig.EndpointPublicAccess = true
	c.ResourcesVpcConfig.PublicAccessCidrs = []string{"0.0.0.0/0"}
	c.Logging = nil
	c.EncryptionConfig = nil
	r := w6bEKSFetchOne(t, c)

	for _, code := range []domain.FindingCode{
		w6bEKSCodePublicEndpoint, w6bEKSCodeLoggingOff, w6bEKSCodeSecretsNotKMS, w6bEKSCodeVersionOld,
	} {
		pw1RequireNoFinding(t, r.Findings, code)
	}
}

// ─── catalog ────────────────────────────────────────────────────────────────

func TestW6BEKS_FindingDefsRegistered(t *testing.T) {
	w2AssertFindingDef(t, "eks", string(w6bEKSCodePublicEndpoint), w6bEKSPhrasePublicEndpoint, domain.SevBroken, "wave1")
	w2AssertFindingDef(t, "eks", string(w6bEKSCodeLoggingOff), w6bEKSPhraseLoggingOff, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "eks", string(w6bEKSCodeSecretsNotKMS), w6bEKSPhraseSecretsNotKMS, domain.SevWarn, "wave1")
	w2AssertFindingDef(t, "eks", string(w6bEKSCodeVersionOld), "Kubernetes <version> is out of standard support", domain.SevBroken, "wave1")
}
