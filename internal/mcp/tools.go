package mcp

import (
	"context"
	"encoding/json"
	"errors"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/graphmd/graphmd/internal/knowledge"
)

// --- Typed input structs for MCP tool arguments ---

// ImpactArgs holds the input parameters for the query_impact tool.
// Required fields use json tags without omitempty; optional fields use omitempty.
type ImpactArgs struct {
	Component     string  `json:"component" jsonschema:"the component to analyze for downstream impact"`
	Depth         int     `json:"depth,omitempty" jsonschema:"traversal depth (default 1; use 0 for unlimited)"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence threshold (0.0-1.0)"`
	SourceType    string  `json:"source_type,omitempty" jsonschema:"filter by detection source: markdown or code or both"`
	Graph         string  `json:"graph,omitempty" jsonschema:"named graph to query (default: most recent import)"`
}

// DependenciesArgs holds the input parameters for the query_dependencies tool.
type DependenciesArgs struct {
	Component     string  `json:"component" jsonschema:"the component to analyze for upstream dependencies"`
	Depth         int     `json:"depth,omitempty" jsonschema:"traversal depth (default 1; use 0 for unlimited)"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence threshold (0.0-1.0)"`
	SourceType    string  `json:"source_type,omitempty" jsonschema:"filter by detection source: markdown or code or both"`
	Graph         string  `json:"graph,omitempty" jsonschema:"named graph to query (default: most recent import)"`
}

// PathArgs holds the input parameters for the query_path tool.
type PathArgs struct {
	From          string  `json:"from" jsonschema:"source component"`
	To            string  `json:"to" jsonschema:"target component"`
	Limit         int     `json:"limit,omitempty" jsonschema:"maximum paths to return (default 10)"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence per hop (0.0-1.0)"`
	SourceType    string  `json:"source_type,omitempty" jsonschema:"filter by detection source: markdown or code or both"`
	Graph         string  `json:"graph,omitempty" jsonschema:"named graph to query (default: most recent import)"`
}

// ListArgs holds the input parameters for the list_components tool.
type ListArgs struct {
	Type          string  `json:"type,omitempty" jsonschema:"filter by component type (service, database, cache, etc.)"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence for connected edges (0.0-1.0)"`
	SourceType    string  `json:"source_type,omitempty" jsonschema:"filter by detection source: markdown or code or both"`
	Graph         string  `json:"graph,omitempty" jsonschema:"named graph to query (default: most recent import)"`
}

// GraphInfoArgs holds the input parameters for the graphmd_graph_info tool.
type GraphInfoArgs struct {
	Graph string `json:"graph,omitempty" jsonschema:"named graph to query (default: most recent import)"`
}

// registerTools registers all 5 MCP tools on the server.
func registerTools(server *mcpsdk.Server) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "query_impact",
		Description: "Analyze downstream impact of a component failure. Returns all components that directly or transitively depend on the specified component, with confidence scores. Use this to answer 'if X fails, what breaks?'",
	}, handleImpact)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "query_dependencies",
		Description: "Find what a component depends on. Returns all upstream dependencies with confidence scores. Use this to answer 'what does X need to work?'",
	}, handleDependencies)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "query_path",
		Description: "Find dependency paths between two components. Returns paths with per-hop confidence scores. Use this to answer 'how does X connect to Y?'",
	}, handlePath)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_components",
		Description: "List all components in the dependency graph with optional type and confidence filters. Use this to explore the graph before targeted queries.",
	}, handleList)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "graphmd_graph_info",
		Description: "Get metadata about the loaded dependency graph: name, version, component count, relationship count. Use this first to verify a graph is loaded and assess its scope.",
	}, handleGraphInfo)
}

// --- Tool handler functions ---

// handleImpact handles the query_impact tool call.
func handleImpact(_ context.Context, _ *mcpsdk.CallToolRequest, args ImpactArgs) (*mcpsdk.CallToolResult, any, error) {
	envelope, err := knowledge.ExecuteImpactQuery(knowledge.QueryImpactParams{
		Component:     args.Component,
		Depth:         args.Depth,
		MinConfidence: args.MinConfidence,
		SourceType:    args.SourceType,
		GraphName:     args.Graph,
	})
	if err != nil {
		return handleQueryError(err)
	}
	return marshalResult(envelope)
}

// handleDependencies handles the query_dependencies tool call.
func handleDependencies(_ context.Context, _ *mcpsdk.CallToolRequest, args DependenciesArgs) (*mcpsdk.CallToolResult, any, error) {
	envelope, err := knowledge.ExecuteDependenciesQuery(knowledge.QueryDependenciesParams{
		Component:     args.Component,
		Depth:         args.Depth,
		MinConfidence: args.MinConfidence,
		SourceType:    args.SourceType,
		GraphName:     args.Graph,
	})
	if err != nil {
		return handleQueryError(err)
	}
	return marshalResult(envelope)
}

// handlePath handles the query_path tool call.
func handlePath(_ context.Context, _ *mcpsdk.CallToolRequest, args PathArgs) (*mcpsdk.CallToolResult, any, error) {
	envelope, err := knowledge.ExecutePathQuery(knowledge.QueryPathParams{
		From:          args.From,
		To:            args.To,
		Limit:         args.Limit,
		MinConfidence: args.MinConfidence,
		SourceType:    args.SourceType,
		GraphName:     args.Graph,
	})
	if err != nil {
		return handleQueryError(err)
	}
	return marshalResult(envelope)
}

// handleList handles the list_components tool call.
func handleList(_ context.Context, _ *mcpsdk.CallToolRequest, args ListArgs) (*mcpsdk.CallToolResult, any, error) {
	envelope, err := knowledge.ExecuteListQuery(knowledge.QueryListParams{
		TypeName:      args.Type,
		MinConfidence: args.MinConfidence,
		SourceType:    args.SourceType,
		GraphName:     args.Graph,
	})
	if err != nil {
		return handleQueryError(err)
	}
	return marshalResult(envelope)
}

// handleGraphInfo handles the graphmd_graph_info tool call.
func handleGraphInfo(_ context.Context, _ *mcpsdk.CallToolRequest, args GraphInfoArgs) (*mcpsdk.CallToolResult, any, error) {
	result, err := knowledge.GetGraphInfo(knowledge.GraphInfoParams{
		GraphName: args.Graph,
	})
	if err != nil {
		return handleQueryError(err)
	}
	return marshalResult(result)
}

// --- Shared helpers ---

// queryErrorJSON is the JSON representation of a QueryError for MCP tool responses.
type queryErrorJSON struct {
	Error       string   `json:"error"`
	Code        string   `json:"code"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// handleQueryError distinguishes user errors (QueryError) from infrastructure errors.
// User errors are returned as tool results with IsError=true and structured JSON content.
// Infrastructure errors are returned as regular Go errors, which the SDK wraps appropriately.
func handleQueryError(err error) (*mcpsdk.CallToolResult, any, error) {
	var qe *knowledge.QueryError
	if errors.As(err, &qe) {
		// User error: return as tool result with IsError=true and JSON content.
		errJSON, _ := json.Marshal(queryErrorJSON{
			Error:       qe.Message,
			Code:        qe.Code,
			Suggestions: qe.Suggestions,
		})
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(errJSON)}},
			IsError: true,
		}, nil, nil
	}
	// Infrastructure error: return as Go error for SDK to handle.
	return nil, nil, err
}

// marshalResult JSON-encodes a value and returns it as MCP text content.
func marshalResult(v any) (*mcpsdk.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(data)}},
	}, nil, nil
}
