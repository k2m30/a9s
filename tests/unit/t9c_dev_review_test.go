package unit_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A row replayed from the disk cache holds Fields and no struct. A dotted
// default detail path whose value the fetcher stored under some Fields key
// shows that value on the replayed row too, at every drill level: the key is
// the type's ReplayKeys entry for the path, so a path the table misses is red.
func TestT9cDev_ReplayedRowShowsEveryStoredDetailValue(t *testing.T) {
	cfg := config.DefaultConfig()
	seen := map[string]bool{}
	checked := 0
	for _, row := range t9bDevTreeRows(t) {
		if row.res.RawStruct == nil || len(row.res.Fields) == 0 {
			continue
		}
		td := catalog.FindAny(row.typ)
		if td == nil || td.Project != nil {
			continue
		}
		var owed []string
		for _, df := range config.GetViewDef(cfg, row.typ).Detail {
			v := fieldpath.ExtractScalar(row.res.RawStruct, df.Path)
			if !strings.Contains(df.Path, ".") || v == "" || seen[row.typ+" "+df.Path] {
				continue
			}
			for _, stored := range row.res.Fields {
				if stored == v {
					owed = append(owed, df.Path)
					break
				}
			}
		}
		if len(owed) == 0 {
			continue
		}
		replay := row.res
		replay.RawStruct = nil
		shown := t9bDetailRows(t, replay, row.typ)
		for _, p := range owed {
			seen[row.typ+" "+p] = true
			checked++
			want := fieldpath.ExtractScalar(row.res.RawStruct, p)
			if !slices.ContainsFunc(t9bPathRows(shown, p), func(g string) bool { return strings.HasSuffix(g, "="+want) }) {
				t.Errorf("%s %s: replayed row shows %v, want %q", row.typ, p, t9bPathRows(shown, p), want)
			}
		}
	}
	if checked < 3 {
		t.Fatalf("checked %d stored detail values; the walk reaches too little", checked)
	}
}

// A terminated environment holds no configuration to read, so the secret's
// EB pivot asks AWS about live environments only.
func TestT9cDev_SecretsEBPivotReadsLiveEnvironmentsOnly(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	env := func(name, status string) resource.Resource {
		return resource.Resource{
			ID:        name,
			Fields:    map[string]string{"status": status},
			RawStruct: ebtypes.EnvironmentDescription{EnvironmentName: aws.String(name), ApplicationName: aws.String("acme-app")},
		}
	}
	var asked []string
	fakeEB := &fakeEBChecker{
		describeConfigSettingsFn: func(in *elasticbeanstalk.DescribeConfigurationSettingsInput) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			asked = append(asked, aws.ToString(in.EnvironmentName))
			return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
		},
	}
	cache := resource.ResourceCache{"eb": resource.ResourceCacheEntry{Resources: []resource.Resource{
		env("acme-live", "Ready"), env("acme-gone", "Terminated"), env("acme-going", "Terminating"),
	}}}
	result := secretsCheckerByTarget(t, "eb")(context.Background(), &awsclient.ServiceClients{ElasticBeanstalk: fakeEB}, secretsSourceWithARN(secretARN, "prod/db/password"), cache)
	if !slices.Equal(asked, []string{"acme-live"}) {
		t.Errorf("DescribeConfigurationSettings asked for %v, want [acme-live]", asked)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

type t9cDevSelfRow struct {
	*t9cDevSelfRow
	Name string
}

// A row type that embeds itself resolves its own fields and answers a
// missing one, instead of searching the embed forever.
func TestT9cDev_SelfEmbeddingRowTypeResolves(t *testing.T) {
	td := catalog.ResourceTypeDef{RowTypes: []reflect.Type{reflect.TypeFor[t9cDevSelfRow]()}}
	if !td.CellPath("Name") {
		t.Error("Name is not a cell path of the row")
	}
	if td.CellPath("Missing") {
		t.Error("Missing is a cell path of the row")
	}
}

// A demo policy that bounds a demo principal counts that use, as
// Policy.PermissionsBoundaryUsageCount does.
func TestT9cDev_DemoBoundaryPoliciesCountTheirUses(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	uses := map[string]int32{}
	roles, err := clients.IAM.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roles.Roles {
		got, getErr := clients.IAM.(awsclient.IAMGetRoleAPI).GetRole(ctx, &iam.GetRoleInput{RoleName: r.RoleName})
		if getErr == nil && got.Role.PermissionsBoundary != nil {
			uses[aws.ToString(got.Role.PermissionsBoundary.PermissionsBoundaryArn)]++
		}
	}
	users, err := clients.IAM.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users.Users {
		got, getErr := clients.IAM.(awsclient.IAMGetUserAPI).GetUser(ctx, &iam.GetUserInput{UserName: u.UserName})
		if getErr == nil && got.User.PermissionsBoundary != nil {
			uses[aws.ToString(got.User.PermissionsBoundary.PermissionsBoundaryArn)]++
		}
	}
	if len(uses) == 0 {
		t.Fatal("no demo principal has a permissions boundary")
	}
	policies, err := clients.IAM.ListPolicies(ctx, &iam.ListPoliciesInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range policies.Policies {
		if want := uses[aws.ToString(p.Arn)]; want > 0 && aws.ToInt32(p.PermissionsBoundaryUsageCount) != want {
			t.Errorf("%s bounds %d principals, PermissionsBoundaryUsageCount = %v", aws.ToString(p.PolicyName), want, aws.ToInt32(p.PermissionsBoundaryUsageCount))
		}
	}
}

// A terminated environment's own pivots that read its resources or its
// configuration by name ask AWS nothing; a Ready one still asks.
func TestT9cDev_TerminatedEnvironmentPivotsAskNothing(t *testing.T) {
	td := catalog.FindAny("eb")
	if td == nil {
		t.Fatal("no eb type")
	}
	for _, tc := range []struct {
		status string
		asks   bool
	}{{"Terminated", false}, {"Ready", true}} {
		calls := 0
		fakeEB := &fakeEBChecker{
			describeEnvResourcesFn: func(*elasticbeanstalk.DescribeEnvironmentResourcesInput) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
				calls++
				return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{}, nil
			},
			describeConfigSettingsFn: func(*elasticbeanstalk.DescribeConfigurationSettingsInput) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
				calls++
				return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
			},
		}
		row := resource.Resource{
			ID:        "acme-web",
			Fields:    map[string]string{"status": tc.status},
			RawStruct: ebtypes.EnvironmentDescription{EnvironmentName: aws.String("acme-web"), ApplicationName: aws.String("acme-app")},
		}
		clients := &awsclient.ServiceClients{ElasticBeanstalk: fakeEB, ELBv2: &fakeELBv2CR{}}
		for _, rel := range td.Related {
			if !slices.Contains([]string{"elb", "tg", "sg", "role"}, rel.TargetType) {
				continue
			}
			before := calls
			result := rel.Checker(context.Background(), clients, row, resource.ResourceCache{})
			if asked := calls > before; asked != tc.asks {
				t.Errorf("%s environment, %s pivot: asked AWS = %v, want %v", tc.status, rel.TargetType, asked, tc.asks)
			}
			if !tc.asks && result.Count() != 0 {
				t.Errorf("%s environment, %s pivot: count %d, want 0", tc.status, rel.TargetType, result.Count())
			}
		}
	}
}
