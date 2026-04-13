package knowledge

import (
	"math"
	"testing"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// TestConvertCodeSignalsToDiscovered_Basic verifies that 3 signals with
// different targets produce 3 DiscoveredEdges.
func TestConvertCodeSignalsToDiscovered_Basic(t *testing.T) {
	signals := []code.CodeSignal{
		{SourceFile: "main.go", LineNumber: 10, TargetComponent: "redis", TargetType: "cache", DetectionKind: "cache_client", Evidence: "redis.NewClient()", Language: "go", Confidence: 0.8},
		{SourceFile: "main.go", LineNumber: 20, TargetComponent: "postgres", TargetType: "database", DetectionKind: "db_connection", Evidence: "sql.Open(postgres)", Language: "go", Confidence: 0.9},
		{SourceFile: "main.go", LineNumber: 30, TargetComponent: "auth-service", TargetType: "service", DetectionKind: "http_call", Evidence: "http.Get(auth-service)", Language: "go", Confidence: 0.7},
	}

	edges := convertCodeSignalsToDiscovered(signals, "my-service")
	if len(edges) != 3 {
		t.Fatalf("got %d edges, want 3", len(edges))
	}

	// Check all edges have correct source and extraction method.
	for _, de := range edges {
		if de.Source != "my-service" {
			t.Errorf("edge source = %q, want %q", de.Source, "my-service")
		}
		if de.ExtractionMethod != "code-analysis" {
			t.Errorf("edge ExtractionMethod = %q, want %q", de.ExtractionMethod, "code-analysis")
		}
		if de.SourceType != "code" {
			t.Errorf("edge SourceType = %q, want %q", de.SourceType, "code")
		}
		if de.Type != EdgeDependsOn {
			t.Errorf("edge Type = %q, want %q", de.Type, EdgeDependsOn)
		}
		if len(de.Signals) == 0 {
			t.Error("edge has no signals")
		}
		for _, sig := range de.Signals {
			if sig.SourceType != SignalCode {
				t.Errorf("signal SourceType = %q, want %q", sig.SourceType, SignalCode)
			}
		}
	}
}

// TestConvertCodeSignalsToDiscovered_Dedup verifies that 2 signals with the
// same source+target+type produce 1 edge with highest confidence.
func TestConvertCodeSignalsToDiscovered_Dedup(t *testing.T) {
	signals := []code.CodeSignal{
		{SourceFile: "main.go", LineNumber: 10, TargetComponent: "redis", TargetType: "cache", DetectionKind: "cache_client", Evidence: "redis.NewClient()", Language: "go", Confidence: 0.6},
		{SourceFile: "main.go", LineNumber: 20, TargetComponent: "redis", TargetType: "cache", DetectionKind: "cache_client", Evidence: "redis.Set()", Language: "go", Confidence: 0.9},
	}

	edges := convertCodeSignalsToDiscovered(signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}

	de := edges[0]
	if de.Confidence != 0.9 {
		t.Errorf("confidence = %.2f, want 0.90", de.Confidence)
	}
	if len(de.Signals) != 2 {
		t.Errorf("got %d signals, want 2", len(de.Signals))
	}
}

// TestConvertCodeSignalsToDiscovered_SkipSelfLoop verifies that signals where
// source == target are skipped.
func TestConvertCodeSignalsToDiscovered_SkipSelfLoop(t *testing.T) {
	signals := []code.CodeSignal{
		{SourceFile: "main.go", LineNumber: 10, TargetComponent: "my-service", DetectionKind: "http_call", Evidence: "self-call", Language: "go", Confidence: 0.8},
		{SourceFile: "main.go", LineNumber: 20, TargetComponent: "redis", DetectionKind: "cache_client", Evidence: "redis.Get()", Language: "go", Confidence: 0.7},
	}

	edges := convertCodeSignalsToDiscovered(signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (self-loop should be skipped)", len(edges))
	}
	if edges[0].Target != "redis" {
		t.Errorf("target = %q, want %q", edges[0].Target, "redis")
	}
}

// TestConvertCodeSignalsToDiscovered_EmptyTarget verifies that signals with
// empty target are skipped.
func TestConvertCodeSignalsToDiscovered_EmptyTarget(t *testing.T) {
	signals := []code.CodeSignal{
		{SourceFile: "main.go", LineNumber: 10, TargetComponent: "", DetectionKind: "http_call", Evidence: "unknown", Language: "go", Confidence: 0.8},
		{SourceFile: "main.go", LineNumber: 20, TargetComponent: "redis", DetectionKind: "cache_client", Evidence: "redis.Get()", Language: "go", Confidence: 0.7},
	}

	edges := convertCodeSignalsToDiscovered(signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (empty target should be skipped)", len(edges))
	}
}

// TestApplyMultiSourceConfidence_BothSources verifies probabilistic OR:
// markdown 0.6 + code 0.6 -> ~0.84.
func TestApplyMultiSourceConfidence_BothSources(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.6, "test")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalLink, Confidence: 0.6, Evidence: "markdown link", Weight: 1.0},
			{SourceType: SignalCode, Confidence: 0.6, Evidence: "code import", Weight: 0.85},
		},
	}

	applyMultiSourceConfidence([]*DiscoveredEdge{de})

	expected := 1.0 - (1.0-0.6)*(1.0-0.6) // = 0.84
	if math.Abs(de.Confidence-expected) > 0.001 {
		t.Errorf("confidence = %.4f, want ~%.4f", de.Confidence, expected)
	}
	if math.Abs(de.Edge.Confidence-expected) > 0.001 {
		t.Errorf("edge.Confidence = %.4f, want ~%.4f", de.Edge.Confidence, expected)
	}
}

// TestApplyMultiSourceConfidence_SingleSource verifies that code-only edge
// stays at original confidence.
func TestApplyMultiSourceConfidence_SingleSource(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.7, "code only")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalCode, Confidence: 0.7, Evidence: "import stmt", Weight: 0.85},
		},
	}

	applyMultiSourceConfidence([]*DiscoveredEdge{de})

	if de.Confidence != 0.7 {
		t.Errorf("confidence = %.4f, want 0.7000 (unchanged)", de.Confidence)
	}
}

// TestApplyMultiSourceConfidence_ClampTo1 verifies that merged confidence
// does not exceed 1.0.
func TestApplyMultiSourceConfidence_ClampTo1(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.99, "high")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalLink, Confidence: 0.99, Evidence: "strong link", Weight: 1.0},
			{SourceType: SignalCode, Confidence: 0.99, Evidence: "strong code", Weight: 0.85},
		},
	}

	applyMultiSourceConfidence([]*DiscoveredEdge{de})

	if de.Confidence > 1.0 {
		t.Errorf("confidence = %.4f, exceeds 1.0", de.Confidence)
	}
	// 1 - (1-0.99)*(1-0.99) = 1 - 0.01*0.01 = 0.9999
	expected := 0.9999
	if math.Abs(de.Confidence-expected) > 0.001 {
		t.Errorf("confidence = %.4f, want ~%.4f", de.Confidence, expected)
	}
}

// TestDetermineSourceType_Markdown verifies that signals with only non-code
// sources return "markdown".
func TestDetermineSourceType_Markdown(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.7, "test")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalLink, Confidence: 0.7},
			{SourceType: SignalMention, Confidence: 0.5},
		},
	}
	got := determineSourceType(de)
	if got != "markdown" {
		t.Errorf("determineSourceType = %q, want %q", got, "markdown")
	}
}

// TestDetermineSourceType_Code verifies that signals with only SignalCode
// return "code".
func TestDetermineSourceType_Code(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.7, "test")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalCode, Confidence: 0.7},
		},
	}
	got := determineSourceType(de)
	if got != "code" {
		t.Errorf("determineSourceType = %q, want %q", got, "code")
	}
}

// TestDetermineSourceType_Both verifies that mixed signals return "both".
func TestDetermineSourceType_Both(t *testing.T) {
	edge, _ := NewEdge("a", "b", EdgeDependsOn, 0.7, "test")
	de := &DiscoveredEdge{
		Edge: edge,
		Signals: []Signal{
			{SourceType: SignalLink, Confidence: 0.7},
			{SourceType: SignalCode, Confidence: 0.8},
		},
	}
	got := determineSourceType(de)
	if got != "both" {
		t.Errorf("determineSourceType = %q, want %q", got, "both")
	}
}

// TestEnsureCodeTargetNodes_CreatesStubs verifies that targets not in the
// graph get stub nodes.
func TestEnsureCodeTargetNodes_CreatesStubs(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "my-service", Type: "document", Title: "My Service"})

	edge, _ := NewEdge("my-service", "redis", EdgeDependsOn, 0.8, "redis call")
	codeEdges := []*DiscoveredEdge{
		{Edge: edge, Signals: []Signal{{SourceType: SignalCode}}},
	}

	ensureCodeTargetNodes(g, codeEdges, nil)

	if _, ok := g.Nodes["redis"]; !ok {
		t.Fatal("expected stub node for 'redis' to be created")
	}
	node := g.Nodes["redis"]
	if node.Type != "infrastructure" {
		t.Errorf("stub node type = %q, want %q", node.Type, "infrastructure")
	}
}

// TestIsNoiseTarget verifies that common English words, bare domains, and
// short tokens are rejected, while valid component names are accepted.
func TestIsNoiseTarget(t *testing.T) {
	noisy := []string{
		"the", "here", "made", "non", "pagination",
		"kubernetes.io", "swagger.io", "docs.example.com", "paketo.io",
		"it", "an", "ab", // short tokens
		"configuration", "implementation",
	}
	for _, name := range noisy {
		if !isNoiseTarget(name) {
			t.Errorf("isNoiseTarget(%q) = false, want true", name)
		}
	}

	valid := []string{
		"redis", "postgres", "kafka", "auth-service",
		"app-manifest-manager", "KUBERNETES_SERVICE_HOST",
	}
	for _, name := range valid {
		if isNoiseTarget(name) {
			t.Errorf("isNoiseTarget(%q) = true, want false", name)
		}
	}
}

// TestEnsureCodeTargetNodes_FiltersNoise verifies that noise targets do not
// get stub nodes created in the graph.
func TestEnsureCodeTargetNodes_FiltersNoise(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "my-service", Type: "document", Title: "My Service"})

	noisy := []string{"the", "here", "kubernetes.io", "pagination"}
	var codeEdges []*DiscoveredEdge
	for _, target := range noisy {
		edge, _ := NewEdge("my-service", target, EdgeDependsOn, 0.5, "noisy")
		codeEdges = append(codeEdges, &DiscoveredEdge{Edge: edge, Signals: []Signal{{SourceType: SignalCode}}})
	}

	ensureCodeTargetNodes(g, codeEdges, nil)

	for _, target := range noisy {
		if _, ok := g.Nodes[target]; ok {
			t.Errorf("noise target %q should not have a node in the graph", target)
		}
	}
}

// TestEnsureCodeTargetNodes_InfersType verifies that valid code-detected
// targets get proper component type classification via InferComponentType.
func TestEnsureCodeTargetNodes_InfersType(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "my-service", Type: "document", Title: "My Service"})

	edge, _ := NewEdge("my-service", "redis", EdgeDependsOn, 0.8, "redis call")
	codeEdges := []*DiscoveredEdge{
		{Edge: edge, Signals: []Signal{{SourceType: SignalCode}}},
	}

	ensureCodeTargetNodes(g, codeEdges, nil)

	node, ok := g.Nodes["redis"]
	if !ok {
		t.Fatal("expected stub node for 'redis'")
	}
	if node.ComponentType == "" || node.ComponentType == ComponentTypeUnknown {
		t.Errorf("expected inferred component type for redis, got %q", node.ComponentType)
	}
}

// TestEnsureCodeTargetNodes_SkipsExisting verifies that existing nodes are
// not overwritten.
func TestEnsureCodeTargetNodes_SkipsExisting(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "my-service", Type: "document", Title: "My Service"})
	_ = g.AddNode(&Node{ID: "redis", Type: "document", Title: "Redis Cache", ComponentType: ComponentTypeCache})

	edge, _ := NewEdge("my-service", "redis", EdgeDependsOn, 0.8, "redis call")
	codeEdges := []*DiscoveredEdge{
		{Edge: edge, Signals: []Signal{{SourceType: SignalCode}}},
	}

	ensureCodeTargetNodes(g, codeEdges, nil)

	node := g.Nodes["redis"]
	if node.Title != "Redis Cache" {
		t.Errorf("existing node title changed to %q, want %q", node.Title, "Redis Cache")
	}
	if node.ComponentType != ComponentTypeCache {
		t.Errorf("existing node ComponentType changed to %q, want %q", node.ComponentType, ComponentTypeCache)
	}
}
