package knowledge

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestGetComponentEmbeddings_Integration tests the full embeddings flow with a real graph.
// Run with: go test -v ./internal/knowledge -run TestGetComponentEmbeddings_Integration
func TestGetComponentEmbeddings_Integration(t *testing.T) {
	// This test requires the test-graph to be imported.
	// Skip if graph is not available.
	_, _, err := LoadStoredGraph("test-graph")
	if err != nil {
		t.Skip("test-graph not found - run: semanticmesh import test-data.zip --name test-graph")
		return
	}

	// Test 1: Get embeddings for valid components
	t.Run("valid components", func(t *testing.T) {
		params := EmbeddingParams{
			Components:    []string{"payment-service/README.md", "auth-service/README.md", "api-gateway-service/architecture.md"},
			GraphName:     "test-graph",
			EmbeddingType: "description",
		}

		result, err := GetComponentEmbeddings(params)
		if err != nil {
			t.Fatalf("GetComponentEmbeddings failed: %v", err)
		}

		// Verify result structure
		if result.Metadata.Count != 3 {
			t.Errorf("Expected count=3, got %d", result.Metadata.Count)
		}

		if result.Metadata.Dimension != 384 {
			t.Errorf("Expected dimension=384, got %d", result.Metadata.Dimension)
		}

		if result.Metadata.EmbeddingMethod != "placeholder-deterministic" {
			t.Errorf("Expected method=placeholder-deterministic, got %s", result.Metadata.EmbeddingMethod)
		}

		if result.Metadata.EmbeddingType != "description" {
			t.Errorf("Expected type=description, got %s", result.Metadata.EmbeddingType)
		}

		// Verify embeddings
		if len(result.Embeddings) != 3 {
			t.Errorf("Expected 3 embeddings, got %d", len(result.Embeddings))
		}

		for _, emb := range result.Embeddings {
			if emb.Dimension != 384 {
				t.Errorf("Component %s: expected dimension=384, got %d", emb.Component, emb.Dimension)
			}

			if len(emb.Vector) != 384 {
				t.Errorf("Component %s: expected vector length=384, got %d", emb.Component, len(emb.Vector))
			}

			if emb.Method != "placeholder-deterministic" {
				t.Errorf("Component %s: expected method=placeholder-deterministic, got %s", emb.Component, emb.Method)
			}

			// Verify the vector is normalized
			magnitude := 0.0
			for _, v := range emb.Vector {
				magnitude += v * v
			}
			magnitude = 1.0 / magnitude
			if magnitude < 0.99 || magnitude > 1.01 {
				t.Errorf("Component %s: vector not normalized (magnitude=%f)", emb.Component, 1.0/magnitude)
			}
		}

		// Print a sample for manual inspection
		if len(result.Embeddings) > 0 {
			first := result.Embeddings[0]
			t.Logf("Sample embedding for %s: [%.4f, %.4f, %.4f, ...] (dim=%d)",
				first.Component,
				first.Vector[0],
				first.Vector[1],
				first.Vector[2],
				first.Dimension,
			)
		}
	})

	// Test 2: Mix of valid and invalid components
	t.Run("mixed valid and invalid", func(t *testing.T) {
		params := EmbeddingParams{
			Components:    []string{"payment-service/README.md", "nonexistent-component"},
			GraphName:     "test-graph",
			EmbeddingType: "description",
		}

		result, err := GetComponentEmbeddings(params)
		if err != nil {
			t.Fatalf("GetComponentEmbeddings failed: %v", err)
		}

		// Should return 1 embedding and 1 not found
		if result.Metadata.Count != 1 {
			t.Errorf("Expected count=1, got %d", result.Metadata.Count)
		}

		if len(result.Metadata.NotFound) != 1 {
			t.Errorf("Expected 1 not found, got %d", len(result.Metadata.NotFound))
		}

		if len(result.Metadata.NotFound) > 0 && result.Metadata.NotFound[0] != "nonexistent-component" {
			t.Errorf("Expected 'nonexistent-component' in not found, got %v", result.Metadata.NotFound)
		}
	})

	// Test 3: Empty components list
	t.Run("empty components", func(t *testing.T) {
		params := EmbeddingParams{
			Components:    []string{},
			GraphName:     "test-graph",
			EmbeddingType: "description",
		}

		_, err := GetComponentEmbeddings(params)
		if err == nil {
			t.Error("Expected error for empty components list, got nil")
		}
	})

	// Test 4: Print full JSON for manual inspection
	t.Run("print full result", func(t *testing.T) {
		params := EmbeddingParams{
			Components:    []string{"payment-service/README.md"},
			GraphName:     "test-graph",
			EmbeddingType: "description",
		}

		result, err := GetComponentEmbeddings(params)
		if err != nil {
			t.Fatalf("GetComponentEmbeddings failed: %v", err)
		}

		// Truncate vector for readable output
		if len(result.Embeddings) > 0 {
			original := result.Embeddings[0].Vector
			result.Embeddings[0].Vector = original[:10] // Show first 10 elements
			result.Embeddings[0].Vector = append(result.Embeddings[0].Vector, 999.0) // Marker for truncation
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatalf("JSON marshal failed: %v", err)
		}

		fmt.Println("\n=== Sample output (vector truncated) ===")
		fmt.Println(string(output))
	})
}
