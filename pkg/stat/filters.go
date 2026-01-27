package stat

import (
	"regexp"
	"time"
)

// Filters holds all filtering criteria for directory walk results.
//
// All filter fields are optional (zero value means no filtering for that dimension).
// Multiple filters are combined with AND logic: a FileInfo must pass all active filters
// to be included in results. Filter fields can be safely left unset for unused criteria.
type Filters struct {
	// Type filtering - map of inode types to include (e.g., "file", "dir", "symlink")
	Types map[string]bool // "file", "dir", "symlink", "other"

	// Time filtering - modification time bounds relative to current time
	MtimeOlderThan   *time.Duration // Include files modified older than this duration
	MtimeYoungerThan *time.Duration // Include files modified younger than this duration

	// Size filtering - file size bounds
	SizeMin *int64 // Minimum file size in bytes
	SizeMax *int64 // Maximum file size in bytes

	// Name filtering - regex pattern for filename matching
	NameRegex *regexp.Regexp

	// User/Group filtering - owner criteria
	Usernames  []string // List of usernames to include
	UIDs       []uint32 // List of user IDs to include
	Groupnames []string // List of group names to include (not implemented)
	GIDs       []uint32 // List of group IDs to include

	// Permission filtering - permission bit matching
	PermsHas uint32 // File must have ALL these permission bits
	PermsNot uint32 // File must NOT have ANY of these permission bits
}

// Matches checks if a FileInfo passes all active filters.
// Returns true only if the file passes all enabled filter criteria.
// Filters are combined with AND logic: all must pass for a match.
func (f *Filters) Matches(fi *FileInfo) bool {
	// Type filter
	if len(f.Types) > 0 {
		fileType := getFileType(fi)
		if !f.Types[fileType] {
			return false
		}
	}

	// Mtime filters
	now := time.Now()

	if f.MtimeOlderThan != nil {
		cutoff := now.Add(-*f.MtimeOlderThan)
		if fi.ModTime.After(cutoff) {
			return false // File is too new
		}
	}

	if f.MtimeYoungerThan != nil {
		cutoff := now.Add(-*f.MtimeYoungerThan)
		if fi.ModTime.Before(cutoff) {
			return false // File is too old
		}
	}

	// Size filters
	if f.SizeMin != nil && fi.Size < *f.SizeMin {
		return false
	}

	if f.SizeMax != nil && fi.Size > *f.SizeMax {
		return false
	}

	// Name filter
	if f.NameRegex != nil {
		// Extract filename from path
		filename := ""
		path := fi.Path
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == '/' {
				filename = path[i+1:]
				break
			}
		}
		if filename == "" {
			filename = path
		}

		if !f.NameRegex.MatchString(filename) {
			return false
		}
	}

	// UID filter
	if len(f.UIDs) > 0 {
		found := false
		for _, uid := range f.UIDs {
			if fi.UID == uid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// GID filter
	if len(f.GIDs) > 0 {
		found := false
		for _, gid := range f.GIDs {
			if fi.GID == gid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Permission filters
	if f.PermsHas != 0 {
		mode := fi.Mode.Perm()
		if (uint32(mode) & f.PermsHas) != f.PermsHas {
			return false
		}
	}

	if f.PermsNot != 0 {
		mode := fi.Mode.Perm()
		if (uint32(mode) & f.PermsNot) != 0 {
			return false
		}
	}

	// Note: Username and Groupname filters are applied separately
	// during the aggregation since they require lookups
	_ = f.Usernames
	_ = f.Groupnames

	return true
}

// getFileType determines the type classification of a FileInfo entry.
// Returns one of: "dir", "symlink", or "file".
func getFileType(fi *FileInfo) string {
	if fi.IsDir {
		return "dir"
	}
	if fi.IsSymlink {
		return "symlink"
	}
	if fi.Mode.IsRegular() {
		return "file"
	}
	return "other"
}
