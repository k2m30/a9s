// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

const (
	holderRole  = "role"
	holderUser  = "user"
	holderGroup = "group"
)

// IAMListUserPoliciesAPI, IAMGetUserPolicyAPI and IAMGetGroupPolicyAPI read
// a user's and a group's inline policies. They are asserted, not part of
// IAMAPI: a client that does not serve them has its holders' inline policies
// left unread, as nothing was attempted, the way every asserted IAM read is.
type IAMListUserPoliciesAPI interface {
	ListUserPolicies(ctx context.Context, params *iam.ListUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error)
}

type IAMGetUserPolicyAPI interface {
	GetUserPolicy(ctx context.Context, params *iam.GetUserPolicyInput, optFns ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error)
}

type IAMGetGroupPolicyAPI interface {
	GetGroupPolicy(ctx context.Context, params *iam.GetGroupPolicyInput, optFns ...func(*iam.Options)) (*iam.GetGroupPolicyOutput, error)
}

// policySet is the policies one holder carries. via names the group a user
// holds them through, "" for the principal's own.
type policySet struct {
	kind, holder, via string
	attached          []iamtypes.AttachedPolicy
	inline            []string
}

type heldPolicy struct {
	name, via string
	inline    bool
}

// rows are the supporting Attention rows naming the policy and, for one
// inherited from a group, the group.
func (h heldPolicy) rows(tier string) []domain.DetailRow {
	rows := []domain.DetailRow{{Label: "Policy", Value: h.name, Tier: tier}}
	if h.via != "" {
		rows = append(rows, domain.DetailRow{Label: "Group", Value: h.via, Tier: tier})
	}
	return rows
}

// principalVerdict is what the documents a principal holds make it, read the
// way IAM reads them: the union of every identity policy, where an explicit
// Deny in any of them wins, cut down to what the permissions boundary also
// allows. admin is the policy whose statement grants everything; privEsc
// the escalation combinations, empty when admin is set (one fact is stated
// once), granted through the policies in grantedBy. inlineNeeded is true when
// no combination is granted without an inline policy. errs are the reads that
// did not answer: a document nobody read may hold the Deny or the boundary
// that decides, so with any of them the verdict raises nothing.
type principalVerdict struct {
	admin        *heldPolicy
	privEsc      []string
	grantedBy    []heldPolicy
	inlineNeeded bool
	boundary     string
	errs         []error
}

func (v principalVerdict) adminRows() []domain.DetailRow {
	return v.withBoundary(v.admin.rows("~"), "~")
}

func (v principalVerdict) privEscRows() []domain.DetailRow {
	var rows []domain.DetailRow
	for _, h := range v.grantedBy {
		rows = append(rows, h.rows("!")...)
	}
	return v.withBoundary(append(rows, privEscComboRows(v.privEsc)...), "!")
}

func (v principalVerdict) withBoundary(rows []domain.DetailRow, tier string) []domain.DetailRow {
	if v.boundary == "" {
		return rows
	}
	return append(rows, domain.DetailRow{Label: "Permissions boundary", Value: v.boundary, Tier: tier})
}

// principalPolicyVerdict is the one judge of "administrator" and "privilege
// escalation" for a role, user or group. It reads every document of every set
// and of the boundary through the session's policy document cache, once per
// policy however many principals hold it, joins the sets' statements into one
// document and asks core/iampolicy its two questions of that union and of the
// boundary. unread says a list the sets came from was not read in full.
func principalPolicyVerdict(ctx context.Context, clients *ServiceClients, sets []policySet, boundaryARN string, unread bool) principalVerdict {
	var v principalVerdict
	var held identityPolicies
	add := func(h heldPolicy, call string, doc any, err error) {
		if err != nil {
			v.errs = append(v.errs, err)
			return
		}
		parsed, ok := parsePolicyDoc(doc)
		if !ok {
			v.errs = append(v.errs, UnusableAnswerErr{Call: call, Field: "policy document for " + h.name})
			return
		}
		held.add(h, parsed)
	}
	for _, s := range sets {
		for _, p := range s.attached {
			policyARN := aws.ToString(p.PolicyArn)
			doc, err := managedPolicyDocument(ctx, clients, policyARN)
			add(heldPolicy{name: cmp.Or(aws.ToString(p.PolicyName), policyARN), via: s.via}, "GetPolicyVersion", doc, err)
		}
		read, call := inlinePolicyReader(clients.IAM, s.kind)
		if read == nil {
			continue
		}
		for _, name := range s.inline {
			doc, err := clients.PolicyDocs().load(ctx, holderInlineKey(s.kind, s.holder, name), func(ctx context.Context) (any, error) {
				return read(ctx, s.holder, name)
			})
			add(heldPolicy{name: name, via: s.via, inline: true}, call, doc, err)
		}
	}
	var boundary iampolicy.Document
	hasBoundary := boundaryARN != ""
	if hasBoundary {
		doc, err := managedPolicyDocument(ctx, clients, boundaryARN)
		parsed, ok := parsePolicyDoc(doc)
		switch {
		case err != nil:
			v.errs = append(v.errs, err)
		case !ok:
			v.errs = append(v.errs, UnusableAnswerErr{Call: "GetPolicyVersion", Field: "permissions boundary document"})
		}
		boundary = parsed
		v.boundary = policyNameOf(boundaryARN)
	}
	if unread || len(v.errs) > 0 {
		return v
	}
	if admin := held.adminSource(); admin != nil && (!hasBoundary || boundary.IsAdmin()) {
		v.admin = admin
		return v
	}
	withinBoundary := func(combos []string) []string {
		if !hasBoundary {
			return combos
		}
		bounded := boundary.PrivilegeEscalation()
		var both []string
		for _, c := range combos {
			if slices.Contains(bounded, c) {
				both = append(both, c)
			}
		}
		return both
	}
	v.privEsc = withinBoundary(held.escalation())
	if len(v.privEsc) == 0 {
		return v
	}
	v.grantedBy = held.grantedBy(v.privEsc)
	withoutInline := withinBoundary(held.withoutInline().escalation())
	v.inlineNeeded = !slices.ContainsFunc(v.privEsc, func(c string) bool { return slices.Contains(withoutInline, c) })
	return v
}

func policyNameOf(policyARN string) string {
	a, ok := ARNForService(policyARN, "iam")
	if !ok {
		return policyARN
	}
	return path.Base(a.Resource)
}

// managedPolicyDocument is a managed policy's default document, read once per
// session for every list that judges it.
func managedPolicyDocument(ctx context.Context, clients *ServiceClients, policyARN string) (any, error) {
	return clients.PolicyDocs().load(ctx, ManagedKey(policyARN), func(ctx context.Context) (any, error) {
		return FetchManagedPolicyDocument(ctx, clients.IAM, clients.IAM, policyARN)
	})
}

// inlinePolicyReader returns the read of kind's inline policies and the call
// it makes, or nil when api does not serve it.
func inlinePolicyReader(api IAMAPI, kind string) (func(ctx context.Context, holder, name string) (any, error), string) {
	switch kind {
	case holderUser:
		getAPI, ok := api.(IAMGetUserPolicyAPI)
		if !ok {
			return nil, ""
		}
		return func(ctx context.Context, holder, name string) (any, error) {
			return fetchUserInlinePolicy(ctx, getAPI, holder, name)
		}, "GetUserPolicy"
	case holderGroup:
		getAPI, ok := api.(IAMGetGroupPolicyAPI)
		if !ok {
			return nil, ""
		}
		return func(ctx context.Context, holder, name string) (any, error) {
			return fetchGroupInlinePolicy(ctx, getAPI, holder, name)
		}, "GetGroupPolicy"
	}
	return func(ctx context.Context, holder, name string) (any, error) {
		return FetchInlinePolicyDocument(ctx, api, holder, name)
	}, "GetRolePolicy"
}

func fetchUserInlinePolicy(ctx context.Context, api IAMGetUserPolicyAPI, userName, name string) (any, error) {
	out, err := api.GetUserPolicy(ctx, &iam.GetUserPolicyInput{UserName: aws.String(userName), PolicyName: aws.String(name)})
	if err != nil {
		return nil, fmt.Errorf("GetUserPolicy: %w", err)
	}
	if out.PolicyDocument == nil {
		return nil, UnusableAnswerErr{Call: "GetUserPolicy", Field: "policy document"}
	}
	return decodePolicyDocument(*out.PolicyDocument)
}

func fetchGroupInlinePolicy(ctx context.Context, api IAMGetGroupPolicyAPI, groupName, name string) (any, error) {
	out, err := api.GetGroupPolicy(ctx, &iam.GetGroupPolicyInput{GroupName: aws.String(groupName), PolicyName: aws.String(name)})
	if err != nil {
		return nil, fmt.Errorf("GetGroupPolicy: %w", err)
	}
	if out.PolicyDocument == nil {
		return nil, UnusableAnswerErr{Call: "GetGroupPolicy", Field: "policy document"}
	}
	return decodePolicyDocument(*out.PolicyDocument)
}

// walkErr is the error a policy list walk leaves its holder's verdict with:
// the call that failed, or the page cap when the walk stopped short of the
// last page.
func walkErr(complete bool, err error) error {
	if err == nil && !complete {
		return errPageCap
	}
	return err
}

func rolePolicySet(ctx context.Context, api IAMAPI, roleName string) (policySet, []error) {
	attached, attachedComplete, attachedErr := listAttachedRolePolicies(ctx, api, roleName)
	inline, inlineComplete, inlineErr := listRolePolicyNames(ctx, api, roleName)
	return policySet{kind: holderRole, holder: roleName, attached: attached, inline: inline},
		collectErrs(walkErr(attachedComplete, attachedErr), walkErr(inlineComplete, inlineErr))
}

type groupPolicyLists struct {
	attached                         []iamtypes.AttachedPolicy
	inline                           []string
	attachedComplete, inlineComplete bool
	attachedErr, inlineErr           error
}

// groupPolicies lists a group's attached and inline policies once per
// session: the group list and every member's verdict read the same answer.
// An answer with a failed walk is not kept.
func groupPolicies(ctx context.Context, clients *ServiceClients, groupName string) groupPolicyLists {
	got, _ := clients.PolicyDocs().load(ctx, "group-policies:"+groupName, func(ctx context.Context) (any, error) {
		var g groupPolicyLists
		g.attached, g.attachedComplete, g.attachedErr = iamGroupAttachedPolicies(ctx, clients.IAM, groupName)
		g.inline, g.inlineComplete, g.inlineErr = iamGroupInlinePolicies(ctx, clients.IAM, groupName)
		if g.attachedErr != nil || g.inlineErr != nil {
			return g, errGroupListUnread
		}
		return g, nil
	})
	g, _ := got.(groupPolicyLists)
	return g
}

var errGroupListUnread = fmt.Errorf("group policy list unread")

// userPolicySets lists a user's own attached and inline policies and those
// of every group the user is in: IAM grants a user the union of all of them.
func userPolicySets(ctx context.Context, clients *ServiceClients, userName string) ([]policySet, []error) {
	api := clients.IAM
	attached, attachedComplete, attachedErr := listAttachedUserPolicies(ctx, api, userName)
	var inline []string
	inlineComplete, inlineErr := true, error(nil)
	if listAPI, ok := api.(IAMListUserPoliciesAPI); ok {
		inline, inlineComplete, inlineErr = PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]string, *string, error) {
			out, err := listAPI.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{UserName: aws.String(userName), Marker: marker})
			if err != nil {
				return nil, nil, err
			}
			return out.PolicyNames, iamNextMarker(out.IsTruncated, out.Marker), nil
		})
	}
	groups, groupsComplete, groupsErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.Group, *string, error) {
		out, err := api.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{UserName: aws.String(userName), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.Groups, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
	sets := []policySet{{kind: holderUser, holder: userName, attached: attached, inline: inline}}
	errs := collectErrs(walkErr(attachedComplete, attachedErr), walkErr(inlineComplete, inlineErr), walkErr(groupsComplete, groupsErr))
	for _, g := range groups {
		name := aws.ToString(g.GroupName)
		lists := groupPolicies(ctx, clients, name)
		sets = append(sets, policySet{kind: holderGroup, holder: name, via: name, attached: lists.attached, inline: lists.inline})
		errs = append(errs, collectErrs(walkErr(lists.attachedComplete, lists.attachedErr), walkErr(lists.inlineComplete, lists.inlineErr))...)
	}
	return sets, errs
}

func listRolePolicyNames(ctx context.Context, api IAMListRolePoliciesAPI, roleName string) ([]string, bool, error) {
	// ListRolePolicies pages by IsTruncated and Marker
	// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListRolePolicies.html).
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]string, *string, error) {
		out, err := api.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String(roleName), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		if out == nil {
			return nil, nil, UnusableAnswerErr{Call: "ListRolePolicies", Field: "policy names"}
		}
		return out.PolicyNames, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}

func collectErrs(errs ...error) []error {
	var out []error
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return out
}
