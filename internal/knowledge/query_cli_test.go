package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupQueryTestGraph creates a test graph with 5 components and 6 edges,
// imports it via ZIP into XDG storage, and returns the XDG dir for cleanup.
//
// Graph structure (A -> B means A depends on B):
//
//	payment-api  --> primary-db    (0.95, explicit-link)
//	payment-api  --> redis-cache   (0.85, co-occurrence)
//	auth-service --> primary-db    (0.90, structural)
//	auth-service --> payment-api   (0.70, semantic)
//	web-frontend --> auth-service  (0.80, explicit-link)
//	web-frontend --> payment-api   (0.75, co-occurrence)
func setupQueryTestGraph(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	g := NewGraph()
	_ = g.AddNode(&Node{ID: "payment-api", Title: "Payment API", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "primary-db", Title: "Primary DB", Type: "document", ComponentType: ComponentTypeDatabase})
	_ = g.AddNode(&Node{ID: "redis-cache", Title: "Redis Cache", Type: "document", ComponentType: ComponentTypeCache})
	_ = g.AddNode(&Node{ID: "auth-service", Title: "Auth Service", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "web-frontend", Title: "Web Frontend", Type: "document", ComponentType: ComponentTypeService})

	edges := []*Edge{
		{ID: "payment-api->primary-db", Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.95, SourceFile: "payment-api.md", ExtractionMethod: "explicit-link"},
		{ID: "payment-api->redis-cache", Source: "payment-api", Target: "redis-cache", Type: EdgeDependsOn, Confidence: 0.85, SourceFile: "payment-api.md", ExtractionMethod: "co-occurrence"},
		{ID: "auth-service->primary-db", Source: "auth-service", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.90, SourceFile: "auth-service.md", ExtractionMethod: "structural"},
		{ID: "auth-service->payment-api", Source: "auth-service", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.70, SourceFile: "auth-service.md", ExtractionMethod: "semantic"},
		{ID: "web-frontend->auth-service", Source: "web-frontend", Target: "auth-service", Type: EdgeDependsOn, Confidence: 0.80, SourceFile: "web-frontend.md", ExtractionMethod: "explicit-link"},
		{ID: "web-frontend->payment-api", Source: "web-frontend", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.75, SourceFile: "web-frontend.md", ExtractionMethod: "co-occurrence"},
	}
	for _, e := range edges {
		_ = g.AddEdge(e)
	}

	// Save to SQLite.
	dbPath := filepath.Join(buildDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	db.Close()

	// Create metadata.
	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-03-23T00:00:00Z",
		ComponentCount:    5,
		RelationshipCount: 6,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	// Package as ZIP.
	zipPath := filepath.Join(buildDir, "query-test.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)
	zw.Close()
	zf.Close()

	// Import.
	if err := ImportZIP(zipPath, "query-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}
}

// captureQueryOutput captures stdout during fn execution and returns the output.
func captureQueryOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// --- Impact tests ------------------------------------------------------------

func TestQueryImpact_DirectDependents(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	// primary-db impact at depth 1: payment-api and auth-service depend on it.
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["payment-api"] {
		t.Error("expected payment-api in impact results")
	}
	if !names["auth-service"] {
		t.Error("expected auth-service in impact results")
	}
	if names["redis-cache"] {
		t.Error("redis-cache should NOT be in depth-1 impact results")
	}
	if names["web-frontend"] {
		t.Error("web-frontend should NOT be in depth-1 impact results")
	}
}

func TestQueryImpact_TransitiveDependents(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	// Transitive: web-frontend depends on auth-service which depends on primary-db.
	if !names["web-frontend"] {
		t.Error("expected web-frontend in transitive impact results")
	}
	if !names["payment-api"] {
		t.Error("expected payment-api in transitive impact results")
	}
	if !names["auth-service"] {
		t.Error("expected auth-service in transitive impact results")
	}
}

func TestQueryImpact_DifferentFromDependencies(t *testing.T) {
	setupQueryTestGraph(t)

	// Impact on payment-api: things that DEPEND on payment-api.
	impactOutput := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "payment-api", "--depth", "all", "--graph", "query-test"})
	})

	// Dependencies of payment-api: things payment-api DEPENDS ON.
	depsOutput := captureQueryOutput(t, func() {
		CmdQuery([]string{"dependencies", "--component", "payment-api", "--depth", "all", "--graph", "query-test"})
	})

	var impactEnv, depsEnv QueryEnvelope
	json.Unmarshal([]byte(impactOutput), &impactEnv)
	json.Unmarshal([]byte(depsOutput), &depsEnv)

	impactJSON, _ := json.Marshal(impactEnv.Results)
	depsJSON, _ := json.Marshal(depsEnv.Results)

	var impactResult, depsResult ImpactResult
	json.Unmarshal(impactJSON, &impactResult)
	json.Unmarshal(depsJSON, &depsResult)

	impactNames := make(map[string]bool)
	for _, n := range impactResult.AffectedNodes {
		impactNames[n.Name] = true
	}
	depsNames := make(map[string]bool)
	for _, n := range depsResult.AffectedNodes {
		depsNames[n.Name] = true
	}

	// Impact should include auth-service and web-frontend (they depend on payment-api).
	if !impactNames["auth-service"] {
		t.Error("impact should include auth-service (depends on payment-api)")
	}
	if !impactNames["web-frontend"] {
		t.Error("impact should include web-frontend (depends on payment-api)")
	}

	// Dependencies should include primary-db and redis-cache (payment-api depends on them).
	if !depsNames["primary-db"] {
		t.Error("dependencies should include primary-db (payment-api depends on it)")
	}
	if !depsNames["redis-cache"] {
		t.Error("dependencies should include redis-cache (payment-api depends on it)")
	}

	// They must be different!
	if impactNames["primary-db"] && depsNames["auth-service"] {
		t.Error("impact and dependencies should return DIFFERENT results for the same component")
	}
}

// --- Dependencies tests ------------------------------------------------------

func TestQueryDependencies_Direct(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "web-frontend", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery dependencies: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["auth-service"] {
		t.Error("expected auth-service in web-frontend dependencies")
	}
	if !names["payment-api"] {
		t.Error("expected payment-api in web-frontend dependencies")
	}
	if len(result.AffectedNodes) != 2 {
		t.Errorf("expected 2 direct dependencies, got %d", len(result.AffectedNodes))
	}
}

// --- Path tests --------------------------------------------------------------

func TestQueryPath_Found(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"path", "--from", "web-frontend", "--to", "primary-db", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery path: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result PathResult
	json.Unmarshal(resultsJSON, &result)

	if result.Count == 0 {
		t.Fatal("expected at least one path from web-frontend to primary-db")
	}

	// Check per-hop confidence and provenance.
	firstPath := result.Paths[0]
	for _, hop := range firstPath.Hops {
		if hop.Confidence == 0 {
			t.Errorf("hop %s -> %s has zero confidence", hop.From, hop.To)
		}
		if hop.ConfidenceTier == "" {
			t.Errorf("hop %s -> %s missing confidence_tier", hop.From, hop.To)
		}
	}
	if firstPath.TotalConfidence == 0 {
		t.Error("total_confidence should be non-zero for a valid path")
	}
}

func TestQueryPath_NotFound(t *testing.T) {
	setupQueryTestGraph(t)

	// primary-db has no outgoing edges to redis-cache.
	output := captureQueryOutput(t, func() {
		// path from primary-db to redis-cache: no path exists.
		err := CmdQuery([]string{"path", "--from", "primary-db", "--to", "redis-cache", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("expected no error for no-path-found, got: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result PathResult
	json.Unmarshal(resultsJSON, &result)

	if result.Count != 0 {
		t.Errorf("expected 0 paths, got %d", result.Count)
	}
	if result.Reason == "" {
		t.Error("expected reason field when no path found")
	}
}

// --- List tests --------------------------------------------------------------

func TestQueryList_AllComponents(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"list", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery list: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ListResult
	json.Unmarshal(resultsJSON, &result)

	if result.Count != 5 {
		t.Errorf("expected 5 components, got %d", result.Count)
	}
}

func TestQueryList_FilterByType(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"list", "--type", "service", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery list: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ListResult
	json.Unmarshal(resultsJSON, &result)

	if result.Count != 3 {
		t.Errorf("expected 3 services, got %d", result.Count)
	}
	for _, c := range result.Components {
		if c.Type != "service" {
			t.Errorf("expected type=service, got %q for %s", c.Type, c.Name)
		}
	}
}

// --- Envelope tests ----------------------------------------------------------

func TestQueryEnvelope_Structure(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "primary-db", "--graph", "query-test"})
	})

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Must have query, results, metadata top-level keys.
	for _, key := range []string{"query", "results", "metadata"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}

	// Check metadata fields.
	meta, ok := raw["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata is not an object")
	}
	for _, field := range []string{"graph_version", "created_at", "component_count", "node_count", "edge_count", "graph_name"} {
		if _, ok := meta[field]; !ok {
			t.Errorf("metadata missing field %q", field)
		}
	}

	// graph_name must be non-empty and match the --graph flag value.
	gn, _ := meta["graph_name"].(string)
	if gn == "" {
		t.Error("metadata.graph_name must be non-empty")
	}
	if gn != "query-test" {
		t.Errorf("expected graph_name=%q, got %q", "query-test", gn)
	}
}

func TestQueryEnvelope_ConfidenceTier(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "primary-db", "--graph", "query-test"})
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	for _, r := range result.Relationships {
		if r.Confidence == 0 {
			t.Errorf("relationship %s -> %s missing confidence", r.From, r.To)
		}
		if r.ConfidenceTier == "" {
			t.Errorf("relationship %s -> %s missing confidence_tier", r.From, r.To)
		}
	}
}

// --- Error tests -------------------------------------------------------------

func TestQueryError_UnknownComponent(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "nonexistent-service", "--graph", "query-test"})
	})

	var errObj map[string]interface{}
	if err := json.Unmarshal([]byte(output), &errObj); err != nil {
		t.Fatalf("invalid JSON error: %v\noutput: %s", err, output)
	}

	code, ok := errObj["code"].(string)
	if !ok || code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %v", errObj["code"])
	}

	suggestions, ok := errObj["suggestions"].([]interface{})
	if !ok || len(suggestions) == 0 {
		t.Error("expected non-empty suggestions array")
	}
}

func TestQueryError_NoGraph(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "empty-xdg"))

	output := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "anything", "--graph", ""})
	})

	var errObj map[string]interface{}
	if err := json.Unmarshal([]byte(output), &errObj); err != nil {
		t.Fatalf("invalid JSON error: %v\noutput: %s", err, output)
	}

	code, ok := errObj["code"].(string)
	if !ok || code != "NO_GRAPH" {
		t.Errorf("expected code NO_GRAPH, got %v", errObj["code"])
	}

	if _, ok := errObj["action"]; !ok {
		t.Error("expected action field in NO_GRAPH error")
	}
}

// --- Fuzzy matching tests ----------------------------------------------------

func TestSuggestComponents(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "payment-api"})
	_ = g.AddNode(&Node{ID: "payment-service"})
	_ = g.AddNode(&Node{ID: "auth-service"})
	_ = g.AddNode(&Node{ID: "redis-cache"})

	tests := []struct {
		query    string
		expected string
	}{
		{"payment", "payment-api"},
		{"auth", "auth-service"},
		{"pay", "payment-api"},
	}

	for _, tc := range tests {
		suggestions := suggestComponents(g, tc.query)
		found := false
		for _, s := range suggestions {
			if strings.Contains(s, tc.expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("suggestComponents(%q) should include %q, got %v", tc.query, tc.expected, suggestions)
		}
	}
}

// --- Cycle detection tests ---------------------------------------------------

// setupCyclicQueryTestGraph creates a graph identical to setupQueryTestGraph but
// adds an edge from primary-db back to payment-api, creating a cycle:
//
//	payment-api -> primary-db -> payment-api
func setupCyclicQueryTestGraph(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	g := NewGraph()
	_ = g.AddNode(&Node{ID: "payment-api", Title: "Payment API", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "primary-db", Title: "Primary DB", Type: "document", ComponentType: ComponentTypeDatabase})
	_ = g.AddNode(&Node{ID: "redis-cache", Title: "Redis Cache", Type: "document", ComponentType: ComponentTypeCache})
	_ = g.AddNode(&Node{ID: "auth-service", Title: "Auth Service", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "web-frontend", Title: "Web Frontend", Type: "document", ComponentType: ComponentTypeService})

	edges := []*Edge{
		{ID: "payment-api->primary-db", Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.95, SourceFile: "payment-api.md", ExtractionMethod: "explicit-link"},
		{ID: "payment-api->redis-cache", Source: "payment-api", Target: "redis-cache", Type: EdgeDependsOn, Confidence: 0.85, SourceFile: "payment-api.md", ExtractionMethod: "co-occurrence"},
		{ID: "auth-service->primary-db", Source: "auth-service", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.90, SourceFile: "auth-service.md", ExtractionMethod: "structural"},
		{ID: "auth-service->payment-api", Source: "auth-service", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.70, SourceFile: "auth-service.md", ExtractionMethod: "semantic"},
		{ID: "web-frontend->auth-service", Source: "web-frontend", Target: "auth-service", Type: EdgeDependsOn, Confidence: 0.80, SourceFile: "web-frontend.md", ExtractionMethod: "explicit-link"},
		{ID: "web-frontend->payment-api", Source: "web-frontend", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.75, SourceFile: "web-frontend.md", ExtractionMethod: "co-occurrence"},
		// Cycle edge: primary-db depends on payment-api (creates payment-api -> primary-db -> payment-api cycle)
		{ID: "primary-db->payment-api", Source: "primary-db", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.60, SourceFile: "primary-db.md", ExtractionMethod: "structural"},
	}
	for _, e := range edges {
		_ = g.AddEdge(e)
	}

	// Save to SQLite.
	dbPath := filepath.Join(buildDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	db.Close()

	// Create metadata.
	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-03-29T00:00:00Z",
		ComponentCount:    5,
		RelationshipCount: 7,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	// Package as ZIP.
	zipPath := filepath.Join(buildDir, "cyclic-test.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)
	zw.Close()
	zf.Close()

	// Import.
	if err := ImportZIP(zipPath, "cyclic-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}
}

func TestQueryImpact_CyclicGraph_DetectsCycles(t *testing.T) {
	setupCyclicQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "cyclic-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	// metadata.cycles_detected must be non-empty.
	if len(env.Metadata.CyclesDetected) == 0 {
		t.Fatal("expected cycles_detected to be non-empty for cyclic graph")
	}

	// Check that the cycle entry has correct from/to fields.
	foundCycle := false
	for _, c := range env.Metadata.CyclesDetected {
		if c.From != "" && c.To != "" {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Error("expected cycle entry with non-empty from/to fields")
	}
}

func TestQueryDependencies_CyclicGraph_DetectsCycles(t *testing.T) {
	setupCyclicQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "payment-api", "--depth", "all", "--graph", "cyclic-test"})
		if err != nil {
			t.Fatalf("CmdQuery dependencies: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	// metadata.cycles_detected must be non-empty.
	if len(env.Metadata.CyclesDetected) == 0 {
		t.Fatal("expected cycles_detected to be non-empty for cyclic graph (dependencies)")
	}
}

func TestQueryImpact_AcyclicGraph_NoCycles(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	// cycles_detected must be nil/absent for acyclic graph.
	if len(env.Metadata.CyclesDetected) != 0 {
		t.Errorf("expected no cycles_detected for acyclic graph, got %d", len(env.Metadata.CyclesDetected))
	}
}

func TestQueryImpact_CyclicGraph_CorrectBFSDistances(t *testing.T) {
	setupCyclicQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "cyclic-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	// Verify BFS distances are correct despite cycle detection.
	distMap := make(map[string]int)
	for _, n := range result.AffectedNodes {
		distMap[n.Name] = n.Distance
	}

	// payment-api depends on primary-db directly => distance 1
	if d, ok := distMap["payment-api"]; !ok || d != 1 {
		t.Errorf("expected payment-api at distance 1, got %d (ok=%v)", d, ok)
	}
	// auth-service depends on primary-db directly => distance 1
	if d, ok := distMap["auth-service"]; !ok || d != 1 {
		t.Errorf("expected auth-service at distance 1, got %d (ok=%v)", d, ok)
	}
}

func TestQueryImpact_CyclicGraph_RootNotFalselyCycleParticipant(t *testing.T) {
	// On the acyclic graph, root should not be falsely reported as a cycle participant.
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)

	// On acyclic graph, no cycles should be reported at all (root not falsely reported).
	if len(env.Metadata.CyclesDetected) != 0 {
		t.Errorf("root node should not be falsely reported as cycle participant on acyclic graph, got %d cycles", len(env.Metadata.CyclesDetected))
	}
}

// --- Provenance tests --------------------------------------------------------

// setupProvenanceQueryTestGraph creates a test graph with component_mentions populated.
// Uses the same graph structure as setupQueryTestGraph but adds mention data.
func setupProvenanceQueryTestGraph(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	g := NewGraph()
	_ = g.AddNode(&Node{ID: "payment-api", Title: "Payment API", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "primary-db", Title: "Primary DB", Type: "document", ComponentType: ComponentTypeDatabase})
	_ = g.AddNode(&Node{ID: "redis-cache", Title: "Redis Cache", Type: "document", ComponentType: ComponentTypeCache})
	_ = g.AddNode(&Node{ID: "auth-service", Title: "Auth Service", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "web-frontend", Title: "Web Frontend", Type: "document", ComponentType: ComponentTypeService})

	edges := []*Edge{
		{ID: "payment-api->primary-db", Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.95, SourceFile: "payment-api.md", ExtractionMethod: "explicit-link"},
		{ID: "payment-api->redis-cache", Source: "payment-api", Target: "redis-cache", Type: EdgeDependsOn, Confidence: 0.85, SourceFile: "payment-api.md", ExtractionMethod: "co-occurrence"},
		{ID: "auth-service->primary-db", Source: "auth-service", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.90, SourceFile: "auth-service.md", ExtractionMethod: "structural"},
		{ID: "auth-service->payment-api", Source: "auth-service", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.70, SourceFile: "auth-service.md", ExtractionMethod: "semantic"},
		{ID: "web-frontend->auth-service", Source: "web-frontend", Target: "auth-service", Type: EdgeDependsOn, Confidence: 0.80, SourceFile: "web-frontend.md", ExtractionMethod: "explicit-link"},
		{ID: "web-frontend->payment-api", Source: "web-frontend", Target: "payment-api", Type: EdgeDependsOn, Confidence: 0.75, SourceFile: "web-frontend.md", ExtractionMethod: "co-occurrence"},
	}
	for _, e := range edges {
		_ = g.AddEdge(e)
	}

	// Save to SQLite and add component_mentions.
	dbPath := filepath.Join(buildDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	// Insert component mentions.
	mentions := []ComponentMention{
		{ComponentID: "payment-api", FilePath: "docs/payment.md", HeadingHierarchy: "Payment > API", DetectedBy: "explicit-link", Confidence: 0.95},
		{ComponentID: "payment-api", FilePath: "docs/arch.md", HeadingHierarchy: "Architecture", DetectedBy: "co-occurrence", Confidence: 0.80},
		{ComponentID: "auth-service", FilePath: "docs/auth.md", HeadingHierarchy: "Auth > Service", DetectedBy: "structural", Confidence: 0.90},
		{ComponentID: "auth-service", FilePath: "docs/security.md", HeadingHierarchy: "Security", DetectedBy: "semantic", Confidence: 0.75},
		{ComponentID: "primary-db", FilePath: "docs/database.md", HeadingHierarchy: "Database", DetectedBy: "explicit-link", Confidence: 0.92},
		{ComponentID: "web-frontend", FilePath: "docs/frontend.md", HeadingHierarchy: "Frontend", DetectedBy: "explicit-link", Confidence: 0.88},
	}
	if err := db.SaveComponentMentions(mentions); err != nil {
		t.Fatalf("save mentions: %v", err)
	}
	db.Close()

	// Create metadata.
	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-03-29T00:00:00Z",
		ComponentCount:    5,
		RelationshipCount: 6,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	// Package as ZIP.
	zipPath := filepath.Join(buildDir, "prov-test.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)
	zw.Close()
	zf.Close()

	// Import.
	if err := ImportZIP(zipPath, "prov-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}
}

// setupProvenanceQueryTestGraphManyMentions creates a graph where one component
// has 8 mentions, for testing --max-mentions truncation.
func setupProvenanceQueryTestGraphManyMentions(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	g := NewGraph()
	_ = g.AddNode(&Node{ID: "payment-api", Title: "Payment API", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "primary-db", Title: "Primary DB", Type: "document", ComponentType: ComponentTypeDatabase})

	_ = g.AddEdge(&Edge{ID: "payment-api->primary-db", Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.95, SourceFile: "payment-api.md", ExtractionMethod: "explicit-link"})

	dbPath := filepath.Join(buildDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	// Insert 8 mentions for payment-api.
	var mentions []ComponentMention
	for i := 0; i < 8; i++ {
		mentions = append(mentions, ComponentMention{
			ComponentID:      "payment-api",
			FilePath:         fmt.Sprintf("docs/file%d.md", i),
			HeadingHierarchy: fmt.Sprintf("Section %d", i),
			DetectedBy:       "explicit-link",
			Confidence:       0.90 - float64(i)*0.05,
		})
	}
	if err := db.SaveComponentMentions(mentions); err != nil {
		t.Fatalf("save mentions: %v", err)
	}
	db.Close()

	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-03-29T00:00:00Z",
		ComponentCount:    2,
		RelationshipCount: 1,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	zipPath := filepath.Join(buildDir, "prov-many-test.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)
	zw.Close()
	zf.Close()

	if err := ImportZIP(zipPath, "prov-many-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}
}

func TestQueryImpact_WithProvenance(t *testing.T) {
	setupProvenanceQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--include-provenance", "--graph", "prov-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact with provenance: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	resultsJSON, _ := json.Marshal(env.Results)
	var raw map[string]json.RawMessage
	json.Unmarshal(resultsJSON, &raw)

	var nodes []json.RawMessage
	json.Unmarshal(raw["affected_nodes"], &nodes)

	if len(nodes) == 0 {
		t.Fatal("expected affected nodes")
	}

	// Check first node has mentions.
	for _, nodeJSON := range nodes {
		var node map[string]interface{}
		json.Unmarshal(nodeJSON, &node)

		mentionsRaw, hasMentions := node["mentions"]
		if !hasMentions {
			t.Errorf("node %v missing 'mentions' field", node["name"])
			continue
		}

		mentionsList, ok := mentionsRaw.([]interface{})
		if !ok || len(mentionsList) == 0 {
			t.Errorf("node %v has empty or invalid mentions", node["name"])
			continue
		}

		// Verify mention fields.
		firstMention, _ := mentionsList[0].(map[string]interface{})
		for _, field := range []string{"file_path", "detection_method", "confidence"} {
			if _, ok := firstMention[field]; !ok {
				t.Errorf("mention missing field %q", field)
			}
		}

		// Verify mention_count is present.
		if _, hasMentionCount := node["mention_count"]; !hasMentionCount {
			t.Errorf("node %v missing 'mention_count' field", node["name"])
		}
	}
}

func TestQueryImpact_WithoutProvenance(t *testing.T) {
	setupProvenanceQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--graph", "prov-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact without provenance: %v", err)
		}
	})

	// Verify no mentions or mention_count fields in output.
	if strings.Contains(output, "mentions") {
		t.Error("output should NOT contain 'mentions' when --include-provenance is not set")
	}
	if strings.Contains(output, "mention_count") {
		t.Error("output should NOT contain 'mention_count' when --include-provenance is not set")
	}
}

func TestQueryDependencies_WithProvenance(t *testing.T) {
	setupProvenanceQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "web-frontend", "--include-provenance", "--graph", "prov-test"})
		if err != nil {
			t.Fatalf("CmdQuery dependencies with provenance: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	resultsJSON, _ := json.Marshal(env.Results)
	var raw map[string]json.RawMessage
	json.Unmarshal(resultsJSON, &raw)

	var nodes []json.RawMessage
	json.Unmarshal(raw["affected_nodes"], &nodes)

	if len(nodes) == 0 {
		t.Fatal("expected affected nodes in dependencies query")
	}

	// Check that at least one node has mentions.
	hasMentions := false
	for _, nodeJSON := range nodes {
		var node map[string]interface{}
		json.Unmarshal(nodeJSON, &node)
		if _, ok := node["mentions"]; ok {
			hasMentions = true
			break
		}
	}
	if !hasMentions {
		t.Error("expected at least one node with mentions in dependencies query")
	}
}

func TestQueryImpact_MaxMentions(t *testing.T) {
	setupProvenanceQueryTestGraphManyMentions(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--include-provenance", "--max-mentions", "3", "--graph", "prov-many-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact with max-mentions: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	resultsJSON, _ := json.Marshal(env.Results)
	var raw map[string]json.RawMessage
	json.Unmarshal(resultsJSON, &raw)

	var nodes []json.RawMessage
	json.Unmarshal(raw["affected_nodes"], &nodes)

	// payment-api depends on primary-db, so it should appear.
	for _, nodeJSON := range nodes {
		var node map[string]interface{}
		json.Unmarshal(nodeJSON, &node)

		if node["name"] == "payment-api" {
			mentionsList, _ := node["mentions"].([]interface{})
			if len(mentionsList) != 3 {
				t.Errorf("expected 3 mentions (limited by --max-mentions), got %d", len(mentionsList))
			}
			mentionCount, _ := node["mention_count"].(float64)
			if int(mentionCount) != 8 {
				t.Errorf("expected mention_count=8 (total before truncation), got %d", int(mentionCount))
			}
		}
	}
}

func TestQueryImpact_MaxMentionsZero(t *testing.T) {
	setupProvenanceQueryTestGraphManyMentions(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--include-provenance", "--max-mentions", "0", "--graph", "prov-many-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact with max-mentions=0: %v", err)
		}
	})

	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	resultsJSON, _ := json.Marshal(env.Results)
	var raw map[string]json.RawMessage
	json.Unmarshal(resultsJSON, &raw)

	var nodes []json.RawMessage
	json.Unmarshal(raw["affected_nodes"], &nodes)

	for _, nodeJSON := range nodes {
		var node map[string]interface{}
		json.Unmarshal(nodeJSON, &node)

		if node["name"] == "payment-api" {
			mentionsList, _ := node["mentions"].([]interface{})
			if len(mentionsList) != 8 {
				t.Errorf("expected all 8 mentions (max-mentions=0 means unlimited), got %d", len(mentionsList))
			}
		}
	}
}

func TestQueryImpact_CyclicGraph_CyclesDetectedAbsentInJSON(t *testing.T) {
	setupQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		CmdQuery([]string{"impact", "--component", "primary-db", "--depth", "all", "--graph", "query-test"})
	})

	// For acyclic graph, cycles_detected should be absent from JSON (omitempty).
	if strings.Contains(output, "cycles_detected") {
		t.Error("cycles_detected should be absent from JSON for acyclic graph (omitempty)")
	}
}

// --- Source type filter tests ------------------------------------------------

// setupSourceTypeQueryTestGraph creates a graph with edges of mixed source_type values.
//
// Graph:
//   app -> database     (markdown)
//   app -> redis        (code)
//   app -> queue        (both)
//   worker -> database  (code)
//   worker -> queue     (markdown)
func setupSourceTypeQueryTestGraph(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_DATA_HOME", xdgDir)

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0o755)

	g := NewGraph()
	_ = g.AddNode(&Node{ID: "app", Title: "App", Type: "document", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "database", Title: "Database", Type: "document", ComponentType: ComponentTypeDatabase})
	_ = g.AddNode(&Node{ID: "redis", Title: "Redis", Type: "document", ComponentType: ComponentTypeCache})
	_ = g.AddNode(&Node{ID: "queue", Title: "Queue", Type: "document", ComponentType: ComponentTypeUnknown})
	_ = g.AddNode(&Node{ID: "worker", Title: "Worker", Type: "document", ComponentType: ComponentTypeService})

	edges := []*Edge{
		{ID: "app->database", Source: "app", Target: "database", Type: EdgeDependsOn, Confidence: 0.90, SourceFile: "app.md", ExtractionMethod: "explicit-link", SourceType: "markdown"},
		{ID: "app->redis", Source: "app", Target: "redis", Type: EdgeDependsOn, Confidence: 0.75, SourceFile: "main.go", ExtractionMethod: "code-analysis", SourceType: "code"},
		{ID: "app->queue", Source: "app", Target: "queue", Type: EdgeDependsOn, Confidence: 0.84, SourceFile: "app.md", ExtractionMethod: "explicit-link", SourceType: "both"},
		{ID: "worker->database", Source: "worker", Target: "database", Type: EdgeDependsOn, Confidence: 0.70, SourceFile: "worker.go", ExtractionMethod: "code-analysis", SourceType: "code"},
		{ID: "worker->queue", Source: "worker", Target: "queue", Type: EdgeDependsOn, Confidence: 0.80, SourceFile: "worker.md", ExtractionMethod: "structural", SourceType: "markdown"},
	}
	for _, e := range edges {
		_ = g.AddEdge(e)
	}

	dbPath := filepath.Join(buildDir, "graph.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("save graph: %v", err)
	}
	db.Close()

	meta := ExportMetadata{
		Version:           "1.0.0",
		SchemaVersion:     SchemaVersion,
		CreatedAt:         "2026-04-02T00:00:00Z",
		ComponentCount:    5,
		RelationshipCount: 5,
		InputPath:         "/test",
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	zipPath := filepath.Join(buildDir, "st-test.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	dbData, _ := os.ReadFile(dbPath)
	w, _ := zw.Create("graph.db")
	w.Write(dbData)
	w, _ = zw.Create("metadata.json")
	w.Write(metaJSON)
	zw.Close()
	zf.Close()

	if err := ImportZIP(zipPath, "st-test"); err != nil {
		t.Fatalf("ImportZIP: %v", err)
	}
}

func TestEnrichedRelationshipSourceType(t *testing.T) {
	setupSourceTypeQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "app", "--depth", "1", "--graph", "st-test"})
		if err != nil {
			t.Fatalf("CmdQuery deps: %v", err)
		}
	})

	// Verify source_type appears in JSON output for every relationship.
	var env QueryEnvelope
	if err := json.Unmarshal([]byte(output), &env); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}

	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	if len(result.Relationships) == 0 {
		t.Fatal("expected relationships")
	}

	for _, rel := range result.Relationships {
		if rel.SourceType == "" {
			t.Errorf("relationship %s->%s has empty source_type", rel.From, rel.To)
		}
	}

	// Verify specific source types.
	stMap := make(map[string]string)
	for _, rel := range result.Relationships {
		stMap[rel.From+"->"+rel.To] = rel.SourceType
	}

	if st, ok := stMap["app->database"]; !ok || st != "markdown" {
		t.Errorf("app->database: expected source_type=markdown, got %q", st)
	}
	if st, ok := stMap["app->redis"]; !ok || st != "code" {
		t.Errorf("app->redis: expected source_type=code, got %q", st)
	}
	if st, ok := stMap["app->queue"]; !ok || st != "both" {
		t.Errorf("app->queue: expected source_type=both, got %q", st)
	}
}

func TestSourceTypeFilter_Code(t *testing.T) {
	setupSourceTypeQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "app", "--depth", "1", "--source-type", "code", "--graph", "st-test"})
		if err != nil {
			t.Fatalf("CmdQuery deps --source-type code: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	// --source-type code: should match "code" and "both" edges.
	// app->redis (code) and app->queue (both), but NOT app->database (markdown).
	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["redis"] {
		t.Error("expected redis in code-filtered results (source_type=code)")
	}
	if !names["queue"] {
		t.Error("expected queue in code-filtered results (source_type=both)")
	}
	if names["database"] {
		t.Error("database should NOT appear in code-filtered results (source_type=markdown)")
	}

	// Verify the filter appears in query params.
	if env.Query.SourceType != "code" {
		t.Errorf("expected query.source_type=code, got %q", env.Query.SourceType)
	}
}

func TestSourceTypeFilter_Markdown(t *testing.T) {
	setupSourceTypeQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "app", "--depth", "1", "--source-type", "markdown", "--graph", "st-test"})
		if err != nil {
			t.Fatalf("CmdQuery deps --source-type markdown: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	// --source-type markdown: should match "markdown" and "both" edges.
	// app->database (markdown) and app->queue (both), but NOT app->redis (code).
	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["database"] {
		t.Error("expected database in markdown-filtered results (source_type=markdown)")
	}
	if !names["queue"] {
		t.Error("expected queue in markdown-filtered results (source_type=both)")
	}
	if names["redis"] {
		t.Error("redis should NOT appear in markdown-filtered results (source_type=code)")
	}
}

func TestSourceTypeFilter_Both(t *testing.T) {
	setupSourceTypeQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"dependencies", "--component", "app", "--depth", "1", "--source-type", "both", "--graph", "st-test"})
		if err != nil {
			t.Fatalf("CmdQuery deps --source-type both: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	// --source-type both: should match ONLY "both" edges.
	// app->queue (both) only.
	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["queue"] {
		t.Error("expected queue in both-filtered results (source_type=both)")
	}
	if names["database"] {
		t.Error("database should NOT appear in both-filtered results (source_type=markdown)")
	}
	if names["redis"] {
		t.Error("redis should NOT appear in both-filtered results (source_type=code)")
	}
}

func TestSourceTypeDefault(t *testing.T) {
	// Edges without explicit source_type (empty) should default to "markdown" in output.
	setupQueryTestGraph(t) // This graph has no SourceType set on edges.

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "primary-db", "--graph", "query-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	for _, rel := range result.Relationships {
		if rel.SourceType != "markdown" {
			t.Errorf("relationship %s->%s: expected default source_type=markdown, got %q", rel.From, rel.To, rel.SourceType)
		}
	}
}

func TestSourceTypeFilter_Impact(t *testing.T) {
	// Verify --source-type works on impact queries (reverse traversal).
	setupSourceTypeQueryTestGraph(t)

	output := captureQueryOutput(t, func() {
		err := CmdQuery([]string{"impact", "--component", "database", "--depth", "1", "--source-type", "code", "--graph", "st-test"})
		if err != nil {
			t.Fatalf("CmdQuery impact --source-type code: %v", err)
		}
	})

	var env QueryEnvelope
	json.Unmarshal([]byte(output), &env)
	resultsJSON, _ := json.Marshal(env.Results)
	var result ImpactResult
	json.Unmarshal(resultsJSON, &result)

	// database impact with code filter: worker->database (code) matches, app->database (markdown) does not.
	names := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		names[n.Name] = true
	}

	if !names["worker"] {
		t.Error("expected worker in code-filtered impact results (worker->database is code)")
	}
	if names["app"] {
		t.Error("app should NOT appear in code-filtered impact results (app->database is markdown)")
	}
}
