package knowledge

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
)

// EmbeddingParams holds parameters for an embeddings query.
type EmbeddingParams struct {
	Components    []string
	GraphName     string
	EmbeddingType string
}

// ComponentEmbedding represents a single component's embedding vector with metadata.
type ComponentEmbedding struct {
	Component string    `json:"component"`
	Vector    []float64 `json:"vector"`
	Dimension int       `json:"dimension"`
	Method    string    `json:"method"`
}

// EmbeddingResult is the results payload for embedding queries.
type EmbeddingResult struct {
	Embeddings []ComponentEmbedding `json:"embeddings"`
	Metadata   EmbeddingMetadata    `json:"metadata"`
}

// EmbeddingMetadata contains metadata about the embedding generation.
type EmbeddingMetadata struct {
	Count            int    `json:"count"`
	EmbeddingMethod  string `json:"embedding_method"`
	Dimension        int    `json:"dimension"`
	EmbeddingType    string `json:"embedding_type"`
	NotFound         []string `json:"not_found,omitempty"`
}

// GetComponentEmbeddings generates embeddings for the specified components.
// This is a placeholder implementation that generates deterministic vectors
// based on component properties until real LLM embeddings are integrated.
//
// To integrate real embeddings (e.g., OpenAI, Cohere, Voyage):
// 1. Add an embedding client dependency (e.g., OpenAI SDK)
// 2. Replace generatePlaceholderEmbedding() with a call to the embedding API
// 3. Update the "method" field to reflect the model used (e.g., "text-embedding-3-small")
// 4. Consider caching embeddings in the database to avoid repeated API calls
// 5. Add error handling for rate limits, network issues, etc.
func GetComponentEmbeddings(params EmbeddingParams) (*EmbeddingResult, error) {
	if len(params.Components) == 0 {
		return nil, &QueryError{
			Message: "components list cannot be empty",
			Code:    "MISSING_ARG",
		}
	}

	// Load the graph to get component metadata.
	g, _, err := LoadStoredGraph(params.GraphName)
	if err != nil {
		return nil, err
	}

	embeddingType := params.EmbeddingType
	if embeddingType == "" {
		embeddingType = "description"
	}

	var embeddings []ComponentEmbedding
	var notFound []string

	for _, componentName := range params.Components {
		node, ok := g.Nodes[componentName]
		if !ok {
			notFound = append(notFound, componentName)
			continue
		}

		// Generate placeholder embedding based on component properties.
		vector := generatePlaceholderEmbedding(node, embeddingType)

		embeddings = append(embeddings, ComponentEmbedding{
			Component: componentName,
			Vector:    vector,
			Dimension: len(vector),
			Method:    "placeholder-deterministic",
		})
	}

	result := &EmbeddingResult{
		Embeddings: embeddings,
		Metadata: EmbeddingMetadata{
			Count:           len(embeddings),
			EmbeddingMethod: "placeholder-deterministic",
			Dimension:       384, // Standard dimension for many embedding models
			EmbeddingType:   embeddingType,
			NotFound:        notFound,
		},
	}

	return result, nil
}

// generatePlaceholderEmbedding creates a deterministic 384-dimensional vector
// based on component properties. This is a placeholder until real embeddings
// are integrated.
//
// The algorithm:
// 1. Concatenates component name, type, and title into a text representation
// 2. Hashes the text using SHA-256
// 3. Uses the hash bytes to seed a deterministic random number generator
// 4. Generates a 384-dimensional vector with values in [-1, 1]
// 5. Normalizes the vector to unit length (L2 norm = 1)
//
// This ensures:
// - Same component always produces the same embedding (deterministic)
// - Different components produce different embeddings (hash collision unlikely)
// - Embeddings have the same dimensionality as real models (384d)
// - Embeddings are normalized like real models (unit vectors)
func generatePlaceholderEmbedding(node *Node, embeddingType string) []float64 {
	const dimension = 384

	// Create a text representation of the component for hashing.
	// Include type and title to make embeddings more representative.
	text := fmt.Sprintf("%s|%s|%s|%s", node.ID, node.Type, node.ComponentType, node.Title)
	if embeddingType != "" {
		text = embeddingType + "|" + text
	}

	// Hash the text to get a deterministic seed.
	hash := sha256.Sum256([]byte(text))

	// Use the hash to generate a deterministic vector.
	// We'll use groups of 8 bytes from the hash as uint64 seeds.
	vector := make([]float64, dimension)

	// Initialize with hash-derived values.
	for i := 0; i < dimension; i++ {
		// Use different sections of the hash for different dimensions
		// by XORing with the dimension index.
		hashIdx := (i * 8) % len(hash)
		seed := binary.BigEndian.Uint64(hash[hashIdx:]) ^ uint64(i)

		// Convert to float in range [-1, 1]
		// Using a deterministic transformation of the seed
		vector[i] = float64(int64(seed)) / float64(math.MaxInt64)
	}

	// Normalize to unit length (L2 norm = 1)
	// This matches real embedding models which return unit vectors.
	magnitude := 0.0
	for _, v := range vector {
		magnitude += v * v
	}
	magnitude = math.Sqrt(magnitude)

	if magnitude > 0 {
		for i := range vector {
			vector[i] /= magnitude
		}
	}

	return vector
}

// CosineSimilarity computes the cosine similarity between two embedding vectors.
// Returns a value in [-1, 1] where 1 means identical, 0 means orthogonal, and
// -1 means opposite.
//
// This is a utility function for future use when implementing semantic search
// or similarity-based queries.
func CosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same dimension (got %d and %d)", len(a), len(b))
	}

	if len(a) == 0 {
		return 0, fmt.Errorf("vectors must not be empty")
	}

	dotProduct := 0.0
	magnitudeA := 0.0
	magnitudeB := 0.0

	for i := range a {
		dotProduct += a[i] * b[i]
		magnitudeA += a[i] * a[i]
		magnitudeB += b[i] * b[i]
	}

	magnitudeA = math.Sqrt(magnitudeA)
	magnitudeB = math.Sqrt(magnitudeB)

	if magnitudeA == 0 || magnitudeB == 0 {
		return 0, fmt.Errorf("zero-magnitude vectors have undefined cosine similarity")
	}

	return dotProduct / (magnitudeA * magnitudeB), nil
}

// FindSimilarComponents finds components with embeddings similar to the query
// component, sorted by cosine similarity (descending).
//
// This is a placeholder for future semantic search capabilities. When real
// embeddings are integrated, this can power:
// - "Find components similar to X"
// - "What's related to Y but not explicitly connected?"
// - "Cluster components by semantic similarity"
func FindSimilarComponents(graphName string, queryComponent string, topK int) ([]SimilarComponent, error) {
	if topK <= 0 {
		topK = 10
	}

	g, _, err := LoadStoredGraph(graphName)
	if err != nil {
		return nil, err
	}

	queryNode, ok := g.Nodes[queryComponent]
	if !ok {
		return nil, &QueryError{
			Message: fmt.Sprintf("component %q not found", queryComponent),
			Code:    "NOT_FOUND",
			Suggestions: suggestComponents(g, queryComponent),
		}
	}

	queryEmbedding := generatePlaceholderEmbedding(queryNode, "description")

	var results []SimilarComponent
	for id, node := range g.Nodes {
		if id == queryComponent {
			continue // Skip self
		}

		nodeEmbedding := generatePlaceholderEmbedding(node, "description")
		similarity, err := CosineSimilarity(queryEmbedding, nodeEmbedding)
		if err != nil {
			continue // Skip if similarity computation fails
		}

		results = append(results, SimilarComponent{
			Name:       id,
			Type:       string(node.ComponentType),
			Similarity: similarity,
		})
	}

	// Sort by similarity descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// SimilarComponent represents a component with its similarity score to a query component.
type SimilarComponent struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Similarity float64 `json:"similarity"`
}
