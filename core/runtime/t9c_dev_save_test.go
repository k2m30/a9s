// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The save lane stores what the cell showed: a key the type produces is
// answered by its producer, so the struct member never lands under it.
func TestT9cDev_SaveWritesNoProducedKey(t *testing.T) {
	r := resource.Resource{
		ID:        "acme-app",
		Fields:    map[string]string{"last_used": ""},
		RawStruct: iamtypes.Role{RoleName: aws.String("acme-app"), RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: aws.Time(time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC))}, Path: aws.String("/svc/")},
	}
	cols := []config.ListColumn{
		{Key: "last_used", Title: "Last Used", Path: "RoleLastUsed.LastUsedDate"},
		{Title: "Role Path", Path: "Path"},
	}
	got := materializeResourceFields(r, cols, resource.FindResourceType("role"))
	if v := got.Fields["last_used"]; v != "" {
		t.Errorf("Fields[last_used] = %q, want the producer's empty value", v)
	}
	if v := got.Fields["role_path"]; v != "/svc/" {
		t.Errorf("Fields[role_path] = %q, want the path-only column's struct value", v)
	}
}
