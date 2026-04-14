package extractors

import (
	"fmt"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

// ExtractPages extracts pages from the catalog
func ExtractPages(cm *mendixparser.CatalogManager) ([]mendixparser.PageInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, URL, WidgetCount, Excluded
		FROM pages
		WHERE Excluded = 0
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.PageInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query pages: %w", err)
	}

	var pages []mendixparser.PageInfo
	for _, row := range results {
		pages = append(pages, mendixparser.PageInfo{
			Name:          getString(row, "Name"),
			QualifiedName: getString(row, "QualifiedName"),
			ModuleName:    getString(row, "ModuleName"),
			URL:           getString(row, "URL"),
			WidgetCount:   getInt(row, "WidgetCount"),
		})
	}

	return pages, nil
}
