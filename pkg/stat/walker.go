// Package stat provides directory statistics collection and analysis.
//
// It uses the cwalk package for parallel directory traversal and provides
// flexible filtering, aggregation by multiple dimensions (summary, per-year,
// per-uid), and thread-safe concurrent processing.
package stat

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	cwalk "github.com/otuschhoff/cwalk"
)

// FileInfo holds aggregated file information for a single filesystem entry.
type FileInfo struct {
	Path      string      // Absolute path to the file
	Size      int64       // Size in bytes
	Mode      os.FileMode // File mode and permissions
	ModTime   time.Time   // Last modification time
	IsDir     bool        // True if entry is a directory
	IsSymlink bool        // True if entry is a symbolic link
	UID       uint32      // User ID of the owner
	GID       uint32      // Group ID of the owner
}

// Results holds all aggregated statistics from a directory walk.
// It provides multiple dimensions of analysis: summary totals, per-year breakdown,
// and per-UID (owner) breakdown.
type Results struct {
	Summary      *SummaryStat
	ByYear       map[int]*YearStat   // Year -> stats
	ByUID        map[uint32]*UIDStat // UID -> stats
	TotalFiles   map[string]int64    // Type -> count
	TotalSize    map[string]int64    // Type -> size
	TotalInodes  map[string]int64    // Type -> inode count
	AllFileInfos []FileInfo          // For detailed analysis
}

// SummaryStat holds aggregate statistics across all files.
// It includes counts and sizes for each inode type.
type SummaryStat struct {
	TotalSize    int64 // Total size of all files in bytes
	TotalInodes  int64 // Total count of all inodes
	Files        int64 // Count of regular files
	Dirs         int64 // Count of directories
	Symlinks     int64 // Count of symbolic links
	Others       int64 // Count of other inode types
	FilesSize    int64 // Total size of regular files
	DirsSize     int64 // Total size of directories (usually 0 or block size)
	SymlinksSize int64 // Total size of symbolic links
	OthersSize   int64 // Total size of other inode types
}

// YearStat holds statistics grouped by modification year.
// Provides breakdown of file counts and sizes for files modified in a specific year.
type YearStat struct {
	Year         int   // Calendar year (e.g., 2024)
	TotalSize    int64 // Total size of files modified in this year
	TotalInodes  int64 // Total count of inodes modified in this year
	Files        int64 // Count of regular files
	Dirs         int64 // Count of directories
	Symlinks     int64 // Count of symbolic links
	Others       int64 // Count of other inode types
	FilesSize    int64 // Total size of regular files
	DirsSize     int64 // Total size of directories
	SymlinksSize int64 // Total size of symbolic links
	OthersSize   int64 // Total size of other inode types
}

// UIDStat holds statistics grouped by file owner (UID).
// Provides breakdown of file counts and sizes for each user.
type UIDStat struct {
	UID          uint32 // User ID of the file owner
	Username     string // Login name of the user (if resolvable)
	TotalSize    int64  // Total size of files owned by this user
	TotalInodes  int64  // Total count of inodes owned by this user
	Files        int64  // Count of regular files
	Dirs         int64  // Count of directories
	Symlinks     int64  // Count of symbolic links
	Others       int64  // Count of other inode types
	FilesSize    int64  // Total size of regular files
	DirsSize     int64  // Total size of directories
	SymlinksSize int64  // Total size of symbolic links
	OthersSize   int64  // Total size of other inode types
}

// StatsWalker performs parallel directory traversal with statistics collection.
// It applies filters to entries and aggregates statistics across multiple dimensions.
// Safe for concurrent use via mutex-protected results aggregation.
type StatsWalker struct {
	paths   []string   // Directories to walk
	workers int        // Number of parallel workers
	filters *Filters   // Filters to apply during walk
	results *Results   // Aggregated results (protected by mu)
	mu      sync.Mutex // Protects concurrent access to results
}

// NewStatsWalker creates a new statistics walker for the given paths with filters.
// The workers parameter controls parallelism; typical values are 1-8.
// If filters is nil, all entries are included.
func NewStatsWalker(paths []string, workers int, filters *Filters) *StatsWalker {
	return &StatsWalker{
		paths:   paths,
		workers: workers,
		filters: filters,
		results: &Results{
			Summary:      &SummaryStat{},
			ByYear:       make(map[int]*YearStat),
			ByUID:        make(map[uint32]*UIDStat),
			TotalFiles:   make(map[string]int64),
			TotalSize:    make(map[string]int64),
			TotalInodes:  make(map[string]int64),
			AllFileInfos: []FileInfo{},
		},
	}
}

// Walk performs the directory walk and collects statistics.
// It walks all configured paths, applies filters, aggregates statistics,
// and returns the Results object. Returns an error if directory traversal fails.
func (sw *StatsWalker) Walk() (*Results, error) {
	// Walk each path
	for _, rootPath := range sw.paths {
		if err := sw.walkPath(rootPath); err != nil {
			return nil, err
		}
	}

	// Calculate summary from all collected data
	sw.calculateSummary()

	return sw.results, nil
}

// walkPath walks a single directory tree using cwalk with the configured workers.
// It calls the OnLstat callback for each entry, applying filters and aggregating statistics.
func (sw *StatsWalker) walkPath(rootPath string) error {
	callbacks := cwalk.Callbacks{
		OnLstat: func(isDir bool, relPath string, info os.FileInfo, err error) {
			if err != nil {
				return
			}
			if info == nil {
				return
			}

			// Extract file info
			fi := FileInfo{
				Path:    relPath,
				Size:    info.Size(),
				Mode:    info.Mode(),
				ModTime: info.ModTime(),
				IsDir:   info.IsDir(),
			}

			// Check if symlink
			if info.Mode()&os.ModeSymlink != 0 {
				fi.IsSymlink = true
			}

			// Get UID/GID from syscall.Stat_t
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				fi.UID = stat.Uid
				fi.GID = stat.Gid
			}

			// Apply filters
			if !sw.filters.Matches(&fi) {
				return
			}

			sw.mu.Lock()
			defer sw.mu.Unlock()

			// Record the file info
			sw.results.AllFileInfos = append(sw.results.AllFileInfos, fi)

			// Determine type
			fileType := "other"
			if fi.IsDir {
				fileType = "dir"
			} else if fi.IsSymlink {
				fileType = "symlink"
			} else {
				fileType = "file"
			}

			// Update counts
			sw.results.TotalFiles[fileType]++
			sw.results.TotalSize[fileType] += fi.Size
			sw.results.TotalInodes[fileType]++

			// Update year stats
			year := fi.ModTime.Year()
			if _, ok := sw.results.ByYear[year]; !ok {
				sw.results.ByYear[year] = &YearStat{Year: year}
			}
			ys := sw.results.ByYear[year]
			ys.TotalInodes++
			ys.TotalSize += fi.Size
			switch fileType {
			case "file":
				ys.Files++
				ys.FilesSize += fi.Size
			case "dir":
				ys.Dirs++
				ys.DirsSize += fi.Size
			case "symlink":
				ys.Symlinks++
				ys.SymlinksSize += fi.Size
			default:
				ys.Others++
				ys.OthersSize += fi.Size
			}

			// Update UID stats
			if _, ok := sw.results.ByUID[fi.UID]; !ok {
				username := lookupUsername(fi.UID)
				sw.results.ByUID[fi.UID] = &UIDStat{
					UID:      fi.UID,
					Username: username,
				}
			}
			us := sw.results.ByUID[fi.UID]
			us.TotalInodes++
			us.TotalSize += fi.Size
			switch fileType {
			case "file":
				us.Files++
				us.FilesSize += fi.Size
			case "dir":
				us.Dirs++
				us.DirsSize += fi.Size
			case "symlink":
				us.Symlinks++
				us.SymlinksSize += fi.Size
			default:
				us.Others++
				us.OthersSize += fi.Size
			}
		},
	}

	walker := cwalk.NewWalker(rootPath, sw.workers, callbacks)
	return walker.Run()
}

func (sw *StatsWalker) calculateSummary() {
	sum := sw.results.Summary

	for _, count := range sw.results.TotalInodes {
		sum.TotalInodes += count
	}

	for _, size := range sw.results.TotalSize {
		sum.TotalSize += size
	}

	sum.Files = sw.results.TotalFiles["file"]
	sum.Dirs = sw.results.TotalFiles["dir"]
	sum.Symlinks = sw.results.TotalFiles["symlink"]
	sum.Others = sw.results.TotalFiles["other"]

	sum.FilesSize = sw.results.TotalSize["file"]
	sum.DirsSize = sw.results.TotalSize["dir"]
	sum.SymlinksSize = sw.results.TotalSize["symlink"]
	sum.OthersSize = sw.results.TotalSize["other"]
}

// lookupUsername resolves a UID to a username.
// Returns a string like "username" on success, or "uid:1000" on lookup failure.
func lookupUsername(uid uint32) string {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return fmt.Sprintf("uid:%d", uid)
	}
	return u.Username
}
