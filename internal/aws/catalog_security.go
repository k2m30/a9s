package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorRole(r domain.Resource) domain.Color {
	doc := r.Fields["assume_role_policy_document"]
	if doc != "" &&
		(strings.Contains(doc, `"Principal":"*"`) || strings.Contains(doc, `"Principal": "*"`)) {
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func colorPolicy(r domain.Resource) domain.Color {
	if r.Fields["attachment_count"] == "0" && r.Fields["is_attachable"] == "true" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorIAMUser(r domain.Resource) domain.Color {
	if r.Fields["has_console_password"] != "true" {
		return domain.ColorHealthy
	}
	plu := r.Fields["password_last_used"]
	t, err := time.Parse("2006-01-02 15:04", plu)
	if err != nil {
		return domain.ColorHealthy
	}
	if time.Since(t) > 90*24*time.Hour {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorIAMGroup(_ domain.Resource) domain.Color { return domain.ColorHealthy }
func colorWAF(_ domain.Resource) domain.Color      { return domain.ColorHealthy }

var securityTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "IAM Roles",
		ShortName:     "role",
		Aliases:       []string{"role", "roles", "iam-roles", "iam_roles"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "Username:Name",
		Columns: []domain.Column{
			{Key: "role_name", Title: "Role Name", Width: 36, Sortable: true},
			{Key: "role_id", Title: "Role ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "role_policies",
			Key:            "enter",
			ContextKeys:    map[string]string{"role_name": "ID"},
			DisplayNameKey: "role_name",
		}},
		Color: colorRole,
	},
	{
		Name:          "IAM Policies",
		ShortName:     "policy",
		Aliases:       []string{"policy", "policies", "iam-policies", "iam_policies"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "policy_name", Title: "Policy Name", Width: 36, Sortable: true},
			{Key: "policy_type", Title: "Type", Width: 10, Sortable: true},
			{Key: "attachment_count", Title: "Attached", Width: 10, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorPolicy,
	},
	{
		Name:          "IAM Users",
		ShortName:     "iam-user",
		Aliases:       []string{"iam-user", "iam-users", "users", "iam_users"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "Username:ID",
		Columns: []domain.Column{
			{Key: "user_name", Title: "User Name", Width: 32, Sortable: true},
			{Key: "user_id", Title: "User ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
		},
		Color: colorIAMUser,
	},
	{
		Name:          "IAM Groups",
		ShortName:     "iam-group",
		Aliases:       []string{"iam-group", "iam-groups", "groups", "iam_groups"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "group_name", Title: "Group Name", Width: 32, Sortable: true},
			{Key: "group_id", Title: "Group ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "arn", Title: "ARN", Width: 60, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "iam_group_members",
			Key:            "enter",
			ContextKeys:    map[string]string{"group_name": "ID"},
			DisplayNameKey: "group_name",
		}},
		Color: colorIAMGroup,
	},
	{
		Name:          "WAF Web ACLs",
		ShortName:     "waf",
		Aliases:       []string{"waf", "webacl", "web-acl"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "id", Title: "ID", Width: 38, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorWAF,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchWAFWebACLsPage(ctx, c.WAFv2, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichWAFLogging, Priority: 100},
		FieldKeys: []string{"name", "id", "description"},
		Related: []domain.RelatedDef{
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkWAFELB, NeedsTargetCache: false},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkWAFAPIGW, NeedsTargetCache: false},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkWAFCF, NeedsTargetCache: false},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkWAFAlarm},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkWAFLogs},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("waf")},
		},
		// wafv2types.WebACLSummary: no cross-ref fields — Name, Id, ARN, Description, LockToken only.
		// Associations (ELB/APIGW/CF) are resolved via checkWAF* related checkers at runtime.
	},
}
