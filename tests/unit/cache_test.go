package unit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/cache"
)

// ---------------------------------------------------------------------------
// Dir()
// ---------------------------------------------------------------------------

func TestCache_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Dir()
	want := filepath.Join(tmpDir, "cache")
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestCache_Dir_Default(t *testing.T) {
	// Unset env var to exercise the fallback path.
	t.Setenv("A9S_CONFIG_FOLDER", "")

	got := cache.Dir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}
	want := filepath.Join(home, ".a9s", "cache")
	if got != want {
		t.Errorf("Dir() without env var = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Path()
// ---------------------------------------------------------------------------

func TestCache_Path(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("test-profile", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "test-profile--us-east-1.yaml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestCache_Path_SanitizesSlashes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("my/profile", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "my_profile--us-east-1.yaml")
	if got != want {
		t.Errorf("Path() with slashes = %q, want %q", got, want)
	}
}

func TestCache_Path_SanitizesSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("my profile", "us west 2")
	want := filepath.Join(tmpDir, "cache", "my_profile--us_west_2.yaml")
	if got != want {
		t.Errorf("Path() with spaces = %q, want %q", got, want)
	}
}

func TestCache_Path_SanitizesBackslash(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("corp\\admin", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "corp_admin--us-east-1.yaml")
	if got != want {
		t.Errorf("Path() with backslash = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Load()
// ---------------------------------------------------------------------------

func TestCache_Load_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	f, err := cache.Load("nonexistent-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() for missing file should return nil error, got: %v", err)
	}
	if f != nil {
		t.Error("Load() for missing file should return nil *File")
	}
}

func TestCache_Load_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Create the cache dir and an empty file.
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	emptyFile := filepath.Join(cacheDir, "test-profile--us-east-1.yaml")
	if err := os.WriteFile(emptyFile, []byte{}, 0600); err != nil {
		t.Fatalf("writing empty file: %v", err)
	}

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() for empty file should return nil error, got: %v", err)
	}
	if f != nil {
		t.Error("Load() for empty file should return nil *File")
	}
}

func TestCache_Load_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}

	yamlContent := `profile: test-profile
region: us-east-1
checked_at: 2026-03-27T10:00:00Z
resources:
  ec2:
    has_resources: true
    count: 7
  s3:
    has_resources: false
    count: 0
    error: "access denied"
`
	filePath := filepath.Join(cacheDir, "test-profile--us-east-1.yaml")
	if err := os.WriteFile(filePath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load() returned nil *File for valid content")
	}

	if f.Profile != "test-profile" {
		t.Errorf("Profile = %q, want %q", f.Profile, "test-profile")
	}
	if f.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q", f.Region, "us-east-1")
	}
	if f.CheckedAt.IsZero() {
		t.Error("CheckedAt should not be zero")
	}
	if len(f.Resources) != 2 {
		t.Fatalf("Resources count = %d, want 2", len(f.Resources))
	}

	ec2Entry, ok := f.Resources["ec2"]
	if !ok {
		t.Fatal("Resources missing ec2 entry")
	}
	if !ec2Entry.HasResources {
		t.Error("ec2 HasResources should be true")
	}
	if ec2Entry.Count != 7 {
		t.Errorf("ec2 Count = %d, want 7", ec2Entry.Count)
	}
	if ec2Entry.Error != "" {
		t.Errorf("ec2 Error = %q, want empty", ec2Entry.Error)
	}

	s3Entry, ok := f.Resources["s3"]
	if !ok {
		t.Fatal("Resources missing s3 entry")
	}
	if s3Entry.HasResources {
		t.Error("s3 HasResources should be false")
	}
	if s3Entry.Count != 0 {
		t.Errorf("s3 Count = %d, want 0", s3Entry.Count)
	}
	if s3Entry.Error != "access denied" {
		t.Errorf("s3 Error = %q, want %q", s3Entry.Error, "access denied")
	}
}

func TestCache_Load_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}

	corruptContent := `{{{not: valid: yaml: [[[`
	filePath := filepath.Join(cacheDir, "test-profile--us-east-1.yaml")
	if err := os.WriteFile(filePath, []byte(corruptContent), 0600); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	f, err := cache.Load("test-profile", "us-east-1")
	if err == nil {
		t.Error("Load() for corrupt file should return error")
	}
	if f != nil {
		t.Error("Load() for corrupt file should return nil *File")
	}
}

// ---------------------------------------------------------------------------
// Save()
// ---------------------------------------------------------------------------

func TestCache_Save_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	f := &cache.File{
		Profile:   "test-profile",
		Region:    "eu-west-1",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 5},
		},
	}

	if err := cache.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Verify the cache directory was created.
	cacheDir := filepath.Join(tmpDir, "cache")
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("cache directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache path should be a directory")
	}

	// Verify the file exists.
	expectedPath := filepath.Join(cacheDir, "test-profile--eu-west-1.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("cache file was not created at %s: %v", expectedPath, err)
	}
}

func TestCache_Save_WritesValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	f := &cache.File{
		Profile:   "test-profile",
		Region:    "us-west-2",
		CheckedAt: time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
		Resources: map[string]cache.Entry{
			"rds":    {HasResources: true, Count: 3},
			"lambda": {HasResources: false, Count: 0, Error: "timeout"},
		},
	}

	if err := cache.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Read the file back and verify it's valid YAML by loading it.
	loaded, err := cache.Load("test-profile", "us-west-2")
	if err != nil {
		t.Fatalf("Load() after Save() returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() after Save() returned nil")
	}
	if loaded.Profile != "test-profile" {
		t.Errorf("loaded Profile = %q, want %q", loaded.Profile, "test-profile")
	}
	if loaded.Region != "us-west-2" {
		t.Errorf("loaded Region = %q, want %q", loaded.Region, "us-west-2")
	}
}

func TestCache_Save_NilFile(t *testing.T) {
	err := cache.Save(nil)
	if err != nil {
		t.Errorf("Save(nil) should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// IsExpired()
// ---------------------------------------------------------------------------

func TestCache_IsExpired_NilFile(t *testing.T) {
	var f *cache.File
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("nil File.IsExpired() should return true")
	}
}

func TestCache_IsExpired_ZeroTime(t *testing.T) {
	f := &cache.File{
		Profile: "test-profile",
		Region:  "us-east-1",
		// CheckedAt is zero value
	}
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("File with zero CheckedAt.IsExpired() should return true")
	}
}

func TestCache_IsExpired_Fresh(t *testing.T) {
	f := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now(),
	}
	if f.IsExpired(cache.DefaultTTL) {
		t.Error("File checked just now should not be expired with 1h TTL")
	}
}

func TestCache_IsExpired_Stale(t *testing.T) {
	f := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now().Add(-2 * time.Hour),
	}
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("File checked 2h ago should be expired with 1h TTL")
	}
}

func TestCache_IsExpired_ExactlyAtTTL(t *testing.T) {
	// At exactly TTL boundary, time.Since > ttl should be false (or barely true).
	// Use a slightly past TTL to avoid race conditions.
	f := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now().Add(-cache.DefaultTTL - time.Second),
	}
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("File checked just past TTL should be expired")
	}
}

func TestCache_IsExpired_CustomTTL(t *testing.T) {
	f := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now().Add(-30 * time.Minute),
	}
	// With 1h TTL, 30min old should be fresh.
	if f.IsExpired(time.Hour) {
		t.Error("30min old file should NOT be expired with 1h TTL")
	}
	// With 15min TTL, 30min old should be stale.
	if !f.IsExpired(15 * time.Minute) {
		t.Error("30min old file should be expired with 15min TTL")
	}
}

// ---------------------------------------------------------------------------
// Round-trip: Save + Load
// ---------------------------------------------------------------------------

func TestCache_SaveLoad_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	checkedAt := time.Date(2026, 3, 27, 15, 30, 0, 0, time.UTC)
	original := &cache.File{
		Profile:   "prod-account",
		Region:    "ap-southeast-1",
		CheckedAt: checkedAt,
		Resources: map[string]cache.Entry{
			"ec2":     {HasResources: true, Count: 12},
			"s3":      {HasResources: true, Count: 45},
			"rds":     {HasResources: false, Count: 0},
			"lambda":  {HasResources: true, Count: 8},
			"vpc":     {HasResources: false, Count: 0, Error: "access denied"},
			"eks":     {HasResources: false, Count: 0},
			"redis":   {HasResources: true, Count: 2},
			"docdb":   {HasResources: false, Count: 0},
			"secrets": {HasResources: true, Count: 15},
			"sg":      {HasResources: true, Count: 30},
		},
	}

	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := cache.Load("prod-account", "ap-southeast-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after Save()")
	}

	// Verify all fields preserved.
	if loaded.Profile != original.Profile {
		t.Errorf("Profile = %q, want %q", loaded.Profile, original.Profile)
	}
	if loaded.Region != original.Region {
		t.Errorf("Region = %q, want %q", loaded.Region, original.Region)
	}
	if !loaded.CheckedAt.Equal(original.CheckedAt) {
		t.Errorf("CheckedAt = %v, want %v", loaded.CheckedAt, original.CheckedAt)
	}
	if len(loaded.Resources) != len(original.Resources) {
		t.Fatalf("Resources count = %d, want %d", len(loaded.Resources), len(original.Resources))
	}

	for name, origEntry := range original.Resources {
		loadedEntry, ok := loaded.Resources[name]
		if !ok {
			t.Errorf("Resources missing %q after round-trip", name)
			continue
		}
		if loadedEntry.HasResources != origEntry.HasResources {
			t.Errorf("Resources[%q].HasResources = %v, want %v", name, loadedEntry.HasResources, origEntry.HasResources)
		}
		if loadedEntry.Count != origEntry.Count {
			t.Errorf("Resources[%q].Count = %d, want %d", name, loadedEntry.Count, origEntry.Count)
		}
		if loadedEntry.Error != origEntry.Error {
			t.Errorf("Resources[%q].Error = %q, want %q", name, loadedEntry.Error, origEntry.Error)
		}
	}
}

func TestCache_SaveLoad_RoundTrip_EmptyResources(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	original := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now().UTC(),
		Resources: map[string]cache.Entry{},
	}

	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after Save()")
	}
	if len(loaded.Resources) != 0 {
		t.Errorf("Resources count = %d, want 0", len(loaded.Resources))
	}
}

// ---------------------------------------------------------------------------
// DefaultTTL constant
// ---------------------------------------------------------------------------

func TestCache_DefaultTTL_IsOneHour(t *testing.T) {
	if cache.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want %v", cache.DefaultTTL, time.Hour)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestCache_Save_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Save first version.
	f1 := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 10},
		},
	}
	if err := cache.Save(f1); err != nil {
		t.Fatalf("Save() first write error: %v", err)
	}

	// Save second version with different data.
	f2 := &cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: false, Count: 0},
			"s3":  {HasResources: true, Count: 20},
		},
	}
	if err := cache.Save(f2); err != nil {
		t.Fatalf("Save() second write error: %v", err)
	}

	loaded, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}
	if len(loaded.Resources) != 2 {
		t.Errorf("Resources count = %d, want 2 (should reflect second write)", len(loaded.Resources))
	}
	if loaded.Resources["ec2"].HasResources {
		t.Error("ec2 should be false after overwrite")
	}
}

func TestCache_Path_EmptyProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "--us-east-1.yaml")
	if got != want {
		t.Errorf("Path('', 'us-east-1') = %q, want %q", got, want)
	}
}

func TestCache_Path_EmptyRegion(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.Path("test-profile", "")
	want := filepath.Join(tmpDir, "cache", "test-profile--.yaml")
	if got != want {
		t.Errorf("Path('test-profile', '') = %q, want %q", got, want)
	}
}

func TestCache_Load_AllResourceTypes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Build a cache file with many resource types to verify all survive round-trip.
	entries := map[string]cache.Entry{
		"ec2":          {HasResources: true, Count: 10},
		"s3":           {HasResources: true, Count: 25},
		"rds":          {HasResources: false, Count: 0},
		"redis":        {HasResources: true, Count: 3},
		"docdb":        {HasResources: false, Count: 0},
		"eks":          {HasResources: true, Count: 2},
		"secrets":      {HasResources: false, Count: 0, Error: "no access"},
		"vpc":          {HasResources: true, Count: 4},
		"sg":           {HasResources: true, Count: 50},
		"nodegroups":   {HasResources: false, Count: 0},
		"lambda":       {HasResources: true, Count: 100},
		"elb":          {HasResources: false, Count: 0},
		"tg":           {HasResources: true, Count: 8},
		"subnets":      {HasResources: true, Count: 12},
		"nat":          {HasResources: false, Count: 0},
		"igw":          {HasResources: true, Count: 1},
		"eip":          {HasResources: false, Count: 0},
		"eni":          {HasResources: true, Count: 20},
		"iam_roles":    {HasResources: true, Count: 75},
		"iam_policies": {HasResources: true, Count: 200},
		"iam_users":    {HasResources: false, Count: 0},
		"iam_groups":   {HasResources: false, Count: 0},
		"waf":          {HasResources: true, Count: 5},
		"ssm":          {HasResources: true, Count: 30},
		"kms":          {HasResources: false, Count: 0},
		"r53":          {HasResources: true, Count: 15},
		"cloudfront":   {HasResources: false, Count: 0},
		"acm":          {HasResources: true, Count: 10},
		"apigw":        {HasResources: true, Count: 3},
		"cfn":          {HasResources: false, Count: 0},
		"codebuild":    {HasResources: true, Count: 7},
		"codepipeline": {HasResources: false, Count: 0},
		"ecr":          {HasResources: true, Count: 14},
		"codeartifact": {HasResources: false, Count: 0},
		"cw_alarms":    {HasResources: true, Count: 40},
		"log_groups":   {HasResources: true, Count: 60},
		"cloudtrail":   {HasResources: false, Count: 0},
		"sqs":          {HasResources: true, Count: 9},
		"sns":          {HasResources: false, Count: 0},
		"eventbridge":  {HasResources: true, Count: 6},
		"kinesis":      {HasResources: false, Count: 0},
		"sfn":          {HasResources: true, Count: 4},
		"msk":          {HasResources: false, Count: 0},
		"glue":         {HasResources: true, Count: 11},
		"athena":       {HasResources: false, Count: 0},
		"opensearch":   {HasResources: true, Count: 2},
		"redshift":     {HasResources: false, Count: 0},
		"dynamodb":     {HasResources: true, Count: 18},
		"asg":          {HasResources: true, Count: 5},
		"eb":           {HasResources: false, Count: 0},
		"ecs":          {HasResources: true, Count: 8},
		"backup":       {HasResources: false, Count: 0},
		"ses":          {HasResources: true, Count: 3},
		"efs":          {HasResources: false, Count: 0},
	}

	original := &cache.File{
		Profile:   "multi-resource-profile",
		Region:    "us-east-1",
		CheckedAt: time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
		Resources: entries,
	}

	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := cache.Load("multi-resource-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	for name, origEntry := range entries {
		loadedEntry, ok := loaded.Resources[name]
		if !ok {
			t.Errorf("missing resource type %q after round-trip", name)
			continue
		}
		if loadedEntry.HasResources != origEntry.HasResources {
			t.Errorf("%q: HasResources = %v, want %v", name, loadedEntry.HasResources, origEntry.HasResources)
		}
		if loadedEntry.Count != origEntry.Count {
			t.Errorf("%q: Count = %d, want %d", name, loadedEntry.Count, origEntry.Count)
		}
		if loadedEntry.Error != origEntry.Error {
			t.Errorf("%q: Error = %q, want %q", name, loadedEntry.Error, origEntry.Error)
		}
	}
}

// ---------------------------------------------------------------------------
// Round-trip: Count field survives Save + Load
// ---------------------------------------------------------------------------

func TestCache_SaveLoad_RoundTrip_Count(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	checkedAt := time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC)
	original := &cache.File{
		Profile:   "count-test-profile",
		Region:    "eu-west-1",
		CheckedAt: checkedAt,
		Resources: map[string]cache.Entry{
			"ec2":    {HasResources: false, Count: 0},
			"s3":     {HasResources: true, Count: 1},
			"rds":    {HasResources: true, Count: 100},
			"lambda": {HasResources: true, Count: 1000},
		},
	}

	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := cache.Load("count-test-profile", "eu-west-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after Save()")
	}

	if len(loaded.Resources) != len(original.Resources) {
		t.Fatalf("Resources count = %d, want %d", len(loaded.Resources), len(original.Resources))
	}

	for name, origEntry := range original.Resources {
		loadedEntry, ok := loaded.Resources[name]
		if !ok {
			t.Errorf("Resources missing %q after round-trip", name)
			continue
		}
		if loadedEntry.Count != origEntry.Count {
			t.Errorf("Resources[%q].Count = %d, want %d", name, loadedEntry.Count, origEntry.Count)
		}
		if loadedEntry.HasResources != origEntry.HasResources {
			t.Errorf("Resources[%q].HasResources = %v, want %v", name, loadedEntry.HasResources, origEntry.HasResources)
		}
	}
}

// ---------------------------------------------------------------------------
// Save() atomicity tests
// ---------------------------------------------------------------------------

// TestCache_Save_NoTempFileLingers verifies that after Save returns, no .tmp
// file remains in the cache directory. With the current os.WriteFile
// implementation this passes trivially (no temp file is ever created). After
// the temp+rename fix lands it continues to pass — the rename removes the
// temp file as part of the atomic swap.
func TestCache_Save_NoTempFileLingers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	f := &cache.File{
		Profile:   "p",
		Region:    "r",
		CheckedAt: time.Now().UTC(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 3},
		},
	}

	if err := cache.Save(f); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", cacheDir, err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("stale temp file found after Save: %s", entry.Name())
		}
	}
}

// TestCache_Save_ConcurrentWrites_NoCorruption spawns N goroutines that each
// call Save concurrently. After all goroutines complete, Load must succeed and
// return a well-formed *File. With os.WriteFile (truncate then write) two
// goroutines can interleave their writes, producing a partial YAML that
// Load cannot parse. This test surfaces that corruption.
func TestCache_Save_ConcurrentWrites_NoCorruption(t *testing.T) {
	const N = 20
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Write a baseline so the file exists before the race begins.
	baseline := &cache.File{
		Profile:   "race-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now().UTC(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 0},
		},
	}
	if err := cache.Save(baseline); err != nil {
		t.Fatalf("Save() baseline error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := range N {
		go func(idx int) {
			defer wg.Done()
			f := &cache.File{
				Profile:   "race-profile",
				Region:    "us-east-1",
				CheckedAt: time.Date(2026, 1, idx+1, 0, 0, 0, 0, time.UTC),
				Resources: map[string]cache.Entry{
					"ec2": {HasResources: true, Count: idx + 1},
				},
			}
			//nolint:errcheck // best-effort concurrent write; we check result via Load below
			_ = cache.Save(f)
		}(i)
	}
	wg.Wait()

	// After all writers have finished, the file must be parseable.
	loaded, err := cache.Load("race-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load() after concurrent Save() returned parse error (file corrupted): %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() after concurrent Save() returned nil (file missing or empty)")
	}
	// The file must have a non-zero CheckedAt — a zero value indicates a
	// partial write that produced an otherwise valid but empty struct.
	if loaded.CheckedAt.IsZero() {
		t.Error("Load() returned a File with zero CheckedAt — indicates partial write corruption")
	}
}

// TestCache_Save_AtomicVisibility is the strongest atomicity test. A writer
// goroutine repeatedly calls Save in a tight loop. The main goroutine
// concurrently calls Load and asserts that every successful (non-nil-error)
// Load returns a well-formed *File. A half-written file will cause Load to
// return a YAML parse error — that error IS the failure signal. The test runs
// for ~500 ms. With os.WriteFile this test is expected to fail intermittently;
// after the temp+rename fix it must not fail.
func TestCache_Save_AtomicVisibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file locking prevents concurrent read during rename")
	}
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Seed a valid file so readers always have something to observe.
	seed := &cache.File{
		Profile:   "atomic-profile",
		Region:    "eu-central-1",
		CheckedAt: time.Now().UTC(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 1},
			"rds": {HasResources: false, Count: 0},
			"eks": {HasResources: true, Count: 2},
		},
	}
	if err := cache.Save(seed); err != nil {
		t.Fatalf("Save() seed error: %v", err)
	}

	deadline := time.Now().Add(50 * time.Millisecond)

	// Writer goroutine: alternate between two distinct payloads so the file
	// content actually changes on each iteration.
	stopWriter := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		alt := false
		for {
			select {
			case <-stopWriter:
				return
			default:
			}
			count := 1
			if alt {
				count = 9999
			}
			alt = !alt
			f := &cache.File{
				Profile:   "atomic-profile",
				Region:    "eu-central-1",
				CheckedAt: time.Now().UTC(),
				Resources: map[string]cache.Entry{
					"ec2": {HasResources: true, Count: count},
					"rds": {HasResources: false, Count: 0},
					"eks": {HasResources: true, Count: 2},
				},
			}
			//nolint:errcheck // writer races are expected; we observe via reader
			_ = cache.Save(f)
		}
	}()

	// Reader loop: every successful Load must be well-formed.
	corruptReads := 0
	for time.Now().Before(deadline) {
		loaded, err := cache.Load("atomic-profile", "eu-central-1")
		if err != nil {
			// A parse error means the reader observed a half-written file —
			// the atomicity guarantee was violated.
			corruptReads++
			t.Errorf("Load() returned parse error during concurrent Save() (atomicity violation): %v", err)
			if corruptReads >= 3 {
				// Bail early after 3 violations to avoid flooding the log.
				break
			}
		}
		// nil + nil means file disappeared between writes — also a violation.
		if err == nil && loaded == nil {
			corruptReads++
			t.Errorf("Load() returned (nil, nil) during concurrent Save() — file vanished mid-write")
			if corruptReads >= 3 {
				break
			}
		}
	}

	close(stopWriter)
	<-writerDone
}
