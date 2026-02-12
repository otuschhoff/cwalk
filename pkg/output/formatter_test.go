package output

import (
	"strings"
	"testing"

	"github.com/otuschhoff/cwalk/pkg/stat"
	"github.com/stretchr/testify/assert"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		mode     string
		noHeader bool
	}{
		{"default", "table", "summary", false},
		{"json", "json", "per-year", false},
		{"csv with header", "csv", "per-uid", false},
		{"xlsx no header", "xlsx", "summary", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFormatter(tt.format, tt.mode, tt.noHeader)

			assert.Equal(t, tt.format, f.format)
			assert.Equal(t, tt.mode, f.mode)
			assert.Equal(t, tt.noHeader, f.noHeader)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"terabytes", 1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{"zero", 0, "0 B"},
		{"1.5 MB", int64(1.5 * 1024 * 1024), "1.5 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSummary(t *testing.T) {
	results := &stat.Results{
		Summary: &stat.SummaryStat{
			TotalSize:    1048576,
			TotalInodes:  100,
			Files:        80,
			Dirs:         15,
			Symlinks:     5,
			FilesSize:    900000,
			DirsSize:     100000,
			SymlinksSize: 48576,
		},
		ByYear:      make(map[int]*stat.YearStat),
		ByUID:       make(map[uint32]*stat.UIDStat),
		TotalFiles:  make(map[string]int64),
		TotalSize:   make(map[string]int64),
		TotalInodes: make(map[string]int64),
	}

	tests := []struct {
		name   string
		format string
	}{
		{"json format", "json"},
		{"csv format", "csv"},
		{"table format", "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFormatter(tt.format, "summary", false)
			output := f.Format(results)

			assert.NotEmpty(t, output)

			// Check format-specific content
			switch tt.format {
			case "json":
				assert.True(t, strings.Contains(output, "summary") || strings.Contains(output, "Total"))
			case "csv":
				assert.Contains(t, output, ",")
			case "table":
				assert.Contains(t, output, "Total")
			}
		})
	}
}

func TestFormatSummaryConditionalColumns(t *testing.T) {
	// Test table output hides columns with zero values
	results := &stat.Results{
		Summary: &stat.SummaryStat{
			TotalSize:    1048576,
			TotalInodes:  100,
			Files:        80,
			Dirs:         15,
			Symlinks:     0, // Zero value - should be hidden
			Others:       0, // Zero value - should be hidden
			FilesSize:    900000,
			DirsSize:     100000,
			SymlinksSize: 0,
			OthersSize:   0,
		},
		ByYear:      make(map[int]*stat.YearStat),
		ByUID:       make(map[uint32]*stat.UIDStat),
		TotalFiles:  make(map[string]int64),
		TotalSize:   make(map[string]int64),
		TotalInodes: make(map[string]int64),
	}

	f := NewFormatter("table", "summary", false)
	output := f.Format(results)

	assert.NotEmpty(t, output)

	// Symlinks and Others should not appear in table when zero
	assert.NotContains(t, output, "Symlink", "Table output should NOT show Symlinks column when value is 0")
	assert.NotContains(t, output, "Other", "Table output should NOT show Others column when value is 0")
}

func TestFormatJSON(t *testing.T) {
	f := NewFormatter("json", "summary", false)

	data := map[string]interface{}{
		"test":   "value",
		"number": 42,
		"array":  []int{1, 2, 3},
	}

	output := f.toJSON(data)

	assert.Contains(t, output, "test")
	assert.Contains(t, output, "value")
	assert.True(t, strings.Contains(output, "{") && strings.Contains(output, "}"), "JSON output should be properly formatted")
}

func TestFormatCSV(t *testing.T) {
	f := NewFormatter("csv", "summary", false)

	headers := []string{"Name", "Size", "Count"}
	data := []map[string]interface{}{
		{
			"Name":  "file1",
			"Size":  "1KB",
			"Count": "10",
		},
		{
			"Name":  "file2",
			"Size":  "2KB",
			"Count": "20",
		},
	}

	output := f.toCSV(headers, data)

	assert.True(t, strings.Contains(output, "Name") || strings.Contains(output, "Size"))
	assert.True(t, strings.Contains(output, "file1") || strings.Contains(output, "file2"))

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.GreaterOrEqual(t, len(lines), 2)
}

func TestFormatterFields(t *testing.T) {
	f := NewFormatter("json", "per-year", true)

	assert.Equal(t, "json", f.format)
	assert.Equal(t, "per-year", f.mode)
	assert.True(t, f.noHeader)
}

func TestFormatAlignedColumnThreshold(t *testing.T) {
	tests := []struct {
		name      string
		values    []int64
		isBytes   bool
		shouldHas bool   // Whether output should contain "<"
		checkDim  bool   // Whether to check for dimming ANSI code
	}{
		{
			name:      "bytes below threshold",
			values:    []int64{1024 * 1024, 100}, // 1MB, 100B - 100B is 0.00 MB
			isBytes:   true,
			shouldHas: true,
			checkDim:  true,
		},
		{
			name:      "all byte values above threshold",
			values:    []int64{1024 * 1024, 1024 * 1024 / 2}, // 1MB, 0.5MB
			isBytes:   true,
			shouldHas: false,
			checkDim:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAlignedColumn(tt.values, tt.isBytes)
			
			hasLess := false
			hasDimming := false
			for _, v := range result {
				if strings.Contains(v, "<") {
					hasLess = true
					if strings.Contains(v, "\x1b[90m") {
						hasDimming = true
					}
				}
			}
			
			if hasLess != tt.shouldHas {
				assert.Equal(t, tt.shouldHas, hasLess, "formatAlignedColumn(%v, %v) has '<'=%v, want %v. Output: %v", 
					tt.values, tt.isBytes, hasLess, tt.shouldHas, result)
			}
			
			if tt.checkDim && tt.shouldHas {
				assert.True(t, hasDimming, "formatAlignedColumn(%v, %v) has '<' but not dimmed. Output: %v",
					tt.values, tt.isBytes, result)
			}
		})
	}
}
