package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// registerStorageHandlers registers EFS, Backup, SES, and S3 object handlers.
func registerStorageHandlers(t *Transport) {
	registerEFSHandlers(t)
	registerBackupHandlers(t)
	registerSESHandlers(t)
	registerS3ObjectHandlers(t)
}

// ---------------------------------------------------------------------------
// EFS (restjson1, service "elasticfilesystem")
// ---------------------------------------------------------------------------

func registerEFSHandlers(t *Transport) {
	t.Handle("elasticfilesystem", "DescribeFileSystems", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["efs"]()
		fss := ExtractSDK[efstypes.FileSystemDescription](resources)

		out := &efs.DescribeFileSystemsOutput{
			FileSystems: fss,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// Backup (restjson1, service "backup")
// ---------------------------------------------------------------------------

func registerBackupHandlers(t *Transport) {
	t.Handle("backup", "ListBackupPlans", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["backup"]()
		plans := ExtractSDK[backuptypes.BackupPlansListMember](resources)

		out := &backup.ListBackupPlansOutput{
			BackupPlansList: plans,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// SES v2 (restjson1, service "email")
// ---------------------------------------------------------------------------

func registerSESHandlers(t *Transport) {
	t.Handle("email", "ListEmailIdentities", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ses"]()
		identities := ExtractSDK[sesv2types.IdentityInfo](resources)

		out := &sesv2.ListEmailIdentitiesOutput{
			EmailIdentities: identities,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// S3 ListObjectsV2 (restxml, service "s3")
// ---------------------------------------------------------------------------

func registerS3ObjectHandlers(t *Transport) {
	t.Handle("s3", "ListObjectsV2", func(req *http.Request) (*http.Response, error) {
		// Try virtual-hosted-style first: "{bucket}.s3.{region}.amazonaws.com"
		// In that case the path is "/" and the bucket is in the host.
		bucket := bucketFromVirtualHostedStyle(req.Host)
		if bucket == "" {
			bucket = bucketFromVirtualHostedStyle(req.URL.Host)
		}
		// Fall back to path-style: "/{bucket}?..."
		if bucket == "" {
			bucket = bucketFromPath(req.URL.Path)
		}
		prefix := req.URL.Query().Get("prefix")

		var objResources []s3types.Object
		var commonPrefixes []s3types.CommonPrefix

		s3resources, ok := GetS3Objects(bucket, prefix)
		if ok {
			for _, r := range s3resources {
				switch v := r.RawStruct.(type) {
				case s3types.Object:
					objResources = append(objResources, v)
				case s3types.CommonPrefix:
					commonPrefixes = append(commonPrefixes, v)
				}
			}
		}

		xml := buildListObjectsV2XML(bucket, prefix, objResources, commonPrefixes)
		return XMLResponse(xml), nil
	})
}

// bucketFromPath extracts the bucket name from an S3 path-style URL path.
// e.g. "/my-bucket?list-type=2" → "my-bucket"
func bucketFromPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	if idx := strings.Index(path, "/"); idx != -1 {
		return path[:idx]
	}
	return path
}

// bucketFromVirtualHostedStyle extracts the bucket name from a virtual-hosted-style S3 host.
// e.g. "my-bucket.s3.us-east-1.amazonaws.com" → "my-bucket"
// Returns "" if the host is not virtual-hosted-style.
func bucketFromVirtualHostedStyle(host string) string {
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
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
	// Match "{bucket}.s3.{anything}"
	if idx := strings.Index(host, ".s3."); idx != -1 {
		return host[:idx]
	}
	return ""
}

func buildListObjectsV2XML(bucket, prefix string, objects []s3types.Object, prefixes []s3types.CommonPrefix) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	fmt.Fprintf(&sb, `<Name>%s</Name>`, xmlEscape(bucket))
	fmt.Fprintf(&sb, `<Prefix>%s</Prefix>`, xmlEscape(prefix))
	sb.WriteString(`<Delimiter>/</Delimiter>`)
	fmt.Fprintf(&sb, `<KeyCount>%d</KeyCount>`, len(objects)+len(prefixes))
	sb.WriteString(`<MaxKeys>1000</MaxKeys>`)
	sb.WriteString(`<IsTruncated>false</IsTruncated>`)

	for _, cp := range prefixes {
		p := aws.ToString(cp.Prefix)
		fmt.Fprintf(&sb, `<CommonPrefixes><Prefix>%s</Prefix></CommonPrefixes>`, xmlEscape(p))
	}

	for _, obj := range objects {
		key := aws.ToString(obj.Key)
		size := int64(0)
		if obj.Size != nil {
			size = *obj.Size
		}
		lastModified := ""
		if obj.LastModified != nil {
			lastModified = obj.LastModified.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		storageClass := string(obj.StorageClass)
		if storageClass == "" {
			storageClass = "STANDARD"
		}

		sb.WriteString(`<Contents>`)
		fmt.Fprintf(&sb, `<Key>%s</Key>`, xmlEscape(key))
		fmt.Fprintf(&sb, `<Size>%d</Size>`, size)
		fmt.Fprintf(&sb, `<LastModified>%s</LastModified>`, lastModified)
		fmt.Fprintf(&sb, `<StorageClass>%s</StorageClass>`, xmlEscape(storageClass))
		sb.WriteString(`<ETag>&quot;demo-etag&quot;</ETag>`)
		sb.WriteString(`</Contents>`)
	}

	sb.WriteString(`</ListBucketResult>`)
	return sb.String()
}
