package unit

// The colour fallback for a ct-events row that carries no Finding rebuilds the
// finding from the row's own Fields, so it can only read keys the fetcher
// actually writes. The cause is what decides which finding the row stands for
// — a denied call is not the same event as a destructive one — so the cause
// must survive in Fields alongside the tier and the error code.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// ct0916LookupClient serves one canned LookupEvents page.
type ct0916LookupClient struct {
	events []cloudtrailtypes.Event
}

func (c *ct0916LookupClient) LookupEvents(_ context.Context, _ *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: c.events}, nil
}

// ct0916Fetch runs the real ct-events fetcher over one event whose
// CloudTrailEvent blob is rawJSON.
func ct0916Fetch(t *testing.T, eventName, rawJSON string) domain.Resource {
	t.Helper()
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	client := &ct0916LookupClient{events: []cloudtrailtypes.Event{{
		EventId:         aws.String("evt-ct0916-" + eventName),
		EventName:       aws.String(eventName),
		EventTime:       &eventTime,
		EventSource:     aws.String("ec2.amazonaws.com"),
		Username:        aws.String("Deployer"),
		ReadOnly:        aws.String("true"),
		CloudTrailEvent: aws.String(rawJSON),
	}}}
	res, err := awsclient.FetchCloudTrailEventsPage(context.Background(), client, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

// ct0916EventJSON is a read-only ec2:DescribeInstances call as CloudTrail
// records it, recorded by account 123456789012. callerAccount and errorLines
// are what the three tiers differ by.
func ct0916EventJSON(callerAccount, errorLines string) string {
	return `{
  "eventVersion": "1.08",
  "userIdentity": {"type": "AssumedRole", "accountId": "` + callerAccount + `",
    "arn": "arn:aws:sts::` + callerAccount + `:assumed-role/Deployer/session1"},
  "eventTime": "2025-03-15T12:00:00Z",
  "eventSource": "ec2.amazonaws.com",
  "eventName": "DescribeInstances",
  "awsRegion": "eu-west-1",
  "recipientAccountId": "123456789012",` + errorLines + `
  "eventCategory": "Management",
  "eventType": "AwsApiCall"
}`
}

var (
	ct0916DeniedEventJSON = ct0916EventJSON("123456789012", `
  "errorCode": "AccessDenied",
  "errorMessage": "User is not authorized to perform ec2:DescribeInstances",`)
	ct0916CrossAccountEventJSON = ct0916EventJSON("210987654321", "")
	ct0916RoutineEventJSON      = ct0916EventJSON("123456789012", "")
)

func TestCT_0916_Fetcher_WritesTheCauseTheColourFallbackReads(t *testing.T) {
	cases := []struct {
		name      string
		rawJSON   string
		wantCause string
		wantTier  string
	}{
		{"denied call", ct0916DeniedEventJSON, "error", "ct-danger"},
		{"cross-account read", ct0916CrossAccountEventJSON, "cross_account", "ct-attention"},
		{"routine read", ct0916RoutineEventJSON, "", "ct-info"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ct0916Fetch(t, "DescribeInstances", tc.rawJSON)
			if got := r.Fields["status"]; got != tc.wantTier {
				t.Fatalf("Fields[status] = %q, want %q", got, tc.wantTier)
			}
			if got := r.Fields["_ct.cause"]; got != tc.wantCause {
				t.Errorf("Fields[_ct.cause] = %q, want %q — the tier alone does not say which finding the row stands for",
					got, tc.wantCause)
			}
		})
	}
}

func TestCT_0916_DeniedCall_CarriesTheErrorCodeAndNamesTheError(t *testing.T) {
	r := ct0916Fetch(t, "DescribeInstances", ct0916DeniedEventJSON)
	if got := r.Fields["_ct.error_code"]; got != "AccessDenied" {
		t.Errorf("Fields[_ct.error_code] = %q, want %q", got, "AccessDenied")
	}
	if len(r.Findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(r.Findings), r.Findings)
	}
	if r.Findings[0].Code != awsclient.CodeCTEventFailedCall {
		t.Errorf("finding code = %q, want %q", r.Findings[0].Code, awsclient.CodeCTEventFailedCall)
	}
	if !strings.Contains(r.Findings[0].Phrase, "access denied") {
		t.Errorf("finding phrase = %q, want it to name %q", r.Findings[0].Phrase, "access denied")
	}
}

// The ct-events FieldKeys list enumerates the _ct.* keys a row carries; a key
// the colour fallback depends on that is absent from the list is a key nothing
// guarantees survives a cache round trip.
func TestCT_0916_CTEventsFieldKeys_EnumerateTheCause(t *testing.T) {
	td := catalog.FindAny("ct-events")
	if td == nil {
		t.Fatal("no ct-events type def in the catalog")
	}
	for _, want := range []string{"_ct.cause", "_ct.error_code", "status"} {
		found := false
		for _, k := range td.FieldKeys {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ct-events FieldKeys does not enumerate %q; have %v", want, td.FieldKeys)
		}
	}
}

// A row restored from cache has its Fields but no Findings. It must keep the
// colour the fetched row had — the fallback exists for exactly this row.
func TestCT_0916_ColourSurvivesWhenTheRowCarriesNoFinding(t *testing.T) {
	td := catalog.FindAny("ct-events")
	if td == nil {
		t.Fatal("no ct-events type def in the catalog")
	}
	if td.Color == nil {
		t.Fatal("ct-events type def has no Color func")
	}

	for _, tc := range []struct {
		name    string
		rawJSON string
	}{
		{"denied call", ct0916DeniedEventJSON},
		{"cross-account read", ct0916CrossAccountEventJSON},
		{"routine read", ct0916RoutineEventJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetched := ct0916Fetch(t, "DescribeInstances", tc.rawJSON)
			want := td.Color(fetched)

			restored := fetched.Clone()
			restored.Findings = nil
			if got := td.Color(restored); got != want {
				t.Errorf("colour of the finding-less row = %v, want %v (the colour of the same row as the fetcher built it)", got, want)
			}
		})
	}
}
