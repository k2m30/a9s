package unit

// qa_d4_verify_test.go — the rendered contracts for the rows that added a
// demo witness so a phrase, a drill, or a badge could be seen at all.
//
// Each of these was previously provable only in a unit test against a
// hand-built input. A phrase that merges several ports, a role reached by
// drilling rather than by listing, and a workgroup's two independent settings
// are all things the fixture set has to demonstrate before any surface test
// can watch them.

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
)

// d4FoldedRows drains a type's demo rows and folds its wave-2 result onto
// them, which is the row the list and detail views read.
func d4FoldedRows(t *testing.T, shortName string) ([]resource.Resource, resource.ResourceTypeDef) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	clients := demo.NewServiceClients()
	rows := DrainPages(t, shortName, func(token string) (resource.FetchResult, error) {
		return td.Fetcher(context.Background(), clients, token)
	})
	enricher, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok || enricher.Fn == nil {
		return rows, *td
	}
	cache := resource.ResourceCache{shortName: resource.ResourceCacheEntry{Resources: rows}}
	res, err := enricher.Fn(context.Background(), clients, rows, cache)
	if err != nil {
		t.Fatalf("%s wave-2: %v", shortName, err)
	}
	for i := range rows {
		a9sruntime.ApplyWave2ToRow(&rows[i], *td, res.Findings, res.AttentionDetails)
	}
	return rows, *td
}

func d4RowByID(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id || r.Name == id {
			return r
		}
	}
	t.Fatalf("no demo row named %q", id)
	return resource.Resource{}
}

func d4FindingByCode(t *testing.T, r resource.Resource, code domain.FindingCode) domain.Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Code == code {
			return f
		}
	}
	t.Fatalf("%s carries no %s; findings = %+v", r.ID, code, r.Findings)
	return domain.Finding{}
}

// ── Row 18 — the merged listener phrases ──────────────────────────────────

// TestD4Row18_CleartextPhraseMergesPortsInAscendingOrder pins the phrase a
// balancer with two offending listeners shows.
//
// The fixture supplies 8080 before 80, so a phrase that simply followed the
// API's order would read "ports 8080, 80". Ascending order is what makes two
// balancers with the same problem render the same text, which is what an
// operator scanning a list is actually comparing.
func TestD4Row18_CleartextPhraseMergesPortsInAscendingOrder(t *testing.T) {
	rows, td := d4FoldedRows(t, "elb")
	r := d4RowByID(t, rows, fixtures.ELBPlainHTTP)

	f := d4FindingByCode(t, r, "elb.plain-http-listener")
	if f.Phrase != "ports 80, 8080 in the clear" {
		t.Errorf("Status cell = %q, want %q", f.Phrase, "ports 80, 8080 in the clear")
	}
	// Warning, not Broken: cleartext on a listener is posture, and the
	// FindingDef at catalog_networking.go:231 declares the same.
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want Warn", f.Severity)
	}
	if f.Detail == "" {
		t.Error("the finding renders no Detail sentence")
	}

	// One finding for both listeners, not one per listener: two rows in the
	// Attention block saying the same thing is what merging removed.
	n := 0
	for _, got := range r.Findings {
		if got.Code == "elb.plain-http-listener" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d plain-listener findings on one balancer, want 1 merged", n)
	}

	if td.ResolveColor(r) != domain.ColorWarning {
		t.Errorf("row colour = %v, want Warning", td.ResolveColor(r))
	}
}

// TestD4Row18_WeakTLSPhraseMergesPortsInAscendingOrder is the same contract on
// the sibling rule, which merges independently and could regress alone.
func TestD4Row18_WeakTLSPhraseMergesPortsInAscendingOrder(t *testing.T) {
	rows, _ := d4FoldedRows(t, "elb")
	r := d4RowByID(t, rows, fixtures.ELBWeakTLS)

	f := d4FindingByCode(t, r, "elb.weak-tls-policy")
	if !strings.HasPrefix(f.Phrase, "weak TLS policy on ports ") {
		t.Fatalf("phrase = %q, want it to name several ports", f.Phrase)
	}
	ports := strings.TrimPrefix(f.Phrase, "weak TLS policy on ports ")
	got := strings.Split(ports, ", ")
	if len(got) < 2 {
		t.Fatalf("phrase names %d port(s) (%q); the merged form is only witnessed with two or more",
			len(got), f.Phrase)
	}
	if !slices.IsSorted(got) {
		t.Errorf("ports read %v, want ascending order", got)
	}
	if f.Detail == "" {
		t.Error("the finding renders no Detail sentence")
	}
}

// TestD4Row18_ListenerRowsLeadWithTheirPort pins the supporting rows. The
// phrase names the ports, so a row has to start from the port for the operator
// to line the two up; a row leading with the protocol makes them read twice.
func TestD4Row18_ListenerRowsLeadWithTheirPort(t *testing.T) {
	rows, _ := d4FoldedRows(t, "elb")
	for _, id := range []string{fixtures.ELBPlainHTTP, fixtures.ELBWeakTLS} {
		r := d4RowByID(t, rows, id)
		for code, ad := range r.AttentionDetails {
			for _, row := range ad.Rows {
				if row.Label != "Listener" {
					continue
				}
				port, _, found := strings.Cut(row.Value, "/")
				if !found || port == "" {
					t.Errorf("%s %s: Listener row = %q, want <port>/<protocol>", id, code, row.Value)
					continue
				}
				if strings.IndexFunc(port, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
					t.Errorf("%s %s: Listener row = %q, want the port first", id, code, row.Value)
				}
			}
		}
	}
}

// TestD4Row18_ELBBadgeCountsEachBalancerOnce pins the derivation behind the
// menu badge: merging several listeners into one finding must not merge two
// balancers, and must not count one balancer twice.
func TestD4Row18_ELBBadgeCountsEachBalancerOnce(t *testing.T) {
	rows, td := d4FoldedRows(t, "elb")
	issueRows := 0
	for _, r := range rows {
		if td.ResolveColor(r) == domain.ColorBroken || td.ResolveColor(r) == domain.ColorWarning {
			issueRows++
		}
	}
	if issueRows == 0 {
		t.Fatal("no demo balancer carries an issue, so the badge derivation is unwitnessed")
	}

	carriers := map[string]int{}
	for _, r := range rows {
		for _, f := range r.Findings {
			if f.Code == "elb.plain-http-listener" || f.Code == "elb.weak-tls-policy" {
				carriers[string(f.Code)+"/"+r.ID]++
			}
		}
	}
	for key, n := range carriers {
		if n != 1 {
			t.Errorf("%s appears %d times on one row, so the badge double-counts it", key, n)
		}
	}
}

// ── Row 19 — a role reached by drilling ───────────────────────────────────

// TestD4Row19_DrilledRoleCarriesTheSameFindingsAsTheListedRole pins both
// surfaces of the same role.
//
// The list path and the by-ID path are different fetchers, and only the list
// path used to read inline policies. A role that shows an escalation finding
// when listed and none when drilled into tells the operator the problem went
// away because they clicked on it.
func TestD4Row19_DrilledRoleCarriesTheSameFindingsAsTheListedRole(t *testing.T) {
	const code domain.FindingCode = "role.inline-privilege-escalation"
	clients := demo.NewServiceClients()

	listed := d4RowByID(t, DrainPages(t, "role", func(token string) (resource.FetchResult, error) {
		return awsclient.FetchIAMRolesPage(context.Background(), clients.IAM, token)
	}), fixtures.RoleInlinePrivEsc)
	listedFinding := d4FindingByCode(t, listed, code)

	// Through the catalog's own FetchByIDs, which is the path a drill takes.
	td := resource.FindResourceType("role")
	if td == nil || td.FetchByIDs == nil {
		t.Fatal("role has no FetchByIDs, so a drilled role cannot be fetched at all")
	}
	drilled, err := td.FetchByIDs(context.Background(), clients, []string{fixtures.RoleInlinePrivEsc})
	if err != nil {
		t.Fatalf("role FetchByIDs: %v", err)
	}
	if len(drilled) == 0 {
		t.Fatalf("drilling to %s resolved no role", fixtures.RoleInlinePrivEsc)
	}
	drilledFinding := d4FindingByCode(t, drilled[0], code)

	if drilledFinding.Phrase != listedFinding.Phrase {
		t.Errorf("drilled phrase = %q, listed = %q", drilledFinding.Phrase, listedFinding.Phrase)
	}
	if drilledFinding.Severity != listedFinding.Severity {
		t.Errorf("drilled severity = %v, listed = %v", drilledFinding.Severity, listedFinding.Severity)
	}
	if drilledFinding.Detail != listedFinding.Detail {
		t.Errorf("drilled Detail = %q, listed = %q", drilledFinding.Detail, listedFinding.Detail)
	}
}

// TestD4Row19_SomeDemoResourcePivotsToTheEscalatingRole pins that the drill is
// reachable at all. Without a resource whose related panel lands on that role,
// the parity above is a contract nothing in the demo exercises.
func TestD4Row19_SomeDemoResourcePivotsToTheEscalatingRole(t *testing.T) {
	clients := demo.NewServiceClients()
	var reached []string

	for _, td := range resource.AllResourceTypes() {
		var checker resource.RelatedChecker
		for _, rel := range td.Related {
			if rel.TargetType == "role" {
				checker = rel.Checker
			}
		}
		if checker == nil || td.Fetcher == nil {
			continue
		}
		rows, ok := DrainFixtures(t, td, clients)
		if !ok {
			continue
		}
		cache := resource.ResourceCache{td.ShortName: resource.ResourceCacheEntry{Resources: rows}}
		for _, r := range rows {
			if slices.Contains(checker(context.Background(), clients, r, cache).ResourceIDs(), fixtures.RoleInlinePrivEsc) {
				reached = append(reached, td.ShortName+"/"+r.ID)
			}
		}
	}

	if len(reached) == 0 {
		t.Errorf("no demo resource's related panel drills to %s, so the drilled-role surface has no witness",
			fixtures.RoleInlinePrivEsc)
	}
}

// ── Rows 21 and 22 — the athena workgroup ─────────────────────────────────

// TestD4Row22_AthenaEmitsTwoIndependentFindings pins that the two workgroup
// settings are two findings.
//
// They are independent: a workgroup that enforces its configuration can still
// write results in the clear. Merged, the phrase named whichever fired first
// and the other went unsaid, which is how a cell came to read as a row label
// with a count after it.
func TestD4Row22_AthenaEmitsTwoIndependentFindings(t *testing.T) {
	rows, td := d4FoldedRows(t, "athena")
	r := d4RowByID(t, rows, fixtures.AthenaGovernanceMisconfigured)

	enforced := d4FindingByCode(t, r, "athena.settings-not-enforced")
	if enforced.Phrase != "settings can be overridden per query" {
		t.Errorf("enforcement phrase = %q, want %q", enforced.Phrase, "settings can be overridden per query")
	}
	if enforced.Detail == "" {
		t.Error("the enforcement finding renders no Detail sentence")
	}
	if _, hasRows := r.AttentionDetails["athena.settings-not-enforced"]; hasRows {
		t.Errorf("the enforcement finding carries supporting rows: %+v",
			r.AttentionDetails["athena.settings-not-enforced"].Rows)
	}

	unencrypted := d4FindingByCode(t, r, "athena.results-unencrypted")
	if unencrypted.Phrase != "query results stored unencrypted" {
		t.Errorf("encryption phrase = %q, want %q", unencrypted.Phrase, "query results stored unencrypted")
	}
	if unencrypted.Detail == "" {
		t.Error("the encryption finding renders no Detail sentence")
	}
	ad, ok := r.AttentionDetails["athena.results-unencrypted"]
	if !ok || len(ad.Rows) != 1 {
		t.Fatalf("encryption rows = %+v, want one Results-written-to row", ad.Rows)
	}
	if ad.Rows[0].Label != "Results written to" {
		t.Errorf("row label = %q, want %q", ad.Rows[0].Label, "Results written to")
	}
	if !strings.HasPrefix(ad.Rows[0].Value, "s3://") {
		t.Errorf("row value = %q, want the S3 location the operator has to go look at", ad.Rows[0].Value)
	}

	// The retired code must not still be emitted anywhere.
	for _, row := range rows {
		for _, f := range row.Findings {
			if f.Code == "athena.governance-misconfigured" {
				t.Errorf("%s still carries the merged code %s", row.ID, f.Code)
			}
		}
	}

	if td.ResolveColor(r) == domain.ColorHealthy {
		t.Errorf("the witness colours healthy while carrying two findings")
	}
}

// TestD4Row21_AthenaWitnessIsTheOnlyCarrier pins the fixture side: each code
// fires on exactly one workgroup, so the badge moving is attributable and the
// row-values sweep has a row to look at.
func TestD4Row21_AthenaWitnessIsTheOnlyCarrier(t *testing.T) {
	rows, _ := d4FoldedRows(t, "athena")
	if len(rows) < 2 {
		t.Fatalf("%d demo workgroups; a single-carrier claim needs a healthy neighbour", len(rows))
	}

	for _, code := range []domain.FindingCode{"athena.settings-not-enforced", "athena.results-unencrypted"} {
		var carriers []string
		for _, r := range rows {
			for _, f := range r.Findings {
				if f.Code == code {
					carriers = append(carriers, r.ID)
				}
			}
		}
		if len(carriers) != 1 {
			t.Errorf("%s fires on %d workgroups, want exactly 1: %v", code, len(carriers), carriers)
		}
		if len(carriers) == 1 && carriers[0] != fixtures.AthenaGovernanceMisconfigured {
			t.Errorf("%s fires on %q, want the named witness %q",
				code, carriers[0], fixtures.AthenaGovernanceMisconfigured)
		}
	}
}

// TestD4Row22_AthenaDocQuotesTheDetailConstants pins the doc side of the same
// change: the §4 cells quote what the two findings now say. Task w27 deleted
// the hand-written §4 "Detail text (S5)" column this test used to parse
// (backtick-quoted last cell); the Detail column now lives in the generated
// findings table catalogen writes (five plain cells, no backticks), so this
// reads that table's row for each code instead.
func TestD4Row22_AthenaDocQuotesTheDetailConstants(t *testing.T) {
	rows, _ := d4FoldedRows(t, "athena")
	r := d4RowByID(t, rows, fixtures.AthenaGovernanceMisconfigured)

	generated := athenaGeneratedDocDetails(t)
	for _, code := range []domain.FindingCode{"athena.settings-not-enforced", "athena.results-unencrypted"} {
		f := d4FindingByCode(t, r, code)
		got, ok := generated[code]
		if !ok {
			t.Errorf("docs/resources/athena.md's generated findings table has no row for %s", code)
			continue
		}
		if got != f.Detail {
			t.Errorf("docs/resources/athena.md's generated Detail cell for %s = %q, want %q", code, got, f.Detail)
		}
	}
}

// athenaGeneratedDocDetails returns the generated findings table's Detail
// column, keyed by Code — the table catalogen writes between
// "<!-- BEGIN GENERATED: findings -->" and "<!-- END GENERATED: findings -->",
// five plain (non-backtick) cells: Code | Phrase | Severity | Source | Detail.
func athenaGeneratedDocDetails(t *testing.T) map[domain.FindingCode]string {
	t.Helper()
	b, err := os.ReadFile("../../docs/resources/athena.md")
	if err != nil {
		t.Fatalf("read athena.md: %v", err)
	}
	out := map[domain.FindingCode]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 5 {
			continue
		}
		code := strings.TrimSpace(cells[0])
		if code == "" || code == "Code" || strings.HasPrefix(code, "---") {
			continue
		}
		out[domain.FindingCode(code)] = strings.TrimSpace(cells[4])
	}
	return out
}

// TestD4Row25_AthenaDocRowsSitInTheWarningBucket pins the glyph surface of the
// two workgroup findings.
//
// Both codes are Warning and the witness renders yellow, so a §4 row claiming
// bucket Healthy on S3 promises a glyph on a green row that the operator will
// never see. §3.2 already said Warning, so the page disagreed with itself.
func TestD4Row25_AthenaDocRowsSitInTheWarningBucket(t *testing.T) {
	b, err := os.ReadFile("../../docs/resources/athena.md")
	if err != nil {
		t.Fatalf("read athena.md: %v", err)
	}
	want := map[string]bool{
		"EnforceWorkGroupConfiguration == false":             false,
		"ResultConfiguration.EncryptionConfiguration == nil": false,
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		for signal := range want {
			if !strings.Contains(line, signal) {
				continue
			}
			want[signal] = true
			cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
			if len(cells) < 5 {
				t.Errorf("§4 row for %s has %d cells: %s", signal, len(cells), strings.TrimSpace(line))
				continue
			}
			if bucket := strings.TrimSpace(cells[2]); bucket != "Warning" {
				t.Errorf("%s: state bucket = %q, want Warning — the code is SevWarn and the witness renders yellow",
					signal, bucket)
			}
			if surfaces := strings.TrimSpace(cells[4]); surfaces != "S2, S4, S5" {
				t.Errorf("%s: surfaces = %q, want %q", signal, surfaces, "S2, S4, S5")
			}
		}
	}
	for signal, seen := range want {
		if !seen {
			t.Errorf("athena.md §4 has no row for %s", signal)
		}
	}
}

// TestD4Row26_ListenerPhrasesAgreeInNumber pins that both merged phrases and
// their Detail sentences count.
//
// The phrase pluralised unconditionally, so a balancer with one offending
// listener read "ports 443 in the clear" while its Detail said "This listener".
// One is a grammar slip the operator reads as a second port they cannot find.
func TestD4Row26_ListenerPhrasesAgreeInNumber(t *testing.T) {
	rows, _ := d4FoldedRows(t, "elb")

	cases := []struct {
		id       string
		code     domain.FindingCode
		singular bool
	}{
		{fixtures.ELBPlainTCPListener, "elb.plain-http-listener", true},
		{fixtures.ELBPlainHTTP, "elb.plain-http-listener", false},
		{fixtures.ELBWeakTLS, "elb.weak-tls-policy", false},
	}

	sawSingular, sawPlural := false, false
	for _, c := range cases {
		r := d4RowByID(t, rows, c.id)
		f := d4FindingByCode(t, r, c.code)

		wantNoun, otherNoun := "ports ", "port "
		if c.singular {
			wantNoun, otherNoun = "port ", "ports "
			sawSingular = true
		} else {
			sawPlural = true
		}
		if !strings.Contains(f.Phrase, wantNoun) || strings.Contains(f.Phrase, otherNoun) {
			t.Errorf("%s %s: phrase = %q, want it to say %q", c.id, c.code, f.Phrase, strings.TrimSpace(wantNoun))
		}

		// Task w27: Detail is now one static sentence per code, worded to
		// cover one-or-many listeners without pluralising ("a listener...
		// the ports are listed below"), so it no longer needs to agree in
		// number with the phrase — the actual listener(s) live in their own
		// rows instead.
		wantDetail := map[domain.FindingCode]string{
			"elb.plain-http-listener": "A listener on this load balancer carries traffic in the clear, so credentials and session cookies cross the network readable by anyone on the path; the ports are listed below. Terminate TLS on the listener, or redirect it to an HTTPS listener.",
			"elb.weak-tls-policy":     "A listener's security policy still negotiates older protocol versions or ciphers without forward secrecy, so a client can be steered onto a breakable connection; the ports are listed below. Move the listener to one of the modern security policies that require version 1.2 or later.",
		}[c.code]
		if f.Detail != wantDetail {
			t.Errorf("%s %s: Detail = %q, want %q", c.id, c.code, f.Detail, wantDetail)
		}
		rowLabel := map[domain.FindingCode]string{
			"elb.plain-http-listener": "Listener",
			"elb.weak-tls-policy":     "Security policy",
		}[c.code]
		ad, ok := r.AttentionDetails[c.code]
		if !ok {
			t.Fatalf("%s %s: AttentionDetails not found", c.id, c.code)
		}
		rowCount := 0
		for _, row := range ad.Rows {
			if row.Label == rowLabel {
				rowCount++
			}
		}
		wantRows := 1
		if !c.singular {
			wantRows = 2
		}
		if rowCount != wantRows {
			t.Errorf("%s %s: %d %q row(s), want %d (one per listener)", c.id, c.code, rowCount, rowLabel, wantRows)
		}
	}

	if !sawSingular || !sawPlural {
		t.Fatal("the bench does not witness both counts, so this pins only one of them")
	}
}
