package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// P2.1 — ACM cert expired less than 24h ago must show "expired", not "0 days".
type acmExpiredFake struct{}

func (acmExpiredFake) ListCertificates(_ context.Context, _ *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	expiredJustNow := time.Now().Add(-1 * time.Hour)
	return &acm.ListCertificatesOutput{
		CertificateSummaryList: []acmtypes.CertificateSummary{
			{
				DomainName:     aws.String("just-expired.example.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/abc"),
				NotAfter:       &expiredJustNow,
				Status:         acmtypes.CertificateStatusExpired,
			},
		},
	}, nil
}

func TestACM_DaysLeft_RecentlyExpired(t *testing.T) {
	resources, err := awsclient.FetchACMCertificates(context.Background(), acmExpiredFake{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("want 1 cert, got %d", len(resources))
	}
	got := resources[0].Fields["days_left"]
	if got != "expired" {
		t.Errorf("days_left = %q, want %q (cert expired 1h ago must not show 0 days)", got, "expired")
	}
}

// P2.2 was a regression pin for the retired `last_status` field on backup. The
// backup spec (docs/resources/backup.md §4) collapsed last_status into the
// unified Status column as part of the 2026-04-23 rewrite — no column, no
// field, no test. Removed rather than updated because there is no equivalent
// invariant to preserve: last_status always surfaced the newest job regardless
// of the 24h cutoff, but the new Status column carries only the in-window
// Wave-2 finding phrase and is blank for healthy plans.

// P3.1 — CodeArtifact ListPackages pagination: count must include all pages.
type codeArtifactPagedFake struct {
	awsclient.CodeArtifactAPI
	calls int
}

func (f *codeArtifactPagedFake) ListPackages(_ context.Context, _ *codeartifact.ListPackagesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListPackagesOutput, error) {
	f.calls++
	switch f.calls {
	case 1:
		return &codeartifact.ListPackagesOutput{
			Packages:  []codeartifacttypes.PackageSummary{{Package: aws.String("pkg1")}, {Package: aws.String("pkg2")}},
			NextToken: aws.String("token-page-2"),
		}, nil
	case 2:
		return &codeartifact.ListPackagesOutput{
			Packages: []codeartifacttypes.PackageSummary{{Package: aws.String("pkg3")}},
		}, nil
	}
	return &codeartifact.ListPackagesOutput{}, nil
}

func (f *codeArtifactPagedFake) GetRepositoryPermissionsPolicy(_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	return nil, &codeartifacttypes.ResourceNotFoundException{}
}

func TestCodeArtifact_PackageCount_FollowsAllPages(t *testing.T) {
	fake := &codeArtifactPagedFake{}
	clients := &awsclient.ServiceClients{CodeArtifact: fake}
	res := []resource.Resource{{
		ID:     "repo1",
		Fields: map[string]string{"repo_name": "repo1", "domain_name": "shared"},
	}}
	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(), clients, res)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	updates, ok := result.FieldUpdates["repo1"]
	if !ok {
		t.Fatal("expected FieldUpdates for repo1")
	}
	if updates["package_count"] != "3" {
		t.Errorf("package_count = %q, want %q (must include both pages: 2+1)", updates["package_count"], "3")
	}
	if fake.calls < 2 {
		t.Errorf("ListPackages called %d times; want >=2 (pagination)", fake.calls)
	}
}

// P3.2 — SNS subs_count pagination: must include all pages.
type snsPagedFake struct {
	awsclient.SNSAPI
	calls int
}

func (f *snsPagedFake) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	f.calls++
	switch f.calls {
	case 1:
		return &sns.ListSubscriptionsByTopicOutput{
			Subscriptions: []snstypes.Subscription{
				{SubscriptionArn: aws.String("arn:aws:sns:::sub1")},
				{SubscriptionArn: aws.String("arn:aws:sns:::sub2")},
			},
			NextToken: aws.String("token-page-2"),
		}, nil
	case 2:
		return &sns.ListSubscriptionsByTopicOutput{
			Subscriptions: []snstypes.Subscription{
				{SubscriptionArn: aws.String("arn:aws:sns:::sub3")},
				{SubscriptionArn: aws.String("arn:aws:sns:::sub4")},
				{SubscriptionArn: aws.String("arn:aws:sns:::sub5")},
			},
		}, nil
	}
	return &sns.ListSubscriptionsByTopicOutput{}, nil
}

func TestSNS_SubsCount_FollowsAllPages(t *testing.T) {
	fake := &snsPagedFake{}
	clients := &awsclient.ServiceClients{SNS: fake}
	res := []resource.Resource{{ID: "arn:aws:sns:us-east-1:123456789012:topic1"}}
	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, res)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	got := result.FieldUpdates["arn:aws:sns:us-east-1:123456789012:topic1"]["subs_count"]
	if got != "5" {
		t.Errorf("subs_count = %q, want %q (must include both pages: 2+3)", got, "5")
	}
	if fake.calls < 2 {
		t.Errorf("ListSubscriptionsByTopic called %d times; want >=2 (pagination)", fake.calls)
	}
}
