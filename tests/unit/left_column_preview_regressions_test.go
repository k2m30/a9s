package unit_test

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func makePreviewEC2Detail(t *testing.T, w, h int) views.DetailModel {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	k := keys.Default()
	cfg := config.DefaultConfig()
	d := views.NewDetail(ec2[0], "ec2", cfg, k)
	d.SetSize(w, h)
	return d
}

func mustDemoEC2(t *testing.T) []resource.Resource {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	return ec2
}
