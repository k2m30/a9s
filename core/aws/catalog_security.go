// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func colorRole(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorPolicy(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	if r.Fields["attachment_count"] == "0" && r.Fields["is_attachable"] == "true" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorIAMUser(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
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

func colorIAMGroup(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorWAF(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

var securityTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "IAM Roles",
		ShortName:     "role",
		Aliases:       []string{"role", "roles", "iam-roles", "iam_roles"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "_localfield.role_name:Fields.role_name",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "iam/home#/roles/details/"+url.PathEscape(r.ID))
		},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchIAMRolesPage(ctx, c.IAM, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichIAMRoleLastUsed, Priority: 100},
		FieldKeys: []string{
			"role_name", "role_id", "path", "create_date", "description",
			"assume_role_policy_document", "trust_wildcard", "trust_summary",
			"policy_resources",
		},
		FetchByIDs: fetchByIDsWithClients(func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
			getRoleAPI, ok := c.IAM.(IAMGetRoleAPI)
			if !ok || getRoleAPI == nil {
				return nil, fmt.Errorf("IAM client does not support GetRole")
			}
			return FetchRolesByIDs(ctx, getRoleAPI, ids)
		}),
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkRoleLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkRoleGlue, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ng", DisplayName: "Node Groups", Checker: checkRoleNG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkRolePolicy, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkRoleEC2, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkRoleEKS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "iam-group", DisplayName: "IAM Groups (trust)", Checker: checkRoleIamGroup, NeedsTargetCache: false},
			{TargetType: "iam-user", DisplayName: "IAM Users (trust)", Checker: checkRoleIamUser, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("role"), NeedsTargetCache: false},
		},
		Findings: []catalog.FindingDef{
			{Code: roleCodeWildcardTrust, Phrase: "anyone can assume this role", Severity: domain.SevBroken, Source: "wave1"},
			{Code: roleCodeConfusedDeputy, Phrase: "service can assume without source scoping", Severity: domain.SevWarn, Source: "wave1"},
			{Code: roleCodeInlinePrivEsc, Phrase: "inline policy allows privilege escalation: <combo>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: iamRoleCodeDormant, Phrase: "dormant role (>90d)", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamRoleCodeAdminAttached, Phrase: "has AdministratorAccess", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "IAM Policies",
		ShortName:     "policy",
		Aliases:       []string{"policy", "policies", "iam-policies", "iam_policies"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Global(region, "iam/home#/policies/details/"+url.QueryEscape(arn))
		},
		Columns: []domain.Column{
			{Key: "policy_name", Title: "Policy Name", Width: 36, Sortable: true},
			{Key: "policy_type", Title: "Type", Width: 10, Sortable: true},
			{Key: "risk", Title: "Status", Width: 14, Sortable: true},
			{Key: "attachment_count", Title: "Attached", Width: 10, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorPolicy,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			result, err := FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
			if err != nil {
				return result, err
			}
			// Inline group policies are not paginated by AWS — fetch once on the
			// first page only. Appending on every continuation token would
			// duplicate the same inline rows across pages. fetchInlineGroupPolicies
			// itself fans the per-group ListGroupPolicies sweep out with bounded
			// concurrency (core/aws/iam_policies.go) so this stays well inside
			// any real list-open caller's deadline. The availability/count probe
			// never reaches this sweep at all — see AvailabilityFetcher below,
			// registered specifically to keep the probe on the cheap managed-only
			// path (core/runtime/probes.go's ProbeResourceAvailability).
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
		}),
		// AvailabilityFetcher is the cheap, managed-only probe path
		// (core/runtime/probes.go's ProbeResourceAvailability, via
		// resource.GetAvailabilityFetcher): managed policies alone are
		// sufficient for an availability/count signal, so this never reaches
		// fetchInlineGroupPolicies's per-group IAM sweep — the live symptom
		// this registration exists to prevent ("availability policy:
		// ListGroupPolicies failed for N of M IDs" on a wide-group account).
		// IsTruncated is forced true regardless of what ListPolicies itself
		// reports: this probe deliberately never checks group-inline
		// policies, so it can never confirm a true zero/exact total — an
		// inline-only account (zero managed policies, many inline ones on
		// groups) would otherwise report a confirmed-empty "0" instead of
		// the honest lower-bound "N+", making it look unnavigable.
		// LowerBoundOnly marks this pairing (IsTruncated=true, no cursor) as
		// deliberate: without it, resource.sanitizeFetchResult cannot tell this
		// apart from a fetcher that hit a local cap and forgot to wire a
		// cursor, and would downgrade it back to a confirmed "0", the exact
		// bug this registration exists to prevent.
		AvailabilityFetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			result, err := FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
			if result.Pagination == nil {
				result.Pagination = &resource.PaginationMeta{}
			}
			result.Pagination.IsTruncated = true
			result.Pagination.LowerBoundOnly = true
			return result, err
		}),
		Wave2: IssueEnricher{Fn: EnrichIAMPolicy, Priority: 100},
		FieldKeys: []string{
			"policy_name", "policy_type", "attachment_count", "is_attachable",
			"path", "create_date", "arn",
		},
		IssueEnricherFieldKeys: []string{"risk"},
		FetchByIDs: fetchByIDsWithClients(func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error) {
			if c.IAMPolicies() == nil {
				return nil, fmt.Errorf("IAMPolicies store not initialized on ServiceClients")
			}
			return FetchIAMPoliciesByIDsFull(ctx, c.IAM, ids, c.IAMPolicies())
		}),
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkPolicyRole, NeedsTargetCache: false},
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkPolicyUser, NeedsTargetCache: false},
			{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkPolicyGroup, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("policy"), NeedsTargetCache: false},
		},
		DetailEnrich: enrichPolicy,
		Findings: []catalog.FindingDef{
			{Code: iamPolicyCodeOrphanUnattached, Phrase: "unattached, no roles/users/groups use it", Severity: domain.SevWarn, Source: "wave1"},
			{Code: iamPolicyCodeAdminStar, Phrase: "admin star (allows * on *)", Severity: domain.SevBroken, Source: "wave2"},
			{Code: iamPolicyCodePrivEsc, Phrase: "allows privilege escalation: <combo>", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
	{
		Name:          "IAM Users",
		ShortName:     "iam-user",
		Aliases:       []string{"iam-user", "iam-users", "users", "iam_users"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "Username:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "iam/home#/users/details/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "user_name", Title: "User Name", Width: 32, Sortable: true},
			{Key: "mfa", Title: "MFA", Width: 5, Sortable: true},
			{Key: "risk", Title: "Status", Width: 14, Sortable: true},
			{Key: "user_id", Title: "User ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
		},
		Color: colorIAMUser,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchIAMUsersPage(ctx, c.IAM, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichIAMUserMFA, Priority: 100},
		FieldKeys: []string{
			"user_name", "user_id", "path", "create_date", "password_last_used",
			"has_console_password",
		},
		IssueEnricherFieldKeys: []string{"mfa", "risk", "has_console_password"},
		Related: []domain.RelatedDef{
			{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkUserGroup, NeedsTargetCache: false},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkUserPolicy, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkIAMUserCtEvents, NeedsTargetCache: false},
		},
		Findings: []catalog.FindingDef{
			{Code: iamUserCodeNoMFA, Phrase: "console user without MFA", Severity: domain.SevBroken, Source: "wave2"},
			{Code: iamUserCodeOldKey, Phrase: "key <keyID> >90d (rotation)", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamUserCodeAdminAttached, Phrase: "has AdministratorAccess", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamUserCodeConsoleNeverUsed, Phrase: "console password never used", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamUserCodeKeyUnused, Phrase: "access key unused for <N> days", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamUserCodeTwoActiveKeys, Phrase: "two active access keys", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "IAM Groups",
		ShortName:     "iam-group",
		Aliases:       []string{"iam-group", "iam-groups", "groups", "iam_groups"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "iam/home#/groups/details/"+url.PathEscape(r.ID))
		},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchIAMGroupsPage(ctx, c.IAM, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichIAMGroup, Priority: 100},
		FieldKeys:              []string{"group_name", "group_id", "path", "create_date", "arn"},
		IssueEnricherFieldKeys: []string{"member_count"},
		Related: []domain.RelatedDef{
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkGroupUser, NeedsTargetCache: false},
			{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkGroupPolicy, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("iam-group"), NeedsTargetCache: false},
		},
		Findings: []catalog.FindingDef{
			{Code: iamGroupCodeOrphanOrNoop, Phrase: "group has no members (orphan)", Severity: domain.SevWarn, Source: "wave2"},
			{Code: iamGroupCodeAdminAttached, Phrase: "has AdministratorAccess", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "WAF Web ACLs",
		ShortName:     "waf",
		Aliases:       []string{"waf", "webacl", "web-acl"},
		Category:      "SECURITY & IAM",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			scope := r.Fields["scope"]
			if r.Name == "" || scope == "" {
				return ""
			}
			tail := "wafv2-pro/protections/" + url.PathEscape(r.Name) + "/" + r.ID + "?panel=protectionPackHome"
			if scope == "CLOUDFRONT" {
				return consolelink.Regional("us-east-1", tail+"&region=us-east-1&scope=global")
			}
			return consolelink.Regional(region, tail+"&region="+region+"&scope=regional")
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "id", Title: "ID", Width: 38, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorWAF,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchWAFWebACLsPageWithCloudFront(ctx, c.WAFv2, c.WAFv2CloudFront, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichWAFLogging, Priority: 100},
		FieldKeys:              []string{"name", "id", "description", "scope"},
		IssueEnricherFieldKeys: []string{"rules_summary"},
		Related: []domain.RelatedDef{
			{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkWAFELB, NeedsTargetCache: false},
			{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkWAFAPIGW, NeedsTargetCache: false},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkWAFCF, NeedsTargetCache: false},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkWAFAlarm, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkWAFLogs},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("waf")},
		},
		// wafv2types.WebACLSummary: no cross-ref fields — Name, Id, ARN, Description, LockToken only.
		// Associations (ELB/APIGW/CF) are resolved via checkWAF* related checkers at runtime.
		Findings: []catalog.FindingDef{
			{Code: wafCodeNoLogging, Phrase: "no logging configuration", Severity: domain.SevWarn, Source: "wave2"},
			{Code: wafCodeNoRules, Phrase: "web ACL has no rules", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
}

// securityChildTypes carries the migrated child-type entries for the security
// category. Replayed onto resource.childTypes + paginatedChildRegistry by
// aws.Install() via the bridge in install.go.
var securityChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Group Members",
		ShortName: "iam_group_members",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			user := r.Fields["user_name"]
			if user == "" {
				return ""
			}
			return consolelink.Global(region, "iam/home#/users/details/"+url.PathEscape(user))
		},
		Columns:   resource.IAMGroupMemberColumns(),
		CopyField: "user_name",
		FieldKeys: []string{
			"user_name", "user_id", "arn", "path", "create_date", "password_last_used",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchIAMGroupMembers(ctx, c.IAM, parentCtx, continuationToken)
		}),
	},
	{
		Name:      "Role Policies",
		ShortName: "role_policies",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			if arn := r.Fields["policy_arn"]; arn != "" {
				return consolelink.Global(region, "iam/home#/policies/details/"+url.QueryEscape(arn))
			}
			role := r.Fields["role_name"]
			if role == "" {
				return ""
			}
			return consolelink.Global(region, "iam/home#/roles/details/"+url.PathEscape(role))
		},
		Columns:   resource.RolePolicyColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{"policy_name", "policy_arn", "policy_type"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchRolePolicies(ctx, c.IAM, c.IAM, parentCtx, continuationToken)
		}),
		DetailEnrich: enrichRolePolicy,
		Findings: []catalog.FindingDef{
			{Code: CodeRolePolicyOverPrivileged, Phrase: "over-privileged", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRolePolicyInline, Phrase: "inline", Severity: domain.SevDim, Source: "wave1"},
		},
	},
}
