package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAliasLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadAliasConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Aliases) != 0 {
		t.Errorf("expected empty aliases, got %v", cfg.Aliases)
	}
}

func TestAliasLoadYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `aliases:
  postgres-primary:
    - pg-main
    - primary-db
  redis-cache:
    - redis
    - cache-server
`
	if err := os.WriteFile(filepath.Join(dir, "graphmd-aliases.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAliasConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Aliases) != 2 {
		t.Fatalf("expected 2 alias groups, got %d", len(cfg.Aliases))
	}
	if len(cfg.Aliases["postgres-primary"]) != 2 {
		t.Errorf("expected 2 aliases for postgres-primary, got %d", len(cfg.Aliases["postgres-primary"]))
	}
}

func TestAliasResolveKnown(t *testing.T) {
	cfg := &AliasConfig{
		Aliases: map[string][]string{
			"postgres-primary": {"pg-main", "primary-db"},
			"redis-cache":     {"redis", "cache-server"},
		},
	}

	tests := []struct {
		input string
		want  string
	}{
		{"pg-main", "postgres-primary"},
		{"primary-db", "postgres-primary"},
		{"redis", "redis-cache"},
		{"cache-server", "redis-cache"},
	}

	for _, tt := range tests {
		got := cfg.ResolveAlias(tt.input)
		if got != tt.want {
			t.Errorf("ResolveAlias(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAliasResolveUnknown(t *testing.T) {
	cfg := &AliasConfig{
		Aliases: map[string][]string{
			"postgres-primary": {"pg-main"},
		},
	}

	// Unknown names pass through unchanged.
	got := cfg.ResolveAlias("unknown-service")
	if got != "unknown-service" {
		t.Errorf("ResolveAlias(unknown) = %q, want %q", got, "unknown-service")
	}

	// Canonical names also pass through (they are not aliases).
	got = cfg.ResolveAlias("postgres-primary")
	if got != "postgres-primary" {
		t.Errorf("ResolveAlias(canonical) = %q, want %q", got, "postgres-primary")
	}
}

func TestAliasResolveCaseSensitive(t *testing.T) {
	cfg := &AliasConfig{
		Aliases: map[string][]string{
			"postgres-primary": {"PG-Main"},
		},
	}

	// Exact case should match.
	if got := cfg.ResolveAlias("PG-Main"); got != "postgres-primary" {
		t.Errorf("ResolveAlias(PG-Main) = %q, want postgres-primary", got)
	}
	// Wrong case should NOT match (case-sensitive).
	if got := cfg.ResolveAlias("pg-main"); got != "pg-main" {
		t.Errorf("ResolveAlias(pg-main) = %q, want pg-main (passthrough)", got)
	}
}

func TestAliasResolveBatch(t *testing.T) {
	cfg := &AliasConfig{
		Aliases: map[string][]string{
			"postgres-primary": {"pg-main", "primary-db"},
		},
	}

	input := []string{"pg-main", "unknown", "primary-db"}
	got := cfg.ResolveAliases(input)
	want := []string{"postgres-primary", "unknown", "postgres-primary"}

	if len(got) != len(want) {
		t.Fatalf("ResolveAliases: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ResolveAliases[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
