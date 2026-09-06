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
	"slices"
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

// ---------------------------------------------------------------------------
// Row 9 — the two id forms are alternatives, and one rule serves every pivot.
// ---------------------------------------------------------------------------

// ctSlashEventNaming builds an event whose Resources envelope names each of the
// given resources, all of the pivot's AWS type.
func ctSlashEventNaming(p ctPivot, names ...string) resource.Resource {
	var refs []cloudtrailtypes.Resource
	for _, n := range names {
		refs = append(refs, cloudtrailtypes.Resource{
			ResourceType: aws.String(p.awsType),
			ResourceName: aws.String(n),
		})
	}
	return resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7b8",
		Name: "GetSecretValue",
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("evt-0a1b2c3d4e5f6a7b8"),
			EventName:       aws.String("GetSecretValue"),
			Resources:       refs,
			CloudTrailEvent: aws.String(`{"requestParameters":{}}`),
		},
	}
}

// TestCtEventsPivots_OneRowPerResourceNamed pins the counting rule now that a
// candidate group stands for one event resource. Two entries that resolve to
// the same resource are one row — the event touched it twice, the account holds
// it once — and two entries naming different resources stay two, which is the
// half a de-duplicating fix would quietly eat.
func TestCtEventsPivots_OneRowPerResourceNamed(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			other := "second-" + p.id
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{
					{ID: p.id, Name: p.id},
					{ID: other, Name: other},
				},
			}}
			checker := ctEventsCheckerByTarget(t, p.target)

			t.Run("the same resource named twice is one row", func(t *testing.T) {
				event := ctSlashEventNaming(p, "resource/"+p.id, p.id)
				result := checker(context.Background(), nil, event, cache)
				if result.Count() != 1 || result.ResourceIDs()[0] != p.id {
					t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), p.id)
				}
			})

			t.Run("two resources named are two rows", func(t *testing.T) {
				event := ctSlashEventNaming(p, p.id, other)
				result := checker(context.Background(), nil, event, cache)
				if result.Count() != 2 {
					t.Errorf("Count = %d, want 2: the event named both; IDs = %v",
						result.Count(), result.ResourceIDs())
				}
			})
		})
	}
}

// TestCtEventsPivots_TruncatedListConfirmsTheTail pins the group rule against a
// partial list: the page that was read confirms the trimmed form and nothing
// else, so that one id is real and the truncated flag rides along.
func TestCtEventsPivots_TruncatedListConfirmsTheTail(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources:   []resource.Resource{{ID: p.id, Name: p.id}},
				IsTruncated: true,
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil,
				ctSlashEvent(p, "resource/"+p.id), cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the page read confirmed %q", got, p.id)
			}
			if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != p.id {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), p.id)
			}
			if !result.Truncated() {
				t.Error("Truncated = false, want true: a later page may name more")
			}
		})
	}
}

// TestCtEventsPivots_ListIDOutranksAnotherResourceName pins the precedence the
// matcher's lookup map has to keep. When one resource's Name is another's ID,
// the candidate names the resource whose ID it is — a list is keyed by id, and
// a name is only a fallback for resources whose id is not what the event wrote.
func TestCtEventsPivots_ListIDOutranksAnotherResourceName(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			// "decoy" carries p.id as its NAME; the real one carries it as its ID.
			for _, order := range []struct {
				what string
				list []resource.Resource
			}{
				{"decoy first", []resource.Resource{{ID: "decoy-" + p.id, Name: p.id}, {ID: p.id, Name: "real"}}},
				{"decoy last", []resource.Resource{{ID: p.id, Name: "real"}, {ID: "decoy-" + p.id, Name: p.id}}},
			} {
				t.Run(order.what, func(t *testing.T) {
					cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{Resources: order.list}}
					result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil,
						ctSlashEvent(p, p.id), cache)

					if result.Count() != 1 || result.ResourceIDs()[0] != p.id {
						t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), p.id)
					}
				})
			}
		})
	}
}

// TestCtEventsDBI_ThroughTheSharedExtractor pins the RDS pivot after its own
// copy of the rule was deleted: the envelope still resolves in both id shapes,
// the request-body fallback still runs when the envelope names no instance, and
// a cluster entry is still not an instance id.
func TestCtEventsDBI_ThroughTheSharedExtractor(t *testing.T) {
	const dbID = "acme-orders-prod"
	cache := resource.ResourceCache{"dbi": resource.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: dbID, Name: dbID}},
	}}
	checker := ctEventsCheckerByTarget(t, "dbi")

	dbiEvent := func(refs []cloudtrailtypes.Resource, body string) resource.Resource {
		return resource.Resource{
			ID:   "evt-0a1b2c3d4e5f6a7b9",
			Name: "ModifyDBInstance",
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f6a7b9"),
				EventName:       aws.String("ModifyDBInstance"),
				Resources:       refs,
				CloudTrailEvent: aws.String(body),
			},
		}
	}
	instanceRef := func(name string) []cloudtrailtypes.Resource {
		return []cloudtrailtypes.Resource{{
			ResourceType: aws.String("AWS::RDS::DBInstance"),
			ResourceName: aws.String(name),
		}}
	}

	for _, tc := range []struct {
		what  string
		event resource.Resource
		want  int
	}{
		{
			what:  "the envelope names the instance outright",
			event: dbiEvent(instanceRef(dbID), `{"requestParameters":{}}`),
			want:  1,
		},
		{
			what:  "the envelope names it as a path",
			event: dbiEvent(instanceRef("db/"+dbID), `{"requestParameters":{}}`),
			want:  1,
		},
		{
			what:  "the request body names it when the envelope does not",
			event: dbiEvent(nil, `{"requestParameters":{"dBInstanceIdentifier":"`+dbID+`"}}`),
			want:  1,
		},
		{
			what: "a cluster entry is not an instance id",
			event: dbiEvent([]cloudtrailtypes.Resource{{
				ResourceType: aws.String("AWS::RDS::DBCluster"),
				ResourceName: aws.String(dbID),
			}}, `{"requestParameters":{}}`),
			want: 0,
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			result := checker(context.Background(), nil, tc.event, cache)
			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the dbi list is complete", got)
			}
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d; IDs = %v", result.Count(), tc.want, result.ResourceIDs())
			}
		})
	}
}

// TestCtEventsCFN_StackARNResolvesToTheName pins the CloudFormation pivot after
// its own copy was deleted. A stack ARN is ".../stack/<name>/<uuid>", so the
// generic last-segment candidate would be the uuid; the stack is keyed by name,
// and an account that happens to hold a stack named like the uuid must not be
// offered in its place.
func TestCtEventsCFN_StackARNResolvesToTheName(t *testing.T) {
	const (
		stackName = "acme-network"
		stackUUID = "12345678-1234-1234-1234-123456789012"
	)
	cache := resource.ResourceCache{"cfn": resource.ResourceCacheEntry{
		Resources: []resource.Resource{
			{ID: stackName, Name: stackName},
			{ID: stackUUID, Name: stackUUID},
		},
	}}
	event := resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7ba",
		Name: "UpdateStack",
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-0a1b2c3d4e5f6a7ba"),
			EventName: aws.String("UpdateStack"),
			Resources: []cloudtrailtypes.Resource{{
				ResourceType: aws.String("AWS::CloudFormation::Stack"),
				ResourceName: aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" +
					stackName + "/" + stackUUID),
			}},
			CloudTrailEvent: aws.String(`{"requestParameters":{}}`),
		},
	}

	result := ctEventsCheckerByTarget(t, "cfn")(context.Background(), nil, event, cache)

	if got := result.EffectiveState(); got != domain.RelatedResolved {
		t.Fatalf("state = %v, want RelatedResolved: the cfn list is complete", got)
	}
	if result.Count() != 1 || result.ResourceIDs()[0] != stackName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), stackName)
	}
}

// TestCtEventsLambda_QualifiedARNResolvesTheFunction pins the request-body
// shape an Invoke against an alias writes: functionName is the qualified ARN
// "...:function:<name>:<alias>". Cutting at the last colon leaves the alias,
// which names no function, so the pivot reports a confident zero for a call
// that names a function the account holds.
func TestCtEventsLambda_QualifiedARNResolvesTheFunction(t *testing.T) {
	const fn = "acme-order-processor"
	cache := resource.ResourceCache{"lambda": resource.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: fn, Name: fn}},
	}}

	for _, tc := range []struct {
		what         string
		functionName string
	}{
		{"a bare name", fn},
		{"an unqualified ARN", "arn:aws:lambda:us-east-1:123456789012:function:" + fn},
		{"a name qualified by an alias", fn + ":PROD"},
		{"an ARN qualified by an alias", "arn:aws:lambda:us-east-1:123456789012:function:" + fn + ":PROD"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			event := resource.Resource{
				ID:   "evt-0a1b2c3d4e5f6a7bb",
				Name: "Invoke",
				RawStruct: cloudtrailtypes.Event{
					EventId:   aws.String("evt-0a1b2c3d4e5f6a7bb"),
					EventName: aws.String("Invoke"),
					CloudTrailEvent: aws.String(
						`{"requestParameters":{"functionName":"` + tc.functionName + `"}}`),
				},
			}

			result := ctEventsCheckerByTarget(t, "lambda")(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the lambda list is complete", got)
			}
			if result.Count() != 1 || result.ResourceIDs()[0] != fn {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), fn)
			}
		})
	}
}

// TestCtEventsKMS_AliasResolvesTheKey pins the other request-body shape that
// still guesses one form. A KMS list is keyed by key id and carries the alias,
// prefix included, as the resource's name; an event that names the key by its
// alias must resolve to that key. Cutting at the last slash leaves a string
// that is neither the id nor the name, so the pivot reports a confident zero
// for a call on a key the account holds.
func TestCtEventsKMS_AliasResolvesTheKey(t *testing.T) {
	const (
		keyID = "1234abcd-12ab-34cd-56ef-1234567890ab"
		alias = "alias/acme-prod-key"
	)
	cache := resource.ResourceCache{"kms": resource.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: keyID, Name: alias}},
	}}

	for _, tc := range []struct {
		what  string
		keyID string
	}{
		{"a bare key id", keyID},
		{"a key ARN", "arn:aws:kms:us-east-1:123456789012:key/" + keyID},
		{"an alias name", alias},
		{"an alias ARN", "arn:aws:kms:us-east-1:123456789012:" + alias},
	} {
		t.Run(tc.what, func(t *testing.T) {
			event := resource.Resource{
				ID:   "evt-0a1b2c3d4e5f6a7bc",
				Name: "Decrypt",
				RawStruct: cloudtrailtypes.Event{
					EventId:         aws.String("evt-0a1b2c3d4e5f6a7bc"),
					EventName:       aws.String("Decrypt"),
					CloudTrailEvent: aws.String(`{"requestParameters":{"keyId":"` + tc.keyID + `"}}`),
				},
			}

			result := ctEventsCheckerByTarget(t, "kms")(context.Background(), nil, event, cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the kms list is complete", got)
			}
			if result.Count() != 1 || result.ResourceIDs()[0] != keyID {
				t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), keyID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 10 — one id rule for every path. A wider candidate list is only safe
// while every candidate is a form of the id the event named.
// ---------------------------------------------------------------------------

// TestCtEventsPivots_AnARNSegmentIsNotAnID pins the boundary of the candidate
// rule. An ARN is "arn:<partition>:<service>:<region>:<account>:<type>:<id>",
// and only the last part is an id — the type word is a literal every ARN of
// that service carries. Offering it lets an account holding a resource named
// after that word answer for a call on a resource that is gone, which is the
// row-6 rule broken from the other side: the panel invents a row.
//
// The realistic instance is Secrets Manager, whose ARNs end ":secret:<name>"
// and whose names are free-form, so a secret named "secret" is a legal thing to
// have. The type word here is synthetic because the rule is not about which
// word it is.
func TestCtEventsPivots_AnARNSegmentIsNotAnID(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			arn := "arn:aws:example:us-east-1:123456789012:thing:" + p.id
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: "thing", Name: "thing"}},
			}}

			result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil,
				ctSlashEvent(p, arn), cache)

			if got := result.EffectiveState(); got != domain.RelatedResolved {
				t.Fatalf("state = %v, want RelatedResolved: the list is complete", got)
			}
			if result.Count() != 0 {
				t.Errorf("Count = %d, want 0: %q is gone, and \"thing\" is the ARN's type word, not its id; IDs = %v",
					result.Count(), p.id, result.ResourceIDs())
			}
		})
	}
}

// TestCtEventsSecrets_ARNNamedSecretResolvesTheWholeName pins which candidate
// wins when more than one is real. A secret ARN ends ":secret:<name>" and the
// name may itself contain slashes, so the whole of it after the type word is
// the secret the event named.
//
// The second case is INVERTED against an earlier revision of this file, which
// expected the trailing segment to resolve when nothing held the whole name.
// It does not: "stripe-key" is a piece of "prod/api/stripe-key", so a secret
// under that name is a different secret, and answering with it invents a row
// for one the account no longer holds. Restoring the old expectation would
// re-open that.
func TestCtEventsSecrets_ARNNamedSecretResolvesTheWholeName(t *testing.T) {
	const (
		name = "prod/api/stripe-key"
		tail = "stripe-key"
	)
	arn := "arn:aws:secretsmanager:us-east-1:123456789012:secret:" + name
	event := resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7bd",
		Name: "GetSecretValue",
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("evt-0a1b2c3d4e5f6a7bd"),
			EventName:       aws.String("GetSecretValue"),
			CloudTrailEvent: aws.String(`{"requestParameters":{"secretId":"` + arn + `"}}`),
		},
	}

	for _, tc := range []struct {
		what string
		list []resource.Resource
		want []string
	}{
		{
			what: "both forms exist",
			list: []resource.Resource{{ID: name, Name: name}, {ID: tail, Name: tail}},
			want: []string{name},
		},
		{
			what: "only a piece of the name exists",
			list: []resource.Resource{{ID: tail, Name: tail}},
			want: nil,
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			cache := resource.ResourceCache{"secrets": resource.ResourceCacheEntry{Resources: tc.list}}
			result := ctEventsCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

			if !slices.Equal(result.ResourceIDs(), tc.want) {
				t.Errorf("ResourceIDs = %v, want %v", result.ResourceIDs(), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Row 11 — a candidate is a form of the id, never a fragment of one.
// ---------------------------------------------------------------------------

// TestCtEventsPivots_ANameFragmentIsNotAnID pins rule 5. Stripping one leading
// type word is a form of the id — "instance/i-abc" and "i-abc" name the same
// thing. Cutting again is not: an id that survived the strip is the name, and
// its last segment is a piece of that name, naming a different resource. An
// account holding a resource under that piece must not answer for one the event
// named and the account no longer holds, which is the deleted-resource rule
// from the other side.
func TestCtEventsPivots_ANameFragmentIsNotAnID(t *testing.T) {
	for _, p := range ctSlashPivots() {
		t.Run(p.target, func(t *testing.T) {
			// The account holds only the fragment, under that name. The
			// resource the event named is gone.
			cache := resource.ResourceCache{p.target: resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: p.id, Name: p.id}},
			}}

			for _, tc := range []struct{ what, named string }{
				{"a bare name carrying slashes", "prod/api/" + p.id},
				{"the same name inside an ARN", "arn:aws:example:us-east-1:123456789012:secret:prod/api/" + p.id},
			} {
				t.Run(tc.what, func(t *testing.T) {
					result := ctEventsCheckerByTarget(t, p.target)(context.Background(), nil,
						ctSlashEvent(p, tc.named), cache)

					if got := result.EffectiveState(); got != domain.RelatedResolved {
						t.Fatalf("state = %v, want RelatedResolved: the list is complete", got)
					}
					if result.Count() != 0 {
						t.Errorf("Count = %d, want 0: the event named %q, and %q is a fragment of that name, not the resource; IDs = %v",
							result.Count(), tc.named, p.id, result.ResourceIDs())
					}
				})
			}
		})
	}
}

// TestCtEventsLambda_TheTypeWordIsNotAFunction pins the boundary of the Lambda
// qualifier rule. An unqualified function ARN ends ":function:<name>", so the
// head before the last colon is the type word — grammar, not a name. A function
// really called "function" is legal, and it must not answer for every event that
// names some other function.
func TestCtEventsLambda_TheTypeWordIsNotAFunction(t *testing.T) {
	cache := resource.ResourceCache{"lambda": resource.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "function", Name: "function"}},
	}}
	event := resource.Resource{
		ID:   "evt-0a1b2c3d4e5f6a7be",
		Name: "Invoke",
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String("evt-0a1b2c3d4e5f6a7be"),
			EventName: aws.String("Invoke"),
			CloudTrailEvent: aws.String(
				`{"requestParameters":{"functionName":"arn:aws:lambda:us-east-1:123456789012:function:acme-order-processor"}}`),
		},
	}

	result := ctEventsCheckerByTarget(t, "lambda")(context.Background(), nil, event, cache)

	if got := result.EffectiveState(); got != domain.RelatedResolved {
		t.Fatalf("state = %v, want RelatedResolved: the lambda list is complete", got)
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0: acme-order-processor is gone and \"function\" is the ARN's type word; IDs = %v",
			result.Count(), result.ResourceIDs())
	}
}
