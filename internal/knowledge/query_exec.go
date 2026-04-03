package knowledge

import (
	"fmt"
	"sort"
	"time"
)

// --- QueryError type for structured error returns ----------------------------

// QueryError represents a user-facing error that can be distinguished from
// infrastructure errors. The Code field enables programmatic classification
// (e.g., MISSING_ARG, NOT_FOUND, INVALID_ARG) for MCP and CLI layers.
type QueryError struct {
	Message     string
	Code        string
	Suggestions []string
}

// Error implements the error interface.
func (e *QueryError) Error() string {
	return e.Message
}

// --- Param structs -----------------------------------------------------------

// QueryImpactParams holds parameters for an impact query.
type QueryImpactParams struct {
	Component         string
	Depth             int
	MinConfidence     float64
	SourceType        string
	GraphName         string
	IncludeProvenance bool
	MaxMentions       int
}

// QueryDependenciesParams holds parameters for a dependencies query.
type QueryDependenciesParams struct {
	Component         string
	Depth             int
	MinConfidence     float64
	SourceType        string
	GraphName         string
	IncludeProvenance bool
	MaxMentions       int
}

// QueryPathParams holds parameters for a path query.
type QueryPathParams struct {
	From          string
	To            string
	Limit         int
	MinConfidence float64
	SourceType    string
	GraphName     string
}

// QueryListParams holds parameters for a list query.
type QueryListParams struct {
	TypeName      string
	MinConfidence float64
	SourceType    string
	GraphName     string
}

// GraphInfoParams holds parameters for the graph info query.
type GraphInfoParams struct {
	GraphName string
}

// --- GraphInfoResult ---------------------------------------------------------

// GraphInfoResult contains high-level graph metadata.
type GraphInfoResult struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	CreatedAt         string `json:"created_at"`
	ComponentCount    int    `json:"component_count"`
	RelationshipCount int    `json:"relationship_count"`
	SchemaVersion     int    `json:"schema_version"`
}

// --- Exported query execution functions --------------------------------------

// ExecuteImpactQuery runs an impact (reverse traversal) query and returns the
// result envelope. It performs no stdout writes and no flag parsing.
func ExecuteImpactQuery(params QueryImpactParams) (*QueryEnvelope, error) {
	if params.Component == "" {
		return nil, &QueryError{Message: "--component is required", Code: "MISSING_ARG"}
	}

	if msg := validateSourceType(params.SourceType); msg != "" {
		return nil, &QueryError{Message: msg, Code: "INVALID_ARG"}
	}

	depth := params.Depth
	if depth <= 0 {
		depth = 1
	}

	g, meta, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	if _, ok := g.Nodes[params.Component]; !ok {
		suggestions := suggestComponents(g, params.Component)
		return nil, &QueryError{
			Message:     fmt.Sprintf("component %q not found", params.Component),
			Code:        "NOT_FOUND",
			Suggestions: suggestions,
		}
	}

	// Impact = reverse traversal: follow ByTarget to find things that depend on this component.
	var minConf *float64
	if params.MinConfidence > 0 {
		minConf = &params.MinConfidence
	}
	affectedNodes, relationships, cycles := executeImpactReverse(g, params.Component, depth, minConf, params.SourceType)

	// Decorate with provenance if requested.
	if params.IncludeProvenance {
		mentions := loadMentionsForGraph(params.GraphName)
		if mentions != nil {
			decorateWithMentions(affectedNodes, mentions, params.MaxMentions)
		}
	}

	elapsed := time.Since(start).Milliseconds()

	result := ImpactResult{
		AffectedNodes: affectedNodes,
		Relationships: relationships,
	}

	var confParam float64
	if params.MinConfidence > 0 {
		confParam = params.MinConfidence
	}

	md := buildMetadata(g, meta, params.GraphName, elapsed)
	md.CyclesDetected = cycles

	envelope := &QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "impact",
			Component:     params.Component,
			Depth:         depth,
			MinConfidence: confParam,
			SourceType:    params.SourceType,
		},
		Results:  result,
		Metadata: md,
	}

	return envelope, nil
}

// ExecuteDependenciesQuery runs a dependencies (forward traversal) query and
// returns the result envelope. It performs no stdout writes and no flag parsing.
func ExecuteDependenciesQuery(params QueryDependenciesParams) (*QueryEnvelope, error) {
	if params.Component == "" {
		return nil, &QueryError{Message: "--component is required", Code: "MISSING_ARG"}
	}

	if msg := validateSourceType(params.SourceType); msg != "" {
		return nil, &QueryError{Message: msg, Code: "INVALID_ARG"}
	}

	depth := params.Depth
	if depth <= 0 {
		depth = 1
	}

	g, meta, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	if _, ok := g.Nodes[params.Component]; !ok {
		suggestions := suggestComponents(g, params.Component)
		return nil, &QueryError{
			Message:     fmt.Sprintf("component %q not found", params.Component),
			Code:        "NOT_FOUND",
			Suggestions: suggestions,
		}
	}

	// Dependencies = forward traversal: follow BySource to find what this component depends on.
	var minConf *float64
	if params.MinConfidence > 0 {
		minConf = &params.MinConfidence
	}
	affectedNodes, relationships, cycles := executeForwardTraversal(g, params.Component, depth, minConf, params.SourceType)

	// Decorate with provenance if requested.
	if params.IncludeProvenance {
		mentions := loadMentionsForGraph(params.GraphName)
		if mentions != nil {
			decorateWithMentions(affectedNodes, mentions, params.MaxMentions)
		}
	}

	elapsed := time.Since(start).Milliseconds()

	result := ImpactResult{
		AffectedNodes: affectedNodes,
		Relationships: relationships,
	}

	var confParam float64
	if params.MinConfidence > 0 {
		confParam = params.MinConfidence
	}

	md := buildMetadata(g, meta, params.GraphName, elapsed)
	md.CyclesDetected = cycles

	envelope := &QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "dependencies",
			Component:     params.Component,
			Depth:         depth,
			MinConfidence: confParam,
			SourceType:    params.SourceType,
		},
		Results:  result,
		Metadata: md,
	}

	return envelope, nil
}

// ExecutePathQuery runs a path query between two components and returns the
// result envelope. Empty paths are not an error. It performs no stdout writes.
func ExecutePathQuery(params QueryPathParams) (*QueryEnvelope, error) {
	if params.From == "" || params.To == "" {
		return nil, &QueryError{Message: "--from and --to are required", Code: "MISSING_ARG"}
	}

	if msg := validateSourceType(params.SourceType); msg != "" {
		return nil, &QueryError{Message: msg, Code: "INVALID_ARG"}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	g, meta, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Validate both components exist.
	if _, ok := g.Nodes[params.From]; !ok {
		suggestions := suggestComponents(g, params.From)
		return nil, &QueryError{
			Message:     fmt.Sprintf("component %q not found", params.From),
			Code:        "NOT_FOUND",
			Suggestions: suggestions,
		}
	}
	if _, ok := g.Nodes[params.To]; !ok {
		suggestions := suggestComponents(g, params.To)
		return nil, &QueryError{
			Message:     fmt.Sprintf("component %q not found", params.To),
			Code:        "NOT_FOUND",
			Suggestions: suggestions,
		}
	}

	// Find paths.
	rawPaths := g.FindPaths(params.From, params.To, 20)

	var pathInfos []PathInfo
	for _, nodePath := range rawPaths {
		hops, totalConf, valid := buildHopsWithFilter(g, nodePath, params.MinConfidence, params.SourceType)
		if !valid {
			continue
		}
		pathInfos = append(pathInfos, PathInfo{
			Nodes:           nodePath,
			Hops:            hops,
			TotalConfidence: totalConf,
		})
	}

	// Sort by total confidence descending.
	sort.Slice(pathInfos, func(i, j int) bool {
		return pathInfos[i].TotalConfidence > pathInfos[j].TotalConfidence
	})

	// Apply limit.
	if len(pathInfos) > limit {
		pathInfos = pathInfos[:limit]
	}

	result := PathResult{
		Paths: pathInfos,
		Count: len(pathInfos),
	}
	if result.Paths == nil {
		result.Paths = []PathInfo{}
	}
	if len(pathInfos) == 0 {
		result.Reason = fmt.Sprintf("no path found between %s and %s", params.From, params.To)
	}

	elapsed := time.Since(start).Milliseconds()

	var confParam float64
	if params.MinConfidence > 0 {
		confParam = params.MinConfidence
	}

	envelope := &QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "path",
			From:          params.From,
			To:            params.To,
			MinConfidence: confParam,
			SourceType:    params.SourceType,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, params.GraphName, elapsed),
	}

	return envelope, nil
}

// ExecuteListQuery runs a list query with optional type, confidence, and
// source-type filters. It performs no stdout writes.
func ExecuteListQuery(params QueryListParams) (*QueryEnvelope, error) {
	if msg := validateSourceType(params.SourceType); msg != "" {
		return nil, &QueryError{Message: msg, Code: "INVALID_ARG"}
	}

	g, meta, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	var components []ListComponent
	for id, node := range g.Nodes {
		// Type filter.
		if params.TypeName != "" && string(node.ComponentType) != params.TypeName {
			continue
		}

		incoming := len(g.ByTarget[id])
		outgoing := len(g.BySource[id])

		// Min-confidence filter: only include nodes where at least one connected edge meets threshold.
		if params.MinConfidence > 0 {
			hasQualifyingEdge := false
			for _, e := range g.BySource[id] {
				if e.Confidence >= params.MinConfidence {
					hasQualifyingEdge = true
					break
				}
			}
			if !hasQualifyingEdge {
				for _, e := range g.ByTarget[id] {
					if e.Confidence >= params.MinConfidence {
						hasQualifyingEdge = true
						break
					}
				}
			}
			if !hasQualifyingEdge {
				continue
			}
		}

		// Source-type filter: only include nodes where at least one connected edge matches.
		if params.SourceType != "" {
			hasMatchingEdge := false
			for _, e := range g.BySource[id] {
				if matchesSourceTypeFilter(e, params.SourceType) {
					hasMatchingEdge = true
					break
				}
			}
			if !hasMatchingEdge {
				for _, e := range g.ByTarget[id] {
					if matchesSourceTypeFilter(e, params.SourceType) {
						hasMatchingEdge = true
						break
					}
				}
			}
			if !hasMatchingEdge {
				continue
			}
		}

		nodeType := string(node.ComponentType)
		if nodeType == "" {
			nodeType = "unknown"
		}

		components = append(components, ListComponent{
			Name:          id,
			Type:          nodeType,
			IncomingEdges: incoming,
			OutgoingEdges: outgoing,
		})
	}

	// Sort by name for deterministic output.
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	result := ListResult{
		Components: components,
		Count:      len(components),
	}
	if result.Components == nil {
		result.Components = []ListComponent{}
	}

	elapsed := time.Since(start).Milliseconds()

	var confParam float64
	if params.MinConfidence > 0 {
		confParam = params.MinConfidence
	}

	envelope := &QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "list",
			MinConfidence: confParam,
			SourceType:    params.SourceType,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, params.GraphName, elapsed),
	}

	return envelope, nil
}

// GetGraphInfo returns high-level metadata about a stored graph without
// performing any query traversal. It performs no stdout writes.
func GetGraphInfo(params GraphInfoParams) (*GraphInfoResult, error) {
	g, meta, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	result := &GraphInfoResult{
		Name:              params.GraphName,
		ComponentCount:    len(g.Nodes),
		RelationshipCount: len(g.Edges),
	}

	if meta != nil {
		result.Version = meta.Version
		result.CreatedAt = meta.CreatedAt
		result.SchemaVersion = meta.SchemaVersion
	}

	return result, nil
}
