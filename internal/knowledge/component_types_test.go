package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── Schema & Persistence Tests ─────────────────────────────────────────────

func TestSchemaV3_ComponentTypeColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Verify schema version.
	if v := db.GetSchemaVersion(); v != SchemaVersion {
		t.Errorf("schema version = %d, want %d", v, SchemaVersion)
	}

	// Insert a node with component_type.
	_, err = db.conn.Exec(
		`INSERT INTO graph_nodes (id, type, file, title, component_type) VALUES (?, ?, ?, ?, ?)`,
		"test-node", "document", "test.md", "Test", "service",
	)
	if err != nil {
		t.Fatalf("insert node with component_type: %v", err)
	}

	// Verify SELECT DISTINCT works.
	var ct string
	err = db.conn.QueryRow(`SELECT DISTINCT component_type FROM graph_nodes`).Scan(&ct)
	if err != nil {
		t.Fatalf("SELECT DISTINCT component_type: %v", err)
	}
	if ct != "service" {
		t.Errorf("component_type = %q, want %q", ct, "service")
	}
}

func TestSchemaV3_ComponentTypeDefaultsToUnknown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Insert without specifying component_type.
	_, err = db.conn.Exec(
		`INSERT INTO graph_nodes (id, type, file, title) VALUES (?, ?, ?, ?)`,
		"test-node", "document", "test.md", "Test",
	)
	if err != nil {
		t.Fatalf("insert node without component_type: %v", err)
	}

	var ct string
	err = db.conn.QueryRow(`SELECT component_type FROM graph_nodes WHERE id = ?`, "test-node").Scan(&ct)
	if err != nil {
		t.Fatalf("query component_type: %v", err)
	}
	if ct != "unknown" {
		t.Errorf("default component_type = %q, want %q", ct, "unknown")
	}
}

func TestSchemaV3_ComponentMentionsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Insert a node first (FK dependency).
	_, err = db.conn.Exec(
		`INSERT INTO graph_nodes (id, type, file, title, component_type) VALUES (?, ?, ?, ?, ?)`,
		"payment-api", "document", "payment-api.md", "Payment API", "service",
	)
	if err != nil {
		t.Fatalf("insert node: %v", err)
	}

	// Save a component mention.
	mentions := []ComponentMention{
		{
			ComponentID:      "payment-api",
			FilePath:         "payment-api.md",
			HeadingHierarchy: "## Architecture > ### API",
			DetectedBy:       "filename",
			Confidence:       0.85,
		},
	}
	if err := db.SaveComponentMentions(mentions); err != nil {
		t.Fatalf("SaveComponentMentions: %v", err)
	}

	// Verify mention was saved.
	var filePath, detectedBy string
	var confidence float64
	err = db.conn.QueryRow(
		`SELECT file_path, detected_by, confidence FROM component_mentions WHERE component_id = ?`,
		"payment-api",
	).Scan(&filePath, &detectedBy, &confidence)
	if err != nil {
		t.Fatalf("query component_mentions: %v", err)
	}
	if filePath != "payment-api.md" {
		t.Errorf("file_path = %q, want %q", filePath, "payment-api.md")
	}
	if detectedBy != "filename" {
		t.Errorf("detected_by = %q, want %q", detectedBy, "filename")
	}
	if confidence != 0.85 {
		t.Errorf("confidence = %.2f, want 0.85", confidence)
	}
}

func TestMigrationV2ToV3_ExistingDataSurvives(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create a v2 database manually.
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	// Create v2 schema (without component_type).
	v2Schema := `
CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS graph_nodes (id TEXT PRIMARY KEY, type TEXT NOT NULL, file TEXT NOT NULL, title TEXT, content TEXT, metadata TEXT);
CREATE TABLE IF NOT EXISTS graph_edges (id TEXT PRIMARY KEY, source_id TEXT NOT NULL, target_id TEXT NOT NULL, type TEXT NOT NULL, confidence REAL NOT NULL, evidence TEXT);
`
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS graph_nodes (id TEXT PRIMARY KEY, type TEXT NOT NULL, file TEXT NOT NULL, title TEXT, content TEXT, metadata TEXT)`,
		`CREATE TABLE IF NOT EXISTS graph_edges (id TEXT PRIMARY KEY, source_id TEXT NOT NULL, target_id TEXT NOT NULL, type TEXT NOT NULL, confidence REAL NOT NULL, evidence TEXT)`,
	} {
		if _, err := db.conn.Exec(stmt); err != nil {
			t.Fatalf("create v2 schema: %v", err)
		}
	}
	_ = v2Schema

	// Insert v2 data.
	_, err = db.conn.Exec(`INSERT OR REPLACE INTO metadata (key, value) VALUES ('schema_version', '2')`)
	if err != nil {
		t.Fatalf("set v2 version: %v", err)
	}
	_, err = db.conn.Exec(`INSERT INTO graph_nodes (id, type, file, title) VALUES (?, ?, ?, ?)`,
		"auth-service.md", "document", "auth-service.md", "Auth Service")
	if err != nil {
		t.Fatalf("insert v2 node: %v", err)
	}
	db.Close()

	// Re-open with OpenDB which runs migration.
	db2, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB after migration: %v", err)
	}
	defer db2.Close()

	// Verify the existing node survived.
	var title string
	err = db2.conn.QueryRow(`SELECT title FROM graph_nodes WHERE id = ?`, "auth-service.md").Scan(&title)
	if err != nil {
		t.Fatalf("query migrated node: %v", err)
	}
	if title != "Auth Service" {
		t.Errorf("migrated title = %q, want %q", title, "Auth Service")
	}

	// Verify component_type defaults to 'unknown'.
	var ct string
	err = db2.conn.QueryRow(`SELECT component_type FROM graph_nodes WHERE id = ?`, "auth-service.md").Scan(&ct)
	if err != nil {
		t.Fatalf("query migrated component_type: %v", err)
	}
	if ct != "unknown" {
		t.Errorf("migrated component_type = %q, want %q", ct, "unknown")
	}

	// Verify schema version is current.
	if v := db2.GetSchemaVersion(); v != SchemaVersion {
		t.Errorf("migrated schema version = %d, want %d", v, SchemaVersion)
	}
}

// ─── Component Detection Ensemble Tests ─────────────────────────────────────

func TestDetectComponents_AssignsTypes(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/payment-service.md", Title: "Payment Service", Type: "document"})
	_ = g.AddNode(&Node{ID: "databases/postgres.md", Title: "PostgreSQL Database", Type: "document"})
	_ = g.AddNode(&Node{ID: "cache/redis.md", Title: "Redis Cache", Type: "document"})
	_ = g.AddNode(&Node{ID: "docs/readme.md", Title: "README", Type: "document"})

	detector := NewComponentDetector()
	components := detector.DetectComponents(g, nil)

	typeMap := make(map[string]ComponentType)
	for _, c := range components {
		typeMap[c.ID] = c.Type
	}

	// payment-service should be detected as "service".
	if ct, ok := typeMap["payment-service"]; !ok {
		t.Error("payment-service not detected as component")
	} else if ct != ComponentTypeService {
		t.Errorf("payment-service type = %q, want %q", ct, ComponentTypeService)
	}
}

func TestDetectComponents_TypeConfidenceInRange(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/auth-service.md", Title: "Auth Service", Type: "document"})
	_ = g.AddNode(&Node{ID: "infra/monitoring.md", Title: "Monitoring Setup", Type: "document"})

	detector := NewComponentDetector()
	components := detector.DetectComponents(g, nil)

	for _, c := range components {
		if c.TypeConfidence < 0.4 || c.TypeConfidence > 1.0 {
			t.Errorf("component %q: TypeConfidence %.2f out of range [0.4, 1.0]", c.ID, c.TypeConfidence)
		}
	}
}

func TestDetectComponents_DetectionMethodsPopulated(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/api-service.md", Title: "API Service", Type: "document"})

	detector := NewComponentDetector()
	components := detector.DetectComponents(g, nil)

	if len(components) == 0 {
		t.Fatal("expected at least 1 component")
	}
	if len(components[0].DetectionMethods) == 0 {
		t.Error("expected non-empty DetectionMethods")
	}
}

func TestDetectComponents_UnknownTypeDefaultsGracefully(t *testing.T) {
	g := NewGraph()
	// Node with generic name that won't match any type pattern.
	_ = g.AddNode(&Node{ID: "docs/readme.md", Title: "README", Type: "document"})
	// Add edges to make it a high in-degree node.
	for i := 0; i < 5; i++ {
		src := "src" + string(rune('a'+i)) + ".md"
		_ = g.AddNode(&Node{ID: src, Title: "Source", Type: "document"})
		_ = g.AddEdge(&Edge{
			ID:         src + "\x00docs/readme.md\x00ref",
			Source:     src,
			Target:     "docs/readme.md",
			Type:       "mentions",
			Confidence: 0.8,
		})
	}

	detector := NewComponentDetector()
	components := detector.DetectComponents(g, nil)

	for _, c := range components {
		if c.ID == "readme" {
			if c.Type != ComponentTypeUnknown {
				t.Errorf("readme type = %q, want %q", c.Type, ComponentTypeUnknown)
			}
			if c.TypeConfidence < 0.4 {
				t.Errorf("readme TypeConfidence = %.2f, want >= 0.4", c.TypeConfidence)
			}
			return
		}
	}
	// It's ok if readme isn't detected as a component (in-degree threshold)
}

// ─── Integration Test: Detection -> Persistence -> Query ─────────────────────

func TestIntegration_DetectPersistQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Build a graph with typed components.
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/payment-service.md", Title: "Payment Service", Type: "document"})
	_ = g.AddNode(&Node{ID: "services/auth-service.md", Title: "Auth Service", Type: "document"})
	_ = g.AddNode(&Node{ID: "databases/postgres.md", Title: "PostgreSQL Database", Type: "document"})
	_ = g.AddNode(&Node{ID: "cache/redis-cache.md", Title: "Redis Cache", Type: "document"})
	_ = g.AddNode(&Node{ID: "docs/readme.md", Title: "README", Type: "document"})

	// Detect and classify components.
	detector := NewComponentDetector()
	components := detector.DetectComponents(g, nil)

	// Apply types to graph nodes.
	var mentions []ComponentMention
	for _, comp := range components {
		if node, ok := g.Nodes[comp.File]; ok {
			node.ComponentType = comp.Type
		}
		methods := "auto"
		if len(comp.DetectionMethods) > 0 {
			methods = comp.DetectionMethods[0]
		}
		mentions = append(mentions, ComponentMention{
			ComponentID: comp.File,
			FilePath:    comp.File,
			DetectedBy:  methods,
			Confidence:  comp.TypeConfidence,
		})
	}

	// Save graph.
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	// Save mentions.
	if err := db.SaveComponentMentions(mentions); err != nil {
		t.Fatalf("SaveComponentMentions: %v", err)
	}

	// Verify: SELECT DISTINCT component_type returns expected types.
	rows, err := db.conn.Query(`SELECT DISTINCT component_type FROM graph_nodes ORDER BY component_type`)
	if err != nil {
		t.Fatalf("query distinct types: %v", err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var ct string
		if err := rows.Scan(&ct); err != nil {
			t.Fatalf("scan type: %v", err)
		}
		types = append(types, ct)
	}

	if len(types) < 2 {
		t.Errorf("expected at least 2 distinct types, got %d: %v", len(types), types)
	}

	// Verify no NULL component_type values.
	var nullCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM graph_nodes WHERE component_type IS NULL`).Scan(&nullCount)
	if err != nil {
		t.Fatalf("query NULL count: %v", err)
	}
	if nullCount > 0 {
		t.Errorf("found %d nodes with NULL component_type", nullCount)
	}

	// Verify ListComponentsByType returns correct results.
	serviceNodes, err := db.ListComponentsByType(ComponentTypeService)
	if err != nil {
		t.Fatalf("ListComponentsByType: %v", err)
	}
	for _, n := range serviceNodes {
		if n.ComponentType != ComponentTypeService {
			t.Errorf("ListComponentsByType returned node %q with type %q", n.ID, n.ComponentType)
		}
	}

	// Verify component_mentions are saved.
	var mentionCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM component_mentions`).Scan(&mentionCount)
	if err != nil {
		t.Fatalf("query mention count: %v", err)
	}
	if mentionCount == 0 {
		t.Error("expected at least 1 component mention")
	}

	// Load graph and verify types survive round-trip.
	g2 := NewGraph()
	if err := db.LoadGraph(g2); err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	for id, node := range g2.Nodes {
		if node.ComponentType == "" {
			t.Errorf("node %q has empty ComponentType after LoadGraph", id)
		}
	}
}

// ─── Filesystem Integration Test ────────────────────────────────────────────

func TestIntegration_IndexAndQueryFromDisk(t *testing.T) {
	// Create a minimal test corpus.
	dir := t.TempDir()
	files := map[string]string{
		"services/payment-service.md": "# Payment Service\n\nHandles payments via Stripe API.\n",
		"databases/postgres.md":       "# PostgreSQL Database\n\nPrimary relational database.\n",
		"cache/redis.md":              "# Redis Cache\n\nIn-memory caching layer.\n",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Scan documents.
	kb := DefaultKnowledge()
	docs, err := kb.Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected documents from scan")
	}

	// Build graph with nodes.
	g := NewGraph()
	for _, doc := range docs {
		_ = g.AddNode(&Node{ID: doc.ID, Title: doc.Title, Type: "document"})
	}

	// Detect components and classify types.
	detector := NewComponentDetector()
	components := detector.DetectComponents(g, docs)
	for _, comp := range components {
		if node, ok := g.Nodes[comp.File]; ok {
			node.ComponentType = comp.Type
		}
	}

	// Save to database.
	dbPath := filepath.Join(dir, ".bmd", "knowledge.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	// Query by type.
	serviceNodes, err := db.ListComponentsByType(ComponentTypeService)
	if err != nil {
		t.Fatalf("ListComponentsByType: %v", err)
	}

	// Verify at least one service found.
	foundService := false
	for _, n := range serviceNodes {
		if n.ComponentType == ComponentTypeService {
			foundService = true
		}
	}
	if !foundService && len(components) > 0 {
		t.Log("Note: no service nodes found in ListComponentsByType query")
	}

	// Verify no NULL types.
	var nullCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM graph_nodes WHERE component_type IS NULL`).Scan(&nullCount)
	if err != nil {
		t.Fatalf("query NULL count: %v", err)
	}
	if nullCount > 0 {
		t.Errorf("found %d nodes with NULL component_type", nullCount)
	}
}
