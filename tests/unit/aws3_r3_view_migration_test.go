// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// aws3_r3_view_migration_test.go — task aws3 round 3. A built-in column whose
// SOURCE changed never reached an operator who had already run a9s once: the
// merge only ever added a column whose title was missing, and both of the
// corrected SNS columns keep their titles. These pin the upgrade, and the two
// cases it must not touch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"

	"context"
)

// aws3ViewsAt writes one view file exactly as the named build generated it and
// returns the directory, so the migration runs over what an operator's disk
// actually holds rather than over a hand-written approximation.
func aws3ViewsAt(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0600); err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
	return dir
}

// The two files as the build before this task generated them.
const (
	aws3SNSViewV1 = `generated: 1
list:
  Topic Name:
    path: TopicArn
    width: 40
  Status:
    width: 12
  Subs:
    key: subs_count
    width: 6
  Topic ARN:
    path: TopicArn
    width: 60

detail:
  - TopicArn
  - Attributes
`
	aws3SNSSubViewV1 = `generated: 1
list:
  Topic ARN:
    path: TopicArn
    width: 48
  Status:
    width: 12
  Protocol:
    path: Protocol
    width: 10
  Endpoint:
    path: Endpoint
    width: 48
  Confirmed:
    path: SubscriptionArn
    width: 22
  Subscription ARN:
    path: SubscriptionArn
    width: 60

detail:
  - SubscriptionArn
  - TopicArn
  - Protocol
  - Endpoint
  - Owner
`
)

func aws3ColumnOnDisk(t *testing.T, dir, view, title string) config.ListColumn {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, view+".yaml"))
	if err != nil {
		t.Fatalf("reading %s.yaml: %v", view, err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing %s.yaml: %v", view, err)
	}
	for _, c := range vd.List {
		if c.Title == title {
			return c
		}
	}
	t.Fatalf("%s.yaml has no %q column", view, title)
	return config.ListColumn{}
}

// TestUpgradeCarriesACorrectedColumnSource pins the reject: a view file an
// earlier build wrote takes this build's source and width for a column it
// still carries under the same title.
func TestUpgradeCarriesACorrectedColumnSource(t *testing.T) {
	dir := aws3ViewsAt(t, "sns", aws3SNSViewV1)
	if err := os.WriteFile(filepath.Join(dir, "sns-sub.yaml"), []byte(aws3SNSSubViewV1), 0600); err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	name := aws3ColumnOnDisk(t, dir, "sns", "Topic Name")
	if name.Key != "display_name" || name.Path != "" {
		t.Errorf("sns Topic Name on disk = {path:%q key:%q}, want the corrected key display_name", name.Path, name.Key)
	}
	confirmed := aws3ColumnOnDisk(t, dir, "sns-sub", "Confirmed")
	if confirmed.Key != "confirmed" || confirmed.Path != "" {
		t.Errorf("sns-sub Confirmed on disk = {path:%q key:%q}, want the corrected key confirmed", confirmed.Path, confirmed.Key)
	}
	if confirmed.Width != 12 {
		t.Errorf("sns-sub Confirmed width = %d, want 12 — the cell holds a word now, not an ARN", confirmed.Width)
	}
}

// TestUpgradeLeavesAnOperatorsOwnSourceAlone pins the other half: a column
// whose source is not the one the old build generated is the operator's, and
// an upgrade that overwrote it would be taking their configuration away.
func TestUpgradeLeavesAnOperatorsOwnSourceAlone(t *testing.T) {
	mine := strings.Replace(aws3SNSViewV1,
		"  Topic Name:\n    path: TopicArn\n    width: 40\n",
		"  Topic Name:\n    key: topic_arn\n    width: 25\n", 1)
	dir := aws3ViewsAt(t, "sns", mine)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	got := aws3ColumnOnDisk(t, dir, "sns", "Topic Name")
	if got.Key != "topic_arn" || got.Width != 25 {
		t.Errorf("sns Topic Name = {key:%q width:%d}, want the operator's own {key:topic_arn width:25}", got.Key, got.Width)
	}
}

// TestUpgradeLeavesACurrentFileByteForByte pins that a file already stamped
// with this build is not rewritten at all.
func TestUpgradeLeavesACurrentFileByteForByte(t *testing.T) {
	dir := t.TempDir()
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	path := filepath.Join(dir, "sns.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir (second pass): %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a file at the current stamp was rewritten:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

// aws3CellFromDir renders one column of one row through the view files in dir,
// which is the path the app takes on an operator's machine.
func aws3CellFromDir(t *testing.T, dir, shortName, title string, r resource.Resource) string {
	t.Helper()
	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Fatalf("LoadFromDirs: %v", err)
	}
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	for _, col := range resource.ResolveListColumnCascade(cfg, shortName, td) {
		if col.Title == title {
			return app.ExtractCellValue(app.ColumnDef{
				Title: col.Title, Key: col.Key, Path: col.Path, Width: col.Width,
			}, td, r)
		}
	}
	t.Fatalf("%s has no %q column", shortName, title)
	return ""
}

// TestEarlierBuildStillShowsTheNameAndTheConfirmationState is the two row-9
// pins on an existing installation: after the upgrade, the rendered cells say
// what their headings say.
func TestEarlierBuildStillShowsTheNameAndTheConfirmationState(t *testing.T) {
	dir := aws3ViewsAt(t, "sns", aws3SNSViewV1)
	if err := os.WriteFile(filepath.Join(dir, "sns-sub.yaml"), []byte(aws3SNSSubViewV1), 0600); err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	topics, err := awsclient.FetchSNSTopicsPage(context.Background(), fakes.NewSNS(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range topics.Resources {
		if got := aws3CellFromDir(t, dir, "sns", "Topic Name", r); got != r.Name {
			t.Errorf("after upgrading a v1 file, sns Topic Name = %q, want the name %q", got, r.Name)
		}
	}

	subs, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), fakes.NewSNS(), "")
	if err != nil {
		t.Fatal(err)
	}
	var sawYes bool
	for _, r := range subs.Resources {
		got := aws3CellFromDir(t, dir, "sns-sub", "Confirmed", r)
		if strings.HasPrefix(got, "arn:") {
			t.Errorf("after upgrading a v1 file, sns-sub Confirmed = %q — still the ARN", got)
		}
		if got == "yes" {
			sawYes = true
		}
	}
	if !sawYes {
		t.Error("no confirmed subscription rendered \"yes\" after the upgrade")
	}
}

// TestUpgradeKeepsAWidthTheOperatorSet pins the narrower half of the width
// rule: the corrected source reaches a column nobody re-sourced, but a width
// they widened is still theirs. Found by probing the migration before the
// gates — the first form reverted it.
func TestUpgradeKeepsAWidthTheOperatorSet(t *testing.T) {
	wider := strings.Replace(aws3SNSViewV1,
		"  Topic Name:\n    path: TopicArn\n    width: 40\n",
		"  Topic Name:\n    path: TopicArn\n    width: 99\n", 1)
	dir := aws3ViewsAt(t, "sns", wider)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	got := aws3ColumnOnDisk(t, dir, "sns", "Topic Name")
	if got.Key != "display_name" {
		t.Errorf("sns Topic Name key = %q, want the corrected display_name", got.Key)
	}
	if got.Width != 99 {
		t.Errorf("sns Topic Name width = %d, want the 99 the operator set", got.Width)
	}
}
