package knowledge

import (
	"testing"
)

func TestAllComponentTypes_Returns13Types(t *testing.T) {
	types := AllComponentTypes()
	if len(types) != 16 {
		t.Fatalf("expected 16 types (15 + unknown), got %d", len(types))
	}

	// Verify all 15 named types plus unknown are present.
	expected := map[ComponentType]bool{
		ComponentTypeService:           true,
		ComponentTypeDatabase:          true,
		ComponentTypeCache:             true,
		ComponentTypeQueue:             true,
		ComponentTypeMessageBroker:     true,
		ComponentTypeLoadBalancer:      true,
		ComponentTypeGateway:           true,
		ComponentTypeStorage:           true,
		ComponentTypeContainerRegistry: true,
		ComponentTypeConfigServer:      true,
		ComponentTypeMonitoring:        true,
		ComponentTypeLogAggregator:     true,
		ComponentTypeOrchestrator:      true,
		ComponentTypeSecretsManager:    true,
		ComponentTypeSearch:            true,
		ComponentTypeUnknown:           true,
	}
	for _, ct := range types {
		if !expected[ct] {
			t.Errorf("unexpected type in AllComponentTypes: %q", ct)
		}
		delete(expected, ct)
	}
	for ct := range expected {
		t.Errorf("missing type from AllComponentTypes: %q", ct)
	}
}

func TestIsValidComponentType(t *testing.T) {
	// All 15 types + unknown must be valid.
	for _, ct := range AllComponentTypes() {
		if !IsValidComponentType(ct) {
			t.Errorf("IsValidComponentType(%q) = false, want true", ct)
		}
	}

	// Invalid types must return false.
	invalids := []ComponentType{"foo", "SERVICE", "Database", "", "microservice"}
	for _, ct := range invalids {
		if IsValidComponentType(ct) {
			t.Errorf("IsValidComponentType(%q) = true, want false", ct)
		}
	}
}

func TestComponentTypeDescription_AllTypesHaveDescriptions(t *testing.T) {
	for _, ct := range AllComponentTypes() {
		desc := ComponentTypeDescription(ct)
		if desc == "" || desc == "Unknown type" {
			t.Errorf("ComponentTypeDescription(%q) returned empty or fallback", ct)
		}
	}
}

func TestInferComponentType_NameMatches(t *testing.T) {
	tests := []struct {
		name     string
		wantType ComponentType
		wantMin  float64
	}{
		{"payment-service", ComponentTypeService, 0.8},
		{"user-api", ComponentTypeService, 0.8},
		{"primary-database", ComponentTypeDatabase, 0.8},
		{"postgres-db", ComponentTypeDatabase, 0.8},
		{"redis-cache", ComponentTypeCache, 0.8},
		{"task-queue", ComponentTypeQueue, 0.8},
		{"kafka-broker", ComponentTypeMessageBroker, 0.8},
		{"rabbitmq-events", ComponentTypeMessageBroker, 0.8},
		{"nginx-lb", ComponentTypeLoadBalancer, 0.8},
		{"api-gateway", ComponentTypeGateway, 0.8},
		{"s3-storage", ComponentTypeStorage, 0.8},
		{"docker-registry", ComponentTypeContainerRegistry, 0.8},
		{"consul-config", ComponentTypeConfigServer, 0.8},
		{"prometheus-monitoring", ComponentTypeMonitoring, 0.8},
		{"fluentd-logging", ComponentTypeLogAggregator, 0.8},
		{"k8s-cluster", ComponentTypeOrchestrator, 0.8},
		{"vault-secrets", ComponentTypeSecretsManager, 0.8},
		{"algolia-search", ComponentTypeSearch, 0.8},
		{"deployment-manager", ComponentTypeService, 0.8},
		{"cluster-operator", ComponentTypeService, 0.8},
		{"app-manifest-manager/README.md", ComponentTypeService, 0.8},
	}

	for _, tt := range tests {
		ct, conf := InferComponentType(tt.name)
		if ct != tt.wantType {
			t.Errorf("InferComponentType(%q) type = %q, want %q", tt.name, ct, tt.wantType)
		}
		if conf < tt.wantMin {
			t.Errorf("InferComponentType(%q) confidence = %.2f, want >= %.2f", tt.name, conf, tt.wantMin)
		}
	}
}

func TestInferComponentType_ContextMatches(t *testing.T) {
	ct, conf := InferComponentType("primary-data", "This is a PostgreSQL database cluster")
	if ct != ComponentTypeDatabase {
		t.Errorf("context match: got type %q, want %q", ct, ComponentTypeDatabase)
	}
	if conf < 0.7 || conf > 0.8 {
		t.Errorf("context match: got confidence %.2f, want ~0.75", conf)
	}
}

func TestInferComponentType_NoMatch(t *testing.T) {
	ct, conf := InferComponentType("foobar-widget")
	if ct != ComponentTypeUnknown {
		t.Errorf("no match: got type %q, want %q", ct, ComponentTypeUnknown)
	}
	if conf < 0.4 || conf > 1.0 {
		t.Errorf("no match: confidence %.2f out of valid range [0.4, 1.0]", conf)
	}
}

func TestInferComponentType_ExactTypeNameMatch(t *testing.T) {
	ct, conf := InferComponentType("database")
	if ct != ComponentTypeDatabase {
		t.Errorf("exact match: got type %q, want %q", ct, ComponentTypeDatabase)
	}
	if conf < 0.9 {
		t.Errorf("exact match: confidence %.2f, want >= 0.9", conf)
	}
}

func TestSeedConfig_ApplySeedConfig(t *testing.T) {
	sc := &SeedConfig{
		TypeMappings: []SeedMapping{
			{Pattern: "redis*", Type: ComponentTypeCache},
			{Pattern: "postgres*", Type: ComponentTypeDatabase},
			{Pattern: "my-custom-service", Type: ComponentTypeService},
		},
	}

	tests := []struct {
		name     string
		wantType ComponentType
		wantConf float64
	}{
		{"redis-cluster", ComponentTypeCache, 1.0},
		{"redis-cache", ComponentTypeCache, 1.0},
		{"postgres-primary", ComponentTypeDatabase, 1.0},
		{"my-custom-service", ComponentTypeService, 1.0},
		{"unknown-widget", "", 0},
	}

	for _, tt := range tests {
		ct, conf := sc.ApplySeedConfig(tt.name)
		if ct != tt.wantType {
			t.Errorf("ApplySeedConfig(%q) type = %q, want %q", tt.name, ct, tt.wantType)
		}
		if conf != tt.wantConf {
			t.Errorf("ApplySeedConfig(%q) confidence = %.2f, want %.2f", tt.name, conf, tt.wantConf)
		}
	}
}

func TestSeedConfig_NilSafe(t *testing.T) {
	var sc *SeedConfig
	ct, conf := sc.ApplySeedConfig("anything")
	if ct != "" || conf != 0 {
		t.Errorf("nil SeedConfig: got (%q, %.2f), want (\"\", 0)", ct, conf)
	}
}

func TestRelationshipLocationKey_Deterministic(t *testing.T) {
	loc := RelationshipLocation{File: "docs/service.yaml", Line: 42}
	key1 := RelationshipLocationKey(loc)
	key2 := RelationshipLocationKey(loc)
	if key1 != key2 {
		t.Errorf("keys not deterministic: %q != %q", key1, key2)
	}
	if key1 != "docs/service.yaml:42" {
		t.Errorf("key format: got %q, want %q", key1, "docs/service.yaml:42")
	}
}

func TestRelationshipLocationKey_DifferentLocations(t *testing.T) {
	loc1 := RelationshipLocation{File: "file1.md", Line: 10}
	loc2 := RelationshipLocation{File: "file2.md", Line: 20}
	loc3 := RelationshipLocation{File: "file1.md", Line: 20}

	k1 := RelationshipLocationKey(loc1)
	k2 := RelationshipLocationKey(loc2)
	k3 := RelationshipLocationKey(loc3)

	if k1 == k2 {
		t.Error("different files should have different keys")
	}
	if k1 == k3 {
		t.Error("different lines should have different keys")
	}
	if k2 == k3 {
		t.Error("different file+line combos should have different keys")
	}
}

func TestRelationshipLocation_String(t *testing.T) {
	loc := RelationshipLocation{File: "docs/api.md", Line: 15, Evidence: "depends on postgres"}
	s := loc.String()
	if s != "docs/api.md:15 (depends on postgres)" {
		t.Errorf("String(): got %q", s)
	}

	loc2 := RelationshipLocation{File: "docs/api.md", Line: 15}
	s2 := loc2.String()
	if s2 != "docs/api.md:15" {
		t.Errorf("String() no evidence: got %q", s2)
	}
}

func TestRelationshipLocation_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		loc   RelationshipLocation
		valid bool
	}{
		{"valid", RelationshipLocation{File: "docs/api.md", Line: 10}, true},
		{"line zero", RelationshipLocation{File: "docs/api.md", Line: 0}, true},
		{"empty file", RelationshipLocation{File: "", Line: 10}, false},
		{"absolute path", RelationshipLocation{File: "/tmp/api.md", Line: 10}, false},
		{"negative line", RelationshipLocation{File: "docs/api.md", Line: -1}, false},
	}
	for _, tt := range tests {
		if got := tt.loc.IsValid(); got != tt.valid {
			t.Errorf("%s: IsValid() = %v, want %v", tt.name, got, tt.valid)
		}
	}
}
