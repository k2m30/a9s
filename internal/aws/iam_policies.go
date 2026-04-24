package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("policy", []string{"policy_name", "policy_type", "attachment_count", "is_attachable", "path", "create_date"})

	resource.RegisterPaginated("policy", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		result, err := FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
		if err != nil {
			return result, err
		}
		inlines := fetchInlineGroupPolicies(ctx, c.IAM)
		result.Resources = append(result.Resources, inlines...)
		if result.Pagination != nil {
			result.Pagination.PageSize = len(result.Resources)
		}
		return result, nil
	})

	resource.RegisterFetchByIDs("policy", func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMPoliciesByIDsFull(ctx, c.IAM, ids)
	})

	resource.RegisterRelated("policy", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkPolicyRole, NeedsTargetCache: false},
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkPolicyUser, NeedsTargetCache: false},
		{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkPolicyGroup, NeedsTargetCache: false},
	})
}

// FetchIAMPolicies calls the IAM ListPolicies API and returns all pages of
// customer-managed policies. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchIAMPolicies(ctx context.Context, api IAMListPoliciesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchIAMPoliciesPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchIAMPoliciesPage calls the IAM ListPolicies API with Scope=Local
// and returns a single page of customer-managed policies.
// Pass an empty continuationToken for the first page.
func FetchIAMPoliciesPage(ctx context.Context, api IAMListPoliciesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &iam.ListPoliciesInput{
		Scope:    iamtypes.PolicyScopeTypeLocal,
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListPolicies(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching IAM policies: %w", err)
	}

	var resources []resource.Resource
	for _, policy := range output.Policies {
		policyName := ""
		if policy.PolicyName != nil {
			policyName = *policy.PolicyName
		}

		attachmentCount := "0"
		if policy.AttachmentCount != nil {
			attachmentCount = fmt.Sprintf("%d", *policy.AttachmentCount)
		}

		path := ""
		if policy.Path != nil {
			path = *policy.Path
		}

		createDate := ""
		if policy.CreateDate != nil {
			createDate = policy.CreateDate.Format("2006-01-02 15:04")
		}

		isAttachable := "false"
		if policy.IsAttachable {
			isAttachable = "true"
		}

		r := resource.Resource{
			ID:     policyName,
			Name:   policyName,
			Status: "",
			Fields: map[string]string{
				"policy_name":      policyName,
				"policy_type":      "managed",
				"attachment_count": attachmentCount,
				"is_attachable":    isAttachable,
				"path":             path,
				"create_date":      createDate,
			},
			RawStruct: policy,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata — IAM uses IsTruncated bool + Marker *string
	nextToken := ""
	isTruncated := output.IsTruncated
	if isTruncated && output.Marker != nil {
		nextToken = *output.Marker
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// allPoliciesCache memoizes the name→Resource map built from
// ListPolicies(Scope=All). AWS-managed policy names are stable globally;
// customer-managed names are stable within an account. The cache survives for
// the lifetime of the process, which matches the `kms` cache semantics (key
// IDs memoized across drills). The mutex guards concurrent rebuilds from
// parallel related-checks targeting "policy".
//
// Known limitation: long-running sessions that create or rename a policy
// mid-session won't see the new name via this lazy-add path until the
// process restarts. Policies are typically created via IaC at deploy time,
// so a cache flush within a single session is rarely needed. If that
// assumption breaks, add a TTL or plumb a Ctrl+R invalidation to reset
// allPoliciesBuilt. The primary top-level policy list refreshes via the
// paginated fetcher on demand and is unaffected.
var (
	allPoliciesMu    sync.Mutex
	allPoliciesBuilt bool
	allPoliciesByID  map[string]resource.Resource
)

// FetchIAMPoliciesByIDsFull is the production entry point called by the
// related-panel lazy-add path. It resolves policy PolicyNames across BOTH
// managed (customer + AWS) and inline group policies, so a checker that
// emits an attached AWS-managed policy name (AdministratorAccess, …) OR an
// inline group policy name (group/policy pair surfaced by
// ListGroupPolicies) drills into a real entry.
//
// Managed resolution: ListPolicies(Scope=All) paginated on first call,
// memoized in allPoliciesByID. Inline resolution: ListGroups +
// ListGroupPolicies, memoized alongside.
//
// Invariant: the returned Resource shape matches FetchIAMPoliciesPage and
// fetchInlineGroupPolicies (same Fields keys) so reverse-scan checkers
// reading Fields on a lazily-added policy observe the same fields as on a
// paginated-fetched policy.
func FetchIAMPoliciesByIDsFull(ctx context.Context, api IAMAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	allPoliciesMu.Lock()
	defer allPoliciesMu.Unlock()

	if !allPoliciesBuilt {
		allPoliciesByID = make(map[string]resource.Resource)
		if err := buildAllManagedPolicies(ctx, api); err != nil {
			return nil, err
		}
		// Include inline group policies so checkGroupPolicy (which emits
		// both attached and inline names) finds the inline entries in
		// cache too. fetchInlineGroupPolicies swallows errors per-group;
		// partial results are preserved.
		for _, r := range fetchInlineGroupPolicies(ctx, api) {
			allPoliciesByID[r.ID] = r
		}
		allPoliciesBuilt = true
	}

	resources := make([]resource.Resource, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if r, hit := allPoliciesByID[id]; hit {
			resources = append(resources, r)
		}
	}
	return resources, nil
}

// FetchIAMPoliciesByIDs is the narrower test-friendly variant: resolves
// names from ListPolicies(Scope=All) only, no inlines. Useful when the
// caller only has an IAMListPoliciesAPI (unit tests with a minimal mock).
// Production code goes through FetchIAMPoliciesByIDsFull.
func FetchIAMPoliciesByIDs(ctx context.Context, api IAMListPoliciesAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	allPoliciesMu.Lock()
	defer allPoliciesMu.Unlock()

	if !allPoliciesBuilt {
		allPoliciesByID = make(map[string]resource.Resource)
		if err := buildAllManagedPolicies(ctx, api); err != nil {
			return nil, err
		}
		allPoliciesBuilt = true
	}

	resources := make([]resource.Resource, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if r, hit := allPoliciesByID[id]; hit {
			resources = append(resources, r)
		}
	}
	return resources, nil
}

// buildAllManagedPolicies paginates ListPolicies(Scope=All) and populates
// allPoliciesByID with every managed policy (customer + AWS). Caller MUST
// hold allPoliciesMu.
func buildAllManagedPolicies(ctx context.Context, api IAMListPoliciesAPI) error {
	var marker *string
	for {
		out, err := api.ListPolicies(ctx, &iam.ListPoliciesInput{
			Scope:    iamtypes.PolicyScopeTypeAll,
			MaxItems: aws.Int32(DefaultPageSize),
			Marker:   marker,
		})
		if err != nil {
			return fmt.Errorf("listing all IAM policies for lazy-add: %w", err)
		}
		for _, p := range out.Policies {
			policyName := ""
			if p.PolicyName != nil {
				policyName = *p.PolicyName
			}
			if policyName == "" {
				continue
			}
			attachmentCount := "0"
			if p.AttachmentCount != nil {
				attachmentCount = fmt.Sprintf("%d", *p.AttachmentCount)
			}
			path := ""
			if p.Path != nil {
				path = *p.Path
			}
			createDate := ""
			if p.CreateDate != nil {
				createDate = p.CreateDate.Format("2006-01-02 15:04")
			}
			isAttachable := "false"
			if p.IsAttachable {
				isAttachable = "true"
			}
			policyType := "managed"
			if p.Arn != nil && !IsCustomerManagedIAMPolicyARN(*p.Arn) {
				policyType = "aws-managed"
			}
			allPoliciesByID[policyName] = resource.Resource{
				ID:     policyName,
				Name:   policyName,
				Status: "",
				Fields: map[string]string{
					"policy_name":      policyName,
					"policy_type":      policyType,
					"attachment_count": attachmentCount,
					"is_attachable":    isAttachable,
					"path":             path,
					"create_date":      createDate,
				},
				RawStruct: p,
			}
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		marker = out.Marker
	}
	return nil
}

func fetchInlineGroupPolicies(ctx context.Context, api IAMAPI) []resource.Resource {
	var resources []resource.Resource
	groupsOut, err := api.ListGroups(ctx, &iam.ListGroupsInput{})
	if err != nil {
		return nil
	}
	for _, group := range groupsOut.Groups {
		if group.GroupName == nil {
			continue
		}
		out, err := api.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: group.GroupName})
		if err != nil {
			continue
		}
		for _, name := range out.PolicyNames {
			resources = append(resources, resource.Resource{
				ID:   name,
				Name: name,
				Fields: map[string]string{
					"policy_name":      name,
					"policy_type":      "inline",
					"attachment_count": "",
					"path":             "inline/" + *group.GroupName,
					"create_date":      "",
				},
			})
		}
	}
	return resources
}
