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

// ─── Cross-reference tests ──────────────────────────────────────────────────

// TestCrossRef_SoleTypeMatch verifies that when there's exactly one TF resource
// of a matching type, a cross-reference edge is created at confidence 0.60.
func TestCrossRef_SoleTypeMatch(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "my-service", Type: "document"})
	_ = g.AddNode(&Node{ID: "prod-postgres.internal", Type: "infrastructure", ComponentType: ComponentTypeDatabase})
	_ = g.AddNode(&Node{ID: "aws_db_instance.prod_db", Type: "infrastructure", ComponentType: ComponentTypeDatabase})

	signals := []code.CodeSignal{
		{TargetComponent: "aws_db_instance.prod_db", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "prod-postgres.internal", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	de := edges[0]
	if de.Source != "prod-postgres.internal" {
		t.Errorf("source = %q, want %q", de.Source, "prod-postgres.internal")
	}
	if de.Target != "aws_db_instance.prod_db" {
		t.Errorf("target = %q, want %q", de.Target, "aws_db_instance.prod_db")
	}
	if de.ExtractionMethod != "terraform-crossref" {
		t.Errorf("ExtractionMethod = %q, want %q", de.ExtractionMethod, "terraform-crossref")
	}
	// Sole type match with name similarity < 0.3 → confidence 0.60.
	// But "prod" token overlaps → name similarity kicks in first.
	// tokenize("prod-postgres.internal") → ["prod", "postgres"]
	// tokenize("aws_db_instance.prod_db") → ["db", "prod"]
	// overlap: ["prod"] → 1/2 = 0.50 → confidence 0.70
	if de.Confidence != 0.70 {
		t.Errorf("confidence = %.2f, want 0.70", de.Confidence)
	}
}

// TestCrossRef_SoleTypeMatch_NoNameOverlap verifies sole type match fallback
// when names have no token overlap.
func TestCrossRef_SoleTypeMatch_NoNameOverlap(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_sqs_queue.notifications", TargetType: "queue", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "alert-broker", TargetType: "queue", Language: "go", DetectionKind: "queue_client", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	// "alert-broker" → ["alert", "broker"]
	// "aws_sqs_queue.notifications" → ["sqs", "queue", "notifications"]
	// No overlap → falls through to sole type match → 0.60
	if edges[0].Confidence != 0.60 {
		t.Errorf("confidence = %.2f, want 0.60 (sole type match)", edges[0].Confidence)
	}
}

// TestCrossRef_AmbiguousType verifies that when multiple TF resources share
// a type and names don't match, no cross-reference edge is created.
func TestCrossRef_AmbiguousType(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_db_instance.users_db", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "aws_db_instance.orders_db", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "my-database", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 0 {
		t.Fatalf("got %d edges, want 0 (ambiguous — two TF databases, no name match)", len(edges))
	}
}

// TestCrossRef_NameSimilarityMatch verifies that token overlap above threshold
// produces a cross-reference edge.
func TestCrossRef_NameSimilarityMatch(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_elasticache_cluster.session_cache", TargetType: "cache", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "aws_elasticache_cluster.rate_limit_cache", TargetType: "cache", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "session-redis", TargetType: "cache", Language: "go", DetectionKind: "cache_client", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	// "session-redis" → ["session", "redis"]
	// "aws_elasticache_cluster.session_cache" → ["elasticache", "session", "cache"]
	// overlap: ["session"] → 1/2 = 0.50 → confidence 0.70
	if edges[0].Target != "aws_elasticache_cluster.session_cache" {
		t.Errorf("target = %q, want session_cache (matched by name)", edges[0].Target)
	}
	if edges[0].Confidence != 0.70 {
		t.Errorf("confidence = %.2f, want 0.70", edges[0].Confidence)
	}
}

// TestCrossRef_NameSimilarityBelowThreshold verifies no match when token
// overlap is too low and there are multiple candidates.
func TestCrossRef_NameSimilarityBelowThreshold(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_db_instance.analytics", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "aws_db_instance.reporting", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "prod-postgres.internal", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 0 {
		t.Fatalf("got %d edges, want 0 (names don't overlap enough)", len(edges))
	}
}

// TestCrossRef_TFvarsHostnameMatch verifies that when a code target appears
// literally in .tfvars, it produces an edge at confidence 0.80.
func TestCrossRef_TFvarsHostnameMatch(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		// TF resource
		{TargetComponent: "aws_db_instance.prod_db", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		// TFvars hostname that matches the code target exactly
		{TargetComponent: "prod-postgres.internal", TargetType: "database", Language: "terraform", DetectionKind: "env_var_ref", Confidence: 0.65},
		// Code signal with same hostname
		{TargetComponent: "prod-postgres.internal", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].Confidence != 0.80 {
		t.Errorf("confidence = %.2f, want 0.80 (tfvars hostname match)", edges[0].Confidence)
	}
	if edges[0].Target != "aws_db_instance.prod_db" {
		t.Errorf("target = %q, want aws_db_instance.prod_db", edges[0].Target)
	}
}

// TestCrossRef_NoTFSignals verifies no panic and no edges when only code
// signals are present.
func TestCrossRef_NoTFSignals(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "redis", TargetType: "cache", Language: "go", DetectionKind: "cache_client", Confidence: 0.80},
		{TargetComponent: "postgres", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 0 {
		t.Fatalf("got %d edges, want 0 (no TF signals)", len(edges))
	}
}

// TestCrossRef_NoCodeSignals verifies no panic and no edges when only
// Terraform signals are present.
func TestCrossRef_NoCodeSignals(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_db_instance.prod_db", TargetType: "database", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 0 {
		t.Fatalf("got %d edges, want 0 (no code signals)", len(edges))
	}
}

// TestCrossRef_TypeMismatch verifies that a code database signal never
// matches a TF queue resource.
func TestCrossRef_TypeMismatch(t *testing.T) {
	g := NewGraph()
	signals := []code.CodeSignal{
		{TargetComponent: "aws_sqs_queue.notifications", TargetType: "queue", Language: "terraform", DetectionKind: "terraform_resource", Confidence: 0.85},
		{TargetComponent: "prod-postgres", TargetType: "database", Language: "go", DetectionKind: "db_connection", Confidence: 0.80},
	}

	edges := crossReferenceTerraformSignals(g, nil, signals, "my-service")
	if len(edges) != 0 {
		t.Fatalf("got %d edges, want 0 (type mismatch: database vs queue)", len(edges))
	}
}

// ─── Tokenizer tests ────────────────────────────────────────────────────────

func TestTokenizeName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{"terraform resource", "aws_db_instance.prod_db", []string{"db", "prod"}},
		{"hostname", "prod-postgres.internal", []string{"prod", "postgres"}},
		{"simple", "redis", []string{"redis"}},
		{"path", "modules/networking/vpc", []string{"modules", "networking", "vpc"}},
		{"noise only", "aws_instance.main", nil},
		{"dedup", "prod.prod.prod", []string{"prod"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizeName(tt.input)
			if tt.expect == nil {
				if len(got) != 0 {
					t.Errorf("tokenizeName(%q) = %v, want empty", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.expect) {
				t.Fatalf("tokenizeName(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.expect, len(tt.expect))
			}
			for i, tok := range got {
				if tok != tt.expect[i] {
					t.Errorf("tokenizeName(%q)[%d] = %q, want %q", tt.input, i, tok, tt.expect[i])
				}
			}
		})
	}
}

func TestTokenOverlapScore(t *testing.T) {
	tests := []struct {
		name   string
		a, b   []string
		expect float64
	}{
		{"empty a", nil, []string{"x"}, 0},
		{"empty b", []string{"x"}, nil, 0},
		{"both empty", nil, nil, 0},
		{"identical", []string{"prod", "db"}, []string{"prod", "db"}, 1.0},
		{"partial", []string{"prod", "postgres"}, []string{"db", "prod"}, 0.5},
		{"no overlap", []string{"foo", "bar"}, []string{"baz", "qux"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenOverlapScore(tt.a, tt.b)
			if math.Abs(got-tt.expect) > 0.001 {
				t.Errorf("tokenOverlapScore(%v, %v) = %.4f, want %.4f", tt.a, tt.b, got, tt.expect)
			}
		})
	}
}
