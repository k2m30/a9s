// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// iamPolicyCodeOrphanUnattached is the canonical FindingCode for a
// customer-managed policy that is attachable but currently attached to
// nothing (AttachmentCount == 0) — dead permission surface worth pruning.
const iamPolicyCodeOrphanUnattached domain.FindingCode = "iam-policy.orphan-unattached"

// orphanUnattachedPolicyFinding returns the wave1 Finding for an attachable
// policy with zero attachments, or nil when the policy doesn't match.
func orphanUnattachedPolicyFinding(attachmentCount string, isAttachable bool) []domain.Finding {
	if attachmentCount == "0" && isAttachable {
		return []domain.Finding{wave1Finding(iamPolicyCodeOrphanUnattached)}
	}
	return nil
}

// iamPolicyStore is the subset of session.PolicyStore consumed by this file.
// Defined locally to avoid an import cycle (core/session imports core/aws).
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

// managedPolicyToResource builds the canonical policy Resource from an IAM
// Policy — used by the ListPolicies page fetch, the local-policy sweep and
// the per-name GetPolicy lazy-add, so the three paths cannot drift in shape.
//
// The row is keyed by the ARN, which is the identity IAM itself uses: a name
// is unique among the account's own policies only, and an AWS-managed policy
// of that name lives under account "aws" alongside it.
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
	// Distinguish AWS-managed from customer-managed by ARN so lazy-added
	// AWS-managed policies (resolved via getAWSManagedPolicy) match the
	// vocabulary buildLocalPolicies uses and callers/tests rely on.
	policyType := "managed"
	if policy.Arn != nil && !IsCustomerManagedIAMPolicyARN(*policy.Arn) {
		policyType = "aws-managed"
	}
	arn := ""
	if policy.Arn != nil {
		arn = *policy.Arn
	}
	return resource.Resource{
		ID:   arn,
		Name: policyName,
		Fields: map[string]string{
			"policy_name":      policyName,
			"policy_type":      policyType,
			"attachment_count": attachmentCount,
			"is_attachable":    isAttachable,
			"path":             path,
			"create_date":      createDate,
			"arn":              arn,
		},
		Findings:  orphanUnattachedPolicyFinding(attachmentCount, policy.IsAttachable),
		RawStruct: policy,
	}
}

// awsManagedPolicyPathPrefixes are the ARN path prefixes AWS-managed policies
// live under. Most are root ("/"); job-function and service-role policies carry
// a path that the bare policy name does not reveal, so getAWSManagedPolicy
// tries each in turn — a handful of GetPolicy calls, still bounded and vastly
// cheaper than listing the whole ~1000+ AWS-managed catalog.
var awsManagedPolicyPaths = []string{ //nolint:gochecknoglobals // static AWS-managed path set
	"policy/",
	"policy/service-role/",
	"policy/job-function/",
	"policy/aws-service-role/",
}

// getAWSManagedPolicy resolves ONE AWS-managed policy via GetPolicy: on the
// ARN itself when the reference is one, otherwise on the well-known ARNs
// arn:aws:iam::aws:policy[/<path>]/<name>. This is how the lazy-add resolves
// the handful of AWS-managed policies a checker emitted, WITHOUT listing the
// whole ~1000+ AWS-managed catalog.
func getAWSManagedPolicy(ctx context.Context, api IAMGetPolicyAPI, partition, ref string) (resource.Resource, error) {
	paths := awsManagedPolicyPaths
	name := ref
	if strings.HasPrefix(ref, "arn:") {
		paths, name = []string{""}, ""
	}
	var lastErr error
	for _, path := range paths {
		arn := "arn:" + partition + ":iam::aws:" + path + name
		if name == "" {
			arn = ref
		}
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
// Concurrency trade-off: no top-level lock is held across the
// check-build-mark sequence. Two concurrent lazy-add calls can both observe
// `store.ManagedBuilt() == false` and both invoke buildLocalPolicies.
// The store itself remains correct (writes are mutex-guarded inside the
// PolicyStore impl), so duplicate Set calls are idempotent — but two AWS
// ListPolicies pagination walks may run in parallel before one wins the
// MarkManagedBuilt race. Acceptable here because: (a) lazy-add is the
// related-panel drill-in path, not high-volume; (b) duplicate Sets are
// last-write-wins on identical data, so the final cache state is consistent;
// (c) introducing a sync.Once or
// build-in-progress flag would re-couple the transport layer to a session
// concern the transport layer should not own. Same applies to InlineBuilt.
func FetchIAMPoliciesByIDsFull(ctx context.Context, api IAMAPI, ids []string, store iamPolicyStore, partition string) ([]resource.Resource, error) {
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
		inlines, _, inlineErr := fetchInlineGroupPolicies(ctx, api)
		for _, r := range inlines {
			store.Set(r.ID, r)
		}
		if inlineErr != nil {
			// Non-fatal: managed policies are already loaded; partial inline
			// results are incorporated above. Leave InlineBuilt=false
			// so next call retries the inline fetch — it might succeed after a
			// transient throttle. Surface as aggregate failure with the partial
			// results we did recover.
			var failures []Failure
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
					failures = append(failures, UnusableAnswer(id, "not found"))
				}
			}
			return resources, JoinAggregates(AggregateFailures("policy FetchByIDs", failures, len(ids)), inlineErr)
		}
		store.MarkInlineBuilt()
	}

	var failures []Failure
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
		// Not customer-managed or inline — resolve as an AWS-managed policy
		// via a single GetPolicy, rather than having listed the whole
		// AWS-managed catalog. An id that is already an ARN names the policy
		// outright; a bare name is tried under each path AWS-managed policies
		// live under. Cache the hit.
		if r, err := getAWSManagedPolicy(ctx, api, partition, id); err == nil {
			store.Set(id, r)
			resources = append(resources, r)
		} else {
			failures = append(failures, FailedCall(id, err))
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
	// ListPolicies pages by IsTruncated and Marker, at most 1000 items a page
	// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListPolicies.html).
	policies, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.Policy, *string, error) {
		out, err := api.ListPolicies(ctx, &iam.ListPoliciesInput{
			Scope:    iamtypes.PolicyScopeTypeLocal,
			MaxItems: aws.Int32(1000),
			Marker:   marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Policies, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
	if err == nil && !complete {
		err = fmt.Errorf("more than %d pages", PerParentPageCap)
	}
	if err != nil {
		return fmt.Errorf("listing customer-managed IAM policies for lazy-add: %w", err)
	}
	for _, p := range policies {
		if aws.ToString(p.PolicyName) == "" {
			continue
		}
		r := managedPolicyToResource(p)
		// Indexed by name as well as by ID, so a reference that names the
		// policy the way a checker read it — a bare name from an
		// attachment, an ARN from an SDK struct — resolves either way.
		store.Set(r.ID, r)
		if name := aws.ToString(p.PolicyName); name != r.ID {
			store.Set(name, r)
		}
	}
	return nil
}

// fetchInlineGroupPolicies enumerates all groups via ListGroups and collects
// their inline policy names via ListGroupPolicies. Both API calls are wrapped
// in RetryOnThrottle. Per-group ListGroupPolicies failures are collected and
// returned as a composite error alongside any partial results — callers must
// check both return values.
// iamInlineGroupSweepParallelism bounds the per-group ListGroupPolicies
// fan-out fetchInlineGroupPolicies runs, distinct from the general
// EnrichmentParallelism: IAM's global per-account API rate limit is shared
// across every IAM call the whole session makes, not just this sweep, so it
// stays capped lower than a typical per-resource enrichment fan-out.
const iamInlineGroupSweepParallelism = 5

func fetchInlineGroupPolicies(ctx context.Context, api IAMAPI) ([]resource.Resource, bool, error) {
	// ListGroups pages by IsTruncated and Marker
	// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListGroups.html).
	groups, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.Group, *string, error) {
		out, err := api.ListGroups(ctx, &iam.ListGroupsInput{Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.Groups, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("listing IAM groups for inline policies: %w", err)
	}

	n := len(groups)
	// perGroup keeps each group's inline resources at its own index — the
	// bounded fan-out below writes concurrently, so a single shared slice
	// (mutex or not) would need its own ordering discipline; indexing by
	// group position instead makes the final flatten below deterministic
	// regardless of completion order.
	perGroup := make([][]resource.Resource, n)
	visited := make([]bool, n)
	var mu sync.Mutex
	var groupFailures []Failure

	// Bounded fan-out (ForEachParallel, core/aws/parallel.go — the same
	// mechanism issue-enrichment fetchers use): sequential per-group calls
	// cannot finish a wide account (49+ groups) inside any shared deadline
	// (the probe's 10s budget, or a real list-open's own timeout); unbounded
	// fan-out risks IAM throttling instead. RetryOnThrottle still wraps each
	// individual ListGroupPolicies call. sweepErr is ForEachParallel's own
	// return value (a ctx error when the sweep's deadline expired before
	// every group was even scheduled) — captured, not discarded: a group
	// whose turn never comes contributes nothing to groupFailures (only a
	// group's OWN ListGroupPolicies call actually failing does), so without
	// this the sweep would silently return a partial result with a nil
	// error whenever the deadline expires before any single group's call
	// has failed on its own.
	capped := false
	sweepErr := ForEachParallel(ctx, n, iamInlineGroupSweepParallelism, func(i int) {
		group := groups[i]
		if group.GroupName == nil {
			return
		}
		groupName := *group.GroupName
		mu.Lock()
		visited[i] = true
		mu.Unlock()
		policyNames, whole, gpErr := iamGroupInlinePolicies(ctx, api, groupName)
		if gpErr != nil {
			mu.Lock()
			groupFailures = append(groupFailures, FailedCall(groupName, gpErr))
			mu.Unlock()
			return
		}
		names := make([]resource.Resource, 0, len(policyNames))
		for _, name := range policyNames {
			names = append(names, resource.Resource{
				ID:   inlinePolicyID(groupName, name),
				Name: name,
				Fields: map[string]string{
					"policy_name":      name,
					"policy_type":      "inline",
					"attachment_count": "",
					"path":             inlinePolicyPathPrefix + groupName,
					"create_date":      "",
				},
			})
		}
		mu.Lock()
		perGroup[i] = names
		capped = capped || !whole
		mu.Unlock()
	})

	var resources []resource.Resource
	for _, rs := range perGroup {
		resources = append(resources, rs...)
	}

	if sweepErr != nil {
		unvisited := 0
		for _, v := range visited {
			if !v {
				unvisited++
			}
		}
		if unvisited > 0 {
			// Prepended, not appended: AggregateFailures enumerates only the
			// first aggregateFailuresCap entries verbatim before summarizing
			// the rest — appending this summary last would let a wide
			// per-group failure set (as here) push it past the cap and
			// silently drop the one entry that names the truly UNSWEPT
			// remainder (never even attempted), the whole point of this
			// check.
			groupFailures = append([]Failure{FailedCall(
				fmt.Sprintf("%d of %d groups never visited before the sweep's context ended", unvisited, n), sweepErr)}, groupFailures...)
		}
	}

	return resources, complete && !capped, AggregateFailures("ListGroupPolicies", groupFailures, n)
}

// inlinePolicyPathPrefix marks the path of a group's inline policy row,
// followed by the group's name.
const inlinePolicyPathPrefix = "inline/"

// inlinePolicyID is the row identity of a group's inline policy: the name is
// the group's to choose, so two groups may each carry one called "s3-read",
// and the account may hold a managed policy of that name as well.
func inlinePolicyID(groupName, policyName string) string {
	return inlinePolicyPathPrefix + groupName + "/" + policyName
}

// inlinePolicyGroup returns the group an inline policy row belongs to.
func inlinePolicyGroup(res resource.Resource) (string, bool) {
	return afterPrefix(res.Fields["path"], inlinePolicyPathPrefix)
}
