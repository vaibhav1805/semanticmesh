package knowledge

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestZIP builds a minimal graph, saves to SQLite, creates metadata.json,
// and packages them as a ZIP archive. Returns the path to the ZIP file.
func createTestZIP(t *testing.T, dir string) string {
	t.Helper()

	// Build a minimal graph.
	graph := NewGraph()
	_ = graph.AddNode(&Node{ID: "api-gateway", Title: "API Gateway", Type: "document"})
	_ = graph.AddNode(&Node{ID: "auth-service", Title: "Auth Service", Type: "document"})
	_ = graph.AddEdge(&Edge{
		ID:         "api-gateway->auth-service",
		Source:     "api-gateway",
		Target:     "auth-service",
		Type:       "depends_on",
		Confidence: 0.9,
	})

	// Save to SQLite.
	dbPath := filepath.Join(dir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	db.Close()

	// Create metadata.
	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-03-23T00:00:00Z",
		ComponentCount:    2,
		RelationshipCount: 1,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	// Package as ZIP.
	zipPath := filepath.Join(dir, "test-graph.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(zf)

	// Add graph.db
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)

	// Add metadata.json
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)

	zw.Close()
	zf.Close()

	return zipPath
}

// createTestZIPWithMeta builds a ZIP with custom metadata (for schema version tests).
func createTestZIPWithMeta(t *testing.T, dir string, meta ExportMetadata) string {
	t.Helper()

	// Build a minimal graph and save to SQLite.
	graph := NewGraph()
	_ = graph.AddNode(&Node{ID: "test-node", Title: "Test", Type: "document"})

	dbPath := filepath.Join(dir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(graph); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	db.Close()

	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	zipPath := filepath.Join(dir, "custom-meta.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)

	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)

	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)

	zw.Close()
	zf.Close()

	return zipPath
}

func TestImportZIP_ValidArchive(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	zipPath := createTestZIP(t, buildDir)

	if err := ImportZIP(zipPath, "test-graph"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}

	// Verify files exist in storage.
	graphDir := filepath.Join(xdgDir, "semanticmesh", "graphs", "test-graph")
	if _, err := os.Stat(filepath.Join(graphDir, "graph.db")); err != nil {
		t.Errorf("graph.db not found in storage: %v", err)
	}
	if _, err := os.Stat(filepath.Join(graphDir, "metadata.json")); err != nil {
		t.Errorf("metadata.json not found in storage: %v", err)
	}

	// Verify current marker.
	storageDir := filepath.Join(xdgDir, "semanticmesh", "graphs")
	current, err := getCurrentGraph(storageDir)
	if err != nil {
		t.Fatalf("getCurrentGraph: %v", err)
	}
	if current != "test-graph" {
		t.Errorf("current graph = %q, want %q", current, "test-graph")
	}
}

func TestImportZIP_NamedGraph(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	zipPath := createTestZIP(t, buildDir)

	if err := ImportZIP(zipPath, "prod-infra"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}

	graphDir := filepath.Join(xdgDir, "semanticmesh", "graphs", "prod-infra")
	if _, err := os.Stat(graphDir); err != nil {
		t.Errorf("named graph directory not found: %v", err)
	}
}

func TestImportZIP_DefaultNameFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	zipPath := createTestZIP(t, buildDir)

	// Rename to my-graph.zip to test name derivation.
	renamedPath := filepath.Join(buildDir, "my-graph.zip")
	os.Rename(zipPath, renamedPath)

	// Use CmdImport without --name to test derivation.
	if err := CmdImport([]string{renamedPath}); err != nil {
		t.Fatalf("CmdImport: %v", err)
	}

	graphDir := filepath.Join(xdgDir, "semanticmesh", "graphs", "my-graph")
	if _, err := os.Stat(graphDir); err != nil {
		t.Errorf("expected graph name 'my-graph' from filename, got error: %v", err)
	}
}

func TestImportZIP_SchemaVersionTooNew(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	meta := ExportMetadata{
		Version:       "1.0.0",
		SchemaVersion: SchemaVersion + 100, // Way newer than supported.
		CreatedAt:     "2026-03-23T00:00:00Z",
	}
	zipPath := createTestZIPWithMeta(t, buildDir, meta)

	err := ImportZIP(zipPath, "future-graph")
	if err == nil {
		t.Fatal("expected error for schema version too new")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("error should mention 'newer than supported', got: %v", err)
	}
}

func TestImportZIP_MissingGraphDB(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "xdg"))

	// Create a ZIP with only metadata.json (no graph.db).
	zipPath := filepath.Join(tmpDir, "bad.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("metadata.json")
	w.Write([]byte(`{"version":"1.0.0","schema_version":1}`))
	zw.Close()
	zf.Close()

	err := ImportZIP(zipPath, "bad")
	if err == nil {
		t.Fatal("expected error for missing graph.db")
	}
	if !strings.Contains(err.Error(), "missing graph.db") {
		t.Errorf("error should mention 'missing graph.db', got: %v", err)
	}
}

func TestImportZIP_ReimportOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	zipPath := createTestZIP(t, buildDir)

	// First import.
	if err := ImportZIP(zipPath, "overwrite-test"); err != nil {
		t.Fatalf("first ImportZIP: %v", err)
	}

	// Second import (should succeed, overwriting).
	if err := ImportZIP(zipPath, "overwrite-test"); err != nil {
		t.Fatalf("second ImportZIP (overwrite): %v", err)
	}

	// Verify files still exist.
	graphDir := filepath.Join(xdgDir, "semanticmesh", "graphs", "overwrite-test")
	if _, err := os.Stat(filepath.Join(graphDir, "graph.db")); err != nil {
		t.Errorf("graph.db missing after overwrite: %v", err)
	}
}

func TestLoadStoredGraph_NoGraphImported(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "xdg"))

	_, _, err := LoadStoredGraph("")
	if err == nil {
		t.Fatal("expected error when no graph imported")
	}
	if !strings.Contains(err.Error(), "no graph imported") {
		t.Errorf("error should mention 'no graph imported', got: %v", err)
	}
}

func TestLoadStoredGraph_NamedGraph(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	zipPath := createTestZIP(t, buildDir)
	if err := ImportZIP(zipPath, "query-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}

	graph, meta, err := LoadStoredGraph("query-test")
	if err != nil {
		t.Fatalf("LoadStoredGraph: %v", err)
	}

	if graph.NodeCount() != 2 {
		t.Errorf("node count = %d, want 2", graph.NodeCount())
	}
	if graph.EdgeCount() != 1 {
		t.Errorf("edge count = %d, want 1", graph.EdgeCount())
	}
	if meta.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", meta.Version, "1.0.0")
	}
}
