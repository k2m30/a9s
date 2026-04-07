package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	smithycbor "github.com/aws/smithy-go/encoding/cbor"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// JSONResponse creates an HTTP 200 response with a JSON-marshaled body.
// Content-Type: application/x-amz-json-1.1
// time.Time fields are serialized as epoch-seconds float64.
func JSONResponse(v interface{}) (*http.Response, error) {
	data, err := marshalAWSJSON(v)
	if err != nil {
		return nil, fmt.Errorf("demo: JSONResponse marshal: %w", err)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/x-amz-json-1.1"},
		},
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

// XMLResponse creates an HTTP 200 response with an XML body.
func XMLResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"text/xml"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

// Paginate returns a page of items using offset-based demo tokens.
// Token format: "demo:OFFSET" (e.g., "demo:20"). Empty token = start from 0.
// Returns the page slice and a nextToken (*string, nil if last page).
// Returns a non-nil error when the token cannot be parsed.
func Paginate[T any](items []T, pageSize int, token string) (page []T, nextToken *string, err error) {
	offset := 0
	if token != "" {
		if _, scanErr := fmt.Sscanf(token, "demo:%d", &offset); scanErr != nil {
			return nil, nil, fmt.Errorf("invalid pagination token %q: %w", token, scanErr)
		}
	}

	end := offset + pageSize
	if end >= len(items) {
		end = len(items)
		return items[offset:end], nil, nil
	}

	next := fmt.Sprintf("demo:%d", end)
	return items[offset:end], &next, nil
}

// ExtractSDK extracts RawStruct values of type T from resource.Resource slices.
func ExtractSDK[T any](resources []resource.Resource) []T {
	result := make([]T, 0, len(resources))
	for _, r := range resources {
		if v, ok := r.RawStruct.(T); ok {
			result = append(result, v)
		}
	}
	return result
}

// ec2QueryXML wraps items in a standard ec2query response envelope.
// action is the operation name (e.g., "DescribeVpcs"), listName is the XML element
// that holds the items (e.g., "vpcSet"), and items is the XML content inside listName.
func ec2QueryXML(action, listName, items string) string {
	return fmt.Sprintf(`<%sResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>demo-request-id</requestId><%s>%s</%s></%sResponse>`,
		action, listName, items, listName, action)
}

// awsQueryXML wraps result body in a standard awsquery response envelope.
// xmlns should be the service's XML namespace.
func awsQueryXML(action, xmlns, resultBody string) string {
	return fmt.Sprintf(`<%sResponse xmlns=%q><%sResult>%s</%sResult><ResponseMetadata><RequestId>demo-request-id</RequestId></ResponseMetadata></%sResponse>`,
		action, xmlns, action, resultBody, action, action)
}

// CBORResponse creates an HTTP 200 response with a CBOR-encoded body and the
// required smithy-protocol: rpc-v2-cbor header used by modern AWS SDK v2 clients
// that use the Smithy RPCv2 CBOR wire protocol (e.g., CloudWatch).
// The value v must be a smithycbor.Value (Map, List, String, etc.).
func CBORResponse(v smithycbor.Value) *http.Response {
	var body []byte
	if v != nil {
		body = smithycbor.Encode(v)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type":    []string{"application/cbor"},
			"Smithy-Protocol": []string{"rpc-v2-cbor"},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}
}

// marshalAWSJSON marshals v to JSON, converting any time.Time values to epoch-seconds float64.
func marshalAWSJSON(v interface{}) ([]byte, error) {
	converted := convertTimes(reflect.ValueOf(v))
	return json.Marshal(converted)
}

// JSONResponseCamelCase creates an HTTP 200 response with a JSON body where
// all struct field names are converted to camelCase (first letter lowercased).
// Use for services whose wire format uses camelCase (ECS, ECR, CW Logs,
// CodeBuild, CodePipeline, SFN, CodeArtifact).
func JSONResponseCamelCase(v interface{}) (*http.Response, error) {
	data, err := marshalAWSJSONCamelCase(v)
	if err != nil {
		return nil, fmt.Errorf("demo: JSONResponseCamelCase marshal: %w", err)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/x-amz-json-1.1"},
		},
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func marshalAWSJSONCamelCase(v interface{}) ([]byte, error) {
	converted := convertTimesCamelCase(reflect.ValueOf(v))
	return json.Marshal(converted)
}

// convertTimesCamelCase is identical to convertTimes but lowercases the first
// letter of each struct field name so the output matches the camelCase wire
// format expected by ECS, ECR, CW Logs, CodeBuild, CodePipeline, SFN, and
// CodeArtifact SDK deserializers.
func convertTimesCamelCase(v reflect.Value) interface{} {
	// Dereference pointer
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	timeType := reflect.TypeOf(time.Time{})

	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == timeType {
			t := v.Interface().(time.Time)
			return float64(t.Unix())
		}
		// Build a map for struct fields with camelCase keys
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			fieldVal := v.Field(i)
			// Get JSON tag name if present
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				parts := strings.Split(tag, ",")
				if parts[0] != "" && parts[0] != "-" {
					name = parts[0]
				}
			}
			// Convert first letter to lowercase (camelCase)
			if len(name) > 0 {
				name = strings.ToLower(name[:1]) + name[1:]
			}
			result[name] = convertTimesCamelCase(fieldVal)
		}
		return result

	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		result := make([]interface{}, v.Len())
		for i := range result {
			result[i] = convertTimesCamelCase(v.Index(i))
		}
		return result

	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		result := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			result[fmt.Sprintf("%v", key.Interface())] = convertTimesCamelCase(v.MapIndex(key))
		}
		return result

	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return convertTimesCamelCase(v.Elem())

	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}

// convertTimes recursively walks a value and converts time.Time to epoch-seconds float64.
func convertTimes(v reflect.Value) interface{} {
	// Dereference pointer
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	timeType := reflect.TypeOf(time.Time{})

	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == timeType {
			t := v.Interface().(time.Time)
			return float64(t.Unix())
		}
		// Build a map for struct fields
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			fieldVal := v.Field(i)
			// Get JSON tag name if present
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				parts := strings.Split(tag, ",")
				if parts[0] != "" && parts[0] != "-" {
					name = parts[0]
				}
			}
			result[name] = convertTimes(fieldVal)
		}
		return result

	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		result := make([]interface{}, v.Len())
		for i := range result {
			result[i] = convertTimes(v.Index(i))
		}
		return result

	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		result := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			result[fmt.Sprintf("%v", key.Interface())] = convertTimes(v.MapIndex(key))
		}
		return result

	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return convertTimes(v.Elem())

	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}
