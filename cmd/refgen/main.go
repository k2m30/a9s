package main

import (
	"fmt"
	"reflect"

	"github.com/k2m30/a9s/internal/fieldpath"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type resourceDef struct {
	name    string
	comment string
	typ     reflect.Type
}

func main() {
	resources := []resourceDef{
		{"s3", "s3types.Bucket", reflect.TypeOf(s3types.Bucket{})},
		{"s3_objects", "s3types.Object", reflect.TypeOf(s3types.Object{})},
		{"ec2", "ec2types.Instance", reflect.TypeOf(ec2types.Instance{})},
		{"dbi", "rdstypes.DBInstance", reflect.TypeOf(rdstypes.DBInstance{})},
		{"redis", "elasticachetypes.CacheCluster", reflect.TypeOf(elasticachetypes.CacheCluster{})},
		{"dbc", "docdbtypes.DBCluster", reflect.TypeOf(docdbtypes.DBCluster{})},
		{"eks", "ekstypes.Cluster", reflect.TypeOf(ekstypes.Cluster{})},
		{"secrets", "smtypes.SecretListEntry", reflect.TypeOf(smtypes.SecretListEntry{})},
		{"vpc", "ec2types.Vpc", reflect.TypeOf(ec2types.Vpc{})},
		{"sg", "ec2types.SecurityGroup", reflect.TypeOf(ec2types.SecurityGroup{})},
		{"ng", "ekstypes.Nodegroup", reflect.TypeOf(ekstypes.Nodegroup{})},
	}

	fmt.Println("# views_reference.yaml")
	fmt.Println("# Generated from AWS SDK Go v2 struct reflection")
	fmt.Println("# Use these paths in your views.yaml configuration")
	fmt.Println()

	for _, r := range resources {
		paths := fieldpath.EnumeratePaths(r.typ, "")
		fmt.Printf("%s:  # %s\n", r.name, r.comment)
		for _, p := range paths {
			fmt.Printf("  - %s\n", p)
		}
		fmt.Println()
	}
}
