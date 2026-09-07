package unit

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/secretscan"
)

// --- ScanKV ---

func TestScanKV_KeywordHit(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"DB_PASSWORD": "hunter2hunter2"})
	want := []secretscan.Hit{{Kind: "keyword", Where: "DB_PASSWORD"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v", hits, want)
	}
}

func TestScanKV_NoHit(t *testing.T) {
	tests := []struct {
		name string
		kv   map[string]string
	}{
		{"arn reference is not a secret", map[string]string{"DB_PASSWORD": "arn:aws:secretsmanager:us-east-1:123456789012:secret:x"}},
		{"CFN dynamic reference", map[string]string{"DB_PASSWORD": "{{resolve:secretsmanager:x}}"}},
		{"SSM parameter path", map[string]string{"DB_PASSWORD": "/prod/db/password"}},
		{"unresolved template placeholder", map[string]string{"DB_PASSWORD": "${DB_PASSWORD}"}},
		{"empty value", map[string]string{"DB_PASSWORD": ""}},
		{"placeholder value xxxx", map[string]string{"DB_PASSWORD": "xxxx"}},
		{"placeholder value changeme", map[string]string{"DB_PASSWORD": "changeme"}},
		{"boolean-like value", map[string]string{"API_TOKEN": "true"}},
		{"innocent key and value", map[string]string{"LOG_LEVEL": "debug", "REGION": "us-east-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits := secretscan.ScanKV(tt.kv)
			if hits != nil {
				t.Errorf("ScanKV(%v) = %v, want nil", tt.kv, hits)
			}
		})
	}
}

func TestScanKV_AWSAccessKey_ExactlyOneHit(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"AWS_ACCESS_KEY_ID": "AKIAIOSFODNN7EXAMPLE"})
	want := []secretscan.Hit{{Kind: "aws-access-key", Where: "AWS_ACCESS_KEY_ID"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want exactly %v (keyword must NOT also be reported for the same key)", hits, want)
	}
}

func TestScanKV_AWSAccessKey_InnocentKeyName(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"SOME_CONFIG": "AKIAIOSFODNN7EXAMPLE"})
	want := []secretscan.Hit{{Kind: "aws-access-key", Where: "SOME_CONFIG"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v (value scanned regardless of key name)", hits, want)
	}
}

func TestScanKV_URLWithUserinfo_IsKeywordHit(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"DATABASE_URL": "postgres://app:s3cretpass@db.internal:5432/app"})
	want := []secretscan.Hit{{Kind: "keyword", Where: "DATABASE_URL"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v (URL with userinfo is a hit)", hits, want)
	}
}

func TestScanKV_PrivateKeyValue(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"TLS_KEY": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA1234567890abcdef\n-----END RSA PRIVATE KEY-----"})
	want := []secretscan.Hit{{Kind: "private-key", Where: "TLS_KEY"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v", hits, want)
	}
}

func TestScanKV_MixedCaseKeyMatches(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"Db_Password": "hunter2hunter2"})
	want := []secretscan.Hit{{Kind: "keyword", Where: "Db_Password"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v (key match must be case-insensitive, Where preserves original case)", hits, want)
	}
}

func TestScanKV_MultipleHitsSortedByWhere(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{
		"Z_TOKEN": "hunter2hunter2",
		"A_TOKEN": "hunter3hunter3",
	})
	want := []secretscan.Hit{
		{Kind: "keyword", Where: "A_TOKEN"},
		{Kind: "keyword", Where: "Z_TOKEN"},
	}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanKV() = %v, want %v (sorted by Where)", hits, want)
	}
}

// --- ScanText ---

func TestScanText_KeywordHit_LineNumber(t *testing.T) {
	hits := secretscan.ScanText("#!/bin/bash\nexport DB_PASSWORD=hunter2hunter2\n")
	want := []secretscan.Hit{{Kind: "keyword", Where: "line 2"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText() = %v, want %v", hits, want)
	}
}

func TestScanText_AWSAccessKey_NoDuplicateKeywordHit(t *testing.T) {
	hits := secretscan.ScanText("aws_access_key_id = AKIAIOSFODNN7EXAMPLE")
	want := []secretscan.Hit{{Kind: "aws-access-key", Where: "line 1"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText() = %v, want exactly %v", hits, want)
	}
}

func TestScanText_NoHit(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"template placeholder value", "password: ${SSM_PASSWORD}"},
		{"empty quoted value", `PASSWORD=""`},
		{"decoy AMI id and hash, no keyword context", "ami-0abcdef1234567890 sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
		{"empty string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits := secretscan.ScanText(tt.text)
			if hits != nil {
				t.Errorf("ScanText(%q) = %v, want nil", tt.text, hits)
			}
		})
	}
}

func TestScanText_JWTHit(t *testing.T) {
	text := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	hits := secretscan.ScanText(text)
	found := false
	for _, h := range hits {
		if h.Kind == "jwt" && h.Where == "line 1" {
			found = true
		}
	}
	if !found {
		t.Errorf("ScanText(%q) = %v, want a jwt hit at line 1", text, hits)
	}
}

// INVERTED, and deliberately so: this pin used to require BOTH a keyword and
// a high-entropy hit on one line. A generated credential under a credential
// name satisfies both rules, but it is one credential, and the entropy hit
// was only ever appended behind a keyword hit, so it named nothing the
// keyword hit had not already named. One line, one hit, by the reason that
// identifies it.
func TestScanText_GeneratedValueUnderCredentialKey_IsOneHit(t *testing.T) {
	text := "SECRET_KEY=OhbVrpoiVgRV5IfLBcbfnoGMbJmTPS\n"
	hits := secretscan.ScanText(text)
	want := []secretscan.Hit{{Kind: "keyword", Where: "line 1"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText(%q) = %v, want %v", text, hits, want)
	}
}

func TestScanText_CRLFLineEndings_LineNumbersCorrect(t *testing.T) {
	text := "#!/bin/bash\r\necho hello\r\nexport DB_PASSWORD=hunter2hunter2\r\n"
	hits := secretscan.ScanText(text)
	want := []secretscan.Hit{{Kind: "keyword", Where: "line 3"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText(CRLF) = %v, want %v (line numbers must be correct despite \\r\\n endings)", hits, want)
	}
}

func TestScanText_CRLF_AWSAccessKeyOnSecondLine(t *testing.T) {
	text := "no secrets here\r\naws_access_key_id = AKIAIOSFODNN7EXAMPLE\r\n"
	hits := secretscan.ScanText(text)
	want := []secretscan.Hit{{Kind: "aws-access-key", Where: "line 2"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText(CRLF) = %v, want %v", hits, want)
	}
}

// --- Redact ---

func TestRedact(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"hunter2hunter2", "hu…r2"},
		{"abc", "…"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := secretscan.Redact(tt.value)
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// --- verify round: adversarial attacks against the landed engine ---

func TestScanKV_ARNValueWithTrailingWhitespace_NoHit(t *testing.T) {
	hits := secretscan.ScanKV(map[string]string{"DB_PASSWORD": "arn:aws:secretsmanager:us-east-1:123456789012:secret:x   "})
	if hits != nil {
		t.Errorf("ScanKV() = %v, want nil (trailing whitespace must not defeat the arn: reference exclusion)", hits)
	}
}

func TestScanText_SecretOnLastLineNoTrailingNewline(t *testing.T) {
	hits := secretscan.ScanText("#!/bin/bash\nexport DB_PASSWORD=hunter2hunter2")
	want := []secretscan.Hit{{Kind: "keyword", Where: "line 2"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText() = %v, want %v (a match on the final line must still be found when the text has no trailing newline)", hits, want)
	}
}

func fiveThousandLineUserData() string {
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("echo 'this is a totally normal line of user data with nothing interesting ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("'\n")
	}
	sb.WriteString("export DB_PASSWORD=hunter2hunter2\n")
	return sb.String()
}

func TestScanText_5000Lines_FindsPlantedSecretAtRightLine(t *testing.T) {
	hits := secretscan.ScanText(fiveThousandLineUserData())
	want := []secretscan.Hit{{Kind: "keyword", Where: "line 5001"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("ScanText(5000 lines) = %v, want %v", hits, want)
	}
}

func BenchmarkScanText_5000Lines(b *testing.B) {
	text := fiveThousandLineUserData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secretscan.ScanText(text)
	}
}
