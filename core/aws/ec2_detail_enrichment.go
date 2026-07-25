// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// maxUserDataDecompressedSize bounds gunzip output for cloud-init user data.
// Raw (pre-compression) user data caps at 16 KiB per the EC2 API itself, but
// decompression of an adversarial or corrupt payload must still be bounded.
const maxUserDataDecompressedSize = 1 << 20 // 1 MiB

// gunzipUserData decompresses gzip-magic-prefixed user data, returning an
// explicit error when the decompressed size exceeds
// maxUserDataDecompressedSize rather than silently truncating and presenting
// partial content as complete: reads one byte past the cap via io.LimitReader
// so a still-full reader after that read means the true size is unknown but
// at least cap+1, definitively over the limit.
func gunzipUserData(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close() //nolint:errcheck // read-only decompression, nothing to flush
	out, err := io.ReadAll(io.LimitReader(r, maxUserDataDecompressedSize+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxUserDataDecompressedSize {
		return nil, fmt.Errorf("decompressed user data exceeds %d bytes", maxUserDataDecompressedSize)
	}
	return out, nil
}

// decodeUserData runs the DescribeInstanceAttribute UserData decode chain:
// base64 (failure attaches the raw value as-is), then gunzip when the
// decoded bytes carry the gzip magic (cloud-init commonly compresses user
// data; failure falls through with the un-gunzipped bytes), then a
// utf8.Valid gate — non-UTF-8 output attaches the original base64 raw value
// instead of mojibake.
func decodeUserData(raw string) string {
	decoded, decErr := base64.StdEncoding.DecodeString(raw)
	if decErr != nil {
		return raw
	}
	text := decoded
	if len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
		if gunzipped, gzErr := gunzipUserData(decoded); gzErr == nil {
			text = gunzipped
		}
	}
	if utf8.Valid(text) {
		return string(text)
	}
	return raw
}

// InstanceEnriched wraps ec2types.Instance with the decoded UserData
// DescribeInstanceAttribute returns — the list call (DescribeInstances)
// never carries it.
type InstanceEnriched struct {
	ec2types.Instance
	UserData string `json:"UserData,omitempty" yaml:"UserData,omitempty"`
}

// enrichEc2 fetches and decodes the instance's user-data script via
// DescribeInstanceAttribute (decodeUserData). Uncached: a single cheap call,
// and user data can be changed (via ModifyInstanceAttribute) while an
// instance is stopped — a background cache would go stale exactly when an
// operator is investigating a boot-script change.
func enrichEc2(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[ec2types.Instance, string]{
		unwrap: unwrapEnriched(func(w InstanceEnriched) ec2types.Instance { return w.Instance }),
		id: func(instance ec2types.Instance, _ resource.Resource) (string, error) {
			if instance.InstanceId == nil || *instance.InstanceId == "" {
				return "", fmt.Errorf("instance has no InstanceId")
			}
			return *instance.InstanceId, nil
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ ec2types.Instance, _ resource.Resource) (string, error) {
			api, ok := c.EC2.(EC2DescribeInstanceAttributeAPI)
			if !ok {
				return "", fmt.Errorf("EC2 client does not support DescribeInstanceAttribute")
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeInstanceAttributeOutput, error) {
				return api.DescribeInstanceAttribute(ctx, &ec2.DescribeInstanceAttributeInput{
					InstanceId: &id,
					Attribute:  ec2types.InstanceAttributeNameUserData,
				})
			})
			if err != nil {
				return "", err
			}
			if out.UserData == nil || out.UserData.Value == nil || *out.UserData.Value == "" {
				return "", nil
			}
			return decodeUserData(*out.UserData.Value), nil
		},
		wrap: func(instance ec2types.Instance, userData string) any {
			return InstanceEnriched{Instance: instance, UserData: userData}
		},
	})
}
