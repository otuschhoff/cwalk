package stat

import (
	"os"
	"regexp"
	"testing"
	"time"
)

func TestFiltersMatches(t *testing.T) {
	now := time.Now()
	oneWeekAgo := 7 * 24 * time.Hour
	oneHourAgo := time.Hour

	tests := []struct {
		name    string
		filters *Filters
		fi      *FileInfo
		want    bool
	}{
		{
			name:    "no filters - all match",
			filters: &Filters{},
			fi:      &FileInfo{Path: "/test/file", Size: 1024, IsDir: false},
			want:    true,
		},
		{
			name: "type filter - file match",
			filters: &Filters{
				Types: map[string]bool{"file": true},
			},
			fi:   &FileInfo{Path: "/test/file", Size: 1024, IsDir: false, IsSymlink: false},
			want: true,
		},
		{
			name: "type filter - dir no match",
			filters: &Filters{
				Types: map[string]bool{"file": true},
			},
			fi:   &FileInfo{Path: "/test/dir", Size: 4096, IsDir: true},
			want: false,
		},
		{
			name: "size min filter - match",
			filters: &Filters{
				SizeMin: &[]int64{1000}[0],
			},
			fi:   &FileInfo{Path: "/test/file", Size: 2048},
			want: true,
		},
		{
			name: "size min filter - no match",
			filters: &Filters{
				SizeMin: &[]int64{5000}[0],
			},
			fi:   &FileInfo{Path: "/test/file", Size: 2048},
			want: false,
		},
		{
			name: "size max filter - match",
			filters: &Filters{
				SizeMax: &[]int64{5000}[0],
			},
			fi:   &FileInfo{Path: "/test/file", Size: 2048},
			want: true,
		},
		{
			name: "size max filter - no match",
			filters: &Filters{
				SizeMax: &[]int64{1000}[0],
			},
			fi:   &FileInfo{Path: "/test/file", Size: 2048},
			want: false,
		},
		{
			name: "mtime older than - match",
			filters: &Filters{
				MtimeOlderThan: &oneWeekAgo,
			},
			fi: &FileInfo{
				Path:    "/test/file",
				ModTime: now.Add(-8 * 24 * time.Hour),
			},
			want: true,
		},
		{
			name: "mtime older than - no match",
			filters: &Filters{
				MtimeOlderThan: &oneWeekAgo,
			},
			fi: &FileInfo{
				Path:    "/test/file",
				ModTime: now.Add(-1 * time.Hour),
			},
			want: false,
		},
		{
			name: "mtime younger than - match",
			filters: &Filters{
				MtimeYoungerThan: &oneHourAgo,
			},
			fi: &FileInfo{
				Path:    "/test/file",
				ModTime: now.Add(-30 * time.Minute),
			},
			want: true,
		},
		{
			name: "mtime younger than - no match",
			filters: &Filters{
				MtimeYoungerThan: &oneHourAgo,
			},
			fi: &FileInfo{
				Path:    "/test/file",
				ModTime: now.Add(-2 * time.Hour),
			},
			want: false,
		},
		{
			name: "name regex - match",
			filters: &Filters{
				NameRegex: regexp.MustCompile(`\.txt$`),
			},
			fi:   &FileInfo{Path: "/test/file.txt"},
			want: true,
		},
		{
			name: "name regex - no match",
			filters: &Filters{
				NameRegex: regexp.MustCompile(`\.txt$`),
			},
			fi:   &FileInfo{Path: "/test/file.log"},
			want: false,
		},
		{
			name: "uid filter - match",
			filters: &Filters{
				UIDs: []uint32{1000, 1001},
			},
			fi:   &FileInfo{Path: "/test/file", UID: 1000},
			want: true,
		},
		{
			name: "uid filter - no match",
			filters: &Filters{
				UIDs: []uint32{1000, 1001},
			},
			fi:   &FileInfo{Path: "/test/file", UID: 2000},
			want: false,
		},
		{
			name: "gid filter - match",
			filters: &Filters{
				GIDs: []uint32{1000, 1001},
			},
			fi:   &FileInfo{Path: "/test/file", GID: 1000},
			want: true,
		},
		{
			name: "gid filter - no match",
			filters: &Filters{
				GIDs: []uint32{1000, 1001},
			},
			fi:   &FileInfo{Path: "/test/file", GID: 2000},
			want: false,
		},
		{
			name: "combined filters - all match",
			filters: &Filters{
				Types:   map[string]bool{"file": true},
				SizeMin: &[]int64{1000}[0],
			},
			fi: &FileInfo{
				Path:      "/test/file",
				Size:      2048,
				IsDir:     false,
				IsSymlink: false,
			},
			want: true,
		},
		{
			name: "combined filters - one fails",
			filters: &Filters{
				Types:   map[string]bool{"file": true},
				SizeMin: &[]int64{5000}[0],
			},
			fi: &FileInfo{
				Path:      "/test/file",
				Size:      2048,
				IsDir:     false,
				IsSymlink: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.Matches(tt.fi)
			if result != tt.want {
				t.Errorf("match mismatch: got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGetFileType(t *testing.T) {
	tests := []struct {
		name     string
		fi       *FileInfo
		expected string
	}{
		{
			name:     "directory",
			fi:       &FileInfo{IsDir: true, IsSymlink: false},
			expected: "dir",
		},
		{
			name:     "symlink",
			fi:       &FileInfo{IsDir: false, IsSymlink: true},
			expected: "symlink",
		},
		{
			name:     "regular file",
			fi:       &FileInfo{IsDir: false, IsSymlink: false},
			expected: "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileType(tt.fi)
			if result != tt.expected {
				t.Errorf("type mismatch: got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestUIDFilter(t *testing.T) {
	tests := []struct {
		name    string
		filters *Filters
		fi      *FileInfo
		want    bool
	}{
		{
			name: "empty uid list",
			filters: &Filters{
				UIDs: []uint32{},
			},
			fi:   &FileInfo{UID: 1000},
			want: true,
		},
		{
			name: "single uid match",
			filters: &Filters{
				UIDs: []uint32{1000},
			},
			fi:   &FileInfo{UID: 1000},
			want: true,
		},
		{
			name: "multiple uids one matches",
			filters: &Filters{
				UIDs: []uint32{500, 1000, 2000},
			},
			fi:   &FileInfo{UID: 1000},
			want: true,
		},
		{
			name: "uid list no match",
			filters: &Filters{
				UIDs: []uint32{500, 1000},
			},
			fi:   &FileInfo{UID: 2000},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.Matches(tt.fi)
			if result != tt.want {
				t.Errorf("uid filter mismatch: got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestPermissionFilter(t *testing.T) {
	// Create a FileInfo with specific permissions (0755)
	// Owner: rwx (7), Group: r-x (5), Other: r-x (5)
	mode := os.FileMode(0o755)

	tests := []struct {
		name    string
		filters *Filters
		fi      *FileInfo
		want    bool
	}{
		{
			name: "perms has - user readable",
			filters: &Filters{
				PermsHas: 0o400, // User read
			},
			fi:   &FileInfo{Mode: mode},
			want: true,
		},
		{
			name: "perms has - user writable",
			filters: &Filters{
				PermsHas: 0o200, // User write
			},
			fi:   &FileInfo{Mode: mode},
			want: true,
		},
		{
			name: "perms not - other writable",
			filters: &Filters{
				PermsNot: 0o002, // Other write
			},
			fi:   &FileInfo{Mode: mode},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.Matches(tt.fi)
			if result != tt.want {
				t.Errorf("permission filter mismatch: got %v, want %v", result, tt.want)
			}
		})
	}
}
