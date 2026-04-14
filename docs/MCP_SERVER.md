# MCP Server

semanticmesh includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that exposes the dependency graph to AI agents. The server runs over stdio transport and provides six tools for querying infrastructure relationships and generating embeddings.

## Starting the Server

```bash
semanticmesh mcp
```

This starts the MCP server on stdio (stdin/stdout). The server identifies itself as `semanticmesh` version `2.0.0`. Log messages are written to stderr to keep the stdio transport clean.

The server handles `SIGTERM` and `SIGINT` for graceful shutdown.

## Prerequisites

Before querying, you must import at least one graph:

```bash
# Build and export a graph
semanticmesh export --input ./my-project --output graph.zip --analyze-code

# Import into persistent storage
semanticmesh import graph.zip --name my-project
```

The MCP tools query against imported graphs. If no graph is available, tools return a `NO_GRAPH` error.

## Tools

### query_impact

**Description:** Analyze downstream impact of a component failure. Returns all components that directly or transitively depend on the specified component, with confidence scores. Use this to answer "if X fails, what breaks?"

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `component` | string | yes | The component to analyze for downstream impact |
| `depth` | integer | no | Traversal depth (default 1; use 0 for unlimited) |
| `min_confidence` | number | no | Minimum confidence threshold (0.0-1.0) |
| `source_type` | string | no | Filter by detection source: `markdown`, `code`, or `both` |
| `graph` | string | no | Named graph to query (default: most recent import) |

### query_dependencies

**Description:** Find what a component depends on. Returns all upstream dependencies with confidence scores. Use this to answer "what does X need to work?"

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `component` | string | yes | The component to analyze for upstream dependencies |
| `depth` | integer | no | Traversal depth (default 1; use 0 for unlimited) |
| `min_confidence` | number | no | Minimum confidence threshold (0.0-1.0) |
| `source_type` | string | no | Filter by detection source: `markdown`, `code`, or `both` |
| `graph` | string | no | Named graph to query (default: most recent import) |

### query_path

**Description:** Find dependency paths between two components. Returns paths with per-hop confidence scores. Use this to answer "how does X connect to Y?"

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `from` | string | yes | Source component |
| `to` | string | yes | Target component |
| `limit` | integer | no | Maximum paths to return (default 10) |
| `min_confidence` | number | no | Minimum confidence per hop (0.0-1.0) |
| `source_type` | string | no | Filter by detection source: `markdown`, `code`, or `both` |
| `graph` | string | no | Named graph to query (default: most recent import) |

### list_components

**Description:** List all components in the dependency graph with optional type and confidence filters. Use this to explore the graph before targeted queries.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | no | Filter by component type (`service`, `database`, `cache`, etc.) |
| `min_confidence` | number | no | Minimum confidence for connected edges (0.0-1.0) |
| `source_type` | string | no | Filter by detection source: `markdown`, `code`, or `both` |
| `graph` | string | no | Named graph to query (default: most recent import) |

### semanticmesh_graph_info

**Description:** Get metadata about the loaded dependency graph: name, version, component count, relationship count. Use this first to verify a graph is loaded and assess its scope.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `graph` | string | no | Named graph to query (default: most recent import) |

### get_component_embeddings

**Description:** Fetch text embeddings for specified components. Returns 384-dimensional vectors based on component metadata (name, type, description). Use this for semantic similarity analysis, clustering, or when you need dense vector representations of components.

**Note:** Currently uses placeholder embeddings (deterministic hash-based vectors). For production use, integrate with real embedding models (OpenAI, Cohere, Voyage, etc.).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `components` | array[string] | yes | List of component names to get embeddings for |
| `graph` | string | no | Named graph to query (default: most recent import) |
| `embedding_type` | string | no | Type of embedding: `description`, `context` (default: `description`) |

**Response:**

```json
{
  "embeddings": [
    {
      "component": "payment-service/README.md",
      "vector": [-0.0543, 0.0188, 0.0658, ...],
      "dimension": 384,
      "method": "placeholder-deterministic"
    }
  ],
  "metadata": {
    "count": 1,
    "embedding_method": "placeholder-deterministic",
    "dimension": 384,
    "embedding_type": "description",
    "not_found": []
  }
}
```

**Use Cases:**
- Semantic search: "Find components similar to payment-api"
- Clustering: Group related components by vector similarity
- Context enrichment: Provide embeddings to LLMs for better understanding
- Recommendation: Suggest related components based on cosine similarity

**Integration Guide for Real Embeddings:**

To replace placeholder embeddings with real LLM embeddings:

1. Add an embedding client dependency (e.g., OpenAI SDK, Cohere SDK)
2. In `internal/knowledge/embeddings.go`, replace `generatePlaceholderEmbedding()` with API calls
3. Update the `method` field to reflect the model used (e.g., `"text-embedding-3-small"`)
4. Consider caching embeddings in the database to avoid repeated API calls
5. Add error handling for rate limits, network issues, etc.
6. Update dimension based on your chosen model (e.g., 1536 for OpenAI's large model)

Example integration with OpenAI:

```go
import "github.com/openai/openai-go"

func generateRealEmbedding(node *Node, embeddingType string) ([]float64, error) {
    client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
    
    text := fmt.Sprintf("%s: %s (%s)", node.Title, node.ID, node.ComponentType)
    
    resp, err := client.Embeddings.Create(context.Background(), &openai.EmbeddingCreateParams{
        Model: openai.EmbeddingModelTextEmbedding3Small,
        Input: text,
    })
    if err != nil {
        return nil, err
    }
    
    return resp.Data[0].Embedding, nil
}
```

## Response Format

All query tools return a JSON envelope with three top-level fields:

```json
{
  "query": {
    "type": "impact",
    "component": "payment-api",
    "depth": 2,
    "min_confidence": 0.7,
    "source_type": ""
  },
  "results": {
    "affected_nodes": [
      {
        "name": "checkout-service",
        "type": "service",
        "distance": 1,
        "confidence_tier": "high"
      }
    ],
    "relationships": [
      {
        "from": "checkout-service",
        "to": "payment-api",
        "confidence": 0.9,
        "confidence_tier": "high",
        "type": "depends_on",
        "source_file": "architecture.md",
        "extraction_method": "heading_proximity",
        "source_type": "markdown"
      }
    ]
  },
  "metadata": {
    "execution_time_ms": 3,
    "node_count": 24,
    "edge_count": 47,
    "graph_name": "prod-infra",
    "graph_version": "1.0",
    "created_at": "2026-03-28T10:00:00Z",
    "component_count": 24
  }
}
```

The `semanticmesh_graph_info` tool returns a simpler structure:

```json
{
  "name": "prod-infra",
  "version": "1.0",
  "created_at": "2026-03-28T10:00:00Z",
  "component_count": 24,
  "relationship_count": 47,
  "schema_version": 3
}
```

### Result Types by Query

| Query | Results Field | Contents |
|-------|---------------|----------|
| `query_impact` | `affected_nodes` + `relationships` | Nodes reached by reverse traversal, with enriched edges |
| `query_dependencies` | `affected_nodes` + `relationships` | Nodes reached by forward traversal, with enriched edges |
| `query_path` | `paths` + `count` | Array of paths, each with nodes, hops, and total confidence |
| `list_components` | `components` + `count` | Array of components with name, type, and edge counts |
| `get_component_embeddings` | `embeddings` + `metadata` | Array of embeddings with vectors, dimensions, and metadata |

## Error Handling

Query errors are returned as tool results with `isError: true` and a structured JSON body:

```json
{
  "error": "component \"paymnet-api\" not found",
  "code": "NOT_FOUND",
  "suggestions": ["payment-api", "payment-service"]
}
```

### Error Codes

| Code | Meaning |
|------|---------|
| `NOT_FOUND` | The specified component does not exist in the graph. Includes fuzzy-match suggestions. |
| `NO_GRAPH` | No graph has been imported yet, or the named graph does not exist. |
| `MISSING_ARG` | A required parameter was not provided (e.g., `component` for impact queries). |
| `INVALID_ARG` | A parameter has an invalid value (e.g., unrecognized `source_type`). |

Infrastructure errors (database failures, I/O errors) are returned as standard MCP errors rather than tool results.

## Integration Examples

### Claude Desktop

Add to your Claude Desktop MCP configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "semanticmesh": {
      "command": "semanticmesh",
      "args": ["mcp"]
    }
  }
}
```

### Cursor

Add to your Cursor MCP settings (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "semanticmesh": {
      "command": "semanticmesh",
      "args": ["mcp"]
    }
  }
}
```

### Generic MCP Client

Any MCP client that supports stdio transport can connect to semanticmesh:

```bash
# The server communicates via JSON-RPC 2.0 over stdin/stdout
semanticmesh mcp
```

The server responds to the standard MCP `initialize` handshake, then accepts `tools/list` and `tools/call` requests.

### Typical Agent Workflow

1. **Start with graph info** -- call `semanticmesh_graph_info` to verify a graph is loaded and see its scope.
2. **Explore components** -- call `list_components` to discover what's in the graph, optionally filtering by type.
3. **Targeted queries** -- use `query_impact`, `query_dependencies`, or `query_path` to answer specific questions about component relationships.

```
Agent: "If the payment database goes down, what services are affected?"

1. semanticmesh_graph_info() -> 24 components, 47 relationships
2. query_impact(component: "payment-db", depth: 0) -> checkout-service, payment-api, billing-service
```
