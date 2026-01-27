package stat

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewStatsWalker(t *testing.T) {
	paths := []string{"/tmp", "/home"}
	workers := 4
	filters := &Filters{}

	walker := NewStatsWalker(paths, workers, filters)

	if walker == nil {
		t.Fatal("NewStatsWalker returned nil")
	}

	if len(walker.paths) != len(paths) {
		t.Errorf("paths length mismatch: got %d, want %d", len(walker.paths), len(paths))
	}

	if walker.workers != workers {
		t.Errorf("workers mismatch: got %d, want %d", walker.workers, workers)
	}

	if walker.filters != filters {
		t.Error("filters not set correctly")
	}

	if walker.results == nil {
		t.Fatal("results not initialized")
	}

	if walker.results.Summary == nil {
		t.Fatal("summary not initialized")
	}

	if walker.results.ByYear == nil {
		t.Fatal("ByYear map not initialized")
	}

	if walker.results.ByUID == nil {
		t.Fatal("ByUID map not initialized")
	}
}

func TestResultsInitialization(t *testing.T) {
	walker := NewStatsWalker([]string{"/tmp"}, 1, &Filters{})
	results := walker.results

	if results.Summary == nil {
		t.Fatal("Summary not initialized")
	}

	if results.ByYear == nil {
		t.Fatal("ByYear not initialized")
	}

	if results.ByUID == nil {
		t.Fatal("ByUID not initialized")
	}

	if results.TotalFiles == nil {
		t.Fatal("TotalFiles not initialized")
	}

	if results.TotalSize == nil {
		t.Fatal("TotalSize not initialized")
	}

	if results.TotalInodes == nil {
		t.Fatal("TotalInodes not initialized")
	}

	if results.AllFileInfos == nil {
		t.Fatal("AllFileInfos not initialized")
	}
}

func TestSummaryStatFields(t *testing.T) {
	summary := &SummaryStat{
		TotalSize:    1024000,
		TotalInodes:  100,
		Files:        80,
		Dirs:         15,
		Symlinks:     5,
		FilesSize:    900000,
		DirsSize:     100000,
		SymlinksSize: 24000,
	}

	tests := []struct {
		name  string
		field int64
		want  int64
	}{
		{"TotalSize", summary.TotalSize, 1024000},
		{"TotalInodes", summary.TotalInodes, 100},
		{"Files", summary.Files, 80},
		{"Dirs", summary.Dirs, 15},
		{"Symlinks", summary.Symlinks, 5},
		{"FilesSize", summary.FilesSize, 900000},
		{"DirsSize", summary.DirsSize, 100000},
		{"SymlinksSize", summary.SymlinksSize, 24000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field != tt.want {
				t.Errorf("field value mismatch: got %d, want %d", tt.field, tt.want)
			}
		})
	}
}

func TestYearStatFields(t *testing.T) {
	yearStat := &YearStat{
		Year:         2024,
		TotalSize:    512000,
		TotalInodes:  50,
		Files:        40,
		Dirs:         8,
		Symlinks:     2,
		FilesSize:    450000,
		DirsSize:     50000,
		SymlinksSize: 12000,
	}

	if yearStat.Year != 2024 {
		t.Errorf("year mismatch: got %d, want %d", yearStat.Year, 2024)
	}

	if yearStat.TotalSize != 512000 {
		t.Errorf("total size mismatch: got %d, want %d", yearStat.TotalSize, 512000)
	}

	if yearStat.TotalInodes != 50 {
		t.Errorf("total inodes mismatch: got %d, want %d", yearStat.TotalInodes, 50)
	}
}

func TestUIDStatFields(t *testing.T) {
	uidStat := &UIDStat{
		UID:         1000,
		Username:    "testuser",
		TotalSize:   256000,
		TotalInodes: 30,
		Files:       25,
		Dirs:        4,
		FilesSize:   240000,
		DirsSize:    16000,
	}

	if uidStat.UID != 1000 {
		t.Errorf("uid mismatch: got %d, want %d", uidStat.UID, 1000)
	}

	if uidStat.Username != "testuser" {
		t.Errorf("username mismatch: got %s, want %s", uidStat.Username, "testuser")
	}

	if uidStat.TotalSize != 256000 {
		t.Errorf("total size mismatch: got %d, want %d", uidStat.TotalSize, 256000)
	}
}

func TestFileInfoFields(t *testing.T) {
	now := time.Now()
	fi := &FileInfo{
		Path:      "/test/file",
		Size:      1024,
		ModTime:   now,
		IsDir:     false,
		IsSymlink: false,
		UID:       1000,
		GID:       1000,
	}

	if fi.Path != "/test/file" {
		t.Errorf("path mismatch: got %s, want %s", fi.Path, "/test/file")
	}

	if fi.Size != 1024 {
		t.Errorf("size mismatch: got %d, want %d", fi.Size, 1024)
	}

	if fi.IsDir {
		t.Error("IsDir should be false")
	}

	if fi.IsSymlink {
		t.Error("IsSymlink should be false")
	}

	if fi.UID != 1000 {
		t.Errorf("uid mismatch: got %d, want %d", fi.UID, 1000)
	}

	if fi.GID != 1000 {
		t.Errorf("gid mismatch: got %d, want %d", fi.GID, 1000)
	}
}

func TestWalkerConcurrency(t *testing.T) {
	// Test that walker is created with proper synchronization
	walker := NewStatsWalker([]string{"/tmp"}, 4, &Filters{})

	if walker.results == nil {
		t.Fatal("results should be initialized")
	}

	// The mu field should exist and be zero-initialized
	// We can't directly test mutex functionality without actual concurrent access,
	// but we can verify the walker was created properly
	if walker.workers != 4 {
		t.Errorf("workers mismatch: got %d, want %d", walker.workers, 4)
	}
}

// Test that repeated walks always start and collect entries (guards against race conditions).
func TestWalkStartsConsistently(t *testing.T) {
	root := t.TempDir()

	// Create deterministic files
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	const runs = 50
	for i := 0; i < runs; i++ {
		walker := NewStatsWalker([]string{root}, 4, &Filters{})
		res, err := walker.Walk()
		if err != nil {
			t.Fatalf("walk iteration %d failed: %v", i, err)
		}
		if res.Summary.TotalInodes == 0 {
			t.Fatalf("walk iteration %d collected zero inodes", i)
		}
		if len(res.AllFileInfos) == 0 {
			t.Fatalf("walk iteration %d collected no file infos", i)
		}
	}
}

// Run multiple walkers in parallel to surface any startup race.
func TestWalkStartsConcurrently(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "d.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(iter int) {
			defer wg.Done()
			walker := NewStatsWalker([]string{root}, 4, &Filters{})
			res, err := walker.Walk()
			if err != nil {
				errCh <- err
				return
			}
			if res.Summary.TotalInodes == 0 {
				errCh <- fmt.Errorf("iteration %d: zero inodes", iter)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent walk failed: %v", err)
		}
	}
}

func TestLookupUsername(t *testing.T) {
	// Test that lookupUsername returns a string
	result := lookupUsername(0)
	if result == "" {
		t.Error("lookupUsername should return non-empty string")
	}

	// For UID 0 (root), we should get either "root" or "uid:0"
	if result != "root" && result != "uid:0" {
		t.Logf("lookupUsername(0) returned: %s (this is OK if root is not available)", result)
	}

	// Test with a likely non-existent UID
	result = lookupUsername(999999)
	if result == "" {
		t.Error("lookupUsername should return fallback string for invalid UID")
	}
	// Should be in format "uid:999999" if not found
	t.Logf("lookupUsername(999999) returned: %s", result)
}
