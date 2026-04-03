package knowledge

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CmdImport implements the `semanticmesh import` CLI command.
// It parses flags, derives the graph name, and calls ImportZIP.
func CmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	name := fs.String("name", "", "Name for the imported graph (default: derived from ZIP filename)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("import: ZIP path is required\nUsage: semanticmesh import <file.zip> [--name <graph-name>]")
	}

	zipPath := fs.Arg(0)

	// Derive graph name from filename if --name not provided.
	graphName := *name
	if graphName == "" {
		graphName = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}

	if err := ImportZIP(zipPath, graphName); err != nil {
		return err
	}

	return nil
}

// ImportZIP extracts a graph ZIP archive into XDG storage with the given name.
// It validates the ZIP structure, schema version, and database integrity before
// committing files to persistent storage.
func ImportZIP(zipPath, graphName string) error {
	// Step 1: Open the ZIP archive.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("import: open ZIP %q: %w", zipPath, err)
	}
	defer zr.Close()

	// Step 2: Extract to a temporary directory.
	tmpDir, err := os.MkdirTemp("", "semanticmesh-import-*")
	if err != nil {
		return fmt.Errorf("import: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	hasGraphDB := false
	hasMetadata := false

	for _, f := range zr.File {
		// Sanitize: skip directories and suspicious paths.
		if f.FileInfo().IsDir() || strings.Contains(f.Name, "..") {
			continue
		}

		destPath := filepath.Join(tmpDir, filepath.Base(f.Name))

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("import: open entry %q: %w", f.Name, err)
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("import: create %q: %w", destPath, err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			return fmt.Errorf("import: extract %q: %w", f.Name, err)
		}
		outFile.Close()
		rc.Close()

		switch filepath.Base(f.Name) {
		case "graph.db":
			hasGraphDB = true
		case "metadata.json":
			hasMetadata = true
		}
	}

	// Step 3: Validate ZIP structure.
	if !hasGraphDB || !hasMetadata {
		return fmt.Errorf("import: invalid archive — missing graph.db or metadata.json")
	}

	// Step 4: Parse and validate metadata.
	metaPath := filepath.Join(tmpDir, "metadata.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("import: read metadata.json: %w", err)
	}

	var meta ExportMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return fmt.Errorf("import: parse metadata.json: %w", err)
	}

	// Step 5: Schema version check.
	if meta.SchemaVersion > SchemaVersion {
		return fmt.Errorf("import: archive schema version %d is newer than supported version %d — upgrade semanticmesh", meta.SchemaVersion, SchemaVersion)
	}

	// Step 6: Validate database integrity.
	tmpDBPath := filepath.Join(tmpDir, "graph.db")
	testDB, err := OpenDB(tmpDBPath)
	if err != nil {
		return fmt.Errorf("import: graph database validation failed: %w", err)
	}
	testGraph := NewGraph()
	if err := testDB.LoadGraph(testGraph); err != nil {
		testDB.Close()
		return fmt.Errorf("import: graph database validation failed: %w", err)
	}
	testDB.Close()

	// Step 7: Create destination directory and copy files.
	destDir, err := graphStoragePath(graphName)
	if err != nil {
		return fmt.Errorf("import: resolve storage path: %w", err)
	}

	// Check if we're overwriting an existing graph.
	if _, err := os.Stat(destDir); err == nil {
		fmt.Fprintf(os.Stderr, "Replacing existing graph %q\n", graphName)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("import: create storage dir: %w", err)
	}

	// Copy graph.db
	if err := copyFile(tmpDBPath, filepath.Join(destDir, "graph.db")); err != nil {
		return fmt.Errorf("import: copy graph.db: %w", err)
	}

	// Copy metadata.json
	if err := copyFile(metaPath, filepath.Join(destDir, "metadata.json")); err != nil {
		return fmt.Errorf("import: copy metadata.json: %w", err)
	}

	// Step 8: Update the current graph marker.
	storageDir, err := GraphStorageDir()
	if err != nil {
		return fmt.Errorf("import: resolve storage dir: %w", err)
	}
	if err := setCurrentGraph(storageDir, graphName); err != nil {
		return fmt.Errorf("import: set current graph: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Imported graph %q (%d components, %d relationships)\n",
		graphName, meta.ComponentCount, meta.RelationshipCount)

	return nil
}

// LoadStoredGraph loads a named graph from XDG storage. If graphName is empty,
// it reads the "current" marker to determine the default graph.
// Returns structured errors for common cases:
//   - NO_GRAPH: no graph has been imported yet
//   - NOT_FOUND: the named graph does not exist
func LoadStoredGraph(graphName string) (*Graph, *ExportMetadata, error) {
	storageDir, err := GraphStorageDir()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve storage dir: %w", err)
	}

	// Resolve graph name.
	if graphName == "" {
		graphName, err = getCurrentGraph(storageDir)
		if err != nil {
			return nil, nil, fmt.Errorf("no graph imported — run 'semanticmesh import <file.zip>' first")
		}
	}

	// Check the named graph exists.
	graphDir := filepath.Join(storageDir, graphName)
	if _, err := os.Stat(graphDir); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("graph %q not found", graphName)
	}

	// Load metadata.
	metaData, err := os.ReadFile(filepath.Join(graphDir, "metadata.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("load metadata for graph %q: %w", graphName, err)
	}
	var meta ExportMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, nil, fmt.Errorf("parse metadata for graph %q: %w", graphName, err)
	}

	// Open and load the graph database.
	dbPath := filepath.Join(graphDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open graph %q database: %w", graphName, err)
	}
	defer db.Close()

	graph := NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		return nil, nil, fmt.Errorf("load graph %q: %w", graphName, err)
	}

	return graph, &meta, nil
}

// copyFile copies src to dst using streaming io.Copy.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
