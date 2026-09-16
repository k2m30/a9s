package unit_test

// s3_0916_row1_bucket_acl_public_test.go pins the bucket posture scan reading
// the bucket ACL as a second route to "publicly accessible".
//
// GetBucketPolicyStatus answers only about the policy. A bucket carrying a
// legacy grant to AllUsers or AuthenticatedUsers is open to the internet with
// no policy at all, and a scan that reads the policy alone renders it as
// merely "public access block incomplete" — a warning, where the truth is a
// broken-tier finding. The bucket's own IgnorePublicAcls is the one setting
// that makes such a grant inert; BlockPublicAcls only refuses new ones.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	row1Bucket   = "acme-public-site"
	row1OwnerID  = "c1f2e3d4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2"
	row1AllUsers = "http://acs.amazonaws.com/groups/global/AllUsers"
	row1AuthUser = "http://acs.amazonaws.com/groups/global/AuthenticatedUsers"

	row1CodePublic = domain.FindingCode("s3.public")
	row1CodePAB    = domain.FindingCode("s3.public-access-block-incomplete")
	row1ACLLabel   = "Access control list"
	row1PolicyRow  = "Policy status"
)

// row1S3Fake answers the per-bucket posture calls. Every call other than the
// public-access block, the policy status and the ACL answers "configured", so
// only the conditions under test can fire.
type row1S3Fake struct {
	awsclient.S3API
	pab          *s3types.PublicAccessBlockConfiguration
	policyPublic bool
	grants       []s3types.Grant
	aclErr       error
}

func (f *row1S3Fake) GetPublicAccessBlock(
	_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	if f.pab == nil {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchPublicAccessBlockConfiguration",
			Message: "The public access block configuration was not found",
		}
	}
	return &s3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: f.pab}, nil
}

func (f *row1S3Fake) GetBucketPolicyStatus(
	_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options),
) (*s3.GetBucketPolicyStatusOutput, error) {
	if !f.policyPublic {
		return nil, &smithy.GenericAPIError{Code: "NoSuchBucketPolicy", Message: "The bucket policy does not exist"}
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: &s3types.PolicyStatus{IsPublic: aws.Bool(true)},
	}, nil
}

func (f *row1S3Fake) GetBucketAcl(
	_ context.Context, _ *s3.GetBucketAclInput, _ ...func(*s3.Options),
) (*s3.GetBucketAclOutput, error) {
	if f.aclErr != nil {
		return nil, f.aclErr
	}
	return &s3.GetBucketAclOutput{
		Owner:  &s3types.Owner{ID: aws.String(row1OwnerID), DisplayName: aws.String("acme-ops")},
		Grants: f.grants,
	}, nil
}

func (f *row1S3Fake) GetBucketVersioning(
	_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options),
) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{
		Status:    s3types.BucketVersioningStatusEnabled,
		MFADelete: s3types.MFADeleteStatusEnabled,
	}, nil
}

func (f *row1S3Fake) GetBucketLogging(
	_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options),
) (*s3.GetBucketLoggingOutput, error) {
	return &s3.GetBucketLoggingOutput{
		LoggingEnabled: &s3types.LoggingEnabled{
			TargetBucket: aws.String("acme-access-logs"),
			TargetPrefix: aws.String(row1Bucket + "/"),
		},
	}, nil
}

func (f *row1S3Fake) GetBucketLifecycleConfiguration(
	_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options),
) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{
		Rules: []s3types.LifecycleRule{{
			ID:         aws.String("expire-old"),
			Status:     s3types.ExpirationStatusEnabled,
			Filter:     &s3types.LifecycleRuleFilter{Prefix: aws.String("")},
			Expiration: &s3types.LifecycleExpiration{Days: aws.Int32(365)},
		}},
	}, nil
}

func (f *row1S3Fake) GetObjectLockConfiguration(
	_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options),
) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
			ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
		},
	}, nil
}

// The spec names this interface as the seam GetBucketAcl joins the posture
// scan through; the assertion keeps the name from drifting to an inline
// method set.
var _ awsclient.S3GetBucketAclAPI = (*row1S3Fake)(nil)

func row1GroupGrant(uri string, perm s3types.Permission) s3types.Grant {
	return s3types.Grant{
		Grantee:    &s3types.Grantee{Type: s3types.TypeGroup, URI: aws.String(uri)},
		Permission: perm,
	}
}

func row1OwnerGrant() s3types.Grant {
	return s3types.Grant{
		Grantee: &s3types.Grantee{
			Type:        s3types.TypeCanonicalUser,
			ID:          aws.String(row1OwnerID),
			DisplayName: aws.String("acme-ops"),
		},
		Permission: s3types.PermissionFullControl,
	}
}

func row1Run(t *testing.T, fake *row1S3Fake) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	clients := &awsclient.ServiceClients{S3: fake, Region: "us-east-1"}
	res := []resource.Resource{{
		ID:     row1Bucket,
		Name:   row1Bucket,
		Fields: map[string]string{"name": row1Bucket},
	}}
	return awsclient.EnrichS3Posture(context.Background(), clients, res, nil)
}

func row1Finding(r awsclient.IssueEnricherResult, code domain.FindingCode) (domain.Finding, bool) {
	for _, f := range r.Findings[row1Bucket] {
		if f.Code == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

func row1Rows(r awsclient.IssueEnricherResult, code domain.FindingCode) []domain.DetailRow {
	return r.AttentionDetails[row1Bucket][code].Rows
}

func row1RowValue(r awsclient.IssueEnricherResult, code domain.FindingCode, label string) (string, bool) {
	for _, row := range row1Rows(r, code) {
		if row.Label == label {
			return row.Value, true
		}
	}
	return "", false
}

func TestS3_0916_Row1_AllUsersGrantMakesTheBucketPublic(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{grants: []s3types.Grant{
		row1OwnerGrant(),
		row1GroupGrant(row1AllUsers, s3types.PermissionRead),
	}})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}

	f, ok := row1Finding(got, row1CodePublic)
	if !ok {
		t.Fatalf("findings = %v, want one carrying %s", got.Findings[row1Bucket], row1CodePublic)
	}
	if f.Phrase != "publicly accessible" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "publicly accessible")
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", f.Severity)
	}
	if v, found := row1RowValue(got, row1CodePublic, row1ACLLabel); !found || v != "AllUsers read" {
		t.Errorf("row %q = %q (found=%v), want %q", row1ACLLabel, v, found, "AllUsers read")
	}
	if _, ok := row1Finding(got, row1CodePAB); !ok {
		t.Error("the public-access-block finding disappeared; the ACL route is an addition, not a replacement")
	}
}

func TestS3_0916_Row1_AuthenticatedUsersGrantMakesTheBucketPublic(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{grants: []s3types.Grant{
		row1GroupGrant(row1AuthUser, s3types.PermissionWrite),
	}})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if _, ok := row1Finding(got, row1CodePublic); !ok {
		t.Fatalf("findings = %v, want one carrying %s", got.Findings[row1Bucket], row1CodePublic)
	}
	if v, found := row1RowValue(got, row1CodePublic, row1ACLLabel); !found || v != "AuthenticatedUsers write" {
		t.Errorf("row %q = %q (found=%v), want %q", row1ACLLabel, v, found, "AuthenticatedUsers write")
	}
}

func TestS3_0916_Row1_IgnorePublicAclsNeutralisesTheGrant(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		pab: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(false),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(false),
			RestrictPublicBuckets: aws.Bool(false),
		},
		grants: []s3types.Grant{row1GroupGrant(row1AllUsers, s3types.PermissionRead)},
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if _, ok := row1Finding(got, row1CodePublic); ok {
		t.Errorf("findings = %v, want no %s — the bucket ignores public ACLs", got.Findings[row1Bucket], row1CodePublic)
	}
	if rows := row1Rows(got, row1CodePublic); len(rows) != 0 {
		t.Errorf("rows for %s = %v, want none", row1CodePublic, rows)
	}
}

func TestS3_0916_Row1_BlockPublicAclsDoesNotNeutraliseAnExistingGrant(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		pab: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(false),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
		grants: []s3types.Grant{row1GroupGrant(row1AllUsers, s3types.PermissionRead)},
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if _, ok := row1Finding(got, row1CodePublic); !ok {
		t.Fatalf("findings = %v, want %s — BlockPublicAcls refuses new grants, it does not disarm this one",
			got.Findings[row1Bucket], row1CodePublic)
	}
}

func TestS3_0916_Row1_OwnerOnlyGrantIsNotPublic(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		pab: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
		grants: []s3types.Grant{row1OwnerGrant()},
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if len(got.Findings[row1Bucket]) != 0 {
		t.Fatalf("findings = %v, want none — a canonical-user grant to the owner is the default ACL", got.Findings[row1Bucket])
	}
}

func TestS3_0916_Row1_PublicByPolicyAndByACLFiresOnceWithBothRows(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		policyPublic: true,
		grants:       []s3types.Grant{row1GroupGrant(row1AllUsers, s3types.PermissionRead)},
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}

	n := 0
	for _, f := range got.Findings[row1Bucket] {
		if f.Code == row1CodePublic {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%s emitted %d times, want 1 — one condition, two supporting rows", row1CodePublic, n)
	}
	if v, found := row1RowValue(got, row1CodePublic, row1PolicyRow); !found || v != "public" {
		t.Errorf("row %q = %q (found=%v), want %q", row1PolicyRow, v, found, "public")
	}
	if v, found := row1RowValue(got, row1CodePublic, row1ACLLabel); !found || v != "AllUsers read" {
		t.Errorf("row %q = %q (found=%v), want %q", row1ACLLabel, v, found, "AllUsers read")
	}
}

func TestS3_0916_Row1_ACLDeniedIsAFailureAndLeavesTheRestIntact(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		aclErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: s3:GetBucketAcl"},
	})
	if err == nil {
		t.Fatal("error = nil, want the refusal aggregated")
	}
	if !strings.Contains(err.Error(), row1Bucket) {
		t.Errorf("error = %q, want it to name bucket %q", err.Error(), row1Bucket)
	}
	if _, ok := row1Finding(got, row1CodePublic); ok {
		t.Errorf("findings = %v, want no %s — the ACL never answered", got.Findings[row1Bucket], row1CodePublic)
	}
	if _, ok := row1Finding(got, row1CodePAB); !ok {
		t.Errorf("findings = %v, want %s — a refused ACL call does not silence the other conditions",
			got.Findings[row1Bucket], row1CodePAB)
	}
	if _, marked := got.TruncatedIDs[row1Bucket]; !marked {
		t.Error("TruncatedIDs has no entry for the bucket; the row must carry the incomplete-data marker")
	}
}

func TestS3_0916_Row1_ACLCrossRegionMarksTheBucketUnreachable(t *testing.T) {
	got, err := row1Run(t, &row1S3Fake{
		aclErr: &smithy.GenericAPIError{Code: "PermanentRedirect", Message: "The bucket is in this region: eu-west-1"},
	})
	if err != nil {
		t.Fatalf("error = %v, want nil — a cross-region bucket is operational, not a failure", err)
	}
	if _, marked := got.TruncatedIDs[row1Bucket]; !marked {
		t.Error("TruncatedIDs has no entry for the bucket; a cross-region bucket renders the \"?\" marker")
	}
	if len(got.Findings[row1Bucket]) != 0 {
		t.Errorf("findings = %v, want none — nothing was inspected", got.Findings[row1Bucket])
	}
}

func TestS3_0916_Row1_PublicFindingDetailNamesBothRoutes(t *testing.T) {
	detail := strings.ToLower(catalog.Detail(row1CodePublic))
	if detail == "" {
		t.Fatalf("catalog.Detail(%s) is empty", row1CodePublic)
	}
	if !strings.Contains(detail, "policy") {
		t.Errorf("detail = %q, want it to name the policy route", detail)
	}
	if !strings.Contains(detail, "access control list") && !strings.Contains(detail, "acl") {
		t.Errorf("detail = %q, want it to name the ACL route", detail)
	}
}
