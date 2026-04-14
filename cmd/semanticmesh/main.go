package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/goparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/jsparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/pyparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/tfparser"
	"github.com/vaibhav1805/semanticmesh/internal/knowledge"
	mcpserver "github.com/vaibhav1805/semanticmesh/internal/mcp"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "index":
		cmdIndex()
	case "crawl":
		cmdCrawl()
	case "depends":
		cmdDepends()
	case "components":
		cmdComponents()
	case "list":
		cmdList()
	case "context":
		cmdContext()
	case "relationships":
		cmdRelationships()
	case "graph":
		cmdGraph()
	case "export":
		cmdExport()
	case "import":
		cmdImport()
	case "mcp":
		cmdMCP()
	case "query":
		cmdQueryMain()
	case "clean":
		cmdClean()
	case "discover":
		cmdDiscover()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func cmdIndex() {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory to index")
	skipDiscovery := fs.Bool("skip-discovery", false, "Skip relationship discovery")
	llmDiscovery := fs.Bool("llm-discovery", false, "Enable LLM-based discovery")
	minConfidence := fs.Float64("min-confidence", 0.5, "Minimum confidence threshold")
	analyzeCode := fs.Bool("analyze-code", false, "Analyze source code for infrastructure dependencies")

	fs.Parse(os.Args[2:])

	// Get absolute directory path
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize knowledge base
	kb := knowledge.DefaultKnowledge()

	// Scan documents
	docs, err := kb.Scan(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning documents: %v\n", err)
		os.Exit(1)
	}

	if len(docs) == 0 {
		fmt.Fprintf(os.Stderr, "No markdown files found in %s\n", absDir)
		os.Exit(1)
	}

	graph := knowledge.NewGraph()

	// Add documents as graph nodes.
	for _, doc := range docs {
		_ = graph.AddNode(&knowledge.Node{
			ID:    doc.ID,
			Title: doc.Title,
			Type:  "document",
		})
	}

	// Extract link-based edges
	fmt.Fprintf(os.Stderr, "Extracting links...\n")
	extractor := knowledge.NewExtractor(absDir)
	for _, doc := range docs {
		docCopy := doc // copy for iteration
		edges := extractor.Extract(&docCopy)
		for _, edge := range edges {
			_ = graph.AddEdge(edge)
		}
	}

	// Run discovery algorithms
	if !*skipDiscovery {
		fmt.Fprintf(os.Stderr, "Running discovery algorithms...\n")
		discovered := knowledge.DiscoverRelationships(docs, nil)
		for _, de := range discovered {
			if de.Edge != nil && de.Edge.Confidence >= *minConfidence {
				_ = graph.AddEdge(de.Edge)
			}
		}
	}

	// Save to database
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .bmd directory: %v\n", err)
		os.Exit(1)
	}

	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Classify component types on graph nodes before saving.
	fmt.Fprintf(os.Stderr, "Classifying component types...\n")
	detector := knowledge.NewComponentDetector()
	components := detector.DetectComponents(graph, docs)

	// Add infrastructure components as nodes to the graph
	// (they were extracted from text but don't have file nodes)
	for _, comp := range components {
		if len(comp.DetectionMethods) > 0 && comp.DetectionMethods[0] == "infrastructure-extraction" {
			// Add as graph node if not already present
			if _, exists := graph.Nodes[comp.ID]; !exists {
				_ = graph.AddNode(&knowledge.Node{
					ID:            comp.ID,
					Type:          "infrastructure",
					Title:         comp.Name,
					ComponentType: comp.Type,
				})
			}
		}
	}

	var mentions []knowledge.ComponentMention
	typeCount := 0
	for _, comp := range components {
		// For infrastructure components, use comp.ID as the node ID
		// For document-based components, use comp.File as the node ID
		nodeID := comp.File
		if len(comp.DetectionMethods) > 0 && comp.DetectionMethods[0] == "infrastructure-extraction" {
			nodeID = comp.ID
		}

		// Update the graph node with the detected component type.
		if node, ok := graph.Nodes[nodeID]; ok {
			node.ComponentType = comp.Type
			typeCount++
		}
		// Build component mention for provenance tracking.
		methods := "auto"
		if len(comp.DetectionMethods) > 0 {
			methods = strings.Join(comp.DetectionMethods, ",")
		}
		mentions = append(mentions, knowledge.ComponentMention{
			ComponentID: nodeID,  // Use nodeID (could be comp.ID for infra or comp.File for docs)
			FilePath:    comp.File,
			DetectedBy:  methods,
			Confidence:  comp.TypeConfidence,
		})
	}

	fmt.Fprintf(os.Stderr, "Saving graph...\n")
	if err := db.SaveGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving graph: %v\n", err)
		os.Exit(1)
	}

	// Save component mentions for provenance tracking.
	if len(mentions) > 0 {
		if err := db.SaveComponentMentions(mentions); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save component mentions: %v\n", err)
		}
	}

	// Run code analysis if requested.
	if *analyzeCode {
		fmt.Fprintf(os.Stderr, "Analyzing source code...\n")
		signals, codeErr := code.RunCodeAnalysis(absDir,
			goparser.NewGoParser(),
			pyparser.NewPythonParser(),
			jsparser.NewJSParser(),
			tfparser.NewTerraformParser(),
			mendixparser.NewMendixParser(""),
		)
		if codeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: code analysis failed: %v\n", codeErr)
		} else {
			code.PrintCodeSignalsSummary(os.Stderr, signals)
			fmt.Fprintf(os.Stderr, "Code analysis: %d signals detected\n", len(signals))

			// Save code signals to database
			if len(signals) > 0 {
				sourceComponent := code.InferSourceComponent(absDir)
				if err := db.SaveCodeSignals(signals, sourceComponent); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save code signals: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "✓ Saved %d code signals to database\n", len(signals))
				}
			}
		}
	}

	fmt.Printf("✓ Indexed %d documents\n", len(docs))
	fmt.Printf("✓ Found %d relationships\n", graph.EdgeCount())
	fmt.Printf("✓ Classified %d component types\n", typeCount)
	if *llmDiscovery {
		fmt.Printf("✓ LLM discovery enabled\n")
	}
}

func cmdCrawl() {
	err := knowledge.CmdCrawl(os.Args[2:])
	if err == nil {
		return
	}
	if errors.Is(err, knowledge.ErrLegacyCrawl) {
		// Fall through to existing targeted traversal logic.
		cmdCrawlLegacy()
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

// cmdCrawlLegacy implements the original --from-multiple targeted traversal mode.
func cmdCrawlLegacy() {
	fs := flag.NewFlagSet("crawl", flag.ExitOnError)
	fromMultiple := fs.String("from-multiple", "", "Comma-separated starting files")
	dir := fs.String("dir", ".", "Directory that was indexed")
	direction := fs.String("direction", "backward", "Traversal direction: forward, backward, or both")
	depth := fs.Int("depth", 3, "Max traversal depth")
	format := fs.String("format", "json", "Output format: json, tree, dot, or list")

	fs.Parse(os.Args[2:])

	if *fromMultiple == "" {
		fmt.Fprintf(os.Stderr, "Error: --from-multiple is required\n")
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph from the indexed directory
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	startFiles := strings.Split(*fromMultiple, ",")
	for i := range startFiles {
		startFiles[i] = strings.TrimSpace(startFiles[i])
	}

	opts := knowledge.CrawlOptions{
		FromFiles:     startFiles,
		Direction:     *direction,
		MaxDepth:      *depth,
		IncludeCycles: false,
	}

	result := graph.CrawlMulti(opts)

	switch *format {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	case "tree":
		// ASCII tree output
		for node := range result.Nodes {
			fmt.Printf("  %s\n", node)
		}
	case "dot":
		// DOT format for Graphviz
		fmt.Println("digraph {")
		for _, edge := range graph.Edges {
			if _, ok := result.Nodes[edge.Source]; ok {
				if _, ok := result.Nodes[edge.Target]; ok {
					fmt.Printf("  \"%s\" -> \"%s\";\n", edge.Source, edge.Target)
				}
			}
		}
		fmt.Println("}")
	case "list":
		// Simple list format
		for node := range result.Nodes {
			fmt.Println(node)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s\n", *format)
		os.Exit(1)
	}
}

func cmdDepends() {
	fs := flag.NewFlagSet("depends", flag.ExitOnError)
	service := fs.String("service", "", "Service name")
	dir := fs.String("dir", ".", "Directory that was indexed")
	format := fs.String("format", "json", "Output format: json or text")
	transitive := fs.Bool("transitive", false, "Include transitive dependencies")

	fs.Parse(os.Args[2:])

	if *service == "" && fs.NArg() > 0 {
		*service = fs.Arg(0)
	}

	if *service == "" {
		fmt.Fprintf(os.Stderr, "Error: service name is required\n")
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	// Find dependencies
	deps := make(map[string]int)
	if *transitive {
		// BFS for transitive dependencies
		visited := make(map[string]bool)
		queue := []string{*service}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if visited[current] {
				continue
			}
			visited[current] = true
			for _, edge := range graph.Edges {
				if edge.Source == current && edge.Target != *service {
					deps[edge.Target]++
					if !visited[edge.Target] {
						queue = append(queue, edge.Target)
					}
				}
			}
		}
	} else {
		// Direct dependencies only
		for _, edge := range graph.Edges {
			if edge.Source == *service && edge.Target != *service {
				deps[edge.Target]++
			}
		}
	}

	if *format == "json" {
		data, _ := json.MarshalIndent(deps, "", "  ")
		fmt.Println(string(data))
	} else {
		for dep := range deps {
			fmt.Println(dep)
		}
	}
}

func cmdComponents() {
	fs := flag.NewFlagSet("components", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory that was indexed")
	format := fs.String("format", "json", "Output format: json or text")

	fs.Parse(os.Args[2:])

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph to extract components
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	components := make(map[string]int)
	for node := range graph.Nodes {
		components[node]++
	}

	if *format == "json" {
		data, _ := json.MarshalIndent(components, "", "  ")
		fmt.Println(string(data))
	} else {
		for comp := range components {
			fmt.Println(comp)
		}
	}
}

func cmdList() {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory that was indexed")
	typeName := fs.String("type", "", "Filter by component type (e.g. service, database, cache)")
	includeTags := fs.Bool("include-tags", false, "Include tag-based matches in addition to primary type matches")

	fs.Parse(os.Args[2:])

	if *typeName == "" {
		fmt.Fprintf(os.Stderr, "Error: --type is required\n")
		fmt.Fprintf(os.Stderr, "Usage: semanticmesh list --type TYPE [--include-tags]\n")
		fmt.Fprintf(os.Stderr, "\nValid types: ")
		for _, ct := range knowledge.AllComponentTypes() {
			fmt.Fprintf(os.Stderr, "%s ", string(ct))
		}
		fmt.Fprintf(os.Stderr, "\n")
		os.Exit(1)
	}

	ct := knowledge.ComponentType(*typeName)

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph from the indexed directory.
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	// Collect results.
	type listResult struct {
		Name       string  `json:"name"`
		ID         string  `json:"id"`
		Type       string  `json:"type"`
		MatchType  string  `json:"match_type"`
		Confidence float64 `json:"confidence"`
		File       string  `json:"file"`
	}

	var results []listResult
	primaryCount := 0
	tagCount := 0

	for _, node := range graph.Nodes {
		if node.ComponentType == ct {
			// Primary type match.
			results = append(results, listResult{
				Name:       node.Title,
				ID:         node.ID,
				Type:       string(node.ComponentType),
				MatchType:  "primary",
				Confidence: 1.0, // Exact type match.
				File:       node.ID,
			})
			primaryCount++
		} else if *includeTags {
			// Tag-based match: check if the node's name or title contains the type keyword.
			inferredType, inferredConf := knowledge.InferComponentType(node.ID, node.Title)
			if inferredType == ct {
				results = append(results, listResult{
					Name:       node.Title,
					ID:         node.ID,
					Type:       string(node.ComponentType),
					MatchType:  "tag",
					Confidence: inferredConf,
					File:       node.ID,
				})
				tagCount++
			}
		}
	}

	// Build output.
	type listOutput struct {
		Components []listResult `json:"components"`
		Summary    struct {
			Type         string `json:"type"`
			Mode         string `json:"mode"`
			PrimaryCount int    `json:"primary_count"`
			TagCount     int    `json:"tag_count"`
			TotalCount   int    `json:"total_count"`
		} `json:"summary"`
	}

	output := listOutput{Components: results}
	if output.Components == nil {
		output.Components = []listResult{}
	}
	output.Summary.Type = *typeName
	if *includeTags {
		output.Summary.Mode = "inclusive"
	} else {
		output.Summary.Mode = "strict"
	}
	output.Summary.PrimaryCount = primaryCount
	output.Summary.TagCount = tagCount
	output.Summary.TotalCount = primaryCount + tagCount

	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

func cmdContext() {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory to search")
	format := fs.String("format", "markdown", "Output format: markdown or json")
	top := fs.Int("top", 5, "Max sections to return")

	fs.Parse(os.Args[2:])

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: query is required\n")
		os.Exit(1)
	}

	query := fs.Arg(0)
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load documents for context assembly
	docs, err := knowledge.DefaultKnowledge().Scan(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning documents: %v\n", err)
		os.Exit(1)
	}

	// Simple context assembly (in practice, would use semantic search)
	type contextResult struct {
		Query   string   `json:"query"`
		Sections []string `json:"sections"`
	}

	result := contextResult{Query: query, Sections: []string{}}
	for i, doc := range docs {
		if i >= *top {
			break
		}
		result.Sections = append(result.Sections, doc.Path)
	}

	if *format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Context for: %s\n", query)
		for _, section := range result.Sections {
			fmt.Printf("  - %s\n", section)
		}
	}
}

func cmdRelationships() {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory that was indexed")
	format := fs.String("format", "json", "Output format: json or text")
	minConfidence := fs.Float64("min-confidence", 0.0, "Minimum confidence threshold")

	fs.Parse(os.Args[2:])

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	// Filter edges by confidence
	type edgeInfo struct {
		Source     string  `json:"source"`
		Target     string  `json:"target"`
		Type       string  `json:"type"`
		Confidence float64 `json:"confidence"`
	}

	var edges []edgeInfo
	for _, edge := range graph.Edges {
		if edge.Confidence >= *minConfidence {
			edges = append(edges, edgeInfo{
				Source:     edge.Source,
				Target:     edge.Target,
				Type:       string(edge.Type),
				Confidence: edge.Confidence,
			})
		}
	}

	if *format == "json" {
		data, _ := json.MarshalIndent(edges, "", "  ")
		fmt.Println(string(data))
	} else {
		for _, edge := range edges {
			fmt.Printf("%s -> %s (%s, %.2f)\n", edge.Source, edge.Target, edge.Type, edge.Confidence)
		}
	}
}

func cmdGraph() {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory that was indexed")
	format := fs.String("format", "json", "Output format: json or dot")

	fs.Parse(os.Args[2:])

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	// Load the graph
	dbPath := filepath.Join(absDir, ".bmd", "knowledge.db")
	db, err := knowledge.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	graph := knowledge.NewGraph()
	if err := db.LoadGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		os.Exit(1)
	}

	if *format == "dot" {
		fmt.Println("digraph {")
		for _, edge := range graph.Edges {
			fmt.Printf("  \"%s\" -> \"%s\";\n", edge.Source, edge.Target)
		}
		fmt.Println("}")
	} else {
		data, _ := json.MarshalIndent(graph, "", "  ")
		fmt.Println(string(data))
	}
}

func cmdExport() {
	if err := knowledge.CmdExport(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdImport() {
	if err := knowledge.CmdImport(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdQueryMain() {
	if err := knowledge.CmdQuery(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdMCP() {
	if err := mcpserver.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

func cmdClean() {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory to clean")

	fs.Parse(os.Args[2:])

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Cleaning BMD artifacts from %s...\n", absDir)
	fmt.Fprintf(os.Stderr, "  Removing .bmd directory\n")
	if err := os.RemoveAll(filepath.Join(absDir, ".bmd")); err != nil {
		fmt.Fprintf(os.Stderr, "Error cleaning: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Clean complete\n")
}

func cmdDiscover() {
	fmt.Println("Discovery subcommand - not yet implemented")
	os.Exit(1)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `semanticmesh - intelligent graph discovery service for markdown documentation

Usage:
  semanticmesh <command> [options]

Commands:
  index           Run discovery algorithms on documentation
  crawl           Traverse the dependency graph from starting files
  depends         Show service dependencies
  components      List discovered components
  list            List components filtered by type
  context         Assemble RAG context for a query
  relationships   List discovered relationships with confidence scores
  graph           Export the full dependency graph
  export          Package knowledge as a ZIP archive
  import          Load an exported graph ZIP into persistent storage
  mcp             Start MCP server for LLM agent access (stdio transport)
  query impact    Query downstream impact of a component failure
  query deps      Query what a component depends on
  query path      Find paths between two components
  query list      List components with optional filters
  clean           Remove all BMD artifacts from directory
  discover        Run semantic discovery (experimental)
  help            Show this help message

Examples:
  semanticmesh index --dir ./docs
  semanticmesh crawl --from-multiple api.md,auth.md --direction backward
  semanticmesh depends --service api-gateway --dir ./docs
  semanticmesh components --dir ./docs --format json
  semanticmesh list --type service --dir ./docs
  semanticmesh list --type database --include-tags --dir ./docs
  semanticmesh context "how does auth work" --dir ./docs
  semanticmesh relationships --dir ./docs --min-confidence 0.8
  semanticmesh graph --dir ./docs --format dot
  semanticmesh export --input ./docs --output graph.zip
  semanticmesh import graph.zip --name prod-infra
  semanticmesh query impact --component payment-api
  semanticmesh query dependencies --component web-frontend --depth all
  semanticmesh query path --from web-frontend --to primary-db
  semanticmesh query list --type service --min-confidence 0.7
  semanticmesh mcp

`)

}
