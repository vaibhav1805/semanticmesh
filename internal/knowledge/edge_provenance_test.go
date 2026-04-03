package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadGraph_PreservesProvenance(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	now := time.Now().Unix()

	graph := NewGraph()
	_ = graph.AddNode(&Node{ID: "payment-api.md", Type: "document", Title: "Payment API"})
	_ = graph.AddNode(&Node{ID: "primary-db.md", Type: "document", Title: "Primary DB"})

	edge := &Edge{
		ID:                "edge-1",
		Source:            "payment-api.md",
		Target:            "primary-db.md",
		Type:              EdgeDependsOn,
		Confidence:        0.8,
		Evidence:          "calls for transaction storage",
		SourceFile:        "docs/service.yaml",
		ExtractionMethod:  "explicit-link",
		DetectionEvidence: "calls primary-db for transaction storage",
		EvidencePointer:   "service.yaml:42",
		LastModified:      now,
	}
	_ = graph.AddEdge(edge)

	if err := db.SaveGraph(graph); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	loaded := NewGraph()
	if err := db.LoadGraph(loaded); err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if len(loaded.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(loaded.Edges))
	}

	var loadedEdge *Edge
	for _, e := range loaded.Edges {
		loadedEdge = e
	}

	if loadedEdge.SourceFile != "docs/service.yaml" {
		t.Errorf("SourceFile = %q, want %q", loadedEdge.SourceFile, "docs/service.yaml")
	}
	if loadedEdge.ExtractionMethod != "explicit-link" {
		t.Errorf("ExtractionMethod = %q, want %q", loadedEdge.ExtractionMethod, "explicit-link")
	}
	if loadedEdge.DetectionEvidence != "calls primary-db for transaction storage" {
		t.Errorf("DetectionEvidence = %q, want %q", loadedEdge.DetectionEvidence, "calls primary-db for transaction storage")
	}
	if loadedEdge.EvidencePointer != "service.yaml:42" {
		t.Errorf("EvidencePointer = %q, want %q", loadedEdge.EvidencePointer, "service.yaml:42")
	}
	if loadedEdge.LastModified != now {
		t.Errorf("LastModified = %d, want %d", loadedEdge.LastModified, now)
	}
}

func TestSaveLoadGraph_MultipleExtractionMethods(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	methods := []string{"explicit-link", "co-occurrence", "structural", "NER", "semantic", "LLM"}
	now := time.Now().Unix()

	graph := NewGraph()
	for i, method := range methods {
		srcID := "src" + string(rune('A'+i)) + ".md"
		tgtID := "tgt" + string(rune('A'+i)) + ".md"
		_ = graph.AddNode(&Node{ID: srcID, Type: "document", Title: srcID})
		_ = graph.AddNode(&Node{ID: tgtID, Type: "document", Title: tgtID})
		e, _ := NewEdge(srcID, tgtID, EdgeMentions, 0.7, "evidence")
		e.SourceFile = "docs/test.md"
		e.ExtractionMethod = method
		e.DetectionEvidence = "detected via " + method
		e.EvidencePointer = "test.md:10"
		e.LastModified = now
		_ = graph.AddEdge(e)
	}

	if err := db.SaveGraph(graph); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	loaded := NewGraph()
	if err := db.LoadGraph(loaded); err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if len(loaded.Edges) != len(methods) {
		t.Fatalf("expected %d edges, got %d", len(methods), len(loaded.Edges))
	}

	// Verify all extraction methods survive round-trip.
	foundMethods := make(map[string]bool)
	for _, e := range loaded.Edges {
		foundMethods[e.ExtractionMethod] = true
	}
	for _, method := range methods {
		if !foundMethods[method] {
			t.Errorf("extraction method %q not found after round-trip", method)
		}
	}
}

func TestLoadGraph_BackwardCompat_NullProvenance(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Insert edge without provenance columns (simulating v3-era data).
	graph := NewGraph()
	_ = graph.AddNode(&Node{ID: "a.md", Type: "document", Title: "A"})
	_ = graph.AddNode(&Node{ID: "b.md", Type: "document", Title: "B"})
	e, _ := NewEdge("a.md", "b.md", EdgeReferences, 1.0, "link")
	_ = graph.AddEdge(e)

	if err := db.SaveGraph(graph); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	loaded := NewGraph()
	if err := db.LoadGraph(loaded); err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if len(loaded.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(loaded.Edges))
	}

	var loadedEdge *Edge
	for _, e := range loaded.Edges {
		loadedEdge = e
	}

	// NULL provenance columns should yield zero-value strings and int64.
	if loadedEdge.SourceFile != "" {
		t.Errorf("SourceFile = %q, want empty", loadedEdge.SourceFile)
	}
	if loadedEdge.ExtractionMethod != "" {
		t.Errorf("ExtractionMethod = %q, want empty", loadedEdge.ExtractionMethod)
	}
	if loadedEdge.DetectionEvidence != "" {
		t.Errorf("DetectionEvidence = %q, want empty", loadedEdge.DetectionEvidence)
	}
	if loadedEdge.EvidencePointer != "" {
		t.Errorf("EvidencePointer = %q, want empty", loadedEdge.EvidencePointer)
	}
	if loadedEdge.LastModified != 0 {
		t.Errorf("LastModified = %d, want 0", loadedEdge.LastModified)
	}
}

func TestValidateEdge_Valid(t *testing.T) {
	e := &Edge{
		SourceFile:        "docs/service.yaml",
		ExtractionMethod:  "explicit-link",
		DetectionEvidence: "calls primary-db",
		EvidencePointer:   "service.yaml:42",
		LastModified:      time.Now().Unix(),
	}
	if err := ValidateEdge(e); err != nil {
		t.Errorf("ValidateEdge: unexpected error: %v", err)
	}
}

func TestValidateEdge_NoProvenance_Passes(t *testing.T) {
	e := &Edge{}
	if err := ValidateEdge(e); err != nil {
		t.Errorf("ValidateEdge with no provenance: unexpected error: %v", err)
	}
}

func TestValidateEdge_InvalidExtractionMethod(t *testing.T) {
	e := &Edge{
		SourceFile:        "docs/service.yaml",
		ExtractionMethod:  "magic",
		DetectionEvidence: "some evidence",
		LastModified:      time.Now().Unix(),
	}
	if err := ValidateEdge(e); err == nil {
		t.Error("ValidateEdge: expected error for invalid ExtractionMethod")
	}
}

func TestValidateEdge_AbsoluteSourceFile(t *testing.T) {
	e := &Edge{
		SourceFile:        "/tmp/service.yaml",
		ExtractionMethod:  "structural",
		DetectionEvidence: "some evidence",
		LastModified:      time.Now().Unix(),
	}
	if err := ValidateEdge(e); err == nil {
		t.Error("ValidateEdge: expected error for absolute SourceFile")
	}
}

func TestValidateEdge_EmptySourceFileWithOtherProvenance(t *testing.T) {
	e := &Edge{
		ExtractionMethod:  "NER",
		DetectionEvidence: "some evidence",
		LastModified:      time.Now().Unix(),
	}
	if err := ValidateEdge(e); err == nil {
		t.Error("ValidateEdge: expected error when SourceFile empty but other provenance set")
	}
}

func TestValidateEdge_Nil(t *testing.T) {
	if err := ValidateEdge(nil); err == nil {
		t.Error("ValidateEdge(nil): expected error")
	}
}

func TestIsValidExtractionMethod(t *testing.T) {
	valid := []string{"explicit-link", "co-occurrence", "structural", "NER", "semantic", "LLM"}
	for _, m := range valid {
		if !IsValidExtractionMethod(m) {
			t.Errorf("IsValidExtractionMethod(%q) = false, want true", m)
		}
	}

	invalid := []string{"", "magic", "regex", "explicit_link", "ner"}
	for _, m := range invalid {
		if IsValidExtractionMethod(m) {
			t.Errorf("IsValidExtractionMethod(%q) = true, want false", m)
		}
	}
}

func TestMigration_V3toV4(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-v3.db")

	// Create a fresh database — it will be at SchemaVersion 4 already.
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}

	// Verify we're at v4.
	if v := db.GetSchemaVersion(); v != 4 {
		t.Errorf("schema version = %d, want 4", v)
	}

	// Verify the provenance columns exist by inserting a full edge.
	_, err = db.conn.Exec(
		`INSERT INTO graph_nodes (id, type, file, title, component_type) VALUES ('n1', 'document', 'n1.md', 'N1', 'unknown')`,
	)
	if err != nil {
		t.Fatalf("insert node: %v", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO graph_nodes (id, type, file, title, component_type) VALUES ('n2', 'document', 'n2.md', 'N2', 'unknown')`,
	)
	if err != nil {
		t.Fatalf("insert node: %v", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO graph_edges (id, source_id, target_id, type, confidence, evidence, source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)
		 VALUES ('e1', 'n1', 'n2', 'mentions', 0.7, 'test', 'docs/test.md', 'structural', 'detected via structural', 'test.md:10', ?)`,
		time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("insert edge with provenance: %v", err)
	}

	// Verify we can read back the provenance.
	var sf, em, de, ep string
	var lm int64
	err = db.conn.QueryRow(
		`SELECT source_file, extraction_method, detection_evidence, evidence_pointer, last_modified FROM graph_edges WHERE id = 'e1'`,
	).Scan(&sf, &em, &de, &ep, &lm)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if sf != "docs/test.md" {
		t.Errorf("source_file = %q, want %q", sf, "docs/test.md")
	}
	if em != "structural" {
		t.Errorf("extraction_method = %q, want %q", em, "structural")
	}

	db.Close()

	// Re-open and run Migrate again (idempotent).
	db2, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer db2.Close()
	if v := db2.GetSchemaVersion(); v != 4 {
		t.Errorf("after re-open schema version = %d, want 4", v)
	}
}

func TestEdge_String_WithProvenance(t *testing.T) {
	e := &Edge{
		Source:           "a.md",
		Target:           "b.md",
		Type:             EdgeReferences,
		Confidence:       1.0,
		Evidence:         "link",
		SourceFile:       "docs/a.md",
		ExtractionMethod: "explicit-link",
	}
	s := e.String()
	if s == "" {
		t.Error("String() returned empty")
	}
	// Should include provenance info.
	if !contains(s, "explicit-link") || !contains(s, "docs/a.md") {
		t.Errorf("String() = %q, expected provenance info", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure os is used (test file cleanup).
var _ = os.TempDir
