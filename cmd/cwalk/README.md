# cwalk CLI - Command-Line Interface

The cwalk CLI tool provides a comprehensive interface for analyzing directory statistics with advanced filtering, multiple output modes, and flexible export formats.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Building](#building)
3. [Usage](#usage)
4. [Output Modes](#output-modes)
5. [Output Formats](#output-formats)
6. [Filtering](#filtering)
7. [Options Reference](#options-reference)
8. [Examples](#examples)
9. [Performance Tips](#performance-tips)
10. [Architecture](#architecture)

## Quick Start

### Building

```bash
go build -o cwalk ./cmd/cwalk/main.go
```

### Basic Usage

```bash
# Analyze single directory
./cwalk /home/user

# Analyze multiple directories
./cwalk /home /var /opt

# Show help
./cwalk --help
```

## Building

The CLI tool is built using the Cobra framework and can be compiled with standard Go tooling:

```bash
go build -o cwalk ./cmd/cwalk/main.go
```

This produces a single executable binary that can be run without additional dependencies.

## Usage

```bash
cwalk [paths...] [flags]
```

### Basic Examples

```bash
# Analyze current directory
./cwalk .

# Analyze home directory
./cwalk /home/user

# Analyze multiple paths
./cwalk /home /var /opt
```

## Output Modes

The CLI supports three different output modes for aggregating statistics:

### Summary Mode (Default)

Shows total statistics broken down by file type.

```bash
./cwalk /home
```

Output:
```
 METRIC        COUNT/SIZE  FILES   DIRS      SYMLINKS  OTHERS 
 Total Inodes  267         106     161       0         0      
 Total Size    7.0 MB      6.4 MB  644.0 KB  0 B       0 B
```

### Per-Year Mode

Groups statistics by file modification year. Useful for identifying old data.

```bash
./cwalk --output-mode per-year /home
```

Output:
```
 YEAR  SIZE    INODES  FILES  DIRS  SYMLINKS  OTHERS  FILES SIZE  DIRS SIZE 
 2026  7.0 MB     267    106   161         0       0  6.4 MB      644.0 KB
 2025  1.2 MB      45     20    25         0       0  0.8 MB      400.0 KB
```

### Per-UID Mode

Groups statistics by file owner with username lookup. Useful for quota management.

```bash
./cwalk --output-mode per-uid /home
```

Output:
```
  UID  USERNAME  SIZE    INODES  FILES  DIRS  SYMLINKS  OTHERS  FILES SIZE  
 1000  quark     7.0 MB     267    106   161         0       0  6.4 MB      
 0     root      512.0 KB    25     15    10         0       0  300.0 KB
```

## Output Formats

The CLI supports multiple output formats for different use cases:

### Table Format (Default)

Human-readable colored ASCII tables using go-pretty.

```bash
./cwalk /home
```

### JSON Format

Machine-readable structured output for programmatic processing.

```bash
./cwalk -f json /home
```

### CSV Format

Spreadsheet-compatible comma-separated values.

```bash
./cwalk -f csv /home
```

### XLSX Format

Direct Excel export (framework in place for future enhancement).

```bash
./cwalk -f xlsx /home
```

### Save to File

Save any format to a file instead of stdout.

```bash
./cwalk -o report.json -f json /home
./cwalk -o stats.csv -f csv --output-mode per-year /home
```

## Filtering

The CLI provides comprehensive filtering capabilities to narrow down analysis:

### By File Type

```bash
./cwalk --type file /home        # Only files
./cwalk --type dir /home         # Only directories
./cwalk --type symlink /home     # Only symlinks
./cwalk --type file,dir /home    # Files and directories
```

### By Size

```bash
./cwalk --size-min 100M /home    # Files ≥ 100 MB
./cwalk --size-max 1G /home      # Files ≤ 1 GB
./cwalk --size-min 1M --size-max 100M /home
```

Supported units: B (bytes), K (kilobytes), M (megabytes), G (gigabytes), T (terabytes)

### By Age (Modification Time)

```bash
./cwalk --mtime-older 30d /home  # Modified > 30 days ago
./cwalk --mtime-younger 7d /home # Modified < 7 days ago
./cwalk --mtime-older 1y /home   # Modified > 1 year ago
```

Supported units: d (days), w (weeks), m (months), h (hours), s (seconds), y (years)

### By Name (Regex)

```bash
./cwalk --name ".*\.log$" /var/log              # Log files
./cwalk --name "^\..*" /home                     # Hidden files
./cwalk --name ".*\.(jpg|png|gif)$" /media      # Image files
```

### By Owner (UID)

```bash
./cwalk --uid 1000 /home                    # UID 1000
./cwalk --username quark /home              # Username 'quark'
./cwalk --uid 1000,1001,1002 /home         # Multiple UIDs
```

### By Group (GID)

```bash
./cwalk --gid 1000 /home                    # GID 1000
./cwalk --groupname wheel /home             # Group name 'wheel'
./cwalk --gid 1000,1001 /home              # Multiple GIDs
```

### By Permissions

```bash
./cwalk --perms-has o+r /home        # World-readable
./cwalk --perms-not o+w /home        # NOT world-writable
./cwalk --perms-has u+x /home        # Owner executable
```

Permission format: `[u|g|o|a][+|-][r|w|x]` (e.g., u+r, o+w, a+x)

## Options Reference

### Output Options

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--output-format` | `-f` | string | table | Format: table, json, csv, xlsx |
| `--output-file` | `-o` | string | | Write to file instead of stdout |
| `--output-mode` | `-m` | string | summary | Mode: summary, per-year, per-uid |
| `--no-header` | | bool | false | Hide table headers |

### Filter Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | | Inode type: file, dir, symlink, other (comma-separated) |
| `--size-min` | string | | Minimum file size (K, M, G, T units) |
| `--size-max` | string | | Maximum file size |
| `--mtime-older` | string | | Files older than (d, w, m, h, s, y units) |
| `--mtime-younger` | string | | Files younger than |
| `--name` | string | | Filename regex pattern |
| `--uid` | string | | UID filter (comma-separated) |
| `--username` | string | | Username filter (comma-separated) |
| `--gid` | string | | GID filter (comma-separated) |
| `--groupname` | string | | Group name filter (comma-separated) |
| `--perms-has` | string | | Required permission bits (e.g., u+r,g+x) |
| `--perms-not` | string | | Forbidden permission bits (e.g., o+w) |

### Other Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workers` | int | 4 | Number of parallel workers |

## Examples

### Disk Space Analysis

```bash
./cwalk --output-mode per-uid -o space_usage.csv -f csv /home
```

Exports user space usage to CSV for import into spreadsheets.

### Finding Large Files

```bash
./cwalk --type file --size-min 100M /
```

Lists all files larger than 100 MB on the system.

### Backup Planning

```bash
./cwalk --output-mode per-year /data
```

Shows data distribution across years for backup planning.

### Cleanup Candidates

```bash
./cwalk --type file --size-min 10M --mtime-older 1y /home
```

Finds files larger than 10 MB modified more than 1 year ago.

### Security Audit

```bash
./cwalk --perms-has o+w /etc
./cwalk --perms-has o+x -f csv --output-mode per-uid /usr/bin
```

Identifies world-writable or world-executable files.

### Quota Management

```bash
./cwalk --output-mode per-uid -o quotas.json -f json /home
```

Generates JSON report of user space usage for quota management.

### Go Source Code Statistics

```bash
./cwalk --name ".*\.go$" -f json -o stats.json ./
```

Analyzes Go source code distribution.

### Recent Large Files

```bash
./cwalk --type file --size-min 10M --mtime-younger 30d /home
```

Finds recently created/modified large files.

### Large Old Files

```bash
./cwalk --type file --size-min 100M --mtime-older 90d /var
```

Finds large files not accessed in 90 days.

## Performance Tips

### 1. Use Specific Filters

Reduce data processed by applying filters early:

```bash
./cwalk --type file --size-min 1M /huge/directory
```

### 2. Increase Workers for Slow Storage

For network filesystems or slow disk, increase parallel workers:

```bash
./cwalk --workers 8 /network/share
```

### 3. Export to JSON for Post-Processing

Use JSON format for further analysis with tools like `jq`:

```bash
./cwalk -f json /home | jq '.summary.TotalSize'
```

### 4. Use Per-Year Mode to Identify Old Data

Quickly identify data by age:

```bash
./cwalk --output-mode per-year --mtime-older 1y /archive
```

## Architecture

### Package Structure

```
cwalk/
├── cmd/cwalk/
│   ├── main.go              # Entry point
│   ├── cmd/
│   │   └── root.go          # Root command with flags
│   └── README.md            # This file
├── pkg/
│   ├── stat/
│   │   ├── walker.go        # Statistics collection
│   │   ├── filters.go       # Filtering logic
│   │   └── *_test.go        # Unit tests
│   └── output/
│       ├── formatter.go     # Output formatting
│       └── *_test.go        # Unit tests
├── go.mod                   # Dependencies
└── ...
```

### Core Components

#### `cmd/cwalk/cmd/root.go`

Implements the root Cobra command with:
- Flag parsing for all options
- Command execution
- Error handling
- Integration with statistics engine and formatter

#### `pkg/stat/walker.go`

Collects statistics by:
- Walking directories using cwalk
- Applying filters to entries
- Aggregating statistics by type, year, and UID
- Thread-safe concurrent aggregation

#### `pkg/stat/filters.go`

Implements filtering logic for:
- Inode type (file, dir, symlink, other)
- Size ranges
- Time ranges
- Name patterns (regex)
- UID/GID
- Permissions

#### `pkg/output/formatter.go`

Formats output in:
- Table format (go-pretty)
- JSON (encoding/json)
- CSV (encoding/csv)
- XLSX (framework for future enhancement)

### Design Decisions

#### Thread Safety

Mutex-protected concurrent map updates ensure thread safety when multiple workers process entries simultaneously.

#### Performance

- Leverages cwalk's work-stealing architecture for scalability
- Efficient filtering applied before aggregation
- Minimal memory overhead with streaming processing

#### User Experience

- Human-readable byte formatting (B, KB, MB, GB, TB)
- Natural duration parsing (7d, 2w, 30m, 1y)
- Colored table output for readability
- Comprehensive help text with examples
- Stderr logging of operations

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/jedib0t/go-pretty/v6` - Table formatting
- `github.com/xuri/excelize/v2` - Excel export (optional)

## Testing

The implementation includes comprehensive tests:

```bash
go test ./...
```

Tests cover:
- Statistics calculation accuracy
- Filter application correctness
- Output format generation
- Concurrent safety
- Edge cases and error handling

## Contributing

To extend the CLI:

1. **Add new filters**: Edit `pkg/stat/filters.go`
2. **Add output formats**: Edit `pkg/output/formatter.go`
3. **Add aggregation modes**: Edit `pkg/stat/walker.go`
4. **Add tests**: Include `*_test.go` files alongside implementation

## License

See [LICENSE](../../LICENSE) in the project root.
