package knowledge

import (
	"context"
	"fmt"
	"strings"
)

// EdgeRefinementResult holds the results of LLM-based edge refinement.
type EdgeRefinementResult struct {
	// ValidEdges are edges that passed validation.
	ValidEdges []*Edge

	// FalsePositives are edges identified as false positives.
	FalsePositives []*Edge

	// ReplacedLowConfidence are edges that were replaced with higher confidence versions.
	ReplacedLowConfidence []*Edge

	// Stats contains refinement statistics.
	Stats EdgeRefinementStats
}

// EdgeRefinementStats tracks statistics for the edge refinement process.
type EdgeRefinementStats struct {
	TotalEdges          int
	ValidEdges          int
	FalsePositives      int
	LowConfidenceEdges  int
	ReplacedEdges       int
	EnrichedEdges       int
}

// RefineEdges uses LLM to validate and enrich discovered edges (dependencies).
//
// Pipeline:
//  1. Separate edges into high-confidence and low-confidence groups
//  2. Validate low-confidence edges with LLM
//  3. Filter false positives
//  4. Enrich edge metadata (relationship type, strength)
//  5. Return refined edges
func RefineEdges(
	ctx context.Context,
	llmClient *BedrockLLMClient,
	rawEdges []*Edge,
	validComponents map[string]bool, // Set of valid component IDs
	confidenceThreshold float64,
) (*EdgeRefinementResult, error) {
	result := &EdgeRefinementResult{
		ValidEdges:            []*Edge{},
		FalsePositives:        []*Edge{},
		ReplacedLowConfidence: []*Edge{},
		Stats:                 EdgeRefinementStats{TotalEdges: len(rawEdges)},
	}

	if len(rawEdges) == 0 {
		return result, nil
	}

	// Step 1: Separate high-confidence and low-confidence edges.
	highConfEdges := []*Edge{}
	lowConfEdges := []*Edge{}

	for _, edge := range rawEdges {
		// Skip edges referencing invalid components.
		if !validComponents[edge.Source] || !validComponents[edge.Target] {
			result.FalsePositives = append(result.FalsePositives, edge)
			result.Stats.FalsePositives++
			continue
		}

		if edge.Confidence >= confidenceThreshold {
			highConfEdges = append(highConfEdges, edge)
		} else {
			lowConfEdges = append(lowConfEdges, edge)
			result.Stats.LowConfidenceEdges++
		}
	}

	// Step 2: Accept all high-confidence edges.
	result.ValidEdges = append(result.ValidEdges, highConfEdges...)
	result.Stats.ValidEdges += len(highConfEdges)

	// Step 3: Validate low-confidence edges with LLM (in batches).
	if len(lowConfEdges) > 0 {
		batchSize := 30 // Process 30 edges per LLM call
		for start := 0; start < len(lowConfEdges); start += batchSize {
			end := start + batchSize
			if end > len(lowConfEdges) {
				end = len(lowConfEdges)
			}
			batch := lowConfEdges[start:end]

			validated, err := validateEdgeBatch(ctx, llmClient, batch)
			if err != nil {
				// On error, skip this batch (graceful degradation).
				continue
			}

			// Process batch results.
			for _, ve := range validated {
				if ve.IsValid {
					// Create enriched edge.
					enrichedEdge := &Edge{
						ID:         ve.EdgeID,
						Source:     ve.Source,
						Target:     ve.Target,
						Type:       EdgeType(ve.RelationshipType),
						Confidence: ve.Confidence,
						Evidence:   ve.Evidence,
						SourceType: "llm-validated",
					}
					result.ValidEdges = append(result.ValidEdges, enrichedEdge)
					result.Stats.ValidEdges++
					result.Stats.EnrichedEdges++

					// Check if this replaces a low-confidence edge.
					if ve.Confidence > confidenceThreshold {
						result.ReplacedLowConfidence = append(result.ReplacedLowConfidence, findEdgeByID(batch, ve.EdgeID))
						result.Stats.ReplacedEdges++
					}
				} else {
					// Mark as false positive.
					result.FalsePositives = append(result.FalsePositives, findEdgeByID(batch, ve.EdgeID))
					result.Stats.FalsePositives++
				}
			}
		}
	}

	return result, nil
}

// validateEdgeBatch validates a batch of edges with a single LLM call.
func validateEdgeBatch(ctx context.Context, llmClient *BedrockLLMClient, edges []*Edge) ([]llmEdgeValidation, error) {
	// Build prompt for batch validation.
	prompt := buildEdgeValidationPrompt(edges)

	// Call LLM.
	var response []llmEdgeValidation
	if err := llmClient.CallLLMJSON(ctx, prompt, &response); err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	return response, nil
}

// llmEdgeValidation is the expected JSON shape from the LLM.
type llmEdgeValidation struct {
	EdgeID           string  `json:"edge_id"`
	Source           string  `json:"source"`
	Target           string  `json:"target"`
	RelationshipType string  `json:"relationship_type"`
	Confidence       float64 `json:"confidence"`
	Evidence         string  `json:"evidence"`
	IsValid          bool    `json:"is_valid"`
	Reasoning        string  `json:"reasoning"`
}

// buildEdgeValidationPrompt constructs the LLM prompt for edge validation.
func buildEdgeValidationPrompt(edges []*Edge) string {
	var b strings.Builder

	b.WriteString("You are a software architecture expert. I have discovered the following component dependencies from infrastructure documentation.\n\n")
	b.WriteString("Your task:\n")
	b.WriteString("1. Validate each dependency (is it a real, meaningful relationship?)\n")
	b.WriteString("2. Identify false positives (accidental co-occurrences, generic mentions)\n")
	b.WriteString("3. Classify relationship type (depends_on, calls, references, implements, mentions, related)\n")
	b.WriteString("4. Assign confidence score (0.0-1.0)\n")
	b.WriteString("5. Provide evidence/reasoning\n\n")

	b.WriteString("Dependencies to validate:\n")
	for i, edge := range edges {
		b.WriteString(fmt.Sprintf("%d. %s -> %s (detected: %s, confidence: %.2f, evidence: %q)\n",
			i+1, edge.Source, edge.Target, edge.Type, edge.Confidence, truncate(edge.Evidence, 100)))
	}

	b.WriteString("\nRelationship types:\n")
	b.WriteString("- depends_on: Component requires the target to function\n")
	b.WriteString("- calls: Component makes API/RPC calls to the target\n")
	b.WriteString("- references: Component references target in documentation\n")
	b.WriteString("- implements: Component implements interface/protocol defined by target\n")
	b.WriteString("- mentions: Generic mention (weak relationship)\n")
	b.WriteString("- related: Related but unclear relationship\n\n")

	b.WriteString("Rules:\n")
	b.WriteString("- Set is_valid=false for false positives\n")
	b.WriteString("- Confidence: 0.9-1.0 for explicit dependencies, 0.5-0.8 for inferred, <0.5 for weak/uncertain\n")
	b.WriteString("- Provide clear reasoning\n\n")

	b.WriteString("Return JSON only (no markdown, no explanation):\n")
	b.WriteString("[\n")
	b.WriteString("  {\n")
	b.WriteString("    \"edge_id\": \"source\\x00target\\x00type\",\n")
	b.WriteString("    \"source\": \"payment-service\",\n")
	b.WriteString("    \"target\": \"primary-db\",\n")
	b.WriteString("    \"relationship_type\": \"depends_on\",\n")
	b.WriteString("    \"confidence\": 0.92,\n")
	b.WriteString("    \"evidence\": \"Payment service connects to primary-db for transaction storage\",\n")
	b.WriteString("    \"is_valid\": true,\n")
	b.WriteString("    \"reasoning\": \"Clear database dependency with explicit connection evidence\"\n")
	b.WriteString("  }\n")
	b.WriteString("]\n")

	return b.String()
}

// findEdgeByID finds an edge in a list by its ID.
func findEdgeByID(edges []*Edge, id string) *Edge {
	for _, e := range edges {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// truncate truncates a string to the given length and adds "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FilterEdgesByConfidence filters edges by confidence threshold.
func FilterEdgesByConfidence(edges []*Edge, minConfidence float64) []*Edge {
	filtered := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		if edge.Confidence >= minConfidence {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

// RemoveDuplicateEdges deduplicates edges by source+target+type.
// For duplicates, keeps the edge with the highest confidence.
func RemoveDuplicateEdges(edges []*Edge) []*Edge {
	// Build map of edge key -> best edge.
	bestEdges := make(map[string]*Edge)

	for _, edge := range edges {
		key := edge.Source + "\x00" + edge.Target + "\x00" + string(edge.Type)
		if existing, ok := bestEdges[key]; ok {
			// Keep the edge with higher confidence.
			if edge.Confidence > existing.Confidence {
				bestEdges[key] = edge
			}
		} else {
			bestEdges[key] = edge
		}
	}

	// Convert map to slice.
	result := make([]*Edge, 0, len(bestEdges))
	for _, edge := range bestEdges {
		result = append(result, edge)
	}

	return result
}

// DetectMissingEdges identifies potential missing edges based on component types.
// For example, services typically depend on databases, APIs depend on auth services, etc.
// This is a heuristic-based approach that can be validated by LLM.
func DetectMissingEdges(components []EnrichedComponent, existingEdges []*Edge) []EdgeHypothesis {
	// Build sets for quick lookup.
	serviceNames := []string{}
	dbNames := []string{}
	gatewayNames := []string{}

	for _, comp := range components {
		switch comp.Type {
		case ComponentTypeService:
			serviceNames = append(serviceNames, comp.Name)
		case ComponentTypeDatabase:
			dbNames = append(dbNames, comp.Name)
		case ComponentTypeGateway:
			gatewayNames = append(gatewayNames, comp.Name)
		}
	}

	// Build edge lookup.
	edgeSet := make(map[string]bool)
	for _, edge := range existingEdges {
		key := edge.Source + "\x00" + edge.Target
		edgeSet[key] = true
	}

	// Generate hypotheses.
	hypotheses := []EdgeHypothesis{}

	// Heuristic 1: Services typically depend on databases.
	for _, service := range serviceNames {
		for _, db := range dbNames {
			key := service + "\x00" + db
			if !edgeSet[key] {
				hypotheses = append(hypotheses, EdgeHypothesis{
					Source:     service,
					Target:     db,
					Hypothesis: fmt.Sprintf("%s might depend on %s", service, db),
					Confidence: 0.3, // Low confidence - needs validation
				})
			}
		}
	}

	// Heuristic 2: Services typically depend on API gateways.
	for _, service := range serviceNames {
		for _, gateway := range gatewayNames {
			key := service + "\x00" + gateway
			if !edgeSet[key] {
				hypotheses = append(hypotheses, EdgeHypothesis{
					Source:     service,
					Target:     gateway,
					Hypothesis: fmt.Sprintf("%s might route through %s", service, gateway),
					Confidence: 0.4,
				})
			}
		}
	}

	return hypotheses
}

// EdgeHypothesis represents a potential missing edge.
type EdgeHypothesis struct {
	Source     string
	Target     string
	Hypothesis string
	Confidence float64
}
