// Package cwalk provides fast recursive directory walking with extensible callbacks.
//
// It implements a worker pool architecture for parallel directory tree traversal.
// Users can register callbacks to process files, directories, and file metadata
// as the walker encounters them. Multiple worker goroutines distribute the work
// automatically, with work-stealing support for load balancing.
//
// Basic usage:
//
// callbacks := cwalk.Callbacks{
// OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
// // Process file
// },
// OnDirectory: func(relPath string, entry os.DirEntry) {
// // Process directory
// },
// }
// walker := cwalk.NewWalker(".", 4, callbacks)
// if err := walker.Run(); err != nil {
// // Handle error
// }
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
	"time"
)

const Version = "v0.1.0"

// Logger defines the interface for logging in the walker.
// If not set, logs will use the standard library log package.
type Logger interface {
	// Printf logs a formatted message similar to log.Printf
	Printf(format string, v ...interface{})
}

// FileSystem supplies non-following metadata and directory entries for a walk.
// Implementations must support concurrent calls when multiple workers are used.
type FileSystem interface {
	Lstat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
}

// DirEntryInfo pairs an entry with metadata returned by the same directory read.
// A nil Info causes the walker to call Lstat for that child.
type DirEntryInfo struct {
	Entry os.DirEntry
	Info  os.FileInfo
}

// ReadDirPlusFS optionally supplies directory entries with non-following metadata.
// An NFSv3 adapter can implement this using READDIRPLUS; if an entry's attributes
// are unavailable, leave Info nil so the walker falls back to Lstat.
type ReadDirPlusFS interface {
	ReadDirPlus(path string) ([]DirEntryInfo, error)
}

// SMBExtendedAttribute preserves the flags and value of a named NTFS EA.
type SMBExtendedAttribute struct {
	Flags uint8
	Value []byte
}

// SMBMetadata contains SMB/NTFS metadata supplied by an SMB client.
// FileID identifies a file within VolumeSerialNumber. SecurityDescriptor
// holds the raw self-relative descriptor (owner, group, DACL and SACL).
// The adapter must report an error rather than silently return partial data.
type SMBMetadata struct {
	FileID             uint64
	VolumeSerialNumber uint64
	SecurityDescriptor []byte
	ExtendedAttributes map[string]SMBExtendedAttribute
}

// SMBMetadataFS is implemented by an SMB client adapter that can retrieve
// the file's 64-bit ID and complete security and EA information.
type SMBMetadataFS interface {
	SMBMetadata(path string) (SMBMetadata, error)
}

type localFileSystem struct{}

func (localFileSystem) Lstat(path string) (os.FileInfo, error)     { return os.Lstat(path) }
func (localFileSystem) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

// Callbacks define optional handlers that are invoked during the walk.
// All callbacks are optional (zero value means no callback).
type Callbacks struct {
	// OnLstat is called after successfully lstat'ing a path (both src and dst).
	// Called for every path processed.
	OnLstat func(isDir bool, relPath string, fileInfo os.FileInfo, err error)

	// OnSMBMetadata is called for each successfully stat'd path when the
	// injected filesystem implements SMBMetadataFS. FileID is the SMB inode
	// equivalent; err reports unavailable ACLs/EAs (including denied SACLs).
	OnSMBMetadata func(relPath string, metadata SMBMetadata, err error)

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
	rootPath   string
	fs         FileSystem
	callbacks  Callbacks
	logger     Logger
	monitorCtx context.Context
	cancel     context.CancelFunc

	ignoreNames map[string]struct{}
	ignoreFunc  func(name, relPath string, info os.FileInfo) bool

	// Worker pool management
	numWorkers int
	workers    []*walkWorker
	workerMu   sync.Mutex
	workerDone chan *walkWorker
	running    bool
	nextWorker int

	// Staggered worker startup control
	rootProcessed  chan struct{} // Closed when root directory is fully processed
	rootClosedOnce sync.Once     // Ensures rootProcessed is closed only once
	runErr         error
	runErrOnce     sync.Once
}

// walkWorker represents a single worker processing directories.
type walkWorker struct {
	id         int
	walker     *Walker
	queue      []*walkBranch
	mu         sync.Mutex
	retire     chan struct{}
	retireOnce sync.Once
	startDelay time.Duration
}

// walkBranch represents a directory node in the traversal tree.
type walkBranch struct {
	parent   *walkBranch
	basename string
	info     os.FileInfo
	metadata *SMBMetadata
	metaErr  error
}

func (cb *walkBranch) isRoot() bool {
	return cb.parent == nil
}

func (cb *walkBranch) relPath() string {
	return strings.Join(cb.relPathElems(), "/")
}

func (cb *walkBranch) relPathElems() []string {
	if cb.isRoot() {
		return []string{}
	}
	return append(cb.parent.relPathElems(), cb.basename)
}

func (cb *walkBranch) absPath(rootPath string) string {
	if cb.isRoot() {
		return rootPath
	}
	return filepath.Join(rootPath, cb.relPath())
}

func (cw *walkWorker) queueLen() int {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return len(cw.queue)
}

func (cw *walkWorker) queuePush(item *walkBranch) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
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
}

func (cw *walkWorker) retiring() bool {
	select {
	case <-cw.retire:
		return true
	default:
		return false
	}
}

func (cw *walkWorker) requestRetirement() {
	cw.retireOnce.Do(func() { close(cw.retire) })
}

// NewWalker creates a new Walker for the given root path.
func NewWalker(rootPath string, numWorkers int, callbacks Callbacks) *Walker {
	return NewWalkerWithFS(rootPath, numWorkers, callbacks, localFileSystem{})
}

// NewWalkerWithFS creates a walker using the supplied filesystem client.
// Pass a non-nil implementation; use NewWalker for the local filesystem.
func NewWalkerWithFS(rootPath string, numWorkers int, callbacks Callbacks, fs FileSystem) *Walker {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Walker{
		rootPath:      filepath.Clean(rootPath),
		fs:            fs,
		callbacks:     callbacks,
		logger:        &stdLogger{},
		monitorCtx:    ctx,
		cancel:        cancel,
		numWorkers:    numWorkers,
		ignoreNames:   map[string]struct{}{},
		rootProcessed: make(chan struct{}),
	}
}

// Run starts the walking process.
func (c *Walker) Run() error {
	if c.fs == nil {
		return fmt.Errorf("filesystem must not be nil")
	}
	if err := c.monitorCtx.Err(); err != nil {
		return err
	}
	// Recreate the channel and sync.Once for this run
	c.rootProcessed = make(chan struct{})
	c.rootClosedOnce = sync.Once{}
	c.runErr = nil
	c.runErrOnce = sync.Once{}

	c.workerMu.Lock()
	if c.running {
		c.workerMu.Unlock()
		return fmt.Errorf("walker is already running")
	}
	c.running = true
	c.workers = nil
	c.workerDone = make(chan *walkWorker)
	c.nextWorker = 0
	for i := 0; i < c.numWorkers; i++ {
		c.addWorkerLocked(time.Duration(i) * 100 * time.Millisecond)
	}

	// Queue the root before workers start so worker 0 cannot exit before work is available.
	c.workers[0].queuePush(&walkBranch{})
	workers := append([]*walkWorker(nil), c.workers...)
	c.workerMu.Unlock()

	for _, worker := range workers {
		go c.startWorker(worker)
	}

	for worker := range c.workerDone {
		c.workerMu.Lock()
		for i, activeWorker := range c.workers {
			if activeWorker == worker {
				c.workers = append(c.workers[:i], c.workers[i+1:]...)
				break
			}
		}
		if len(c.workers) == 0 {
			c.running = false
			close(c.workerDone)
			c.workerMu.Unlock()
			break
		}
		c.workerMu.Unlock()
	}

	if err := c.monitorCtx.Err(); err != nil {
		return err
	}
	return c.runErr
}

func (c *Walker) addWorkerLocked(startDelay time.Duration) *walkWorker {
	worker := &walkWorker{
		id:         c.nextWorker,
		walker:     c,
		retire:     make(chan struct{}),
		startDelay: startDelay,
	}
	c.nextWorker++
	c.workers = append(c.workers, worker)
	return worker
}

// ResizeWorkers changes the worker-pool size. Retiring workers finish queued work first.
func (c *Walker) ResizeWorkers(numWorkers int) error {
	if numWorkers < 1 {
		return fmt.Errorf("worker count must be at least 1")
	}

	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	c.numWorkers = numWorkers
	if !c.running {
		return nil
	}

	current := 0
	for _, worker := range c.workers {
		if !worker.retiring() {
			current++
		}
	}
	if numWorkers > current {
		for i := current; i < numWorkers; i++ {
			worker := c.addWorkerLocked(0)
			go c.startWorker(worker)
		}
		return nil
	}

	toRetire := current - numWorkers
	for i := len(c.workers) - 1; i >= 0 && toRetire > 0; i-- {
		if !c.workers[i].retiring() {
			c.workers[i].requestRetirement()
			toRetire--
		}
	}
	return nil
}

// WorkerCount returns the configured worker-pool size.
func (c *Walker) WorkerCount() int {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	return c.numWorkers
}

// startWorker runs the main worker loop.
func (c *Walker) startWorker(worker *walkWorker) {
	defer func() { c.workerDone <- worker }()

	// Non-zero workers wait for root to be processed before starting work
	if worker.id > 0 {
		select {
		case <-c.rootProcessed:
		case <-c.monitorCtx.Done():
			return
		}
		timer := time.NewTimer(worker.startDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-c.monitorCtx.Done():
			return
		}
	}

	for {
		if c.monitorCtx.Err() != nil {
			return
		}
		branch := worker.queuePop()

		if branch != nil {
			if err := worker.processBranch(branch); err != nil {
				c.runErrOnce.Do(func() {
					c.runErr = err
				})
				c.logger.Printf("ERROR processing '%s': %v", branch.relPath(), err)
			}

			// If worker 0 just finished processing root, signal other workers
			if worker.id == 0 && branch.isRoot() {
				c.rootClosedOnce.Do(func() {
					close(c.rootProcessed)
				})
			}
		} else {
			if worker.retiring() {
				return
			}
			if !c.stealWork(worker) {
				// No work available, exit
				return
			}
		}
	}
}

// stealWork attempts to steal work from other workers.
func (c *Walker) stealWork(thief *walkWorker) bool {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()

	for _, victim := range c.workers {
		if victim.id == thief.id || thief.retiring() {
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
	if err := w.walker.monitorCtx.Err(); err != nil {
		return err
	}
	absPath := branch.absPath(w.walker.rootPath)
	relPath := branch.relPath()

	// Call OnLstat for the directory itself
	info, err := branch.info, error(nil)
	if info == nil {
		info, err = w.walker.fs.Lstat(absPath)
	}
	if w.walker.callbacks.OnLstat != nil {
		w.walker.callbacks.OnLstat(true, relPath, info, err)
	}

	if err != nil {
		return fmt.Errorf("lstat failed for '%s': %w", absPath, err)
	}
	if callback := w.walker.callbacks.OnSMBMetadata; callback != nil {
		if smbFS, ok := w.walker.fs.(SMBMetadataFS); ok {
			metadata, metaErr := branch.metadata, branch.metaErr
			if metadata == nil && metaErr == nil {
				value, fetchErr := smbFS.SMBMetadata(absPath)
				metadata, metaErr = &value, fetchErr
			}
			callback(relPath, *metadata, metaErr)
		}
	}
	if err := w.walker.monitorCtx.Err(); err != nil {
		return err
	}

	// ReadDir the current branch
	var entries []os.DirEntry
	var plusEntries []DirEntryInfo
	if plusFS, ok := w.walker.fs.(ReadDirPlusFS); ok {
		plusEntries, err = plusFS.ReadDirPlus(absPath)
		entries = make([]os.DirEntry, len(plusEntries))
		for index, item := range plusEntries {
			entries[index] = item.Entry
		}
	} else {
		entries, err = w.walker.fs.ReadDir(absPath)
	}
	if w.walker.callbacks.OnReadDir != nil {
		w.walker.callbacks.OnReadDir(relPath, entries, err)
	}

	if err != nil {
		return fmt.Errorf("readdir failed for '%s': %w", absPath, err)
	}

	// Process each entry
	for index, entry := range entries {
		if err := w.walker.monitorCtx.Err(); err != nil {
			return err
		}
		entryName := entry.Name()

		childRelPath := relPath
		if !branch.isRoot() {
			childRelPath = relPath + "/" + entryName
		} else {
			childRelPath = entryName
		}

		childAbsPath := filepath.Join(absPath, entryName)
		var childInfo os.FileInfo
		var childErr error
		if plusEntries != nil {
			childInfo = plusEntries[index].Info
		}
		if childInfo == nil {
			childInfo, childErr = w.walker.fs.Lstat(childAbsPath)
		}
		if w.walker.callbacks.OnLstat != nil {
			w.walker.callbacks.OnLstat(childErr == nil && childInfo.IsDir(), childRelPath, childInfo, childErr)
		}
		if childErr != nil {
			return fmt.Errorf("lstat failed for '%s': %w", childAbsPath, childErr)
		}
		var metadata *SMBMetadata
		var metaErr error
		if callback := w.walker.callbacks.OnSMBMetadata; callback != nil {
			if smbFS, ok := w.walker.fs.(SMBMetadataFS); ok {
				value, fetchErr := smbFS.SMBMetadata(childAbsPath)
				metadata, metaErr = &value, fetchErr
				callback(childRelPath, value, fetchErr)
			}
		}
		if err := w.walker.monitorCtx.Err(); err != nil {
			return err
		}

		if w.walker.shouldIgnore(entryName, childRelPath, childInfo) {
			continue
		}

		if childInfo.IsDir() {
			// Call OnDirectory callback
			if w.walker.callbacks.OnDirectory != nil {
				w.walker.callbacks.OnDirectory(childRelPath, entry)
			}

			// Queue child branch for processing
			childBranch := &walkBranch{
				parent:   branch,
				basename: entryName,
				info:     childInfo,
				metadata: metadata,
				metaErr:  metaErr,
			}
			w.queuePush(childBranch)
		} else {
			// Call OnFileOrSymlink callback
			if w.walker.callbacks.OnFileOrSymlink != nil {
				w.walker.callbacks.OnFileOrSymlink(childRelPath, entry)
			}
		}
	}

	return nil
}

// Stop cancels the walking process.
func (c *Walker) Stop() {
	c.cancel()
}

// SetIgnoreNames sets names (files or directories) to be skipped during the walk.
// Matching is case-sensitive and applies to entry basenames only.
func (c *Walker) SetIgnoreNames(names []string) {
	c.ignoreNames = map[string]struct{}{}
	for _, name := range names {
		c.ignoreNames[name] = struct{}{}
	}
}

// SetIgnoreFunc sets a callback to decide whether to ignore a path.
// The callback receives the entry name, its relative path, and the lstat info.
// If the callback returns true, the entry is skipped.
func (c *Walker) SetIgnoreFunc(fn func(name, relPath string, info os.FileInfo) bool) {
	c.ignoreFunc = fn
}

func (c *Walker) shouldIgnore(name, relPath string, info os.FileInfo) bool {
	if c.ignoreNames != nil {
		if _, ok := c.ignoreNames[name]; ok {
			return true
		}
	}

	if c.ignoreFunc != nil {
		return c.ignoreFunc(name, relPath, info)
	}

	return false
}

// SetLogger sets a custom logger for the walker.
// If not called, the default standard library logger is used.
func (c *Walker) SetLogger(logger Logger) {
	if logger != nil {
		c.logger = logger
	}
}

// stdLogger is the default logger using the standard library log package.
type stdLogger struct{}

func (s *stdLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}
