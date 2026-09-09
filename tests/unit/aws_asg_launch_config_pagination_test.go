package unit

// aws_asg_launch_config_pagination_test.go — asgLaunchConfigurationPosture
// batches one DescribeLaunchConfigurations for every group on screen, so what
// that walk does with a page it could not read decides what up to
// EnrichmentCap rows report.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// asgLaunchConfigPagesFake answers DescribeLaunchConfigurations from a
// scripted page list. DescribeScalingActivities answers empty for every
// group, so the activities half of the enricher contributes nothing and the
// assertions below are about the launch-configuration walk alone.
type asgLaunchConfigPagesFake struct {
	awsclient.ASGAPI

	mu sync.Mutex
	// pages are returned in order; the last one is repeated when endless is
	// set, so a walk that never sees an empty NextToken runs to the cap.
	pages    []*autoscaling.DescribeLaunchConfigurationsOutput
	errAt    int // 1-based page number that fails; 0 means no page fails
	endless  bool
	calls    int
	lastPage *autoscaling.DescribeLaunchConfigurationsOutput
}

func (f *asgLaunchConfigPagesFake) DescribeScalingActivities(
	_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options),
) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *asgLaunchConfigPagesFake) DescribeLaunchConfigurations(
	_ context.Context, _ *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options),
) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.errAt == f.calls {
		return nil, errors.New("asg: describe launch configurations failed on page 2")
	}
	if f.calls <= len(f.pages) {
		f.lastPage = f.pages[f.calls-1]
		return f.lastPage, nil
	}
	if f.endless && f.lastPage != nil {
		return f.lastPage, nil
	}
	return &autoscaling.DescribeLaunchConfigurationsOutput{}, nil
}

func (f *asgLaunchConfigPagesFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

var _ awsclient.ASGAPI = (*asgLaunchConfigPagesFake)(nil)

// asgGroupOn builds a group row referencing lcName.
func asgGroupOn(id, lcName string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   id,
		Fields: map[string]string{"asg_name": id, "status": "active"},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:    aws.String(id),
			LaunchConfigurationName: aws.String(lcName),
		},
	}
}

// imdsv1Config is a launch configuration that trips the IMDSv1 rule: no
// MetadataOptions at all means tokens are optional.
func imdsv1Config(name string) asgtypes.LaunchConfiguration {
	return asgtypes.LaunchConfiguration{LaunchConfigurationName: aws.String(name)}
}

// TestASGLaunchConfigPosture_PageErrorLeavesEveryGroupUninspected pins that a
// page failing part-way through the walk reports every group in the batch as
// uninspected rather than reporting what the earlier pages happened to say.
//
// The call is one batch for all the groups on screen, so a partial read
// cannot distinguish "this configuration is fine" from "this configuration
// was on the page that never arrived". Applying page one's findings and
// staying silent about the rest would render every remaining group as
// inspected-and-clean, which is the one thing a failed check must never claim.
func TestASGLaunchConfigPosture_PageErrorLeavesEveryGroupUninspected(t *testing.T) {
	fake := &asgLaunchConfigPagesFake{
		pages: []*autoscaling.DescribeLaunchConfigurationsOutput{
			{
				LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config("lc-first")},
				NextToken:            aws.String("page-2"),
			},
		},
		errAt: 2,
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		asgGroupOn("asg-first", "lc-first"),
		asgGroupOn("asg-second", "lc-second"),
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("a failed page must be reported as an error, not swallowed")
	}
	if fake.callCount() != 2 {
		t.Fatalf("DescribeLaunchConfigurations called %d times, want 2 (the walk must follow NextToken)", fake.callCount())
	}
	// Page one's configuration is deliberately dropped: it was read, but the
	// batch it belongs to was not.
	for _, id := range []string{"asg-first", "asg-second"} {
		if fs := result.Findings[id]; len(fs) != 0 {
			t.Errorf("%s carries %d finding(s) from a partial read, want none: %+v", id, len(fs), fs)
		}
		if _, marked := result.TruncatedIDs[id]; !marked {
			t.Errorf("%s is not marked uninspected, so its row renders as inspected-and-clean", id)
		}
	}
	if !result.Truncated {
		t.Error("result.Truncated = false after a failed page, want true")
	}
}

// TestASGLaunchConfigPosture_ReadsEveryPage is the negative counterpart: with
// every page answered, the configuration on the second page reaches its group.
// Without it the test above would pass on an enricher that gave up after page
// one for any reason at all.
func TestASGLaunchConfigPosture_ReadsEveryPage(t *testing.T) {
	fake := &asgLaunchConfigPagesFake{
		pages: []*autoscaling.DescribeLaunchConfigurationsOutput{
			{
				LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config("lc-first")},
				NextToken:            aws.String("page-2"),
			},
			{LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config("lc-second")}},
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		asgGroupOn("asg-first", "lc-first"),
		asgGroupOn("asg-second", "lc-second"),
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []string{"asg-first", "asg-second"} {
		fs := result.Findings[id]
		if len(fs) != 1 {
			t.Fatalf("%s carries %d finding(s), want 1", id, len(fs))
		}
		if fs[0].Phrase != "launch configuration allows IMDSv1" {
			t.Errorf("%s phrase = %q, want %q", id, fs[0].Phrase, "launch configuration allows IMDSv1")
		}
		if _, marked := result.TruncatedIDs[id]; marked {
			t.Errorf("%s is marked uninspected although its page was read", id)
		}
	}
	if result.Truncated {
		t.Error("result.Truncated = true although the whole walk succeeded")
	}
}

// TestASGLaunchConfigPosture_PageCapExhaustedMarksTheCountALowerBound pins the
// other end of the walk. When the pages do not run out before
// PerParentPageCap, the configurations behind the cap were never read, and the
// groups that reference them are as uninspected as the ones behind a failed
// page. The result must say so — ebsSnapPublicShares sets Truncated on the
// same fall-through (ebs_snap_issue_enrichment.go:101), and this enricher can
// emit an issue-severity finding (the user-data credential rule), so the
// count really is a lower bound.
func TestASGLaunchConfigPosture_PageCapExhaustedMarksTheCountALowerBound(t *testing.T) {
	fake := &asgLaunchConfigPagesFake{
		pages: []*autoscaling.DescribeLaunchConfigurationsOutput{
			{
				LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config("lc-first")},
				NextToken:            aws.String("more"),
			},
		},
		endless: true,
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{asgGroupOn("asg-first", "lc-first")}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.callCount(); got != awsclient.PerParentPageCap {
		t.Fatalf("DescribeLaunchConfigurations called %d times, want %d (PerParentPageCap)", got, awsclient.PerParentPageCap)
	}
	if !result.Truncated {
		t.Error("result.Truncated = false after the walk hit the page cap with pages still unread, want true")
	}
}

// TestASGLaunchConfigPosture_PageCapMarksOnlyTheGroupsItNeverRead pins the
// distinction the cap has to make. A group whose launch configuration arrived
// before the cap was inspected, and hiding its finding behind a "?" would lose
// a real one; a group whose configuration sat behind the cap was not inspected
// at all, and reporting it clean would claim a check that never ran. One walk,
// both outcomes.
func TestASGLaunchConfigPosture_PageCapMarksOnlyTheGroupsItNeverRead(t *testing.T) {
	// Every page carries a token, so the walk ends on the cap rather than on
	// the pages. Only the first page carries a configuration a group
	// references; the rest are unrelated, so "lc-behind-the-cap" is never
	// read.
	pages := []*autoscaling.DescribeLaunchConfigurationsOutput{{
		LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config("lc-arrived")},
		NextToken:            aws.String("more"),
	}}
	for i := 2; i <= awsclient.PerParentPageCap; i++ {
		pages = append(pages, &autoscaling.DescribeLaunchConfigurationsOutput{
			LaunchConfigurations: []asgtypes.LaunchConfiguration{imdsv1Config(fmt.Sprintf("lc-unrelated-%d", i))},
			NextToken:            aws.String("more"),
		})
	}
	fake := &asgLaunchConfigPagesFake{pages: pages}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		asgGroupOn("asg-arrived", "lc-arrived"),
		asgGroupOn("asg-behind-the-cap", "lc-behind-the-cap"),
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.callCount(); got != awsclient.PerParentPageCap {
		t.Fatalf("DescribeLaunchConfigurations called %d times, want %d (PerParentPageCap)", got, awsclient.PerParentPageCap)
	}
	if !result.Truncated {
		t.Error("result.Truncated = false although configurations were left unread behind the cap")
	}

	arrived := result.Findings["asg-arrived"]
	if len(arrived) != 1 {
		t.Fatalf("asg-arrived carries %d finding(s), want 1 — its configuration was read before the cap: %+v", len(arrived), arrived)
	}
	if arrived[0].Phrase != "launch configuration allows IMDSv1" {
		t.Errorf("asg-arrived phrase = %q, want %q", arrived[0].Phrase, "launch configuration allows IMDSv1")
	}
	if _, marked := result.TruncatedIDs["asg-arrived"]; marked {
		t.Error("asg-arrived is marked uninspected although its configuration arrived — the finding it earned would render as a bare \"?\"")
	}

	if fs := result.Findings["asg-behind-the-cap"]; len(fs) != 0 {
		t.Errorf("asg-behind-the-cap carries %d finding(s) although its configuration was never read: %+v", len(fs), fs)
	}
	if _, marked := result.TruncatedIDs["asg-behind-the-cap"]; !marked {
		t.Error("asg-behind-the-cap is not marked uninspected, so it renders as inspected-and-clean on a check that never ran")
	}
}
