package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupCrawlTestDir creates a temporary directory with sample markdown files
// that reference each other to produce a non-trivial graph.
func setupCrawlTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// api-gateway.md references auth-service and user-db
	writeFile(t, filepath.Join(dir, "api-gateway.md"), `# API Gateway
This service routes requests to the [Auth Service](auth-service.md) and stores data in [User DB](user-db.md).
`)

	// auth-service.md references user-db
	writeFile(t, filepath.Join(dir, "auth-service.md"), `# Auth Service
Handles authentication. Uses [User DB](user-db.md) for credential storage.
`)

	// user-db.md is a leaf node
	writeFile(t, filepath.Join(dir, "user-db.md"), `# User Database
PostgreSQL database for user records.
`)

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCmdCrawl_TextOutput(t *testing.T) {
	dir := setupCrawlTestDir(t)

	// Capture stdout by redirecting CmdCrawl output.
	// CmdCrawl prints to stdout, so we capture via os.Pipe.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CmdCrawl([]string{"--input", dir, "--format", "text"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CmdCrawl returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify expected sections.
	if !strings.Contains(output, "Graph Summary") {
		t.Error("text output missing 'Graph Summary' header")
	}
	if !strings.Contains(output, "Components by Type") {
		t.Error("text output missing 'Components by Type' section")
	}
	if !strings.Contains(output, "Confidence Distribution") {
		t.Error("text output missing 'Confidence Distribution' section")
	}
	if !strings.Contains(output, "Quality") {
		t.Error("text output missing quality information")
	}
	// Should mention component count > 0
	if strings.Contains(output, "0 components") {
		t.Error("expected non-zero component count in text output")
	}
}

func TestCmdCrawl_JSONOutput(t *testing.T) {
	dir := setupCrawlTestDir(t)

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CmdCrawl([]string{"--input", dir, "--format", "json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CmdCrawl returned error: %v", err)
	}

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify valid JSON.
	var result crawlStatsJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON output is not valid: %v\nOutput:\n%s", err, output)
	}

	// Check summary fields.
	if result.Summary.ComponentCount == 0 {
		t.Error("expected non-zero component_count in JSON summary")
	}
	if result.Summary.InputPath == "" {
		t.Error("expected non-empty input_path in JSON summary")
	}

	// Check confidence tiers have range arrays.
	for _, tier := range result.Confidence.Tiers {
		if tier.Range[0] == 0 && tier.Range[1] == 0 {
			t.Errorf("tier %s has zero range", tier.Tier)
		}
		if tier.Count == 0 {
			t.Errorf("tier %s has zero count but was included", tier.Tier)
		}
	}
}

func TestCmdCrawl_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Capture stderr (the "No markdown files" message goes there).
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := CmdCrawl([]string{"--input", dir})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("CmdCrawl should return nil for empty dir, got: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "No markdown files found") {
		t.Errorf("expected 'No markdown files found' message on stderr, got: %s", output)
	}
}

// TestCmdCrawl_LegacyFallback removed - legacy --from-multiple flag no longer supported
