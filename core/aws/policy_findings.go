// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// policy_findings.go owns the vocabulary every policy-derived finding shares:
// the one definition of "an AWS-managed policy that hands out
// administrator-equivalent power" (so the role, user and group
// ListAttached*Policies sweeps can never disagree), and the one reader of a
// policy attached to a resource: its parse, its evaluation, and the findings,
// marks, verdict words and rows it yields.
package aws

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// adminManagedPolicyResources are the AWS-managed policies whose attachment
// makes the principal effectively an account administrator, i.e. whose
// document allows every action on every resource.
// They are named by their resource path rather than their full ARN: AWS
// returns the ARN in the partition the session is connected to, so a literal
// commercial ARN reports every China and GovCloud administrator unprivileged.
//
// AdministratorAccess is the only member. PowerUserAccess is deliberately not
// one: AWS's document allows "*" on "*" only with a NotAction that excludes
// IAM, Organizations and Account, so its holder can neither grant itself
// permissions nor touch the account.
var adminManagedPolicyResources = []string{ //nolint:gochecknoglobals // static AWS-managed policy set
	"policy/AdministratorAccess",
}

// broadPowerManagedPolicyResources are the AWS-managed policies that grant
// far more than any one job needs without amounting to administrator. They
// are worth showing where the operator is reading a principal's policy list,
// but they are NOT administrator-equivalent and never feed the admin-attached
// finding.
var broadPowerManagedPolicyResources = []string{ //nolint:gochecknoglobals // static AWS-managed policy set
	"policy/PowerUserAccess",
}

// awsManagedPolicyIn reports whether policyARN names an AWS-managed policy
// belonging to one of the given sets. It is the only test of that question:
// the ARN is parsed, so a customer-managed policy an operator happened to
// name "AdministratorAccess" is not one of AWS's, and every partition's
// spelling of AWS's own is.
func awsManagedPolicyIn(policyARN string, sets ...[]string) bool {
	a, awsOwned := awsManagedPolicy(policyARN)
	if !awsOwned {
		return false
	}
	for _, set := range sets {
		if slices.Contains(set, a.Resource) {
			return true
		}
	}
	return false
}

// iamNextMarker is the token of the next IAM list page: IAM marks a page
// followed by another with IsTruncated, and its Marker is meaningful only then.
func iamNextMarker(isTruncated bool, marker *string) *string {
	if !isTruncated {
		return nil
	}
	return marker
}

// listAttachedRolePolicies walks every page of a role's attached managed
// policies. complete is false when the walk failed or stopped at the page
// cap; the policies read so far are returned either way.
func listAttachedRolePolicies(ctx context.Context, api IAMListAttachedRolePoliciesAPI, roleName string) ([]iamtypes.AttachedPolicy, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.AttachedPolicy, *string, error) {
		out, err := api.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
			Marker:   marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.AttachedPolicies, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}

// listAttachedUserPolicies is listAttachedRolePolicies for a user.
func listAttachedUserPolicies(ctx context.Context, api IAMListAttachedUserPoliciesAPI, userName string) ([]iamtypes.AttachedPolicy, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.AttachedPolicy, *string, error) {
		out, err := api.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(userName),
			Marker:   marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.AttachedPolicies, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}

// AdminAttachedPolicyNameForTest is an exported test-only wrapper for the
// unexported adminAttachedPolicyName — production code does not call it.
// Lives outside _test.go because tests in tests/unit/ are package unit and
// can't see same-package test helpers.
func AdminAttachedPolicyNameForTest(attached []iamtypes.AttachedPolicy) string {
	return adminAttachedPolicyName(attached)
}

// adminAttachedPolicyName returns the name of the first admin-equivalent
// managed policy in the attachment list, or "" when none is attached.
func adminAttachedPolicyName(attached []iamtypes.AttachedPolicy) string {
	for _, p := range attached {
		if awsManagedPolicyIn(aws.ToString(p.PolicyArn), adminManagedPolicyResources) {
			name := aws.ToString(p.PolicyName)
			if name == "" {
				name = aws.ToString(p.PolicyArn)
			}
			return name
		}
	}
	return ""
}

// adminAttachedPhrase is the cause for every admin-attached finding. It
// names the class rather than one policy, so the set can grow without the
// phrase becoming a lie; the Policy row says which one is attached.
const adminAttachedPhrase = "has an administrator policy"

// adminAttachedRows is the supporting Attention row shared by the role, user
// and group admin-attached findings.
func adminAttachedRows(policyName string) []domain.DetailRow {
	return []domain.DetailRow{{Label: "Policy", Value: policyName, Tier: "~"}}
}

// parsePolicy is the one parse of a policy document in core/aws. A policy
// attached to a resource is read through readPolicy; an identity policy, read
// for its privileges, comes here and is asked its questions through
// core/iampolicy's named methods, so every reader of a document that does not
// parse gets the same error.
func parsePolicy(document string) (iampolicy.Document, error) {
	return iampolicy.Parse(document)
}

// ownAccountOf is the account a policy attached to a resource is judged
// against: the session's, or the one the resource's ARN names when STS has
// not answered.
func ownAccountOf(sessionAccount, resourceARN string) string {
	return cmp.Or(sessionAccount, iampolicy.AccountFromARN(resourceARN))
}

// grantedPrincipalRefs returns the IAM principal ARNs of kind ("role/",
// "user/", "group/") that policy grants once its explicit Deny statements are
// applied, in every account: the resolver leaves out another account's. ok is
// false when the policy does not parse.
func grantedPrincipalRefs(policy, kind string) (refs []string, ok bool) {
	doc, err := parsePolicy(policy)
	if err != nil {
		return nil, false
	}
	for _, p := range iampolicy.AllowedPrincipals(doc, "") {
		if a, isIAM := ARNForService(p.Value, "iam"); p.Kind == "AWS" && isIAM && strings.HasPrefix(a.Resource, kind) {
			refs = append(refs, p.Value)
		}
	}
	return refs, true
}

// CheckPolicyUnreadable is what a row records when the policy attached to it
// does not parse, and CheckPolicyMissing when a read that always carries a
// document (catalog.PolicyDecl.Required) answered without one: nothing about
// who the policy grants can be read, so the policy checks did not run.
// Nothing refused, so neither is a failed call.
const (
	CheckPolicyUnreadable = "policy does not parse"
	CheckPolicyMissing    = "no policy document returned"
)

// policyDeclOf is the declaration of short. A type that reads a policy
// without declaring one is a programming error, caught by the tests that
// drive every reader.
func policyDeclOf(short string) catalog.PolicyDecl {
	if td := catalog.FindAny(short); td != nil && td.Policy != nil {
		return *td.Policy
	}
	if testing.Testing() {
		panic("readPolicy: type " + short + " declares no catalog Policy")
	}
	return catalog.PolicyDecl{}
}

// policyReading is one read of a policy attached to a resource: a resource,
// key, queue, topic or repository policy, a role's trust policy, a domain's
// access policy, an endpoint policy. Every finding, mark and verdict word
// read from such a document comes from it, so "a document nobody could read
// is never clean", the own-account fallback and the rows of the finding hold
// for every type at once.
type policyReading struct {
	short   string
	decl    catalog.PolicyDecl
	present bool
	parsed  bool
	own     string
	doc     iampolicy.Document
	ex      iampolicy.Exposure
}

// readPolicy reads the policy of a resource of type short, the way its
// declaration says. resourceARN is "" for a type whose ARN names no account.
// An empty document and the JSON null both carry no policy.
func readPolicy(short, document, ownAccount, resourceARN string) policyReading {
	p := policyReading{short: short, decl: policyDeclOf(short)}
	document = strings.TrimSpace(document)
	p.present = document != "" && document != "null"
	if !p.present {
		return p
	}
	doc, err := parsePolicy(document)
	if err != nil {
		return p
	}
	p.parsed, p.doc, p.own = true, doc, ownAccountOf(ownAccount, resourceARN)
	if p.decl.Kind == catalog.PolicyEndpoint {
		p.ex = endpointExposure(doc)
	} else {
		p.ex = iampolicy.Evaluate(doc, p.own)
	}
	return p
}

// endpointExposure reads a VPC endpoint policy, which is public only when a
// wildcard principal, a wildcard action and a wildcard resource meet in ONE
// statement no Deny closes: that combination, and only it, is the full-access
// policy AWS attaches when the endpoint is created without one. Each Allow is
// evaluated beside the document's Deny statements on its own, because the
// aggregate Exposure merges the actions of every public statement, so one
// statement granting "*" on a bucket and another granting s3:GetObject on "*"
// would read as a full-access policy neither of them is.
func endpointExposure(doc iampolicy.Document) iampolicy.Exposure {
	var denies []iampolicy.Statement
	for _, st := range doc.Statement {
		if st.Effect == "Deny" {
			denies = append(denies, st)
		}
	}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" || !slices.Contains(st.Resource, "*") {
			continue
		}
		ex := iampolicy.EvaluateEndpoint(iampolicy.Document{Statement: append([]iampolicy.Statement{st}, denies...)})
		if ex.Public && slices.ContainsFunc(ex.PublicActions, isWildcardAction) {
			return ex
		}
	}
	return iampolicy.Exposure{}
}

// isWildcardAction reports whether an IAM action string grants every action
// the endpoint can reach. An endpoint only ever fronts one service, so
// "s3:*" withholds nothing that "*" would have granted through it.
func isWildcardAction(action string) bool {
	_, verb, hasService := strings.Cut(action, ":")
	if hasService {
		return verb == "*"
	}
	return action == "*"
}

func (p policyReading) unreadCheck() string {
	switch {
	case p.present && !p.parsed:
		return CheckPolicyUnreadable
	case !p.present && p.decl.Required:
		return CheckPolicyMissing
	}
	return ""
}

func (p policyReading) unread() bool { return p.unreadCheck() != "" }

func (p policyReading) absent() bool { return !p.present }

func (p policyReading) public() bool { return p.ex.Public }

// word is the Wave 1 verdict a row shows for its policy: unknown when the
// document could not be read, so a row never reads private or scoped on a
// policy nobody could read.
func (p policyReading) word(public, closed string) string {
	switch {
	case p.unread():
		return "unknown"
	case p.public():
		return public
	}
	return closed
}

// withUnread is the Wave 1 form of "this policy could not be read": the
// type's declared Unread finding, since a fetcher row has no not-inspected
// mark to carry it.
func (p policyReading) withUnread(findings []domain.Finding) []domain.Finding {
	if p.unread() && p.decl.Unread != "" {
		return append(findings, wave1Finding(p.decl.Unread))
	}
	return findings
}

// servicePrincipalsWithoutSourceScope is the confused-deputy reading of a
// trust policy: the services it trusts with no source condition.
func (p policyReading) servicePrincipalsWithoutSourceScope() []string {
	if !p.parsed {
		return nil
	}
	return p.doc.HasServicePrincipalWithoutSourceScope()
}

// detail is the Attention block of the public finding: who, which actions,
// the conditions the grant carries and the conditional Denies that did not
// close it. Neither condition scopes the caller, so together they say what
// the grant is still open to (every AWS service, whoever presents an external
// ID, whoever calls over TLS); a Deny row reads "Deny when" because IAM
// applies the Deny when its condition matches. A NotAction grant lists no
// actions, so it has no Actions row.
func (p policyReading) detail() domain.AttentionDetail {
	tier := tierOf(p.decl.Public)
	rows := []domain.DetailRow{{Label: "Principal", Value: "*", Tier: tier}}
	add := func(label string, items []string, sep string) {
		if len(items) > 0 {
			rows = append(rows, domain.DetailRow{Label: label, Value: strings.Join(items, sep), Tier: tier})
		}
	}
	add("Actions", p.ex.PublicActions, ", ")
	add("Condition", p.ex.PublicConditions, "; ")
	add("Deny when", p.ex.PublicDenyConditions, "; ")
	return domain.AttentionDetail{Rows: rows}
}

// markUnread marks the row not inspected when its policy could not be read,
// and reports whether it did. The policy it could not read may grant anyone,
// a "!" finding, so the mark makes the issue count a lower bound.
func (p policyReading) markUnread(result *IssueEnricherResult, id string) bool {
	check := p.unreadCheck()
	if check != "" {
		markUninspected(result, id, check)
	}
	return check != ""
}

func (p policyReading) raise(result *IssueEnricherResult, id string) {
	p.raiseIn(result, id, catalog.LifecycleNormal, "")
}

// raiseIn is raise for a row whose lifecycle the enricher read itself: each
// code stands only in the classes it declares. A grant to another account is
// judged only against a known own account: without one every principal reads
// as foreign, which would report the account's own roles as an outside share.
func (p policyReading) raiseIn(result *IssueEnricherResult, id string, class catalog.LifecycleClass, state string) {
	d := p.decl
	switch {
	case p.markUnread(result, id):
	case p.public():
		if catalog.Applies(p.short, d.Public, class, state, "") {
			setWave2Finding(result, id, d.Public, p.detail().Rows)
		}
	case d.Cross == "" || len(p.ex.CrossAccount) == 0:
	case p.own == "":
		markUninspected(result, id, CheckOwnAccountUnknown)
	case catalog.Applies(p.short, d.Cross, class, state, ""):
		setWave2Finding(result, id, d.Cross, []domain.DetailRow{
			{Label: "Accounts", Value: strings.Join(p.ex.CrossAccount, ", "), Tier: tierOf(d.Cross)},
		})
	}
}
