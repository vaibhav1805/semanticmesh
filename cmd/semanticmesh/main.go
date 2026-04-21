package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/goparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/jsparser"
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
	case "context":
		cmdContext()
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
	llmModel := fs.String("llm-model", "us.anthropic.claude-sonnet-4-5-20250929-v1:0", "AWS Bedrock model ID")
	llmRegion := fs.String("llm-region", "us-east-1", "AWS region for Bedrock")
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

	// Run LLM-based refinement if requested.
	if *llmDiscovery {
		fmt.Fprintf(os.Stderr, "Running LLM-based refinement...\n")
		if err := runLLMRefinement(absDir, graph, *llmModel, *llmRegion, *minConfidence); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: LLM refinement failed: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Saving graph...\n")
	if err := db.SaveGraph(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving graph: %v\n", err)
		os.Exit(1)
	}

	// Save component aliases if LLM discovery was used.
	if *llmDiscovery {
		// Load aliases from cache and save to database.
		cacheDir := filepath.Join(absDir, ".bmd-llm-cache")
		cacheCfg := knowledge.DefaultLLMCacheConfig()
		cacheCfg.CacheDir = cacheDir
		cacheManager := knowledge.NewLLMCacheManager(cacheCfg)
		if err := cacheManager.Load(); err == nil {
			compCache := cacheManager.GetComponentCache()
			var aliases []knowledge.ComponentAlias
			for _, entry := range compCache.GetAllValid() {
				if entry.NameVariant != entry.CanonicalName {
					aliases = append(aliases, knowledge.ComponentAlias{
						Alias:        entry.NameVariant,
						CanonicalID:  entry.CanonicalName,
						NormalizedBy: "llm",
						Confidence:   entry.Confidence,
						CreatedAt:    entry.GeneratedAt.Unix(),
					})
				}
			}
			if len(aliases) > 0 {
				if err := db.SaveComponentAliases(aliases); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save aliases: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "✓ Saved %d component aliases\n", len(aliases))
				}
			}
		}
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
	if err := knowledge.CmdCrawl(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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

func printUsage() {
	fmt.Fprintf(os.Stderr, `semanticmesh - dependency graph analyzer for infrastructure documentation and source code

Usage:
  semanticmesh <command> [options]

Core Commands:
  export          Package knowledge as a ZIP archive (scan + discover + package)
  import          Load an exported graph ZIP into persistent storage
  query impact    Query downstream impact of a component failure
  query deps      Query what a component depends on
  query path      Find paths between two components
  query list      List components with optional filters
  mcp             Start MCP server for LLM agent access (stdio transport)

Legacy Commands (local .bmd/ workflow):
  index           Run discovery algorithms on documentation
  crawl           Preview graph statistics before exporting
  context         Assemble RAG context for a query
  graph           Export the full dependency graph as JSON or DOT

Utilities:
  clean           Remove all BMD artifacts from directory
  help            Show this help message

Examples:
  # Modern workflow (recommended)
  semanticmesh export --input ./docs --output graph.zip
  semanticmesh import graph.zip --name prod-infra
  semanticmesh query impact --component payment-api
  semanticmesh query dependencies --component web-frontend --depth all
  semanticmesh query path --from web-frontend --to primary-db
  semanticmesh query list --type service --min-confidence 0.7
  semanticmesh mcp

  # Legacy workflow
  semanticmesh index --dir ./docs
  semanticmesh crawl --input ./docs --format json
  semanticmesh context "how does auth work" --dir ./docs
  semanticmesh graph --dir ./docs --format dot

`)

}

// runLLMRefinement runs the LLM-based component and edge refinement pipeline.
func runLLMRefinement(dir string, graph *knowledge.Graph, model, region string, minConfidence float64) error {
	ctx := context.Background()

	// Initialize LLM client.
	llmCfg := knowledge.DefaultBedrockLLMConfig()
	llmCfg.Model = model
	llmCfg.AWSRegion = region
	llmClient, err := knowledge.NewBedrockLLMClient(llmCfg)
	if err != nil {
		return fmt.Errorf("create llm client: %w", err)
	}

	// Initialize cache manager.
	cacheDir := filepath.Join(dir, ".bmd-llm-cache")
	cacheCfg := knowledge.DefaultLLMCacheConfig()
	cacheCfg.CacheDir = cacheDir
	cacheManager := knowledge.NewLLMCacheManager(cacheCfg)
	if err := cacheManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to load cache: %v\n", err)
	}

	// Step 1: Refine components.
	fmt.Fprintf(os.Stderr, "  Refining components with LLM...\n")
	rawComponents := make([]*knowledge.Node, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		rawComponents = append(rawComponents, node)
	}

	compResult, err := knowledge.RefineComponents(ctx, llmClient, cacheManager, rawComponents)
	if err != nil {
		return fmt.Errorf("refine components: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  ✓ Component refinement: %d valid, %d false positives, %d normalized\n",
		compResult.Stats.ValidComponents, compResult.Stats.FalsePositives, compResult.Stats.NormalizationsFound)

	// Update graph nodes with enrichments.
	validComponentSet := make(map[string]bool)
	for _, comp := range compResult.ValidComponents {
		if node, ok := graph.Nodes[comp.Name]; ok {
			node.Description = comp.Description
			node.Tags = comp.Tags
			node.ComponentType = comp.Type
			validComponentSet[comp.Name] = true
		}
	}

	// Remove false positive nodes.
	for _, fpName := range compResult.FalsePositives {
		delete(graph.Nodes, fpName)
	}

	// Step 2: Refine edges.
	fmt.Fprintf(os.Stderr, "  Refining edges with LLM...\n")
	rawEdges := make([]*knowledge.Edge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		rawEdges = append(rawEdges, edge)
	}

	edgeResult, err := knowledge.RefineEdges(ctx, llmClient, rawEdges, validComponentSet, minConfidence)
	if err != nil {
		return fmt.Errorf("refine edges: %w", err)
	}

	fmt.Fprintf(os.Stderr, "  ✓ Edge refinement: %d valid, %d false positives, %d low-confidence\n",
		edgeResult.Stats.ValidEdges, edgeResult.Stats.FalsePositives, edgeResult.Stats.LowConfidenceEdges)

	// Rebuild graph edges with refined edges.
	graph.Edges = make(map[string]*knowledge.Edge)
	graph.BySource = make(map[string][]*knowledge.Edge)
	graph.ByTarget = make(map[string][]*knowledge.Edge)
	for _, edge := range edgeResult.ValidEdges {
		graph.Edges[edge.ID] = edge
		graph.BySource[edge.Source] = append(graph.BySource[edge.Source], edge)
		graph.ByTarget[edge.Target] = append(graph.ByTarget[edge.Target], edge)
	}

	// Save cache.
	if err := cacheManager.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to save cache: %v\n", err)
	}

	// Print metrics.
	metrics := llmClient.GetMetrics()
	fmt.Fprintf(os.Stderr, "  LLM metrics: %d requests, %d tokens, avg latency %dms\n",
		metrics.TotalRequests, metrics.TotalTokens, metrics.TotalLatencyMs/int64(max(metrics.TotalRequests, 1)))

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
