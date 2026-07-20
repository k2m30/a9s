package unit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/cache"
)

// ---------------------------------------------------------------------------
// Dir(profile, region)
// ---------------------------------------------------------------------------

func TestCache_DirForTest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("test-profile", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "test-profile--us-east-1")
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestCache_DirForTest_SanitizesSlashes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("my/profile", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "my_profile--us-east-1")
	if got != want {
		t.Errorf("Dir() with slashes = %q, want %q", got, want)
	}
}

func TestCache_DirForTest_SanitizesSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("my profile", "us west 2")
	want := filepath.Join(tmpDir, "cache", "my_profile--us_west_2")
	if got != want {
		t.Errorf("Dir() with spaces = %q, want %q", got, want)
	}
}

func TestCache_DirForTest_SanitizesBackslash(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("corp\\admin", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "corp_admin--us-east-1")
	if got != want {
		t.Errorf("Dir() with backslash = %q, want %q", got, want)
	}
}

func TestCache_DirForTest_EmptyProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("", "us-east-1")
	want := filepath.Join(tmpDir, "cache", "--us-east-1")
	if got != want {
		t.Errorf("Dir('', 'us-east-1') = %q, want %q", got, want)
	}
}

func TestCache_DirForTest_EmptyRegion(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	got := cache.DirForTest("test-profile", "")
	want := filepath.Join(tmpDir, "cache", "test-profile--")
	if got != want {
		t.Errorf("Dir('test-profile', '') = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// LoadDir() — never fails, absent directory
// ---------------------------------------------------------------------------

func TestCache_LoadDirForTest_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("nonexistent-profile", "us-east-1")
	if store == nil {
		t.Fatal("LoadDir() on an absent directory must return a non-nil (empty) Store, per C7 — never fail")
	}
	if len(store.Types()) != 0 {
		t.Errorf("LoadDir() on an absent directory returned %d types, want 0", len(store.Types()))
	}
}

func TestCache_LoadDirForTest_EmptyTypeFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	dir := cache.DirForTest("test-profile", "us-east-1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	emptyFile := filepath.Join(dir, "ec2.yaml")
	if err := os.WriteFile(emptyFile, []byte{}, 0600); err != nil {
		t.Fatalf("writing empty file: %v", err)
	}

	store := cache.LoadDirForTest("test-profile", "us-east-1")
	if store == nil {
		t.Fatal("LoadDir() must return a non-nil Store even with a corrupt/empty per-type file")
	}
	if _, ok := store.Type("ec2"); ok {
		t.Error(`Type("ec2") should not be present — an empty/unparseable per-type file means "no cache" for that type only (C7)`)
	}
}

func TestCache_LoadDirForTest_ValidTypeFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("test-profile", "us-east-1")
	if store == nil {
		t.Fatal("LoadDir() returned nil on an empty directory")
	}
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 7})
	store.Put("s3", cache.TypeFile{HasResources: false, Count: 0})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType(ec2): %v", err)
	}
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType(s3): %v", err)
	}

	reloaded := cache.LoadDirForTest("test-profile", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir() returned nil for a populated directory")
	}

	ec2Entry, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`Type("ec2") missing after reload`)
	}
	if !ec2Entry.HasResources {
		t.Error("ec2 HasResources should be true")
	}
	if ec2Entry.Count != 7 {
		t.Errorf("ec2 Count = %d, want 7", ec2Entry.Count)
	}

	s3Entry, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal(`Type("s3") missing after reload`)
	}
	if s3Entry.HasResources {
		t.Error("s3 HasResources should be false")
	}
	if s3Entry.Count != 0 {
		t.Errorf("s3 Count = %d, want 0", s3Entry.Count)
	}
}

func TestCache_LoadDirForTest_CorruptTypeFile_SkipsOnlyThatType(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	dir := cache.DirForTest("test-profile", "us-east-1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	corruptContent := `{{{not: valid: yaml: [[[`
	corruptPath := filepath.Join(dir, "ec2.yaml")
	if err := os.WriteFile(corruptPath, []byte(corruptContent), 0600); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	// A sibling healthy type file must still load normally (C7: "An
	// unreadable or wrong-version file means 'no cache' for that type
	// only: the other types load normally").
	healthy := cache.LoadDirForTest("test-profile", "us-east-1")
	healthy.Put("s3", cache.TypeFile{HasResources: true, Count: 3})
	if err := healthy.SaveType("s3"); err != nil {
		t.Fatalf("SaveType(s3): %v", err)
	}

	store := cache.LoadDirForTest("test-profile", "us-east-1")
	if store == nil {
		t.Fatal("LoadDir() must return a non-nil Store even with a corrupt sibling type file")
	}
	if _, ok := store.Type("ec2"); ok {
		t.Error(`Type("ec2") should be absent — the corrupt file must not be surfaced as valid cache data`)
	}
	s3Entry, ok := store.Type("s3")
	if !ok {
		t.Fatal(`Type("s3") missing — a corrupt sibling file must not prevent a healthy type from loading (C7)`)
	}
	if s3Entry.Count != 3 {
		t.Errorf("s3 Count = %d, want 3", s3Entry.Count)
	}
}

// ---------------------------------------------------------------------------
// Store.Put / Store.SaveType — round trip
// ---------------------------------------------------------------------------

func TestCache_SaveType_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("test-profile", "eu-west-1")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 5})

	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() returned error: %v", err)
	}

	dir := cache.DirForTest("test-profile", "eu-west-1")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cache directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache path should be a directory")
	}

	expectedPath := filepath.Join(dir, "ec2.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("per-type cache file was not created at %s: %v", expectedPath, err)
	}
}

func TestCache_SaveType_WritesValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("test-profile", "us-west-2")
	store.Put("rds", cache.TypeFile{HasResources: true, Count: 3})
	store.Put("lambda", cache.TypeFile{HasResources: false, Count: 0})

	if err := store.SaveType("rds"); err != nil {
		t.Fatalf("SaveType(rds): %v", err)
	}
	if err := store.SaveType("lambda"); err != nil {
		t.Fatalf("SaveType(lambda): %v", err)
	}

	reloaded := cache.LoadDirForTest("test-profile", "us-west-2")
	rds, ok := reloaded.Type("rds")
	if !ok || rds.Count != 3 {
		t.Errorf("Type(rds) = %+v (ok=%v), want Count=3", rds, ok)
	}
	lambda, ok := reloaded.Type("lambda")
	if !ok || lambda.Count != 0 {
		t.Errorf("Type(lambda) = %+v (ok=%v), want Count=0", lambda, ok)
	}
}

func TestCache_SaveType_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("test-profile", "us-east-1")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 10})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() first write error: %v", err)
	}

	store2 := cache.LoadDirForTest("test-profile", "us-east-1")
	store2.Put("ec2", cache.TypeFile{HasResources: false, Count: 0})
	if err := store2.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() second write error: %v", err)
	}

	reloaded := cache.LoadDirForTest("test-profile", "us-east-1")
	ec2, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`Type("ec2") missing after overwrite`)
	}
	if ec2.HasResources {
		t.Error("ec2 should be false after overwrite (should reflect second write)")
	}
	if ec2.Count != 0 {
		t.Errorf("ec2 Count = %d, want 0 after overwrite", ec2.Count)
	}
}

func TestCache_LoadDirForTest_AllResourceTypes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	entries := map[string]cache.TypeFile{
		"ec2":          {HasResources: true, Count: 10},
		"s3":           {HasResources: true, Count: 25},
		"rds":          {HasResources: false, Count: 0},
		"redis":        {HasResources: true, Count: 3},
		"docdb":        {HasResources: false, Count: 0},
		"eks":          {HasResources: true, Count: 2},
		"secrets":      {HasResources: false, Count: 0},
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

	store := cache.LoadDirForTest("multi-resource-profile", "us-east-1")
	for name, tf := range entries {
		store.Put(name, tf)
	}
	for name := range entries {
		if err := store.SaveType(name); err != nil {
			t.Fatalf("SaveType(%s): %v", name, err)
		}
	}

	reloaded := cache.LoadDirForTest("multi-resource-profile", "us-east-1")
	for name, orig := range entries {
		got, ok := reloaded.Type(name)
		if !ok {
			t.Errorf("missing resource type %q after round-trip", name)
			continue
		}
		if got.HasResources != orig.HasResources {
			t.Errorf("%q: HasResources = %v, want %v", name, got.HasResources, orig.HasResources)
		}
		if got.Count != orig.Count {
			t.Errorf("%q: Count = %d, want %d", name, got.Count, orig.Count)
		}
	}
}

// ---------------------------------------------------------------------------
// SchemaVersion constant
// ---------------------------------------------------------------------------

func TestCache_SchemaVersion_IsTwo(t *testing.T) {
	if cache.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2 (issue #463: per-finding FirstSeen bumped the on-disk schema)", cache.SchemaVersion)
	}
}

// ---------------------------------------------------------------------------
// SaveType() atomicity — per-type file
// ---------------------------------------------------------------------------

// TestCache_SaveType_NoTempFileLingers verifies that after SaveType returns,
// no .tmp file remains in the per-pair cache directory.
func TestCache_SaveType_NoTempFileLingers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	store := cache.LoadDirForTest("p", "r")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 3})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() error: %v", err)
	}

	dir := cache.DirForTest("p", "r")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error: %v", dir, err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("stale temp file found after SaveType: %s", entry.Name())
		}
	}
}

// TestCache_SaveType_ConcurrentWrites_NoCorruption spawns N goroutines that
// each call SaveType on the SAME type concurrently. After all goroutines
// complete, LoadDir must succeed and return a well-formed TypeFile for that
// type — the atomic-rename write must never leave a half-written file
// observable.
func TestCache_SaveType_ConcurrentWrites_NoCorruption(t *testing.T) {
	const N = 20
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	baseline := cache.LoadDirForTest("race-profile", "us-east-1")
	baseline.Put("ec2", cache.TypeFile{HasResources: true, Count: 0})
	if err := baseline.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() baseline error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := range N {
		go func(idx int) {
			defer wg.Done()
			store := cache.LoadDirForTest("race-profile", "us-east-1")
			store.Put("ec2", cache.TypeFile{HasResources: true, Count: idx + 1})
			//nolint:errcheck // best-effort concurrent write; we check result via LoadDir below
			_ = store.SaveType("ec2")
		}(i)
	}
	wg.Wait()

	reloaded := cache.LoadDirForTest("race-profile", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir() after concurrent SaveType() returned nil")
	}
	tf, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal("LoadDir() after concurrent SaveType() found no ec2 type file — corrupted or missing")
	}
	if tf.Count < 1 || tf.Count > N {
		t.Errorf("ec2.Count = %d after concurrent writes, want a value written by one of the goroutines (1..%d)", tf.Count, N)
	}
}

// TestCache_SaveType_AtomicVisibility is the strongest atomicity test. A
// writer goroutine repeatedly calls SaveType in a tight loop on one type. The
// main goroutine concurrently calls LoadDir and asserts every read observes
// a well-formed TypeFile — never a corrupt/partial one.
func TestCache_SaveType_AtomicVisibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file locking prevents concurrent read during rename")
	}
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	seed := cache.LoadDirForTest("atomic-profile", "eu-central-1")
	seed.Put("ec2", cache.TypeFile{HasResources: true, Count: 1})
	if err := seed.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType() seed error: %v", err)
	}

	deadline := time.Now().Add(50 * time.Millisecond)

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
			store := cache.LoadDirForTest("atomic-profile", "eu-central-1")
			store.Put("ec2", cache.TypeFile{HasResources: true, Count: count})
			//nolint:errcheck // writer races are expected; we observe via reader
			_ = store.SaveType("ec2")
		}
	}()

	corruptReads := 0
	for time.Now().Before(deadline) {
		reloaded := cache.LoadDirForTest("atomic-profile", "eu-central-1")
		if reloaded == nil {
			corruptReads++
			t.Errorf("LoadDir() returned nil during concurrent SaveType() — atomicity violation")
			if corruptReads >= 3 {
				break
			}
			continue
		}
		if _, ok := reloaded.Type("ec2"); !ok {
			corruptReads++
			t.Errorf("LoadDir() found no ec2 type file during concurrent SaveType() — file vanished mid-write")
			if corruptReads >= 3 {
				break
			}
		}
	}

	close(stopWriter)
	<-writerDone
}
