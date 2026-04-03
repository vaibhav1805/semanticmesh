package knowledge

import (
	"archive/tar"
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/semanticmesh/semanticmesh/internal/code"
	"github.com/semanticmesh/semanticmesh/internal/code/goparser"
	"github.com/semanticmesh/semanticmesh/internal/code/jsparser"
	"github.com/semanticmesh/semanticmesh/internal/code/pyparser"
)

// ExportArgs holds parsed arguments for CmdExport.
type ExportArgs struct {
	From          string  // source directory to export (--from)
	Input         string  // alias for From (--input)
	Output        string  // output ZIP file path
	DB            string  // database path override
	Version       string  // semantic version (e.g. "2.0.0")
	GitVersion    bool    // auto-detect version from git describe --tags
	Publish       string  // optional S3 URI (e.g. "s3://bucket/prefix")
	SkipDiscovery bool    // skip relationship discovery algorithms
	LLMDiscovery  bool    // enable LLM-based discovery (opt-in, default off)
	MinConfidence float64 // minimum confidence threshold for discovered edges
	AnalyzeCode   bool    // analyze source code for infrastructure dependencies
}

// ExportMetadata is the metadata stored in metadata.json inside the ZIP archive.
type ExportMetadata struct {
	Version           string   `json:"version"`
	SchemaVersion     int      `json:"schema_version"`
	CreatedAt         string   `json:"created_at"`
	ComponentCount    int      `json:"component_count"`
	RelationshipCount int      `json:"relationship_count"`
	InputPath         string   `json:"input_path"`
	IgnorePatterns    []string `json:"ignore_patterns,omitempty"`
	AliasesApplied    int      `json:"aliases_applied"`
	Checksum          string   `json:"checksum,omitempty"`
}

// KnowledgeMetadata is the metadata stored in knowledge.json inside tar.gz archives.
// Retained for backward compatibility with the legacy import pipeline.
type KnowledgeMetadata struct {
	Version   string    `json:"version"`
	Checksum  string    `json:"checksum,omitempty"` // "sha256:<hex>"
	CreatedAt time.Time `json:"created_at"`
	FileCount int       `json:"file_count"`
	DBSize    int64     `json:"db_size_bytes"`
	SourceDir string    `json:"source_dir"`
	FromRepo  string    `json:"from_repo,omitempty"`
	GitTag    string    `json:"git_tag,omitempty"`
	GitCommit string    `json:"git_commit,omitempty"`
}

// ParseExportArgs parses raw CLI arguments for the export command.
//
// Usage: semanticmesh export --input <path> --output <path> [--skip-discovery] [--llm-discovery] [--min-confidence 0.5]
func ParseExportArgs(args []string) (*ExportArgs, error) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var a ExportArgs
	fs.StringVar(&a.From, "from", ".", "Source directory to export")
	fs.StringVar(&a.Input, "input", "", "Source directory to export (alias for --from)")
	fs.StringVar(&a.Output, "output", "graph.zip", "Output ZIP file path")
	fs.StringVar(&a.DB, "db", "", "Database path override")
	fs.StringVar(&a.Version, "version", "", "Semantic version tag (e.g. 2.0.0)")
	fs.BoolVar(&a.GitVersion, "git-version", false, "Auto-detect version from git describe --tags")
	fs.StringVar(&a.Publish, "publish", "", "S3 URI to publish artifact (e.g. s3://bucket/path)")
	fs.BoolVar(&a.SkipDiscovery, "skip-discovery", false, "Skip relationship discovery algorithms")
	fs.BoolVar(&a.LLMDiscovery, "llm-discovery", false, "Enable LLM-based discovery (opt-in)")
	fs.Float64Var(&a.MinConfidence, "min-confidence", 0.5, "Minimum confidence threshold for discovered edges")
	fs.BoolVar(&a.AnalyzeCode, "analyze-code", false, "Analyze source code for infrastructure dependencies")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	// --input takes precedence over --from when both specified.
	if a.Input != "" {
		a.From = a.Input
	}

	// Positional argument overrides --from/--input.
	if pos := fs.Args(); len(pos) > 0 {
		a.From = pos[0]
	}

	return &a, nil
}

// CmdExport implements `semanticmesh export`. It runs the full export pipeline:
// scan -> detect components -> apply aliases -> discover relationships ->
// save to SQLite -> package as ZIP with graph.db + metadata.json.
func CmdExport(args []string) error {
	a, err := ParseExportArgs(args)
	if err != nil {
		return err
	}

	absFrom, err := filepath.Abs(a.From)
	if err != nil {
		return fmt.Errorf("export: resolve dir %q: %w", a.From, err)
	}

	// Verify source directory exists.
	info, err := os.Stat(absFrom)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("export: source directory %q does not exist or is not a directory", absFrom)
	}

	// Resolve version: explicit flag > git auto-detect > default.
	version := a.Version
	if version == "" && a.GitVersion {
		if gitVer, gitErr := DetectGitVersion(absFrom); gitErr == nil && gitVer != "" {
			version = gitVer
			fmt.Fprintf(os.Stderr, "  Auto-detected version from git: %s\n", version)
		} else {
			fmt.Fprintf(os.Stderr, "  Warning: git version detection failed, using default\n")
		}
	}
	if version == "" {
		version = "1.0.0"
	}

	fmt.Fprintf(os.Stderr, "Exporting graph from %s (v%s)...\n", absFrom, version)
	start := time.Now()

	// Step 1: Load .semanticmeshignore patterns.
	ignoreDirs, ignoreFiles, err := LoadIgnoreFile(absFrom)
	if err != nil {
		return fmt.Errorf("export: load .semanticmeshignore: %w", err)
	}

	// Step 2: Load alias config.
	aliasCfg, err := LoadAliasConfig(absFrom)
	if err != nil {
		return fmt.Errorf("export: load alias config: %w", err)
	}

	// Step 3: Scan directory with ignore patterns.
	scanCfg := ScanConfig{
		IgnoreDirs:        ignoreDirs,
		IgnoreFiles:       ignoreFiles,
		UseDefaultIgnores: true,
	}
	docs, err := ScanDirectory(absFrom, scanCfg)
	if err != nil {
		return fmt.Errorf("export: scan: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  %d markdown files found\n", len(docs))

	if len(docs) == 0 {
		return fmt.Errorf("export: no markdown files found in %q", absFrom)
	}

	// Step 4: Build initial graph (nodes from docs, link-based edges).
	graph := NewGraph()
	for _, doc := range docs {
		_ = graph.AddNode(&Node{
			ID:    doc.ID,
			Title: doc.Title,
			Type:  "document",
		})
	}

	extractor := NewExtractor(absFrom)
	for _, doc := range docs {
		docCopy := doc
		edges := extractor.Extract(&docCopy)
		for _, edge := range edges {
			_ = graph.AddEdge(edge)
		}
	}

	// Step 5: Run component detection.
	fmt.Fprintf(os.Stderr, "  Classifying component types...\n")
	detector := NewComponentDetector()
	components := detector.DetectComponents(graph, docs)
	var mentions []ComponentMention
	for _, comp := range components {
		if node, ok := graph.Nodes[comp.File]; ok {
			node.ComponentType = comp.Type
		}
		methods := "auto"
		if len(comp.DetectionMethods) > 0 {
			methods = strings.Join(comp.DetectionMethods, ",")
		}
		mentions = append(mentions, ComponentMention{
			ComponentID: comp.File,
			FilePath:    comp.File,
			DetectedBy:  methods,
			Confidence:  comp.TypeConfidence,
		})
	}

	// Step 6: Apply aliases — resolve node IDs and edge endpoints.
	aliasesApplied := 0
	if len(aliasCfg.Aliases) > 0 {
		fmt.Fprintf(os.Stderr, "  Applying aliases...\n")
		aliasesApplied = applyAliases(graph, aliasCfg)
	}

	// Step 7: Run discovery algorithms (unless skipped).
	var discovered []*DiscoveredEdge
	if !a.SkipDiscovery {
		fmt.Fprintf(os.Stderr, "  Running discovery algorithms...\n")
		discovered = DiscoverRelationships(docs, nil)
		for _, de := range discovered {
			if de.Edge != nil && de.Edge.Confidence >= a.MinConfidence {
				_ = graph.AddEdge(de.Edge)
			}
		}
	}

	// Step 7b: Run code analysis if requested — integrate signals into graph.
	var codeSignals []code.CodeSignal
	var codeSourceComponent string
	if a.AnalyzeCode {
		fmt.Fprintf(os.Stderr, "  Analyzing source code...\n")
		signals, codeErr := code.RunCodeAnalysis(absFrom,
			goparser.NewGoParser(),
			pyparser.NewPythonParser(),
			jsparser.NewJSParser(),
		)
		if codeErr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: code analysis failed: %v\n", codeErr)
		} else {
			code.PrintCodeSignalsSummary(os.Stderr, signals)
			codeSignals = signals
			codeSourceComponent = code.InferSourceComponent(absFrom)
			discovered = integrateCodeSignals(graph, discovered, signals, codeSourceComponent)
			fmt.Fprintf(os.Stderr, "  Code analysis: %d signals → %d total merged edges\n",
				len(signals), len(discovered))
		}
	}

	// Step 8: Save to temporary SQLite database.
	tmpDir, err := os.MkdirTemp("", "semanticmesh-export-*")
	if err != nil {
		return fmt.Errorf("export: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDBPath := filepath.Join(tmpDir, "graph.db")
	db, err := OpenDB(tmpDBPath)
	if err != nil {
		return fmt.Errorf("export: open temp db: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  Saving graph (%d nodes, %d edges)...\n", graph.NodeCount(), graph.EdgeCount())
	if err := db.SaveGraph(graph); err != nil {
		db.Close()
		return fmt.Errorf("export: save graph: %w", err)
	}

	if len(mentions) > 0 {
		if err := db.SaveComponentMentions(mentions); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to save component mentions: %v\n", err)
		}
	}

	// Save raw code signals for provenance (after DB is created and graph saved).
	if len(codeSignals) > 0 {
		if err := db.SaveCodeSignals(codeSignals, codeSourceComponent); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to save code signals: %v\n", err)
		}
	}

	db.Close()

	// Step 9: Build ExportMetadata.
	allIgnorePatterns := append(ignoreDirs, ignoreFiles...)
	meta := ExportMetadata{
		Version:           version,
		SchemaVersion:     SchemaVersion,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		ComponentCount:    graph.NodeCount(),
		RelationshipCount: graph.EdgeCount(),
		InputPath:         absFrom,
		IgnorePatterns:    allIgnorePatterns,
		AliasesApplied:    aliasesApplied,
	}

	// Step 10: Package as ZIP.
	outputPath := a.Output
	if !strings.HasSuffix(strings.ToLower(outputPath), ".zip") {
		outputPath += ".zip"
	}

	// Ensure output directory exists.
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("export: create output dir %q: %w", dir, err)
		}
	}

	if err := packageZIP(outputPath, tmpDBPath, meta); err != nil {
		return fmt.Errorf("export: package ZIP: %w", err)
	}

	// Step 11: Print summary.
	elapsed := time.Since(start)
	absOutput, _ := filepath.Abs(outputPath)
	outStat, _ := os.Stat(outputPath)
	var sizeStr string
	if outStat != nil {
		sizeStr = humanBytes(outStat.Size())
	}

	fmt.Fprintf(os.Stderr, "  Archive: %s (%s)\n", absOutput, sizeStr)
	fmt.Fprintf(os.Stderr, "  Version: %s | Schema: v%d\n", version, SchemaVersion)
	fmt.Fprintf(os.Stderr, "  Components: %d | Relationships: %d\n", meta.ComponentCount, meta.RelationshipCount)
	fmt.Fprintf(os.Stderr, "  Aliases applied: %d\n", aliasesApplied)
	fmt.Fprintf(os.Stderr, "  Completed in %dms\n", elapsed.Milliseconds())

	// Step 12: Publish to S3 if requested.
	if a.Publish != "" {
		fmt.Fprintf(os.Stderr, "  Publishing to %s...\n", a.Publish)
		if err := PublishToS3(absOutput, a.Publish, version); err != nil {
			return fmt.Errorf("export: publish: %w", err)
		}
		fmt.Fprintf(os.Stderr, "  Published successfully\n")
	}

	return nil
}

// applyAliases resolves node IDs and edge endpoints using the alias config.
// Returns the number of aliases that were applied.
func applyAliases(graph *Graph, cfg *AliasConfig) int {
	applied := 0

	// Collect nodes that need renaming.
	renames := make(map[string]string) // old ID -> new ID
	for id := range graph.Nodes {
		resolved := cfg.ResolveAlias(id)
		if resolved != id {
			renames[id] = resolved
			applied++
		}
	}

	// Apply node renames.
	for oldID, newID := range renames {
		node := graph.Nodes[oldID]
		if node == nil {
			continue
		}
		// If the canonical name already exists as a node, skip (don't overwrite).
		if _, exists := graph.Nodes[newID]; exists {
			continue
		}
		node.ID = newID
		graph.Nodes[newID] = node
		delete(graph.Nodes, oldID)

		// Update edges that reference the old ID.
		for _, edge := range graph.Edges {
			if edge.Source == oldID {
				edge.Source = newID
			}
			if edge.Target == oldID {
				edge.Target = newID
			}
		}

		// Rebuild adjacency indices for this node.
		if outgoing, ok := graph.BySource[oldID]; ok {
			graph.BySource[newID] = outgoing
			delete(graph.BySource, oldID)
		}
		if incoming, ok := graph.ByTarget[oldID]; ok {
			graph.ByTarget[newID] = incoming
			delete(graph.ByTarget, oldID)
		}
	}

	// Also resolve alias references in edge endpoints (target might be an alias).
	for _, edge := range graph.Edges {
		resolvedSource := cfg.ResolveAlias(edge.Source)
		if resolvedSource != edge.Source {
			edge.Source = resolvedSource
			applied++
		}
		resolvedTarget := cfg.ResolveAlias(edge.Target)
		if resolvedTarget != edge.Target {
			edge.Target = resolvedTarget
			applied++
		}
	}

	return applied
}

// packageZIP creates a ZIP archive containing graph.db and metadata.json.
func packageZIP(outputPath, dbPath string, meta ExportMetadata) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output %q: %w", outputPath, err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	// Add graph.db.
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("read graph.db: %w", err)
	}
	dbWriter, err := zw.Create("graph.db")
	if err != nil {
		return fmt.Errorf("create graph.db entry: %w", err)
	}
	if _, err := dbWriter.Write(dbData); err != nil {
		return fmt.Errorf("write graph.db: %w", err)
	}

	// Compute checksum over the DB file.
	h := sha256.Sum256(dbData)
	meta.Checksum = "sha256:" + hex.EncodeToString(h[:])

	// Add metadata.json.
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	metaWriter, err := zw.Create("metadata.json")
	if err != nil {
		return fmt.Errorf("create metadata.json entry: %w", err)
	}
	if _, err := metaWriter.Write(metaJSON); err != nil {
		return fmt.Errorf("write metadata.json: %w", err)
	}

	return nil
}

// addFileToTar adds a file from disk to the tar archive at the given archive path.
// Retained for backward compatibility with the legacy tar.gz import pipeline.
func addFileToTar(tw *tar.Writer, diskPath, archivePath string) error {
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    archivePath,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, f)
	return err
}

// addBytesToTar adds in-memory bytes to the tar archive at the given path.
// Retained for backward compatibility with the legacy tar.gz import pipeline.
func addBytesToTar(tw *tar.Writer, archivePath string, data []byte) error {
	header := &tar.Header{
		Name:    archivePath,
		Size:    int64(len(data)),
		Mode:    0o644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err := tw.Write(data)
	return err
}

// ─── checksum functions ──────────────────────────────────────────────────────

// ArchiveFile represents a file to be included in a knowledge archive.
type ArchiveFile struct {
	DiskPath    string
	ArchivePath string
}

// ComputeArchiveChecksum computes a SHA256 checksum over all files that will
// be included in the archive. Each file contributes its archive path (sorted
// for determinism) and content to the hash.
func ComputeArchiveChecksum(entries interface{}) (string, error) {
	var items []ArchiveFile

	switch v := entries.(type) {
	case []ArchiveFile:
		items = v
	default:
		return "", fmt.Errorf("unsupported entries type")
	}

	// Sort by archive path for deterministic checksums.
	sorted := make([]ArchiveFile, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ArchivePath < sorted[j].ArchivePath
	})

	h := sha256.New()
	for _, entry := range sorted {
		data, err := os.ReadFile(entry.DiskPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", entry.ArchivePath, err)
		}
		h.Write([]byte(entry.ArchivePath))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDirectoryChecksum computes SHA256 over extracted files in a directory,
// matching the export checksum algorithm. Skips knowledge.json itself since
// that file contains the checksum being verified.
func ComputeDirectoryChecksum(dir string) (string, error) {
	type entry struct {
		relPath string
		absPath string
	}
	var entries []entry

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Skip knowledge.json from checksum (it contains the checksum itself).
		if d.Name() == "knowledge.json" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{relPath: filepath.ToSlash(rel), absPath: path})
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort by relative path for deterministic checksums.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	h := sha256.New()
	for _, e := range entries {
		data, err := os.ReadFile(e.absPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.relPath, err)
		}
		h.Write([]byte(e.relPath))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ValidateChecksum verifies that the extracted files match the checksum in
// the metadata. Returns nil if valid, error if mismatch or computation fails.
func ValidateChecksum(extractDir string, meta KnowledgeMetadata) error {
	if meta.Checksum == "" {
		return nil // no checksum to validate
	}

	expected := strings.TrimPrefix(meta.Checksum, "sha256:")

	actual, err := ComputeDirectoryChecksum(extractDir)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected sha256:%s, got sha256:%s (artifact may be corrupted)", expected, actual)
	}

	return nil
}

// ─── git integration ─────────────────────────────────────────────────────────

// DetectGitVersion attempts to determine the version from git tags.
// Returns the version string (without leading 'v') or empty string on failure.
func DetectGitVersion(dir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	ver := strings.TrimSpace(string(out))
	ver = strings.TrimPrefix(ver, "v")
	return ver, nil
}

// DetectGitProvenance extracts git metadata (remote URL, current tag, commit hash).
func DetectGitProvenance(dir string) (fromRepo, gitTag, gitCommit string) {
	// Get remote URL.
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		fromRepo = strings.TrimSpace(string(out))
	}

	// Get current tag (exact match only).
	cmd = exec.Command("git", "describe", "--tags", "--exact-match")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		gitTag = strings.TrimSpace(string(out))
	}

	// Get commit hash.
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		gitCommit = strings.TrimSpace(string(out))
	}

	return
}

// ─── S3 distribution ─────────────────────────────────────────────────────────

// PublishToS3 uploads a local file to an S3 URI using the AWS CLI.
// Returns a descriptive error if the AWS CLI is not installed.
func PublishToS3(localPath, s3URI, version string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI not found. Install via: pip install awscli\n" +
			"Or download from: https://aws.amazon.com/cli/")
	}

	// Construct the full S3 destination path.
	destURI := s3URI
	if !strings.HasSuffix(destURI, ".zip") && !strings.HasSuffix(destURI, ".tar.gz") {
		if !strings.HasSuffix(destURI, "/") {
			destURI += "/"
		}
		destURI += fmt.Sprintf("graph-v%s.zip", version)
	}

	cmd := exec.Command("aws", "s3", "cp", localPath, destURI)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws s3 cp failed: %w", err)
	}

	return nil
}

// DownloadFromS3 downloads a file from S3 to a temporary local file.
// Returns the path to the temp file (caller must clean up).
func DownloadFromS3(s3URI string) (string, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return "", fmt.Errorf("AWS CLI not found. Install via: pip install awscli\n" +
			"Or download from: https://aws.amazon.com/cli/")
	}

	tmpFile, err := os.CreateTemp("", "semanticmesh-download-*")
	if err != nil {
		return "", err
	}
	tmpFile.Close()

	cmd := exec.Command("aws", "s3", "cp", s3URI, tmpFile.Name())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("aws s3 cp failed: %w", err)
	}

	return tmpFile.Name(), nil
}
