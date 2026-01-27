package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCLIRunsConsistently builds the cwalk binary and runs it repeatedly
// to ensure the walk actually starts (guards against startup races).
func TestCLIRunsConsistently(t *testing.T) {
	root := t.TempDir()

	// Create deterministic fixtures
	mustWrite := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("a.txt", "data")
	mustWrite(filepath.Join("sub", "b.txt"), "more")

	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "cwalk_test_bin")

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build cwalk: %v", err)
	}

	const runs = 20
	for i := 0; i < runs; i++ {
		cmd := exec.Command(binaryPath, "--output-format", "json", root)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("run %d failed: %v", i, err)
		}

		var payload struct {
			Summary struct {
				TotalInodes int64 `json:"TotalInodes"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("run %d: unmarshal json: %v", i, err)
		}
		if payload.Summary.TotalInodes == 0 {
			t.Fatalf("run %d: walker returned zero inodes", i)
		}
	}
}
