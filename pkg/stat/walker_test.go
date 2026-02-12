package stat

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStatsWalker(t *testing.T) {
	paths := []string{"/tmp", "/home"}
	workers := 4
	filters := &Filters{}

	walker := NewStatsWalker(paths, workers, filters)

	assert.NotNil(t, walker)
	assert.Equal(t, len(paths), len(walker.paths))
	assert.Equal(t, workers, walker.workers)
	assert.Equal(t, filters, walker.filters)
	assert.NotNil(t, walker.results)
	assert.NotNil(t, walker.results.Summary)
	assert.NotNil(t, walker.results.ByYear)
	assert.NotNil(t, walker.results.ByUID)
}

func TestResultsInitialization(t *testing.T) {
	walker := NewStatsWalker([]string{"/tmp"}, 1, &Filters{})
	results := walker.results

	assert.NotNil(t, results.Summary)
	assert.NotNil(t, results.ByYear)
	assert.NotNil(t, results.ByUID)
	assert.NotNil(t, results.TotalFiles)
	assert.NotNil(t, results.TotalSize)
	assert.NotNil(t, results.TotalInodes)
	assert.NotNil(t, results.AllFileInfos)
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
			assert.Equal(t, tt.want, tt.field)
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

	assert.Equal(t, 2024, yearStat.Year)
	assert.Equal(t, int64(512000), yearStat.TotalSize)
	assert.Equal(t, int64(50), yearStat.TotalInodes)
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

	assert.Equal(t, uint32(1000), uidStat.UID)
	assert.Equal(t, "testuser", uidStat.Username)
	assert.Equal(t, int64(256000), uidStat.TotalSize)
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

	assert.Equal(t, "/test/file", fi.Path)
	assert.Equal(t, int64(1024), fi.Size)
	assert.False(t, fi.IsDir)
	assert.False(t, fi.IsSymlink)
	assert.Equal(t, uint32(1000), fi.UID)
	assert.Equal(t, uint32(1000), fi.GID)
}

func TestWalkerConcurrency(t *testing.T) {
	// Test that walker is created with proper synchronization
	walker := NewStatsWalker([]string{"/tmp"}, 4, &Filters{})

	assert.NotNil(t, walker.results)

	// The mu field should exist and be zero-initialized
	// We can't directly test mutex functionality without actual concurrent access,
	// but we can verify the walker was created properly
	assert.Equal(t, 4, walker.workers)
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
		assert.NoError(t, err, "walk iteration %d failed", i)
		assert.NotZero(t, res.Summary.TotalInodes, "walk iteration %d collected zero inodes", i)
		assert.NotEmpty(t, res.AllFileInfos, "walk iteration %d collected no file infos", i)
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
		assert.NoError(t, err, "concurrent walk failed")
	}
}

func TestLookupUsername(t *testing.T) {
	// Test that lookupUsername returns a string
	result := lookupUsername(0)
	assert.NotEmpty(t, result)

	// For UID 0 (root), we should get either "root" or "uid:0"
	if result != "root" && result != "uid:0" {
		t.Logf("lookupUsername(0) returned: %s (this is OK if root is not available)", result)
	}

	// Test with a likely non-existent UID
	result = lookupUsername(999999)
	assert.NotEmpty(t, result, "lookupUsername should return fallback string for invalid UID")
	// Should be in format "uid:999999" if not found
	t.Logf("lookupUsername(999999) returned: %s", result)
}
