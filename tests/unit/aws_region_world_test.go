package unit_test

// aws_region_world_test.go — a multi-region AWS answered over HTTP, for the
// tests that pin which region each call is issued against.
//
// Clients are built the way production builds them (CreateServiceClients over
// an aws.Config), so every call is a real SDK request. The region a call went
// to is read from its SigV4 credential scope, which is what AWS itself routes
// and authorises on. Each region answers only for what lives in it, the way
// AWS does: a CLOUDFRONT-scope web ACL exists only in us-east-1, a bucket
// answers PermanentRedirect outside its own region, a log group, certificate
// or alarm is absent from every region but its own.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go/encoding/cbor"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	rwEdgeACLID  = "3f8e1c2a-7b4d-4e6f-9a1b-2c3d4e5f6a7b"
	rwEdgeACLArn = "arn:aws:wafv2:us-east-1:123456789012:global/webacl/acme-edge-acl/" + rwEdgeACLID
	rwAPIACLID   = "9b2d4f6a-1c3e-4a5b-8d7f-0e1a2b3c4d5e"
	rwAPIACLArn  = "arn:aws:wafv2:eu-west-1:123456789012:regional/webacl/acme-api-acl/" + rwAPIACLID
	rwAPIALBArn  = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/acme-api-alb/50dc6c495c0c9188"
	rwEdgeLogArn = "arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-acme-edge"

	rwLegacyACLID  = "5d6e7f8a-9b0c-4d1e-8f2a-3b4c5d6e7f8a"
	rwLegacyACLArn = "arn:aws:wafv2:us-east-1:123456789012:global/webacl/acme-legacy-edge-acl/" + rwLegacyACLID

	rwDistID      = "E2EXAMPLE1ACME"
	rwEdgeCertArn = "arn:aws:acm:us-east-1:123456789012:certificate/6f1e2d3c-4b5a-4968-8a7b-9c0d1e2f3a4b"
	rwAPICertArn  = "arn:aws:acm:eu-west-1:123456789012:certificate/0a1b2c3d-4e5f-4a6b-9c8d-7e6f5a4b3c2d"
	rwEdgeAlarm   = "acme-edge-5xx-rate"
	rwAPIAlarm    = "acme-api-alb-5xx"

	rwZoneID       = "/hostedzone/Z0EXAMPLE1ACME"
	rwZoneLogGroup = "/aws/route53/acme-example.com"

	rwOrgTrail         = "acme-org-trail"
	rwOrgTrailLogGroup = "acme-org-trail-events"
	rwEUTrail          = "acme-eu-trail"
	rwEUTrailLogGroup  = "acme-eu-trail-events"

	rwAssetsBucket = "acme-assets-eu"
	rwWebBucket    = "acme-web-use1"
	rwTrailBucket  = "acme-org-cloudtrail"
	rwAssetsQueue  = "arn:aws:sqs:eu-west-1:123456789012:acme-ingest"
	rwAssetsKMSKey = "arn:aws:kms:eu-west-1:123456789012:key/0f1e2d3c-4b5a-4978-8a9b-0c1d2e3f4a5b"

	rwReportsBucket    = "acme-reports-eu"
	rwAssetsAlias      = "alias/acme-assets"
	rwUSAssetsKeyID    = "5c4d3e2f-1a0b-4c9d-8e7f-6a5b4c3d2e1f"
	rwEUAssetsKeyID    = "0f1e2d3c-4b5a-4978-8a9b-0c1d2e3f4a5b"
	rwOrgTrailKMSKeyID = "7a6b5c4d-3e2f-4a1b-9c8d-7e6f5a4b3c2d"
	rwOrgTrailTopic    = "arn:aws:sns:us-west-2:123456789012:acme-org-trail-alerts"
	rwEUTrailKMSKeyID  = "1b2c3d4e-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
)

// rwCall is one request the world answered: the service and region from its
// credential scope, the operation, and the resource it named.
type rwCall struct {
	Service, Region, Op, Subject string
}

type rwWorld struct {
	mu    sync.Mutex
	calls []rwCall
	deny  map[string]bool // "service@region" answers AccessDenied
}

func newRegionWorld() *rwWorld {
	return &rwWorld{deny: map[string]bool{}}
}

// clients builds the session's client set the way production does, for a
// session whose selected region is sessionRegion.
func (w *rwWorld) clients(sessionRegion string) *awsclient.ServiceClients {
	return awsclient.CreateServiceClients(aws.Config{
		Region:           sessionRegion,
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: w},
		RetryMaxAttempts: 1,
	})
}

func (w *rwWorld) denyAccess(service, region string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deny[service+"@"+region] = true
}

func (w *rwWorld) resetCalls() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = nil
}

// callsFor returns every recorded call to service whose subject contains
// subject ("" matches all).
func (w *rwWorld) callsFor(service, subject string) []rwCall {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []rwCall
	for _, c := range w.calls {
		if c.Service == service && strings.Contains(c.Subject, subject) {
			out = append(out, c)
		}
	}
	return out
}

// requireCallsOnlyIn exempts region discovery (GetBucketLocation, HeadBucket):
// asking the wrong region where a bucket lives is how its region is learned.
func (w *rwWorld) requireCallsOnlyIn(t *testing.T, service, subject, region string) {
	t.Helper()
	var in int
	var elsewhere []string
	for _, c := range w.callsFor(service, subject) {
		if c.Op == "location" || c.Op == "HeadBucket" {
			continue
		}
		if c.Region == region {
			in++
			continue
		}
		elsewhere = append(elsewhere, c.Op+"@"+c.Region)
	}
	if len(elsewhere) > 0 {
		t.Errorf("%s calls about %q went to the wrong region %v; every one must go to %s", service, subject, elsewhere, region)
	}
	if in == 0 {
		t.Errorf("no %s call about %q was issued against %s", service, subject, region)
	}
}

func (w *rwWorld) requireNoCallsIn(t *testing.T, service, subject, region string) {
	t.Helper()
	for _, c := range w.callsFor(service, subject) {
		if c.Region == region {
			t.Errorf("%s %s about %q was issued against %s, where that resource does not live", service, c.Op, subject, region)
		}
	}
}

func (w *rwWorld) RoundTrip(req *http.Request) (*http.Response, error) {
	region, service := rwScope(req.Header.Get("Authorization"))
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	}
	op, subject := rwOperation(service, req, body)

	w.mu.Lock()
	w.calls = append(w.calls, rwCall{Service: service, Region: region, Op: op, Subject: subject})
	denied := w.deny[service+"@"+region]
	w.mu.Unlock()

	if denied {
		return rwDenied(service, op, req.Method), nil
	}
	switch service {
	case "wafv2":
		return rwWAF(region, op, body), nil
	case "acm":
		return rwACM(region, op, body), nil
	case "logs":
		return rwLogs(region, op, body), nil
	case "monitoring":
		return rwAlarms(region, op), nil
	case "cloudtrail":
		return rwTrails(region, op), nil
	case "s3":
		return rwS3(region, op, subject, req), nil
	case "route53":
		return rwRoute53(op, req), nil
	case "cloudfront":
		return rwCloudFront(req), nil
	case "kms":
		return rwKMS(region, op, body), nil
	case "sns":
		return rwSNS(region, body), nil
	}
	return rwJSONError(400, "UnknownOperationException", service+" "+op+" is not modelled by the region test world"), nil
}

// rwScope reads region and service from the SigV4 credential scope:
// Credential=<key>/<date>/<region>/<service>/aws4_request.
func rwScope(auth string) (region, service string) {
	_, cred, ok := strings.Cut(auth, "Credential=")
	if !ok {
		return "", ""
	}
	cred, _, _ = strings.Cut(cred, ",")
	parts := strings.Split(cred, "/")
	if len(parts) < 5 {
		return "", ""
	}
	return parts[2], parts[3]
}

func rwOperation(service string, req *http.Request, body []byte) (op, subject string) {
	if target := req.Header.Get("X-Amz-Target"); target != "" {
		_, op, _ = strings.Cut(target, ".")
		var m map[string]any
		_ = json.Unmarshal(body, &m) //nolint:errcheck // an empty body carries no subject
		for _, k := range []string{"ResourceArn", "WebACLArn", "CertificateArn", "Id", "logGroupNamePrefix", "Name"} {
			if s, ok := m[k].(string); ok && s != "" {
				return op, s
			}
		}
		if ids, ok := m["logGroupIdentifiers"].([]any); ok && len(ids) > 0 {
			s, _ := ids[0].(string)
			return op, s
		}
		return op, ""
	}
	if _, after, ok := strings.Cut(req.URL.Path, "/operation/"); ok {
		return after, ""
	}
	if service == "s3" {
		return rwS3Op(req), rwS3Bucket(req)
	}
	return req.Method + " " + req.URL.Path, req.URL.RawQuery
}

func rwResponse(status int, contentType string, body []byte, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}

func rwJSON(v any) *http.Response {
	b, _ := json.Marshal(v) //nolint:errcheck // test-built maps always marshal
	return rwResponse(200, "application/x-amz-json-1.1", b, nil)
}

func rwJSONError(status int, code, message string) *http.Response {
	b, _ := json.Marshal(map[string]string{"__type": code, "message": message}) //nolint:errcheck // test-built maps always marshal
	return rwResponse(status, "application/x-amz-json-1.1", b, http.Header{"X-Amzn-Errortype": []string{code}})
}

func rwDenied(service, op, method string) *http.Response {
	msg := "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: " + service + ":" + op
	switch service {
	case "s3":
		if method == http.MethodHead {
			return rwResponse(403, "", nil, nil)
		}
		return rwResponse(403, "application/xml", []byte(`<Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`), nil)
	case "monitoring":
		body := cbor.Encode(cbor.Map{"__type": cbor.String("AccessDenied"), "message": cbor.String(msg)})
		return rwResponse(403, "application/cbor", body, http.Header{
			"Smithy-Protocol":    []string{"rpc-v2-cbor"},
			"X-Amzn-Query-Error": []string{"AccessDenied;Sender"},
		})
	}
	return rwJSONError(400, "AccessDeniedException", msg)
}

// ─── WAFv2 ─────────────────────────────────────────────────────────────────

type rwACL struct {
	name, id, arn, scope string
	rules                []bool // true = Block action
	logDest              []string
	albs                 []string
}

var rwACLs = map[string][]rwACL{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {
		{name: "acme-edge-acl", id: rwEdgeACLID, arn: rwEdgeACLArn, scope: "CLOUDFRONT", rules: []bool{true, false}, logDest: []string{rwEdgeLogArn}},
		{name: "acme-legacy-edge-acl", id: rwLegacyACLID, arn: rwLegacyACLArn, scope: "CLOUDFRONT", rules: []bool{true}, logDest: []string{rwEdgeLogArn}},
	},
	"eu-west-1": {{name: "acme-api-acl", id: rwAPIACLID, arn: rwAPIACLArn, scope: "REGIONAL", rules: []bool{true, false, false}, albs: []string{rwAPIALBArn}}},
}

func rwWAF(region, op string, body []byte) *http.Response {
	var in map[string]any
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no input
	str := func(k string) string { s, _ := in[k].(string); return s }
	find := func(match func(rwACL) bool) (rwACL, bool) {
		for _, a := range rwACLs[region] {
			if match(a) {
				return a, true
			}
		}
		return rwACL{}, false
	}
	missing := rwJSONError(400, "WAFNonexistentItemException", "AWS WAF couldn’t perform the operation because your resource doesn't exist.")

	switch op {
	case "ListWebACLs":
		scope := str("Scope")
		if scope == "CLOUDFRONT" && region != "us-east-1" {
			return rwJSONError(400, "WAFInvalidParameterException", "The scope CLOUDFRONT is only valid in us-east-1.")
		}
		acls := []map[string]any{}
		for _, a := range rwACLs[region] {
			if a.scope == scope {
				acls = append(acls, map[string]any{"Name": a.name, "Id": a.id, "ARN": a.arn, "Description": a.name + " rules", "LockToken": "0a1b2c3d-0000-4000-8000-00000000000" + a.id[:1]})
			}
		}
		return rwJSON(map[string]any{"WebACLs": acls})
	case "GetLoggingConfiguration":
		a, ok := find(func(a rwACL) bool { return a.arn == str("ResourceArn") })
		if !ok || len(a.logDest) == 0 {
			return missing
		}
		return rwJSON(map[string]any{"LoggingConfiguration": map[string]any{"ResourceArn": a.arn, "LogDestinationConfigs": a.logDest}})
	case "ListResourcesForWebACL":
		a, ok := find(func(a rwACL) bool { return a.arn == str("WebACLArn") })
		if !ok {
			return missing
		}
		arns := []string{}
		if rt := str("ResourceType"); rt == "" || rt == "APPLICATION_LOAD_BALANCER" {
			arns = append(arns, a.albs...)
		}
		return rwJSON(map[string]any{"ResourceArns": arns})
	case "GetWebACL":
		a, ok := find(func(a rwACL) bool { return a.id == str("Id") && a.name == str("Name") && a.scope == str("Scope") })
		if !ok {
			return missing
		}
		rules := []map[string]any{}
		for i, block := range a.rules {
			action := map[string]any{"Count": map[string]any{}}
			if block {
				action = map[string]any{"Block": map[string]any{}}
			}
			rules = append(rules, map[string]any{
				"Name":             fmt.Sprintf("%s-rule-%d", a.name, i),
				"Priority":         i,
				"Action":           action,
				"VisibilityConfig": map[string]any{"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": fmt.Sprintf("rule%d", i)},
			})
		}
		return rwJSON(map[string]any{"WebACL": map[string]any{
			"Name": a.name, "Id": a.id, "ARN": a.arn,
			"DefaultAction":    map[string]any{"Allow": map[string]any{}},
			"Rules":            rules,
			"VisibilityConfig": map[string]any{"SampledRequestsEnabled": true, "CloudWatchMetricsEnabled": true, "MetricName": a.name},
		}, "LockToken": "0a1b2c3d-0000-4000-8000-000000000001"})
	}
	return rwJSONError(400, "WAFInvalidOperationException", "wafv2 "+op+" is not modelled by the region test world")
}

// ─── ACM ───────────────────────────────────────────────────────────────────

type rwCert struct{ arn, domain string }

var rwCerts = map[string][]rwCert{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {{arn: rwEdgeCertArn, domain: "www.acme-example.com"}},
	"eu-west-1": {{arn: rwAPICertArn, domain: "api.acme-example.com"}},
}

func rwACM(region, op string, body []byte) *http.Response {
	notAfter := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	switch op {
	case "ListCertificates":
		list := []map[string]any{}
		for _, c := range rwCerts[region] {
			list = append(list, map[string]any{
				"CertificateArn": c.arn, "DomainName": c.domain, "Status": "ISSUED", "Type": "AMAZON_ISSUED",
				"InUse": true, "NotAfter": notAfter, "KeyAlgorithm": "RSA-2048",
			})
		}
		return rwJSON(map[string]any{"CertificateSummaryList": list})
	case "DescribeCertificate":
		var in struct{ CertificateArn string }
		_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body finds nothing
		for _, c := range rwCerts[region] {
			if c.arn == in.CertificateArn {
				return rwJSON(map[string]any{"Certificate": map[string]any{
					"CertificateArn": c.arn, "DomainName": c.domain, "Status": "ISSUED", "Type": "AMAZON_ISSUED",
					"NotAfter": notAfter, "KeyAlgorithm": "RSA-2048",
					"InUseBy": []string{"arn:aws:cloudfront::123456789012:distribution/" + rwDistID},
				}})
			}
		}
		return rwJSONError(400, "ResourceNotFoundException", "Could not find certificate "+in.CertificateArn+".")
	}
	return rwJSONError(400, "ValidationException", "acm "+op+" is not modelled by the region test world")
}

// ─── CloudWatch Logs ───────────────────────────────────────────────────────

var rwLogGroups = map[string][]string{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {rwZoneLogGroup, "aws-waf-logs-acme-edge"},
	"us-west-2": {rwOrgTrailLogGroup},
	"eu-west-1": {"/aws/lambda/acme-api", rwEUTrailLogGroup},
}

func rwLogs(region, op string, body []byte) *http.Response {
	if op != "DescribeLogGroups" {
		return rwJSONError(400, "InvalidOperationException", "logs "+op+" is not modelled by the region test world")
	}
	var in struct {
		LogGroupNamePrefix  string   `json:"logGroupNamePrefix"`
		LogGroupIdentifiers []string `json:"logGroupIdentifiers"`
	}
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body lists everything
	groups := []map[string]any{}
	for _, name := range rwLogGroups[region] {
		arn := "arn:aws:logs:" + region + ":123456789012:log-group:" + name
		if in.LogGroupNamePrefix != "" && !strings.HasPrefix(name, in.LogGroupNamePrefix) {
			continue
		}
		if len(in.LogGroupIdentifiers) > 0 {
			hit := false
			for _, id := range in.LogGroupIdentifiers {
				if id == name || strings.TrimSuffix(id, ":*") == arn {
					hit = true
				}
			}
			if !hit {
				continue
			}
		}
		groups = append(groups, map[string]any{
			"logGroupName": name, "arn": arn + ":*", "logGroupArn": arn,
			"creationTime": int64(1767225600000), "retentionInDays": 365, "storedBytes": 2048,
		})
	}
	return rwJSON(map[string]any{"logGroups": groups})
}

// ─── CloudWatch alarms (rpc-v2-cbor) ───────────────────────────────────────

type rwAlarm struct{ name, namespace, metric, dimName, dimValue string }

var rwAlarmsByRegion = map[string][]rwAlarm{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {{name: rwEdgeAlarm, namespace: "AWS/CloudFront", metric: "5xxErrorRate", dimName: "DistributionId", dimValue: rwDistID}},
	"eu-west-1": {{name: rwAPIAlarm, namespace: "AWS/ApplicationELB", metric: "HTTPCode_ELB_5XX_Count", dimName: "LoadBalancer", dimValue: "app/acme-api-alb/50dc6c495c0c9188"}},
}

func rwAlarms(region, op string) *http.Response {
	header := http.Header{"Smithy-Protocol": []string{"rpc-v2-cbor"}}
	if op != "DescribeAlarms" && op != "DescribeAlarmsForMetric" {
		body := cbor.Encode(cbor.Map{"__type": cbor.String("InvalidAction"), "message": cbor.String("monitoring " + op + " is not modelled by the region test world")})
		return rwResponse(400, "application/cbor", body, header)
	}
	list := cbor.List{}
	for _, a := range rwAlarmsByRegion[region] {
		dims := cbor.List{cbor.Map{"Name": cbor.String(a.dimName), "Value": cbor.String(a.dimValue)}}
		if a.namespace == "AWS/CloudFront" {
			dims = append(dims, cbor.Map{"Name": cbor.String("Region"), "Value": cbor.String("Global")})
		}
		list = append(list, cbor.Map{
			"AlarmName":          cbor.String(a.name),
			"AlarmArn":           cbor.String("arn:aws:cloudwatch:" + region + ":123456789012:alarm:" + a.name),
			"Namespace":          cbor.String(a.namespace),
			"MetricName":         cbor.String(a.metric),
			"Dimensions":         dims,
			"StateValue":         cbor.String("OK"),
			"ActionsEnabled":     cbor.Bool(true),
			"AlarmActions":       cbor.List{cbor.String("arn:aws:sns:" + region + ":123456789012:acme-oncall")},
			"Statistic":          cbor.String("Average"),
			"Period":             cbor.Uint(300),
			"EvaluationPeriods":  cbor.Uint(1),
			"Threshold":          cbor.Float64(5),
			"ComparisonOperator": cbor.String("GreaterThanThreshold"),
		})
	}
	return rwResponse(200, "application/cbor", cbor.Encode(cbor.Map{"MetricAlarms": list, "CompositeAlarms": cbor.List{}}), header)
}

// ─── CloudTrail ────────────────────────────────────────────────────────────

func rwTrails(region, op string) *http.Response {
	switch op {
	case "DescribeTrails":
		trails := []map[string]any{{
			"Name":                       rwOrgTrail,
			"TrailARN":                   "arn:aws:cloudtrail:us-west-2:123456789012:trail/" + rwOrgTrail,
			"HomeRegion":                 "us-west-2",
			"IsMultiRegionTrail":         true,
			"IsOrganizationTrail":        false,
			"IncludeGlobalServiceEvents": true,
			"LogFileValidationEnabled":   true,
			"S3BucketName":               rwTrailBucket,
			"KmsKeyId":                   "arn:aws:kms:us-west-2:123456789012:key/" + rwOrgTrailKMSKeyID,
			"SnsTopicName":               "acme-org-trail-alerts",
			"SnsTopicARN":                rwOrgTrailTopic,
			"CloudWatchLogsLogGroupArn":  "arn:aws:logs:us-west-2:123456789012:log-group:" + rwOrgTrailLogGroup + ":*",
			"CloudWatchLogsRoleArn":      "arn:aws:iam::123456789012:role/acme-cloudtrail-to-logs",
		}}
		if region == "eu-west-1" {
			trails = append(trails, map[string]any{
				"Name":                      rwEUTrail,
				"TrailARN":                  "arn:aws:cloudtrail:eu-west-1:123456789012:trail/" + rwEUTrail,
				"HomeRegion":                "eu-west-1",
				"IsMultiRegionTrail":        false,
				"LogFileValidationEnabled":  true,
				"S3BucketName":              rwTrailBucket,
				"KmsKeyId":                  "arn:aws:kms:eu-west-1:123456789012:key/" + rwEUTrailKMSKeyID,
				"CloudWatchLogsLogGroupArn": "arn:aws:logs:eu-west-1:123456789012:log-group:" + rwEUTrailLogGroup + ":*",
				"CloudWatchLogsRoleArn":     "arn:aws:iam::123456789012:role/acme-cloudtrail-to-logs",
			})
		}
		return rwJSON(map[string]any{"trailList": trails})
	case "GetTrailStatus":
		return rwJSON(map[string]any{"IsLogging": true, "LatestDeliveryTime": time.Now().Add(-5 * time.Minute).Unix()})
	}
	return rwJSONError(400, "UnsupportedOperationException", "cloudtrail "+op+" is not modelled by the region test world")
}

// ─── KMS ───────────────────────────────────────────────────────────────────

// The same infrastructure code deployed per Region gives each Region its own
// key under the same alias name: an alias names a key only inside its Region.
var rwKMSKeys = map[string][]struct{ id, alias string }{ //nolint:gochecknoglobals // read-only fixture world
	"us-east-1": {{rwUSAssetsKeyID, rwAssetsAlias}},
	"eu-west-1": {{rwEUAssetsKeyID, rwAssetsAlias}, {rwEUTrailKMSKeyID, "alias/acme-eu-trail"}},
	"us-west-2": {{rwOrgTrailKMSKeyID, "alias/acme-org-trail"}},
}

func rwKMS(region, op string, body []byte) *http.Response {
	var in struct{ KeyId string }
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body names no key
	prefix := "arn:aws:kms:" + region + ":123456789012:"
	switch op {
	case "ListKeys":
		keys := []map[string]any{}
		for _, k := range rwKMSKeys[region] {
			keys = append(keys, map[string]any{"KeyId": k.id, "KeyArn": prefix + "key/" + k.id})
		}
		return rwJSON(map[string]any{"Keys": keys, "Truncated": false})
	case "ListAliases":
		aliases := []map[string]any{}
		for _, k := range rwKMSKeys[region] {
			if in.KeyId == "" || in.KeyId == k.id {
				aliases = append(aliases, map[string]any{"AliasName": k.alias, "AliasArn": prefix + k.alias, "TargetKeyId": k.id})
			}
		}
		return rwJSON(map[string]any{"Aliases": aliases, "Truncated": false})
	case "DescribeKey":
		for _, k := range rwKMSKeys[region] {
			if in.KeyId == k.id || in.KeyId == prefix+"key/"+k.id || in.KeyId == k.alias || in.KeyId == prefix+k.alias {
				return rwJSON(map[string]any{"KeyMetadata": map[string]any{
					"AWSAccountId": "123456789012", "KeyId": k.id, "Arn": prefix + "key/" + k.id,
					"CreationDate": 1767225600, "Enabled": true, "KeyState": "Enabled", "KeyManager": "CUSTOMER",
					"KeyUsage": "ENCRYPT_DECRYPT", "KeySpec": "SYMMETRIC_DEFAULT", "Origin": "AWS_KMS", "MultiRegion": false,
				}})
			}
		}
		return rwJSONError(400, "NotFoundException", "Key '"+in.KeyId+"' does not exist")
	}
	return rwJSONError(400, "UnsupportedOperationException", "kms "+op+" is not modelled by the region test world")
}

// ─── SNS (awsquery) ────────────────────────────────────────────────────────

var rwTopics = map[string][]string{ //nolint:gochecknoglobals // read-only fixture world
	"us-west-2": {rwOrgTrailTopic},
	"eu-west-1": {"arn:aws:sns:eu-west-1:123456789012:acme-eu-alerts"},
}

func rwSNS(region string, body []byte) *http.Response {
	form, _ := url.ParseQuery(string(body)) //nolint:errcheck // a malformed body names no action
	if action := form.Get("Action"); action != "ListTopics" {
		return rwResponse(400, "text/xml", []byte(`<ErrorResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><Error><Type>Sender</Type><Code>InvalidAction</Code><Message>sns `+action+` is not modelled by the region test world</Message></Error><RequestId>7a62c49f-347e-4fc4-9331-6e8eEXAMPLE</RequestId></ErrorResponse>`), nil)
	}
	var b strings.Builder
	for _, arn := range rwTopics[region] {
		b.WriteString(`<member><TopicArn>` + arn + `</TopicArn></member>`)
	}
	return rwResponse(200, "text/xml", []byte(`<ListTopicsResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><ListTopicsResult><Topics>`+b.String()+
		`</Topics></ListTopicsResult><ResponseMetadata><RequestId>3f1478c7-33a9-11df-9540-99d0768312d3</RequestId></ResponseMetadata></ListTopicsResponse>`), nil)
}

// ─── Route 53 and CloudFront (global, signed for us-east-1) ─────────────────

func rwRoute53(op string, req *http.Request) *http.Response {
	if !strings.HasSuffix(req.URL.Path, "/queryloggingconfig") {
		return rwResponse(400, "text/xml", []byte(`<ErrorResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><Error><Type>Sender</Type><Code>InvalidInput</Code><Message>route53 `+op+` is not modelled by the region test world</Message></Error></ErrorResponse>`), nil)
	}
	configs := ""
	if strings.Contains(req.URL.Query().Get("hostedzoneid"), strings.TrimPrefix(rwZoneID, "/hostedzone/")) {
		configs = `<QueryLoggingConfig><Id>87654321-dcba-4321-abcd-0123456789ab</Id><HostedZoneId>` + strings.TrimPrefix(rwZoneID, "/hostedzone/") +
			`</HostedZoneId><CloudWatchLogsLogGroupArn>arn:aws:logs:us-east-1:123456789012:log-group:` + rwZoneLogGroup + `:*</CloudWatchLogsLogGroupArn></QueryLoggingConfig>`
	}
	return rwResponse(200, "text/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListQueryLoggingConfigsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><QueryLoggingConfigs>`+configs+`</QueryLoggingConfigs></ListQueryLoggingConfigsResponse>`), nil)
}

func rwCloudFront(req *http.Request) *http.Response {
	path, _ := url.PathUnescape(req.URL.Path) //nolint:errcheck // a malformed path matches nothing
	items, quantity := "", 0
	if strings.Contains(path, "/distributionsByWebACLId/") && strings.Contains(path, rwEdgeACLID) {
		items = `<Items><DistributionSummary><Id>` + rwDistID + `</Id><ARN>arn:aws:cloudfront::123456789012:distribution/` + rwDistID +
			`</ARN><Status>Deployed</Status><DomainName>d111111abcdef8.cloudfront.net</DomainName><Enabled>true</Enabled><WebACLId>` + rwEdgeACLArn +
			`</WebACLId></DistributionSummary></Items>`
		quantity = 1
	}
	if !strings.Contains(path, "/distributionsByWebACLId/") {
		return rwResponse(400, "text/xml", []byte(`<ErrorResponse><Error><Type>Sender</Type><Code>InvalidArgument</Code><Message>cloudfront `+req.URL.Path+` is not modelled by the region test world</Message></Error></ErrorResponse>`), nil)
	}
	return rwResponse(200, "text/xml", []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<DistributionList xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Marker></Marker><MaxItems>100</MaxItems><IsTruncated>false</IsTruncated><Quantity>%d</Quantity>%s</DistributionList>`, quantity, items)), nil)
}

// ─── S3 ────────────────────────────────────────────────────────────────────

type rwBucket struct {
	region      string
	versioning  string // inner XML of VersioningConfiguration
	loggingTo   string
	policy      string
	cors        bool
	kmsKey      string
	notifyQueue string
}

var rwBuckets = map[string]rwBucket{ //nolint:gochecknoglobals // read-only fixture world
	rwAssetsBucket: {
		region:      "eu-west-1",
		versioning:  "<Status>Enabled</Status><MfaDelete>Disabled</MfaDelete>",
		loggingTo:   "acme-access-logs-eu",
		policy:      `{"Version":"2012-10-17","Statement":[{"Sid":"AllowCloudFrontOAC","Effect":"Allow","Principal":{"Service":"cloudfront.amazonaws.com"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-assets-eu/*","Condition":{"StringEquals":{"AWS:SourceArn":"arn:aws:cloudfront::123456789012:distribution/E2EXAMPLE1ACME"}}}]}`,
		cors:        true,
		kmsKey:      rwAssetsKMSKey,
		notifyQueue: rwAssetsQueue,
	},
	rwWebBucket: {
		region:    "us-east-1",
		loggingTo: "acme-access-logs-use1",
	},
	rwTrailBucket: {
		region: "us-west-2",
	},
	rwReportsBucket: {
		region: "eu-west-1",
		kmsKey: "arn:aws:kms:eu-west-1:123456789012:" + rwAssetsAlias,
	},
}

func rwS3Bucket(req *http.Request) string {
	host := req.URL.Host
	if i := strings.Index(host, ".s3."); i > 0 {
		return host[:i]
	}
	if i := strings.Index(host, ".s3-"); i > 0 {
		return host[:i]
	}
	b, _, _ := strings.Cut(strings.TrimPrefix(req.URL.Path, "/"), "/")
	return b
}

func rwS3Op(req *http.Request) string {
	if req.Method == http.MethodHead {
		return "HeadBucket"
	}
	q := req.URL.Query()
	for _, k := range []string{"publicAccessBlock", "policyStatus", "acl", "versioning", "logging", "lifecycle", "object-lock", "notification", "encryption", "tagging", "policy", "cors", "location"} {
		if q.Has(k) {
			return k
		}
	}
	if q.Get("list-type") == "2" {
		return "ListObjectsV2"
	}
	if rwS3Bucket(req) == "" {
		return "ListBuckets"
	}
	return req.Method + " " + req.URL.Path
}

const rwS3NS = `xmlns="http://s3.amazonaws.com/doc/2006-03-01/"`

func rwS3XML(inner string) *http.Response {
	return rwResponse(200, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+inner), nil)
}

func rwS3Error(status int, code, message, bucket string) *http.Response {
	return rwResponse(status, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>`+code+`</Code><Message>`+message+`</Message><BucketName>`+bucket+`</BucketName><RequestId>4442587FB7D0A2F9</RequestId></Error>`), nil)
}

func rwS3(region, op, name string, req *http.Request) *http.Response {
	if op == "ListBuckets" {
		var b strings.Builder
		for _, n := range []string{rwAssetsBucket, rwReportsBucket, rwTrailBucket, rwWebBucket} {
			fmt.Fprintf(&b, `<Bucket><Name>%s</Name><CreationDate>2025-03-01T09:30:00.000Z</CreationDate><BucketRegion>%s</BucketRegion></Bucket>`, n, rwBuckets[n].region)
		}
		return rwS3XML(`<ListAllMyBucketsResult ` + rwS3NS + `><Owner><ID>79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be</ID></Owner><Buckets>` + b.String() + `</Buckets></ListAllMyBucketsResult>`)
	}
	bucket, ok := rwBuckets[name]
	if !ok {
		if op == "HeadBucket" {
			return rwResponse(404, "", nil, nil)
		}
		return rwS3Error(404, "NoSuchBucket", "The specified bucket does not exist", name)
	}
	regionHeader := http.Header{"X-Amz-Bucket-Region": []string{bucket.region}}
	if op == "location" {
		constraint := bucket.region
		if constraint == "us-east-1" {
			constraint = ""
		}
		return rwS3XML(`<LocationConstraint ` + rwS3NS + `>` + constraint + `</LocationConstraint>`)
	}
	if bucket.region != region {
		if op == "HeadBucket" {
			return rwResponse(301, "", nil, regionHeader)
		}
		resp := rwS3Error(301, "PermanentRedirect", "The bucket you are attempting to access must be addressed using the specified endpoint. Please send all future requests to this endpoint.", name)
		resp.Header.Set("X-Amz-Bucket-Region", bucket.region)
		return resp
	}

	switch op {
	case "HeadBucket":
		return rwResponse(200, "", nil, regionHeader)
	case "publicAccessBlock":
		return rwS3XML(`<PublicAccessBlockConfiguration ` + rwS3NS + `><BlockPublicAcls>true</BlockPublicAcls><IgnorePublicAcls>true</IgnorePublicAcls><BlockPublicPolicy>true</BlockPublicPolicy><RestrictPublicBuckets>true</RestrictPublicBuckets></PublicAccessBlockConfiguration>`)
	case "policyStatus":
		return rwS3XML(`<PolicyStatus ` + rwS3NS + `><IsPublic>false</IsPublic></PolicyStatus>`)
	case "acl":
		return rwS3XML(`<AccessControlPolicy ` + rwS3NS + `><Owner><ID>79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be</ID></Owner><AccessControlList><Grant><Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="CanonicalUser"><ID>79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2be</ID></Grantee><Permission>FULL_CONTROL</Permission></Grant></AccessControlList></AccessControlPolicy>`)
	case "versioning":
		return rwS3XML(`<VersioningConfiguration ` + rwS3NS + `>` + bucket.versioning + `</VersioningConfiguration>`)
	case "logging":
		inner := ""
		if bucket.loggingTo != "" {
			inner = `<LoggingEnabled><TargetBucket>` + bucket.loggingTo + `</TargetBucket><TargetPrefix>` + name + `/</TargetPrefix></LoggingEnabled>`
		}
		return rwS3XML(`<BucketLoggingStatus ` + rwS3NS + `>` + inner + `</BucketLoggingStatus>`)
	case "lifecycle":
		return rwS3XML(`<LifecycleConfiguration ` + rwS3NS + `><Rule><ID>expire-tmp</ID><Filter><Prefix>tmp/</Prefix></Filter><Status>Enabled</Status><Expiration><Days>30</Days></Expiration></Rule></LifecycleConfiguration>`)
	case "object-lock":
		return rwS3Error(404, "ObjectLockConfigurationNotFoundError", "Object Lock configuration does not exist for this bucket", name)
	case "notification":
		inner := ""
		if bucket.notifyQueue != "" {
			inner = `<QueueConfiguration><Id>ingest-new-objects</Id><Queue>` + bucket.notifyQueue + `</Queue><Event>s3:ObjectCreated:*</Event></QueueConfiguration>`
		}
		return rwS3XML(`<NotificationConfiguration ` + rwS3NS + `>` + inner + `</NotificationConfiguration>`)
	case "encryption":
		sse := `<SSEAlgorithm>AES256</SSEAlgorithm>`
		if bucket.kmsKey != "" {
			sse = `<SSEAlgorithm>aws:kms</SSEAlgorithm><KMSMasterKeyID>` + bucket.kmsKey + `</KMSMasterKeyID>`
		}
		return rwS3XML(`<ServerSideEncryptionConfiguration ` + rwS3NS + `><Rule><ApplyServerSideEncryptionByDefault>` + sse + `</ApplyServerSideEncryptionByDefault><BucketKeyEnabled>true</BucketKeyEnabled></Rule></ServerSideEncryptionConfiguration>`)
	case "tagging":
		return rwS3Error(404, "NoSuchTagSet", "The TagSet does not exist", name)
	case "policy":
		if bucket.policy == "" {
			return rwS3Error(404, "NoSuchBucketPolicy", "The bucket policy does not exist", name)
		}
		return rwResponse(200, "application/json", []byte(bucket.policy), nil)
	case "cors":
		if !bucket.cors {
			return rwS3Error(404, "NoSuchCORSConfiguration", "The CORS configuration does not exist", name)
		}
		return rwS3XML(`<CORSConfiguration ` + rwS3NS + `><CORSRule><AllowedMethod>GET</AllowedMethod><AllowedOrigin>https://www.acme-example.com</AllowedOrigin><MaxAgeSeconds>3000</MaxAgeSeconds></CORSRule></CORSConfiguration>`)
	case "ListObjectsV2":
		return rwS3XML(`<ListBucketResult ` + rwS3NS + `><Name>` + name + `</Name><Prefix></Prefix><KeyCount>2</KeyCount><MaxKeys>1000</MaxKeys><Delimiter>/</Delimiter><IsTruncated>false</IsTruncated>` +
			`<Contents><Key>index.html</Key><LastModified>2026-09-01T10:00:00.000Z</LastModified><ETag>&quot;9b2cf535f27731c974343645a3985328&quot;</ETag><Size>2048</Size><StorageClass>STANDARD</StorageClass></Contents>` +
			`<CommonPrefixes><Prefix>css/</Prefix></CommonPrefixes></ListBucketResult>`)
	}
	return rwS3Error(400, "NotImplemented", "s3 "+op+" is not modelled by the region test world", name)
}

// ─── helpers over the real catalog ─────────────────────────────────────────

func rwFetch(t *testing.T, clients *awsclient.ServiceClients, shortName string) []resource.Resource {
	t.Helper()
	pf := resource.GetPaginatedFetcher(shortName)
	if pf == nil {
		t.Fatalf("no paginated fetcher registered for %q", shortName)
	}
	res, err := pf(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("%s fetcher: %v", shortName, err)
	}
	return res.Resources
}

func rwByID(t *testing.T, rs []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rs {
		if r.ID == id {
			return r
		}
	}
	ids := make([]string, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
	}
	t.Fatalf("no resource %q among %v", id, ids)
	return resource.Resource{}
}

func rwChecker(t *testing.T, shortName, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(shortName) {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("%s has no related checker for %q", shortName, target)
	return nil
}
