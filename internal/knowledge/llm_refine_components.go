package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ComponentRefinementResult holds the results of LLM-based component refinement.
type ComponentRefinementResult struct {
	// ValidComponents are components that passed validation.
	ValidComponents []EnrichedComponent

	// FalsePositives are components identified as false positives.
	FalsePositives []string

	// Aliases maps name variants to canonical names.
	Aliases map[string]string

	// Stats contains refinement statistics.
	Stats RefinementStats
}

// EnrichedComponent is a component with LLM-generated enrichments.
type EnrichedComponent struct {
	Name        string
	Type        ComponentType
	Description string
	Tags        []string
	Confidence  float64
}

// RefinementStats tracks statistics for the refinement process.
type RefinementStats struct {
	TotalComponents     int
	ValidComponents     int
	FalsePositives      int
	Duplicates          int
	EnrichedFromCache   int
	EnrichedFromLLM     int
	NormalizationsFound int
}

// RefineComponents uses LLM to clean, normalize, and enrich discovered components.
//
// Pipeline:
//  1. Load component cache
//  2. Identify uncached components
//  3. Batch process with LLM:
//     - Filter false positives
//     - Normalize names (merge duplicates)
//     - Enrich with descriptions and tags
//  4. Update cache
//  5. Return refined components
func RefineComponents(
	ctx context.Context,
	llmClient *BedrockLLMClient,
	cacheManager *LLMCacheManager,
	rawComponents []*Node,
) (*ComponentRefinementResult, error) {
	result := &ComponentRefinementResult{
		ValidComponents: []EnrichedComponent{},
		FalsePositives:  []string{},
		Aliases:         make(map[string]string),
		Stats:           RefinementStats{TotalComponents: len(rawComponents)},
	}

	if len(rawComponents) == 0 {
		return result, nil
	}

	// Step 1: Check cache for existing enrichments.
	compCache := cacheManager.GetComponentCache()
	uncachedComponents := []*Node{}

	for _, comp := range rawComponents {
		if cached, ok := compCache.Get(comp.Title); ok {
			// Use cached result.
			if cached.IsValidComponent {
				result.ValidComponents = append(result.ValidComponents, EnrichedComponent{
					Name:        cached.CanonicalName,
					Type:        ComponentType(cached.ComponentType),
					Description: cached.Description,
					Tags:        cached.Tags,
					Confidence:  cached.Confidence,
				})
				result.Stats.ValidComponents++
				result.Stats.EnrichedFromCache++
				// Track alias mapping.
				if cached.NameVariant != cached.CanonicalName {
					result.Aliases[cached.NameVariant] = cached.CanonicalName
					result.Stats.NormalizationsFound++
				}
			} else {
				result.FalsePositives = append(result.FalsePositives, comp.Title)
				result.Stats.FalsePositives++
			}
		} else {
			uncachedComponents = append(uncachedComponents, comp)
		}
	}

	// Step 2: Process uncached components with LLM (in batches).
	if len(uncachedComponents) > 0 {
		batchSize := 20 // Process 20 components per LLM call
		for start := 0; start < len(uncachedComponents); start += batchSize {
			end := start + batchSize
			if end > len(uncachedComponents) {
				end = len(uncachedComponents)
			}
			batch := uncachedComponents[start:end]

			enriched, err := refineComponentBatch(ctx, llmClient, batch)
			if err != nil {
				return nil, fmt.Errorf("refine batch: %w", err)
			}

			// Process batch results.
			for _, ec := range enriched {
				// Update result.
				if ec.IsValidComponent {
					result.ValidComponents = append(result.ValidComponents, EnrichedComponent{
						Name:        ec.CanonicalName,
						Type:        ComponentType(ec.ComponentType),
						Description: ec.Description,
						Tags:        ec.Tags,
						Confidence:  ec.Confidence,
					})
					result.Stats.ValidComponents++
					result.Stats.EnrichedFromLLM++

					// Track alias if name changed.
					if ec.NameVariant != ec.CanonicalName {
						result.Aliases[ec.NameVariant] = ec.CanonicalName
						result.Stats.NormalizationsFound++
					}
				} else {
					result.FalsePositives = append(result.FalsePositives, ec.NameVariant)
					result.Stats.FalsePositives++
				}

				// Update cache.
				compCache.Put(ec)
			}
		}
	}

	// Step 3: Detect duplicates (after normalization).
	result.Stats.Duplicates = result.Stats.TotalComponents - result.Stats.ValidComponents - result.Stats.FalsePositives

	return result, nil
}

// refineComponentBatch processes a batch of components with a single LLM call.
func refineComponentBatch(ctx context.Context, llmClient *BedrockLLMClient, components []*Node) ([]ComponentCacheEntry, error) {
	// Build prompt for batch refinement.
	prompt := buildComponentRefinementPrompt(components)

	// Call LLM.
	var response []llmComponentRefinement
	if err := llmClient.CallLLMJSON(ctx, prompt, &response); err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	// Convert LLM response to cache entries.
	entries := make([]ComponentCacheEntry, 0, len(response))
	for _, r := range response {
		entry := ComponentCacheEntry{
			NameVariant:      r.OriginalName,
			CanonicalName:    r.CanonicalName,
			ComponentType:    r.ComponentType,
			Description:      r.Description,
			Tags:             r.Tags,
			Confidence:       r.Confidence,
			IsValidComponent: r.IsValid,
			GeneratedAt:      time.Now(),
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// llmComponentRefinement is the expected JSON shape from the LLM.
type llmComponentRefinement struct {
	OriginalName  string   `json:"original_name"`
	CanonicalName string   `json:"canonical_name"`
	ComponentType string   `json:"component_type"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	Confidence    float64  `json:"confidence"`
	IsValid       bool     `json:"is_valid"`
	Reasoning     string   `json:"reasoning"`
}

// buildComponentRefinementPrompt constructs the LLM prompt for component refinement.
func buildComponentRefinementPrompt(components []*Node) string {
	var b strings.Builder

	b.WriteString("You are a software architecture expert. I have discovered the following components from infrastructure documentation.\n\n")
	b.WriteString("Your task:\n")
	b.WriteString("1. Identify false positives (generic terms like 'system', 'overview', 'architecture', 'introduction')\n")
	b.WriteString("2. Normalize component names (e.g., 'payments-service', 'paymentService', 'Payment Service' -> 'payment-service')\n")
	b.WriteString("3. Classify component type (service, database, cache, queue, storage, network, auth, monitoring, api-gateway, load-balancer, cdn, other, unknown)\n")
	b.WriteString("4. Generate a concise description (1 sentence)\n")
	b.WriteString("5. Extract relevant tags (e.g., ['authentication', 'api', 'http'])\n\n")

	b.WriteString("Components to analyze:\n")
	for i, comp := range components {
		b.WriteString(fmt.Sprintf("%d. %s (detected type: %s)\n", i+1, comp.Title, comp.ComponentType))
	}

	b.WriteString("\nRules:\n")
	b.WriteString("- Use kebab-case for canonical names (lowercase with hyphens)\n")
	b.WriteString("- Set is_valid=false for false positives\n")
	b.WriteString("- Confidence: 1.0 for explicit component names, lower for ambiguous ones\n")
	b.WriteString("- Keep descriptions under 100 characters\n")
	b.WriteString("- Limit tags to 3-5 most relevant keywords\n\n")

	b.WriteString("Return JSON only (no markdown, no explanation):\n")
	b.WriteString("[\n")
	b.WriteString("  {\n")
	b.WriteString("    \"original_name\": \"Payment Service\",\n")
	b.WriteString("    \"canonical_name\": \"payment-service\",\n")
	b.WriteString("    \"component_type\": \"service\",\n")
	b.WriteString("    \"description\": \"Handles payment processing and transaction management\",\n")
	b.WriteString("    \"tags\": [\"payment\", \"api\", \"http\"],\n")
	b.WriteString("    \"confidence\": 0.95,\n")
	b.WriteString("    \"is_valid\": true,\n")
	b.WriteString("    \"reasoning\": \"Clear service name with specific domain purpose\"\n")
	b.WriteString("  }\n")
	b.WriteString("]\n")

	return b.String()
}

// NormalizeComponentName applies basic name normalization (lowercase, kebab-case).
// This is used as a fallback when LLM is not available.
func NormalizeComponentName(name string) string {
	// Convert to lowercase.
	name = strings.ToLower(name)

	// Replace spaces and underscores with hyphens.
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove consecutive hyphens.
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim hyphens from start/end.
	name = strings.Trim(name, "-")

	return name
}

// FindDuplicateComponents identifies components with similar names.
// Returns groups of duplicate components mapped to their canonical name.
func FindDuplicateComponents(components []EnrichedComponent) map[string][]string {
	// Build a map of canonical name -> variants.
	canonicalMap := make(map[string][]string)
	for _, comp := range components {
		canonical := NormalizeComponentName(comp.Name)
		canonicalMap[canonical] = append(canonicalMap[canonical], comp.Name)
	}

	// Filter out single-entry groups (no duplicates).
	duplicates := make(map[string][]string)
	for canonical, variants := range canonicalMap {
		if len(variants) > 1 {
			duplicates[canonical] = variants
		}
	}

	return duplicates
}

// MergeComponentsByAlias merges components using the alias map.
// Returns a deduplicated list with canonical names.
func MergeComponentsByAlias(components []EnrichedComponent, aliases map[string]string) []EnrichedComponent {
	// Build a map of canonical name -> merged component.
	merged := make(map[string]EnrichedComponent)

	for _, comp := range components {
		// Resolve canonical name via alias map.
		canonical := comp.Name
		if alias, ok := aliases[comp.Name]; ok {
			canonical = alias
		}

		// Merge into existing entry or create new one.
		if existing, ok := merged[canonical]; ok {
			// Merge tags (deduplicate).
			tagSet := make(map[string]bool)
			for _, tag := range existing.Tags {
				tagSet[tag] = true
			}
			for _, tag := range comp.Tags {
				tagSet[tag] = true
			}
			merged[canonical] = EnrichedComponent{
				Name:        canonical,
				Type:        existing.Type, // Keep first type
				Description: existing.Description, // Keep first description
				Tags:        mapKeys(tagSet),
				Confidence:  max(existing.Confidence, comp.Confidence),
			}
		} else {
			merged[canonical] = EnrichedComponent{
				Name:        canonical,
				Type:        comp.Type,
				Description: comp.Description,
				Tags:        comp.Tags,
				Confidence:  comp.Confidence,
			}
		}
	}

	// Convert map to slice.
	result := make([]EnrichedComponent, 0, len(merged))
	for _, comp := range merged {
		result = append(result, comp)
	}

	return result
}

// mapKeys returns the keys of a map as a slice.
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// max returns the larger of two float64 values.
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
