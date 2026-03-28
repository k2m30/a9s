package demo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Handler processes a mock AWS API request.
type Handler func(req *http.Request) (*http.Response, error)

// Transport implements http.RoundTripper for intercepting AWS SDK HTTP requests.
type Transport struct {
	handlers map[string]Handler // "service:Action" → handler
}

// NewTransport creates a Transport with no handlers registered yet.
func NewTransport() *Transport {
	return &Transport{
		handlers: make(map[string]Handler),
	}
}

// Handle registers a handler for a service + action pair.
func (t *Transport) Handle(service, action string, h Handler) {
	key := service + ":" + action
	t.handlers[key] = h
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	service := serviceFromHost(req.Host)
	if service == "" {
		service = serviceFromHost(req.URL.Host)
	}

	action, err := actionFromRequest(service, req)
	if err != nil {
		return errorResponse(500, "InternalError", err.Error()), nil
	}

	key := service + ":" + action
	h, ok := t.handlers[key]
	if !ok {
		return errorResponse(501, "NotImplemented",
			fmt.Sprintf("demo transport: no handler for %s", key)), nil
	}

	return h(req)
}

// serviceFromHost extracts the service name from an AWS endpoint host.
// e.g. "lambda.us-east-1.amazonaws.com" → "lambda"
// IAM is global: "iam.amazonaws.com" → "iam"
// S3 path-style: "s3.us-east-1.amazonaws.com" → "s3"
// ECR: "api.ecr.us-east-1.amazonaws.com" → "ecr"
func serviceFromHost(host string) string {
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Only strip if it looks like a port (all digits after last colon)
		port := host[idx+1:]
		allDigits := true
		for _, c := range port {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			host = host[:idx]
		}
	}

	// ECR uses a multi-segment prefix: "api.ecr.*"
	if strings.HasPrefix(host, "api.ecr.") {
		return "ecr"
	}

	// S3 virtual-hosted-style: "{bucket}.s3.{region}.amazonaws.com"
	if strings.Contains(host, ".s3.") {
		return "s3"
	}

	// Take the first segment before "."
	if idx := strings.Index(host, "."); idx != -1 {
		return host[:idx]
	}
	return host
}

// actionFromRequest identifies the AWS API action from the request.
func actionFromRequest(service string, req *http.Request) (string, error) {
	// Check X-Amz-Target header first (awsjson10/11 services)
	if target := req.Header.Get("X-Amz-Target"); target != "" {
		// Format: "ServicePrefix.OperationName"
		if idx := strings.LastIndex(target, "."); idx != -1 {
			return target[idx+1:], nil
		}
		return target, nil
	}

	// Smithy RPCv2 CBOR protocol (used by modern CloudWatch SDK v2):
	// URL path: /service/{ServiceName}/operation/{OperationName}
	// Content-Type: application/cbor
	if ct := req.Header.Get("Content-Type"); strings.Contains(ct, "application/cbor") {
		path := req.URL.Path
		if idx := strings.LastIndex(path, "/operation/"); idx != -1 {
			return path[idx+len("/operation/"):], nil
		}
	}

	// For EC2 and other query-protocol services, parse form body
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return "", fmt.Errorf("reading request body: %w", err)
		}
		// Restore body for handler to re-read if needed
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		contentType := req.Header.Get("Content-Type")
		if strings.Contains(contentType, "x-www-form-urlencoded") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			vals, err := url.ParseQuery(string(bodyBytes))
			if err == nil {
				if action := vals.Get("Action"); action != "" {
					return action, nil
				}
			}
		}
	}

	// REST services: route by path + method
	return actionFromRESTPath(service, req), nil
}

// actionFromRESTPath infers the action for REST-protocol services from URL path + HTTP method.
func actionFromRESTPath(service string, req *http.Request) string {
	path := req.URL.Path
	method := req.Method

	switch service {
	case "lambda":
		if method == "GET" && strings.HasPrefix(path, "/2015-03-31/functions") {
			return "ListFunctions"
		}
	case "s3":
		// ListObjectsV2: GET with list-type=2, prefix, or delimiter param.
		// Check BEFORE ListBuckets because virtual-hosted-style requests also use path "/"
		// (bucket is in the host: {bucket}.s3.{region}.amazonaws.com).
		if method == "GET" {
			if req.URL.Query().Get("list-type") == "2" || req.URL.Query().Get("prefix") != "" || req.URL.Query().Get("delimiter") != "" {
				return "ListObjectsV2"
			}
		}
		// ListBuckets is GET /
		if method == "GET" && (path == "/" || path == "") {
			return "ListBuckets"
		}
	case "eks":
		// GET /clusters → ListClusters
		if method == "GET" && path == "/clusters" {
			return "ListClusters"
		}
		// GET /clusters/{name} → DescribeCluster
		if method == "GET" && strings.HasPrefix(path, "/clusters/") {
			parts := strings.Split(strings.TrimPrefix(path, "/clusters/"), "/")
			if len(parts) == 1 {
				return "DescribeCluster"
			}
			// GET /clusters/{name}/node-groups → ListNodegroups
			if len(parts) == 2 && parts[1] == "node-groups" {
				return "ListNodegroups"
			}
			// GET /clusters/{name}/node-groups/{ngName} → DescribeNodegroup
			if len(parts) == 3 && parts[1] == "node-groups" {
				return "DescribeNodegroup"
			}
		}
	case "elasticfilesystem":
		if method == "GET" && strings.HasPrefix(path, "/2015-02-01/file-systems") {
			return "DescribeFileSystems"
		}
	case "email":
		if method == "GET" && strings.HasPrefix(path, "/v2/email/identities") {
			return "ListEmailIdentities"
		}
	case "kafka":
		if method == "GET" && strings.HasPrefix(path, "/api/v2/clusters") {
			return "ListClustersV2"
		}
	case "backup":
		if method == "GET" && strings.HasPrefix(path, "/backup/plans") {
			return "ListBackupPlans"
		}
	case "codeartifact":
		if strings.HasPrefix(path, "/v1/repositories") {
			return "ListRepositories"
		}
	case "es":
		// GET /2021-01-01/domain → ListDomainNames
		if method == "GET" && (path == "/2021-01-01/domain" || strings.HasSuffix(path, "/domain")) {
			return "ListDomainNames"
		}
		// POST /2021-01-01/opensearch/domain-info → DescribeDomains
		if method == "POST" && strings.Contains(path, "/domain-info") {
			return "DescribeDomains"
		}
	case "apigateway":
		if method == "GET" && strings.HasPrefix(path, "/v2/apis") {
			return "GetApis"
		}
	case "cloudfront":
		if method == "GET" && strings.Contains(path, "/distribution") {
			return "ListDistributions"
		}
	case "route53":
		// GET /2013-04-01/hostedzone → ListHostedZones
		if method == "GET" && (path == "/2013-04-01/hostedzone" || path == "/2013-04-01/hostedzone/") {
			return "ListHostedZones"
		}
		// GET /2013-04-01/hostedzone/{id}/rrset → ListResourceRecordSets
		if method == "GET" && strings.Contains(path, "/rrset") {
			return "ListResourceRecordSets"
		}
	case "ecr":
		// ECR uses X-Amz-Target so this path shouldn't normally be hit,
		// but return a fallback just in case
		return "Unknown"
	}
	return "Unknown"
}

// errorResponse creates a minimal error HTTP response.
func errorResponse(statusCode int, code, message string) *http.Response {
	body := fmt.Sprintf(`{"__type":%q,"message":%q}`, code, message)
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header: http.Header{
			"Content-Type": []string{"application/x-amz-json-1.1"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

// globalTransport is the package-level singleton transport with all handlers registered.
var globalTransport *Transport

func init() {
	globalTransport = NewTransport()
	registerAllHandlers(globalTransport)
}

// NewDemoAWSConfig creates an aws.Config that routes all SDK calls through the demo transport.
// Uses static fake credentials and us-east-1 region. No file I/O.
func NewDemoAWSConfig() aws.Config {
	return aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("demo", "demo", ""),
		HTTPClient:  &http.Client{Transport: globalTransport},
	}
}
