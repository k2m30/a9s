package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	secretstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

func policyOf(statements ...string) string {
	return `{"Version":"2012-10-17","Statement":[` + strings.Join(statements, ",") + `]}`
}

const unparseablePolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"`

// A recorded GetCallerIdentity error with no account is the session state
// after STS failed; callers do not retry STS.
func identityUnavailable(c *awsclient.ServiceClients) *awsclient.ServiceClients {
	store := session.NewIdentityStore()
	store.Set("", errors.New("operation error STS: GetCallerIdentity, AccessDenied"))
	c.SetIdentityStore(store)
	return c
}

func identityResolved(c *awsclient.ServiceClients) *awsclient.ServiceClients {
	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	c.SetIdentityStore(store)
	return c
}

type policyCaller struct {
	shortName string
	code      domain.FindingCode
	allow     string
	// Empty for lambda: a function policy is written by lambda:AddPermission,
	// which only creates Allow statements.
	deny  string
	wave1 bool
	run   func(t *testing.T, doc string) (findings []domain.Finding, unknown bool)
}

func policyCallers() []policyCaller {
	const orgDeny = `"Condition":{"StringNotEquals":{"aws:PrincipalOrgID":"o-acme12345"}}`
	return []policyCaller{
		{
			shortName: "ddb", code: "ddb.public-policy",
			allow: `{"Sid":"AllowRead","Effect":"Allow","Principal":"*","Action":["dynamodb:GetItem","dynamodb:Query"],"Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-orders"}`,
			deny:  `{"Sid":"DenyOutsideVpce","Effect":"Deny","Principal":"*","Action":"dynamodb:*","Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-orders","Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				res, _ := w2DDBRun(t, &w2DDBPolicyFake{policies: map[string]string{"acme-orders": doc}},
					[]resource.Resource{w2DDBPolicyResource("acme-orders")})
				_, unknown := res.TruncatedIDs["acme-orders"]
				return res.Findings["acme-orders"], unknown
			},
		},
		{
			shortName: "sqs", code: "sqs.public-policy",
			allow: `{"Sid":"AllowSend","Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"}`,
			deny:  `{"Sid":"DenyOutsideVpce","Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders","Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				f := newW5SQSFake()
				f.attrs[w5QueueURL("acme-orders")] = map[string]string{"QueueArn": w5QueueARN("acme-orders"), "Policy": doc}
				res, _ := awsclient.EnrichSQSAttributes(context.Background(), &awsclient.ServiceClients{SQS: f},
					[]resource.Resource{w5QueueRes("acme-orders")}, nil)
				_, unknown := res.TruncatedIDs["acme-orders"]
				return res.Findings["acme-orders"], unknown
			},
		},
		{
			shortName: "sns", code: "sns.public-policy",
			allow: `{"Sid":"AllowPublish","Effect":"Allow","Principal":{"AWS":"*"},"Action":"SNS:Publish","Resource":"arn:aws:sns:us-east-1:123456789012:acme-alerts"}`,
			deny:  `{"Sid":"DenyOutsideOrg","Effect":"Deny","Principal":{"AWS":"*"},"Action":"SNS:Publish","Resource":"arn:aws:sns:us-east-1:123456789012:acme-alerts",` + orgDeny + `}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				arn := w5TopicARN("acme-alerts")
				f := newW5SNSFake()
				f.subs[arn] = w5HealthySub("acme-alerts")
				f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "KmsMasterKeyId": "alias/aws/sns", "Policy": doc}
				res, _ := awsclient.EnrichSNSSubscriptions(context.Background(), &awsclient.ServiceClients{SNS: f},
					[]resource.Resource{w5TopicRes("acme-alerts")}, nil)
				_, unknown := res.TruncatedIDs[arn]
				return res.Findings[arn], unknown
			},
		},
		{
			shortName: "ecr", code: "ecr.public-policy",
			allow: `{"Sid":"AnyonePull","Effect":"Allow","Principal":"*","Action":["ecr:BatchGetImage","ecr:GetDownloadUrlForLayer"]}`,
			deny:  `{"Sid":"DenyOutsideOrg","Effect":"Deny","Principal":"*","Action":"ecr:*",` + orgDeny + `}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				res, _ := w6bECREnrich(t, &w6bECRFake{
					policies:   map[string]string{"acme-api": doc},
					lifecycles: map[string]string{"acme-api": w6bECRLifecycleDoc},
				}, "acme-api")
				_, unknown := res.TruncatedIDs["acme-api"]
				return res.Findings["acme-api"], unknown
			},
		},
		{
			shortName: "efs", code: "efs.public-policy",
			allow: `{"Sid":"AnyoneMount","Effect":"Allow","Principal":{"AWS":"*"},"Action":["elasticfilesystem:ClientMount","elasticfilesystem:ClientWrite"],"Resource":"arn:aws:elasticfilesystem:eu-central-1:123456789012:file-system/fs-0acme0000000abc1"}`,
			deny:  `{"Sid":"DenyOtherAccounts","Effect":"Deny","Principal":{"AWS":"*"},"Action":"elasticfilesystem:*","Resource":"arn:aws:elasticfilesystem:eu-central-1:123456789012:file-system/fs-0acme0000000abc1","Condition":{"StringNotEquals":{"aws:PrincipalAccount":"123456789012"}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				res, _ := w2EFSRun(t, &w2EFSPolicyFake{policies: map[string]string{"fs-0acme0000000abc1": doc}}, "fs-0acme0000000abc1")
				_, unknown := res.TruncatedIDs["fs-0acme0000000abc1"]
				return res.Findings["fs-0acme0000000abc1"], unknown
			},
		},
		{
			shortName: "kms", code: "kms.public-policy",
			allow: `{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"},` +
				`{"Sid":"AllowUse","Effect":"Allow","Principal":{"AWS":"*"},"Action":["kms:Encrypt","kms:Decrypt"],"Resource":"*"}`,
			deny: `{"Sid":"DenyOtherCallers","Effect":"Deny","Principal":{"AWS":"*"},"Action":["kms:Encrypt","kms:Decrypt"],"Resource":"*","Condition":{"StringNotEquals":{"kms:CallerAccount":"123456789012"}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				const id = "5e6f7a8b-1111-2222-3333-444455556666"
				res, _ := w4EnrichKMSErr(t, &w4KMSFake{policies: map[string]string{id: doc}},
					[]resource.Resource{w4KMSResource(id, "alias/acme-orders", kmstypes.KeyManagerTypeCustomer)})
				_, unknown := res.TruncatedIDs[id]
				return res.Findings[id], unknown
			},
		},
		{
			shortName: "secrets", code: "secrets.public-policy",
			allow: `{"Sid":"AllowRead","Effect":"Allow","Principal":{"AWS":"*"},"Action":"secretsmanager:GetSecretValue","Resource":"*"}`,
			deny:  `{"Sid":"DenyOutsideOrg","Effect":"Deny","Principal":{"AWS":"*"},"Action":"secretsmanager:GetSecretValue","Resource":"*",` + orgDeny + `}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				clients := identityResolved(&awsclient.ServiceClients{
					SecretsManager: &w4SecretsFake{policies: map[string]string{"acme-db": doc}}, Region: "us-east-1",
				})
				res, _ := awsclient.EnrichSecretsPolicy(context.Background(), clients, []resource.Resource{w4SecretResource("acme-db")}, nil)
				_, unknown := res.TruncatedIDs["acme-db"]
				return res.Findings["acme-db"], unknown
			},
		},
		{
			shortName: "codeartifact", code: "codeartifact.public-access-policy",
			allow: `{"Sid":"AnyoneRead","Effect":"Allow","Principal":"*","Action":"codeartifact:ReadFromRepository","Resource":"*"}`,
			deny:  `{"Sid":"DenyOutsideOrg","Effect":"Deny","Principal":"*","Action":"codeartifact:*","Resource":"*",` + orgDeny + `}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				res := enrichCodeArtifactPolicy(doc)
				_, unknown := res.TruncatedIDs["acme-artifacts/acme-npm"]
				return res.Findings["acme-artifacts/acme-npm"], unknown
			},
		},
		{
			shortName: "apigw", code: "apigw.no-authorizer-public",
			allow: `{"Effect":"Allow","Principal":"*","Action":"execute-api:Invoke","Resource":"arn:aws:execute-api:eu-central-1:123456789012:rst556acme/*"}`,
			deny:  `{"Effect":"Deny","Principal":"*","Action":"execute-api:Invoke","Resource":"arn:aws:execute-api:eu-central-1:123456789012:rst556acme/*","Condition":{"NotIpAddress":{"aws:SourceIp":["203.0.113.0/24"]}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				return enrichRESTAPIPolicy(t, "rst556acme", doc)
			},
		},
		{
			shortName: "lambda", code: "lambda.public-policy",
			allow: `{"Sid":"public-invoke","Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunction","Resource":"arn:aws:lambda:us-east-1:123456789012:function:acme-api"}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				fake := &pw1LambdaPostureFake{policies: map[string]string{"acme-api": doc}}
				res, _ := awsclient.EnrichLambdaPosture(context.Background(), &awsclient.ServiceClients{Lambda: fake},
					[]resource.Resource{pw1LambdaResource("acme-api")}, nil)
				_, unknown := res.TruncatedIDs["acme-api"]
				return res.Findings["acme-api"], unknown
			},
		},
		{
			shortName: "opensearch", code: "opensearch.public", wave1: true,
			allow: `{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"es:ESHttp*","Resource":"arn:aws:es:eu-central-1:123456789012:domain/acme-search/*"}`,
			deny:  `{"Effect":"Deny","Principal":{"AWS":"*"},"Action":"es:*","Resource":"arn:aws:es:eu-central-1:123456789012:domain/acme-search/*","Condition":{"NotIpAddress":{"aws:SourceIp":["203.0.113.0/24"]}}}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				d := w2OSDomain("acme-search")
				d.VPCOptions = nil
				d.AccessPolicies = aws.String(doc)
				_, res := w2OSRun(t, d)
				return res.Findings["acme-search"], false
			},
		},
		{
			shortName: "vpce", code: "vpce.policy-open", wave1: true,
			allow: `{"Effect":"Allow","Principal":"*","Action":"s3:*","Resource":"*"}`,
			deny:  `{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"*",` + orgDeny + `}`,
			run: func(t *testing.T, doc string) ([]domain.Finding, bool) {
				return d3FetchVPCE(t, doc).Findings, false
			},
		},
	}
}

func enrichCodeArtifactPolicy(doc string) awsclient.IssueEnricherResult {
	fake := &d3CodeArtifactFake{
		domains: []codeartifacttypes.DomainSummary{{Name: aws.String("acme-artifacts")}},
		repos:   []codeartifacttypes.RepositorySummary{{Name: aws.String("acme-npm"), DomainName: aws.String("acme-artifacts")}},
		policy:  doc,
	}
	res, _ := awsclient.EnrichCodeArtifactRepository(context.Background(),
		&awsclient.ServiceClients{CodeArtifact: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: "acme-artifacts/acme-npm", Name: "acme-npm", Type: "codeartifact",
			Fields: map[string]string{"repo_name": "acme-npm", "domain_name": "acme-artifacts"},
		}}, nil)
	return res
}

// A REST API with no authorizer is exposed or not by its resource policy alone.
func enrichRESTAPIPolicy(t *testing.T, id, policy string) ([]domain.Finding, bool) {
	t.Helper()
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes(id, "acme-orders-rest", "REGIONAL", policy),
	)
	_, unknown := res.TruncatedIDs[id]
	return res.Findings[id], unknown
}

// An explicit Deny overrides any Allow it matches, so a wildcard grant fenced
// by a Deny is not public on any type that reads a resource policy.
func TestExplicitDenyClearsThePolicyFindingOnEveryCaller(t *testing.T) {
	for _, c := range policyCallers() {
		if c.deny == "" {
			continue
		}
		t.Run(c.shortName, func(t *testing.T) {
			if fs, _ := c.run(t, policyOf(c.allow, c.deny)); hasCode(fs, c.code) {
				t.Errorf("%s: a wildcard grant fenced by an explicit Deny still raises %s", c.shortName, c.code)
			}
			if fs, _ := c.run(t, policyOf(c.allow)); !hasCode(fs, c.code) {
				t.Errorf("%s: the same wildcard grant with no Deny no longer raises %s; got %v", c.shortName, c.code, w2Codes(fs))
			}
		})
	}
}

// A policy that cannot be parsed says nothing about exposure: the resource is
// unknown, never flagged.
func TestUnparseablePolicyIsUnknownNeverAFinding(t *testing.T) {
	for _, c := range policyCallers() {
		if c.wave1 {
			continue
		}
		t.Run(c.shortName, func(t *testing.T) {
			fs, unknown := c.run(t, unparseablePolicy)
			if hasCode(fs, c.code) {
				t.Errorf("%s: an unparseable policy raised %s", c.shortName, c.code)
			}
			if !unknown {
				t.Errorf("%s: an unparseable policy left the resource known (no TruncatedIDs entry)", c.shortName)
			}
		})
	}
}

// API Gateway hands a REST API's resource policy back JSON-escaped
// (`{\"Version\":...\/*...}`). Read as it stands it does not parse; unescaped
// it is an ordinary document, and the verdict must come from what it says.
func TestAPIGatewayEscapedRESTPolicyIsReadAsTheDocumentItEncodes(t *testing.T) {
	scoped := `{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":\"*\",` +
		`\"Action\":\"execute-api:Invoke\",\"Resource\":\"arn:aws:execute-api:eu-central-1:123456789012:rst556esc\/*\",` +
		`\"Condition\":{\"IpAddress\":{\"aws:SourceIp\":\"203.0.113.0\/24\"}}}]}`
	open := `{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":\"*\",` +
		`\"Action\":\"execute-api:Invoke\",\"Resource\":\"arn:aws:execute-api:eu-central-1:123456789012:rst556esc\/*\"}]}`

	fs, unknown := enrichRESTAPIPolicy(t, "rst556esc", scoped)
	if hasCode(fs, "apigw.no-authorizer-public") {
		t.Errorf("an escaped policy scoped to 203.0.113.0/24 was reported as not scoped: %v", w2Codes(fs))
	}
	if unknown {
		t.Errorf("an escaped policy left the API unknown; it is a readable document once unescaped")
	}

	fs, unknown = enrichRESTAPIPolicy(t, "rst556esc", open)
	if !hasCode(fs, "apigw.no-authorizer-public") {
		t.Errorf("an escaped policy open to everyone did not raise apigw.no-authorizer-public; got %v", w2Codes(fs))
	}
	if unknown {
		t.Errorf("an escaped open policy left the API unknown instead of read")
	}
}

// CodeArtifact lets AWS services read a domain or repository through a
// wildcard principal fenced by aws:PrincipalIsAWSService; only AWS service
// principals satisfy it, so it is not a public repository.
func TestCodeArtifactServicePrincipalGrantIsNotPublic(t *testing.T) {
	const tmpl = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowAWSServices","Effect":"Allow","Principal":"*",` +
		`"Action":["codeartifact:GetAuthorizationToken","codeartifact:ReadFromRepository"],"Resource":"*",` +
		`"Condition":{"Bool":{"aws:PrincipalIsAWSService":"VALUE"}}}]}`

	services := enrichCodeArtifactPolicy(strings.Replace(tmpl, "VALUE", "true", 1))
	if hasCode(services.Findings["acme-artifacts/acme-npm"], "codeartifact.public-access-policy") {
		t.Errorf("a grant only AWS service principals satisfy was reported as public")
	}

	everyoneElse := enrichCodeArtifactPolicy(strings.Replace(tmpl, "VALUE", "false", 1))
	if !hasCode(everyoneElse.Findings["acme-artifacts/acme-npm"], "codeartifact.public-access-policy") {
		t.Errorf("a grant to every non-service caller was not reported as public; got %v",
			w2Codes(everyoneElse.Findings["acme-artifacts/acme-npm"]))
	}
}

// When STS cannot say whose account this is, the resource's own ARN still
// can. A policy that grants the owning account's role is not a cross-account
// share, and one that grants a foreign account still is.
func TestUnresolvedIdentityTakesTheOwnAccountFromTheResourceARN(t *testing.T) {
	ownRole := `{"Sid":"AllowApp","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-app"},"Action":"secretsmanager:GetSecretValue","Resource":"*"}`
	partner := `{"Sid":"AllowPartner","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::210987654321:root"},"Action":"secretsmanager:GetSecretValue","Resource":"*"}`

	secrets := func(t *testing.T, doc string) awsclient.IssueEnricherResult {
		t.Helper()
		clients := identityUnavailable(&awsclient.ServiceClients{
			SecretsManager: &w4SecretsFake{policies: map[string]string{"acme-db": doc}}, Region: "us-east-1",
		})
		res, err := awsclient.EnrichSecretsPolicy(context.Background(), clients, []resource.Resource{w4SecretResource("acme-db")}, nil)
		if err != nil {
			t.Fatalf("EnrichSecretsPolicy: %v", err)
		}
		return res
	}

	t.Run("secrets/own account only", func(t *testing.T) {
		res := secrets(t, policyOf(ownRole))
		if hasCode(res.Findings["acme-db"], "secrets.cross-account-policy") {
			t.Errorf("the owning account's own role was reported as another account: %v",
				res.AttentionDetails["acme-db"]["secrets.cross-account-policy"].Rows)
		}
	})
	t.Run("secrets/own account and a partner", func(t *testing.T) {
		res := secrets(t, policyOf(ownRole, partner))
		w2AssertFinding(t, res.Findings["acme-db"], "secrets.cross-account-policy",
			"resource policy grants another account", domain.SevWarn, "wave2")
		w2AssertRow(t, w2Rows(t, res, "acme-db", "secrets.cross-account-policy"), "Accounts", "210987654321")
	})

	ddbOwn := `{"Sid":"AllowApp","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-app"},"Action":"dynamodb:GetItem","Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-shared"}`
	ddbPartner := `{"Sid":"PartnerRead","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::210987654321:root"},"Action":"dynamodb:GetItem","Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-shared"}`
	ddb := func(t *testing.T, doc string) awsclient.IssueEnricherResult {
		t.Helper()
		clients := identityUnavailable(&awsclient.ServiceClients{
			DynamoDB: &w2DDBPolicyFake{policies: map[string]string{"acme-shared": doc}},
		})
		res, err := w2Enricher(t, "ddb")(context.Background(), clients, []resource.Resource{w2DDBPolicyResource("acme-shared")}, nil)
		if err != nil {
			t.Fatalf("ddb enricher: %v", err)
		}
		return res
	}

	t.Run("ddb/own account only", func(t *testing.T) {
		res := ddb(t, policyOf(ddbOwn))
		if hasCode(res.Findings["acme-shared"], "ddb.cross-account-policy") {
			t.Errorf("the owning account's own role was reported as another account")
		}
	})
	t.Run("ddb/own account and a partner", func(t *testing.T) {
		res := ddb(t, policyOf(ddbOwn, ddbPartner))
		if !hasCode(res.Findings["acme-shared"], "ddb.cross-account-policy") {
			t.Fatalf("a table shared with 210987654321 raised no cross-account finding while STS was unavailable; got %v",
				w2Codes(res.Findings["acme-shared"]))
		}
		w2AssertRow(t, w2Rows(t, res, "acme-shared", "ddb.cross-account-policy"), "Accounts", "210987654321")
	})
}

func relatedCheckerFor(t *testing.T, source, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("no %s -> %s related checker registered", source, target)
	return nil
}

// A local principal may be listed by its name or by its ARN in the owning account.
func assertLocalPrincipals(t *testing.T, got resource.RelatedCheckResult, arnPrefix string, want, absent []string) {
	t.Helper()
	if got.State() != domain.RelatedResolved {
		t.Fatalf("pivot state = %v, want resolved; err=%v", got.State(), got.Err())
	}
	listed := map[string]bool{}
	for _, id := range got.ResourceIDs() {
		listed[id] = true
	}
	for _, n := range want {
		if !listed[n] && !listed[arnPrefix+n] {
			t.Errorf("%s is granted and not denied, yet not listed; got %v", n, got.ResourceIDs())
		}
	}
	for _, n := range absent {
		if listed[n] || listed[arnPrefix+n] {
			t.Errorf("%s is listed as a local principal, but it is either explicitly denied or a foreign account's; got %v",
				n, got.ResourceIDs())
		}
	}
}

func TestRoleTrustPolicyPivotSubtractsDenyAndForeignUsers(t *testing.T) {
	trust := policyOf(
		`{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:user/alice","arn:aws:iam::123456789012:user/bob","arn:aws:iam::210987654321:user/carol"]},"Action":"sts:AssumeRole"}`,
		`{"Sid":"RevokeBob","Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":"sts:AssumeRole"}`,
	)
	role := resource.Resource{
		ID: "acme-deploy", Name: "acme-deploy", Type: "role",
		Fields: map[string]string{"role_name": "acme-deploy", "assume_role_policy_document": trust},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-deploy"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/acme-deploy"),
			Path:     aws.String("/"),
		},
	}
	got := relatedCheckerFor(t, "role", "iam-user")(context.Background(), nil, role, resource.ResourceCache{})
	assertLocalPrincipals(t, got, "arn:aws:iam::123456789012:user/", []string{"alice"}, []string{"bob", "carol"})
}

func TestSNSTopicPolicyPivotSubtractsDenyAndForeignRoles(t *testing.T) {
	arn := w5TopicARN("acme-alerts")
	f := newW5SNSFake()
	f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "Policy": policyOf(
		`{"Sid":"AllowPublishers","Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:role/acme-publisher","arn:aws:iam::123456789012:role/acme-legacy","arn:aws:iam::210987654321:role/acme-auditor"]},"Action":"SNS:Publish","Resource":"`+arn+`"}`,
		`{"Sid":"RetireLegacy","Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-legacy"},"Action":"SNS:Publish","Resource":"`+arn+`"}`,
	)}
	got := relatedCheckerFor(t, "sns", "role")(context.Background(), &awsclient.ServiceClients{SNS: f}, w5TopicRes("acme-alerts"), resource.ResourceCache{})
	assertLocalPrincipals(t, got, "arn:aws:iam::123456789012:role/", []string{"acme-publisher"}, []string{"acme-legacy", "acme-auditor"})
}

func rolePolicyStatements(action, resourceARN string) string {
	return policyOf(
		`{"Sid":"AllowRoles","Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:role/acme-reader","arn:aws:iam::123456789012:role/acme-revoked","arn:aws:iam::210987654321:role/acme-auditor"]},"Action":"`+action+`","Resource":"`+resourceARN+`"}`,
		`{"Sid":"RevokeRole","Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-revoked"},"Action":"`+action+`","Resource":"`+resourceARN+`"}`,
	)
}

type kmsRoleFake struct{ *w4KMSFake }

func (kmsRoleFake) ListGrants(_ context.Context, _ *kms.ListGrantsInput, _ ...func(*kms.Options)) (*kms.ListGrantsOutput, error) {
	return &kms.ListGrantsOutput{}, nil
}

type s3BucketPolicyFake struct {
	awsclient.S3API
	policy string
}

func (f *s3BucketPolicyFake) GetBucketPolicy(_ context.Context, _ *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return &s3.GetBucketPolicyOutput{Policy: aws.String(f.policy)}, nil
}

func localRoleRow(name string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "role",
		Fields: map[string]string{"role_name": name},
		RawStruct: iamtypes.Role{
			RoleName: aws.String(name),
			Arn:      aws.String("arn:aws:iam::123456789012:role/" + name),
			Path:     aws.String("/"),
		},
	}
}

// An explicitly denied role is not granted, and a foreign account's role is
// not the local role that shares its name.
func TestResourcePolicyRolePivotsSubtractDenyAndForeignRoles(t *testing.T) {
	roleCache := resource.ResourceCache{"role": resource.ResourceCacheEntry{Resources: []resource.Resource{
		localRoleRow("acme-reader"), localRoleRow("acme-revoked"), localRoleRow("acme-auditor"),
	}}}
	const keyID = "5e6f7a8b-1111-2222-3333-444455556666"
	cases := []struct {
		source  string
		clients *awsclient.ServiceClients
		res     resource.Resource
	}{
		{
			source: "kms",
			clients: &awsclient.ServiceClients{KMS: kmsRoleFake{&w4KMSFake{policies: map[string]string{
				keyID: rolePolicyStatements("kms:Decrypt", "*"),
			}}}},
			res: w4KMSResource(keyID, "alias/acme-orders", kmstypes.KeyManagerTypeCustomer),
		},
		{
			source: "ecr",
			clients: &awsclient.ServiceClients{ECR: &w6bECRFake{policies: map[string]string{
				"acme-api": policyOf(
					`{"Sid":"AllowRoles","Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:role/acme-reader","arn:aws:iam::123456789012:role/acme-revoked","arn:aws:iam::210987654321:role/acme-auditor"]},"Action":"ecr:BatchGetImage"}`,
					`{"Sid":"RevokeRole","Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-revoked"},"Action":"ecr:BatchGetImage"}`,
				),
			}}},
			res: resource.Resource{ID: "acme-api", Name: "acme-api", Type: "ecr", Fields: map[string]string{}, RawStruct: w6bECRRepo("acme-api")},
		},
		{
			source: "secrets",
			clients: &awsclient.ServiceClients{SecretsManager: &w4SecretsFake{policies: map[string]string{
				"acme-db": rolePolicyStatements("secretsmanager:GetSecretValue", "*"),
			}}},
			res: func() resource.Resource {
				r := w4SecretResource("acme-db")
				r.RawStruct = secretstypes.SecretListEntry{Name: aws.String("acme-db"), ARN: aws.String(w4SecretARN("acme-db"))}
				return r
			}(),
		},
		{
			source: "s3",
			clients: identityResolved(&awsclient.ServiceClients{S3: &s3BucketPolicyFake{
				policy: rolePolicyStatements("s3:GetObject", "arn:aws:s3:::acme-assets/*"),
			}}),
			res: resource.Resource{ID: "acme-assets", Name: "acme-assets", Type: "s3", Fields: map[string]string{"bucket_name": "acme-assets"}},
		},
	}
	for _, c := range cases {
		t.Run(c.source, func(t *testing.T) {
			got := relatedCheckerFor(t, c.source, "role")(context.Background(), c.clients, c.res, roleCache)
			assertLocalPrincipals(t, got, "arn:aws:iam::123456789012:role/",
				[]string{"acme-reader"}, []string{"acme-revoked", "acme-auditor"})
		})
	}
}

func TestQueueFencedByDenyNotPrincipalIsNotPublic(t *testing.T) {
	var sqsCaller policyCaller
	for _, c := range policyCallers() {
		if c.shortName == "sqs" {
			sqsCaller = c
		}
	}
	deny := `{"Sid":"OnlyTheApp","Effect":"Deny","NotPrincipal":{"AWS":"arn:aws:iam::123456789012:role/acme-app"},` +
		`"Action":"sqs:*","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"}`
	if fs, unknown := sqsCaller.run(t, policyOf(sqsCaller.allow, deny)); hasCode(fs, sqsCaller.code) || unknown {
		t.Errorf("a queue only acme-app can reach reads as open to anyone (unknown=%v); got %v", unknown, w2Codes(fs))
	}
}
