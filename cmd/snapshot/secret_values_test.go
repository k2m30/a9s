package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
)

// A Glue job's default arguments are free-form and routinely carry
// credentials, so the snapshot keeps which arguments exist, never their values.
func TestSecretValue_GlueJobRecordKeepsArgumentKeysOnly(t *testing.T) {
	job := gluetypes.Job{
		Name:        aws.String("nightly-orders-etl"),
		Role:        aws.String("arn:aws:iam::123456789012:role/glue-etl"),
		GlueVersion: aws.String("4.0"),
		WorkerType:  gluetypes.WorkerTypeG1x,
		Command: &gluetypes.JobCommand{
			Name:           aws.String("glueetl"),
			ScriptLocation: aws.String("s3://acme-scripts/orders.py"),
		},
		DefaultArguments: map[string]string{
			"--db-password": "hunter2-example",
			"--TempDir":     "s3://acme-tmp/",
		},
	}

	info := glueJobInfoOf(job)

	if want := []string{"--TempDir", "--db-password"}; !reflect.DeepEqual(info.DefaultArgumentKeys, want) {
		t.Errorf("DefaultArgumentKeys = %q, want %q", info.DefaultArgumentKeys, want)
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(raw)
	for _, value := range []string{"hunter2-example", "s3://acme-tmp/"} {
		if strings.Contains(out, value) {
			t.Errorf("record JSON carries argument value %q: %s", value, out)
		}
	}
	if !strings.Contains(out, `"default_argument_keys":["--TempDir","--db-password"]`) {
		t.Errorf("record JSON lacks the sorted argument keys: %s", out)
	}

	bare := glueJobInfoOf(gluetypes.Job{Name: aws.String("no-args-job")})
	raw, err = json.Marshal(bare)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "default_argument") {
		t.Errorf("job without default arguments emits an argument field: %s", raw)
	}
}

// A PLAINTEXT CodeBuild variable holds the secret itself; the two reference
// types hold only the Parameter Store name or Secrets Manager id where the
// secret lives, which is safe and useful to keep.
func TestSecretValue_CodeBuildEnvVarDropsPlaintextValueOnly(t *testing.T) {
	cases := []struct {
		ev        cbtypes.EnvironmentVariable
		wantType  string
		wantValue string
	}{
		{
			ev:       cbtypes.EnvironmentVariable{Name: aws.String("API_TOKEN"), Value: aws.String("tok-example-123"), Type: cbtypes.EnvironmentVariableTypePlaintext},
			wantType: "PLAINTEXT",
		},
		{
			ev:        cbtypes.EnvironmentVariable{Name: aws.String("DB_PASS"), Value: aws.String("/acme/db/pass"), Type: cbtypes.EnvironmentVariableTypeParameterStore},
			wantType:  "PARAMETER_STORE",
			wantValue: "/acme/db/pass",
		},
		{
			ev:        cbtypes.EnvironmentVariable{Name: aws.String("GH"), Value: aws.String("acme/github:token"), Type: cbtypes.EnvironmentVariableTypeSecretsManager},
			wantType:  "SECRETS_MANAGER",
			wantValue: "acme/github:token",
		},
	}
	for _, tc := range cases {
		name := aws.ToString(tc.ev.Name)
		t.Run(name, func(t *testing.T) {
			got := cbEnvVarOf(tc.ev)
			if got.Name != name || got.Type != tc.wantType || got.Value != tc.wantValue {
				t.Errorf("cbEnvVarOf = %+v, want {Name:%s Value:%s Type:%s}", got, name, tc.wantValue, tc.wantType)
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if tc.wantValue == "" && strings.Contains(string(raw), aws.ToString(tc.ev.Value)) {
				t.Errorf("record JSON carries the plaintext value: %s", raw)
			}
			if tc.wantValue != "" && !strings.Contains(string(raw), tc.wantValue) {
				t.Errorf("record JSON lost the reference %q: %s", tc.wantValue, raw)
			}
		})
	}
}
