# cwalk

Fast recursive directory walking with extensible callbacks and parallel worker support.

## Objective

`cwalk` is a Go package that provides high-performance directory tree traversal with a worker pool architecture. It enables efficient recursive directory walking with extensible callback hooks for custom processing of files, directories, and file metadata. The package supports parallel processing through multiple worker goroutines, allowing efficient handling of large directory trees.

## Features

- **Parallel Processing**: Walk directory trees using multiple worker goroutines for improved performance
- **Extensible Callbacks**: Hook into the walking process with custom handlers:
  - `OnLstat`: Called after stat'ing each path (files and directories)
  - `OnReadDir`: Called after reading directory contents
  - `OnDirectory`: Called for each directory before recursing
  - `OnFileOrSymlink`: Called for each non-directory entry
- **Work Stealing**: Workers can steal work from other workers to balance the load
- **Context Cancellation**: Graceful cancellation via the `Stop()` method
- **Automatic Worker Tuning**: Invalid worker counts are automatically adjusted

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

Run the test suite:

```bash
go test ./...
```

Run with verbose output:

```bash
go test -v ./...
```

Run benchmarks:

```bash
go test -bench=. ./...
```

## License

MIT License - See [LICENSE](LICENSE) file for details
