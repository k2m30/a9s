// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// aws3_sns_columns_test.go — spec row 9 (task aws3): a list column shows what
// its heading names. "Topic Name" showed the topic's ARN, and "Confirmed"
// showed the subscription's ARN, so two headings on the same list read from
// the same field and neither answered its own question.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cellFor renders one declared column of one row through the same extractor
// the list body uses.
func cellFor(t *testing.T, shortName, title string, r resource.Resource) string {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	for _, col := range resource.ResolveListColumnCascade(&config.ViewsConfig{}, shortName, td) {
		if col.Title != title {
			continue
		}
		return app.ExtractCellValue(app.ColumnDef{
			Title: col.Title, Key: col.Key, Path: col.Path, Width: col.Width,
		}, td, r)
	}
	t.Fatalf("%s has no %q column", shortName, title)
	return ""
}

func TestSNSTopicNameColumnShowsTheName(t *testing.T) {
	res, err := awsclient.FetchSNSTopicsPage(context.Background(), fakes.NewSNS(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("no demo sns topics")
	}
	for _, r := range res.Resources {
		got := cellFor(t, "sns", "Topic Name", r)
		if strings.HasPrefix(got, "arn:") {
			t.Errorf("Topic Name = %q for %s — the heading says name and the cell shows the ARN", got, r.ID)
		}
		if got != r.Name {
			t.Errorf("Topic Name = %q, want the topic's name %q", got, r.Name)
		}
	}
}

func TestSNSSubConfirmedColumnShowsTheConfirmationState(t *testing.T) {
	res, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), fakes.NewSNS(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) == 0 {
		t.Fatal("no demo sns subscriptions")
	}
	var sawConfirmed, sawPending bool
	for _, r := range res.Resources {
		got := cellFor(t, "sns-sub", "Confirmed", r)
		if strings.HasPrefix(got, "arn:") {
			t.Errorf("Confirmed = %q for %s — the cell shows the subscription ARN, not a confirmation state", got, r.ID)
		}
		switch r.Fields["subscription_arn"] {
		case "PendingConfirmation":
			sawPending = true
			if got != "pending" {
				t.Errorf("Confirmed = %q for an unconfirmed subscription, want %q", got, "pending")
			}
		case "Deleted", "":
		default:
			sawConfirmed = true
			if got != "yes" {
				t.Errorf("Confirmed = %q for a confirmed subscription, want %q", got, "yes")
			}
		}
	}
	if !sawConfirmed || !sawPending {
		t.Fatalf("demo subscriptions witness confirmed=%v pending=%v — both states must be on the demo list", sawConfirmed, sawPending)
	}
}
