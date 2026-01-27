# cwalk

Fast recursive directory walking with extensible callbacks, parallel worker support, and comprehensive CLI statistics tool.

## Objective

`cwalk` is a Go package that provides high-performance directory tree traversal with a worker pool architecture. It enables efficient recursive directory walking with extensible callback hooks for custom processing of files, directories, and file metadata. The package supports parallel processing through multiple worker goroutines, allowing efficient handling of large directory trees.

Additionally, `cwalk` includes a powerful CLI tool for analyzing directory statistics with advanced filtering, multiple aggregation modes, and flexible output formats.

## Features

### Core Package Features
- **Parallel Processing**: Walk directory trees using multiple worker goroutines for improved performance
- **Extensible Callbacks**: Hook into the walking process with custom handlers:
  - `OnLstat`: Called after stat'ing each path (files and directories)
  - `OnReadDir`: Called after reading directory contents
  - `OnDirectory`: Called for each directory before recursing
  - `OnFileOrSymlink`: Called for each non-directory entry
- **Configurable Ignoring**: Skip specific names or decide dynamically via an ignore callback
- **Work Stealing**: Workers can steal work from other workers to balance the load
- **Context Cancellation**: Graceful cancellation via the `Stop()` method
- **Automatic Worker Tuning**: Invalid worker counts are automatically adjusted

### CLI Tool Features
- **Multiple Statistics Modes**: Summary, per-year, and per-UID aggregation
- **Comprehensive Filtering**: Type, size, time, name, owner, and permission filters
- **Flexible Output Formats**: Table, JSON, CSV, and XLSX export
- **Parallel Processing**: Multi-worker support for large directory trees
- **Thread-Safe Aggregation**: Safe concurrent statistics collection
- **Complete GoDoc Documentation**: Full API documentation available

## Documentation

### Library Documentation
- **GoDoc**: Full API documentation with examples
  ```bash
  go doc ./cmd/cwalk
  go doc ./pkg/stat
  go doc ./pkg/output
  ```

### CLI Documentation
- See [cmd/cwalk/README.md](cmd/cwalk/README.md) for comprehensive CLI usage guide
- See [cmd/cwalk/IMPLEMENTATION.md](cmd/cwalk/IMPLEMENTATION.md) for implementation details

### Project Documentation
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contributing guidelines
- [LICENSE](LICENSE) - MIT License

## Installation

```bash
go get github.com/otuschhoff/cwalk
```

## Quick Start

```go
package main

import (
	"fmt"
	"os"
	"github.com/otuschhoff/cwalk"
)

func main() {
	callbacks := cwalk.Callbacks{
		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
			fmt.Printf("File: %s\n", relPath)
		},
		OnDirectory: func(relPath string, entry os.DirEntry) {
			fmt.Printf("Dir: %s\n", relPath)
		},
	}

	walker := cwalk.NewWalker("./mydir", 4, callbacks)
	if err := walker.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Walk error: %v\n", err)
		os.Exit(1)
	}
}
```

## API Reference

### Types

#### `Callbacks`

Defines optional handlers invoked during the walk. All callbacks are optional.

```go
type Callbacks struct {
	// OnLstat is called after successfully lstat'ing a path.
	// Called for every path processed (directories and files).
	OnLstat func(isDir bool, relPath string, fileInfo os.FileInfo, err error)

	// OnReadDir is called after successfully reading a directory.
	OnReadDir func(relPath string, entries []os.DirEntry, err error)

	// OnFileOrSymlink is called for each non-directory entry.
	OnFileOrSymlink func(relPath string, entry os.DirEntry)

	// OnDirectory is called for each directory entry (before recursing).
	OnDirectory func(relPath string, entry os.DirEntry)
}
```

#### `Walker`

The main type that controls directory traversal.

```go
type Walker struct {
	// Contains unexported fields
}
```

### Functions

#### `NewWalker`

Creates a new Walker for the given root path.

```go
func NewWalker(rootPath string, numWorkers int, callbacks Callbacks) *Walker
```

**Parameters:**
- `rootPath`: The root directory to start walking from
- `numWorkers`: Number of worker goroutines (values ≤ 0 default to 1)
- `callbacks`: Callback handlers for walk events

**Returns:** A new Walker instance

#### `Run`

Starts the walking process and blocks until completion.

```go
func (c *Walker) Run() error
```

**Returns:** An error if the root path cannot be stat'd or read

#### `Stop`

Cancels the walking process.

```go
func (c *Walker) Stop()
```

#### `SetLogger`

Sets a custom logger for the walker. If not called, the default standard library logger is used.

```go
func (c *Walker) SetLogger(logger Logger)
```

**Parameters:**
- `logger`: A Logger implementation (nil is ignored and uses the default)

#### `SetIgnoreNames`

Configures basenames (files or directories) to skip during traversal.

```go
func (c *Walker) SetIgnoreNames(names []string)
```

#### `SetIgnoreFunc`

Sets a callback that decides whether to skip a path. The callback receives the entry name, its relative path, and the lstat info.

```go
func (c *Walker) SetIgnoreFunc(fn func(name, relPath string, info os.FileInfo) bool)
```

#### `Logger` (Interface)

Defines the interface for custom logging.

```go
type Logger interface {
	// Printf logs a formatted message similar to log.Printf
	Printf(format string, v ...interface{})
}
```

## Usage Examples

### Basic File Counting

Count all files in a directory tree:

```go
var fileCount int

walker := cwalk.NewWalker(".", 1, cwalk.Callbacks{
	OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
		fileCount++
	},
})

walker.Run()
fmt.Printf("Total files: %d\n", fileCount)
```

### Collecting File Paths

Collect all file paths matching a pattern:

```go
var filePaths []string

walker := cwalk.NewWalker(".", 1, cwalk.Callbacks{
	OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
		if strings.HasSuffix(relPath, ".go") {
			filePaths = append(filePaths, relPath)
		}
	},
})

walker.Run()
```

### Processing Files in Parallel

Use multiple workers for faster processing of large trees:

```go
var mu sync.Mutex
var totalSize int64

walker := cwalk.NewWalker(".", 8, cwalk.Callbacks{
	OnLstat: func(isDir bool, relPath string, fileInfo os.FileInfo, err error) {
		if err != nil {
			return
		}
		if !isDir && fileInfo.Mode().IsRegular() {
			mu.Lock()
			totalSize += fileInfo.Size()
			mu.Unlock()
		}
	},
})

walker.Run()
fmt.Printf("Total size: %d bytes\n", totalSize)
```

### Handling Errors

Monitor errors that occur during walking:

```go
var errors []string

walker := cwalk.NewWalker(".", 1, cwalk.Callbacks{
	OnLstat: func(isDir bool, relPath string, fileInfo os.FileInfo, err error) {
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", relPath, err))
		}
	},
	OnReadDir: func(relPath string, entries []os.DirEntry, err error) {
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", relPath, err))
		}
	},
})

walker.Run()

if len(errors) > 0 {
	fmt.Printf("Encountered %d errors during walk\n", len(errors))
}
```

### Directory Structure Inspection

Print a tree view of the directory structure:

```go
walker := cwalk.NewWalker(".", 1, cwalk.Callbacks{
	OnDirectory: func(relPath string, entry os.DirEntry) {
		depth := strings.Count(relPath, "/")
		indent := strings.Repeat("  ", depth)
		fmt.Printf("%s├── %s/\n", indent, entry.Name())
	},
	OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
		depth := strings.Count(relPath, "/")
		indent := strings.Repeat("  ", depth)
		fmt.Printf("%s├── %s\n", indent, entry.Name())
	},
})

walker.Run()
```

### Custom Logger

Use a custom logger to capture or redirect logging output:

```go
type myLogger struct{}

func (l *myLogger) Printf(format string, v ...interface{}) {
	// Custom logging logic here
	log.Printf("[CUSTOM] " + format, v...)
}

walker := cwalk.NewWalker(".", 4, cwalk.Callbacks{})
walker.SetLogger(&myLogger{})
walker.Run()
```

### Ignoring Entries

Skip specific names and use a custom rule for dynamic ignoring:

```go
walker := cwalk.NewWalker(".", 4, cwalk.Callbacks{})
walker.SetIgnoreNames([]string{".git", ".snapshot"})
walker.SetIgnoreFunc(func(name, relPath string, info os.FileInfo) bool {
	// Skip any path starting with temp-
	return strings.HasPrefix(name, "temp-")
})
walker.Run()
```

## Performance Considerations

- **Worker Count**: Use more workers (4-8) for I/O-bound operations on fast storage. For network filesystems, consider the network throughput limitations.
- **Callback Overhead**: Keep callbacks lightweight. Expensive operations should be deferred or parallelized externally.
- **Memory**: With many workers, consider memory usage if storing large amounts of data per file.
- **Work Stealing**: The walker automatically balances work across workers for better performance on heterogeneous directory trees.

## Special Behavior

- **`.snapshot` Directories**: These directories are automatically skipped and not recursed into.
- **Symlinks**: Symlinks are treated as files and are not followed. Use `OnLstat` to detect symlinks via `fileInfo.Mode()`.
- **Path Separator**: Relative paths always use forward slashes (`/`) as separators, regardless of platform.

## Testing

### Unit Tests
Comprehensive unit tests for all packages:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Test specific package
go test ./cmd/cwalk/cmd
go test ./pkg/stat
go test ./pkg/output
```

### Test Coverage
- **cmd/cwalk/cmd**: 62.1% statement coverage
- **pkg/stat**: 44.4% statement coverage  
- **pkg/output**: 36.9% statement coverage

### Run Benchmarks
```bash
go test -bench=. ./...
```

## License

MIT License - See [LICENSE](LICENSE) file for details

---

## CLI Tool - Directory Statistics Analyzer

The `cwalk` CLI tool provides a powerful command-line interface for analyzing directory statistics with advanced filtering, multiple aggregation modes, and flexible output formats.

### Quick Start

```bash
# Build the CLI
go build -o cwalk ./cmd/cwalk

# Analyze a directory
./cwalk /home/user

# Show help
./cwalk --help
```

### Features

#### Statistics Aggregation
- **Summary Mode**: Total statistics by file type
- **Per-Year Mode**: Breakdown by file modification year
- **Per-UID Mode**: Breakdown by file owner

#### Comprehensive Filtering
- **Type**: Filter by file, directory, symlink, or other
- **Size**: Minimum and maximum file size (K, M, G, T units)
- **Time**: Modified older/younger than (d, w, m, h, s, y durations)
- **Name**: Regex pattern matching on filenames
- **Owner**: Filter by UID, username, GID, or group name
- **Permissions**: Required/forbidden permission bits

#### Output Formats
- **Table**: Human-readable colored ASCII tables
- **JSON**: Machine-readable structured data
- **CSV**: Spreadsheet-compatible format
- **XLSX**: Excel format (infrastructure in place)
- **File Output**: Save results to file

### Building the CLI

```bash
go build -o cwalk ./cmd/cwalk/main.go
```

### Usage

```bash
cwalk [paths...] [flags]
```

### Examples

**Basic usage - summarize a directory:**
```bash
cwalk /home/user
```

**Multiple paths:**
```bash
cwalk /home /var /opt
```

**Output modes:**
```bash
# Per-year breakdown
cwalk --output-mode per-year /home

# Per-UID breakdown  
cwalk --output-mode per-uid /home
```

**Output formats:**
```bash
# JSON output
cwalk -f json --output-mode summary /home

# CSV output
cwalk -f csv --output-mode per-year /home

# Save to file
cwalk -o stats.json -f json /home
```

**Filtering examples:**
```bash
# Only files
cwalk --type file /home

# Only directories
cwalk --type dir /home

# Minimum file size
cwalk --size-min 100M /home

# Maximum file size
cwalk --size-max 1G /home

# Files modified in last 7 days
cwalk --mtime-younger 7d /home

# Files modified more than 1 year ago
cwalk --mtime-older 1y /home

# Regex name matching
cwalk --name ".*\.log$" /home

# Multiple criteria
cwalk --type file --size-min 1M --mtime-older 30d /home
```

**UID/GID filtering:**
```bash
# Specific UID
cwalk --uid 1000 /home

# Specific username
cwalk --username quark /home

# Multiple UIDs
cwalk --uid 1000,1001,1002 /home
```

**Permission filtering:**
```bash
# Files with world-readable bit
cwalk --perms-has o+r /home

# Files without world-writable bit
cwalk --perms-not o+w /home
```

### Flags

**Output Options:**
- `-f, --output-format`: Output format (table, json, csv, xlsx) - default: "table"
- `-o, --output-file`: Write output to file instead of stdout
- `-m, --output-mode`: Output mode (summary, per-year, per-uid) - default: "summary"
- `--no-header`: Hide table headers

**Filter Options:**
- `--type`: Filter by inode type (file, dir, symlink, other) - comma-separated
- `--size-min`: Minimum file size (e.g., 1K, 100M, 1G)
- `--size-max`: Maximum file size
- `--mtime-older`: Files modified older than (e.g., 7d, 2w, 30m, 1y)
- `--mtime-younger`: Files modified younger than (e.g., 1d, 24h)
- `--name`: Filename regex pattern
- `--uid`: UID filter - comma-separated
- `--username`: Username filter - comma-separated
- `--gid`: GID filter - comma-separated
- `--groupname`: Group name filter - comma-separated
- `--perms-has`: Required permission bits (e.g., u+r,g+x)
- `--perms-not`: Forbidden permission bits (e.g., o+w)

**Other Options:**
- `--workers`: Number of parallel workers - default: 4

### Output Modes

**Summary Mode** (default):
Shows aggregated statistics including total size, inode count broken down by file type.

**Per-Year Mode:**
Groups statistics by modification year, useful for analyzing file age distribution.

**Per-UID Mode:**
Groups statistics by file owner (UID/username), useful for quota management.

### Output Formats

**Table Format** (default):
Human-readable colored table using go-pretty.

**JSON Format:**
Machine-readable JSON output with full detail.

**CSV Format:**
Comma-separated values for import into spreadsheets or databases.

**XLSX Format:**
Excel-compatible format for advanced analysis.

## Project Structure

```
cwalk/
├── cmd/cwalk/                # CLI application
│   ├── main.go              # Entry point
│   ├── cmd/
│   │   ├── root.go          # Root command with flags
│   │   └── root_test.go      # Command tests
│   ├── README.md            # CLI documentation
│   └── IMPLEMENTATION.md     # Implementation details
├── pkg/
│   ├── stat/                # Statistics collection
│   │   ├── walker.go        # Statistics walker
│   │   ├── walker_test.go   # Walker tests
│   │   ├── filters.go       # Filtering logic
│   │   └── filters_test.go  # Filter tests
│   └── output/              # Output formatting
│       ├── formatter.go     # Format handler
│       └── formatter_test.go # Formatter tests
├── cwalk.go                 # Core package
├── cwalk_test.go            # Core package tests
├── go.mod                   # Go module definition
├── README.md                # This file
├── LICENSE                  # MIT License
└── CONTRIBUTING.md          # Contribution guidelines
```

## Dependencies

### Core Package
- Standard library only

### CLI Tool
- `github.com/spf13/cobra` - CLI framework
- `github.com/jedib0t/go-pretty/v6` - Table formatting
- Standard library only for core functionality

### Optional
- `github.com/xuri/excelize/v2` - Excel support (infrastructure in place)

## Development

### Code Quality
- **GoDoc Comments**: All public APIs fully documented
- **Unit Tests**: 40+ test cases with coverage tracking
- **No Warnings**: Clean compilation with no warnings
- **Thread Safe**: Mutex-protected concurrent aggregation

### Adding New Features

#### Adding a Filter
1. Add field to `Filters` struct in `pkg/stat/filters.go`
2. Implement matching logic in `Matches()` method
3. Add CLI flag in `cmd/cwalk/cmd/root.go`
4. Parse flag value in root command

#### Adding an Output Format
1. Implement format method in `pkg/output/formatter.go`
2. Add format case in `Format()` method
3. Add flag option in CLI

#### Adding Aggregation Mode
1. Update `Results` struct if needed in `pkg/stat/walker.go`
2. Implement mode-specific formatting in `pkg/output/formatter.go`
3. Add CLI flag option

---

**Version**: 1.0.0  
**Go Version**: 1.24+  
**Status**: Production Ready
