// Package cwalk provides fast recursive directory walking with extensible callbacks.
//
// It implements a worker pool architecture for parallel directory tree traversal.
// Users can register callbacks to process files, directories, and file metadata
// as the walker encounters them. Multiple worker goroutines distribute the work
// automatically, with work-stealing support for load balancing.
//
// Basic usage:
//
//	callbacks := cwalk.Callbacks{
//		OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
//			// Process file
//		},
//		OnDirectory: func(relPath string, entry os.DirEntry) {
//			// Process directory
//		},
//	}
//	walker := cwalk.NewWalker(".", 4, callbacks)
//	if err := walker.Run(); err != nil {
//		// Handle error
//	}
//
// All callbacks are optional. Relative paths use forward slashes (/) as separators
// and are relative to the root path passed to NewWalker.
package cwalk

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Callbacks define optional handlers that are invoked during the walk.
// All callbacks are optional (zero value means no callback).
//
// The callbacks are invoked in the following order for a typical file:
//   1. OnLstat (isDir=false)
//   2. OnFileOrSymlink
//
// For a typical directory:
//   1. OnLstat (isDir=true)
//   2. OnReadDir
//   3. OnDirectory
//   4. (recursively process children)
//
// Callbacks may be invoked concurrently from multiple worker goroutines.
// If state is shared across callbacks, appropriate synchronization is required.
type Callbacks struct {
	// OnLstat is called after successfully lstat'ing a path (both src and dst).
	// Called for every path processed.
	OnLstat func(isDir bool, relPath string, fileInfo os.FileInfo, err error)
//
// A Walker manages a pool of worker goroutines that traverse a directory tree
// in parallel. Workers process directories in a depth-first manner and can steal
// work from each other to balance the load. The Walker is not safe for concurrent
// use; Run() should only be called once per Walker instance.

	// OnReadDir is called after successfully reading a directory.
	// Called for each directory with its entries.
	OnReadDir func(relPath string, entries []os.DirEntry, err error)

	// OnFileOrSymlink is called for each non-directory entry.
	OnFileOrSymlink func(relPath string, entry os.DirEntry)

	// OnDirectory is called for each directory entry (before recursing).
	OnDirectory func(relPath string, entry os.DirEntry)
}

// Walker recursively walks a directory tree with callbacks.
type Walker struct {
	rootPath  string
	callbacks Callbacks
	monitorCtx context.Context
	cancel     context.CancelFunc

	// Worker pool management
	numWorkers   int
	workers      []*walkWorker
	workerMu     sync.Mutex
	workQueue    chan *walkBranch
//
// Each worker maintains a local queue of branches to process and can steal work
// from other workers when its queue is empty. Workers are internal to the Walker.
	wg           sync.WaitGroup
	shutdown     int32
}

// walkWorker represents a single worker processing directories.
//
// Each branch holds a reference to its parent and its basename, allowing
// efficient computation of relative paths. The root branch has a nil parent.
type walkBranch struct {
	parent   *walkBranch
	basename string
}

// isRoot reports whether this branch is the root of the traversal.
func (cb *walkBranch) isRoot() bool {
	return cb.parent == nil
}

// relPath returns the relative path of this branch from the root, using forward slashes.
func (cb *walkBranch) relPath() string {
	return strings.Join(cb.relPathElems(), "/")
}

// relPathElems returns the relative path components of this branch.
func (cb *walkBranch) relPathElems() []string {
	if cb.isRoot() {
		return []string{}
	}
	return append(cb.parent.relPathElems(), cb.basename)
}

// absPath returns the absolute path of this branch given a root path.}

func (cb *walkBranch) relPathElems() []string {
	if cb.isRoot() {
		return []string{}
	}
// queueLen returns the current length of this worker's work queue.
// It acquires the lock to safely read the queue length.
func (cw *walkWorker) queueLen() int {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return len(cw.queue)
}

// queuePush adds an item to this worker's work queue.
func (cw *walkWorker) queuePush(item *walkBranch) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.queue = append(cw.queue, item)
}

// queuePop removes and returns the last item from this worker's work queue,
// or nil if the queue is empty.	cw.mu.Lock()
	defer cw.mu.Unlock()
	return len(cw.queue)
}

func (cw *walkWorker) queuePush(item *walkBranch) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
//
// The numWorkers parameter specifies the number of worker goroutines to use.
// If numWorkers is less than or equal to 0, it defaults to 1. The callbacks
// parameter specifies optional handlers to invoke during the walk; all callbacks
// are optional. The returned Walker is ready to use and should be started with Run().
//
// The rootPath is cleaned using filepath.Clean before being stored.
	cw.queue = append(cw.queue, item)
}

func (cw *walkWorker) queuePop() *walkBranch {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	if len(cw.queue) > 0 {
		item := cw.queue[len(cw.queue)-1]
		cw.queue = cw.queue[:len(cw.queue)-1]
		return item
	}
	return nil
} and blocks until all workers have completed.
//
// Run initializes the worker goroutines and begins traversing from the root path.
// It returns an error if the root path cannot be stat'd or read. Errors occurring
// during traversal of subdirectories are logged but do not stop the walk; they
// are also reported via the OnLstat and OnReadDir callbacks if configured.
//
// Run should only be called once per Walker instance

// NewWalker creates a new Walker for the given root path.
func NewWalker(rootPath string, numWorkers int, callbacks Callbacks) *Walker {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Walker{
		rootPath:   filepath.Clean(rootPath),
		callbacks:  callbacks,
		monitorCtx: ctx,
		cancel:     cancel,
		numWorkers: numWorkers,
	}
}

// Run starts the walking process.
func (c *Walker) Run() error {
	// Initialize workers
	c.workerMu.Lock()
	for i := 0; i < c.numWorkers; i++ {
		worker := &walkWorker{
			id:     i, for a single worker.
//
// The worker repeatedly pops items from its queue and processes them.
// When the queue is empty, it attempts to steal work from other workers.
// If no work is available, the worker exits and signals completion via
// the WaitGroup
			walker: c,
		}
		c.workers = append(c.workers, worker)
		c.wg.Add(1)
		go c.startWorker(worker)
	}
	c.workerMu.Unlock()

	// Start with root directory
	root := &walkBranch{}
	c.workers[0].queuePush(root)

	// Wait for all workers to finish
	c.wg.Wait()

//
// The thief worker locks the worker pool and looks for other workers with
// more than one item in their queue. If found, it steals the last item from
// that worker and adds it to its own queue, returning true. If no work is
// available to steal, it returns false.
	return nil
}

// startWorker runs the main worker loop.
func (c *Walker) startWorker(worker *walkWorker) {
	defer c.wg.Done()

	for {
		branch := worker.queuePop()

		if branch != nil {
			if err := worker.processBranch(branch); err != nil {
				log.Printf("ERROR processing '%s': %v\n", branch.relPath(), err)
			}
		} else {
			if !c.stealWork(worker) {
				// No work available, exit
				return
			}
		}
	}
}

//
// It stat's the directory, reads its entries, invokes the appropriate callbacks,
// and queues subdirectories for processing by workers. Files and symlinks are
// processed via callbacks but not queued for further recursion.
//
// Directories named ".snapshot" are automatically skipped. Any errors encountered
// are reported via callbacks and/or logged, but do not stop processing of other
// entries.
// stealWork attempts to steal work from other workers.
func (c *Walker) stealWork(thief *walkWorker) bool {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()

	for _, victim := range c.workers {
		if victim.id == thief.id {
			continue
		}

		qlen := victim.queueLen()
		if qlen > 1 {
			stolenItem := victim.queuePop()
			if stolenItem != nil {
				thief.queuePush(stolenItem)
				return true
			}
		}
	}

	return false
}

// processBranch processes a single directory branch.
func (w *walkWorker) processBranch(branch *walkBranch) error {
	absPath := branch.absPath(w.walker.rootPath)
	relPath := branch.relPath()

	// Call OnLstat for the directory itself
	info, err := os.Lstat(absPath)
	if w.walker.callbacks.OnLstat != nil {
		w.walker.callbacks.OnLstat(true, relPath, info, err)
	}

	if err != nil {
		return fmt.Errorf("lstat failed for '%s': %w", absPath, err)
	}

	// ReadDir the current branch
	entries, err := os.ReadDir(absPath)
	if w.walker.callbacks.OnReadDir != nil {
		w.walker.callbacks.OnReadDir(relPath, entries, err)
	}

	if err != nil {
		return fmt.Errorf("readdir failed for '%s': %w", absPath, err)
	}

	// Process each entry
	for _, entry := range entries {
		entryName := entry.Name()

		// Skip special directories
		if entry.IsDir() && entryName == ".snapshot" {
			continue
		}

		if entry.IsDir() {
			// Call OnDirectory callback
			if w.walker.callbacks.OnDirectory != nil {
				childRelPath := relPath
				if !branch.isRoot() {
					childRelPath = relPath + "/" + entryName
				} else {
					childRelPath = entryName
				}
				w.walker.callbacks.OnDirectory(childRelPath, entry)
			}
//
// Calling Stop() cancels the context used by the Walker, signaling workers to
// exit. Note that Stop() does not wait for workers to actually exit; use sync
// mechanisms if synchronization is needed. Stop() can be safely called multiple times.

			// Queue child branch for processing
			childBranch := &walkBranch{
				parent:   branch,
				basename: entryName,
			}
			w.queuePush(childBranch)
		} else {
			// Call OnFileOrSymlink callback
			if w.walker.callbacks.OnFileOrSymlink != nil {
				childRelPath := relPath
				if !branch.isRoot() {
					childRelPath = relPath + "/" + entryName
				} else {
					childRelPath = entryName
				}
				w.walker.callbacks.OnFileOrSymlink(childRelPath, entry)

				// Call OnLstat for the file/symlink
				entryAbsPath := filepath.Join(absPath, entryName)
				entryInfo, entryErr := os.Lstat(entryAbsPath)
				if w.walker.callbacks.OnLstat != nil {
					w.walker.callbacks.OnLstat(false, childRelPath, entryInfo, entryErr)
				}
			}
		}
	}

	return nil
}

// Stop cancels the walking process.
func (c *Walker) Stop() {
	c.cancel()
}
