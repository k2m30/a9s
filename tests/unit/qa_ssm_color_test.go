package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestSsmColor(t *testing.T) {
	td := resource.FindResourceType("ssm")
	if td == nil {
		t.Fatal("ssm not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name: "secure_string",
			fields: map[string]string{
				"type": "SecureString",
				"name": "app/db_password",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "string_password_suffix",
			fields: map[string]string{
				"type": "String",
				"name": "app/db_password",
			},
			want: resource.ColorBroken,
		},
		{
			name: "string_secret_suffix",
			fields: map[string]string{
				"type": "String",
				"name": "foo/api_secret",
			},
			want: resource.ColorBroken,
		},
		{
			name: "string_apikey",
			fields: map[string]string{
				"type": "String",
				"name": "svc/my_apikey",
			},
			want: resource.ColorBroken,
		},
		{
			name: "string_credentials",
			fields: map[string]string{
				"type": "String",
				"name": "foo/aws_credentials",
			},
			want: resource.ColorBroken,
		},
		{
			name: "string_safe_name",
			fields: map[string]string{
				"type": "String",
				"name": "feature_flag",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "stale_string",
			fields: map[string]string{
				"type":          "String",
				"name":          "feature_flag",
				"last_modified": time.Now().AddDate(-2, 0, 0).Format("2006-01-02 15:04"),
			},
			want: resource.ColorWarning,
		},
		{
			name: "recent_string",
			fields: map[string]string{
				"type":          "String",
				"name":          "feature_flag",
				"last_modified": time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04"),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "broken_overrides_warning",
			fields: map[string]string{
				"type":          "String",
				"name":          "app/db_password",
				"last_modified": time.Now().AddDate(-2, 0, 0).Format("2006-01-02 15:04"),
			},
			want: resource.ColorBroken,
		},
		{
			name: "invalid_date_ignored",
			fields: map[string]string{
				"type":          "String",
				"name":          "feature_flag",
				"last_modified": "garbage",
			},
			want: resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
