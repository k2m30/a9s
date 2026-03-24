// Package demo provides synthetic fixture data for demo mode.
// When a9s is launched with --demo, these fixtures replace real AWS API calls,
// allowing the full TUI to run without AWS credentials.
package demo

import (
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// DemoRegion is the synthetic region displayed in demo mode.
const DemoRegion = "us-east-1"

// DemoProfile is the synthetic profile displayed in demo mode.
const DemoProfile = "demo"

// r53RecordData maps Route53 hosted zone IDs to record fixture generator functions.
var r53RecordData = map[string]func() []resource.Resource{}

// demoData maps resource short names to fixture generator functions.
// Each call returns a fresh slice (no shared global state).
// Generators are registered via init() in each fixtures_*.go category file.
var demoData = map[string]func() []resource.Resource{}

// GetResources returns fixture data for the given resource type.
// The resourceType should be the canonical short name (e.g., "ec2", "dbi").
// Returns nil, false for resource types without demo data.
func GetResources(resourceType string) ([]resource.Resource, bool) {
	gen, ok := demoData[resourceType]
	if !ok {
		return nil, false
	}
	return gen(), true
}

// GetS3Objects returns fixture data for S3 objects within a bucket.
// Returns nil, false if the bucket is not in demo data.
func GetS3Objects(bucket, prefix string) ([]resource.Resource, bool) {
	gen, ok := s3ObjectData[bucket]
	if !ok {
		return nil, false
	}
	return gen(), true
}

// s3ObjectData maps bucket names to their object fixture generators.
var s3ObjectData = map[string]func() []resource.Resource{
	"data-pipeline-logs":  s3ObjDataPipeline,
	"webapp-assets-prod":  s3ObjWebapp,
	"ml-training-data":    s3ObjMLTraining,
	"terraform-state-prod": s3ObjTerraform,
	"cloudtrail-audit-logs": s3ObjCloudtrail,
	"backup-db-snapshots": s3ObjBackups,
}

// GetR53Records returns fixture data for Route53 records within a hosted zone.
// Returns nil, false if the zone ID is not in demo data.
func GetR53Records(zoneID string) ([]resource.Resource, bool) {
	gen, ok := r53RecordData[zoneID]
	if !ok {
		return nil, false
	}
	return gen(), true
}

// childDemoData maps child type short names to fixture generator functions.
// Each generator receives a parentCtx with parameters from the parent view.
var childDemoData = map[string]func(parentCtx map[string]string) []resource.Resource{
	"s3_objects": func(parentCtx map[string]string) []resource.Resource {
		resources, _ := GetS3Objects(parentCtx["bucket"], parentCtx["prefix"])
		return resources
	},
	"r53_records": func(parentCtx map[string]string) []resource.Resource {
		resources, _ := GetR53Records(parentCtx["zone_id"])
		return resources
	},
}

// RegisterChildDemo registers a child demo data generator for the given child type.
func RegisterChildDemo(childType string, gen func(parentCtx map[string]string) []resource.Resource) {
	childDemoData[childType] = gen
}

// GetChildResources returns fixture data for a child view given its type and parent context.
// Returns nil, false if the child type has no demo data.
func GetChildResources(childType string, parentCtx map[string]string) ([]resource.Resource, bool) {
	gen, ok := childDemoData[childType]
	if !ok {
		return nil, false
	}
	return gen(parentCtx), true
}

// mustParseTime parses a time string in RFC3339 format or panics.
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("demo: invalid time literal: " + s)
	}
	return t
}
