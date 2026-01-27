// Package cwalk provides fast recursive directory walking with extensible callbacks.
//
// It implements a worker pool architecture for parallel directory tree traversal.
// Users can register callbacks to process files, directories, and file metadata
// as the walker encounters them. Multiple worker goroutines distribute the work
// automatically, with work-stealing support for load balancing.
//
// Basic usage:
//
//callbacks := cwalk.Callbacks{
//OnFileOrSymlink: func(relPath string, entry os.DirEntry) {
//// Process file
//},
//OnDirectory: func(relPath string, entry os.DirEntry) {
//// Process directory
//},
//}
//walker := cwalk.NewWalker(".", 4, callbacks)
//if err := walker.Run(); err != nil {
//// Handle error
//}
//
// All callbacks are optional. Relative paths use forward slashes (/) as separators
// and are relative to the root path passed to NewWalker.
package cwalk

const Version = "v0.1.0"

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
type Callbacks struct {
// OnLstat is called after successfully lstat'ing a path (both src and dst).
// Called for every path processed.
OnLstat func(isDir bool, relPath string, fileInfo os.FileInfo, err error)

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
wg           sync.WaitGroup
shutdown     int32
}

// walkWorker represents a single worker processing directories.
type walkWorker struct {
id     int
walker *Walker
queue  []*walkBranch
mu     sync.Mutex
}

// walkBranch represents a directory node in the traversal tree.
type walkBranch struct {
parent   *walkBranch
basename string
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
id:     i,
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
