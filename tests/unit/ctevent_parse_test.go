package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mustParseEvent calls Parse and fails immediately if it returns an error or nil.
// Use only in happy-path subtests.
func mustParseEvent(t *testing.T, rawJSON string) *ctevent.Event {
	t.Helper()
	ev, err := ctevent.Parse(rawJSON)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if ev == nil {
		t.Fatal("Parse returned nil, expected non-nil event")
	}
	return ev
}

// ---------------------------------------------------------------------------
// 1. Happy path × all userIdentity variants
// ---------------------------------------------------------------------------

func TestCTDetailParse_UserIdentity_IAMUser(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T10:00:00Z",
		"eventSource": "iam.amazonaws.com",
		"eventName": "CreateUser",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "192.0.2.1",
		"eventID": "aaa00000-0000-0000-0000-000000000001",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/Alice",
			"accountId": "111111111111",
			"accessKeyId": "AKIAIOSFODNN7EXAMPLE",
			"userName": "Alice"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "IAMUser" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "IAMUser")
	}
	if ev.UserIdentity.ARN != "arn:aws:iam::111111111111:user/Alice" {
		t.Errorf("UserIdentity.ARN = %q", ev.UserIdentity.ARN)
	}
	if ev.UserIdentity.AccountID != "111111111111" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
	if ev.UserIdentity.UserName != "Alice" {
		t.Errorf("UserIdentity.UserName = %q, want %q", ev.UserIdentity.UserName, "Alice")
	}
}

func TestCTDetailParse_UserIdentity_AssumedRole(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T11:00:00Z",
		"eventSource": "s3.amazonaws.com",
		"eventName": "PutObject",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "192.0.2.2",
		"eventID": "aaa00000-0000-0000-0000-000000000002",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "222222222222",
		"userIdentity": {
			"type": "AssumedRole",
			"principalId": "AROAIDPPEZS35WEXAMPLE:MySession",
			"arn": "arn:aws:sts::222222222222:assumed-role/MyRole/MySession",
			"accountId": "222222222222",
			"accessKeyId": "ASIAIOSFODNN7EXAMPLE",
			"sessionContext": {
				"sessionIssuer": {
					"type": "Role",
					"principalId": "AROAIDPPEZS35WEXAMPLE",
					"arn": "arn:aws:iam::222222222222:role/MyRole",
					"accountId": "222222222222",
					"userName": "MyRole"
				},
				"attributes": {
					"mfaAuthenticated": "false",
					"creationDate": "2024-01-15T10:50:00Z"
				}
			}
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "AssumedRole" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "AssumedRole")
	}
	if ev.UserIdentity.ARN != "arn:aws:sts::222222222222:assumed-role/MyRole/MySession" {
		t.Errorf("UserIdentity.ARN = %q", ev.UserIdentity.ARN)
	}
	if ev.UserIdentity.AccountID != "222222222222" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
	if ev.UserIdentity.SessionContext == nil {
		t.Fatal("UserIdentity.SessionContext is nil")
	}
	if ev.UserIdentity.SessionContext.SessionIssuer == nil {
		t.Fatal("UserIdentity.SessionContext.SessionIssuer is nil")
	}
	if ev.UserIdentity.SessionContext.SessionIssuer.UserName != "MyRole" {
		t.Errorf("SessionIssuer.UserName = %q, want %q",
			ev.UserIdentity.SessionContext.SessionIssuer.UserName, "MyRole")
	}
}

func TestCTDetailParse_UserIdentity_IdentityCenterUser(t *testing.T) {
	// IdentityCenterUser / SSO — bearer-token-based direct IDC API calls (§4.9)
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T12:00:00Z",
		"eventSource": "sso.amazonaws.com",
		"eventName": "ListAccountRoles",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "192.0.2.3",
		"eventID": "aaa00000-0000-0000-0000-000000000003",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "333333333333",
		"userIdentity": {
			"type": "IdentityCenterUser",
			"accountId": "333333333333",
			"principalId": "idc-principal-abc123",
			"arn": "arn:aws:sso:::instance/ssoins-abc123",
			"credentialId": "EXAMPLEVHULjJdTUdPJfofVa1sufHDoj7aYcOYcxFVllWR"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "IdentityCenterUser" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "IdentityCenterUser")
	}
	if ev.UserIdentity.AccountID != "333333333333" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
}

func TestCTDetailParse_UserIdentity_AWSService(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T13:00:00Z",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "RunInstances",
		"awsRegion": "us-west-2",
		"sourceIPAddress": "autoscaling.amazonaws.com",
		"eventID": "aaa00000-0000-0000-0000-000000000004",
		"eventType": "AwsServiceEvent",
		"eventCategory": "Management",
		"recipientAccountId": "444444444444",
		"userIdentity": {
			"type": "AWSService",
			"invokedBy": "autoscaling.amazonaws.com",
			"principalId": "AWSService",
			"accountId": "444444444444"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "AWSService" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "AWSService")
	}
	if ev.UserIdentity.InvokedBy != "autoscaling.amazonaws.com" {
		t.Errorf("UserIdentity.InvokedBy = %q, want %q",
			ev.UserIdentity.InvokedBy, "autoscaling.amazonaws.com")
	}
}

func TestCTDetailParse_UserIdentity_Root(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T14:00:00Z",
		"eventSource": "iam.amazonaws.com",
		"eventName": "DeleteUser",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.5",
		"eventID": "aaa00000-0000-0000-0000-000000000005",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "555555555555",
		"userIdentity": {
			"type": "Root",
			"principalId": "555555555555",
			"arn": "arn:aws:iam::555555555555:root",
			"accountId": "555555555555"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "Root" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "Root")
	}
	if ev.UserIdentity.ARN != "arn:aws:iam::555555555555:root" {
		t.Errorf("UserIdentity.ARN = %q", ev.UserIdentity.ARN)
	}
	if ev.UserIdentity.AccountID != "555555555555" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
}

func TestCTDetailParse_UserIdentity_WebIdentityUser(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T15:00:00Z",
		"eventSource": "sts.amazonaws.com",
		"eventName": "AssumeRoleWithWebIdentity",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "203.0.113.1",
		"eventID": "aaa00000-0000-0000-0000-000000000006",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "666666666666",
		"userIdentity": {
			"type": "WebIdentityUser",
			"principalId": "accounts.google.com:app-id.apps.googleusercontent.com:user-xyz",
			"userName": "user-xyz",
			"identityProvider": "accounts.google.com",
			"accountId": "666666666666"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "WebIdentityUser" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "WebIdentityUser")
	}
	if ev.UserIdentity.UserName != "user-xyz" {
		t.Errorf("UserIdentity.UserName = %q, want %q", ev.UserIdentity.UserName, "user-xyz")
	}
}

func TestCTDetailParse_UserIdentity_FederatedUser(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T16:00:00Z",
		"eventSource": "s3.amazonaws.com",
		"eventName": "GetObject",
		"awsRegion": "eu-west-1",
		"sourceIPAddress": "198.51.100.10",
		"eventID": "aaa00000-0000-0000-0000-000000000007",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "777777777777",
		"userIdentity": {
			"type": "FederatedUser",
			"principalId": "777777777777:federation-session",
			"arn": "arn:aws:sts::777777777777:federated-user/federation-session",
			"accountId": "777777777777",
			"accessKeyId": "ASIAIOSFODNN7FED",
			"sessionContext": {
				"sessionIssuer": {
					"type": "IAMUser",
					"principalId": "AIDAJ45Q7YFFAREXAMPLE",
					"arn": "arn:aws:iam::777777777777:user/FedAdmin",
					"accountId": "777777777777",
					"userName": "FedAdmin"
				},
				"attributes": {
					"mfaAuthenticated": "true",
					"creationDate": "2024-01-15T15:55:00Z"
				}
			}
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "FederatedUser" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "FederatedUser")
	}
	if ev.UserIdentity.ARN != "arn:aws:sts::777777777777:federated-user/federation-session" {
		t.Errorf("UserIdentity.ARN = %q", ev.UserIdentity.ARN)
	}
	if ev.UserIdentity.SessionContext == nil {
		t.Fatal("UserIdentity.SessionContext is nil")
	}
}

func TestCTDetailParse_UserIdentity_SAMLUser(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T17:00:00Z",
		"eventSource": "sts.amazonaws.com",
		"eventName": "AssumeRoleWithSAML",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.20",
		"eventID": "aaa00000-0000-0000-0000-000000000008",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "888888888888",
		"userIdentity": {
			"type": "SAMLUser",
			"principalId": "saml-name-qualifier:alice@example.com",
			"userName": "alice@example.com",
			"identityProvider": "arn:aws:iam::888888888888:saml-provider/MyCorpIdP",
			"accountId": "888888888888"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "SAMLUser" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "SAMLUser")
	}
	if ev.UserIdentity.UserName != "alice@example.com" {
		t.Errorf("UserIdentity.UserName = %q, want %q", ev.UserIdentity.UserName, "alice@example.com")
	}
	if ev.UserIdentity.AccountID != "888888888888" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
}

func TestCTDetailParse_UserIdentity_AWSAccount(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T18:00:00Z",
		"eventSource": "sts.amazonaws.com",
		"eventName": "AssumeRole",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.30",
		"eventID": "aaa00000-0000-0000-0000-000000000009",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "AWSAccount",
			"principalId": "AIDAEXAMPLECROSSACCT",
			"accountId": "999999999999",
			"arn": "arn:aws:iam::999999999999:root"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "AWSAccount" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "AWSAccount")
	}
	if ev.UserIdentity.AccountID != "999999999999" {
		t.Errorf("UserIdentity.AccountID = %q, want %q", ev.UserIdentity.AccountID, "999999999999")
	}
}

func TestCTDetailParse_UserIdentity_Unknown(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T19:00:00Z",
		"eventSource": "s3.amazonaws.com",
		"eventName": "GetObject",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.40",
		"eventID": "aaa00000-0000-0000-0000-00000000000a",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "Unknown",
			"accountId": "111111111111"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "Unknown" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "Unknown")
	}
}

func TestCTDetailParse_UserIdentity_Directory(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T20:00:00Z",
		"eventSource": "workspaces.amazonaws.com",
		"eventName": "DescribeWorkspaces",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.50",
		"eventID": "aaa00000-0000-0000-0000-00000000000b",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "222222222222",
		"userIdentity": {
			"type": "Directory",
			"accountId": "222222222222",
			"userName": "corp-alias"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "Directory" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "Directory")
	}
	if ev.UserIdentity.AccountID != "222222222222" {
		t.Errorf("UserIdentity.AccountID = %q", ev.UserIdentity.AccountID)
	}
}

// AssumedRole with AWSReservedSSO_ issuer — SSO "human via permission set" pattern (§4.3)
func TestCTDetailParse_UserIdentity_AssumedRole_SSO(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-01-15T21:00:00Z",
		"eventSource": "iam.amazonaws.com",
		"eventName": "ListRoles",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "198.51.100.60",
		"eventID": "aaa00000-0000-0000-0000-00000000000c",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "333333333333",
		"userIdentity": {
			"type": "AssumedRole",
			"principalId": "AROAIDPPEZS35WEXAMPLE:bob@company.com",
			"arn": "arn:aws:sts::333333333333:assumed-role/AWSReservedSSO_AdminAccess_abc123/bob@company.com",
			"accountId": "333333333333",
			"accessKeyId": "ASIAIOSFODNN7SSO",
			"sessionContext": {
				"sessionIssuer": {
					"type": "Role",
					"principalId": "AROAIDPPEZS35WEXAMPLE",
					"arn": "arn:aws:iam::333333333333:role/aws-reserved/sso.amazonaws.com/AWSReservedSSO_AdminAccess_abc123",
					"accountId": "333333333333",
					"userName": "AWSReservedSSO_AdminAccess_abc123"
				},
				"attributes": {
					"mfaAuthenticated": "false",
					"creationDate": "2024-01-15T20:50:00Z"
				}
			}
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.UserIdentity.Type != "AssumedRole" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "AssumedRole")
	}
	if ev.UserIdentity.SessionContext == nil {
		t.Fatal("UserIdentity.SessionContext is nil")
	}
	issuer := ev.UserIdentity.SessionContext.SessionIssuer
	if issuer == nil {
		t.Fatal("SessionIssuer is nil")
	}
	if !strings.HasPrefix(issuer.UserName, "AWSReservedSSO_") {
		t.Errorf("SessionIssuer.UserName %q does not have AWSReservedSSO_ prefix", issuer.UserName)
	}
}

// ---------------------------------------------------------------------------
// 2. Dispatch matrix — EventCategory × EventType
// ---------------------------------------------------------------------------

func TestCTDetailParse_Dispatch_Management_AwsApiCall(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-02-01T10:00:00Z",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "DescribeInstances",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.0.0.1",
		"eventID": "bbb00000-0000-0000-0000-000000000001",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/Bob",
			"accountId": "111111111111",
			"userName": "Bob"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.EventCategory != "Management" {
		t.Errorf("EventCategory = %q, want %q", ev.EventCategory, "Management")
	}
	if ev.EventType != "AwsApiCall" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "AwsApiCall")
	}
}

func TestCTDetailParse_Dispatch_Management_AwsServiceEvent(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-02-01T11:00:00Z",
		"eventSource": "kms.amazonaws.com",
		"eventName": "RotateKey",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "kms.amazonaws.com",
		"eventID": "bbb00000-0000-0000-0000-000000000002",
		"eventType": "AwsServiceEvent",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "AWSService",
			"invokedBy": "kms.amazonaws.com",
			"accountId": "111111111111"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.EventCategory != "Management" {
		t.Errorf("EventCategory = %q, want %q", ev.EventCategory, "Management")
	}
	if ev.EventType != "AwsServiceEvent" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "AwsServiceEvent")
	}
}

func TestCTDetailParse_Dispatch_Insight(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-02-01T12:00:00Z",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "RunInstances",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "AWS Internal",
		"eventID": "bbb00000-0000-0000-0000-000000000003",
		"eventType": "AwsCloudTrailInsight",
		"eventCategory": "Insight",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/Carol",
			"accountId": "111111111111",
			"userName": "Carol"
		},
		"insightDetails": {
			"state": "Start",
			"eventSource": "ec2.amazonaws.com",
			"eventName": "RunInstances",
			"insightType": "ApiCallRateInsight",
			"insightContext": {}
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.EventCategory != "Insight" {
		t.Errorf("EventCategory = %q, want %q", ev.EventCategory, "Insight")
	}
	if ev.EventType != "AwsCloudTrailInsight" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "AwsCloudTrailInsight")
	}
}

func TestCTDetailParse_Dispatch_NetworkActivity(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-02-01T13:00:00Z",
		"eventSource": "s3.amazonaws.com",
		"eventName": "GetObject",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.0.1.1",
		"eventID": "bbb00000-0000-0000-0000-000000000004",
		"eventType": "AwsVpceEvent",
		"eventCategory": "NetworkActivity",
		"recipientAccountId": "111111111111",
		"errorCode": "VpceAccessDenied",
		"errorMessage": "VPC endpoint access denied",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/Dave",
			"accountId": "111111111111",
			"userName": "Dave"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.EventCategory != "NetworkActivity" {
		t.Errorf("EventCategory = %q, want %q", ev.EventCategory, "NetworkActivity")
	}
	if ev.EventType != "AwsVpceEvent" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "AwsVpceEvent")
	}
}

// ---------------------------------------------------------------------------
// 3. Missing optional fields — no error, zero-valued
// ---------------------------------------------------------------------------

func TestCTDetailParse_MissingOptionalFields(t *testing.T) {
	// No responseElements, no errorCode, no sessionContext.webIdFederationData
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-03-01T10:00:00Z",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "DescribeInstances",
		"awsRegion": "ap-southeast-1",
		"sourceIPAddress": "10.10.10.10",
		"eventID": "ccc00000-0000-0000-0000-000000000001",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "444444444444",
		"requestParameters": {"maxResults": 100},
		"userIdentity": {
			"type": "AssumedRole",
			"principalId": "AROAIDPPEZS35WEXAMPLE:TestSession",
			"arn": "arn:aws:sts::444444444444:assumed-role/TestRole/TestSession",
			"accountId": "444444444444",
			"sessionContext": {
				"sessionIssuer": {
					"type": "Role",
					"principalId": "AROAIDPPEZS35WEXAMPLE",
					"arn": "arn:aws:iam::444444444444:role/TestRole",
					"accountId": "444444444444",
					"userName": "TestRole"
				},
				"attributes": {
					"mfaAuthenticated": "false",
					"creationDate": "2024-03-01T09:50:00Z"
				}
			}
		}
	}`
	ev := mustParseEvent(t, raw)

	// ResponseElements absent → nil map
	if ev.ResponseElements != nil {
		t.Errorf("ResponseElements = %v, want nil", ev.ResponseElements)
	}
	// ErrorCode absent → empty string
	if ev.ErrorCode != "" {
		t.Errorf("ErrorCode = %q, want empty", ev.ErrorCode)
	}
	// ErrorMessage absent → empty string
	if ev.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty", ev.ErrorMessage)
	}
	// No webIdFederationData → field is nil
	if ev.UserIdentity.SessionContext != nil && ev.UserIdentity.SessionContext.WebIDFederationData != nil {
		t.Errorf("WebIDFederationData should be nil when not present in JSON")
	}
}

// ---------------------------------------------------------------------------
// 4. Error paths
// ---------------------------------------------------------------------------

func TestCTDetailParse_Error_EmptyInput(t *testing.T) {
	_, err := ctevent.Parse("")
	if err == nil {
		t.Fatal("Parse(\"\") returned nil error, expected error")
	}
	if !strings.Contains(err.Error(), "ctdetail: empty input") {
		t.Errorf("error message = %q, want to contain %q", err.Error(), "ctdetail: empty input")
	}
}

func TestCTDetailParse_Error_MalformedJSON(t *testing.T) {
	_, err := ctevent.Parse(`{"eventVersion": "1.11", "eventName": `)
	if err == nil {
		t.Fatal("Parse(malformed JSON) returned nil error, expected error")
	}
	if !strings.Contains(err.Error(), "ctdetail: parse failed:") {
		t.Errorf("error message = %q, want to contain %q", err.Error(), "ctdetail: parse failed:")
	}
}

// ---------------------------------------------------------------------------
// 5. Verb classification
// ---------------------------------------------------------------------------

func TestCTDetailParse_Verb_Read(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-04-01T10:00:00Z",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "DescribeInstances",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.0.0.1",
		"eventID": "ddd00000-0000-0000-0000-000000000001",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/ReadUser",
			"accountId": "111111111111",
			"userName": "ReadUser"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.Verb != "R" {
		t.Errorf("Verb = %q for DescribeInstances, want %q", ev.Verb, "R")
	}
}

func TestCTDetailParse_Verb_Delete(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-04-01T11:00:00Z",
		"eventSource": "iam.amazonaws.com",
		"eventName": "DeleteRole",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.0.0.2",
		"eventID": "ddd00000-0000-0000-0000-000000000002",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/AdminUser",
			"accountId": "111111111111",
			"userName": "AdminUser"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.Verb != "D" {
		t.Errorf("Verb = %q for DeleteRole, want %q", ev.Verb, "D")
	}
}

func TestCTDetailParse_Verb_Write(t *testing.T) {
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-04-01T12:00:00Z",
		"eventSource": "s3.amazonaws.com",
		"eventName": "PutObject",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.0.0.3",
		"eventID": "ddd00000-0000-0000-0000-000000000003",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "IAMUser",
			"principalId": "AIDAJ45Q7YFFAREXAMPLE",
			"arn": "arn:aws:iam::111111111111:user/Writer",
			"accountId": "111111111111",
			"userName": "Writer"
		}
	}`
	ev := mustParseEvent(t, raw)
	if ev.Verb != "W" {
		t.Errorf("Verb = %q for PutObject, want %q", ev.Verb, "W")
	}
}

// ---------------------------------------------------------------------------
// 6. Unknown identity type — graceful fallback
// ---------------------------------------------------------------------------

func TestCTDetailParse_UserIdentity_FuturePrincipal(t *testing.T) {
	// A hypothetical future type not yet in the taxonomy must parse without error.
	raw := `{
		"eventVersion": "1.11",
		"eventTime": "2024-05-01T10:00:00Z",
		"eventSource": "newservice.amazonaws.com",
		"eventName": "DescribeWidgets",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "10.1.2.3",
		"eventID": "eee00000-0000-0000-0000-000000000001",
		"eventType": "AwsApiCall",
		"eventCategory": "Management",
		"recipientAccountId": "111111111111",
		"userIdentity": {
			"type": "FuturePrincipal",
			"principalId": "FUTUREPRINCIPALEXAMPLE",
			"arn": "arn:aws:sts::111111111111:future/FuturePrincipal/session",
			"accountId": "111111111111"
		}
	}`
	ev, err := ctevent.Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned unexpected error for unknown identity type: %v", err)
	}
	if ev == nil {
		t.Fatal("Parse returned nil, expected non-nil event")
	}
	if ev.UserIdentity.Type != "FuturePrincipal" {
		t.Errorf("UserIdentity.Type = %q, want %q", ev.UserIdentity.Type, "FuturePrincipal")
	}
	if ev.UserIdentity.ARN != "arn:aws:sts::111111111111:future/FuturePrincipal/session" {
		t.Errorf("UserIdentity.ARN = %q, want the raw ARN value", ev.UserIdentity.ARN)
	}
}
