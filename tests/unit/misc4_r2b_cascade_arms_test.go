// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r2b_cascade_arms_test.go — ResolveListColumnCascade reads one
// declaration one way: the catalog's Key is kept, and the view's own Path and
// SortKey are kept beside it, so the same declaration never produces
// different columns for reasons that have nothing to do with the type.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// misc4ResolvedColumn returns the resolved column with the given title.
func misc4ResolvedColumn(t *testing.T, short, title string) config.ListColumn {
	t.Helper()
	td := resource.FindResourceType(short)
	if td == nil {
		t.Fatalf("%s not registered", short)
	}
	for _, c := range resource.ResolveListColumnCascade(nil, short, td) {
		if strings.EqualFold(c.Title, title) {
			return c
		}
	}
	t.Fatalf("%s has no resolved column titled %q", short, title)
	return config.ListColumn{}
}

// TestCatalogKeySurvivesTheSupersetArm pins that a catalog Key reaches the
// resolved column, and that a column whose key the fetcher writes reads no
// path: Fields is its one source.
func TestCatalogKeySurvivesTheSupersetArm(t *testing.T) {
	got := misc4ResolvedColumn(t, "acm", "Domain Name")
	if got.Key != "domain_name" {
		t.Errorf("acm \"Domain Name\" resolved with Key %q, want \"domain_name\"", got.Key)
	}
	if got.Path != "" {
		t.Errorf("acm \"Domain Name\" resolved with Path %q beside the produced key domain_name, want none", got.Path)
	}
}

// TestDefaultOnlyColumnKeepsItsOwnKey pins that a column the catalog does not
// declare at all keeps the Key the defaults gave it, so the merge adds the
// catalog's identity without erasing the defaults' own columns.
func TestDefaultOnlyColumnKeepsItsOwnKey(t *testing.T) {
	got := misc4ResolvedColumn(t, "acm", "Days Left")
	if got.Key != "days_left" {
		t.Errorf("acm \"Days Left\" resolved with Key %q, want \"days_left\"", got.Key)
	}
}

func TestCatalogArmStillBorrowsTheDefaultsHints(t *testing.T) {
	enc := misc4ResolvedColumn(t, "efs", "Encrypted")
	if enc.Key != "encrypted" || enc.Path != "" {
		t.Errorf("efs \"Encrypted\" resolved as {Key %q, Path %q}, want {encrypted, none}", enc.Key, enc.Path)
	}
	region := misc4ResolvedColumn(t, "s3", "Region")
	if region.Key != "" || region.Path != "BucketRegion" {
		t.Errorf("s3 \"Region\" resolved as {Key %q, Path %q}, want {none, BucketRegion}", region.Key, region.Path)
	}
}
