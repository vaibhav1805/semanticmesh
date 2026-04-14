package knowledge

import (
	"testing"
)

func TestGetComponentEmbeddings(t *testing.T) {
	// Create a simple test graph
	g := NewGraph()
	g.AddNode(&Node{
		ID:            "test-service",
		Title:         "Test Service",
		Type:          "document",
		ComponentType: ComponentTypeService,
	})
	g.AddNode(&Node{
		ID:            "test-db",
		Title:         "Test Database",
		Type:          "document",
		ComponentType: ComponentTypeDatabase,
	})

	// Note: We can't easily test GetComponentEmbeddings without a real graph store,
	// but we can test the placeholder embedding generation directly.
	node := &Node{
		ID:            "payment-api",
		Title:         "Payment API",
		Type:          "document",
		ComponentType: ComponentTypeService,
	}

	vector := generatePlaceholderEmbedding(node, "description")

	// Verify properties
	if len(vector) != 384 {
		t.Errorf("Expected dimension 384, got %d", len(vector))
	}

	// Verify it's normalized (L2 norm should be ~1.0)
	magnitude := 0.0
	for _, v := range vector {
		magnitude += v * v
	}
	magnitude = 1.0 / magnitude // Should be close to 1.0 if normalized

	if magnitude < 0.99 || magnitude > 1.01 {
		t.Errorf("Vector not normalized: magnitude = %f", 1.0/magnitude)
	}

	// Verify determinism: same input should produce same output
	vector2 := generatePlaceholderEmbedding(node, "description")
	for i := range vector {
		if vector[i] != vector2[i] {
			t.Errorf("Not deterministic: vectors differ at index %d", i)
			break
		}
	}

	// Verify different nodes produce different embeddings
	node2 := &Node{
		ID:            "auth-service",
		Title:         "Auth Service",
		Type:          "document",
		ComponentType: ComponentTypeService,
	}
	vector3 := generatePlaceholderEmbedding(node2, "description")

	same := true
	for i := range vector {
		if vector[i] != vector3[i] {
			same = false
			break
		}
	}
	if same {
		t.Errorf("Different nodes produced identical embeddings")
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		a       []float64
		b       []float64
		want    float64
		wantErr bool
	}{
		{
			name: "identical vectors",
			a:    []float64{1.0, 0.0, 0.0},
			b:    []float64{1.0, 0.0, 0.0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float64{1.0, 0.0, 0.0},
			b:    []float64{0.0, 1.0, 0.0},
			want: 0.0,
		},
		{
			name: "opposite vectors",
			a:    []float64{1.0, 0.0, 0.0},
			b:    []float64{-1.0, 0.0, 0.0},
			want: -1.0,
		},
		{
			name:    "different dimensions",
			a:       []float64{1.0, 0.0},
			b:       []float64{1.0, 0.0, 0.0},
			wantErr: true,
		},
		{
			name:    "empty vectors",
			a:       []float64{},
			b:       []float64{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CosineSimilarity(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("CosineSimilarity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Allow small floating point error
				if got < tt.want-0.001 || got > tt.want+0.001 {
					t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFindSimilarComponents(t *testing.T) {
	// This test requires a real graph store, so we'll skip it for now.
	// When the graph store is available, we can add proper integration tests.
	t.Skip("Requires graph store integration")
}
