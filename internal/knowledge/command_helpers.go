package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveStrategy resolves the indexing strategy flag from CLI args or environment
func resolveStrategy(flagValue string) string {
	// Flag value takes precedence
	if flagValue != "" {
		return flagValue
	}
	// Environment variable next
	if env := os.Getenv("BMD_STRATEGY"); env != "" {
		return env
	}
	// Default to BM25
	return "bm25"
}

// splitPositionalsAndFlags splits CLI arguments into positional args and flags
func splitPositionalsAndFlags(args []string) (positionals []string, flags []string) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// This is a flag token. Peek to see if the next token is a value.
			flags = append(flags, arg)
			// Check whether the flag is of the form --flag=value (no next token).
			// Also handle bool flags that have no value.
			if !strings.Contains(arg, "=") {
				// Next arg might be a value if it doesn't start with '-'.
				// We need to consume the value to avoid mis-classifying it as positional.
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					// Check if the current flag is a bool flag by looking at name.
					flagName := strings.TrimLeft(arg, "-")
					if isBoolFlag(flagName) {
						// Bool flags don't consume the next argument.
					} else {
						i++
						flags = append(flags, args[i])
					}
				}
			}
		} else {
			positionals = append(positionals, arg)
		}
		i++
	}
	return positionals, flags
}

// isBoolFlag returns true for known boolean flag names used in our commands.
func isBoolFlag(name string) bool {
	boolFlags := map[string]bool{
		"watch":           true,
		"transitive":      true,
		"registry":        true,
		"no-hybrid":       true,
		"with-llm":        true,
		"include-signals": true,
		"show-confidence": true,
		"accept-all":      true,
		"reject-all":      true,
		"edit":            true,
		"llm-discovery":   true,
		"skip-discovery":  true,
	}
	return boolFlags[name]
}

// defaultDBPath returns the default database path for a given directory.
func defaultDBPath(dir string) string {
	return filepath.Join(dir, ".bmd", "knowledge.db")
}

// openOrBuildIndex opens an existing database at dbPath, or if one does not
// exist, tries to build it from the directory at absDir.
func openOrBuildIndex(absDir, dbPath string) (*Database, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Auto-build index if database doesn't exist.
		fmt.Fprintln(os.Stderr, "No index found, building...")
		if err2 := buildIndex(absDir, dbPath); err2 != nil {
			return nil, fmt.Errorf("auto-build index: %w", err2)
		}
	} else {
		// Database exists — check if the index is stale.
		db, openErr := OpenDB(dbPath)
		if openErr != nil {
			return nil, fmt.Errorf("open db %q: %w", dbPath, openErr)
		}
		stale, staleErr := db.IsIndexStale(absDir)
		_ = db.Close()
		if staleErr == nil && stale {
			// Silently rebuild.
			if err2 := buildIndex(absDir, dbPath); err2 != nil {
				return nil, fmt.Errorf("auto-refresh index: %w", err2)
			}
		}
	}
	db, err := OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db %q: %w", dbPath, err)
	}
	return db, nil
}

// buildIndex is a helper for openOrBuildIndex - builds index from directory
func buildIndex(absDir, dbPath string) error {
	// Create .bmd directory if needed
	bmdDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(bmdDir, 0755); err != nil {
		return fmt.Errorf("create .bmd directory: %w", err)
	}

	// Create/open database
	db, err := OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	defer db.Close()

	// Scan documents
	k := NewKnowledge(ScanConfig{UseDefaultIgnores: true})
	docs, err := k.Scan(absDir)
	if err != nil {
		return fmt.Errorf("scan documents: %w", err)
	}

	// Extract edges and save
	graph := NewGraph()
	extractor := NewExtractor(absDir)
	for _, doc := range docs {
		edges := extractor.Extract(&doc)
		for _, edge := range edges {
			_ = graph.AddEdge(edge)
		}
	}

	// Save to database
	if err := db.SaveGraph(graph); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}

	return nil
}

// classifyIndexError maps an openOrBuildIndex error to the appropriate error classification
func classifyIndexError(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no index") || strings.Contains(msg, "index not found") {
		return "index_not_found"
	}
	return "internal_error"
}

// pruneDanglingEdges removes edges from graph where either the source or
// target node does not exist in graph.Nodes.
func pruneDanglingEdges(graph *Graph) {
	for id, e := range graph.Edges {
		_, srcOK := graph.Nodes[e.Source]
		_, tgtOK := graph.Nodes[e.Target]
		if !srcOK || !tgtOK {
			delete(graph.Edges, id)
			// Clean adjacency lists.
			graph.BySource[e.Source] = removeEdgeFromSlice(graph.BySource[e.Source], id)
			graph.ByTarget[e.Target] = removeEdgeFromSlice(graph.ByTarget[e.Target], id)
		}
	}
}

// findNodeForService searches for a graph node matching serviceID by ID or by
// filename stem. Returns the node ID string, or "" when not found.
func findNodeForService(graph *Graph, serviceID string) string {
	lowerSvc := strings.ToLower(serviceID)

	// Exact match first.
	if _, ok := graph.Nodes[serviceID]; ok {
		return serviceID
	}

	// Case-insensitive match on ID.
	for id := range graph.Nodes {
		if strings.ToLower(id) == lowerSvc {
			return id
		}
	}

	// Match by filename stem.
	for id := range graph.Nodes {
		stem := strings.ToLower(filenameStem(id))
		if stem == lowerSvc {
			return id
		}
	}

	return ""
}

// humanBytes formats a byte count as a human-readable string (KB, MB, etc.).
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	div := int64(unit)
	exp := 0
	for i := 0; i < len(units)-1; i++ {
		if float64(n)/float64(div) < unit {
			break
		}
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%s", float64(n)/float64(div), units[exp])
}
