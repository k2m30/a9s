package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamPolicyCodeOrphanUnattached is the canonical FindingCode for a
// customer-managed policy that is attachable but currently attached to
// nothing (AttachmentCount == 0) — dead permission surface worth pruning.
const iamPolicyCodeOrphanUnattached domain.FindingCode = "iam-policy.orphan-unattached"

// orphanUnattachedPolicyFinding returns the wave1 Finding for an attachable
// policy with zero attachments, or nil when the policy doesn't match.
func orphanUnattachedPolicyFinding(attachmentCount string, isAttachable bool) []domain.Finding {
	if attachmentCount == "0" && isAttachable {
		return []domain.Finding{{
			Code: iamPolicyCodeOrphanUnattached, Phrase: "unattached, no roles/users/groups use it",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	}
	return nil
}

// iamPolicyStore is the subset of session.PolicyStore consumed by this file.
// Defined locally to avoid an import cycle (internal/session imports internal/aws).
// session.PolicyStore satisfies this interface via Go's structural typing.
type iamPolicyStore interface {
	Lookup(key string) (resource.Resource, bool)
	Set(key string, r resource.Resource)
	ManagedBuilt() bool
	MarkManagedBuilt()
	InlineBuilt() bool
	MarkInlineBuilt()
	Clear()
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
		resources = append(resources, managedPolicyToResource(policy))
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

// managedPolicyToResource builds the canonical policy Resource (ID == policy
// name, same Fields keys as the paginated fetcher) from an IAM Policy — used by
// both the ListPolicies page fetch and the per-name GetPolicy lazy-add so the
// two paths can never drift in shape.
func managedPolicyToResource(policy iamtypes.Policy) resource.Resource {
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
	return resource.Resource{
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
		Findings:  orphanUnattachedPolicyFinding(attachmentCount, policy.IsAttachable),
		RawStruct: policy,
	}
}

// awsManagedPolicyPathPrefixes are the ARN path prefixes AWS-managed policies
// live under. Most are root ("/"); job-function and service-role policies carry
// a path that the bare policy name does not reveal, so getAWSManagedPolicyByName
// tries each in turn — a handful of GetPolicy calls, still bounded and vastly
// cheaper than listing the whole ~1000+ AWS-managed catalog.
var awsManagedPolicyPathPrefixes = []string{
	"arn:aws:iam::aws:policy/",
	"arn:aws:iam::aws:policy/service-role/",
	"arn:aws:iam::aws:policy/job-function/",
}

// getAWSManagedPolicyByName resolves ONE AWS-managed policy by name via GetPolicy
// on the well-known ARN(s) arn:aws:iam::aws:policy[/<path>]/<name>. This is how
// the lazy-add resolves the handful of AWS-managed policy names a checker emitted,
// WITHOUT listing the whole ~1000+ AWS-managed catalog.
func getAWSManagedPolicyByName(ctx context.Context, api IAMGetPolicyAPI, name string) (resource.Resource, error) {
	var lastErr error
	for _, prefix := range awsManagedPolicyPathPrefixes {
		arn := prefix + name
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetPolicyOutput, error) {
			return api.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: aws.String(arn)})
		})
		if err != nil {
			lastErr = err
			continue
		}
		if out != nil && out.Policy != nil {
			return managedPolicyToResource(*out.Policy), nil
		}
		lastErr = fmt.Errorf("GetPolicy(%s): no policy returned", arn)
	}
	return resource.Resource{}, fmt.Errorf("AWS-managed policy %q not resolvable by name: %w", name, lastErr)
}

// FetchIAMPoliciesByIDsFull is the production entry point called by the
// related-panel lazy-add path. It resolves policy PolicyNames across BOTH
// managed (customer + AWS) and inline group policies, so a checker that
// emits an attached AWS-managed policy name (AdministratorAccess, …) OR an
// inline group policy name (group/policy pair surfaced by
// ListGroupPolicies) drills into a real entry.
//
// Managed resolution: customer-managed via ListPolicies(Scope=Local) once
// (memoized via store.MarkManagedBuilt()); AWS-managed names resolved on demand
// via one GetPolicy per requested name — NEVER ListPolicies(Scope=All), whose
// ~1000+ AWS-managed catalog times out even on empty accounts. Inline
// resolution: ListGroups + ListGroupPolicies, memoized via MarkInlineBuilt().
//
// Invariant: the returned Resource shape matches FetchIAMPoliciesPage and
// fetchInlineGroupPolicies (same Fields keys) so reverse-scan checkers
// reading Fields on a lazily-added policy observe the same fields as on a
// paginated-fetched policy.
//
// Concurrency trade-off (acknowledged): no top-level lock is held across the
// check-build-mark sequence. Two concurrent lazy-add calls can both observe
// `store.ManagedBuilt() == false` and both invoke buildLocalPolicies.
// The store itself remains correct (writes are mutex-guarded inside the
// PolicyStore impl), so duplicate Set calls are idempotent — but two AWS
// ListPolicies pagination walks may run in parallel before one wins the
// MarkManagedBuilt race. The previous package-global `allPoliciesMu`
// serialized this. Acceptable here because: (a) lazy-add is the
// related-panel drill-in path, not high-volume; (b) duplicate Sets are
// last-write-wins on identical data, so the final cache state is consistent;
// (c) introducing a sync.Once or
// build-in-progress flag would re-couple the transport layer to a session
// concern the transport layer should not own. Same applies to InlineBuilt.
func FetchIAMPoliciesByIDsFull(ctx context.Context, api IAMAPI, ids []string, store iamPolicyStore) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if store == nil {
		return nil, fmt.Errorf("policy FetchByIDs: IAMPolicies store not initialized")
	}

	if !store.ManagedBuilt() {
		if err := buildLocalPolicies(ctx, api, store); err != nil {
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
			continue
		}
		// Not customer-managed or inline — resolve as an AWS-managed policy by
		// name via a single GetPolicy (arn:aws:iam::aws:policy/<name>), rather
		// than having listed the whole AWS-managed catalog. Cache the hit.
		if r, err := getAWSManagedPolicyByName(ctx, api, id); err == nil {
			store.Set(id, r)
			resources = append(resources, r)
		} else {
			failures = append(failures, fmt.Sprintf("%s: %v", id, err))
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
func FetchIAMPoliciesByIDs(ctx context.Context, api IAMListPoliciesAPI, ids []string, store iamPolicyStore) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	if !store.ManagedBuilt() {
		if err := buildLocalPolicies(ctx, api, store); err != nil {
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

// buildLocalPolicies paginates ListPolicies(Scope=Local) and populates the
// store with the account's CUSTOMER-managed policies only. It deliberately does
// NOT list Scope=All: the ~1000+ AWS-managed policy catalog is account-
// independent and huge, so listing it blows the per-open call budget and times
// out even on empty accounts. AWS-managed policy names are instead resolved
// on demand, one GetPolicy per requested name, in FetchIAMPoliciesByIDsFull.
func buildLocalPolicies(ctx context.Context, api IAMListPoliciesAPI, store iamPolicyStore) error {
	var marker *string
	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListPoliciesOutput, error) {
			return api.ListPolicies(ctx, &iam.ListPoliciesInput{
				Scope:    iamtypes.PolicyScopeTypeLocal,
				MaxItems: aws.Int32(DefaultPageSize),
				Marker:   marker,
			})
		})
		if err != nil {
			return fmt.Errorf("listing customer-managed IAM policies for lazy-add: %w", err)
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
				Findings:  orphanUnattachedPolicyFinding(attachmentCount, p.IsAttachable),
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
