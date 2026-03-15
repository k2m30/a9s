package unit

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

func generateResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := 0; i < n; i++ {
		resources[i] = resource.Resource{
			ID:     fmt.Sprintf("i-%010d", i),
			Name:   fmt.Sprintf("instance-%d", i),
			Status: "running",
			Fields: map[string]string{
				"instance_id": fmt.Sprintf("i-%010d", i),
				"name":        fmt.Sprintf("instance-%d", i),
				"state":       "running",
				"type":        "t3.medium",
				"private_ip":  fmt.Sprintf("10.0.%d.%d", i/256, i%256),
				"public_ip":   fmt.Sprintf("54.%d.%d.%d", i/65536, (i/256)%256, i%256),
				"launch_time": "2026-01-15T10:30:00Z",
			},
		}
	}
	// Add some "prod" resources for filter testing
	for i := 0; i < n/10; i++ {
		resources[i].Name = fmt.Sprintf("prod-server-%d", i)
		resources[i].Fields["name"] = fmt.Sprintf("prod-server-%d", i)
	}
	return resources
}

func BenchmarkFilterResources_1000rows(b *testing.B) {
	resources := generateResources(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		views.FilterResources("prod", resources)
	}
}

func BenchmarkFilterResources_500rows(b *testing.B) {
	resources := generateResources(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		views.FilterResources("t3.medium", resources)
	}
}

func TestFilterPerformance_Under200ms(t *testing.T) {
	resources := generateResources(1000)
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			views.FilterResources("prod", resources)
		}
	})
	nsPerOp := result.NsPerOp()
	msPerOp := float64(nsPerOp) / 1e6
	if msPerOp > 200 {
		t.Errorf("Filter too slow: %.2fms per operation (target: <200ms)", msPerOp)
	}
	t.Logf("Filter performance: %.2fms per operation on 1000 rows", msPerOp)
}
