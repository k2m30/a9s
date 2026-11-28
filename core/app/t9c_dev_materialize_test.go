// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// A key the type produces is its column's one source: Wave 2 writes the
// role's last use, so a struct member the list call leaves set or empty is
// never stored under that key, and a path-only column still is.
func TestT9cDev_MaterializeWritesNoProducedKey(t *testing.T) {
	td := resource.FindResourceType("role")
	if td == nil || !td.Produces("last_used") {
		t.Fatal("role does not produce last_used")
	}
	r := resource.Resource{
		ID:        "acme-app",
		Fields:    map[string]string{"last_used": ""},
		RawStruct: iamtypes.Role{RoleName: aws.String("acme-app"), RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: aws.Time(time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC))}, Path: aws.String("/svc/")},
	}
	cols := []ColumnDef{
		{Key: "last_used", Title: "Last Used", Path: "RoleLastUsed.LastUsedDate"},
		{Title: "Role Path", Path: "Path"},
	}
	got := materializeListFields(r, cols, td)
	if v := got.Fields["last_used"]; v != "" {
		t.Errorf("Fields[last_used] = %q, want the producer's empty value", v)
	}
	if v := got.Fields["role_path"]; v != "/svc/" {
		t.Errorf("Fields[role_path] = %q, want the path-only column's struct value", v)
	}
}
