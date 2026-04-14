# Component Embeddings Implementation

**Status:** ✅ Complete  
**Date:** 2026-04-13  
**Feature:** On-demand text embeddings for components via MCP tool

## Overview

This document describes the implementation of the `get_component_embeddings` MCP tool, which provides vector embeddings for infrastructure components. The current implementation uses placeholder embeddings (deterministic, hash-based) that are suitable for development and testing, with a clear path to integrate real LLM embeddings for production use.

## Motivation

Including embeddings in default query responses would add ~756 KB of data for 126 components, making responses unnecessarily large. By implementing embeddings as a separate on-demand tool, we:

1. Keep default query responses small and fast
2. Allow agents to fetch embeddings only when needed (e.g., for semantic similarity)
3. Provide a clear structure for future LLM embedding integration
4. Enable new use cases: semantic search, clustering, recommendation

## Implementation Details

### New Files

#### `internal/knowledge/embeddings.go`
Core embedding logic including:

- `EmbeddingParams` - Input parameters for embedding queries
- `ComponentEmbedding` - Single component embedding with vector and metadata
- `EmbeddingResult` - Response structure with embeddings and metadata
- `GetComponentEmbeddings()` - Main function to fetch embeddings for components
- `generatePlaceholderEmbedding()` - Deterministic hash-based embedding generation (384-dim)
- `CosineSimilarity()` - Utility for computing similarity between embeddings
- `FindSimilarComponents()` - Placeholder for future semantic search capabilities

#### `internal/knowledge/embeddings_test.go`
Unit tests covering:

- Embedding generation (determinism, normalization, dimensionality)
- Cosine similarity computation (identical, orthogonal, opposite vectors)
- Error handling (empty vectors, dimension mismatch)

#### `internal/knowledge/embeddings_integration_test.go`
Integration tests with real graph data:

- Valid component embeddings
- Mixed valid/invalid components (with not_found tracking)
- Empty component list error handling
- Full JSON output verification

### Modified Files

#### `internal/mcp/tools.go`
- Added `GetEmbeddingsArgs` struct for input parameters
- Added `handleGetEmbeddings()` handler function
- Registered `get_component_embeddings` tool in `registerTools()`
- Tool description emphasizes placeholder nature and integration path

#### `docs/MCP_SERVER.md`
- Updated tool count (5 → 6)
- Added comprehensive documentation for `get_component_embeddings`
- Included integration guide for real embeddings
- Provided OpenAI SDK example code
- Added use cases and response format examples

#### `README.md`
- Updated MCP server tool count
- Added note about embeddings feature with link to docs

## Placeholder Embedding Algorithm

The current implementation generates deterministic 384-dimensional vectors using:

1. **Text representation:** Concatenate `name|type|component_type|title|embedding_type`
2. **Hash seed:** SHA-256 hash of the text
3. **Vector generation:** Use hash bytes to seed deterministic random values
4. **Normalization:** Scale to unit length (L2 norm = 1)

**Properties:**
- ✅ Deterministic: same component always produces same embedding
- ✅ Unique: different components produce different embeddings (hash collision unlikely)
- ✅ Standard dimension: 384d matches many real embedding models
- ✅ Normalized: unit vectors like real models
- ❌ No semantic meaning: vectors don't capture actual similarity

## Integration Path for Real Embeddings

To replace placeholder embeddings with real LLM embeddings:

### Step 1: Choose an Embedding Provider

Options:
- **OpenAI:** `text-embedding-3-small` (1536d, $0.02/1M tokens)
- **Cohere:** `embed-english-v3.0` (1024d, $0.10/1M tokens)
- **Voyage AI:** `voyage-02` (1024d, $0.12/1M tokens)
- **Azure OpenAI:** Same models, different pricing
- **Self-hosted:** Sentence Transformers, all-MiniLM-L6-v2 (384d, free)

### Step 2: Add SDK Dependency

```bash
# For OpenAI
go get github.com/openai/openai-go

# For Cohere
go get github.com/cohere-ai/cohere-go/v2
```

### Step 3: Replace `generatePlaceholderEmbedding()`

See `docs/MCP_SERVER.md` for full example. Key changes:

```go
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

### Step 4: Add Caching

To avoid repeated API calls:

1. Add `embeddings` table to SQLite schema with columns: `component_name`, `embedding_type`, `model`, `vector_blob`, `created_at`
2. Check cache before calling API
3. Store new embeddings after generation
4. Invalidate cache when component metadata changes

### Step 5: Update Metadata

- Change `method` field from `"placeholder-deterministic"` to actual model name
- Update `dimension` to match chosen model
- Add rate limit handling and retry logic

## API Contract

### Request

```json
{
  "components": ["payment-service/README.md", "auth-service/README.md"],
  "graph": "test-graph",
  "embedding_type": "description"
}
```

### Response

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

## Use Cases

1. **Semantic Search:** Find components similar to a query component
2. **Clustering:** Group related components by vector similarity
3. **Context Enrichment:** Provide embeddings to LLMs for better understanding
4. **Recommendation:** Suggest related components based on cosine similarity
5. **Anomaly Detection:** Identify components that don't fit expected patterns

## Testing

All tests pass:

```bash
# Unit tests
go test ./internal/knowledge -run TestGetComponentEmbeddings
go test ./internal/knowledge -run TestCosineSimilarity

# Integration tests (requires imported graph)
go test ./internal/knowledge -run TestGetComponentEmbeddings_Integration
```

**Test Coverage:**
- Embedding generation and normalization
- Determinism verification
- Cosine similarity computation
- Error handling (missing components, empty lists)
- Full end-to-end integration with real graph data

## Performance Considerations

### Current (Placeholder)
- **Latency:** ~1ms per component (deterministic computation)
- **Cost:** $0 (no API calls)
- **Scalability:** Unlimited (CPU-bound only)

### With Real Embeddings
- **Latency:** ~50-200ms per API call (network + model inference)
- **Cost:** $0.02-$0.12 per 1M tokens (provider-dependent)
- **Scalability:** Subject to rate limits (batch API calls, implement caching)

**Optimization Strategies:**
1. Batch API calls (most providers support 100+ texts per request)
2. Cache embeddings in database (invalidate on component update)
3. Pre-compute embeddings during graph import
4. Use smaller models for development, larger for production

## Future Enhancements

1. **Hybrid Search:** Combine embeddings with graph structure for better results
2. **Fine-tuning:** Train custom embeddings on infrastructure-specific language
3. **Multi-modal:** Include code snippets, diagrams, architecture decisions
4. **Temporal Embeddings:** Track how component semantics change over time
5. **Cross-graph Similarity:** Compare components across different projects

## Deployment Checklist

Before using embeddings in production:

- [ ] Choose embedding provider and model
- [ ] Add SDK dependency and configure API key
- [ ] Implement caching layer (SQLite table)
- [ ] Replace `generatePlaceholderEmbedding()` with API calls
- [ ] Update tests to use real embeddings (or mock API)
- [ ] Add rate limit handling and retry logic
- [ ] Monitor costs and latency
- [ ] Document model choice and rationale

## References

- [MCP Server Documentation](docs/MCP_SERVER.md#get_component_embeddings)
- [OpenAI Embeddings API](https://platform.openai.com/docs/guides/embeddings)
- [Cohere Embed API](https://docs.cohere.com/docs/embeddings)
- [Sentence Transformers](https://www.sbert.net/) (self-hosted option)

---

**Implemented by:** Claude Sonnet 4.5  
**Reviewed by:** (pending)  
**Status:** Ready for integration with real embeddings
