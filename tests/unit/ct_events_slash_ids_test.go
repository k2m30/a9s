package unit_test

// ct_events_slash_ids_test.go — row 8: a slash inside a resource id is part of
// the name, not a path to trim.
//
// A CloudTrail event names its resource in one of two shapes: an ARN, whose id
// is the segment after the last slash ("...:key/1234abcd"), and a plain name,
// which may itself contain slashes ("prod/api/stripe-key"). Trimming to the
// last segment is right for the first and destroys the second, and nothing in
// the event says which shape it carries. The pins below hold both forms
// resolvable through the same matcher, and hold the count to the one resource
// the event actually named when the account happens to hold both forms.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// ctSlashPivots is the subset of ctPivots that reads its ids out of the
// Resources envelope through the shared extractCTResourceIDs. vpce reads the
// event JSON instead; dbi and cfn keep their own copies of the trim rule, and
// neither an RDS instance identifier nor a CloudFormation stack name may
// contain a slash, so no input distinguishes those copies here.
func ctSlashPivots() []ctPivot {
	shared := map[string]bool{"ec2": true, "s3": true, "lambda": true, "kms": true,
		"secrets": true, "sg": true, "ddb": true, "trail": true}
	var out []ctPivot
	for _, p := range ctPivots() {
		if shared[p.target] {
			out = append(out, p)
		}
	}
	if len(out) != len(shared) {
		panic("ctSlashPivots: table drifted from ctPivots")
	}
	return out
}

// ctSlashEvent builds an event whose Resources envelope names exactly one
// resource of the pivot's AWS type, under the given name.
func ctSlashEvent(p ctPivot, name string) resource.Resource {
	return resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7b8",
		Name: "GetSecretValue",
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-0a1b2c3d4e5f6a7b8"),
			EventName: aws.String("GetSecretValue"),
			Resources: []cloudtrailtypes.Resource{{
				ResourceType: aws.String(p.awsType),
				ResourceName: aws.String(name),
			}},
			CloudTrailEvent: aws.String(`{"requestParameters":{}}`),
		},
	}
}

// TestCtEventsPivots_PathNamedIDResolvesAsWritten pins the shape the fix is
// for: the event names "prod/api/<id>" and the account holds a resource under
// that whole name. The list confirms it, so the pivot resolves to it — the
// trimmed tail names nothing and must not turn the answer into a zero.
func TestCtEventsPivots_PathNamedIDResolvesAsWritten(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			pathName := "prod/api/" + p.id
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: pathName, Name: pathName}},
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, ctSlashEvent(p, pathName), cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the list is complete", got)
			}
			if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != pathName {
				t.Errorf("ResourceIDs = %v, want [%s]: the slash is part of the name", result.ResourceIDs(), pathName)
			}
		})
	}
}

// TestCtEventsPivots_ARNShapedNameResolvesByLastSegment is the counterpart
// that stops the fix from being "never trim": an event that names its resource
// as a path ("instance/i-0abc") is resolved by the segment after the slash,
// which is the id the list is keyed by.
func TestCtEventsPivots_ARNShapedNameResolvesByLastSegment(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: p.id, Name: p.id}},
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, ctSlashEvent(p, "resource/"+p.id), cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the list is complete", got)
			}
			if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != p.id {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), p.id)
			}
		})
	}
}

// TestCtEventsPivots_PathNamedIDDoesNotOfferTheTail pins the count when the
// account holds BOTH forms: a secret named "prod/api/stripe-key" and a
// separate one named "stripe-key". The event named the first. Offering the
// second is the row-6 defect on another axis — a row the event never
// established — so the answer is one resource, the one that was named.
func TestCtEventsPivots_PathNamedIDDoesNotOfferTheTail(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			pathName := "prod/api/" + p.id
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{
					{ID: pathName, Name: pathName},
					{ID: p.id, Name: p.id},
				},
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil, ctSlashEvent(p, pathName), cache)

			if result.Count() != 1 {
				t.Fatalf("Count = %d, want 1: the event named %q only; IDs = %v",
					result.Count(), pathName, result.ResourceIDs())
			}
			if result.ResourceIDs()[0] != pathName {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), pathName)
			}
		})
	}
}

// TestCtEventsSecrets_DemoBenchPathNamedWitness is the bench witness. The demo
// account holds a secret whose name carries slashes, and an event that reads
// it; both that event and a plain-named sibling must resolve to their one
// secret, so a fix cannot trade one shape for the other.
func TestCtEventsSecrets_DemoBenchPathNamedWitness(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	events := byType["ct-events"]
	secrets := byType["secrets"]
	if len(events) == 0 || len(secrets) == 0 {
		t.Fatal("demo ct-events or secrets fixtures missing — the witness cannot be seen")
	}
	cache := resource.ResourceCache{"secrets": resource.ResourceCacheEntry{Resources: secrets}}
	checker := ctEventsCheckerByTarget(t, "secrets")

	byID := make(map[string]resource.Resource, len(events))
	for _, e := range events {
		byID[e.ID] = e
	}

	for _, tc := range []struct {
		id   string
		what string
		want string
	}{
		{demofixtures.CtEventPathNamedSecret, "a secret named as a path", "prod/api/stripe-key"},
		{"evt-secrets-prod-dbi-rotate-001", "a secret named without one", "rds!db-prod-dbi-1-ABCDEF"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			event, ok := byID[tc.id]
			if !ok {
				t.Fatalf("demo fixture %s missing", tc.id)
			}
			result := checker(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the demo secrets list is complete", got)
			}
			if result.Count() != 1 || result.ResourceIDs()[0] != tc.want {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), tc.want)
			}
		})
	}
}

// TestCTEventTargetRow_NavIDNamesTheResource pins the left column's half of the
// same rule. The TARGET row's navigation id is the resource id stripped of its
// TYPE prefix, which is what precedes the FIRST separator — cutting at the last
// one turned "secret:prod/api/stripe-key" into "stripe-key", which navigates to
// nothing. Where the value carries no prefix the row falls back to Value, so the
// assertion is on the id the navigation layer actually uses.
func TestCTEventTargetRow_NavIDNamesTheResource(t *testing.T) {
	const acct = "123456789012"
	for _, tc := range []struct {
		what   string
		arn    string
		typ    string
		target string
		want   string
	}{
		{
			what:   "an instance ARN keeps its id, not its type prefix",
			arn:    "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def4567890",
			typ:    "AWS::EC2::Instance",
			target: "ec2",
			want:   "i-0abc123def4567890",
		},
		{
			what:   "a path-named secret keeps every segment of its name",
			arn:    "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key",
			typ:    "AWS::SecretsManager::Secret",
			target: "secrets",
			want:   "prod/api/stripe-key",
		},
		{
			what:   "a bucket ARN carries no prefix and falls back to its value",
			arn:    "arn:aws:s3:::acme-logs-archive",
			typ:    "AWS::S3::Bucket",
			target: "s3",
			want:   "acme-logs-archive",
		},
		{
			what:   "a key ARN keeps the uuid",
			arn:    "arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab",
			typ:    "AWS::KMS::Key",
			target: "kms",
			want:   "1234abcd-12ab-34cd-56ef-1234567890ab",
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			rows, _ := ctevent.ExtractTarget("GetSecretValue", "secretsmanager.amazonaws.com", acct,
				[]ctevent.ResourceRef{{ARN: tc.arn, AccountID: acct, Type: tc.typ}}, nil)
			if len(rows) != 1 {
				t.Fatalf("rows = %d, want 1", len(rows))
			}
			row := rows[0]
			if !row.IsNavigable || row.TargetType != tc.target {
				t.Fatalf("row navigable=%v target=%q, want true/%q", row.IsNavigable, row.TargetType, tc.target)
			}
			navID := row.NavID
			if navID == "" {
				navID = row.Value
			}
			if navID != tc.want {
				t.Errorf("navigation id = %q, want %q (Value=%q NavID=%q)", navID, tc.want, row.Value, row.NavID)
			}
		})
	}
}

// TestCTEventTargetRows_DemoNavIDsAreResolvable is the class check for the left
// column: every navigable TARGET row the demo events produce must point at a
// resource the demo account holds. CtEventDeletedBucket is the one exception —
// its bucket is gone by design, which is the row-6 witness.
func TestCTEventTargetRows_DemoNavIDsAreResolvable(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	events := byType["ct-events"]
	if len(events) == 0 {
		t.Fatal("no demo ct-events fixtures")
	}

	for _, e := range events {
		if e.ID == demofixtures.CtEventDeletedBucket {
			continue
		}
		ev, ok := e.RawStruct.(cloudtrailtypes.Event)
		if !ok || ev.CloudTrailEvent == nil {
			continue
		}
		parsed, err := ctevent.Parse(*ev.CloudTrailEvent)
		if err != nil {
			t.Errorf("%s: parse: %v", e.ID, err)
			continue
		}
		rows, _ := ctevent.ExtractTarget(parsed.EventName, parsed.EventSource, parsed.RecipientAccountID,
			parsed.Resources, nil)
		for _, row := range rows {
			if !row.IsNavigable || row.TargetType == "" {
				continue
			}
			navID := row.NavID
			if navID == "" {
				navID = row.Value
			}
			if idx := strings.Index(navID, "|"); idx >= 0 {
				navID = navID[:idx]
			}
			pool := byType[row.TargetType]
			if len(pool) == 0 {
				continue
			}
			found := false
			for _, r := range pool {
				if r.ID == navID || r.Name == navID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: TARGET row %s=%q navigates to %q, which no demo %s fixture carries",
					e.ID, row.Key, row.Value, navID, row.TargetType)
			}
		}
	}
}
