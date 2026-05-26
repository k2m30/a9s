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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchIAMRolesPage(ctx, c.IAM, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichIAMRoleLastUsed, Priority: 100},
		FieldKeys: []string{
			"role_name", "role_id", "path", "create_date", "description",
			"assume_role_policy_document", "trust_wildcard", "trust_summary",
			"policy_resources",
		},
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkRoleLambda, NeedsTargetCache: true},
			{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkRoleGlue, NeedsTargetCache: true},
			{TargetType: "ng", DisplayName: "Node Groups", Checker: checkRoleNG, NeedsTargetCache: true},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkRolePolicy, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkRoleEC2, NeedsTargetCache: true},
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkRoleEKS, NeedsTargetCache: true},
			{TargetType: "iam-group", DisplayName: "IAM Groups (trust)", Checker: checkRoleIamGroup, NeedsTargetCache: false},
			{TargetType: "iam-user", DisplayName: "IAM Users (trust)", Checker: checkRoleIamUser, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("role"), NeedsTargetCache: false},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			result, err := FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
			if err != nil {
				return result, err
			}
			// Inline group policies are not paginated by AWS — fetch once on the
			// first page only. Appending on every continuation token would
			// duplicate the same inline rows across pages (CodeRabbit finding
			// on PR #397). The original init() in iam_policies.go had the same
			// bug; the catalog migration site is the natural place to land the fix.
			if continuationToken != "" {
				return result, nil
			}
			inlines, inlineErr := fetchInlineGroupPolicies(ctx, c.IAM)
			// Partial failure: inline group policy enumeration failed for some
			// groups. Preserve the inline results we did get, then propagate the
			// composite error so app.go's ResourcesLoadedMsg handler surfaces it
			// via FlashMsg → `!` log (per E1–E6). Managed policies above are
			// still returned in result.Resources regardless.
			result.Resources = append(result.Resources, inlines...)
			if result.Pagination != nil {
				result.Pagination.PageSize = len(result.Resources)
			}
			return result, inlineErr
		},
		Wave2: IssueEnricher{Fn: EnrichIAMPolicy, Priority: 100},
		FieldKeys: []string{
			"policy_name", "policy_type", "attachment_count", "is_attachable",
			"path", "create_date",
		},
		IssueEnricherFieldKeys: []string{"risk"},
		FetchByIDs: func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return nil, fmt.Errorf("AWS clients not initialized")
			}
			if c.IAMPolicies() == nil {
				return nil, fmt.Errorf("IAMPolicies store not initialized on ServiceClients")
			}
			return FetchIAMPoliciesByIDsFull(ctx, c.IAM, ids, c.IAMPolicies())
		},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkPolicyRole, NeedsTargetCache: false},
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkPolicyUser, NeedsTargetCache: false},
			{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkPolicyGroup, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("policy"), NeedsTargetCache: false},
		},
		DetailEnrich: enrichPolicy,
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchIAMUsersPage(ctx, c.IAM, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichIAMUserMFA, Priority: 100},
		FieldKeys: []string{
			"user_name", "user_id", "path", "create_date", "password_last_used",
			"has_console_password",
		},
		IssueEnricherFieldKeys: []string{"mfa", "risk"},
		Related: []domain.RelatedDef{
			{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkUserGroup, NeedsTargetCache: false},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkUserPolicy, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkIAMUserCtEvents, NeedsTargetCache: false},
		},
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchIAMGroupsPage(ctx, c.IAM, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichIAMGroup, Priority: 100},
		FieldKeys:              []string{"group_name", "group_id", "path", "create_date", "arn"},
		IssueEnricherFieldKeys: []string{"member_count"},
		Related: []domain.RelatedDef{
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkGroupUser, NeedsTargetCache: false},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkGroupPolicy, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("iam-group"), NeedsTargetCache: false},
		},
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
		Wave2:                  IssueEnricher{Fn: EnrichWAFLogging, Priority: 100},
		FieldKeys:              []string{"name", "id", "description"},
		IssueEnricherFieldKeys: []string{"rules_summary"},
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

// securityChildTypes carries the migrated child-type entries for the security
// category. Replayed onto resource.childTypes + paginatedChildRegistry by
// aws.Install() via the bridge in install.go.
var securityChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Group Members",
		ShortName: "iam_group_members",
		Columns:   resource.IAMGroupMemberColumns(),
		CopyField: "user_name",
		FieldKeys: []string{
			"user_name", "user_id", "arn", "path", "create_date", "password_last_used",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchIAMGroupMembers(ctx, c.IAM, parentCtx, continuationToken)
		},
	},
	{
		Name:      "Role Policies",
		ShortName: "role_policies",
		Columns:   resource.RolePolicyColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{"policy_name", "policy_arn", "policy_type"},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchRolePolicies(ctx, c.IAM, c.IAM, parentCtx, continuationToken)
		},
		DetailEnrich: enrichRolePolicy,
	},
}
