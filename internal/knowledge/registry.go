package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RegistryFileName is the name of the JSON file that persists the registry.
const RegistryFileName = ".bmd-registry.json"

// SignalSource identifies the origin of a relationship signal.
type SignalSource string

const (
	// SignalLink is an explicit markdown link (highest confidence: 1.0).
	SignalLink SignalSource = "link"

	// SignalMention is a text mention without an explicit link (0.6–0.75).
	SignalMention SignalSource = "mention"

	// SignalLLM is a relationship inferred by an LLM (PageIndex, 0.65).
	SignalLLM SignalSource = "llm"

	// SignalCode is a relationship detected from source code analysis
	// (import statements, function calls, connection strings).
	SignalCode SignalSource = "code"
)

// ComponentTypeAPI is an alias for ComponentTypeGateway, preserved for
// backward compatibility with existing registry data.
const ComponentTypeAPI = ComponentTypeGateway

// ComponentTypeConfig is an alias for ComponentTypeConfigServer, preserved for
// backward compatibility with existing registry data.
const ComponentTypeConfig = ComponentTypeConfigServer

// RegistryComponent is a component tracked by the ComponentRegistry.
// It extends the existing Component detection with richer metadata.
type RegistryComponent struct {
	// ID is the canonical identifier for this component (e.g. "auth-service").
	ID string `json:"id"`

	// Name is the human-readable display name (e.g. "Auth Service").
	Name string `json:"name"`

	// FileRef is the primary documentation file for this component.
	FileRef string `json:"file_ref"`

	// Type describes the component category.
	Type ComponentType `json:"type"`

	// SourceFile is the file where this component was first detected.
	SourceFile string `json:"source_file"`

	// DetectedAt records when this component was first added to the registry.
	DetectedAt time.Time `json:"detected_at"`
}

// Signal captures a single piece of evidence for a relationship between
// two components.
type Signal struct {
	// SourceType identifies where this signal came from.
	SourceType SignalSource `json:"source_type"`

	// Confidence is a score in [0.0, 1.0] indicating how certain this signal is.
	Confidence float64 `json:"confidence"`

	// Evidence is a human-readable description of where the signal was found.
	Evidence string `json:"evidence"`

	// Weight is an optional multiplier (default 1.0) applied during aggregation.
	Weight float64 `json:"weight"`
}

// RegistryRelationship represents a directed relationship between two components
// backed by one or more evidence signals.
type RegistryRelationship struct {
	// FromComponent is the ID of the source component.
	FromComponent string `json:"from_component"`

	// ToComponent is the ID of the target component.
	ToComponent string `json:"to_component"`

	// Signals is the list of evidence signals supporting this relationship.
	Signals []Signal `json:"signals"`

	// AggregatedConfidence is the computed confidence from all signals.
	// Use AggregateConfidence() to recompute this after adding signals.
	AggregatedConfidence float64 `json:"aggregated_confidence"`
}

// ComponentRegistry is the central store for components and their relationships.
// It aggregates signals from multiple sources (links, text mentions, LLM)
// into a unified, confidence-weighted relationship graph.
//
// Zero value is NOT valid; always create via NewComponentRegistry.
type ComponentRegistry struct {
	// Components maps component ID → *RegistryComponent.
	Components map[string]*RegistryComponent `json:"components"`

	// Relationships is the list of all directed relationships.
	Relationships []RegistryRelationship `json:"relationships"`

	// Index maps "from:to" → index into Relationships for fast lookup.
	// Not serialized — rebuilt from Relationships on load.
	index map[string]int `json:"-"`
}

// NewComponentRegistry creates an empty, ready-to-use ComponentRegistry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		Components:    make(map[string]*RegistryComponent),
		Relationships: []RegistryRelationship{},
		index:         make(map[string]int),
	}
}

// AddComponent registers a component in the registry.
// If a component with the same ID already exists, it is replaced (idempotent).
// Returns an error if the component or its ID is nil/empty.
func (r *ComponentRegistry) AddComponent(comp *RegistryComponent) error {
	if comp == nil {
		return fmt.Errorf("ComponentRegistry.AddComponent: component must not be nil")
	}
	if comp.ID == "" {
		return fmt.Errorf("ComponentRegistry.AddComponent: component.ID must not be empty")
	}
	if comp.DetectedAt.IsZero() {
		comp.DetectedAt = time.Now()
	}
	r.Components[comp.ID] = comp
	return nil
}

// AddSignal appends a signal to the relationship between fromID and toID.
// If no relationship exists between the two components, one is created.
// Returns an error if from/to IDs are empty or equal.
//
// After adding signals, call AggregateConfidence (or let queries call it
// lazily) to refresh AggregatedConfidence.
func (r *ComponentRegistry) AddSignal(fromID, toID string, signal Signal) error {
	if fromID == "" {
		return fmt.Errorf("ComponentRegistry.AddSignal: fromID must not be empty")
	}
	if toID == "" {
		return fmt.Errorf("ComponentRegistry.AddSignal: toID must not be empty")
	}
	if fromID == toID {
		return fmt.Errorf("ComponentRegistry.AddSignal: self-relationship not allowed (%q)", fromID)
	}

	// Default weight to 1.0 if not set.
	if signal.Weight == 0 {
		signal.Weight = 1.0
	}

	key := fromID + ":" + toID
	if idx, ok := r.index[key]; ok {
		// Relationship already exists — deduplicate signals.
		rel := &r.Relationships[idx]
		if !r.hasDuplicateSignal(rel.Signals, signal) {
			rel.Signals = append(rel.Signals, signal)
			rel.AggregatedConfidence = computeAggregatedConfidence(rel.Signals)
		}
		return nil
	}

	// Create a new relationship.
	rel := RegistryRelationship{
		FromComponent:        fromID,
		ToComponent:          toID,
		Signals:              []Signal{signal},
		AggregatedConfidence: signal.Confidence,
	}
	r.index[key] = len(r.Relationships)
	r.Relationships = append(r.Relationships, rel)
	return nil
}

// hasDuplicateSignal returns true when signals already contains a signal
// with the same SourceType and Evidence.
func (r *ComponentRegistry) hasDuplicateSignal(signals []Signal, candidate Signal) bool {
	for _, s := range signals {
		if s.SourceType == candidate.SourceType && s.Evidence == candidate.Evidence {
			return true
		}
	}
	return false
}

// AggregateConfidence recomputes AggregatedConfidence for all relationships
// in the registry. Call this after bulk-loading signals.
func (r *ComponentRegistry) AggregateConfidence() {
	for i := range r.Relationships {
		r.Relationships[i].AggregatedConfidence = computeAggregatedConfidence(r.Relationships[i].Signals)
	}
}

// computeAggregatedConfidence applies the max(signal.Confidence * signal.Weight)
// aggregation rule: the strongest available signal wins.
func computeAggregatedConfidence(signals []Signal) float64 {
	if len(signals) == 0 {
		return 0.0
	}
	max := 0.0
	for _, s := range signals {
		w := s.Weight
		if w <= 0 {
			w = 1.0
		}
		score := s.Confidence * w
		if score > max {
			max = score
		}
	}
	// Cap at 1.0.
	if max > 1.0 {
		max = 1.0
	}
	return max
}

// GetComponent returns the component with the given ID, or nil if not found.
func (r *ComponentRegistry) GetComponent(id string) *RegistryComponent {
	return r.Components[id]
}

// FindByName searches for a component whose Name matches (case-insensitive).
// Returns nil if no match is found.
func (r *ComponentRegistry) FindByName(name string) *RegistryComponent {
	lower := strings.ToLower(name)
	for _, comp := range r.Components {
		if strings.ToLower(comp.Name) == lower {
			return comp
		}
	}
	return nil
}

// FindRelationships returns all outgoing relationships from the component
// with fromID. Returns nil (not an error) when no relationships exist.
func (r *ComponentRegistry) FindRelationships(fromID string) []RegistryRelationship {
	var result []RegistryRelationship
	for _, rel := range r.Relationships {
		if rel.FromComponent == fromID {
			result = append(result, rel)
		}
	}
	return result
}

// QueryByConfidence returns all relationships whose AggregatedConfidence is
// >= minConfidence. Results are sorted by confidence descending, then
// from/to alphabetically for stable output.
func (r *ComponentRegistry) QueryByConfidence(minConfidence float64) []RegistryRelationship {
	var result []RegistryRelationship
	for _, rel := range r.Relationships {
		if rel.AggregatedConfidence >= minConfidence {
			result = append(result, rel)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].AggregatedConfidence != result[j].AggregatedConfidence {
			return result[i].AggregatedConfidence > result[j].AggregatedConfidence
		}
		if result[i].FromComponent != result[j].FromComponent {
			return result[i].FromComponent < result[j].FromComponent
		}
		return result[i].ToComponent < result[j].ToComponent
	})
	return result
}

// ComponentCount returns the number of components in the registry.
func (r *ComponentRegistry) ComponentCount() int { return len(r.Components) }

// RelationshipCount returns the number of relationships in the registry.
func (r *ComponentRegistry) RelationshipCount() int { return len(r.Relationships) }

// InitFromGraph populates the registry from an existing Graph and document
// collection.
//
// Pipeline:
//  1. Extract components from graph nodes.
//  2. Extract link-based relationships from graph edges (SignalLink).
//  3. Extract text mention relationships from document content (SignalMention).
//  4. Extract LLM-inferred relationships from PageIndex (SignalLLM) — optional.
//  5. Aggregate all signals.
// InitFromGraphWithDir is like InitFromGraph but also loads components.yaml from
// the specified directory (if it exists) to override auto-detected components.
// When dir is empty, behaves like InitFromGraph (auto-detection only).
func (r *ComponentRegistry) InitFromGraphWithDir(g *Graph, docs []Document, dir string) {
	r.InitFromGraphWithDirAndLLM(g, docs, dir, QueryLLMConfig{})
}

// InitFromGraphWithDirAndLLM combines component detection with optional components.yaml config
// and LLM extraction. When dir is empty, skips components.yaml loading.
func (r *ComponentRegistry) InitFromGraphWithDirAndLLM(g *Graph, docs []Document, dir string, llmCfg QueryLLMConfig) {
	// Load optional components.yaml config from directory
	var cfg *ComponentConfig
	if dir != "" {
		cfgPath := filepath.Join(dir, "components.yaml")
		var cfgErr error
		cfg, cfgErr = LoadComponentConfig(cfgPath)
		if cfgErr != nil {
			// Non-fatal: proceed with auto-detection
			cfg = nil
		}
	}

	r.InitFromGraphWithLLMAndConfig(g, docs, llmCfg, cfg)
}

func (r *ComponentRegistry) InitFromGraph(g *Graph, docs []Document) {
	r.InitFromGraphWithLLM(g, docs, QueryLLMConfig{})
}

// InitFromGraphWithLLM populates the registry like InitFromGraph, but also
// runs LLM-powered extraction when llmCfg.Enabled is true.
//
// When llmCfg.Enabled is false (the zero value), this is identical to
// InitFromGraph with no LLM overhead.
func (r *ComponentRegistry) InitFromGraphWithLLM(g *Graph, docs []Document, llmCfg QueryLLMConfig) {
	r.InitFromGraphWithLLMAndConfig(g, docs, llmCfg, nil)
}

// InitFromGraphWithLLMAndConfig populates the registry with optional components.yaml config.
// When cfg is nil, uses auto-detection heuristics.
func (r *ComponentRegistry) InitFromGraphWithLLMAndConfig(g *Graph, docs []Document, llmCfg QueryLLMConfig, cfg *ComponentConfig) {
	// Build doc lookup for type inference.
	docByID := make(map[string]*Document, len(docs))
	for i := range docs {
		docByID[docs[i].ID] = &docs[i]
	}

	// Step 0: Detect components (respecting optional components.yaml config)
	var detectedComponentMap map[string]bool // nodeID -> is component
	var componentsByID map[string]*Component  // component.ID -> Component (for unified model)
	if len(docs) > 0 {
		// Create detector with optional config
		detector := NewComponentDetector()
		if cfg != nil {
			detector = NewComponentDetectorWithConfig(cfg)
		}
		components := detector.DetectComponents(g, docs)

		// Build set of detected component node IDs and map by ID for unified model.
		// In the unified model, multiple files can belong to the same component (e.g., all files in a service dir).
		detectedComponentMap = make(map[string]bool)
		componentsByID = make(map[string]*Component, len(components))
		for i, comp := range components {
			detectedComponentMap[comp.File] = true
			if _, exists := componentsByID[comp.ID]; !exists {
				componentsByID[comp.ID] = &components[i]
			}
			// Mark all files in the same service directory as belonging to this component
			// (for files like auth-service/api.md, auth-service/architecture.md, etc.)
			compDir := filepath.Dir(comp.File)
			if strings.HasSuffix(compDir, "-service") {
				// This is a monorepo service component; mark all files in the dir as detected
				for nodeID := range g.Nodes {
					if strings.HasPrefix(nodeID, compDir+string(filepath.Separator)) {
						detectedComponentMap[nodeID] = true
					}
				}
			}
		}
	}

	// Step 1: Register detected components only (if components.yaml present, only those; otherwise auto-detected)
	// Use unified component IDs from the detector when available.
	// Sort node IDs for deterministic registration order.
	nodeIDs := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	registeredComps := make(map[string]bool) // Track already-registered component IDs to avoid duplicates
	for _, id := range nodeIDs {
		node := g.Nodes[id]
		// When components.yaml is present (cfg != nil), only register detected components
		// When absent (cfg == nil), for backward compat, register all nodes
		if cfg != nil && !detectedComponentMap[id] {
			continue // Skip non-detected components when config is present
		}

		// Get the unified component ID from the detector results if available.
		compID := nodeToRegistryID(node.ID)
		if componentsByID != nil && len(componentsByID) > 0 {
			// Try to find this node in one of the detected components
			// Sort detected IDs for deterministic lookup order.
			detectedIDs := make([]string, 0, len(componentsByID))
			for detectedID := range componentsByID {
				detectedIDs = append(detectedIDs, detectedID)
			}
			sort.Strings(detectedIDs)

			for _, detectedID := range detectedIDs {
				detComp := componentsByID[detectedID]
				if detComp.File == id || strings.HasPrefix(id, detectedID+string(filepath.Separator)) {
					compID = detectedID
					break
				}
			}
		}

		// Skip if we already registered this component ID (avoid duplicates)
		if registeredComps[compID] {
			continue
		}
		registeredComps[compID] = true

		compType := inferComponentType(node.ID)
		comp := &RegistryComponent{
			ID:         compID,
			Name:       node.Title,
			FileRef:    id,
			Type:       compType,
			SourceFile: id,
			DetectedAt: time.Now(),
		}
		_ = r.AddComponent(comp)
	}

	// Step 2: Convert graph edges to relationships with link-confidence signals.
	// Sort edges by (source, target, confidence) for deterministic relationship ordering.
	edges := make([]*Edge, 0, len(g.Edges))
	for _, edge := range g.Edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Confidence > edges[j].Confidence // Higher confidence first
	})

	for _, edge := range edges {
		fromCompID := nodeToRegistryID(edge.Source)
		toCompID := nodeToRegistryID(edge.Target)

		signal := Signal{
			SourceType: SignalLink,
			Confidence: edge.Confidence,
			Evidence:   edge.Evidence,
			Weight:     1.0,
		}
		_ = r.AddSignal(fromCompID, toCompID, signal)
	}

	// Step 3: Extract text mentions using the component detector results.
	if len(docs) > 0 && len(detectedComponentMap) > 0 {
		// Create detector with optional config
		detector := NewComponentDetector()
		if cfg != nil {
			detector = NewComponentDetectorWithConfig(cfg)
		}
		components := detector.DetectComponents(g, docs)
		if len(components) > 0 {
			mentions := ExtractMentionsFromDocuments(docs, components)
			r.BuildFromMentions(mentions)

			// Step 4 (optional): LLM extraction — try but don't fail.
			if llmCfg.Enabled && len(docs) > 0 {
				llmRels, _ := RunLLMExtraction(llmCfg, docs, components)
				r.BuildFromLLMExtraction(llmRels)
			}

			return
		}
	}

	// Step 4 (optional): LLM extraction when no component detector ran.
	// Build component list from registered components for filtering.
	if llmCfg.Enabled && len(docs) > 0 {
		components := registryComponentsToComponents(r)
		if len(components) > 0 {
			llmRels, _ := RunLLMExtraction(llmCfg, docs, components)
			r.BuildFromLLMExtraction(llmRels)
		}
	}

	// Step 5: Aggregate all signals (when no mention extraction ran).
	r.AggregateConfidence()
}

// rebuildIndex reconstructs the index map from Relationships.
// Call this after loading from JSON since the index field is not serialized.
func (r *ComponentRegistry) rebuildIndex() {
	r.index = make(map[string]int, len(r.Relationships))
	for i, rel := range r.Relationships {
		key := rel.FromComponent + ":" + rel.ToComponent
		r.index[key] = i
	}
}

// nodeToRegistryID converts a graph node ID (relative file path) to a
// registry component ID. Uses the filename stem in kebab-case.
func nodeToRegistryID(nodeID string) string {
	base := filepath.Base(nodeID)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return strings.ToLower(stem)
}

// inferComponentType infers a ComponentType from the node file path.
func inferComponentType(nodeID string) ComponentType {
	ct, _ := InferComponentType(nodeID)
	return ct
}

// ToJSON serializes the registry to JSON bytes.
func (r *ComponentRegistry) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FromJSON deserializes the registry from JSON bytes.
// Rebuilds the internal index after loading.
func (r *ComponentRegistry) FromJSON(data []byte) error {
	if err := json.Unmarshal(data, r); err != nil {
		return fmt.Errorf("ComponentRegistry.FromJSON: %w", err)
	}
	if r.Components == nil {
		r.Components = make(map[string]*RegistryComponent)
	}
	if r.Relationships == nil {
		r.Relationships = []RegistryRelationship{}
	}
	r.rebuildIndex()
	return nil
}

// SaveRegistry writes the registry to a JSON file at path.
func SaveRegistry(r *ComponentRegistry, path string) error {
	data, err := r.ToJSON()
	if err != nil {
		return fmt.Errorf("SaveRegistry: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Clean(path), data, 0o600); err != nil {
		return fmt.Errorf("SaveRegistry: write %q: %w", path, err)
	}
	return nil
}

// LoadRegistry reads a ComponentRegistry from a JSON file at path.
// Returns nil, nil when the file does not exist (graceful fallback).
func LoadRegistry(path string) (*ComponentRegistry, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("LoadRegistry: read %q: %w", path, err)
	}
	r := NewComponentRegistry()
	if err := r.FromJSON(data); err != nil {
		return nil, err
	}
	return r, nil
}

// BuildFromMentions converts a slice of Mention values into Signal entries
// and adds them to the registry.
//
// Each mention becomes a SignalMention signal with the mention's confidence.
// After loading all mentions, AggregateConfidence is called to refresh scores.
func (r *ComponentRegistry) BuildFromMentions(mentions []Mention) {
	for _, m := range mentions {
		_ = r.AddSignal(m.FromFile, m.ToComponent, Signal{
			SourceType: SignalMention,
			Confidence: m.Confidence,
			Evidence:   m.ExampleEvidence,
			Weight:     1.0,
		})
	}
	r.AggregateConfidence()
}

// GetMentionsFor returns all relationships that target the given component
// and have at least one mention-type signal.
//
// Results are sorted by AggregatedConfidence descending.
func (r *ComponentRegistry) GetMentionsFor(componentID string) []RegistryRelationship {
	var result []RegistryRelationship
	for _, rel := range r.Relationships {
		if rel.ToComponent != componentID {
			continue
		}
		for _, sig := range rel.Signals {
			if sig.SourceType == SignalMention {
				result = append(result, rel)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].AggregatedConfidence != result[j].AggregatedConfidence {
			return result[i].AggregatedConfidence > result[j].AggregatedConfidence
		}
		return result[i].FromComponent < result[j].FromComponent
	})
	return result
}

// BuildFromLLMExtraction converts a slice of LLMRelationship values into
// SignalLLM entries and adds them to the registry.
//
// Each LLM relationship becomes a signal with source type SignalLLM.
// After loading all relationships, AggregateConfidence is called.
func (r *ComponentRegistry) BuildFromLLMExtraction(relationships []LLMRelationship) {
	for _, rel := range relationships {
		_ = r.AddSignal(rel.FromFile, rel.ToComponent, Signal{
			SourceType: SignalLLM,
			Confidence: rel.Confidence,
			Evidence:   rel.Evidence,
			Weight:     1.0,
		})
	}
	r.AggregateConfidence()
}

// GetLLMRelationships returns all relationships that target the given component
// and have at least one LLM-type signal.
//
// Results are sorted by AggregatedConfidence descending.
func (r *ComponentRegistry) GetLLMRelationships(componentID string) []RegistryRelationship {
	var result []RegistryRelationship
	for _, rel := range r.Relationships {
		if rel.ToComponent != componentID {
			continue
		}
		for _, sig := range rel.Signals {
			if sig.SourceType == SignalLLM {
				result = append(result, rel)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].AggregatedConfidence != result[j].AggregatedConfidence {
			return result[i].AggregatedConfidence > result[j].AggregatedConfidence
		}
		return result[i].FromComponent < result[j].FromComponent
	})
	return result
}

// registryComponentsToComponents converts the registry's component map into
// the []Component slice expected by LLM extraction filters.
func registryComponentsToComponents(r *ComponentRegistry) []Component {
	result := make([]Component, 0, len(r.Components))
	for _, rc := range r.Components {
		result = append(result, Component{
			ID:   rc.ID,
			Name: rc.Name,
			File: rc.FileRef,
		})
	}
	return result
}
