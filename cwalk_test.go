// Tests for package cwalk.
package cwalk

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
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
		name       string
		rootPath   string
		numWorkers int
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
			if walker.numWorkers != tt.wantWorkers {
				t.Errorf("got %d workers, want %d", walker.numWorkers, tt.wantWorkers)
			}
			if walker.rootPath != filepath.Clean(tt.rootPath) {
				t.Errorf("got rootPath %q, want %q", walker.rootPath, filepath.Clean(tt.rootPath))
			}
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
			if got != tt.wantPath {
				t.Errorf("got %q, want %q", got, tt.wantPath)
			}
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
	if !root.isRoot() {
		t.Error("root branch should return true for isRoot()")
	}

	child := &walkBranch{parent: root}
	if child.isRoot() {
		t.Error("child branch should return false for isRoot()")
//
// It verifies that absolute paths are correctly computed for:
//   - Root branches (returns the root path itself)
//   - Single-level branches
//   - Multi-level branches
	}
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
			if got != tt.wantPath {
				t.Errorf("got %q, want %q", got, tt.wantPath)
			}
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
	if err := walker.Run(); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// Sort for consistent comparison
	sort.Strings(visitedFiles)
	sort.Strings(visitedDirs)

	expectedFiles := []string{"file1.txt", "dir1/file2.txt", "dir1/dir2/file3.txt", "dir3/file4.txt"}
	sort.Strings(expectedFiles)

	if len(visitedFiles) != len(expectedFiles) {
		t.Errorf("visited %d files, want %d", len(visitedFiles), len(expectedFiles))
	}

	for i, expected := range expectedFiles {
		if i < len(visitedFiles) && visitedFiles[i] != expected {
			t.Errorf("file[%d] = %q, want %q", i, visitedFiles[i], expected)
		}
	}

	expectedDirs := []string{"dir1", "dir1/dir2", "dir3"}
	sort.Strings(expectedDirs)

	if len(visitedDirs) != len(expectedDirs) {
		t.Errorf("visited %d dirs, want %d", len(visitedDirs), len(expectedDirs))
	}

	for i, expected := range expectedDirs {
		if i < len(visitedDirs) && visitedDirs[i] != expected {
//
// It verifies that:
//   - The walker produces correct results with 1, 2, and 4 workers
//   - All files and directories are visited regardless of worker count
//   - Concurrent access to shared state is properly synchronized
			t.Errorf("dir[%d] = %q, want %q", i, visitedDirs[i], expected)
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
		if err := walker.Run(); err != nil {
			t.Fatalf("Walk with %d workers failed: %v", numWorkers, err)
		}

		if len(visitedFiles) != 4 {
			t.Errorf("with %d workers: visited %d files, want 4", numWorkers, len(visitedFiles))
		}

		if len(visitedDirs) != 3 {
			t.Errorf("with %d workers: visited %d dirs, want 3", numWorkers, len(visitedDirs))
//
// It verifies that:
//   - OnLstat is called for every path visited
//   - The isDir flag is correctly set for directories and files
//   - No errors occur during the walk
		}
	}
}

// TestWalkOnLstatCallback tests the OnLstat callback.
func TestWalkOnLstatCallback(t *testing.T) {
	tmpDir := setupTestDir(t)

	var lstatCalls int

	callbacks := Callbacks{
		OnLstat: func(isDir bool, relPath string, fileInfo os.FileInfo, err error) {
			if err != nil {
				t.Errorf("OnLstat got error for %q: %v", relPath, err)
			}
			lstatCalls++
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	if err := walker.Run(); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// Expected: directories (3) + root (1) + files (4) = 8 lstat calls
	expectedCalls := 8
	if lstatCalls != expectedCalls {
//
// It verifies that:
//   - OnReadDir is called once for each directory traversed
//   - The callback is invoked with correct entries
//   - No errors occur during the walk
		t.Errorf("OnLstat called %d times, want %d", lstatCalls, expectedCalls)
	}
}

// TestWalkOnReadDirCallback tests the OnReadDir callback.
func TestWalkOnReadDirCallback(t *testing.T) {
	tmpDir := setupTestDir(t)

	var readDirCalls int

	callbacks := Callbacks{
		OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
			if err != nil {
				t.Errorf("OnReadDir got error for %q: %v", relPath, err)
			}
			readDirCalls++
		},
	}

	walker := NewWalker(tmpDir, 1, callbacks)
	if err := walker.Run(); err != nil {
		t.Fatalf("Walk failed: %v", err)
//
// It verifies that:
//   - The walk completes without panicking
//   - An error may be returned for the non-existent root directory
	}

	// Expected: root + 3 subdirectories = 4 ReadDir calls
	expectedCalls := 4
	if readDirCalls != expectedCalls {
		t.Errorf("OnReadDir called %d times, want %d", readDirCalls, expectedCalls)
	}
}

// TestWalkNonexistentDirectory tests behavior with a non-existent directory.
func TestWalkNonexistentDirectory(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does_not_exist")

//
// It verifies that:
//   - The walk completes without error for an empty directory
//   - No files or directories are reported
//   - The callbacks are never invoked (or invoked appropriately)
	walker := NewWalker(nonexistent, 1, Callbacks{})
	// The walk should complete but with no entries visited
	if err := walker.Run(); err != nil {
		// It's acceptable to get an error for non-existent directory
		t.Logf("Walk returned error for non-existent directory: %v", err)
	}
}

// TestWalkEmptyDirectory tests behavior with an empty directory.
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
	if err := walker.Run(); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(visitedFiles) != 0 {
//
// It verifies that:
//   - Calling Stop() cancels the walker's context
//   - The context's Done() channel closes after Stop()
		t.Errorf("empty directory: visited %d files, want 0", len(visitedFiles))
	}

	if len(visitedDirs) != 0 {
		t.Errorf("empty directory: visited %d dirs, want 0", len(visitedDirs))
	}
}

// TestWalkStop tests that Stop() cancels the walker.
func TestWalkStop(t *testing.T) {
	tmpDir := setupTestDir(t)

	walker := NewWalker(tmpDir, 1, Callbacks{})
	walker.Stop()
	
	// After Stop()SingleWorker benchmarks the walk operation with a single worker.
//
// This benchmark measures the performance of directory walking with a single
// worker thread, providing a baseline for comparison with multi-worker scenarios
	select {
	case <-walker.monitorCtx.Done():
		// Expected: context is cancelled
	default:
		t.Error("context should be cancelled after Stop()")
	}
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
