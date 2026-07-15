package unit

// aws_lt_test.go — fetcher + Wave 2 issue-enrichment tests for EC2 Launch
// Templates (docs/resources/lt.md §3/§4, docs/resources/lt-impl-plan.md
// §0/§1).
//
// lt is an in-fetcher N+1 (DescribeLaunchTemplates + DescribeLaunchTemplate-
// Versions per template, Versions=["$Default"] — the mwaa/eks/transfer
// pattern): imdsv1, unencrypted, and details-denied are fetcher-written
// (Source "wave1"). The deprecated-ami signal alone lives in the separate
// cache-scan enricher EnrichLTDeprecatedAMI (zero SDK calls, scans the
// loaded "ami" ResourceCache). RawStruct is *awsclient.LTRaw{Template,
// DefaultVersion} — the SAME type on healthy AND degraded (details-denied)
// rows, with DefaultVersion zero-valued on the latter (no second RawStruct
// shape, unlike transfer's dual-type fallback).

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// fetchLTDemoPage fetches the shared demo fixture page. The demo set
// includes the details-denied witness (WarnLTDeniedID), so the fetch is a
// designed E5 partial success (rows + composite error naming only that
// fixture); any OTHER error fails the test.
func fetchLTDemoPage(t *testing.T) resource.FetchResult {
	t.Helper()
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fakes.NewEC2(), "")
	if err != nil && !strings.Contains(err.Error(), fixtures.WarnLTDeniedID) {
		t.Fatalf("expected only the details-denied composite error, got %v", err)
	}
	return result
}

// mustFindLTResource returns the resource with the given ID from a slice of
// fetched resources, failing the test if absent.
func mustFindLTResource(t *testing.T, resources []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// ltAsRaw asserts RawStruct is the pinned *awsclient.LTRaw pointer shape —
// the SAME type on healthy and degraded rows (docs/resources/lt-impl-plan.md
// §0), unlike transfer's dual-type (DescribedServer/ListedServer) fallback.
func ltAsRaw(t *testing.T, raw any) *awsclient.LTRaw {
	t.Helper()
	v, ok := raw.(*awsclient.LTRaw)
	if !ok {
		t.Fatalf("RawStruct = %T, want *awsclient.LTRaw", raw)
	}
	return v
}

// ltEC2Fake is a minimal EC2API fake for isolated adversarial cases not
// covered by the shared demo fixture set. It embeds the interface so only
// the two LT-relevant operations need real implementations — any other
// method call panics loudly (nil embedded interface), which doubles as a
// scope-creep guard: the lt fetcher must never call anything else.
type ltEC2Fake struct {
	awsclient.EC2API
	listOut    *ec2.DescribeLaunchTemplatesOutput
	listErr    error
	versions   map[string]*ec2.DescribeLaunchTemplateVersionsOutput
	versionErr map[string]error
}

func (f *ltEC2Fake) DescribeLaunchTemplates(
	_ context.Context, _ *ec2.DescribeLaunchTemplatesInput, _ ...func(*ec2.Options),
) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return f.listOut, f.listErr
}

func (f *ltEC2Fake) DescribeLaunchTemplateVersions(
	_ context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options),
) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	id := aws.ToString(params.LaunchTemplateId)
	if err, ok := f.versionErr[id]; ok {
		return nil, err
	}
	if out, ok := f.versions[id]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("launch template %q not found", id)
}

var _ awsclient.EC2API = (*ltEC2Fake)(nil)

// ---------------------------------------------------------------------------
// healthy_silence
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_HealthySilence(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.ProdWebLTID)

	if len(r.Findings) != 0 {
		t.Errorf("Findings: expected 0 for a healthy template, got %d: %+v", len(r.Findings), r.Findings)
	}
	if r.Fields["status"] != "" {
		t.Errorf(`Fields["status"] = %q, want "" (S4 blank on a healthy row)`, r.Fields["status"])
	}
}

// ---------------------------------------------------------------------------
// E7 sanity — Resource.ID is LaunchTemplateId; Fields["name"] is
// LaunchTemplateName (pinned contract).
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_ResourceIDAndNameMapping(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.ProdWebLTID)

	if r.ID != fixtures.ProdWebLTID {
		t.Errorf("ID = %q, want LaunchTemplateId %q", r.ID, fixtures.ProdWebLTID)
	}
	if r.Fields["name"] != "prod-web-lt" {
		t.Errorf(`Fields["name"] = %q, want %q (LaunchTemplateName)`, r.Fields["name"], "prod-web-lt")
	}
	raw := ltAsRaw(t, r.RawStruct)
	if aws.ToString(raw.Template.LaunchTemplateId) != fixtures.ProdWebLTID {
		t.Errorf("RawStruct.Template.LaunchTemplateId = %q, want %q", aws.ToString(raw.Template.LaunchTemplateId), fixtures.ProdWebLTID)
	}
}

// ---------------------------------------------------------------------------
// imdsv1_explicit
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_IMDSv1Explicit(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.WarnLTIMDSv1ID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "lt.warn.imdsv1" {
		t.Errorf("Code = %q, want %q", f.Code, "lt.warn.imdsv1")
	}
	if f.Phrase != "IMDSv1 allowed" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "IMDSv1 allowed")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Instance metadata does not require session tokens; IMDSv1 credentials are exposed to SSRF."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

// ---------------------------------------------------------------------------
// imdsv1_default — the unset-defaults-to-optional trap witness: MetadataOptions
// nil must fire the SAME finding as HttpTokens=optional explicit.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_IMDSv1DefaultTrapWitness(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.WarnLTIMDSv1DefaultID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1 (MetadataOptions nil == unset == optional), got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "lt.warn.imdsv1" {
		t.Errorf("Code = %q, want %q (nil MetadataOptions must fire the SAME imdsv1 finding as explicit optional)", f.Code, "lt.warn.imdsv1")
	}
	if f.Phrase != "IMDSv1 allowed" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "IMDSv1 allowed")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
}

// ---------------------------------------------------------------------------
// unencrypted_explicit
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_UnencryptedExplicit(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.WarnLTUnencryptedID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "lt.warn.unencrypted" {
		t.Errorf("Code = %q, want %q", f.Code, "lt.warn.unencrypted")
	}
	if f.Phrase != "EBS encryption disabled" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "EBS encryption disabled")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "A block device explicitly sets Encrypted=false; launched instances get unencrypted volumes."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

// ---------------------------------------------------------------------------
// unencrypted_nil_silent — nil Ebs.Encrypted must NEVER be treated as
// unencrypted (default-encryption accounts make nil legitimate). No shared
// demo fixture leaves Encrypted nil, so this is an isolated adversarial case.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_UnencryptedNilSilent(t *testing.T) {
	const id = "lt-0nilencrypted0001a"
	fake := &ltEC2Fake{
		listOut: &ec2.DescribeLaunchTemplatesOutput{
			LaunchTemplates: []ec2types.LaunchTemplate{
				{
					LaunchTemplateId:     aws.String(id),
					LaunchTemplateName:   aws.String("nil-encrypted-witness"),
					DefaultVersionNumber: aws.Int64(1),
					LatestVersionNumber:  aws.Int64(1),
				},
			},
		},
		versions: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			id: {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateId: aws.String(id),
						VersionNumber:    aws.Int64(1),
						DefaultVersion:   aws.Bool(true),
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
								HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired,
							},
							BlockDeviceMappings: []ec2types.LaunchTemplateBlockDeviceMapping{
								{
									DeviceName: aws.String("/dev/xvda"),
									Ebs: &ec2types.LaunchTemplateEbsBlockDevice{
										// Encrypted intentionally left nil — must NOT be
										// treated as Encrypted=false.
										VolumeSize: aws.Int32(20),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindLTResource(t, result.Resources, id)
	if len(r.Findings) != 0 {
		t.Errorf("nil Ebs.Encrypted must not raise a finding, got Findings: %+v", r.Findings)
	}
}

// ---------------------------------------------------------------------------
// imdsv1_endpoint_disabled — HttpEndpoint == disabled means the metadata
// service is unreachable entirely; HttpTokens is moot and must NOT fire the
// imdsv1 finding regardless of its value (docs/resources/lt.md §3.2). No
// shared demo fixture disables the endpoint, so this is an isolated
// adversarial case (mirrors TestFetchLaunchTemplatesPage_UnencryptedNilSilent's
// ltEC2Fake pattern).
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_IMDSv1_EndpointDisabled_NoFinding(t *testing.T) {
	const id = "lt-0endpointdisabled01"
	fake := &ltEC2Fake{
		listOut: &ec2.DescribeLaunchTemplatesOutput{
			LaunchTemplates: []ec2types.LaunchTemplate{
				{
					LaunchTemplateId:     aws.String(id),
					LaunchTemplateName:   aws.String("endpoint-disabled-witness"),
					DefaultVersionNumber: aws.Int64(1),
					LatestVersionNumber:  aws.Int64(1),
				},
			},
		},
		versions: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			id: {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateId: aws.String(id),
						VersionNumber:    aws.Int64(1),
						DefaultVersion:   aws.Bool(true),
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
								// HttpTokens is deliberately the value that WOULD fire
								// imdsv1 on its own (optional, not required) — the only
								// thing suppressing the finding must be HttpEndpoint
								// being disabled.
								HttpTokens:   ec2types.LaunchTemplateHttpTokensStateOptional,
								HttpEndpoint: ec2types.LaunchTemplateInstanceMetadataEndpointStateDisabled,
							},
						},
					},
				},
			},
		},
	}
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindLTResource(t, result.Resources, id)
	if len(r.Findings) != 0 {
		t.Errorf("HttpEndpoint=disabled (metadata service unreachable) must suppress the imdsv1 finding regardless of HttpTokens=optional, got Findings: %+v", r.Findings)
	}
}

// ---------------------------------------------------------------------------
// multi_stack — IMDSv1 + unencrypted stack on one template → ordered
// Findings + "IMDSv1 allowed (+1)" per §4 precedence.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_MultiStackOrdered(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.WarnLTMultiID)

	got := make([]string, len(r.Findings))
	for i, f := range r.Findings {
		got[i] = f.Phrase
	}
	want := []string{"IMDSv1 allowed", "EBS encryption disabled"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordered Findings phrases = %v, want %v", got, want)
	}
	if r.Fields["status"] != "IMDSv1 allowed (+1)" {
		t.Errorf(`Fields["status"] = %q, want %q`, r.Fields["status"], "IMDSv1 allowed (+1)")
	}
}

// ---------------------------------------------------------------------------
// details_denied_rich — a denied DescribeLaunchTemplateVersions keeps the
// row, built from list fields (Fields["name"], Template.DefaultVersionNumber/
// LatestVersionNumber), with DefaultVersion zero-valued and the
// lt.warn.details_denied finding appended carrying lt's own §4 sentence;
// composite error names the id.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_DetailsDeniedRich(t *testing.T) {
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fakes.NewEC2(), "")
	if err == nil {
		t.Fatal("expected a composite error naming the details-denied template, got nil")
	}
	if !strings.Contains(err.Error(), fixtures.WarnLTDeniedID) {
		t.Errorf("composite error must name %q, got: %q", fixtures.WarnLTDeniedID, err.Error())
	}

	r := mustFindLTResource(t, result.Resources, fixtures.WarnLTDeniedID)

	if r.Fields["name"] != "warn-lt-denied" {
		t.Errorf(`Fields["name"] = %q, want %q (list fields kept on a degraded row)`, r.Fields["name"], "warn-lt-denied")
	}

	raw := ltAsRaw(t, r.RawStruct)
	if aws.ToString(raw.Template.LaunchTemplateId) != fixtures.WarnLTDeniedID {
		t.Errorf("RawStruct.Template.LaunchTemplateId = %q, want %q", aws.ToString(raw.Template.LaunchTemplateId), fixtures.WarnLTDeniedID)
	}
	if aws.ToInt64(raw.Template.DefaultVersionNumber) != 2 {
		t.Errorf("RawStruct.Template.DefaultVersionNumber = %d, want 2 (list field kept)", aws.ToInt64(raw.Template.DefaultVersionNumber))
	}
	if aws.ToInt64(raw.Template.LatestVersionNumber) != 2 {
		t.Errorf("RawStruct.Template.LatestVersionNumber = %d, want 2 (list field kept)", aws.ToInt64(raw.Template.LatestVersionNumber))
	}
	if !reflect.DeepEqual(raw.DefaultVersion, ec2types.LaunchTemplateVersion{}) {
		t.Errorf("RawStruct.DefaultVersion = %+v, want the zero value (DescribeLaunchTemplateVersions was denied)", raw.DefaultVersion)
	}

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1 (details-denied only), got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != awsclient.DetailsDeniedCode("lt") {
		t.Errorf("Findings[0].Code = %q, want %q", f.Code, awsclient.DetailsDeniedCode("lt"))
	}
	if f.Phrase != "details denied" {
		t.Errorf("Findings[0].Phrase = %q, want %q", f.Phrase, "details denied")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Access to the default version was denied; only the listed fields are visible."
	if f.Detail != wantDetail {
		t.Errorf("Findings[0].Detail = %q, want %q (lt-specific S5 sentence, not the generic degraded_resource.go text)", f.Detail, wantDetail)
	}
	if r.Fields["status"] != "details denied" {
		t.Errorf(`Fields["status"] = %q, want %q`, r.Fields["status"], "details denied")
	}
}

// ---------------------------------------------------------------------------
// list_denied_is_error — AccessDenied on DescribeLaunchTemplates must never
// render as an empty successful result.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_ListDeniedIsError(t *testing.T) {
	fake := &ltEC2Fake{
		listErr: &smithy.GenericAPIError{
			Code:    "UnauthorizedOperation",
			Message: "You are not authorized to perform ec2:DescribeLaunchTemplates",
		},
	}
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fake, "")
	if err == nil {
		t.Fatal("FetchLaunchTemplatesPage must return a non-nil error when DescribeLaunchTemplates is denied — never an empty successful result")
	}
	if !strings.Contains(err.Error(), "UnauthorizedOperation") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "UnauthorizedOperation")
	}
	if len(result.Resources) != 0 {
		t.Errorf("Resources: expected 0 on error, got %d", len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// partial_describe — E5: 5 listed, 2 DescribeLaunchTemplateVersions fail →
// 5 rows (2 rich-degraded) + composite error naming both, in "N of M" form.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_PartialDescribe(t *testing.T) {
	ids := []string{"lt-partial-a", "lt-partial-b", "lt-partial-c", "lt-partial-denied", "lt-partial-missing"}
	listed := make([]ec2types.LaunchTemplate, len(ids))
	for i, id := range ids {
		listed[i] = ec2types.LaunchTemplate{
			LaunchTemplateId:     aws.String(id),
			LaunchTemplateName:   aws.String(id),
			DefaultVersionNumber: aws.Int64(1),
			LatestVersionNumber:  aws.Int64(1),
		}
	}
	healthyVersion := func(id string) *ec2.DescribeLaunchTemplateVersionsOutput {
		return &ec2.DescribeLaunchTemplateVersionsOutput{
			LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
				{
					LaunchTemplateId: aws.String(id),
					VersionNumber:    aws.Int64(1),
					DefaultVersion:   aws.Bool(true),
					LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
						MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
							HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired,
						},
					},
				},
			},
		}
	}
	fake := &ltEC2Fake{
		listOut: &ec2.DescribeLaunchTemplatesOutput{LaunchTemplates: listed},
		versions: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			"lt-partial-a": healthyVersion("lt-partial-a"),
			"lt-partial-b": healthyVersion("lt-partial-b"),
			"lt-partial-c": healthyVersion("lt-partial-c"),
		},
		versionErr: map[string]error{
			"lt-partial-denied":  &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "not authorized"},
			"lt-partial-missing": &smithy.GenericAPIError{Code: "InvalidLaunchTemplateId.NotFound", Message: "not found"},
		},
	}
	result, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fake, "")
	if err == nil {
		t.Fatal("FetchLaunchTemplatesPage must return a composite error when DescribeLaunchTemplateVersions fails for some templates")
	}
	if len(result.Resources) != 5 {
		t.Fatalf("got %d resources, want 5 — a denied/missing describe must never make a listed template vanish", len(result.Resources))
	}

	// lt-partial-denied → UnauthorizedOperation (EC2's authorization-denied
	// code) → "details denied"; lt-partial-missing → InvalidLaunchTemplateId.
	// NotFound (non-auth) → the neutral "details unavailable". A not-found
	// template must never read as an IAM denial, and EC2 denials do not use
	// the "AccessDenied" code (docs/resources/lt.md §4; DegradedDetails split).
	wantPhrase := map[string]string{"lt-partial-denied": "details denied", "lt-partial-missing": "details unavailable"}
	for _, r := range result.Resources {
		if phrase, ok := wantPhrase[r.ID]; ok {
			if len(r.Findings) != 1 || r.Findings[0].Phrase != phrase {
				t.Errorf("degraded row %q must carry exactly the %q finding, got %+v", r.ID, phrase, r.Findings)
			}
			continue
		}
		if len(r.Findings) != 0 {
			t.Errorf("healthy row %q: expected 0 findings, got %+v", r.ID, r.Findings)
		}
	}

	errStr := err.Error()
	for _, want := range []string{"lt-partial-denied", "lt-partial-missing", "UnauthorizedOperation", "InvalidLaunchTemplateId.NotFound", "2 of 5"} {
		if !strings.Contains(errStr, want) {
			t.Errorf("composite error must contain %q, got: %q", want, errStr)
		}
	}
}

// ---------------------------------------------------------------------------
// ssm_ami_no_pivot (fetcher half) — a resolve:ssm: ImageId is a healthy,
// finding-free template (the related-panel "no pivot" half lives in
// aws_lt_related_test.go).
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_SSMAmiReferenceProducesNoFinding(t *testing.T) {
	result := fetchLTDemoPage(t)
	r := mustFindLTResource(t, result.Resources, fixtures.SSMAmiLTID)

	if len(r.Findings) != 0 {
		t.Errorf("Findings: expected 0 for a healthy resolve:ssm: template, got %d: %+v", len(r.Findings), r.Findings)
	}
}

// ---------------------------------------------------------------------------
// wave3_anti — no Wave-3 out-of-scope text anywhere; default != latest with
// zero references never raises a finding on its own.
// ---------------------------------------------------------------------------

func TestFetchLaunchTemplatesPage_WaveThreeAntiTests(t *testing.T) {
	result := fetchLTDemoPage(t)

	forbidden := []string{"$Latest", "UserData", "GetInstanceProfile", "GetParameter", "DescribeImages"}
	for _, r := range result.Resources {
		for _, f := range r.Findings {
			for _, s := range forbidden {
				if strings.Contains(f.Phrase, s) || strings.Contains(f.Detail, s) {
					t.Errorf("resource %q: Finding %+v must never surface Wave-3 out-of-scope text %q", r.ID, f, s)
				}
			}
		}
	}

	// default(2) != latest(5), zero SG/BDM/NI references, healthy IMDSv2 —
	// must not raise a finding on its own (lt.md §3.1: a pending-rollout
	// latest version is display-only, never an attention signal).
	const id = "lt-0defaultnelatest01a"
	fake := &ltEC2Fake{
		listOut: &ec2.DescribeLaunchTemplatesOutput{
			LaunchTemplates: []ec2types.LaunchTemplate{
				{
					LaunchTemplateId:     aws.String(id),
					LaunchTemplateName:   aws.String("default-ne-latest-witness"),
					DefaultVersionNumber: aws.Int64(2),
					LatestVersionNumber:  aws.Int64(5),
				},
			},
		},
		versions: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			id: {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateId: aws.String(id),
						VersionNumber:    aws.Int64(2),
						DefaultVersion:   aws.Bool(true),
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
								HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired,
							},
						},
					},
				},
			},
		},
	}
	single, err := awsclient.FetchLaunchTemplatesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindLTResource(t, single.Resources, id)
	if len(r.Findings) != 0 {
		t.Errorf("default(2) != latest(5) with zero references must not raise a finding, got Findings: %+v", r.Findings)
	}
}

// ---------------------------------------------------------------------------
// EnrichLTDeprecatedAMI — cache-scan enricher (zero SDK calls, mirrors
// EnrichRoute53Zone's signature over the loaded "ami" ResourceCache).
// ---------------------------------------------------------------------------

// ltAMICache builds an "ami" ResourceCache entry from the REAL demo AMI
// fixtures (FetchAMIsPage + fakes.NewEC2()), so the enricher tests exercise
// the actual shipped fixture graph rather than a synthetic stand-in.
func ltAMICache(t *testing.T) resource.ResourceCache {
	t.Helper()
	result, err := awsclient.FetchAMIsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchAMIsPage: %v", err)
	}
	return resource.ResourceCache{
		"ami": resource.ResourceCacheEntry{Resources: result.Resources},
	}
}

func TestEnrichLTDeprecatedAMI_DeprecatedFindsFinding(t *testing.T) {
	demo := fetchLTDemoPage(t)
	lt := mustFindLTResource(t, demo.Resources, fixtures.WarnLTDeprecatedAMIID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), clients, []resource.Resource{lt}, ltAMICache(t))
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI returned error: %v", err)
	}

	findings := result.Findings[fixtures.WarnLTDeprecatedAMIID]
	if len(findings) != 1 {
		t.Fatalf("Findings[%s]: expected exactly 1, got %d: %+v", fixtures.WarnLTDeprecatedAMIID, len(findings), findings)
	}
	f := findings[0]
	if f.Code != "lt.warn.deprecated_ami" {
		t.Errorf("Code = %q, want %q", f.Code, "lt.warn.deprecated_ami")
	}
	if f.Phrase != "deprecated AMI" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "deprecated AMI")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "The default version references an AMI past its deprecation time."
	if f.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", f.Detail, wantDetail)
	}
}

func TestEnrichLTDeprecatedAMI_CurrentAMINoFinding(t *testing.T) {
	demo := fetchLTDemoPage(t)
	lt := mustFindLTResource(t, demo.Resources, fixtures.ProdWebLTID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), clients, []resource.Resource{lt}, ltAMICache(t))
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI returned error: %v", err)
	}
	if len(result.Findings[fixtures.ProdWebLTID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 (ImageId present but not past DeprecationTime), got %+v",
			fixtures.ProdWebLTID, result.Findings[fixtures.ProdWebLTID])
	}
}

func TestEnrichLTDeprecatedAMI_SSMReferenceNoFinding(t *testing.T) {
	demo := fetchLTDemoPage(t)
	lt := mustFindLTResource(t, demo.Resources, fixtures.SSMAmiLTID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), clients, []resource.Resource{lt}, ltAMICache(t))
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI returned error: %v", err)
	}
	if len(result.Findings[fixtures.SSMAmiLTID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 (resolve:ssm: reference is never a pivot), got %+v",
			fixtures.SSMAmiLTID, result.Findings[fixtures.SSMAmiLTID])
	}
}

func TestEnrichLTDeprecatedAMI_ImageIDAbsentFromCacheNoFinding(t *testing.T) {
	const id = "lt-0unknownami000001a"
	lt := resource.Resource{
		ID: id,
		RawStruct: &awsclient.LTRaw{
			Template: ec2types.LaunchTemplate{LaunchTemplateId: aws.String(id)},
			DefaultVersion: ec2types.LaunchTemplateVersion{
				LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
					ImageId: aws.String("ami-0notinanycache0001"),
				},
			},
		},
	}

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), clients, []resource.Resource{lt}, ltAMICache(t))
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI returned error: %v", err)
	}
	if len(result.Findings[id]) != 0 {
		t.Errorf("Findings[%s]: expected 0 (ImageId absent from the loaded ami cache is not deregistered), got %+v", id, result.Findings[id])
	}
}

func TestEnrichLTDeprecatedAMI_NoAMICacheLoadedSkipsSilently(t *testing.T) {
	demo := fetchLTDemoPage(t)
	lt := mustFindLTResource(t, demo.Resources, fixtures.WarnLTDeprecatedAMIID)

	clients := &awsclient.ServiceClients{EC2: fakes.NewEC2()}
	result, err := awsclient.EnrichLTDeprecatedAMI(context.Background(), clients, []resource.Resource{lt}, resource.ResourceCache{})
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI returned error: %v", err)
	}
	if len(result.Findings[fixtures.WarnLTDeprecatedAMIID]) != 0 {
		t.Errorf("Findings[%s]: expected 0 when the ami cache is not loaded, got %+v",
			fixtures.WarnLTDeprecatedAMIID, result.Findings[fixtures.WarnLTDeprecatedAMIID])
	}
}
