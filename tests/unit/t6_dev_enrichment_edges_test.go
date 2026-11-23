package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func t6DevStoredField(t *testing.T, a *t6App, id, key string) (string, bool) {
	t.Helper()
	rows, _ := a.core.ProbeResources(a.short)
	for _, r := range rows {
		if r.ID == id {
			v, ok := r.Fields[key]
			return v, ok
		}
	}
	t.Fatalf("%s %s is not in the row store", a.short, id)
	return "", false
}

// An answer that errored and carries nothing answered for nobody: the
// declared fields an earlier answer wrote stand, because no row was judged.
func TestT6Dev_ErroredEmptyAnswerKeepsDeclaredFields(t *testing.T) {
	rows := t6DemoRows(t, "s3")[:2]
	target := rows[0]
	a := t6NewApp(t, "s3", &awsclient.ServiceClients{S3: &t6S3Posture{partial: map[string]bool{target.Name: true}}}, rows, nil, true)
	a.wave2(t)
	if v, _ := t6DevStoredField(t, a, target.ID, "status"); v != "public access block incomplete" {
		t.Fatalf("precondition: status = %q after the first answer", v)
	}

	awsclient.SetWave2EnricherForTest(t, "s3", awsclient.IssueEnricher{Fn: func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{}, errors.New("s3: every read refused")
	}})
	a.wave2(t)
	if v, _ := t6DevStoredField(t, a, target.ID, "status"); v != "public access block incomplete" {
		t.Errorf("status = %q after an answer for nobody, want the earlier value kept", v)
	}
}

type t6DevS3Refused struct {
	*t6S3Posture
	refuse map[string]bool
}

func (f *t6DevS3Refused) GetPublicAccessBlock(ctx context.Context, in *s3.GetPublicAccessBlockInput, opts ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	if f.refuse[*in.Bucket] {
		return nil, t576Denied("S3", "GetPublicAccessBlock")
	}
	return f.t6S3Posture.GetPublicAccessBlock(ctx, in, opts...)
}

// A row the enricher marked not inspected was not judged: its declared
// fields keep what the last answer that read it wrote.
func TestT6Dev_MarkedRowKeepsDeclaredFields(t *testing.T) {
	rows := t6DemoRows(t, "s3")[:2]
	target := rows[0]
	posture := &t6S3Posture{partial: map[string]bool{target.Name: true}}
	refused := &t6DevS3Refused{t6S3Posture: posture, refuse: map[string]bool{}}
	a := t6NewApp(t, "s3", &awsclient.ServiceClients{S3: refused}, rows, nil, true)
	a.wave2(t)

	posture.partial[target.Name] = false
	refused.refuse[target.Name] = true
	ev := a.wave2(t)
	if _, marked := ev.TruncatedIDs[target.ID]; !marked {
		t.Fatalf("precondition: %s is not marked after its read was refused: %v", target.ID, ev.TruncatedIDs)
	}
	if v, _ := t6DevStoredField(t, a, target.ID, "status"); v != "public access block incomplete" {
		t.Errorf("status = %q on a row the answer did not read, want the earlier value kept", v)
	}
	if v, ok := t6DevStoredField(t, a, rows[1].ID, "status"); !ok || v != "" {
		t.Errorf("judged row %s status = (%q, present %v), want present and empty", rows[1].ID, v, ok)
	}
}

// The one issue predicate: a Wave-1 signal counts at warning and broken, a
// Wave-2 finding only at broken.
func TestT6Dev_IsIssueRow(t *testing.T) {
	healthy := resource.ResourceTypeDef{ShortName: "t6-dev", Color: func(domain.Resource) domain.Color { return domain.ColorHealthy }}
	red := resource.ResourceTypeDef{ShortName: "t6-dev", Color: func(domain.Resource) domain.Color { return domain.ColorBroken }}
	f := func(sev domain.Severity, source string) []domain.Finding {
		return []domain.Finding{{Code: "t6.dev", Phrase: "x", Severity: sev, Source: source}}
	}
	for _, tc := range []struct {
		name string
		td   resource.ResourceTypeDef
		fs   []domain.Finding
		want bool
	}{
		{"no signal", healthy, nil, false},
		{"wave-1 issue colour", red, nil, true},
		{"wave-1 warning finding", healthy, f(domain.SevWarn, "wave1"), true},
		{"wave-1 dim finding", healthy, f(domain.SevDim, "wave1"), false},
		{"wave-2 warning", healthy, f(domain.SevWarn, "wave2:t6-dev"), false},
		{"wave-2 broken", healthy, f(domain.SevBroken, "wave2:t6-dev"), true},
		{"wave-2 dim", healthy, f(domain.SevDim, "wave2:t6-dev"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runtime.IsIssueRow(tc.td, resource.Resource{ID: "r", Findings: tc.fs}); got != tc.want {
				t.Errorf("IsIssueRow = %v, want %v", got, tc.want)
			}
		})
	}
}

// ExcludeFromIssueBadge hides a type's number and nothing else: its rows are
// counted like any type's, which is what the issues-only menu and filter read.
func TestT6Dev_ExcludedTypeRowsStillCount(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil || !td.ExcludeFromIssueBadge {
		t.Fatal("precondition: ct-events is not registered as excluded from the issue badge")
	}
	row := resource.Resource{ID: "evt-1", Findings: []domain.Finding{{Code: "t6.dev", Phrase: "x", Severity: domain.SevBroken, Source: "wave1"}}}
	if got := runtime.IssueCount(*td, []resource.Resource{row}); got != 1 {
		t.Errorf("IssueCount(ct-events) = %d, want 1", got)
	}
	if runtime.ShowsIssueBadge(*td) {
		t.Error("ShowsIssueBadge(ct-events) = true, want false")
	}
}
