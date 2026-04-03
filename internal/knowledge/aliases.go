package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// AliasConfig holds component name aliases, mapping canonical names to their
// alternate names. This allows multiple names in documentation to resolve to a
// single canonical component identity during graph building.
type AliasConfig struct {
	// Aliases maps canonical component name -> list of alias names.
	// Example: {"postgres-primary": ["pg-main", "primary-db", "pgdb"]}
	Aliases map[string][]string `yaml:"aliases"`

	// reverse is the lazily-built reverse lookup map (alias -> canonical).
	reverse map[string]string
	once    sync.Once
}

// LoadAliasConfig reads a semanticmesh-aliases.yaml file from the given directory.
// If the file does not exist, an empty (but usable) AliasConfig is returned.
func LoadAliasConfig(dir string) (*AliasConfig, error) {
	path := filepath.Join(dir, "semanticmesh-aliases.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AliasConfig{Aliases: make(map[string][]string)}, nil
		}
		return nil, fmt.Errorf("knowledge.LoadAliasConfig: read %q: %w", path, err)
	}

	var cfg AliasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("knowledge.LoadAliasConfig: parse %q: %w", path, err)
	}

	if cfg.Aliases == nil {
		cfg.Aliases = make(map[string][]string)
	}

	return &cfg, nil
}

// buildReverse constructs the reverse lookup map (alias -> canonical name).
// Called lazily on first ResolveAlias invocation.
func (ac *AliasConfig) buildReverse() {
	ac.reverse = make(map[string]string)
	for canonical, aliases := range ac.Aliases {
		for _, alias := range aliases {
			ac.reverse[alias] = canonical
		}
	}
}

// ResolveAlias returns the canonical name for the given name if it matches any
// known alias. If no alias is found, the input name is returned unchanged.
// Matching is case-sensitive.
func (ac *AliasConfig) ResolveAlias(name string) string {
	ac.once.Do(ac.buildReverse)
	if canonical, ok := ac.reverse[name]; ok {
		return canonical
	}
	return name
}

// ResolveAliases resolves a batch of names using ResolveAlias.
func (ac *AliasConfig) ResolveAliases(names []string) []string {
	result := make([]string, len(names))
	for i, n := range names {
		result[i] = ac.ResolveAlias(n)
	}
	return result
}
