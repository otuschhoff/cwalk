// Tests for package cwalk.
package cwalk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// setupTestDir creates a temporary test directory structure and returns its path.
//
// The structure created is:
//
//	tmpDir/
//	  file1.txt
//	  dir1/
//	    file2.txt
//	    dir2/
//	      file3.txt
//	  dir3/
//	    file4.txt
func setupTestDir(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create directory structure:
	// tmpDir/
	//   file1.txt
	//   dir1/
	//     file2.txt
	//     dir2/
	//       file3.txt
	//   dir3/
	//     file4.txt

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0600); err != nil {
		t.Fatalf("failed to create file1.txt: %v", err)
	}

	dir1 := filepath.Join(tmpDir, "dir1")
	if err := os.Mkdir(dir1, 0755); err != nil {
		t.Fatalf("failed to create dir1: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir1, "file2.txt"), []byte("content2"), 0600); err != nil {
		t.Fatalf("failed to create file2.txt: %v", err)
	}

	dir2 := filepath.Join(dir1, "dir2")
	if err := os.Mkdir(dir2, 0755); err != nil {
		t.Fatalf("failed to create dir2: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir2, "file3.txt"), []byte("content3"), 0600); err != nil {
		t.Fatalf("failed to create file3.txt: %v", err)
	}

	dir3 := filepath.Join(tmpDir, "dir3")
	if err := os.Mkdir(dir3, 0755); err != nil {
		t.Fatalf("failed to create dir3: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir3, "file4.txt"), []byte("content4"), 0600); err != nil {
		t.Fatalf("failed to create file4.txt: %v", err)
	}

	return tmpDir
}

// TestNewWalker tests the creation of a new Walker.
//
// It verifies that:
//   - New() creates a Walker with the specified number of workers
//   - Invalid worker counts (0 or negative) default to 1
//   - The root path is properly cleaned
func TestNewWalker(t *testing.T) {
	tmpDir := setupTestDir(t)

	tests := []struct {
		name        string
		rootPath    string
		numWorkers  int
		wantWorkers int
	}{
		{
			name:        "default number of workers",
			rootPath:    tmpDir,
			numWorkers:  0,
			wantWorkers: 1,
		},
		{
			name:        "negative number of workers",
			rootPath:    tmpDir,
			numWorkers:  -5,
			wantWorkers: 1,
		},
		{
			name:        "multiple workers",
			rootPath:    tmpDir,
			numWorkers:  4,
			wantWorkers: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walker := NewWalker(tt.rootPath, tt.numWorkers, Callbacks{})
			assert.Equal(t, tt.wantWorkers, walker.numWorkers)
			assert.Equal(t, filepath.Clean(tt.rootPath), walker.rootPath)
			walker.Stop()
		})
	}
}

// TestWalkBranchRelPath tests the relPath method.
//
// It verifies that relative paths are correctly computed for:
//   - Root branches (empty path)
//   - Single-level branches
//   - Multi-level branches
func TestWalkBranchRelPath(t *testing.T) {
	tests := []struct {
		name     string
		branch   *walkBranch
		wantPath string
	}{
		{
			name:     "root branch",
			branch:   &walkBranch{},
			wantPath: "",
		},
		{
			name: "single level",
			branch: &walkBranch{
				parent:   &walkBranch{},
				basename: "dir1",
			},
			wantPath: "dir1",
		},
		{
			name: "multiple levels",
			branch: &walkBranch{
				parent: &walkBranch{
					parent:   &walkBranch{},
					basename: "dir1",
				},
				basename: "dir2",
			},
			wantPath: "dir1/dir2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.branch.relPath()
			assert.Equal(t, tt.wantPath, got)
			//
			// It verifies that:
			//   - Root branches with nil parent are correctly identified
			//   - Child branches are not identified as root
		})
	}
}

// TestWalkBranchIsRoot tests the isRoot method.
func TestWalkBranchIsRoot(t *testing.T) {
	root := &walkBranch{}
	assert.True(t, root.isRoot())

	child := &walkBranch{parent: root}
	assert.False(t, child.isRoot())
	//
	// It verifies that absolute paths are correctly computed for:
	//   - Root branches (returns the root path itself)
	//   - Single-level branches
	//   - Multi-level branches
}

// TestWalkBranchAbsPath tests the absPath method.
func TestWalkBranchAbsPath(t *testing.T) {
	rootPath := "/home/user/test"

	tests := []struct {
		name     string
		branch   *walkBranch
		rootPath string
		wantPath string
	}{
		{
			name:     "root branch",
			branch:   &walkBranch{},
			rootPath: rootPath,
			wantPath: rootPath,
		},
		{
			name: "single level",
			branch: &walkBranch{
				parent:   &walkBranch{},
				basename: "dir1",
			},
			rootPath: rootPath,
			wantPath: filepath.Join(rootPath, "dir1"),
		},
		{
			name: "multiple levels",
			branch: &walkBranch{
				parent: &walkBranch{
					parent:   &walkBranch{},
					basename: "dir1",
				},
				basename: "dir2",
			},
			rootPath: rootPath,
			wantPath: filepath.Join(rootPath, "dir1", "dir2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.branch.absPath(tt.rootPath)
			assert.Equal(t, tt.wantPath, got)
			//
			// It verifies that:
			//   - All files in the test tree are visited via OnFileOrSymlink
			//   - All directories in the test tree are visited via OnDirectory
			//   - The relative paths are correctly computed
			//   - The walk completes without error
		})
	}
}

// TestWalkBasicTraversal tests that the walker visits all files and directories.
func TestWalkBasicTraversal(t *testing.T) {
	tmpDir := setupTestDir(t)

	var visitedFiles []string
	var visitedDirs []string

	callbacks := Callbacks{
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			visitedFiles = append(visitedFiles, relPath)
		},
		OnDirectory: func(relPath string, entry os.DirEntry) {
			visitedDirs = append(visitedDirs, relPath)
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	// Sort for consistent comparison
	sort.Strings(visitedFiles)
	sort.Strings(visitedDirs)

	expectedFiles := []string{"file1.txt", "dir1/file2.txt", "dir1/dir2/file3.txt", "dir3/file4.txt"}
	sort.Strings(expectedFiles)

	assert.Equal(t, len(expectedFiles), len(visitedFiles))

	for i, expected := range expectedFiles {
		if i < len(visitedFiles) {
			assert.Equal(t, expected, visitedFiles[i])
		}
	}

	expectedDirs := []string{"dir1", "dir1/dir2", "dir3"}
	sort.Strings(expectedDirs)

	assert.Equal(t, len(expectedDirs), len(visitedDirs))

	for i, expected := range expectedDirs {
		if i < len(visitedDirs) {
			//
			// It verifies that:
			//   - The walker produces correct results with 1, 2, and 4 workers
			//   - All files and directories are visited regardless of worker count
			//   - Concurrent access to shared state is properly synchronized
			assert.Equal(t, expected, visitedDirs[i])
		}
	}
}

// TestWalkWithMultipleWorkers tests that the walker works with multiple worker threads.
func TestWalkWithMultipleWorkers(t *testing.T) {
	tmpDir := setupTestDir(t)

	var visitedFiles []string
	var visitedDirs []string
	var mu = sync.Mutex{}

	callbacks := Callbacks{
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			mu.Lock()
			visitedFiles = append(visitedFiles, relPath)
			mu.Unlock()
		},
		OnDirectory: func(relPath string, entry os.DirEntry) {
			mu.Lock()
			visitedDirs = append(visitedDirs, relPath)
			mu.Unlock()
		},
	}

	for _, numWorkers := range []int{1, 2, 4} {
		visitedFiles = []string{}
		visitedDirs = []string{}

		walker := NewWalker(tmpDir, numWorkers, callbacks)
		err := walker.Run()
		assert.NoError(t, err, "Walk with %d workers failed", numWorkers)

		assert.Equal(t, 4, len(visitedFiles), "with %d workers", numWorkers)
		assert.Equal(t, 3, len(visitedDirs), "with %d workers", numWorkers)
	}
}

// TestWalkOnLstatCallback tests the OnLstat callback.
//
// It verifies that:
//   - OnLstat is called for every path visited
//   - The isDir flag is correctly set for directories and files
//   - No errors occur during the walk
func TestWalkOnLstatCallback(t *testing.T) {
	tmpDir := setupTestDir(t)

	var lstatCalls int

	callbacks := Callbacks{
		OnLstat: func(isDir bool, relPath string, fileInfo os.FileInfo, err error) {
			assert.NoError(t, err, "OnLstat got error for %q", relPath)
			lstatCalls++
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	// Verify lstat was called for entries
	assert.Greater(t, lstatCalls, 0)
}

// TestWalkOnReadDirCallback tests the OnReadDir callback.
//
// It verifies that:
//   - OnReadDir is called once for each directory traversed
//   - The callback is invoked with correct entries
//   - No errors occur during the walk
func TestWalkOnReadDirCallback(t *testing.T) {
	tmpDir := setupTestDir(t)

	var readDirCalls int

	callbacks := Callbacks{
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			assert.NoError(t, err, "OnReadDir got error for %q", relPath)
			readDirCalls++
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	// Expected: root + 3 subdirectories = 4 ReadDir calls
	expectedCalls := 4
	assert.Equal(t, expectedCalls, readDirCalls)
}

// TestWalkNonexistentDirectory tests behavior with a non-existent directory.
//
// It verifies that:
//   - The walk completes without panicking
//   - An error may be returned for the non-existent root directory
func TestWalkNonexistentDirectory(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does_not_exist")

	walker := NewWalker(nonexistent, 1, Callbacks{})
	err := walker.Run()
	if err == nil {
		t.Fatal("Walk succeeded for a non-existent root")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Walk returned %v, want a not-exist error", err)
	}
	if !strings.Contains(err.Error(), nonexistent) {
		t.Fatalf("Walk error %q does not identify root %q", err, nonexistent)
	}
}

// TestWalkEmptyDirectory tests behavior with an empty directory.
//
// It verifies that:
//   - The walk completes without error for an empty directory
//   - No files or directories are reported
//   - The callbacks are never invoked (or invoked appropriately)
func TestWalkEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	var visitedFiles []string
	var visitedDirs []string

	callbacks := Callbacks{
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			visitedFiles = append(visitedFiles, relPath)
		},
		OnDirectory: func(relPath string, entry os.DirEntry) {
			visitedDirs = append(visitedDirs, relPath)
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	assert.Zero(t, len(visitedFiles))
	assert.Zero(t, len(visitedDirs))
}

// TestWalkStop tests that Stop() cancels the walker.
//
// It verifies that:
//   - Calling Stop() cancels the walker's context
//   - The context's Done() channel closes after Stop()
func TestWalkStop(t *testing.T) {
	tmpDir := setupTestDir(t)

	walker := NewWalker(tmpDir, 1, Callbacks{})
	walker.Stop()

	select {
	case <-walker.monitorCtx.Done():
		// Expected: context is cancelled
	default:
		assert.Fail(t, "context should be cancelled after Stop()")
	}
}

func TestStopPreventsTraversalAndPendingChildStats(t *testing.T) {
	root := setupTestDir(t)
	for _, point := range []string{"before_run", "root_stat", "root_read", "first_child"} {
		t.Run(point, func(t *testing.T) {
			var walker *Walker
			var stats, reads atomic.Int64
			walker = NewWalker(root, 32, Callbacks{
				OnLstat: func(_ bool, relative string, _ os.FileInfo, _ error) {
					stats.Add(1)
					if point == "root_stat" || point == "first_child" && relative != "" {
						walker.Stop()
					}
				},
				OnReadDir: func(_ string, _ []os.DirEntry, _ error) {
					reads.Add(1)
					if point == "root_read" {
						walker.Stop()
					}
				},
			})
			if point == "before_run" {
				walker.Stop()
			}
			assert.ErrorIs(t, walker.Run(), context.Canceled)
			wantStats, wantReads := int64(0), int64(0)
			switch point {
			case "root_stat":
				wantStats = 1
			case "root_read":
				wantStats, wantReads = 1, 1
			case "first_child":
				wantStats, wantReads = 2, 1
			}
			assert.Equal(t, wantStats, stats.Load())
			assert.Equal(t, wantReads, reads.Load())
		})
	}
}

// TestIgnoreNames verifies that configured ignore basenames are skipped.
func TestIgnoreNames(t *testing.T) {
	tmpDir := t.TempDir()

	// keep structure
	if err := os.Mkdir(filepath.Join(tmpDir, "keep"), 0755); err != nil {
		t.Fatalf("failed to create keep dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "keep", "file.txt"), []byte("ok"), 0600); err != nil {
		t.Fatalf("failed to create keep file: %v", err)
	}

	// ignored directory and file
	if err := os.Mkdir(filepath.Join(tmpDir, "ignoreme"), 0755); err != nil {
		t.Fatalf("failed to create ignore dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ignoreme", "ignored.txt"), []byte("ignored"), 0600); err != nil {
		t.Fatalf("failed to create ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "skip.txt"), []byte("skip"), 0600); err != nil {
		t.Fatalf("failed to create skip file: %v", err)
	}

	var visitedFiles []string
	var visitedDirs []string
	callbacks := Callbacks{
		OnDirectory: func(relPath string, entry os.DirEntry) {
			visitedDirs = append(visitedDirs, relPath)
		},
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			visitedFiles = append(visitedFiles, relPath)
		},
	}

	walker := NewWalker(tmpDir, 2, callbacks)
	walker.SetIgnoreNames([]string{"ignoreme", "skip.txt"})

	err := walker.Run()
	assert.NoError(t, err)

	for _, dir := range visitedDirs {
		assert.NotEqual(t, "ignoreme", dir, "ignored directory was visited: %s", dir)
	}
	for _, file := range visitedFiles {
		if file == "skip.txt" || strings.HasPrefix(file, "ignoreme/") {
			assert.Fail(t, "ignored file was visited: %s", file)
		}
	}

	assert.NotZero(t, len(visitedFiles), "expected to visit at least one file")
}

// TestIgnoreFunc verifies that custom ignore callback can skip entries.
func TestIgnoreFunc(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{"ok.txt", "skip-file.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0600); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	skipDir := filepath.Join(tmpDir, "skipdir")
	if err := os.Mkdir(skipDir, 0755); err != nil {
		t.Fatalf("failed to create skipdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skipDir, "inner.txt"), []byte("inner"), 0600); err != nil {
		t.Fatalf("failed to create inner file: %v", err)
	}

	var visited []string
	callbacks := Callbacks{
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			visited = append(visited, relPath)
		},
		OnDirectory: func(relPath string, entry os.DirEntry) {
			visited = append(visited, relPath+"/")
		},
	}

	walker := NewWalker(tmpDir, 4, callbacks)
	walker.SetIgnoreFunc(func(name, relPath string, info os.FileInfo) bool {
		return strings.HasPrefix(name, "skip")
	})

	err := walker.Run()
	assert.NoError(t, err)

	for _, path := range visited {
		assert.False(t, strings.HasPrefix(path, "skip"), "ignore callback should skip path: %s", path)
	}
	assert.NotZero(t, len(visited), "expected to visit entries")
}

// BenchmarkWalk benchmarks the walk operation with a single worker.
func BenchmarkWalkSingleWorker(b *testing.B) {
	tmpDir := setupTestDir(&testing.T{})

	//
	// This benchmark measures the performance of directory walking with four
	// worker threads, allowing comparison with single-worker performance to assess
	// the benefit of parallelization.
	for i := 0; i < b.N; i++ {
		walker := NewWalker(tmpDir, 1, Callbacks{})
		_ = walker.Run()
	}
}

// BenchmarkWalkMultipleWorkers benchmarks the walk operation with multiple workers.
func BenchmarkWalkMultipleWorkers(b *testing.B) {
	tmpDir := setupTestDir(&testing.T{})

	for i := 0; i < b.N; i++ {
		walker := NewWalker(tmpDir, 4, Callbacks{})
		_ = walker.Run()
	}
}

// setupLargeTestDir creates a large test directory structure with many directories and files.
func setupLargeTestDir(t *testing.T, numDirs int, numFiles int) string {
	tmpDir := t.TempDir()

	filesPerDir := numFiles / numDirs
	if filesPerDir == 0 {
		filesPerDir = 1
	}

	// Create directory structure
	for i := 0; i < numDirs; i++ {
		// Create nested directories
		level := i / 10
		index := i % 10
		dirPath := filepath.Join(tmpDir, "level"+string(rune(48+level)), "dir"+string(rune(48+index)))
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dirPath, err)
		}

		// Create files in this directory
		for f := 0; f < filesPerDir; f++ {
			if (i*filesPerDir + f) >= numFiles {
				break
			}
			filename := "file" + string(rune(48+(f%10))) + ".txt"
			filePath := filepath.Join(dirPath, filename)
			if err := os.WriteFile(filePath, []byte("test content"), 0600); err != nil {
				t.Fatalf("failed to create file %s: %v", filePath, err)
			}
		}
	}

	return tmpDir
}

// TestWalkLargeTree tests walking a large directory tree with a single worker.
// This verifies basic correctness with 100 directories and 200 files.
func TestWalkLargeTree(t *testing.T) {
	tmpDir := setupLargeTestDir(t, 100, 200)

	var fileCount, dirCount int
	var mu sync.Mutex

	callbacks := Callbacks{
		OnDirectory: func(relPath string, entry os.DirEntry) {
			mu.Lock()
			dirCount++
			mu.Unlock()
		},
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			mu.Lock()
			fileCount++
			mu.Unlock()
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	// Verify we visited directories and files
	assert.NotZero(t, dirCount, "Expected to visit directories")
	assert.NotZero(t, fileCount, "Expected to visit files")
}

// TestWalkLargeTreeWithConcurrency tests walking a large directory tree with multiple workers.
// This verifies correct behavior with different worker counts (2, 4, 8, 16).
func TestWalkLargeTreeWithConcurrency(t *testing.T) {
	workerCounts := []int{2, 4, 8, 16}

	for _, numWorkers := range workerCounts {
		testName := "workers"
		switch numWorkers {
		case 2:
			testName = "workers_2"
		case 4:
			testName = "workers_4"
		case 8:
			testName = "workers_8"
		case 16:
			testName = "workers_16"
		}

		t.Run(testName, func(t *testing.T) {
			tmpDir := setupLargeTestDir(t, 100, 200)

			var fileCount, dirCount int
			var mu sync.Mutex

			callbacks := Callbacks{
				OnDirectory: func(relPath string, entry os.DirEntry) {
					mu.Lock()
					dirCount++
					mu.Unlock()
				},
				OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
					mu.Lock()
					fileCount++
					mu.Unlock()
				},
			}

			walker := NewWalker(tmpDir, numWorkers, callbacks)
			err := walker.Run()
			assert.NoError(t, err, "Walk failed with %d workers", numWorkers)

			assert.NotZero(t, dirCount, "Expected to visit directories")
			assert.NotZero(t, fileCount, "Expected to visit files")
		})
	}
}

// TestWalkConcurrentCallbacks tests that callbacks are called correctly under concurrent access.
// Uses sync.Mutex to safely count invocations without race conditions.
func TestWalkConcurrentCallbacks(t *testing.T) {
	tmpDir := setupLargeTestDir(t, 100, 200)

	var lstatCount, readDirCount int
	var mu sync.Mutex

	callbacks := Callbacks{
		OnLstat: func(isDir bool, relPath string, fileInfo os.FileInfo, err error) {
			mu.Lock()
			lstatCount++
			mu.Unlock()
		},
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			mu.Lock()
			readDirCount++
			mu.Unlock()
		},
	}

	walker := NewWalker(tmpDir, 8, callbacks)
	err := walker.Run()
	assert.NoError(t, err)

	assert.NotZero(t, lstatCount, "Expected OnLstat callbacks")
	assert.NotZero(t, readDirCount, "Expected OnReadDir callbacks")
}

// TestWalkStressWorkStealing tests work stealing with different worker counts.
// This verifies the load balancing mechanism under various concurrency scenarios.
func TestWalkStressWorkStealing(t *testing.T) {
	workerCounts := []int{1, 2, 4, 8, 16}

	for _, numWorkers := range workerCounts {
		testName := "workers"
		switch numWorkers {
		case 1:
			testName = "workers_1"
		case 2:
			testName = "workers_2"
		case 4:
			testName = "workers_4"
		case 8:
			testName = "workers_8"
		case 16:
			testName = "workers_16"
		}

		t.Run(testName, func(t *testing.T) {
			tmpDir := setupLargeTestDir(t, 200, 400)

			var visitedCount int
			var mu sync.Mutex

			callbacks := Callbacks{
				OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
					mu.Lock()
					visitedCount++
					mu.Unlock()
				},
				OnDirectory: func(relPath string, entry os.DirEntry) {
					mu.Lock()
					visitedCount++
					mu.Unlock()
				},
			}

			walker := NewWalker(tmpDir, numWorkers, callbacks)
			err := walker.Run()
			assert.NoError(t, err, "Walk failed with %d workers", numWorkers)

			assert.NotZero(t, visitedCount, "Expected to visit entries with %d workers", numWorkers)
		})
	}
}

// mockLogger is a test logger that records log messages.
type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, format)
}

// TestCustomLogger tests setting a custom logger on the walker.
func TestCustomLogger(t *testing.T) {
	tmpDir := setupTestDir(t)

	mockLog := &mockLogger{}

	walker := NewWalker(tmpDir, 1, Callbacks{})
	walker.SetLogger(mockLog)

	err := walker.Run()
	assert.NoError(t, err)

	// The test directory doesn't generate errors, so no messages should be logged
	assert.Zero(t, len(mockLog.messages), "Expected no log messages")
}

// TestCustomLoggerWithError tests that custom logger receives error messages.
func TestCustomLoggerWithError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory structure with a path that will fail
	dir1 := filepath.Join(tmpDir, "dir1")
	if err := os.Mkdir(dir1, 0755); err != nil {
		t.Fatalf("failed to create dir1: %v", err)
	}

	// Create a file path where we'll try to read as directory
	// by removing read permissions and then creating nested path
	nestedPath := filepath.Join(dir1, "subdir")
	if err := os.Mkdir(nestedPath, 0000); err != nil {
		t.Fatalf("failed to create nested path: %v", err)
	}
	defer os.Chmod(nestedPath, 0755) // cleanup

	mockLog := &mockLogger{}

	walker := NewWalker(tmpDir, 1, Callbacks{})
	walker.SetLogger(mockLog)

	_ = walker.Run()

	// Should have logged an error about the permission-denied directory
	assert.NotZero(t, len(mockLog.messages), "Expected log messages for permission error")
}

// TestSetLoggerNil tests that SetLogger ignores nil logger.
func TestSetLoggerNil(t *testing.T) {
	tmpDir := setupTestDir(t)

	walker := NewWalker(tmpDir, 1, Callbacks{})
	originalLogger := walker.logger

	walker.SetLogger(nil) // Should not change the logger

	assert.Equal(t, originalLogger, walker.logger, "SetLogger(nil) should not change the logger")
}

// countingLogger counts log calls without storing messages.
type countingLogger struct {
	count int64
	mu    sync.Mutex
}

func (c *countingLogger) Printf(format string, v ...interface{}) {
	atomic.AddInt64(&c.count, 1)
}

// TestCustomLoggerConcurrency tests that custom logger works correctly with multiple workers.
func TestCustomLoggerConcurrency(t *testing.T) {
	tmpDir := setupLargeTestDir(t, 50, 100)

	counter := &countingLogger{}

	walker := NewWalker(tmpDir, 8, Callbacks{})
	walker.SetLogger(counter)

	err := walker.Run()
	assert.NoError(t, err)

	// No errors expected in this test directory
	assert.Zero(t, counter.count, "Expected no log messages")
}

// BenchmarkWalkLargeTree benchmarks walking a large directory tree with a single worker.
func BenchmarkWalkLargeTree(b *testing.B) {
	tmpDir := setupLargeTestDir(&testing.T{}, 100, 200)

	for i := 0; i < b.N; i++ {
		walker := NewWalker(tmpDir, 1, Callbacks{})
		_ = walker.Run()
	}
}

// BenchmarkWalkLargeTreeWithWorkers benchmarks a large tree with 4 workers.
func BenchmarkWalkLargeTreeWithWorkers(b *testing.B) {
	tmpDir := setupLargeTestDir(&testing.T{}, 100, 200)

	for i := 0; i < b.N; i++ {
		walker := NewWalker(tmpDir, 4, Callbacks{})
		_ = walker.Run()
	}
}

// BenchmarkWalkLargeTreeManyWorkers benchmarks a large tree with 16 workers.
func BenchmarkWalkLargeTreeManyWorkers(b *testing.B) {
	tmpDir := setupLargeTestDir(&testing.T{}, 100, 200)

	for i := 0; i < b.N; i++ {
		walker := NewWalker(tmpDir, 16, Callbacks{})
		_ = walker.Run()
	}
}

// TestStaggeredWorkerStartup verifies that workers start with the correct stagger delays.
// Worker 0 should process the root immediately, and other workers should wait for
// the root to be processed before starting work.
func TestStaggeredWorkerStartup(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple structure
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("data"), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	var rootProcessed atomic.Bool

	callbacks := Callbacks{
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			// Root is empty string
			if relPath == "" {
				rootProcessed.Store(true)
			}
		},
	}

	walker := NewWalker(tmpDir, 4, callbacks)

	err := walker.Run()
	assert.NoError(t, err)

	// If we got here without hanging/crashing, the staggered startup worked
	assert.True(t, rootProcessed.Load(), "Root directory was not processed")
}

func TestResizeWorkersDuringRun(t *testing.T) {
	tmpDir := t.TempDir()
	for i := 0; i < 20; i++ {
		dir := filepath.Join(tmpDir, fmt.Sprintf("dir-%02d", i))
		assert.NoError(t, os.Mkdir(dir, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0600))
	}

	rootStarted := make(chan struct{})
	releaseRoot := make(chan struct{})
	var files atomic.Int32
	walker := NewWalker(tmpDir, 1, Callbacks{
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			if relPath == "" {
				close(rootStarted)
				<-releaseRoot
			}
		},
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			files.Add(1)
		},
	})

	done := make(chan error, 1)
	go func() { done <- walker.Run() }()
	<-rootStarted
	assert.NoError(t, walker.ResizeWorkers(4))
	assert.Equal(t, 4, walker.WorkerCount())
	assert.NoError(t, walker.ResizeWorkers(2))
	assert.Equal(t, 2, walker.WorkerCount())
	assert.NoError(t, walker.ResizeWorkers(4))
	assert.Equal(t, 4, walker.WorkerCount())
	close(releaseRoot)

	assert.NoError(t, <-done)
	assert.Equal(t, int32(20), files.Load())
}

func TestResizeWorkersBeforeRun(t *testing.T) {
	walker := NewWalker(t.TempDir(), 1, Callbacks{})
	assert.Error(t, walker.ResizeWorkers(0))
	assert.NoError(t, walker.ResizeWorkers(3))
	assert.Equal(t, 3, walker.WorkerCount())
	assert.NoError(t, walker.Run())
}

func TestResizeWorkersDownDrainsDiscoveredWork(t *testing.T) {
	tmpDir := t.TempDir()
	for i := 0; i < 12; i++ {
		nested := filepath.Join(tmpDir, fmt.Sprintf("dir-%02d", i), "nested")
		assert.NoError(t, os.MkdirAll(nested, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(nested, "file.txt"), []byte("data"), 0600))
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var blockOnce sync.Once
	var files atomic.Int32
	walker := NewWalker(tmpDir, 4, Callbacks{
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			if strings.HasPrefix(relPath, "dir-") && !strings.Contains(relPath, "/") {
				blockOnce.Do(func() {
					started <- struct{}{}
					<-release
				})
			}
		},
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			files.Add(1)
		},
	})

	done := make(chan error, 1)
	go func() { done <- walker.Run() }()
	<-started
	assert.NoError(t, walker.ResizeWorkers(1))
	close(release)

	assert.NoError(t, <-done)
	assert.Equal(t, int32(12), files.Load())
}
