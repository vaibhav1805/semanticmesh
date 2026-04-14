package mendixparser

import (
	"encoding/json"
	"fmt"

	"github.com/mendixlabs/mxcli/mdl/catalog"
	"github.com/mendixlabs/mxcli/sdk/mpr"
)

// CatalogManager wraps mxcli catalog operations with lifecycle management
type CatalogManager struct {
	reader  *mpr.Reader
	catalog *catalog.Catalog
}

// NewCatalogManager creates a new catalog manager for the given MPR file
func NewCatalogManager(mprPath string) (*CatalogManager, error) {
	reader, err := mpr.Open(mprPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MPR: %w", err)
	}

	cat, err := catalog.New()
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to create catalog: %w", err)
	}

	return &CatalogManager{
		reader:  reader,
		catalog: cat,
	}, nil
}

// BuildFull builds the full catalog
func (cm *CatalogManager) BuildFull() error {
	builder := catalog.NewBuilder(cm.catalog, cm.reader)
	builder.SetFullMode(true) // Enable full parsing for refs/activities
	return builder.Build(nil)  // nil = no progress callback
}

// Query executes an MDL query and returns results
func (cm *CatalogManager) Query(sql string) (*catalog.QueryResult, error) {
	return cm.catalog.Query(sql)
}

// QueryAsMap executes an MDL query and returns results as []map[string]interface{}
// for backward compatibility with existing extraction methods
func (cm *CatalogManager) QueryAsMap(sql string) ([]map[string]interface{}, error) {
	result, err := cm.catalog.Query(sql)
	if err != nil {
		return nil, err
	}

	// Convert rows to map format
	var maps []map[string]interface{}
	for _, row := range result.Rows {
		rowMap := make(map[string]interface{})
		for i, col := range result.Columns {
			if i < len(row) {
				rowMap[col] = row[i]
			}
		}
		maps = append(maps, rowMap)
	}

	return maps, nil
}

// Close releases resources held by the catalog manager
func (cm *CatalogManager) Close() error {
	return cm.reader.Close()
}

// BuildCatalog refreshes the mxcli catalog for a project
func (p *Parser) BuildCatalog(mprPath string) error {
	cm, err := NewCatalogManager(mprPath)
	if err != nil {
		return fmt.Errorf("catalog build failed: %w", err)
	}
	defer cm.Close()

	if err := cm.BuildFull(); err != nil {
		return fmt.Errorf("catalog build failed: %w", err)
	}

	return nil
}

// QueryCatalog executes an MDL query and returns JSON results
// Maintains compatibility with existing code by returning []map[string]interface{}
func (p *Parser) QueryCatalog(mprPath, query string) ([]map[string]interface{}, error) {
	cm, err := NewCatalogManager(mprPath)
	if err != nil {
		return nil, fmt.Errorf("catalog query failed: %w", err)
	}
	defer cm.Close()

	result, err := cm.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert QueryResult to []map[string]interface{} for backward compatibility
	var results []map[string]interface{}

	// Serialize and deserialize to convert to generic map structure
	data, err := json.Marshal(result.Rows)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal results: %w", err)
	}

	return results, nil
}

// CheckMxcliAvailable is deprecated - no longer needed with Go API
// Kept for backward compatibility, always returns nil
func (p *Parser) CheckMxcliAvailable() error {
	// With Go API, we don't need to check for external binary
	// The import will fail at compile time if the package is not available
	return nil
}
