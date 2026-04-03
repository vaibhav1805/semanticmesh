package knowledge

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// --- Response envelope types -------------------------------------------------

// QueryEnvelope is the top-level JSON structure returned by all query commands.
// It wraps query parameters, results, and graph metadata into a consistent
// contract that AI agents can parse reliably.
type QueryEnvelope struct {
	Query    QueryEnvelopeParams   `json:"query"`
	Results  interface{}           `json:"results"`
	Metadata QueryEnvelopeMetadata `json:"metadata"`
}

// QueryEnvelopeParams describes the query that was executed.
type QueryEnvelopeParams struct {
	Type          string  `json:"type"`
	Component     string  `json:"component,omitempty"`
	From          string  `json:"from,omitempty"`
	To            string  `json:"to,omitempty"`
	Depth         int     `json:"depth,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	SourceType    string  `json:"source_type,omitempty"`
}

// QueryEnvelopeMetadata contains graph-level metadata for the response.
type QueryEnvelopeMetadata struct {
	ExecutionTimeMs int64        `json:"execution_time_ms"`
	NodeCount       int          `json:"node_count"`
	EdgeCount       int          `json:"edge_count"`
	GraphName       string       `json:"graph_name"`
	GraphVersion    string       `json:"graph_version"`
	CreatedAt       string       `json:"created_at"`
	ComponentCount  int          `json:"component_count"`
	CyclesDetected  []CycleEntry `json:"cycles_detected,omitempty"`
}

// EnrichedRelationship extends edge data with a human-readable confidence tier.
type EnrichedRelationship struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	Confidence       float64 `json:"confidence"`
	ConfidenceTier   string  `json:"confidence_tier"`
	Type             string  `json:"type"`
	SourceFile       string  `json:"source_file"`
	ExtractionMethod string  `json:"extraction_method"`
	SourceType       string  `json:"source_type"`
}

// --- Path result types -------------------------------------------------------

// PathResult is the results payload for path queries.
type PathResult struct {
	Paths  []PathInfo `json:"paths"`
	Count  int        `json:"count"`
	Reason string     `json:"reason,omitempty"`
}

// PathInfo describes a single path between two components.
type PathInfo struct {
	Nodes           []string  `json:"nodes"`
	Hops            []HopInfo `json:"hops"`
	TotalConfidence float64   `json:"total_confidence"`
}

// HopInfo describes a single hop in a path.
type HopInfo struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	Confidence       float64 `json:"confidence"`
	ConfidenceTier   string  `json:"confidence_tier"`
	SourceFile       string  `json:"source_file"`
	ExtractionMethod string  `json:"extraction_method"`
}

// --- List result types -------------------------------------------------------

// ListResult is the results payload for list queries.
type ListResult struct {
	Components []ListComponent `json:"components"`
	Count      int             `json:"count"`
}

// ListComponent describes a component in a list query result.
type ListComponent struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	IncomingEdges int    `json:"incoming_edges"`
	OutgoingEdges int    `json:"outgoing_edges"`
}

// --- Impact/dependencies result types ----------------------------------------

// ImpactResult is the results payload for impact and dependencies queries.
type ImpactResult struct {
	AffectedNodes []ImpactNode           `json:"affected_nodes"`
	Relationships []EnrichedRelationship `json:"relationships"`
}

// ImpactNode describes a node reached during impact or dependencies traversal.
type ImpactNode struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	Distance       int             `json:"distance"`
	ConfidenceTier string          `json:"confidence_tier"`
	Mentions       []MentionDetail `json:"mentions,omitempty"`
	MentionCount   int             `json:"mention_count,omitempty"`
}

// MentionDetail describes a single detection provenance record for JSON output.
type MentionDetail struct {
	FilePath        string  `json:"file_path"`
	DetectionMethod string  `json:"detection_method"`
	Confidence      float64 `json:"confidence"`
	Context         string  `json:"context,omitempty"`
}

// --- Cycle detection types ---------------------------------------------------

// CycleEntry records a back-edge detected during BFS traversal, indicating
// a cycle in the dependency graph. From is the node being visited, To is the
// already-visited neighbor that creates the cycle.
type CycleEntry struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// --- Error JSON type ---------------------------------------------------------

type queryErrorJSON struct {
	Error       string   `json:"error"`
	Code        string   `json:"code"`
	Suggestions []string `json:"suggestions,omitempty"`
	Action      string   `json:"action,omitempty"`
}

// --- CmdQuery: top-level router ----------------------------------------------

// CmdQuery is the entry point for the `graphmd query` CLI command.
// It routes to subcommands: impact, dependencies, path, list.
func CmdQuery(args []string) error {
	if len(args) == 0 {
		printQueryUsage(os.Stderr)
		return fmt.Errorf("query: subcommand required")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "impact":
		return cmdQueryImpact(subArgs)
	case "dependencies", "deps":
		return cmdQueryDependencies(subArgs)
	case "path":
		return cmdQueryPath(subArgs)
	case "list":
		return cmdQueryList(subArgs)
	default:
		printQueryUsage(os.Stderr)
		return fmt.Errorf("query: unknown subcommand %q", subcommand)
	}
}

// --- Impact subcommand -------------------------------------------------------

func cmdQueryImpact(args []string) error {
	fs := flag.NewFlagSet("query impact", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	component := fs.String("component", "", "Component to query impact for (required)")
	depthStr := fs.String("depth", "1", "Traversal depth (integer or \"all\")")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence threshold")
	sourceType := fs.String("source-type", "", "Filter by detection source: markdown, code, both")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")
	includeProvenance := fs.Bool("include-provenance", false, "Include detection provenance (mentions) for each node")
	maxMentions := fs.Int("max-mentions", 5, "Maximum mentions per component (0 = unlimited)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query impact: %w", err)
	}

	depth, err := parseDepth(*depthStr)
	if err != nil {
		return writeErrorJSONStdout(err.Error(), "INVALID_ARG", nil)
	}

	envelope, err := ExecuteImpactQuery(QueryImpactParams{
		Component:         *component,
		Depth:             depth,
		MinConfidence:     *minConf,
		SourceType:        *sourceType,
		GraphName:         *graphName,
		IncludeProvenance: *includeProvenance,
		MaxMentions:       *maxMentions,
	})
	if err != nil {
		return handleQueryError(err)
	}

	return outputEnvelope(*envelope, *format, "impact")
}

// --- Dependencies subcommand -------------------------------------------------

func cmdQueryDependencies(args []string) error {
	fs := flag.NewFlagSet("query dependencies", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	component := fs.String("component", "", "Component to query dependencies for (required)")
	depthStr := fs.String("depth", "1", "Traversal depth (integer or \"all\")")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence threshold")
	sourceType := fs.String("source-type", "", "Filter by detection source: markdown, code, both")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")
	includeProvenance := fs.Bool("include-provenance", false, "Include detection provenance (mentions) for each node")
	maxMentions := fs.Int("max-mentions", 5, "Maximum mentions per component (0 = unlimited)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query dependencies: %w", err)
	}

	depth, err := parseDepth(*depthStr)
	if err != nil {
		return writeErrorJSONStdout(err.Error(), "INVALID_ARG", nil)
	}

	envelope, err := ExecuteDependenciesQuery(QueryDependenciesParams{
		Component:         *component,
		Depth:             depth,
		MinConfidence:     *minConf,
		SourceType:        *sourceType,
		GraphName:         *graphName,
		IncludeProvenance: *includeProvenance,
		MaxMentions:       *maxMentions,
	})
	if err != nil {
		return handleQueryError(err)
	}

	return outputEnvelope(*envelope, *format, "dependencies")
}

// --- Path subcommand ---------------------------------------------------------

func cmdQueryPath(args []string) error {
	fs := flag.NewFlagSet("query path", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	from := fs.String("from", "", "Source component (required)")
	to := fs.String("to", "", "Target component (required)")
	limit := fs.Int("limit", 10, "Maximum number of paths to return")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence per hop")
	sourceType := fs.String("source-type", "", "Filter by detection source: markdown, code, both")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query path: %w", err)
	}

	envelope, err := ExecutePathQuery(QueryPathParams{
		From:          *from,
		To:            *to,
		Limit:         *limit,
		MinConfidence: *minConf,
		SourceType:    *sourceType,
		GraphName:     *graphName,
	})
	if err != nil {
		return handleQueryError(err)
	}

	// No path found is success (exit 0).
	return outputEnvelopeSuccess(*envelope, *format, "path")
}

// --- List subcommand ---------------------------------------------------------

func cmdQueryList(args []string) error {
	fs := flag.NewFlagSet("query list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	typeName := fs.String("type", "", "Filter by component type")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence for connected edges")
	sourceType := fs.String("source-type", "", "Filter by detection source: markdown, code, both")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query list: %w", err)
	}

	envelope, err := ExecuteListQuery(QueryListParams{
		TypeName:      *typeName,
		MinConfidence: *minConf,
		SourceType:    *sourceType,
		GraphName:     *graphName,
	})
	if err != nil {
		return handleQueryError(err)
	}

	return outputEnvelopeSuccess(*envelope, *format, "list")
}

// --- Mention decoration ------------------------------------------------------

// decorateWithMentions attaches provenance data to impact/dependency nodes.
// limit > 0 truncates mentions per node; limit == 0 means unlimited.
func decorateWithMentions(nodes []ImpactNode, mentions map[string][]ComponentMention, limit int) {
	for i := range nodes {
		cms, ok := mentions[nodes[i].Name]
		if !ok || len(cms) == 0 {
			continue
		}

		nodes[i].MentionCount = len(cms)

		// Apply limit.
		toMap := cms
		if limit > 0 && len(toMap) > limit {
			toMap = toMap[:limit]
		}

		details := make([]MentionDetail, len(toMap))
		for j, cm := range toMap {
			details[j] = MentionDetail{
				FilePath:        cm.FilePath,
				DetectionMethod: cm.DetectedBy,
				Confidence:      cm.Confidence,
				Context:         cm.HeadingHierarchy,
			}
		}
		nodes[i].Mentions = details
	}
}

// loadMentionsForGraph opens the graph DB and loads component mentions.
// Returns nil map on error (non-fatal; caller should continue without mentions).
func loadMentionsForGraph(graphName string) map[string][]ComponentMention {
	storageDir, err := GraphStorageDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: resolve storage dir for mentions: %v\n", err)
		return nil
	}
	if graphName == "" {
		graphName, err = getCurrentGraph(storageDir)
		if err != nil {
			return nil
		}
	}
	dbPath := filepath.Join(storageDir, graphName, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: open DB for mentions: %v\n", err)
		return nil
	}
	defer db.Close()

	mentions, err := db.LoadComponentMentions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: load mentions: %v\n", err)
		return nil
	}
	return mentions
}

// --- Reverse traversal for impact queries ------------------------------------

// executeImpactReverse performs BFS following ByTarget (incoming edges) to find
// components that depend on root. This answers "if root fails, what breaks?"
func executeImpactReverse(g *Graph, root string, maxDepth int, minConf *float64, sourceTypeFilter string) ([]ImpactNode, []EnrichedRelationship, []CycleEntry) {
	type entry struct {
		id    string
		depth int
	}

	visited := map[string]bool{root: true}
	queue := []entry{{id: root, depth: 0}}

	var nodes []ImpactNode
	var rels []EnrichedRelationship

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		// Follow incoming edges: things that depend on cur.id.
		for _, edge := range g.ByTarget[cur.id] {
			if minConf != nil && *minConf > 0 && edge.Confidence < *minConf {
				continue
			}
			if !matchesSourceTypeFilter(edge, sourceTypeFilter) {
				continue
			}
			if !visited[edge.Source] {
				visited[edge.Source] = true
				dist := cur.depth + 1

				nodeType := "unknown"
				if n, ok := g.Nodes[edge.Source]; ok && string(n.ComponentType) != "" {
					nodeType = string(n.ComponentType)
				}

				tier := safeScoreToTier(edge.Confidence)

				nodes = append(nodes, ImpactNode{
					Name:           edge.Source,
					Type:           nodeType,
					Distance:       dist,
					ConfidenceTier: string(tier),
				})

				rels = append(rels, EnrichedRelationship{
					From:             edge.Source,
					To:               edge.Target,
					Confidence:       edge.Confidence,
					ConfidenceTier:   string(tier),
					Type:             string(edge.Type),
					SourceFile:       edge.SourceFile,
					ExtractionMethod: edge.ExtractionMethod,
					SourceType:       edgeSourceType(edge),
				})

				queue = append(queue, entry{id: edge.Source, depth: dist})
			}
		}
	}

	if nodes == nil {
		nodes = []ImpactNode{}
	}
	if rels == nil {
		rels = []EnrichedRelationship{}
	}

	// Detect cycles in the full graph and filter to those involving traversed nodes.
	cycles := detectRelevantCycles(g, visited, root)

	return nodes, rels, cycles
}

// --- Forward traversal for dependencies queries ------------------------------

// executeForwardTraversal performs BFS following BySource (outgoing edges) to find
// what root depends on. This answers "what does root need to work?"
func executeForwardTraversal(g *Graph, root string, maxDepth int, minConf *float64, sourceTypeFilter string) ([]ImpactNode, []EnrichedRelationship, []CycleEntry) {
	type entry struct {
		id    string
		depth int
	}

	visited := map[string]bool{root: true}
	queue := []entry{{id: root, depth: 0}}

	var nodes []ImpactNode
	var rels []EnrichedRelationship

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		// Follow outgoing edges: things that cur.id depends on.
		for _, edge := range g.BySource[cur.id] {
			if minConf != nil && *minConf > 0 && edge.Confidence < *minConf {
				continue
			}
			if !matchesSourceTypeFilter(edge, sourceTypeFilter) {
				continue
			}
			if !visited[edge.Target] {
				visited[edge.Target] = true
				dist := cur.depth + 1

				nodeType := "unknown"
				if n, ok := g.Nodes[edge.Target]; ok && string(n.ComponentType) != "" {
					nodeType = string(n.ComponentType)
				}

				tier := safeScoreToTier(edge.Confidence)

				nodes = append(nodes, ImpactNode{
					Name:           edge.Target,
					Type:           nodeType,
					Distance:       dist,
					ConfidenceTier: string(tier),
				})

				rels = append(rels, EnrichedRelationship{
					From:             edge.Source,
					To:               edge.Target,
					Confidence:       edge.Confidence,
					ConfidenceTier:   string(tier),
					Type:             string(edge.Type),
					SourceFile:       edge.SourceFile,
					ExtractionMethod: edge.ExtractionMethod,
					SourceType:       edgeSourceType(edge),
				})

				queue = append(queue, entry{id: edge.Target, depth: dist})
			}
		}
	}

	if nodes == nil {
		nodes = []ImpactNode{}
	}
	if rels == nil {
		rels = []EnrichedRelationship{}
	}

	// Detect cycles in the full graph and filter to those involving traversed nodes.
	cycles := detectRelevantCycles(g, visited, root)

	return nodes, rels, cycles
}

// --- Cycle detection for query results ---------------------------------------

// detectRelevantCycles uses the graph's DFS-based cycle detection and filters
// results to only include cycles where at least one non-root participant was
// reached by the BFS traversal. Each cycle edge is reported as a CycleEntry.
// The root node is excluded from cycle entries to avoid false positives.
func detectRelevantCycles(g *Graph, visited map[string]bool, _ string) []CycleEntry {
	allCycles := g.DetectCycles()
	if len(allCycles) == 0 {
		return nil
	}

	var entries []CycleEntry
	seen := make(map[[2]string]bool)

	for _, cycle := range allCycles {
		// cycle is [A, B, ..., A] (first == last).
		// Include this cycle only if at least one participant was reached by BFS.
		hasVisited := false
		for _, nodeID := range cycle[:len(cycle)-1] {
			if visited[nodeID] {
				hasVisited = true
				break
			}
		}
		if !hasVisited {
			continue
		}

		// Emit each edge in the cycle.
		for i := 0; i < len(cycle)-1; i++ {
			from := cycle[i]
			to := cycle[i+1]
			if from == to {
				continue // skip degenerate self-loop from reconstruction
			}
			key := [2]string{from, to}
			if !seen[key] {
				seen[key] = true
				entries = append(entries, CycleEntry{From: from, To: to})
			}
		}
	}

	return entries
}

// --- Fuzzy component matching ------------------------------------------------

// suggestComponents returns up to 5 component name suggestions similar to the
// input query. Scoring: substring match = 2, prefix match on first 3 chars = 1.
func suggestComponents(g *Graph, query string) []string {
	type suggestion struct {
		name  string
		score int
	}

	lowerQuery := strings.ToLower(query)
	queryParts := strings.FieldsFunc(lowerQuery, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	var candidates []suggestion

	for id := range g.Nodes {
		lowerID := strings.ToLower(id)
		score := 0
		// Substring match in either direction.
		if strings.Contains(lowerID, lowerQuery) || strings.Contains(lowerQuery, lowerID) {
			score += 2
		}
		// Prefix match on first 3 chars.
		if len(lowerQuery) >= 3 && len(lowerID) >= 3 && lowerID[:3] == lowerQuery[:3] {
			score++
		}
		// Word-level overlap: check if any word from the query appears in the ID.
		idParts := strings.FieldsFunc(lowerID, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
		for _, qp := range queryParts {
			if len(qp) < 3 {
				continue
			}
			for _, ip := range idParts {
				if strings.Contains(ip, qp) || strings.Contains(qp, ip) {
					score++
				}
			}
		}
		if score > 0 {
			candidates = append(candidates, suggestion{name: id, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})

	var result []string
	for i, c := range candidates {
		if i >= 5 {
			break
		}
		result = append(result, c.name)
	}
	return result
}

// --- Helpers -----------------------------------------------------------------

// edgeSourceType returns the edge's source_type, defaulting to "markdown" for
// pre-v6 graphs where SourceType is empty.
func edgeSourceType(e *Edge) string {
	if e.SourceType == "" {
		return "markdown"
	}
	return e.SourceType
}

// matchesSourceTypeFilter returns true if the edge passes the --source-type filter.
// Empty filter matches all edges.
// "code" matches edges with source_type "code" or "both" (code was involved).
// "markdown" matches edges with source_type "markdown" or "both" (markdown was involved).
// "both" matches only edges with source_type "both" (both sources corroborated).
func matchesSourceTypeFilter(e *Edge, filter string) bool {
	if filter == "" {
		return true
	}
	st := edgeSourceType(e)
	switch filter {
	case "code":
		return st == "code" || st == "both"
	case "markdown":
		return st == "markdown" || st == "both"
	case "both":
		return st == "both"
	}
	return true
}

// validateSourceType checks that the --source-type flag value is valid.
// Returns an error string for invalid values, empty string for valid ones.
func validateSourceType(st string) string {
	if st == "" || st == "markdown" || st == "code" || st == "both" {
		return ""
	}
	return fmt.Sprintf("invalid --source-type %q: must be markdown, code, or both", st)
}

func parseDepth(s string) (int, error) {
	if s == "all" {
		return 100, nil
	}
	d, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid depth %q: must be integer or \"all\"", s)
	}
	if d < 1 {
		return 1, nil
	}
	return d, nil
}

func safeScoreToTier(score float64) ConfidenceTier {
	if score < 0.4 {
		return TierThreshold
	}
	return ScoreToTier(score)
}

func buildMetadata(g *Graph, meta *ExportMetadata, graphName string, elapsedMs int64) QueryEnvelopeMetadata {
	m := QueryEnvelopeMetadata{
		ExecutionTimeMs: elapsedMs,
		NodeCount:       len(g.Nodes),
		EdgeCount:       len(g.Edges),
		ComponentCount:  len(g.Nodes),
		GraphName:       graphName,
	}
	if meta != nil {
		m.GraphVersion = meta.Version
		m.CreatedAt = meta.CreatedAt
	}
	return m
}

func buildHopsWithFilter(g *Graph, nodePath []string, minConf float64, sourceTypeFilter string) ([]HopInfo, float64, bool) {
	var hops []HopInfo
	totalConf := 1.0

	for i := 0; i < len(nodePath)-1; i++ {
		from := nodePath[i]
		to := nodePath[i+1]

		// Find edge between consecutive nodes.
		var foundEdge *Edge
		for _, e := range g.BySource[from] {
			if e.Target == to {
				foundEdge = e
				break
			}
		}

		if foundEdge == nil {
			return nil, 0, false
		}

		if minConf > 0 && foundEdge.Confidence < minConf {
			return nil, 0, false
		}

		if !matchesSourceTypeFilter(foundEdge, sourceTypeFilter) {
			return nil, 0, false
		}

		tier := safeScoreToTier(foundEdge.Confidence)

		hops = append(hops, HopInfo{
			From:             from,
			To:               to,
			Confidence:       foundEdge.Confidence,
			ConfidenceTier:   string(tier),
			SourceFile:       foundEdge.SourceFile,
			ExtractionMethod: foundEdge.ExtractionMethod,
		})
		totalConf *= foundEdge.Confidence
	}

	return hops, totalConf, true
}

// --- Error output ------------------------------------------------------------

func writeErrorJSON(w io.Writer, message, code string, suggestions []string) {
	errObj := queryErrorJSON{
		Error:       message,
		Code:        code,
		Suggestions: suggestions,
	}
	if code == "NO_GRAPH" {
		errObj.Action = "run 'graphmd import <file.zip>' to import a graph first"
	}
	data, _ := json.MarshalIndent(errObj, "", "  ")
	fmt.Fprintln(w, string(data))
}

func writeErrorJSONStdout(message, code string, suggestions []string) error {
	writeErrorJSON(os.Stdout, message, code, suggestions)
	return fmt.Errorf("%s", message)
}

// handleQueryError converts errors from Execute* functions into CLI-appropriate
// output. QueryError instances produce structured JSON on stdout (preserving
// existing CLI behavior). Load errors are handled specially. Other errors pass through.
func handleQueryError(err error) error {
	var qe *QueryError
	if errors.As(err, &qe) {
		return writeErrorJSONStdout(qe.Message, qe.Code, qe.Suggestions)
	}
	return handleLoadError(err)
}

func handleLoadError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no graph imported") {
		writeErrorJSON(os.Stdout, msg, "NO_GRAPH", nil)
		return err
	}
	if strings.Contains(msg, "not found") {
		writeErrorJSON(os.Stdout, msg, "NOT_FOUND", nil)
		return err
	}
	return err
}

// --- Output formatting -------------------------------------------------------

// outputEnvelope writes the envelope as JSON or table and returns an error
// (non-nil triggers exit 1 for error cases, nil for success).
func outputEnvelope(envelope QueryEnvelope, format, queryType string) error {
	if format == "table" {
		writeTable(os.Stdout, envelope, queryType)
		return nil
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputEnvelopeSuccess always returns nil (for commands where empty results are not errors).
func outputEnvelopeSuccess(envelope QueryEnvelope, format, queryType string) error {
	if format == "table" {
		writeTable(os.Stdout, envelope, queryType)
		return nil
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// writeTable writes tabular output for different query types.
func writeTable(w io.Writer, env QueryEnvelope, queryType string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	switch queryType {
	case "impact", "dependencies":
		result, ok := env.Results.(ImpactResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		fmt.Fprintln(tw, "NAME\tTYPE\tDISTANCE\tCONFIDENCE\tTIER")
		for _, n := range result.AffectedNodes {
			conf := ""
			for _, r := range result.Relationships {
				if (queryType == "impact" && r.From == n.Name) || (queryType == "dependencies" && r.To == n.Name) {
					conf = fmt.Sprintf("%.2f", r.Confidence)
					break
				}
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", n.Name, n.Type, n.Distance, conf, n.ConfidenceTier)
		}
		tw.Flush()

	case "path":
		result, ok := env.Results.(PathResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		if result.Count == 0 {
			fmt.Fprintf(w, "No paths found. %s\n", result.Reason)
			return
		}
		for i, p := range result.Paths {
			fmt.Fprintf(w, "Path %d (confidence: %.4f):\n", i+1, p.TotalConfidence)
			fmt.Fprintln(tw, "  FROM\tTO\tCONFIDENCE\tTIER")
			for _, h := range p.Hops {
				fmt.Fprintf(tw, "  %s\t%s\t%.2f\t%s\n", h.From, h.To, h.Confidence, h.ConfidenceTier)
			}
			tw.Flush()
			fmt.Fprintln(w)
		}

	case "list":
		result, ok := env.Results.(ListResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		fmt.Fprintln(tw, "NAME\tTYPE\tINCOMING\tOUTGOING")
		for _, c := range result.Components {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", c.Name, c.Type, c.IncomingEdges, c.OutgoingEdges)
		}
		tw.Flush()
	}
}

// --- Usage -------------------------------------------------------------------

func printQueryUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: graphmd query <subcommand> [options]

Subcommands:
  impact          Query downstream impact of a component failure
  dependencies    Query what a component depends on (alias: deps)
  path            Find paths between two components
  list            List components with optional filters

Global flags:
  --graph <name>              Select a named graph (default: most recent import)
  --min-confidence <f>        Filter relationships below threshold
  --source-type <s>           Filter by detection source: markdown, code, both
  --format json|table         Output format (default: json)

The --min-confidence and --source-type filters compose independently:
an edge must pass both filters to appear in results.

Examples:
  graphmd query impact --component payment-api
  graphmd query impact --component primary-db --depth all
  graphmd query dependencies --component web-frontend --source-type code
  graphmd query path --from web-frontend --to primary-db
  graphmd query list --type service --min-confidence 0.7
`)
}
