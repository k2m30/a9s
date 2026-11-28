package unit_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// t576Bench is one type's demo rows and the cache entries its enricher reads.
func t576Bench(t *testing.T, shortName string) (resource.ResourceTypeDef, []resource.Resource, resource.ResourceCache) {
	t.Helper()
	byType, cache := buildVisibilityTypeCache(t)
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	rows := byType[shortName]
	if len(rows) == 0 {
		t.Fatalf("%s: no demo rows", shortName)
	}
	return *td, rows, t576ReadsOf(cache, t576Enricher(t, shortName).Reads)
}

func t576Enricher(t *testing.T, shortName string) awsclient.IssueEnricher {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok {
		t.Fatalf("%s has no list enricher", shortName)
	}
	return e
}

func t576EnrichType(t *testing.T, shortName string, rows []resource.Resource, cache resource.ResourceCache, fail func(service, op string, in any) error) t576Run {
	t.Helper()
	return t576Enrich(t, t576Enricher(t, shortName).Fn, rows, cache, fail)
}

func t576HasCode(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

// t576RenderList loads rows into a list screen and delivers run's result the
// way the runtime does, returning the rendered list body.
func t576RenderList(t *testing.T, shortName string, rows []resource.Resource, run t576Run) app.ListBody {
	t.Helper()
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	c.ApplyResourcesLoaded(shortName, rows, nil, false)
	core.ObserveRows(shortName, rows, nil, session.OriginFetch, false)
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType:     shortName,
		Findings:         run.res.Findings,
		AttentionDetails: run.res.AttentionDetails,
		FieldUpdates:     run.res.FieldUpdates,
		TruncatedIDs:     run.res.TruncatedIDs,
		Err:              run.err,
	})
	c.ApplyIntents(intents)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", shortName)
	}
	return *body
}

// A repository whose packages could not be listed has an unknown package
// count. Its row says so, and the refusal reaches the caller; an empty
// Packages cell next to a clean status reads as a repository nobody checked
// being fine.
func TestT576_CodeArtifactDeniedListPackagesMarksTheRepository(t *testing.T) {
	_, rows, cache := t576Bench(t, "codeartifact")

	clean := t576EnrichType(t, "codeartifact", rows, cache, nil)
	for _, r := range rows {
		if check, ok := clean.res.TruncatedIDs[r.ID]; ok {
			t.Errorf("clean run: %s marked not inspected (%q)", r.ID, check)
		}
		if clean.res.FieldUpdates[r.ID]["package_count"] == "" {
			t.Errorf("clean run: %s has no package_count", r.ID)
		}
	}

	got := t576EnrichType(t, "codeartifact", rows, cache, t576DenyOp("ListPackages"))
	if got.err == nil {
		t.Error("ListPackages refused for every repository, enricher returned no error")
	}
	for _, r := range rows {
		if check := got.res.TruncatedIDs[r.ID]; check != "ListPackages" {
			t.Errorf("%s: not-inspected check = %q, want %q", r.ID, check, "ListPackages")
		}
	}

	body := t576RenderList(t, "codeartifact", rows, got)
	for _, r := range rows {
		if domain.StatusPhrase(got.res.Findings[r.ID]) != "" {
			continue
		}
		if cell := statusCellFor(t, body, r.ID); cell != domain.NotInspectedPhrase {
			t.Errorf("%s Status cell = %q, want %q", r.ID, cell, domain.NotInspectedPhrase)
		}
	}
}

// A log group whose newest stream could not be read has an unknown last
// event — whether the read was refused or throttled past every retry.
func TestT576_LogsFailedLastEventReadMarksTheGroup(t *testing.T) {
	_, rows, cache := t576Bench(t, "logs")
	clean := t576EnrichType(t, "logs", rows, cache, nil)
	var read []string
	for _, r := range rows {
		if clean.res.FieldUpdates[r.ID]["last_event_at"] != "" {
			read = append(read, r.ID)
			if check, ok := clean.res.TruncatedIDs[r.ID]; ok {
				t.Errorf("clean run: %s marked not inspected (%q)", r.ID, check)
			}
		}
	}
	if len(read) == 0 {
		t.Fatal("clean run set last_event_at on no log group; the bench has no witness")
	}

	for name, fail := range map[string]func(string, string, any) error{
		"denied":               t576DenyOp("DescribeLogStreams"),
		"throttled past retry": t576ThrottleAlways("DescribeLogStreams"),
	} {
		t.Run(name, func(t *testing.T) {
			got := t576EnrichType(t, "logs", rows, cache, fail)
			if got.err == nil {
				t.Error("DescribeLogStreams failed for every group, enricher returned no error")
			}
			for _, id := range read {
				if check := got.res.TruncatedIDs[id]; check != "DescribeLogStreams" {
					t.Errorf("%s: last event unread, not-inspected check = %q, want %q", id, check, "DescribeLogStreams")
				}
			}
		})
	}
}

// t576TrailLoad is the trail list as loaded: the fetch and the list enricher,
// both through clients refusing what fail refuses.
type t576TrailLoad struct {
	rows     []resource.Resource
	fetchErr error
	run      t576Run
}

func t576LoadTrails(t *testing.T, fail func(service, op string, in any) error) t576TrailLoad {
	t.Helper()
	td := resource.FindResourceType("trail")
	if td == nil || td.Fetcher == nil {
		t.Fatal("trail has no fetcher")
	}
	clients := demo.NewServiceClients()
	inj := &t576Injector{fail: fail}
	t576WrapClients(clients, inj)
	page, fetchErr := td.Fetcher(context.Background(), clients, "")
	rows := page.Resources
	if len(rows) == 0 {
		t.Fatalf("trail fetch returned no rows (err %v)", fetchErr)
	}
	e := t576Enricher(t, "trail")
	_, cache := buildVisibilityTypeCache(t)
	res, err := e.Fn(context.Background(), clients, rows, t576ReadsOf(cache, e.Reads))
	return t576TrailLoad{rows: rows, fetchErr: fetchErr, run: t576Run{res: res, err: err, calls: inj.calls}}
}

// GetTrailStatus is the only source of a trail's logging health. A failed
// read leaves the trail unknown, which the row says, never a trail that
// reads as logging fine.
func TestT576_TrailStatusFailureMarksTheTrail(t *testing.T) {
	for name, fail := range map[string]func(string, string, any) error{
		"denied":               t576DenyOp("GetTrailStatus"),
		"throttled past retry": t576ThrottleAlways("GetTrailStatus"),
	} {
		t.Run(name, func(t *testing.T) {
			got := t576LoadTrails(t, fail)
			if got.fetchErr == nil && got.run.err == nil {
				t.Error("GetTrailStatus failed for every trail, the load recorded no failure")
			}
			for _, r := range got.rows {
				if r.Fields["trail_arn"] == "" {
					continue
				}
				if check := got.run.res.TruncatedIDs[r.ID]; check != "GetTrailStatus" {
					t.Errorf("%s: logging health unread, not-inspected check = %q, want %q", r.ID, check, "GetTrailStatus")
				}
			}
		})
	}
}

// A group's membership walk and its policy walks answer different questions:
// a refused inline walk leaves membership counted, but the administrator
// verdict open, since an unread inline policy may hold the Deny that decides.
func TestT576_IAMGroupDeniedInlineWalkKeepsMembersAndWithholdsAdmin(t *testing.T) {
	_, rows, cache := t576Bench(t, "iam-group")
	clean := t576EnrichType(t, "iam-group", rows, cache, nil)
	if !t576HasCode(clean.res.Findings[fixtures.IAMGroupAdminAttached], "iam-group.admin-attached") {
		t.Fatalf("clean run: %s has no admin finding; the bench has no witness", fixtures.IAMGroupAdminAttached)
	}

	got := t576EnrichType(t, "iam-group", rows, cache, t576DenyOp("ListGroupPolicies"))
	if got.err == nil {
		t.Error("ListGroupPolicies refused for every group, enricher returned no error")
	}
	for _, r := range rows {
		want := clean.res.FieldUpdates[r.ID]["member_count"]
		if v := got.res.FieldUpdates[r.ID]["member_count"]; v != want || v == "" {
			t.Errorf("%s: member_count = %q, want %q (GetGroup answered)", r.ID, v, want)
		}
		if check := got.res.TruncatedIDs[r.ID]; check != "ListGroupPolicies" {
			t.Errorf("%s: not-inspected check = %q, want %q", r.ID, check, "ListGroupPolicies")
		}
	}
	if t576HasCode(got.res.Findings[fixtures.IAMGroupAdminAttached], "iam-group.admin-attached") {
		t.Errorf("%s: admin finding stands although its inline policies were never read; findings %v", fixtures.IAMGroupAdminAttached, got.res.Findings[fixtures.IAMGroupAdminAttached])
	}
}

// The SES account read is one account-wide answer; DKIM is one answer per
// identity. A refused account read leaves the account signal open on every
// identity and takes no identity's DKIM verdict with it.
func TestT576_SESAccountFailureKeepsIdentityDKIM(t *testing.T) {
	_, rows, cache := t576Bench(t, "ses")
	clean := t576EnrichType(t, "ses", rows, cache, nil)
	var dkimOff []string
	for _, r := range rows {
		if t576HasCode(clean.res.Findings[r.ID], "ses.dkim-off") {
			dkimOff = append(dkimOff, r.ID)
		}
	}
	if len(dkimOff) == 0 {
		t.Fatal("clean run: no identity reports DKIM off; the bench has no witness")
	}

	t.Run("denied", func(t *testing.T) {
		got := t576EnrichType(t, "ses", rows, cache, t576DenyOp("GetAccount"))
		if got.err == nil {
			t.Error("GetAccount refused, enricher returned no error")
		}
		for _, r := range rows {
			if check := got.res.TruncatedIDs[r.ID]; check != "GetAccount" {
				t.Errorf("%s: account signal unread, not-inspected check = %q, want %q", r.ID, check, "GetAccount")
			}
		}
		for _, id := range dkimOff {
			if !t576HasCode(got.res.Findings[id], "ses.dkim-off") {
				t.Errorf("%s: DKIM-off finding lost when only GetAccount was refused", id)
			}
		}
	})
}

// A topic's access policy comes from GetTopicAttributes, not from its
// subscriptions. A refused subscription walk leaves the subscription signal
// open and the policy verdict standing.
func TestT576_SNSDeniedSubscriptionsKeepsPublicPolicy(t *testing.T) {
	td, rows, cache := t576Bench(t, "sns")
	clean := t576EnrichType(t, "sns", rows, cache, nil)
	public := map[string]bool{}
	for _, r := range rows {
		if t576HasCode(clean.res.Findings[r.ID], "sns.public-policy") {
			public[r.ID] = true
		}
	}
	if len(public) == 0 || len(public) == len(rows) {
		t.Fatalf("clean run: %d of %d topics public; the bench needs both a public and a private topic", len(public), len(rows))
	}

	got := t576EnrichType(t, "sns", rows, cache, t576DenyOp("ListSubscriptionsByTopic"))
	if got.err == nil {
		t.Error("ListSubscriptionsByTopic refused for every topic, enricher returned no error")
	}
	var witness resource.Resource
	for _, r := range rows {
		if check := got.res.TruncatedIDs[r.ID]; check != "ListSubscriptionsByTopic" {
			t.Errorf("%s: subscription signal unread, not-inspected check = %q, want %q", r.ID, check, "ListSubscriptionsByTopic")
		}
		has := t576HasCode(got.res.Findings[r.ID], "sns.public-policy")
		switch {
		case public[r.ID] && !has:
			t.Errorf("%s: public-policy finding lost when only the subscription walk was refused", r.ID)
		case !public[r.ID] && has:
			t.Errorf("%s: private topic gained a public-policy finding", r.ID)
		}
		if t576HasCode(clean.res.Findings[r.ID], "sns.no-kms") && !t576HasCode(got.res.Findings[r.ID], "sns.no-kms") {
			t.Errorf("%s: no-kms finding lost when only the subscription walk was refused", r.ID)
		}
		if public[r.ID] && has {
			witness = r
		}
	}
	if witness.ID == "" {
		return
	}

	folded := make([]resource.Resource, len(rows))
	copy(folded, rows)
	for i := range folded {
		runtime.ApplyWave2ToRow(&folded[i], td, got.res.Findings, got.res.AttentionDetails)
		if folded[i].ID == witness.ID {
			witness = folded[i]
		}
	}
	if len(witness.Findings) < 2 {
		t.Fatalf("%s carries %d findings; a witness with one finding cannot falsify the top-finding selector", witness.ID, len(witness.Findings))
	}
	if top := t563AssertWitnessRendered(t, td, folded, witness); top.Code != "sns.public-policy" {
		t.Errorf("%s: top finding %s, want sns.public-policy", witness.ID, top.Code)
	}
}

// An empty backup-plan list read to the end proves no plan selects any
// volume. Whether the volume's ARN can be built does not change that: there
// is no selection for it to be matched against.
func TestT576_EBSEmptyCompleteBackupListReportsUncoveredWithoutARN(t *testing.T) {
	_, rows, cache := t576Bench(t, "ebs")
	fn := t576Enricher(t, "ebs").Fn

	run := func(t *testing.T, account string, backup resource.ResourceCacheEntry) awsclient.IssueEnricherResult {
		t.Helper()
		clients := demo.NewServiceClients()
		store := session.NewIdentityStore()
		if account == "" {
			store.Set("", errors.New("sts:GetCallerIdentity denied"))
		} else {
			store.Set(account, nil)
		}
		clients.SetIdentityStore(store)
		c := resource.ResourceCache{}
		for k, v := range cache {
			c[k] = v
		}
		c["backup"] = backup
		res, err := fn(context.Background(), clients, rows, c)
		if err != nil {
			t.Fatalf("ebs enricher: %v", err)
		}
		return res
	}

	for _, account := range []string{"", "123456789012"} {
		t.Run("account "+account, func(t *testing.T) {
			res := run(t, account, resource.ResourceCacheEntry{IsTruncated: false})
			for _, r := range rows {
				if !t576HasCode(res.Findings[r.ID], awsclient.CodeEBSNotInBackupPlan) {
					t.Errorf("%s: no %s under a complete, empty backup-plan list (checks %q)", r.ID, awsclient.CodeEBSNotInBackupPlan, res.TruncatedIDs[r.ID])
				}
			}

			cut := run(t, account, resource.ResourceCacheEntry{IsTruncated: true})
			for _, r := range rows {
				if t576HasCode(cut.Findings[r.ID], awsclient.CodeEBSNotInBackupPlan) {
					t.Errorf("%s: %s raised from a backup-plan list cut short", r.ID, awsclient.CodeEBSNotInBackupPlan)
				}
			}
		})
	}
}
