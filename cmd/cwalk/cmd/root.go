// Package cmd provides the Cobra CLI command structure for cwalk.
//
// This package defines the root command and all CLI flags for the cwalk
// directory walker tool. It handles flag parsing, filter construction,
// statistics collection, and output formatting.
package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/otuschhoff/cwalk/pkg/output"
	"github.com/otuschhoff/cwalk/pkg/stat"
	"github.com/spf13/cobra"
)

var (
	// Output options
	outputFormat string
	outputFile   string
	outputMode   string
	noHeader     bool

	// Filter options
	filterType            string
	filterMtimeOlderStr   string
	filterMtimeYoungerStr string
	filterSizeMin         string
	filterSizeMax         string
	filterNameRegex       string
	filterUsernames       string
	filterUIDs            string
	filterGroupnames      string
	filterGIDs            string
	filterPerms           string
	filterPermsNot        string

	// Worker options
	workers int
)

// rootCmd represents the base command when called without any subcommands.
// It walks directory trees and produces statistics in various formats with
// comprehensive filtering options.
var rootCmd = &cobra.Command{
	Use:   "cwalk [paths...]",
	Short: "Fast directory walking with statistics",
	Long: `cwalk is a fast recursive directory walker that collects file statistics
and outputs them in various formats with flexible filtering options.

Examples:
  cwalk /home/user
  cwalk -o summary /home /var
  cwalk --output-format json --output-file stats.json /opt
  cwalk --type file --size-min 1M /tmp
  cwalk --mtime-older 7d --output-mode per-year /home/user`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWalk,
}

// init sets up all CLI flags for the root command.
// Flags are organized into three groups: output options, filter options, and worker options.
func init() {
	// Output format flags
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "f", "table",
		"Output format: table, json, csv, xlsx")
	rootCmd.Flags().StringVarP(&outputFile, "output-file", "o", "",
		"Write output to file (default: stdout)")
	rootCmd.Flags().StringVarP(&outputMode, "output-mode", "m", "summary",
		"Output mode: summary, per-year, per-uid")
	rootCmd.Flags().BoolVar(&noHeader, "no-header", false,
		"Hide table headers")

	// Filter flags
	rootCmd.Flags().StringVar(&filterType, "type", "",
		"Filter by inode type: file, dir, symlink, other (comma-separated)")
	rootCmd.Flags().StringVar(&filterMtimeOlderStr, "mtime-older", "",
		"Filter files modified older than (e.g., 7d, 2w, 30m, 1y)")
	rootCmd.Flags().StringVar(&filterMtimeYoungerStr, "mtime-younger", "",
		"Filter files modified younger than (e.g., 1d, 24h)")
	rootCmd.Flags().StringVar(&filterSizeMin, "size-min", "",
		"Minimum file size (e.g., 1K, 100M, 1G)")
	rootCmd.Flags().StringVar(&filterSizeMax, "size-max", "",
		"Maximum file size (e.g., 1K, 100M, 1G)")
	rootCmd.Flags().StringVar(&filterNameRegex, "name", "",
		"Filter by filename regex pattern")
	rootCmd.Flags().StringVar(&filterUsernames, "username", "",
		"Filter by username (comma-separated)")
	rootCmd.Flags().StringVar(&filterUIDs, "uid", "",
		"Filter by UID (comma-separated)")
	rootCmd.Flags().StringVar(&filterGroupnames, "groupname", "",
		"Filter by group name (comma-separated)")
	rootCmd.Flags().StringVar(&filterGIDs, "gid", "",
		"Filter by GID (comma-separated)")
	rootCmd.Flags().StringVar(&filterPerms, "perms-has", "",
		"Filter by required permission bits (e.g., u+r,g+x)")
	rootCmd.Flags().StringVar(&filterPermsNot, "perms-not", "",
		"Filter by forbidden permission bits (e.g., o+w)")

	// Worker options
	rootCmd.Flags().IntVar(&workers, "workers", 4,
		"Number of parallel workers")
}

// runWalk executes the directory walk with specified filters and outputs results.
// It parses all CLI flags into filter objects, performs the walk, and formats output.
func runWalk(cmd *cobra.Command, args []string) error {
	// Parse filters
	filters := &stat.Filters{}

	if filterType != "" {
		filters.Types = parseInodeTypes(filterType)
	}

	if filterMtimeOlderStr != "" {
		older, err := parseDuration(filterMtimeOlderStr)
		if err != nil {
			return fmt.Errorf("invalid --mtime-older: %w", err)
		}
		filters.MtimeOlderThan = &older
	}

	if filterMtimeYoungerStr != "" {
		younger, err := parseDuration(filterMtimeYoungerStr)
		if err != nil {
			return fmt.Errorf("invalid --mtime-younger: %w", err)
		}
		filters.MtimeYoungerThan = &younger
	}

	if filterSizeMin != "" {
		sizeMin, err := parseSize(filterSizeMin)
		if err != nil {
			return fmt.Errorf("invalid --size-min: %w", err)
		}
		filters.SizeMin = &sizeMin
	}

	if filterSizeMax != "" {
		sizeMax, err := parseSize(filterSizeMax)
		if err != nil {
			return fmt.Errorf("invalid --size-max: %w", err)
		}
		filters.SizeMax = &sizeMax
	}

	if filterNameRegex != "" {
		re, err := regexp.Compile(filterNameRegex)
		if err != nil {
			return fmt.Errorf("invalid --name regex: %w", err)
		}
		filters.NameRegex = re
	}

	if filterUsernames != "" {
		filters.Usernames = parseStringList(filterUsernames)
	}

	if filterUIDs != "" {
		uids, err := parseUintList(filterUIDs)
		if err != nil {
			return fmt.Errorf("invalid --uid: %w", err)
		}
		filters.UIDs = uids
	}

	if filterGroupnames != "" {
		filters.Groupnames = parseStringList(filterGroupnames)
	}

	if filterGIDs != "" {
		gids, err := parseUintList(filterGIDs)
		if err != nil {
			return fmt.Errorf("invalid --gid: %w", err)
		}
		filters.GIDs = gids
	}

	if filterPerms != "" {
		perms, err := parsePerms(filterPerms)
		if err != nil {
			return fmt.Errorf("invalid --perms-has: %w", err)
		}
		filters.PermsHas = perms
	}

	if filterPermsNot != "" {
		perms, err := parsePerms(filterPermsNot)
		if err != nil {
			return fmt.Errorf("invalid --perms-not: %w", err)
		}
		filters.PermsNot = perms
	}

	// Create walker and collect stats
	walker := stat.NewStatsWalker(args, workers, filters)
	results, err := walker.Walk()
	if err != nil {
		return err
	}

	// Format and output results
	formatter := output.NewFormatter(outputFormat, outputMode, noHeader)
	out := formatter.Format(results)

	// Write output
	if outputFile != "" {
		if err := formatter.WriteToFile(out, outputFile); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Output written to: %s\n", outputFile)
	} else {
		fmt.Print(out)
	}

	return nil
}

// Execute adds all child commands to the root command and executes it.
func Execute() error {
	return rootCmd.Execute()
}

// parseInodeTypes parses a comma-separated list of inode type filters.
// Valid types are: file, dir, symlink, other.
func parseInodeTypes(s string) map[string]bool {
	types := make(map[string]bool)
	for _, t := range strings.Split(s, ",") {
		types[strings.TrimSpace(t)] = true
	}
	return types
}

// parseDuration parses duration strings with various units.
// Supported formats: Nd (days), Nw (weeks), Nm (minutes), Nh (hours), Ns (seconds), Ny (years).
// Examples: "7d", "2w", "30m", "1y"
func parseDuration(s string) (time.Duration, error) {
	// Handle special formats like "7d", "2w", "30m", "1y"
	s = strings.TrimSpace(s)
	multiplier := int64(1)
	unit := ""

	// Extract number and unit
	i := len(s) - 1
	for i >= 0 && !isDigit(s[i]) {
		i--
	}
	if i < 0 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	numPart := s[:i+1]
	unitPart := s[i+1:]

	num, err := strconv.ParseInt(numPart, 10, 64)
	if err != nil {
		return 0, err
	}

	switch unitPart {
	case "d":
		unit = "h"
		multiplier = num * 24
	case "w":
		unit = "h"
		multiplier = num * 24 * 7
	case "m":
		unit = "m"
		multiplier = num
	case "h":
		unit = "h"
		multiplier = num
	case "s":
		unit = "s"
		multiplier = num
	case "y":
		unit = "h"
		multiplier = num * 24 * 365
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unitPart)
	}

	durationStr := fmt.Sprintf("%d%s", multiplier, unit)
	return time.ParseDuration(durationStr)
}

// parseSize parses file size strings with binary unit multipliers.
// Supported units: B, K/KB, M/MB, G/GB, T/TB.
// Examples: "1K", "100M", "1.5G"
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	multiplier := int64(1)

	// Find where digits end
	i := 0
	for i < len(s) && (isDigit(s[i]) || s[i] == '.') {
		i++
	}

	numPart := s[:i]
	unitPart := strings.ToUpper(strings.TrimSpace(s[i:]))

	num, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, err
	}

	switch unitPart {
	case "", "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unitPart)
	}

	return int64(num * float64(multiplier)), nil
}

// parseStringList parses a comma-separated list of strings, trimming whitespace.
func parseStringList(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseUintList parses a comma-separated list of unsigned integers.
// Returns an error if any value cannot be parsed or is out of uint32 range.
func parseUintList(s string) ([]uint32, error) {
	var result []uint32
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		val, err := strconv.ParseUint(item, 10, 32)
		if err != nil {
			return nil, err
		}
		result = append(result, uint32(val))
	}
	return result, nil
}

// parsePerms parses permission strings in the format "who+bits" or "who-bits".
// who: u (user), g (group), o (other), a (all)
// bits: r (read), w (write), x (execute)
// Examples: "u+r", "g+x", "o+w"
func parsePerms(s string) (uint32, error) {
	// Parse permission strings like "u+r", "g+x", "o+w"
	var perms uint32

	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			return 0, fmt.Errorf("invalid permission format: %s", part)
		}

		who := part[0]
		op := part[1]
		what := part[2:]

		var bits uint32
		if strings.Contains(what, "r") {
			bits |= 4
		}
		if strings.Contains(what, "w") {
			bits |= 2
		}
		if strings.Contains(what, "x") {
			bits |= 1
		}

		switch who {
		case 'u':
			perms |= bits << 6
		case 'g':
			perms |= bits << 3
		case 'o':
			perms |= bits
		case 'a':
			perms |= (bits << 6) | (bits << 3) | bits
		default:
			return 0, fmt.Errorf("invalid permission who: %c", who)
		}

		if op != '+' && op != '-' {
			return 0, fmt.Errorf("invalid permission operator: %c", op)
		}
	}

	return perms, nil
}

// isDigit returns true if the byte is a digit (0-9).
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
