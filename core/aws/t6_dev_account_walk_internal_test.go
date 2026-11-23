package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/resource"
)

// An account-wide walk (the dbc and dbi pending-maintenance walk, the ebs
// volume-status walk, the public-snapshot walk) answers a row the moment a
// page names it. Cut on a later page, by a refused call or by the page
// budget, it marks every row no walked page named, and keeps what the walked
// pages carried; walked to the end, a row no page named is answered "none".
func TestT6Dev_AccountWalkMarksTheRowsItNeverReached(t *testing.T) {
	rows := []resource.Resource{{ID: "a"}, {ID: "b"}}
	denied := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: rds:DescribePendingMaintenanceActions"}
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name     string
		next     func(token *string) ([]string, *string, error)
		wantErr  bool
		wantMark map[string]bool
	}{
		{
			name: "a later page refused",
			next: func(token *string) ([]string, *string, error) {
				if token == nil {
					return []string{"a"}, str("page-2"), nil
				}
				return nil, nil, denied
			},
			wantErr:  true,
			wantMark: map[string]bool{"b": true},
		},
		{
			name: "the page budget ran out",
			next: func(*string) ([]string, *string, error) {
				return []string{"a"}, str("more"), nil
			},
			wantMark: map[string]bool{"b": true},
		},
		{
			name: "walked to the end",
			next: func(token *string) ([]string, *string, error) {
				if token == nil {
					return []string{"a"}, str("page-2"), nil
				}
				return nil, nil, nil
			},
			wantMark: map[string]bool{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := IssueEnricherResult{TruncatedIDs: map[string]string{}}
			items, _, err := walkAccountPages(context.Background(), &result, rows, oneItemPerRow, func(s string) string { return s }, tc.next)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr && !errors.As(err, new(*smithy.GenericAPIError)) {
				t.Errorf("err = %v, want the refused call's error", err)
			}
			if len(items) == 0 || items[0] != "a" {
				t.Errorf("items = %v, want the walked page's item for a kept", items)
			}
			for _, r := range rows {
				_, marked := result.TruncatedIDs[r.ID]
				if marked != tc.wantMark[r.ID] {
					t.Errorf("row %s marked = %v, want %v (marks %v)", r.ID, marked, tc.wantMark[r.ID], result.TruncatedIDs)
				}
				if _, unreached := result.Unreached[r.ID]; unreached != marked {
					t.Errorf("row %s unreached = %v, marked = %v: a marked row is a row nobody answered", r.ID, unreached, marked)
				}
			}
		})
	}
}

// A service that spreads a row's items over the walk answers no row until the
// last page, so a cut walk marks every row, the ones a page named included.
func TestT6Dev_ManyItemsPerRowCutMarksEveryRow(t *testing.T) {
	rows := []resource.Resource{{ID: "a"}, {ID: "b"}}
	more := "more"
	result := IssueEnricherResult{TruncatedIDs: map[string]string{}}
	_, _, _ = walkAccountPages(context.Background(), &result, rows, manyItemsPerRow, func(s string) string { return s }, func(*string) ([]string, *string, error) {
		return []string{"a"}, &more, nil
	})
	if len(result.TruncatedIDs) != len(rows) {
		t.Errorf("marks = %v, want every row", result.TruncatedIDs)
	}
}
