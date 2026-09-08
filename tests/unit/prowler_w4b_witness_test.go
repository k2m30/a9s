package unit

// prowler_w4b_witness_test.go — verification of the two demo witnesses added
// for batch w4: the healthy counterpart of the wildcard-trust rule, and a
// carrier of the second admin-equivalent managed policy.
//
// The wave-1 supporting rows on role (confused-deputy Services, inline
// privilege-escalation Policy and Combo) are asserted on the fetcher's own
// output. The runtime currently drops wave-1 AttentionDetails on the way to
// the rendered surface, so asserting them there would pin a defect that is
// not this batch's.

import (
	"context"
	"slices"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// w4bBench fetches every demo row of shortName and folds the type's
// registered wave-2 enricher onto it, which is the shape the Color funcs and
// the detail view see in production.
func w4bBench(t *testing.T, shortName string) ([]resource.Resource, resource.ResourceTypeDef) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	clients := demo.NewServiceClients()
	rows := DrainPages(t, shortName, func(token string) (resource.FetchResult, error) {
		return td.Fetcher(context.Background(), clients, token)
	})
	if len(rows) == 0 {
		t.Fatalf("%s: no demo rows", shortName)
	}
	enricher, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok || enricher.Fn == nil {
		return rows, *td
	}
	res, err := enricher.Fn(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("%s wave-2: %v", shortName, err)
	}
	for i := range rows {
		runtime.ApplyWave2ToRow(&rows[i], *td, res.Findings, res.AttentionDetails)
	}
	return rows, *td
}

func w4bRow(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no demo row with id %q", id)
	return resource.Resource{}
}

// TestW4bScopedWildcardTrustWitnessIsClean pins the negative case of the
// wildcard-trust rule on screen: a role that trusts any AWS principal but
// only one holding the agreed external ID carries no finding, colours
// healthy, and leaves both the Status and Trust cells blank. A dormant
// warning on this row would fill the cell the healthy claim is read from.
func TestW4bScopedWildcardTrustWitnessIsClean(t *testing.T) {
	rows, td := w4bBench(t, "role")
	r := w4bRow(t, rows, fixtures.RoleScopedWildcardTrust)

	if len(r.Findings) != 0 {
		t.Errorf("witness carries findings: %s", w4CodesOf(r.Findings))
	}
	if got := td.ResolveColor(r); got != domain.ColorHealthy {
		t.Errorf("color = %v, want Healthy", got)
	}
	if got := r.Fields["trust_wildcard"]; got != "false" {
		t.Errorf("trust_wildcard = %q, want %q", got, "false")
	}
	if got := r.Fields["trust_summary"]; got != "" {
		t.Errorf("trust_summary = %q, want empty: the Trust cell reads blank", got)
	}
}

// TestW4bWildcardTrustWitnessesUnchanged pins that adding the healthy
// counterpart did not disturb the positive ones: the same two demo roles are
// flagged as before, and the new row is not among them.
func TestW4bWildcardTrustWitnessesUnchanged(t *testing.T) {
	rows, _ := w4bBench(t, "role")
	var flagged []string
	for _, r := range rows {
		if _, ok := w4FindingByCode(t, r.Findings, "role.trust.wildcard-principal"); ok {
			flagged = append(flagged, r.ID)
		}
	}
	slices.Sort(flagged)
	want := []string{"anyone-can-assume-role", "wildcard-trust-role"}
	if !slices.Equal(flagged, want) {
		t.Errorf("roles flagged wildcard-trust = %v, want %v", flagged, want)
	}
}

// TestW4bPowerUserGroupIsNotAdminAttached pins the near-miss on the bench: a
// group holding only PowerUserAccess is not an administrator.
//
// INVERTED by codex2 row 6 (Codex finding 8). This test used to assert the
// opposite — that platform-engineers carries iam-group.admin-attached with a
// "PowerUserAccess" Policy row — because PowerUserAccess was in the
// admin-equivalent set. AWS's PowerUserAccess allows every action EXCEPT IAM,
// Organizations and Account, so its holder can neither grant itself
// permissions nor touch the account; flagging it reported every developer
// group as an administrator. Do not restore the positive assertion. Whether
// PowerUserAccess deserves a warn-tier finding of its own is a separate
// product question, deliberately not answered here.
func TestW4bPowerUserGroupIsNotAdminAttached(t *testing.T) {
	rows, _ := w4bBench(t, "iam-group")
	r := w4bRow(t, rows, fixtures.IAMGroupPowerUser)

	const code domain.FindingCode = "iam-group.admin-attached"
	w4AssertNoCode(t, r.Findings, code)
}

// TestW4bAdminAttachedWitnessIsTheOneAdminGroup pins that exactly one demo
// group carries the finding and that its Policy row names the policy that
// fired.
//
// INVERTED by codex2 row 6 (Codex finding 8): the want map used to hold a
// second entry, platform-engineers -> PowerUserAccess. There is now one
// administrator-equivalent AWS-managed policy, so there is one witness. Do
// not restore the second entry.
func TestW4bAdminAttachedWitnessIsTheOneAdminGroup(t *testing.T) {
	rows, _ := w4bBench(t, "iam-group")
	const code domain.FindingCode = "iam-group.admin-attached"

	got := map[string]string{}
	for _, r := range rows {
		if _, ok := w4FindingByCode(t, r.Findings, code); !ok {
			continue
		}
		ad, ok := r.AttentionDetails[code]
		if !ok || len(ad.Rows) == 0 {
			t.Fatalf("%s: admin-attached finding with no Policy row", r.ID)
		}
		got[r.ID] = ad.Rows[0].Value
	}
	want := map[string]string{
		fixtures.IAMGroupAdminAttached: "AdministratorAccess",
	}
	if len(got) != len(want) {
		t.Fatalf("groups flagged admin-attached = %v, want %v", got, want)
	}
	for id, policy := range want {
		if got[id] != policy {
			t.Errorf("%s Policy row = %q, want %q", id, got[id], policy)
		}
	}
}

// TestW4bPowerUserResolvesThroughGroupPolicyPivot pins that the policy the
// new witness names is reachable from its row: the group→policy pivot lists
// it, and the by-id fetch behind the pivot resolves it to a real policy
// resource. A pivot that names a policy nothing can open is a dead end.
func TestW4bPowerUserResolvesThroughGroupPolicyPivot(t *testing.T) {
	clients := demo.NewServiceClients()
	td := resource.FindResourceType("iam-group")
	if td == nil {
		t.Fatal("iam-group not registered")
	}
	var checker domain.RelatedChecker
	for _, rel := range td.Related {
		if rel.TargetType == "policy" {
			checker = rel.Checker
		}
	}
	if checker == nil {
		t.Fatal("iam-group has no policy pivot")
	}

	group := resource.Resource{ID: fixtures.IAMGroupPowerUser, Name: fixtures.IAMGroupPowerUser, Type: "iam-group"}
	result := checker(context.Background(), clients, group, nil)
	if !slices.Contains(result.ResourceIDs(), "PowerUserAccess") {
		t.Fatalf("group→policy pivot ids = %v, want to contain PowerUserAccess", result.ResourceIDs())
	}

	store := session.NewPolicyStore()
	resolved, _ := awsclient.FetchIAMPoliciesByIDsFull(
		context.Background(), clients.IAM, []string{"PowerUserAccess"}, store, "aws")
	if len(resolved) == 0 {
		t.Fatal("PowerUserAccess did not resolve to a policy resource")
	}
	if resolved[0].ID != "PowerUserAccess" && resolved[0].Name != "PowerUserAccess" {
		t.Errorf("resolved %q, want PowerUserAccess", resolved[0].ID)
	}
}

// w4bSDKFieldNames are AWS SDK request/response field names. They are how the
// API spells a thing, not how an operator does, so they may not appear in any
// text the views render. IAM action names and condition keys (iam:PassRole,
// sts:ExternalId) are operator vocabulary and are allowed by ruling — they
// carry a colon and never collide with this list.
//
// Ceiling: hand-picked, so it catches a regression only in a name already
// listed. It is not derived from the SDK on purpose — core/config/
// views_reference.yaml holds every field name of every service, and most of
// them ("Policy", "Key", "Name", "Actions") are also the words an operator
// uses, so sweeping against all of them would reject correct text far more
// often than wrong text. Extend by hand when a new type's enricher starts
// reading a field whose name would embarrass the view.
var w4bSDKFieldNames = []string{
	"AccessKeyId", "AssumeRolePolicyDocument", "AttachedPolicies", "CreateDate",
	"KeyManager", "KeyRotationEnabled", "LastUsedDate", "MFADevices",
	"PasswordLastUsed", "PolicyArn", "ResourcePolicy", "RoleLastUsed", "WebACLArn",
}

// TestW4bNoSDKFieldNamesInRenderedText sweeps every phrase, detail sentence
// and attention row the batch's types put on screen in demo mode.
func TestW4bNoSDKFieldNamesInRenderedText(t *testing.T) {
	for _, shortName := range []string{"role", "policy", "iam-user", "iam-group", "waf", "secrets", "kms"} {
		t.Run(shortName, func(t *testing.T) {
			rows, _ := w4bBench(t, shortName)
			for _, r := range rows {
				for _, f := range r.Findings {
					w4bAssertOperatorText(t, r.ID, "phrase", f.Phrase)
					w4bAssertOperatorText(t, r.ID, "detail", f.Detail)
				}
				for code, ad := range r.AttentionDetails {
					for _, row := range ad.Rows {
						w4bAssertOperatorText(t, r.ID, string(code)+" label", row.Label)
						w4bAssertOperatorText(t, r.ID, string(code)+" value", row.Value)
					}
				}
			}
		})
	}
}

func w4bAssertOperatorText(t *testing.T, id, where, text string) {
	t.Helper()
	for _, name := range w4bSDKFieldNames {
		if strings.Contains(text, name) {
			t.Errorf("%s %s carries the SDK field name %q: %q", id, where, name, text)
		}
	}
}

// TestW4bRoleWave1SupportingRowsSurviveTheFetcher pins the confused-deputy
// and inline-privilege-escalation supporting rows at the point the fetcher
// emits them. They are asserted here rather than on the rendered surface
// because the runtime drops wave-1 AttentionDetails today; that gap belongs
// to another batch, and pinning it here would hide these rows regressing.
func TestW4bRoleWave1SupportingRowsSurviveTheFetcher(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := DrainPages(t, "role", func(token string) (resource.FetchResult, error) {
		return awsclient.FetchIAMRolesPage(context.Background(), clients.IAM, token)
	})

	deputy := w4bRow(t, rows, fixtures.RoleConfusedDeputy)
	w4AssertRows(t, deputy.AttentionDetails, "role.trust.confused-deputy",
		[]domain.DetailRow{{Label: "Services", Value: "lambda"}})

	privEsc := w4bRow(t, rows, fixtures.RoleInlinePrivEsc)
	ad, ok := privEsc.AttentionDetails["role.inline-privilege-escalation"]
	if !ok || len(ad.Rows) != 2 {
		t.Fatalf("inline privilege-escalation rows = %+v, want Policy and Combo", ad.Rows)
	}
	if ad.Rows[0].Label != "Policy" || ad.Rows[1].Label != "Combo" {
		t.Errorf("row labels = %q, %q; want Policy, Combo", ad.Rows[0].Label, ad.Rows[1].Label)
	}
	if ad.Rows[1].Value != "PassRole+CreateLambda+Invoke" {
		t.Errorf("Combo = %q, want PassRole+CreateLambda+Invoke", ad.Rows[1].Value)
	}
}

// w4WitnessTypes are the batch's seven types.
var w4WitnessTypes = []string{"role", "policy", "iam-user", "iam-group", "waf", "secrets", "kms"}

// TestW4EveryWave2CodeHasExactlyOneWitness generalises the exactly-one rule
// that was proved for two witnesses by hand.
//
// A demo bench earns its keep only when each signal is demonstrated by one
// row: zero rows means the signal is undemonstrated and no surface test can
// see it, and two rows mean neither is the witness, so changing the rule moves
// a count nobody can attribute. Deriving the set from what the bench actually
// emits means a new wave-2 finding is covered the day it ships, without a hand
// list to forget to extend.
func TestW4EveryWave2CodeHasExactlyOneWitness(t *testing.T) {
	carriers := map[domain.FindingCode][]string{}
	for _, shortName := range w4WitnessTypes {
		rows, _ := w4bBench(t, shortName)
		for _, r := range rows {
			for _, f := range r.Findings {
				if !strings.HasPrefix(f.Source, "wave2:") {
					continue
				}
				carriers[f.Code] = append(carriers[f.Code], shortName+"/"+r.ID)
			}
		}
	}

	if len(carriers) == 0 {
		t.Fatal("no wave-2 finding fired anywhere on the batch's demo bench")
	}

	codes := make([]domain.FindingCode, 0, len(carriers))
	for code := range carriers {
		codes = append(codes, code)
	}
	slices.Sort(codes)

	singles := 0
	for _, code := range codes {
		ids := carriers[code]
		slices.Sort(ids)
		if reason, broad := w4MultiCarrierCodes[code]; broad {
			if len(ids) < 2 {
				t.Errorf("%s is listed as a fleet-wide signal (%s) but fires on %d row(s); "+
					"if it now has a single witness, move it out of w4MultiCarrierCodes", code, reason, len(ids))
			}
			continue
		}
		singles++
		if len(ids) != 1 {
			t.Errorf("%s fires on %d demo rows, want exactly 1: %v", code, len(ids), ids)
		}
	}

	// 16, not 15: waf.orphan joined the bench with its own witness
	// (acme-unattached-waf) when the web-ACL orphan condition was split out
	// of waf.no-logging. The count moves with the witness, which is what this
	// pin exists to force.
	//
	// 17, not 16: iam-group.admin-attached left w4MultiCarrierCodes when
	// codex2 row 6 took PowerUserAccess out of the admin-equivalent set. It
	// now fires on the one group holding AdministratorAccess, so it is a
	// witness like any other.
	const wantWitnessed = 17
	if singles != wantWitnessed {
		t.Errorf("%d witness-backed wave-2 codes on the bench, want %d — a witness was added or lost "+
			"without this count moving with it; codes seen: %v", singles, wantWitnessed, codes)
	}
}

// w4MultiCarrierCodes are the batch's wave-2 codes that describe a fleet-wide
// posture rather than one planted fixture, so more than one demo row carries
// them by design. Everything else is a witness and must fire exactly once.
var w4MultiCarrierCodes = map[domain.FindingCode]string{
	"iam-role.dormant":         "every unused demo role is dormant; the signal is the account's shape, not one row",
	"kms.rotation-disabled":    "rotation is off on most demo keys, as it is on most real ones",
	"iam-group.orphan-or-noop": "an empty group and a no-op group are two different shapes of the same signal",
}

// TestW4AccessKeyRowsRenderKeySuffixAndDate pins how the unused-access-key
// finding renders its two supporting rows. The key is shown by its last four
// characters because an access key id is a credential the operator should not
// have to read in full to recognise, and the date answers "how long has this
// been true" without making them open the console.
func TestW4AccessKeyRowsRenderKeySuffixAndDate(t *testing.T) {
	rows, _ := w4bBench(t, "iam-user")

	const code domain.FindingCode = "iam-user.access-key-unused"
	var carrier *resource.Resource
	for i := range rows {
		if _, ok := rows[i].AttentionDetails[code]; ok {
			carrier = &rows[i]
			break
		}
	}
	if carrier == nil {
		t.Fatalf("no demo iam-user row carries %s, so the rendering below is unwitnessed", code)
	}

	got := map[string]string{}
	for _, r := range carrier.AttentionDetails[code].Rows {
		got[r.Label] = r.Value
	}

	key, ok := got["Key"]
	if !ok {
		t.Fatalf("%s renders no Key row: %+v", code, carrier.AttentionDetails[code].Rows)
	}
	if !strings.HasPrefix(key, "…") {
		t.Errorf("Key row = %q, want the last four characters behind a leading ellipsis so the full "+
			"credential is not printed", key)
	}
	if len([]rune(key)) != 5 {
		t.Errorf("Key row = %q (%d runes), want an ellipsis plus exactly four characters",
			key, len([]rune(key)))
	}

	lastUsed, ok := got["Last used"]
	if !ok {
		t.Fatalf("%s renders no Last used row: %+v", code, carrier.AttentionDetails[code].Rows)
	}
	if lastUsed == "" {
		t.Error("Last used row is empty; the operator cannot tell how long the key has been idle")
	}
}
