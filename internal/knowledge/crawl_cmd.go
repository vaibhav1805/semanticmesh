package knowledge

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/goparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/jsparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/pyparser"
	"github.com/vaibhav1805/semanticmesh/internal/code/tfparser"
)

// CrawlArgs holds parsed arguments for the crawl command.
type CrawlArgs struct {
	Input       string // source directory (--input flag)
	Format      string // output format: text or json (--format flag)
	AnalyzeCode bool   // analyze source code for infrastructure dependencies
}

// ParseCrawlArgs parses raw CLI arguments for the crawl command.
func ParseCrawlArgs(args []string) (*CrawlArgs, error) {
	fs := flag.NewFlagSet("crawl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var a CrawlArgs
	fs.StringVar(&a.Input, "input", ".", "Source directory to crawl")
	fs.StringVar(&a.Format, "format", "text", "Output format: text or json")
	fs.BoolVar(&a.AnalyzeCode, "analyze-code", false, "Analyze source code for infrastructure dependencies")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("crawl: %w", err)
	}

	// Positional argument overrides --input.
	if pos := fs.Args(); len(pos) > 0 {
		a.Input = pos[0]
	}

	return &a, nil
}

// CmdCrawl implements the graph exploration crawl command. It runs the same
// pipeline as export (scan, ignore, alias, detect, discover) and then computes
// and displays graph statistics as a pre-export diagnostic tool.
func CmdCrawl(args []string) error {
	a, err := ParseCrawlArgs(args)
	if err != nil {
		return err
	}

	absInput, err := filepath.Abs(a.Input)
	if err != nil {
		return fmt.Errorf("crawl: resolve dir %q: %w", a.Input, err)
	}

	// Verify directory exists.
	info, err := os.Stat(absInput)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("crawl: source directory %q does not exist or is not a directory", absInput)
	}

	// Step 1: Load .semanticmeshignore patterns.
	ignoreDirs, ignoreFiles, err := LoadIgnoreFile(absInput)
	if err != nil {
		return fmt.Errorf("crawl: load .semanticmeshignore: %w", err)
	}

	// Step 2: Load alias config.
	aliasCfg, err := LoadAliasConfig(absInput)
	if err != nil {
		return fmt.Errorf("crawl: load alias config: %w", err)
	}

	// Step 3: Scan directory with ignore patterns.
	scanCfg := ScanConfig{
		IgnoreDirs:        ignoreDirs,
		IgnoreFiles:       ignoreFiles,
		UseDefaultIgnores: true,
	}
	docs, err := ScanDirectory(absInput, scanCfg)
	if err != nil {
		return fmt.Errorf("crawl: scan: %w", err)
	}

	if len(docs) == 0 {
		fmt.Fprintf(os.Stderr, "No markdown files found in %s\n", absInput)
		return nil
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

	extractor := NewExtractor(absInput)
	for _, doc := range docs {
		docCopy := doc
		edges := extractor.Extract(&docCopy)
		for _, edge := range edges {
			_ = graph.AddEdge(edge)
		}
	}

	// Step 5: Run component detection.
	detector := NewComponentDetector()
	components := detector.DetectComponents(graph, docs)
	for _, comp := range components {
		if node, ok := graph.Nodes[comp.File]; ok {
			node.ComponentType = comp.Type
		}
	}

	// Step 6: Apply aliases.
	if len(aliasCfg.Aliases) > 0 {
		applyAliases(graph, aliasCfg)
	}

	// Step 7: Run discovery algorithms (matching export default of 0.5 min confidence).
	discovered := DiscoverRelationships(docs, nil)
	for _, de := range discovered {
		if de.Edge != nil && de.Edge.Confidence >= 0.5 {
			_ = graph.AddEdge(de.Edge)
		}
	}

	// Step 7b: Run code analysis if requested — integrate signals into graph.
	if a.AnalyzeCode {
		fmt.Fprintf(os.Stderr, "  Analyzing source code...\n")
		signals, codeErr := code.RunCodeAnalysis(absInput,
			goparser.NewGoParser(),
			pyparser.NewPythonParser(),
			jsparser.NewJSParser(),
			tfparser.NewTerraformParser(),
		)
		if codeErr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: code analysis failed: %v\n", codeErr)
		} else {
			code.PrintCodeSignalsSummary(os.Stderr, signals)
			sourceComponent := code.InferSourceComponent(absInput)
			discovered = integrateCodeSignals(graph, discovered, signals, sourceComponent)
			fmt.Fprintf(os.Stderr, "  Code analysis: %d signals → %d total merged edges\n",
				len(signals), len(discovered))
		}
	}

	// Step 8: Compute crawl stats.
	stats := ComputeCrawlStats(graph)

	// Step 9: Format and print.
	switch strings.ToLower(a.Format) {
	case "json":
		output, err := formatCrawlStatsJSON(stats, absInput)
		if err != nil {
			return fmt.Errorf("crawl: format JSON: %w", err)
		}
		fmt.Println(output)
	default:
		fmt.Print(formatCrawlStatsText(stats, absInput))
	}

	return nil
}

// formatCrawlStatsText produces human-readable text output for crawl statistics.
func formatCrawlStatsText(stats CrawlStats, inputPath string) string {
	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "Graph Summary (%s)\n", inputPath)
	fmt.Fprintf(&sb, "%d components, %d relationships, Quality: %.1f%%\n", stats.ComponentCount, stats.RelationshipCount, stats.QualityScore)
	sb.WriteString("\n")

	// Components by Type
	sb.WriteString("Components by Type\n")
	if len(stats.ComponentsByType) == 0 {
		sb.WriteString("  No components detected.\n")
	} else {
		// Sort types for deterministic output.
		types := make([]string, 0, len(stats.ComponentsByType))
		for t := range stats.ComponentsByType {
			types = append(types, string(t))
		}
		sort.Strings(types)

		for _, t := range types {
			ids := stats.ComponentsByType[ComponentType(t)]
			fmt.Fprintf(&sb, "  %s (%d):\n", t, len(ids))
			for _, id := range ids {
				fmt.Fprintf(&sb, "    - %s\n", id)
			}
		}
	}
	sb.WriteString("\n")

	// Confidence Distribution
	sb.WriteString("Confidence Distribution\n")
	if len(stats.ConfidenceDistribution) == 0 {
		sb.WriteString("  No relationships to analyze.\n")
	} else {
		// Find max count for bar scaling.
		maxCount := 0
		for _, tier := range stats.ConfidenceDistribution {
			if tier.Count > maxCount {
				maxCount = tier.Count
			}
		}

		const maxBarWidth = 30
		for _, tier := range stats.ConfidenceDistribution {
			barLen := int(math.Round(float64(tier.Count) / float64(maxCount) * maxBarWidth))
			if barLen < 1 && tier.Count > 0 {
				barLen = 1
			}
			bar := strings.Repeat("|", barLen)
			fmt.Fprintf(&sb, "  %s (%.2f-%.2f):  %d edges (%.1f%%) %s\n",
				string(tier.Tier), tier.RangeLow, tier.RangeHigh, tier.Count, tier.Percentage, bar)
		}
	}
	sb.WriteString("\n")

	// Quality Warnings
	sb.WriteString("Quality Warnings\n")
	if len(stats.QualityWarnings) == 0 {
		sb.WriteString("  No quality issues detected.\n")
	} else {
		for _, w := range stats.QualityWarnings {
			fmt.Fprintf(&sb, "  [%s] %s\n", w.Type, w.Message)
			for _, item := range w.Items {
				fmt.Fprintf(&sb, "    - %s\n", item)
			}
		}
	}

	return sb.String()
}

// ─── JSON output types ───────────────────────────────────────────────────────

// crawlStatsSummaryJSON is the summary section of crawl stats JSON output.
type crawlStatsSummaryJSON struct {
	ComponentCount    int     `json:"component_count"`
	RelationshipCount int     `json:"relationship_count"`
	QualityScore      float64 `json:"quality_score"`
	InputPath         string  `json:"input_path"`
}

// crawlStatsTierJSON is a single confidence tier in JSON output.
type crawlStatsTierJSON struct {
	Tier       string     `json:"tier"`
	Range      [2]float64 `json:"range"`
	Count      int        `json:"count"`
	Percentage float64    `json:"percentage"`
}

// crawlStatsComponentsJSON is the components section of JSON output.
type crawlStatsComponentsJSON struct {
	ByType map[string][]string `json:"by_type"`
}

// crawlStatsConfidenceJSON is the confidence section of JSON output.
type crawlStatsConfidenceJSON struct {
	Tiers []crawlStatsTierJSON `json:"tiers"`
}

// crawlStatsWarningJSON is a single quality warning in JSON output.
type crawlStatsWarningJSON struct {
	Type    string   `json:"type"`
	Message string   `json:"message"`
	Count   int      `json:"count"`
	Items   []string `json:"items"`
}

// crawlStatsJSON is the top-level JSON output for the crawl stats command.
type crawlStatsJSON struct {
	Summary         crawlStatsSummaryJSON    `json:"summary"`
	Components      crawlStatsComponentsJSON `json:"components"`
	Confidence      crawlStatsConfidenceJSON `json:"confidence"`
	QualityWarnings []crawlStatsWarningJSON  `json:"quality_warnings"`
}

// formatCrawlStatsJSON produces JSON output for crawl statistics.
func formatCrawlStatsJSON(stats CrawlStats, inputPath string) (string, error) {
	output := crawlStatsJSON{
		Summary: crawlStatsSummaryJSON{
			ComponentCount:    stats.ComponentCount,
			RelationshipCount: stats.RelationshipCount,
			QualityScore:      math.Round(stats.QualityScore*10) / 10,
			InputPath:         inputPath,
		},
		Components: crawlStatsComponentsJSON{
			ByType: make(map[string][]string),
		},
		Confidence: crawlStatsConfidenceJSON{
			Tiers: make([]crawlStatsTierJSON, 0, len(stats.ConfidenceDistribution)),
		},
		QualityWarnings: make([]crawlStatsWarningJSON, 0),
	}

	// Populate components by type.
	for ct, ids := range stats.ComponentsByType {
		output.Components.ByType[string(ct)] = ids
	}

	// Populate confidence tiers.
	for _, tier := range stats.ConfidenceDistribution {
		output.Confidence.Tiers = append(output.Confidence.Tiers, crawlStatsTierJSON{
			Tier:       string(tier.Tier),
			Range:      [2]float64{tier.RangeLow, tier.RangeHigh},
			Count:      tier.Count,
			Percentage: math.Round(tier.Percentage*10) / 10,
		})
	}

	// Populate quality warnings.
	for _, w := range stats.QualityWarnings {
		output.QualityWarnings = append(output.QualityWarnings, crawlStatsWarningJSON{
			Type:    w.Type,
			Message: w.Message,
			Count:   len(w.Items),
			Items:   w.Items,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
