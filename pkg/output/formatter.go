// Package output provides formatting and export of directory statistics.
//
// It supports multiple output modes (summary, per-year, per-uid) and
// formats (table, JSON, CSV, XLSX), making statistics accessible in
// various ways for different use cases.
package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/otuschhoff/cwalk/pkg/stat"
)

// Formatter handles formatting and exporting statistics in various formats and modes.
//
// Supported formats: "table" (ASCII tables), "json" (JSON), "csv" (CSV), "xlsx" (Excel).
// Supported modes: "summary" (total statistics), "per-year" (grouped by year), "per-uid" (grouped by owner).
type Formatter struct {
	format   string // "table", "json", "csv", "xlsx"
	mode     string // "summary", "per-year", "per-uid"
	noHeader bool   // Omit header row in table output
}

// NewFormatter creates a new Formatter with the specified format and output mode.
func NewFormatter(format, mode string, noHeader bool) *Formatter {
	return &Formatter{
		format:   format,
		mode:     mode,
		noHeader: noHeader,
	}
}

// Format converts results to the appropriate output format as a string.
// The actual formatting depends on the Formatter's format and mode settings.
func (f *Formatter) Format(results *stat.Results) string {
	switch f.mode {
	case "per-year":
		return f.formatPerYear(results)
	case "per-uid":
		return f.formatPerUID(results)
	default:
		return f.formatSummary(results)
	}
}

// WriteToFile writes formatted output to a file, handling format-specific options.
// For XLSX format, content is interpreted as filename base. For other formats,
// content is written as-is to the file.
func (f *Formatter) WriteToFile(content string, filename string) error {
	switch f.format {
	case "xlsx":
		return f.writeXLSX(filename, content)
	default:
		return os.WriteFile(filename, []byte(content), 0644)
	}
}

// formatSummary formats summary statistics in the specified format (table/json/csv).
func (f *Formatter) formatSummary(results *stat.Results) string {
	sum := results.Summary

	data := []map[string]interface{}{
		{
			"Metric":   "Total Size",
			"Value":    formatBytes(sum.TotalSize),
			"Files":    sum.FilesSize,
			"Dirs":     sum.DirsSize,
			"Symlinks": sum.SymlinksSize,
			"Others":   sum.OthersSize,
		},
		{
			"Metric":   "Total Inodes",
			"Value":    sum.TotalInodes,
			"Files":    sum.Files,
			"Dirs":     sum.Dirs,
			"Symlinks": sum.Symlinks,
			"Others":   sum.Others,
		},
	}

	if f.format == "json" {
		return f.toJSON(map[string]interface{}{
			"summary": sum,
			"totals": map[string]interface{}{
				"totalSize":    sum.TotalSize,
				"totalInodes":  sum.TotalInodes,
				"files":        sum.Files,
				"dirs":         sum.Dirs,
				"symlinks":     sum.Symlinks,
				"others":       sum.Others,
				"filesSize":    sum.FilesSize,
				"dirsSize":     sum.DirsSize,
				"symlinksSize": sum.SymlinksSize,
				"othersSize":   sum.OthersSize,
			},
		})
	}

	if f.format == "csv" {
		return f.toCSV([]string{"Metric", "Value", "Files", "Dirs", "Symlinks", "Others"}, data)
	}

	return f.summaryTable(sum)
}

// formatPerYear formats statistics grouped by year
func (f *Formatter) formatPerYear(results *stat.Results) string {
	// Sort years
	var years []int
	for year := range results.ByYear {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	if f.format == "json" {
		return f.toJSON(results.ByYear)
	}

	data := []map[string]interface{}{}
	for _, year := range years {
		stat := results.ByYear[year]
		data = append(data, map[string]interface{}{
			"Year":      year,
			"Size":      formatBytes(stat.TotalSize),
			"Inodes":    stat.TotalInodes,
			"Files":     stat.Files,
			"Dirs":      stat.Dirs,
			"Symlinks":  stat.Symlinks,
			"Others":    stat.Others,
			"FilesSize": formatBytes(stat.FilesSize),
			"DirsSize":  formatBytes(stat.DirsSize),
		})
	}

	if f.format == "csv" {
		headers := []string{"Year", "Size", "Inodes", "Files", "Dirs", "Symlinks", "Others", "FilesSize", "DirsSize"}
		return f.toCSV(headers, data)
	}

	return f.perYearTable(results.ByYear)
}

// formatPerUID formats statistics grouped by UID (file owner).
// Groups all files by their owner UID and presents statistics for each user.
func (f *Formatter) formatPerUID(results *stat.Results) string {
	// Sort UIDs
	var uids []uint32
	for uid := range results.ByUID {
		uids = append(uids, uid)
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })

	if f.format == "json" {
		// Convert to a more JSON-friendly format
		uidData := make([]map[string]interface{}, 0)
		for _, uid := range uids {
			stat := results.ByUID[uid]
			uidData = append(uidData, map[string]interface{}{
				"uid":       uid,
				"username":  stat.Username,
				"size":      stat.TotalSize,
				"inodes":    stat.TotalInodes,
				"files":     stat.Files,
				"dirs":      stat.Dirs,
				"symlinks":  stat.Symlinks,
				"others":    stat.Others,
				"filesSize": stat.FilesSize,
				"dirsSize":  stat.DirsSize,
			})
		}
		return f.toJSON(uidData)
	}

	data := []map[string]interface{}{}
	for _, uid := range uids {
		stat := results.ByUID[uid]
		data = append(data, map[string]interface{}{
			"UID":       uid,
			"Username":  stat.Username,
			"Size":      formatBytes(stat.TotalSize),
			"Inodes":    stat.TotalInodes,
			"Files":     stat.Files,
			"Dirs":      stat.Dirs,
			"Symlinks":  stat.Symlinks,
			"Others":    stat.Others,
			"FilesSize": formatBytes(stat.FilesSize),
			"DirsSize":  formatBytes(stat.DirsSize),
		})
	}

	if f.format == "csv" {
		headers := []string{"UID", "Username", "Size", "Inodes", "Files", "Dirs", "Symlinks", "Others", "FilesSize", "DirsSize"}
		return f.toCSV(headers, data)
	}

	return f.perUIDTable(results.ByUID)
}

// summaryTable creates a formatted summary table, showing only columns with non-zero values
func (f *Formatter) summaryTable(sum *stat.SummaryStat) string {
	t := table.NewWriter()

	// Determine which columns to show (those with non-zero values)
	var headers []string
	headers = append(headers, "Metric", "Count/Size")
	if sum.Files > 0 {
		headers = append(headers, "Files")
	}
	if sum.Dirs > 0 {
		headers = append(headers, "Dirs")
	}
	if sum.Symlinks > 0 {
		headers = append(headers, "Symlinks")
	}
	if sum.Others > 0 {
		headers = append(headers, "Others")
	}

	if !f.noHeader {
		headerRow := make(table.Row, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		t.AppendHeader(headerRow)
	}

	// Build inodes row
	var inodesRow []interface{}
	inodesRow = append(inodesRow, "Total Inodes", sum.TotalInodes)
	if sum.Files > 0 {
		inodesRow = append(inodesRow, sum.Files)
	}
	if sum.Dirs > 0 {
		inodesRow = append(inodesRow, sum.Dirs)
	}
	if sum.Symlinks > 0 {
		inodesRow = append(inodesRow, sum.Symlinks)
	}
	if sum.Others > 0 {
		inodesRow = append(inodesRow, sum.Others)
	}

	// Build size row
	var sizeRow []interface{}
	sizeRow = append(sizeRow, "Total Size", formatBytes(sum.TotalSize))
	if sum.Files > 0 {
		sizeRow = append(sizeRow, formatBytes(sum.FilesSize))
	}
	if sum.Dirs > 0 {
		sizeRow = append(sizeRow, formatBytes(sum.DirsSize))
	}
	if sum.Symlinks > 0 {
		sizeRow = append(sizeRow, formatBytes(sum.SymlinksSize))
	}
	if sum.Others > 0 {
		sizeRow = append(sizeRow, formatBytes(sum.OthersSize))
	}

	t.AppendRows([]table.Row{
		inodesRow,
		sizeRow,
	})

	t.SetStyle(table.StyleColoredDark)
	return fmt.Sprintf("%s\n", t.Render())
}

// perYearTable creates a formatted per-year table, showing only columns with non-zero values
func (f *Formatter) perYearTable(byYear map[int]*stat.YearStat) string {
	t := table.NewWriter()

	// Sort years descending
	var years []int
	for year := range byYear {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	// Determine which columns to show (those with non-zero values across all years)
	var headers []string
	headers = append(headers, "Year", "Size", "Inodes")

	hasFiles := false
	hasDirs := false
	hasSymlinks := false
	hasOthers := false
	hasFilesSize := false
	hasDirsSize := false

	for _, year := range years {
		s := byYear[year]
		if s.Files > 0 {
			hasFiles = true
		}
		if s.Dirs > 0 {
			hasDirs = true
		}
		if s.Symlinks > 0 {
			hasSymlinks = true
		}
		if s.Others > 0 {
			hasOthers = true
		}
		if s.FilesSize > 0 {
			hasFilesSize = true
		}
		if s.DirsSize > 0 {
			hasDirsSize = true
		}
	}

	if hasFiles {
		headers = append(headers, "Files")
	}
	if hasDirs {
		headers = append(headers, "Dirs")
	}
	if hasSymlinks {
		headers = append(headers, "Symlinks")
	}
	if hasOthers {
		headers = append(headers, "Others")
	}
	if hasFilesSize {
		headers = append(headers, "Files Size")
	}
	if hasDirsSize {
		headers = append(headers, "Dirs Size")
	}

	if !f.noHeader {
		headerRow := make(table.Row, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		t.AppendHeader(headerRow)
	}

	for _, year := range years {
		stat := byYear[year]
		var row []interface{}
		row = append(row, year, formatBytes(stat.TotalSize), stat.TotalInodes)

		if hasFiles {
			row = append(row, stat.Files)
		}
		if hasDirs {
			row = append(row, stat.Dirs)
		}
		if hasSymlinks {
			row = append(row, stat.Symlinks)
		}
		if hasOthers {
			row = append(row, stat.Others)
		}
		if hasFilesSize {
			row = append(row, formatBytes(stat.FilesSize))
		}
		if hasDirsSize {
			row = append(row, formatBytes(stat.DirsSize))
		}

		t.AppendRow(table.Row(row))
	}

	t.SetStyle(table.StyleColoredDark)
	return fmt.Sprintf("%s\n", t.Render())
}

// perUIDTable creates a formatted per-UID table, showing only columns with non-zero values
func (f *Formatter) perUIDTable(byUID map[uint32]*stat.UIDStat) string {
	t := table.NewWriter()

	// Sort UIDs
	var uids []uint32
	for uid := range byUID {
		uids = append(uids, uid)
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })

	// Determine which columns to show (those with non-zero values across all UIDs)
	var headers []string
	headers = append(headers, "UID", "Username", "Size", "Inodes")

	hasFiles := false
	hasDirs := false
	hasSymlinks := false
	hasOthers := false
	hasFilesSize := false
	hasDirsSize := false

	for _, uid := range uids {
		s := byUID[uid]
		if s.Files > 0 {
			hasFiles = true
		}
		if s.Dirs > 0 {
			hasDirs = true
		}
		if s.Symlinks > 0 {
			hasSymlinks = true
		}
		if s.Others > 0 {
			hasOthers = true
		}
		if s.FilesSize > 0 {
			hasFilesSize = true
		}
		if s.DirsSize > 0 {
			hasDirsSize = true
		}
	}

	if hasFiles {
		headers = append(headers, "Files")
	}
	if hasDirs {
		headers = append(headers, "Dirs")
	}
	if hasSymlinks {
		headers = append(headers, "Symlinks")
	}
	if hasOthers {
		headers = append(headers, "Others")
	}
	if hasFilesSize {
		headers = append(headers, "Files Size")
	}
	if hasDirsSize {
		headers = append(headers, "Dirs Size")
	}

	if !f.noHeader {
		headerRow := make(table.Row, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		t.AppendHeader(headerRow)
	}

	for _, uid := range uids {
		stat := byUID[uid]
		var row []interface{}
		row = append(row, uid, stat.Username, formatBytes(stat.TotalSize), stat.TotalInodes)

		if hasFiles {
			row = append(row, stat.Files)
		}
		if hasDirs {
			row = append(row, stat.Dirs)
		}
		if hasSymlinks {
			row = append(row, stat.Symlinks)
		}
		if hasOthers {
			row = append(row, stat.Others)
		}
		if hasFilesSize {
			row = append(row, formatBytes(stat.FilesSize))
		}
		if hasDirsSize {
			row = append(row, formatBytes(stat.DirsSize))
		}

		t.AppendRow(table.Row(row))
	}

	t.SetStyle(table.StyleColoredDark)
	return fmt.Sprintf("%s\n", t.Render())
}

// toJSON converts data to a JSON string using indented formatting.
func (f *Formatter) toJSON(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v\n", err)
	}
	return string(b)
}

// toCSV converts tabular data to CSV format.
// Headers are written first, followed by rows with values in header column order.
func (f *Formatter) toCSV(headers []string, data []map[string]interface{}) string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write headers
	writer.Write(headers)

	// Write data rows
	for _, row := range data {
		var values []string
		for _, header := range headers {
			val := row[header]
			values = append(values, fmt.Sprintf("%v", val))
		}
		writer.Write(values)
	}

	writer.Flush()
	return buf.String()
}

// writeXLSX writes data to an Excel file.
// Current implementation writes JSON to a .json file as placeholder.
// TODO: Enhance to use excelize for proper Excel output.
func (f *Formatter) writeXLSX(filename string, content string) error {
	// For now, just write as JSON
	// You can enhance this to use excelize for proper Excel output
	return os.WriteFile(filename+".json", []byte(content), 0644)
}

// formatBytes formats bytes to a human-readable string with binary unit suffixes.
// Uses standard binary prefixes (K, M, G, T, P, E).
// Examples: "1.5 KB", "2.3 MB", "1.0 GB"
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
