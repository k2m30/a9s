package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// The subset of session.PolicyStore consumed by this file is declared as
// IAMPolicyAccess in scope.go — session.PolicyStore satisfies it via Go's
// structural typing.

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
	})

	resource.RegisterFetchByIDs("policy", func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
		s, ok := clients.(*Scope)
		if !ok || s == nil || s.Clients == nil {
			return nil, fmt.Errorf("AWS clients not initialized: policy FetchByIDs requires *aws.Scope")
		}
		if s.IAMPolicies == nil {
			return nil, fmt.Errorf("IAMPolicies store not initialized on aws.Scope")
		}
		return FetchIAMPoliciesByIDsFull(ctx, s.Clients.IAM, ids, s.IAMPolicies)
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
			ID:   policyName,
			Name: policyName,
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

// FetchIAMPoliciesByIDsFull is the production entry point called by the
// related-panel lazy-add path. It resolves policy PolicyNames across BOTH
// managed (customer + AWS) and inline group policies, so a checker that
// emits an attached AWS-managed policy name (AdministratorAccess, …) OR an
// inline group policy name (group/policy pair surfaced by
// ListGroupPolicies) drills into a real entry.
//
// Managed resolution: ListPolicies(Scope=All) paginated on first call,
// memoized via store.MarkManagedBuilt(). Inline resolution: ListGroups +
// ListGroupPolicies, memoized via store.MarkInlineBuilt().
//
// Invariant: the returned Resource shape matches FetchIAMPoliciesPage and
// fetchInlineGroupPolicies (same Fields keys) so reverse-scan checkers
// reading Fields on a lazily-added policy observe the same fields as on a
// paginated-fetched policy.
//
// Concurrency trade-off (acknowledged): no top-level lock is held across the
// check-build-mark sequence. Two concurrent lazy-add calls can both observe
// `store.ManagedBuilt() == false` and both invoke buildAllManagedPolicies.
// The store itself remains correct (writes are mutex-guarded inside the
// PolicyStore impl), so duplicate Set calls are idempotent — but two AWS
// ListPolicies pagination walks may run in parallel before one wins the
// MarkManagedBuilt race. The previous package-global `allPoliciesMu`
// serialized this. Acceptable here because: (a) lazy-add is the
// related-panel drill-in path, not high-volume; (b) duplicate Sets are
// last-write-wins on identical data, so the final cache state is consistent;
// (c) introducing a sync.Once or
// build-in-progress flag would re-couple the transport layer to a session
// concern that PR-02b explicitly removed. Same applies to InlineBuilt.
func FetchIAMPoliciesByIDsFull(ctx context.Context, api IAMAPI, ids []string, store IAMPolicyAccess) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	if api == nil {
		// Defensive: no IAM API surface (test / pre-init). Skip both builds and
		// fall through to a store-only lookup so callers with a pre-populated
		// cache still resolve, and callers without one observe failures via
		// AggregateFailures rather than a nil deref.
		var failures []string
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
			if r, hit := store.Lookup(id); hit {
				resources = append(resources, r)
			} else {
				failures = append(failures, fmt.Sprintf("%s: not found", id))
			}
		}
		return resources, AggregateFailures("policy FetchByIDs", failures, len(ids))
	}

	if !store.ManagedBuilt() {
		if err := buildAllManagedPolicies(ctx, api, store); err != nil {
			// Managed is the trunk — without it we can't resolve any policy.
			return nil, err
		}
		store.MarkManagedBuilt()
	}
	if !store.InlineBuilt() {
		// Include inline group policies so checkGroupPolicy (which emits
		// both attached and inline names) finds the inline entries in
		// cache too. Per-group failures are surfaced via the returned error
		// and propagated into the composite failure list below.
		inlines, inlineErr := fetchInlineGroupPolicies(ctx, api)
		for _, r := range inlines {
			store.Set(r.ID, r)
		}
		if inlineErr != nil {
			// Non-fatal: managed policies are already loaded; partial inline
			// results are incorporated above. Leave InlineBuilt=false
			// so next call retries the inline fetch — it might succeed after a
			// transient throttle. Surface as aggregate failure with the partial
			// results we did recover.
			var failures []string
			failures = append(failures, inlineErr.Error())
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
				if r, hit := store.Lookup(id); hit {
					resources = append(resources, r)
				} else {
					failures = append(failures, fmt.Sprintf("%s: not found", id))
				}
			}
			return resources, AggregateFailures("policy FetchByIDs", failures, len(ids))
		}
		store.MarkInlineBuilt()
	}

	var failures []string
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
		if r, hit := store.Lookup(id); hit {
			resources = append(resources, r)
		} else {
			failures = append(failures, fmt.Sprintf("%s: not found", id))
		}
	}
	return resources, AggregateFailures("policy FetchByIDs", failures, len(ids))
}

// FetchIAMPoliciesByIDs is the narrower test-friendly variant: resolves
// names from ListPolicies(Scope=All) only, no inlines. Useful when the
// caller only has an IAMListPoliciesAPI (unit tests with a minimal mock).
// Production code goes through FetchIAMPoliciesByIDsFull.
//
// Per-ID failures (IDs not present in the all-policies map) are collected into
// a composite error returned alongside the partial success list.
func FetchIAMPoliciesByIDs(ctx context.Context, api IAMListPoliciesAPI, ids []string, store IAMPolicyAccess) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	if !store.ManagedBuilt() {
		if err := buildAllManagedPolicies(ctx, api, store); err != nil {
			return nil, err
		}
		store.MarkManagedBuilt()
	}

	var failures []string
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
		if r, hit := store.Lookup(id); hit {
			resources = append(resources, r)
		} else {
			failures = append(failures, fmt.Sprintf("%s: not found", id))
		}
	}
	return resources, AggregateFailures("policy FetchByIDs", failures, len(ids))
}

// buildAllManagedPolicies paginates ListPolicies(Scope=All) and populates
// the store with every managed policy (customer + AWS). Each paginated
// ListPolicies call is wrapped in RetryOnThrottle so throttling during
// large accounts is handled gracefully.
func buildAllManagedPolicies(ctx context.Context, api IAMListPoliciesAPI, store IAMPolicyAccess) error {
	var marker *string
	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListPoliciesOutput, error) {
			return api.ListPolicies(ctx, &iam.ListPoliciesInput{
				Scope:    iamtypes.PolicyScopeTypeAll,
				MaxItems: aws.Int32(DefaultPageSize),
				Marker:   marker,
			})
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
			r := resource.Resource{
				ID:   policyName,
				Name: policyName,
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
			// Index by PolicyName AND by ARN so callers that emit ARN-based IDs
			// (e.g. checkers that embed policy.Arn from SDK structs) can still
			// resolve the policy via FetchIAMPoliciesByIDsFull.
			store.Set(policyName, r)
			if p.Arn != nil && *p.Arn != policyName {
				store.Set(*p.Arn, r)
			}
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		marker = out.Marker
	}
	return nil
}

// fetchInlineGroupPolicies enumerates all groups via ListGroups and collects
// their inline policy names via ListGroupPolicies. Both API calls are wrapped
// in RetryOnThrottle. Per-group ListGroupPolicies failures are collected and
// returned as a composite error alongside any partial results — callers must
// check both return values.
func fetchInlineGroupPolicies(ctx context.Context, api IAMAPI) ([]resource.Resource, error) {
	groupsOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListGroupsOutput, error) {
		return api.ListGroups(ctx, &iam.ListGroupsInput{})
	})
	if err != nil {
		return nil, fmt.Errorf("listing IAM groups for inline policies: %w", err)
	}

	var resources []resource.Resource
	var groupFailures []string
	for _, group := range groupsOut.Groups {
		if group.GroupName == nil {
			continue
		}
		groupName := *group.GroupName
		out, gpErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListGroupPoliciesOutput, error) {
			return api.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: &groupName})
		})
		if gpErr != nil {
			groupFailures = append(groupFailures, fmt.Sprintf("%s: %v", groupName, gpErr))
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
					"path":             "inline/" + groupName,
					"create_date":      "",
				},
			})
		}
	}

	return resources, AggregateFailures("ListGroupPolicies", groupFailures, len(groupsOut.Groups))
}
