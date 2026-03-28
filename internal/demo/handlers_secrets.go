package demo

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// registerSecretsExtHandlers registers SSM and KMS handlers.
func registerSecretsExtHandlers(t *Transport) {
	registerSSMHandlers(t)
	registerKMSHandlers(t)
}

// ---------------------------------------------------------------------------
// SSM Parameter Store (awsjson11)
// ---------------------------------------------------------------------------

func registerSSMHandlers(t *Transport) {
	t.Handle("ssm", "DescribeParameters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ssm"]()
		params := ExtractSDK[ssmtypes.ParameterMetadata](resources)

		out := &ssm.DescribeParametersOutput{
			Parameters: params,
		}
		return JSONResponse(out)
	})

	// GetParameter — return a demo parameter value for reveal (x key).
	t.Handle("ssm", "GetParameter", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		paramName, _ := body["Name"].(string)
		if paramName == "" {
			paramName = "/app/demo/parameter"
		}

		paramType := ssmtypes.ParameterTypeString
		// Check if the requested parameter is a SecureString in our fixtures.
		resources := demoData["ssm"]()
		metas := ExtractSDK[ssmtypes.ParameterMetadata](resources)
		for _, meta := range metas {
			if meta.Name != nil && *meta.Name == paramName {
				if meta.Type == ssmtypes.ParameterTypeSecureString {
					paramType = ssmtypes.ParameterTypeSecureString
				}
				break
			}
		}

		demoValue := "demo-value-for-" + paramName
		out := &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Name:  &paramName,
				Type:  paramType,
				Value: &demoValue,
			},
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// KMS (awsjson11)
// ---------------------------------------------------------------------------

func registerKMSHandlers(t *Transport) {
	// ListKeys — return key list entries from KeyMetadata fixtures
	t.Handle("kms", "ListKeys", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["kms"]()
		metas := ExtractSDK[*kmstypes.KeyMetadata](resources)

		entries := make([]kmstypes.KeyListEntry, 0, len(metas))
		for _, meta := range metas {
			if meta == nil || meta.KeyId == nil {
				continue
			}
			entries = append(entries, kmstypes.KeyListEntry{
				KeyId:  meta.KeyId,
				KeyArn: meta.Arn,
			})
		}

		out := &kms.ListKeysOutput{
			Keys:      entries,
			Truncated: false,
		}
		return JSONResponse(out)
	})

	// ListAliases — build alias entries from KeyMetadata
	t.Handle("kms", "ListAliases", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["kms"]()
		metas := ExtractSDK[*kmstypes.KeyMetadata](resources)

		aliases := make([]kmstypes.AliasListEntry, 0, len(metas))
		for i, meta := range metas {
			if meta == nil || meta.KeyId == nil {
				continue
			}
			// Build a demo alias name from key ID
			aliasName := "alias/demo-key-" + string(rune('1'+i))
			aliases = append(aliases, kmstypes.AliasListEntry{
				AliasName:   &aliasName,
				TargetKeyId: meta.KeyId,
				AliasArn:    meta.Arn,
			})
		}

		out := &kms.ListAliasesOutput{
			Aliases:   aliases,
			Truncated: false,
		}
		return JSONResponse(out)
	})

	// DescribeKey — return metadata for the requested key ID or ARN.
	t.Handle("kms", "DescribeKey", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		requestedKeyID, _ := body["KeyId"].(string)

		resources := demoData["kms"]()
		metas := ExtractSDK[*kmstypes.KeyMetadata](resources)

		var meta *kmstypes.KeyMetadata
		for _, m := range metas {
			if m == nil {
				continue
			}
			if requestedKeyID != "" {
				if strings.EqualFold(aws.ToString(m.KeyId), requestedKeyID) ||
					strings.EqualFold(aws.ToString(m.Arn), requestedKeyID) {
					meta = m
					break
				}
			}
		}
		// Fall back to first key if no match found
		if meta == nil && len(metas) > 0 {
			meta = metas[0]
		}

		out := &kms.DescribeKeyOutput{
			KeyMetadata: meta,
		}
		return JSONResponse(out)
	})
}
