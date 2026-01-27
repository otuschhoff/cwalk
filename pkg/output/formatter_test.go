package output

import (
	"strings"
	"testing"

	"github.com/otuschhoff/cwalk/pkg/stat"
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

			if f.format != tt.format {
				t.Errorf("format mismatch: got %s, want %s", f.format, tt.format)
			}
			if f.mode != tt.mode {
				t.Errorf("mode mismatch: got %s, want %s", f.mode, tt.mode)
			}
			if f.noHeader != tt.noHeader {
				t.Errorf("noHeader mismatch: got %v, want %v", f.noHeader, tt.noHeader)
			}
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
			if result != tt.expected {
				t.Errorf("format mismatch: got %s, want %s", result, tt.expected)
			}
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

			if output == "" {
				t.Error("output should not be empty")
			}

			// Check format-specific content
			switch tt.format {
			case "json":
				if !strings.Contains(output, "summary") && !strings.Contains(output, "Total") {
					t.Error("JSON output should contain summary data")
				}
			case "csv":
				if !strings.Contains(output, ",") {
					t.Error("CSV output should contain comma separators")
				}
			case "table":
				if !strings.Contains(output, "Total") {
					t.Error("Table output should contain metric names")
				}
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
			Symlinks:     0,      // Zero value - should be hidden
			Others:       0,      // Zero value - should be hidden
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

	if output == "" {
		t.Error("output should not be empty")
	}

	// Symlinks and Others should not appear in table when zero
	if strings.Contains(output, "Symlink") {
		t.Error("Table output should NOT show Symlinks column when value is 0")
	}
	if strings.Contains(output, "Other") {
		t.Error("Table output should NOT show Others column when value is 0")
	}
}

func TestFormatJSON(t *testing.T) {
	f := NewFormatter("json", "summary", false)

	data := map[string]interface{}{
		"test":   "value",
		"number": 42,
		"array":  []int{1, 2, 3},
	}

	output := f.toJSON(data)

	if !strings.Contains(output, "test") {
		t.Error("JSON output should contain the test key")
	}

	if !strings.Contains(output, "value") {
		t.Error("JSON output should contain the value")
	}

	if !strings.Contains(output, "{") && !strings.Contains(output, "}") {
		t.Error("JSON output should be properly formatted")
	}
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

	if !strings.Contains(output, "Name") || !strings.Contains(output, "Size") {
		t.Error("CSV output should contain headers")
	}

	if !strings.Contains(output, "file1") || !strings.Contains(output, "file2") {
		t.Error("CSV output should contain data rows")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Error("CSV output should have at least header and one data row")
	}
}

func TestFormatterFields(t *testing.T) {
	f := NewFormatter("json", "per-year", true)

	if f.format != "json" {
		t.Errorf("format mismatch: got %s, want json", f.format)
	}

	if f.mode != "per-year" {
		t.Errorf("mode mismatch: got %s, want per-year", f.mode)
	}

	if !f.noHeader {
		t.Error("noHeader should be true")
	}
}
