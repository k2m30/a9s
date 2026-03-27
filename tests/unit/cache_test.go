package unit

import (
	"os"
	"path/filepath"
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
  s3:
    has_resources: false
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
			"ec2": {HasResources: true},
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
			"rds":    {HasResources: true},
			"lambda": {HasResources: false, Error: "timeout"},
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
			"ec2":     {HasResources: true},
			"s3":      {HasResources: true},
			"rds":     {HasResources: false},
			"lambda":  {HasResources: true},
			"vpc":     {HasResources: false, Error: "access denied"},
			"eks":     {HasResources: false},
			"redis":   {HasResources: true},
			"docdb":   {HasResources: false},
			"secrets": {HasResources: true},
			"sg":      {HasResources: true},
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
			"ec2": {HasResources: true},
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
			"ec2": {HasResources: false},
			"s3":  {HasResources: true},
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
		"ec2":           {HasResources: true},
		"s3":            {HasResources: true},
		"rds":           {HasResources: false},
		"redis":         {HasResources: true},
		"docdb":         {HasResources: false},
		"eks":           {HasResources: true},
		"secrets":       {HasResources: false, Error: "no access"},
		"vpc":           {HasResources: true},
		"sg":            {HasResources: true},
		"nodegroups":    {HasResources: false},
		"lambda":        {HasResources: true},
		"elb":           {HasResources: false},
		"tg":            {HasResources: true},
		"subnets":       {HasResources: true},
		"nat":           {HasResources: false},
		"igw":           {HasResources: true},
		"eip":           {HasResources: false},
		"eni":           {HasResources: true},
		"iam_roles":     {HasResources: true},
		"iam_policies":  {HasResources: true},
		"iam_users":     {HasResources: false},
		"iam_groups":    {HasResources: false},
		"waf":           {HasResources: true},
		"ssm":           {HasResources: true},
		"kms":           {HasResources: false},
		"r53":           {HasResources: true},
		"cloudfront":    {HasResources: false},
		"acm":           {HasResources: true},
		"apigw":         {HasResources: true},
		"cfn":           {HasResources: false},
		"codebuild":     {HasResources: true},
		"codepipeline":  {HasResources: false},
		"ecr":           {HasResources: true},
		"codeartifact":  {HasResources: false},
		"cw_alarms":     {HasResources: true},
		"log_groups":    {HasResources: true},
		"cloudtrail":    {HasResources: false},
		"sqs":           {HasResources: true},
		"sns":           {HasResources: false},
		"eventbridge":   {HasResources: true},
		"kinesis":       {HasResources: false},
		"sfn":           {HasResources: true},
		"msk":           {HasResources: false},
		"glue":          {HasResources: true},
		"athena":        {HasResources: false},
		"opensearch":    {HasResources: true},
		"redshift":      {HasResources: false},
		"dynamodb":      {HasResources: true},
		"asg":           {HasResources: true},
		"eb":            {HasResources: false},
		"ecs":           {HasResources: true},
		"backup":        {HasResources: false},
		"ses":           {HasResources: true},
		"efs":           {HasResources: false},
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
		if loadedEntry.Error != origEntry.Error {
			t.Errorf("%q: Error = %q, want %q", name, loadedEntry.Error, origEntry.Error)
		}
	}
}
