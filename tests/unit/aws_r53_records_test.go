package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Route 53 DNS Records fetcher tests
// ---------------------------------------------------------------------------

// TestFetchR53Records_Basic verifies parsing of A (multi-value), CNAME, and MX
// records with correct ID, Name, Status, Fields (name, type, ttl, values), and
// RawStruct.
func TestFetchR53Records_Basic(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("1.2.3.4")},
							{Value: aws.String("5.6.7.8")},
						},
					},
					{
						Name: aws.String("www.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(3600),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("example.com.")},
						},
					},
					{
						Name: aws.String("example.com."),
						Type: r53types.RRTypeMx,
						TTL:  aws.Int64(600),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10 mail.example.com.")},
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/Z123", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// Verify A record
	t.Run("A_record_ID", func(t *testing.T) {
		if resources[0].ID != "example.com.|A" {
			t.Errorf("ID: expected %q, got %q", "example.com.|A", resources[0].ID)
		}
	})

	t.Run("A_record_Name", func(t *testing.T) {
		if resources[0].Name != "example.com." {
			t.Errorf("Name: expected %q, got %q", "example.com.", resources[0].Name)
		}
	})

	t.Run("A_record_Type", func(t *testing.T) {
		if got := resources[0].Fields["type"]; got != "A" {
			t.Errorf("Fields[\"type\"]: expected %q, got %q", "A", got)
		}
	})

	t.Run("A_record_fields", func(t *testing.T) {
		r := resources[0]
		if r.Fields["name"] != "example.com." {
			t.Errorf("Fields[name]: expected %q, got %q", "example.com.", r.Fields["name"])
		}
		if r.Fields["type"] != "A" {
			t.Errorf("Fields[type]: expected %q, got %q", "A", r.Fields["type"])
		}
		if r.Fields["ttl"] != "300" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "300", r.Fields["ttl"])
		}
		if r.Fields["values"] != "1.2.3.4, 5.6.7.8" {
			t.Errorf("Fields[values]: expected %q, got %q", "1.2.3.4, 5.6.7.8", r.Fields["values"])
		}
	})

	t.Run("A_record_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(r53types.ResourceRecordSet)
		if !ok {
			t.Fatalf("RawStruct should be r53types.ResourceRecordSet, got %T", r.RawStruct)
		}
		if raw.Name == nil || *raw.Name != "example.com." {
			t.Errorf("RawStruct.Name: expected %q", "example.com.")
		}
	})

	// Verify CNAME record
	t.Run("CNAME_record", func(t *testing.T) {
		r := resources[1]
		if r.ID != "www.example.com.|CNAME" {
			t.Errorf("ID: expected %q, got %q", "www.example.com.|CNAME", r.ID)
		}
		if r.Name != "www.example.com." {
			t.Errorf("Name: expected %q, got %q", "www.example.com.", r.Name)
		}
		if got := r.Fields["type"]; got != "CNAME" {
			t.Errorf("Fields[\"type\"]: expected %q, got %q", "CNAME", got)
		}
		if r.Fields["ttl"] != "3600" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "3600", r.Fields["ttl"])
		}
		if r.Fields["values"] != "example.com." {
			t.Errorf("Fields[values]: expected %q, got %q", "example.com.", r.Fields["values"])
		}
	})

	// Verify MX record
	t.Run("MX_record", func(t *testing.T) {
		r := resources[2]
		if r.ID != "example.com.|MX" {
			t.Errorf("ID: expected %q, got %q", "example.com.|MX", r.ID)
		}
		if got := r.Fields["type"]; got != "MX" {
			t.Errorf("Fields[\"type\"]: expected %q, got %q", "MX", got)
		}
		if r.Fields["ttl"] != "600" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "600", r.Fields["ttl"])
		}
		if r.Fields["values"] != "10 mail.example.com." {
			t.Errorf("Fields[values]: expected %q, got %q", "10 mail.example.com.", r.Fields["values"])
		}
	})

	// Verify all records have required fields
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"name", "type", "ttl", "values"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchR53Records_AliasRecord verifies that alias records (AliasTarget
// instead of ResourceRecords) produce the correct values and empty TTL.
func TestFetchR53Records_AliasRecord(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("cdn.example.com."),
						Type: r53types.RRTypeA,
						AliasTarget: &r53types.AliasTarget{
							DNSName:              aws.String("target.example.com."),
							HostedZoneId:         aws.String("Z2FDTNDATAQYW2"),
							EvaluateTargetHealth: true,
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZALIAS", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("alias_values", func(t *testing.T) {
		expected := "ALIAS: target.example.com."
		if r.Fields["values"] != expected {
			t.Errorf("Fields[values]: expected %q, got %q", expected, r.Fields["values"])
		}
	})

	t.Run("alias_ttl_empty", func(t *testing.T) {
		if r.Fields["ttl"] != "" {
			t.Errorf("Fields[ttl]: expected empty string for alias, got %q", r.Fields["ttl"])
		}
	})

	t.Run("alias_id", func(t *testing.T) {
		if r.ID != "cdn.example.com.|A" {
			t.Errorf("ID: expected %q, got %q", "cdn.example.com.|A", r.ID)
		}
	})

	t.Run("alias_name_and_type", func(t *testing.T) {
		if r.Name != "cdn.example.com." {
			t.Errorf("Name: expected %q, got %q", "cdn.example.com.", r.Name)
		}
		if r.Fields["type"] != "A" {
			t.Errorf("Fields[type]: expected %q, got %q", "A", r.Fields["type"])
		}
	})

	t.Run("alias_type", func(t *testing.T) {
		if got := r.Fields["type"]; got != "A" {
			t.Errorf("Fields[\"type\"]: expected %q, got %q", "A", got)
		}
	})

	t.Run("alias_rawstruct", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(r53types.ResourceRecordSet)
		if !ok {
			t.Fatalf("RawStruct should be r53types.ResourceRecordSet, got %T", r.RawStruct)
		}
		if raw.AliasTarget == nil {
			t.Fatal("RawStruct.AliasTarget should not be nil")
		}
		if raw.AliasTarget.DNSName == nil || *raw.AliasTarget.DNSName != "target.example.com." {
			t.Errorf("RawStruct.AliasTarget.DNSName: expected %q", "target.example.com.")
		}
	})
}

// TestFetchR53Records_Pagination verifies that paginated responses (IsTruncated
// + NextRecordName/NextRecordType) are followed and all records collected.
// TestFetchR53Records_Pagination verifies the single-page pagination contract:
// one API call is made per invocation, resources from that page are returned,
// and IsTruncated/NextToken (compound JSON cursor) reflect whether more pages
// exist. A second call with the continuation token verifies the token is
// forwarded and the final page sets IsTruncated=false.
func TestFetchR53Records_Pagination(t *testing.T) {
	// Page 1: 2 records with IsTruncated=true and NextRecordName indicating more pages.
	page1Mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated:    true,
				NextRecordName: aws.String("page2.example.com."),
				NextRecordType: r53types.RRTypeA,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("first.example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("1.1.1.1")},
						},
					},
					{
						Name: aws.String("second.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("first.example.com.")},
						},
					},
				},
			},
		},
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchR53Records(context.Background(), page1Mock, "/hostedzone/ZPAGE", "")
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		if page1Mock.callIdx != 1 {
			t.Errorf("expected 1 API call for page 1, got %d", page1Mock.callIdx)
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when IsTruncated=true from API")
		}
	})

	t.Run("page1_next_token_non_empty", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken == "" {
			t.Error("page 1: NextToken should be non-empty compound JSON cursor when truncated")
		}
	})

	t.Run("page1_record_names", func(t *testing.T) {
		if result1.Resources[0].Name != "first.example.com." {
			t.Errorf("resources[0].Name: expected %q, got %q", "first.example.com.", result1.Resources[0].Name)
		}
		if result1.Resources[1].Name != "second.example.com." {
			t.Errorf("resources[1].Name: expected %q, got %q", "second.example.com.", result1.Resources[1].Name)
		}
	})

	// Page 2: 1 record with IsTruncated=false — last page.
	page2Mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("page2.example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("2.2.2.2")},
						},
					},
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchR53Records(context.Background(), page2Mock, "/hostedzone/ZPAGE", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 1 {
			t.Fatalf("expected 1 resource on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_empty_next_token", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.NextToken != "" {
			t.Errorf("page 2: NextToken should be empty on last page, got %q", result2.Pagination.NextToken)
		}
	})

	t.Run("page2_record_name", func(t *testing.T) {
		if result2.Resources[0].Name != "page2.example.com." {
			t.Errorf("page 2: resource[0].Name expected %q, got %q", "page2.example.com.", result2.Resources[0].Name)
		}
	})
}

// TestFetchR53Records_SetIdentifier verifies that records with a SetIdentifier
// (weighted/latency/failover routing) include the identifier in the ID, while
// records without a SetIdentifier do not.
func TestFetchR53Records_SetIdentifier(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name:          aws.String("weighted.example.com."),
						Type:          r53types.RRTypeA,
						TTL:           aws.Int64(60),
						SetIdentifier: aws.String("us-east-1"),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10.0.0.1")},
						},
					},
					{
						Name:          aws.String("weighted.example.com."),
						Type:          r53types.RRTypeA,
						TTL:           aws.Int64(60),
						SetIdentifier: aws.String("us-west-2"),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10.0.1.1")},
						},
					},
					{
						Name: aws.String("simple.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("target.example.com.")},
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZWEIGHT", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("id_with_set_identifier_east", func(t *testing.T) {
		expected := "weighted.example.com.|A|us-east-1"
		if resources[0].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[0].ID)
		}
	})

	t.Run("id_with_set_identifier_west", func(t *testing.T) {
		expected := "weighted.example.com.|A|us-west-2"
		if resources[1].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[1].ID)
		}
	})

	t.Run("id_without_set_identifier", func(t *testing.T) {
		expected := "simple.example.com.|CNAME"
		if resources[2].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[2].ID)
		}
	})
}

// TestFetchR53Records_Error verifies that API errors are propagated correctly.
func TestFetchR53Records_Error(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	result, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZERR", "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchR53Records_Empty verifies that an empty record set returns an empty
// non-nil slice with no error.
func TestFetchR53Records_Empty(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated:        false,
				ResourceRecordSets: []r53types.ResourceRecordSet{},
			},
		},
	}

	result, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZEMPTY", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestR53RecordColumns verifies that R53RecordColumns returns the expected
// 4 columns with the correct keys: name, type, ttl, values.
func TestR53RecordColumns(t *testing.T) {
	cols := resource.R53RecordColumns()

	expectedKeys := []string{"name", "type", "ttl", "values"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 4 {
			t.Fatalf("expected 4 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}
