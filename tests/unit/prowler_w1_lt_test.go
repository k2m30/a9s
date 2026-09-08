package unit

// prowler_w1_lt_test.go — behavioural pins for lt.user-data-secret (batch w1).
//
// The launch template's default version is already held by the row (the
// fetcher describes it), so the check reads LaunchTemplateData.UserData,
// base64-decodes it and runs the shared secret scanner over the script. The
// signal is emitted by EnrichLTDeprecatedAMI, the type's single Wave-2
// enricher.

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const pw1LTCodeUserDataSecret = domain.FindingCode("lt.user-data-secret")

// pw1LTResource builds a launch template row in the shape the fetcher emits:
// RawStruct is *LTRaw carrying the template and its "$Default" version.
func pw1LTResource(id, name string, version int64, userData string) resource.Resource {
	data := &ec2types.ResponseLaunchTemplateData{
		ImageId:      aws.String("ami-0aaaa1111bbbb2222"),
		InstanceType: ec2types.InstanceTypeT3Medium,
		MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
			HttpTokens: ec2types.LaunchTemplateHttpTokensStateRequired,
		},
	}
	if userData != "" {
		data.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(userData)))
	}
	raw := &awsclient.LTRaw{
		Template: ec2types.LaunchTemplate{
			LaunchTemplateId:     aws.String(id),
			LaunchTemplateName:   aws.String(name),
			DefaultVersionNumber: aws.Int64(version),
			LatestVersionNumber:  aws.Int64(version),
			CreatedBy:            aws.String("arn:aws:iam::123456789012:user/acme-deployer"),
			CreateTime:           aws.Time(time.Now().Add(-90 * 24 * time.Hour)),
		},
		DefaultVersion: ec2types.LaunchTemplateVersion{
			LaunchTemplateId:   aws.String(id),
			LaunchTemplateName: aws.String(name),
			VersionNumber:      aws.Int64(version),
			DefaultVersion:     aws.Bool(true),
			LaunchTemplateData: data,
		},
	}
	return resource.Resource{
		ID:        id,
		Name:      name,
		RawStruct: raw,
		Fields:    map[string]string{"name": name, "default_version": "3"},
	}
}

func pw1EnrichLT(t *testing.T, cache resource.ResourceCache, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichLTDeprecatedAMI(context.Background(),
		&awsclient.ServiceClients{EC2: fakes.NewEC2()}, rs, cache)
	if err != nil {
		t.Fatalf("EnrichLTDeprecatedAMI: %v", err)
	}
	return res
}

const pw1LTUserDataWithSecret = `#!/bin/bash
yum install -y awscli
export DB_PASSWORD=hunter2hunter2
/opt/app/start.sh
`

const pw1LTUserDataClean = `#!/bin/bash
yum install -y awscli
export DB_PASSWORD_PARAM=/acme/prod/db-password
/opt/app/start.sh
`

// TestLT_UserDataSecret_PlaintextCredential pins the Broken finding, the
// version citation and the Where:Kind row — and that the credential value
// never reaches the finding text.
func TestLT_UserDataSecret_PlaintextCredential(t *testing.T) {
	const id = "lt-0secret000aaaaa1"
	res := pw1EnrichLT(t, resource.ResourceCache{},
		pw1LTResource(id, "acme-web", 3, pw1LTUserDataWithSecret))

	f := pw1RequireFinding(t, res.Findings[id], pw1LTCodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
	rows := pw1Rows(res, id, pw1LTCodeUserDataSecret)
	pw1RequireRow(t, rows, "Version", "3")
	pw1RequireRow(t, rows, "line 3", "keyword")
	if strings.Contains(f.Phrase+f.Detail, "hunter2hunter2") {
		t.Errorf("credential value leaked into the finding text")
	}
	for _, r := range rows {
		if strings.Contains(r.Label+r.Value, "hunter2hunter2") {
			t.Errorf("credential value leaked into an AttentionDetail row: %+v", r)
		}
	}
}

// TestLT_UserDataSecret_ParameterReferenceIsHealthy pins the negative case: a
// script that names an SSM parameter carries a reference, not a secret.
func TestLT_UserDataSecret_ParameterReferenceIsHealthy(t *testing.T) {
	const id = "lt-0clean0000aaaaa1"
	res := pw1EnrichLT(t, resource.ResourceCache{},
		pw1LTResource(id, "acme-web-clean", 3, pw1LTUserDataClean))
	pw1RequireNoFinding(t, res.Findings[id], pw1LTCodeUserDataSecret)
}

// TestLT_UserDataSecret_NoUserDataIsHealthy pins that a template with no user
// data at all emits nothing.
func TestLT_UserDataSecret_NoUserDataIsHealthy(t *testing.T) {
	const id = "lt-0noud00000aaaaa1"
	res := pw1EnrichLT(t, resource.ResourceCache{},
		pw1LTResource(id, "acme-web-noud", 3, ""))
	pw1RequireNoFinding(t, res.Findings[id], pw1LTCodeUserDataSecret)
}

// TestLT_UserDataSecret_NilLaunchTemplateDataIsSilent pins the degraded row:
// a template whose default version could not be described carries no
// LaunchTemplateData, and an unknown script is not a leak.
func TestLT_UserDataSecret_NilLaunchTemplateDataIsSilent(t *testing.T) {
	const id = "lt-0denied000aaaaa1"
	r := pw1LTResource(id, "acme-web-denied", 3, "")
	r.RawStruct.(*awsclient.LTRaw).DefaultVersion.LaunchTemplateData = nil //nolint:errcheck // shape is fixed by pw1LTResource
	res := pw1EnrichLT(t, resource.ResourceCache{}, r)
	pw1RequireNoFinding(t, res.Findings[id], pw1LTCodeUserDataSecret)
}

// TestLT_UserDataSecret_FiresWithoutTheAMICache pins independence from the
// deprecated-AMI rule: the enricher's early return when the ami list has not
// been loaded must not swallow the secret check, which reads nothing but the
// row's own data.
func TestLT_UserDataSecret_FiresWithoutTheAMICache(t *testing.T) {
	const id = "lt-0nocache00aaaaa1"
	res := pw1EnrichLT(t, resource.ResourceCache{},
		pw1LTResource(id, "acme-web-nocache", 3, pw1LTUserDataWithSecret))
	pw1RequireFinding(t, res.Findings[id], pw1LTCodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
}

// TestLT_SecretAndDeprecatedAMIAreTwoFindings pins independence: a template
// that leaks a credential and points at a deprecated image carries both
// findings, each with its own rows.
func TestLT_SecretAndDeprecatedAMIAreTwoFindings(t *testing.T) {
	const id = "lt-0both00000aaaaa1"
	const amiID = "ami-0deprecated0aaa1"
	r := pw1LTResource(id, "acme-web-both", 4, pw1LTUserDataWithSecret)
	r.RawStruct.(*awsclient.LTRaw).DefaultVersion.LaunchTemplateData.ImageId = aws.String(amiID) //nolint:errcheck // shape is fixed by pw1LTResource

	cache := resource.ResourceCache{"ami": resource.ResourceCacheEntry{Resources: []resource.Resource{{
		ID: amiID,
		RawStruct: ec2types.Image{
			ImageId:         aws.String(amiID),
			DeprecationTime: aws.String(time.Now().Add(-24 * time.Hour).Format(time.RFC3339)),
		},
	}}}}

	res := pw1EnrichLT(t, cache, r)
	pw1RequireFinding(t, res.Findings[id], pw1LTCodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
	if _, ok := pw1FindFinding(res.Findings[id], domain.FindingCode("lt.warn.deprecated_ami")); !ok {
		t.Errorf("deprecated-AMI finding lost when the secret finding was added: %+v", res.Findings[id])
	}
	if len(pw1Rows(res, id, pw1LTCodeUserDataSecret)) == 0 {
		t.Errorf("secret rows crowded out by the deprecated-AMI finding's rows")
	}
}

// TestLT_UserDataSecret_EvaluatesEveryRow pins that one leaking template in a
// batch does not mask its healthy neighbours, and vice versa.
func TestLT_UserDataSecret_EvaluatesEveryRow(t *testing.T) {
	res := pw1EnrichLT(t, resource.ResourceCache{},
		pw1LTResource("lt-0clean1000aaaaa1", "acme-a", 1, pw1LTUserDataClean),
		pw1LTResource("lt-0leak11000aaaaa1", "acme-b", 2, pw1LTUserDataWithSecret),
		pw1LTResource("lt-0clean2000aaaaa1", "acme-c", 3, pw1LTUserDataClean),
	)
	pw1RequireFinding(t, res.Findings["lt-0leak11000aaaaa1"], pw1LTCodeUserDataSecret,
		"credential in user data", domain.SevBroken, "wave2")
	pw1RequireRow(t, pw1Rows(res, "lt-0leak11000aaaaa1", pw1LTCodeUserDataSecret), "Version", "2")
	for _, id := range []string{"lt-0clean1000aaaaa1", "lt-0clean2000aaaaa1"} {
		pw1RequireNoFinding(t, res.Findings[id], pw1LTCodeUserDataSecret)
	}
}

// TestLT_DemoBench_OnlyWitnessLeaksACredential pins the demo fixture contract.
func TestLT_DemoBench_OnlyWitnessLeaksACredential(t *testing.T) {
	// The demo set deliberately contains one template whose version describe
	// is denied; the fetcher reports that as an aggregated error alongside the
	// rows it did build, and those rows are what this assertion needs.
	out, _ := awsclient.FetchLaunchTemplatesPage(context.Background(), fakes.NewEC2(), "") //nolint:errcheck // partial-result contract, see above
	if len(out.Resources) == 0 {
		t.Fatal("FetchLaunchTemplatesPage(demo) returned no rows")
	}
	amis, aerr := awsclient.FetchAMIsPage(context.Background(), fakes.NewEC2(), "")
	if aerr != nil {
		t.Fatalf("FetchAMIsPage(demo): %v", aerr)
	}
	cache := resource.ResourceCache{"ami": resource.ResourceCacheEntry{Resources: amis.Resources}}

	res := pw1EnrichLT(t, cache, out.Resources...)
	var carriers []string
	for id, fs := range res.Findings {
		if _, ok := pw1FindFinding(fs, pw1LTCodeUserDataSecret); ok {
			carriers = append(carriers, id)
		}
	}
	pw1RequireOnlyWitness(t, pw1LTCodeUserDataSecret, fixtures.LTUserDataSecret, carriers)
}
