package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestT11aDev_ChecklistS3_JudgesEveryFindingFromTheRecordedAnswers(t *testing.T) {
	cases := []struct {
		name          string
		bucket        string
		account       string
		want          []FlaggedRow
		issues        int
		issuesAtLeast bool
	}{
		{name: "healthy", bucket: `{"name":"acme-healthy",
			"pab":{"outcome":"configured","block_public_acls":true,"ignore_public_acls":true,"block_public_policy":true,"restrict_public_buckets":true},
			"policy_status":{"outcome":"none"},"acl":{"outcome":"ok"},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Enabled"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":1},
			"object_lock":{"outcome":"ok","enabled":true}}`},
		{name: "a full block restricts a public policy", bucket: `{"name":"acme-restricted",
			"pab":{"outcome":"configured","block_public_acls":true,"ignore_public_acls":true,"block_public_policy":true,"restrict_public_buckets":true},
			"policy_status":{"outcome":"ok","is_public":true},"acl":{"outcome":"ok"},
			"versioning":{"outcome":"ok","status":"Suspended"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":1},
			"object_lock":{"outcome":"ok","enabled":true}}`,
			want: []FlaggedRow{{Name: "acme-restricted", Decorator: "~", Status: "versioning off"}}},
		{name: "no block and a public grant", bucket: `{"name":"acme-public",
			"pab":{"outcome":"none"},
			"policy_status":{"outcome":"none"},"acl":{"outcome":"ok","group_grants":[{"uri":"http://acs.amazonaws.com/groups/global/AllUsers","permission":"READ"}]},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Enabled"},
			"logging":{"outcome":"ok","enabled":false},"lifecycle":{"outcome":"none"},
			"object_lock":{"outcome":"ok","enabled":true}}`,
			want:   []FlaggedRow{{Name: "acme-public", Decorator: "!", Status: "publicly accessible (+3)"}},
			issues: 1},
		{name: "a bucket in another region is not inspected", bucket: `{"name":"acme-elsewhere",
			"pab":{"outcome":"error","error_code":"PermanentRedirect"},
			"policy_status":{"outcome":"error","error_code":"PermanentRedirect"},"acl":{"outcome":"error","error_code":"PermanentRedirect"},
			"versioning":{"outcome":"error","error_code":"PermanentRedirect"},"logging":{"outcome":"error","error_code":"PermanentRedirect"},
			"lifecycle":{"outcome":"error","error_code":"PermanentRedirect"},"object_lock":{"outcome":"error","error_code":"PermanentRedirect"}}`,
			issuesAtLeast: true},
		{name: "an unread block leaves the public verdict unjudged", bucket: `{"name":"acme-denied",
			"pab":{"outcome":"error","error_code":"AccessDenied"},
			"policy_status":{"outcome":"ok","is_public":true},"acl":{"outcome":"ok"},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Disabled"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":2},
			"object_lock":{"outcome":"none"}}`,
			want:          []FlaggedRow{{Name: "acme-denied", Decorator: "~", Status: "MFA delete off (+1)"}},
			issuesAtLeast: true},
		{name: "a section without the public reads leaves the badge a lower bound", bucket: `{"name":"acme-pab-only",
			"pab":{"outcome":"configured","block_public_acls":true,"block_public_policy":true,"restrict_public_buckets":true}}`,
			want:          []FlaggedRow{{Name: "acme-pab-only", Decorator: "~", Status: "public access block incomplete"}},
			issuesAtLeast: true},
		{name: "the account's IgnorePublicAcls closes the grant", bucket: `{"name":"acme-ignored",
			"pab":{"outcome":"configured","block_public_acls":true,"block_public_policy":true,"restrict_public_buckets":true},
			"policy_status":{"outcome":"none"},"acl":{"outcome":"ok","group_grants":[{"uri":"http://acs.amazonaws.com/groups/global/AllUsers","permission":"READ"}]},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Enabled"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":1},
			"object_lock":{"outcome":"ok","enabled":true}}`,
			account: `{"outcome":"configured","ignore_public_acls":true}`},
		{name: "an unread account block leaves a bucket it decides unjudged", bucket: `{"name":"acme-open",
			"pab":{"outcome":"none"},
			"policy_status":{"outcome":"ok","is_public":true},"acl":{"outcome":"ok"},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Enabled"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":1},
			"object_lock":{"outcome":"ok","enabled":true}}`,
			account:       `{"outcome":"error","error_code":"AccessDenied"}`,
			issuesAtLeast: true},
		{name: "a full account block closes a bucket whose own block was not read", bucket: `{"name":"acme-closed",
			"pab":{"outcome":"error","error_code":"AccessDenied"},
			"policy_status":{"outcome":"ok","is_public":true},"acl":{"outcome":"ok"},
			"versioning":{"outcome":"ok","status":"Enabled","mfa_delete":"Enabled"},
			"logging":{"outcome":"ok","enabled":true},"lifecycle":{"outcome":"ok","enabled_rules":1},
			"object_lock":{"outcome":"ok","enabled":true}}`,
			account:       `{"outcome":"configured","block_public_acls":true,"ignore_public_acls":true,"block_public_policy":true,"restrict_public_buckets":true}`,
			issuesAtLeast: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			account := c.account
			if account == "" {
				account = `{"outcome":"none"}`
			}
			got, err := checklistS3(json.RawMessage(`{"account_pab":` + account + `,"buckets":[` + c.bucket + `]}`))
			if err != nil {
				t.Fatal(err)
			}
			if len(got.List.Flagged) != len(c.want) || len(c.want) > 0 && !reflect.DeepEqual(got.List.Flagged, c.want) {
				t.Errorf("flagged = %+v, want %+v", got.List.Flagged, c.want)
			}
			if got.Menu.Issues != c.issues || got.Menu.IssuesTruncated != c.issuesAtLeast {
				t.Errorf("issues = %d (lower bound %v), want %d (lower bound %v)", got.Menu.Issues, got.Menu.IssuesTruncated, c.issues, c.issuesAtLeast)
			}
			if got.List.Colors["healthy"] != 1 {
				t.Errorf("colors = %v, want one healthy row", got.List.Colors)
			}
		})
	}
}
