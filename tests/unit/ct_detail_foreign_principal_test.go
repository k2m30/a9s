package unit_test

// A CloudTrail event names the principal that made the call by ARN, and the
// ARN carries the account it belongs to. The account of your session holds its
// own `Admin` role; another account's `Admin` is a different role that this
// account cannot open. The detail must offer to open the first and not the
// second — during an incident, opening the local role for a foreign
// principal's call is a wrong attribution, not a missing one.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const ctLocalAccount = "123456789012"

// ctOneEventAPI serves one LookupEvents page holding a single event.
type ctOneEventAPI struct{ event cloudtrailtypes.Event }

func (a *ctOneEventAPI) LookupEvents(_ context.Context, _ *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: []cloudtrailtypes.Event{a.event}}, nil
}

// ctPrincipalEvent is one AssumeRole call made by principalARN.
func ctPrincipalEvent(id, principalARN string) cloudtrailtypes.Event {
	return cloudtrailtypes.Event{
		EventId:     aws.String(id),
		EventName:   aws.String("ListBuckets"),
		EventSource: aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","arn":"` + principalARN +
			`","accountId":"` + ctAccountOf(principalARN) + `"},"eventSource":"s3.amazonaws.com","eventName":"ListBuckets",` +
			`"awsRegion":"us-east-1","recipientAccountId":"` + ctLocalAccount + `","eventType":"AwsApiCall","eventCategory":"Management"}`),
	}
}

func ctAccountOf(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 {
		return ""
	}
	return parts[4]
}

// ctPrincipalRowOf opens event's detail with roles loaded and returns its
// Principal row. identity is the session's STS answer, nil before it lands.
func ctPrincipalRowOf(t *testing.T, event cloudtrailtypes.Event, roles []resource.Resource, identity *domain.CallerIdentity) app.FieldRow {
	t.Helper()
	page, err := awsclient.FetchCloudTrailEventsPage(t.Context(), &ctOneEventAPI{event: event}, "")
	if err != nil || len(page.Resources) != 1 {
		t.Fatalf("building the event: %v (%d rows)", err, len(page.Resources))
	}
	c := newTestController(t)
	if identity != nil {
		c.ApplyIntents([]runtime.UIIntent{runtime.SetIdentityIntent{Identity: identity}})
	}
	c.ApplyResourcesLoaded("role", roles, nil, false)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ct-events", ResourceID: page.Resources[0].ID},
	}})
	c.EnsureDetailState(page.Resources[0], "ct-events")
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("the event opened no detail body")
	}
	idx := slices.IndexFunc(body.Fields, func(f app.FieldRow) bool { return f.Key == "Principal" })
	if idx < 0 {
		t.Fatalf("the detail carries no Principal row; fields: %+v", body.Fields)
	}
	return body.Fields[idx]
}

var ctLocalIdentity = &domain.CallerIdentity{AccountID: ctLocalAccount, Arn: "arn:aws:iam::" + ctLocalAccount + ":user/demo"} //nolint:gochecknoglobals // read-only fixture

var ctAdminRoles = []resource.Resource{{ //nolint:gochecknoglobals // read-only fixture
	ID:     "Admin",
	Name:   "Admin",
	Type:   "role",
	Fields: map[string]string{"role_name": "Admin", "arn": "arn:aws:iam::" + ctLocalAccount + ":role/Admin"},
}}

func TestCTDetail_ForeignAccountPrincipalDoesNotOpenTheLocalRole(t *testing.T) {
	local := ctPrincipalRowOf(t, ctPrincipalEvent("evt-local-principal",
		"arn:aws:sts::"+ctLocalAccount+":assumed-role/Admin/session-1"), ctAdminRoles, ctLocalIdentity)
	if !local.IsNavigable {
		t.Errorf("the account's own Admin role is not navigable from the event that names it (value %q)", local.Value)
	}
	if local.NavID != "Admin" {
		t.Errorf("Principal NavID = %q, want %q: the row opens the role the ARN names", local.NavID, "Admin")
	}

	foreign := ctPrincipalRowOf(t, ctPrincipalEvent("evt-foreign-principal",
		"arn:aws:sts::999988887777:assumed-role/Admin/session-1"), ctAdminRoles, ctLocalIdentity)
	if foreign.IsNavigable {
		t.Errorf("account 999988887777's Admin is navigable and would open this account's Admin (NavID %q)", foreign.NavID)
	}
	if foreign.Value != "arn:aws:sts::999988887777:assumed-role/Admin/session-1" {
		t.Errorf("Principal Value = %q, want the whole ARN so the operator can see whose it is", foreign.Value)
	}
}

// Every CloudTrail event names the account it was recorded in
// (recipientAccountId), so the Principal row reads its principal against that
// account whether or not the session's own STS call has answered: another
// account's Admin stays another account's before identity lands too, and the
// recorded account's own Admin still opens.
func TestCTDetail_PrincipalReadsTheEventsAccountBeforeIdentityLoads(t *testing.T) {
	foreign := ctPrincipalRowOf(t, ctPrincipalEvent("evt-foreign-no-identity",
		"arn:aws:sts::999988887777:assumed-role/Admin/session-1"), ctAdminRoles, nil)
	if foreign.IsNavigable {
		t.Errorf("with the session identity not loaded, account 999988887777's Admin is navigable and would open this account's Admin (NavID %q)", foreign.NavID)
	}
	local := ctPrincipalRowOf(t, ctPrincipalEvent("evt-local-no-identity",
		"arn:aws:sts::"+ctLocalAccount+":assumed-role/Admin/session-1"), ctAdminRoles, nil)
	if !local.IsNavigable || local.NavID != "Admin" {
		t.Errorf("with the session identity not loaded, the recorded account's own Admin: navigable=%v NavID=%q, want navigable to %q", local.IsNavigable, local.NavID, "Admin")
	}
}
