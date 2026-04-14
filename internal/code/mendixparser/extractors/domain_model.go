package extractors

import (
	"fmt"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

// ExtractEntities extracts domain model entities from the catalog
func ExtractEntities(cm *mendixparser.CatalogManager) ([]mendixparser.EntityInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, EntityType,
		       IsExternal, ExternalService, AttributeCount
		FROM entities
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.EntityInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}

	var entities []mendixparser.EntityInfo
	for _, row := range results {
		entities = append(entities, mendixparser.EntityInfo{
			Name:            getString(row, "Name"),
			QualifiedName:   getString(row, "QualifiedName"),
			ModuleName:      getString(row, "ModuleName"),
			EntityType:      getString(row, "EntityType"),
			IsExternal:      getBool(row, "IsExternal"),
			ExternalService: getString(row, "ExternalService"),
			AttributeCount:  getInt(row, "AttributeCount"),
		})
	}

	return entities, nil
}

// AnalyzeEntityExternalDependencies detects external entity dependencies
func AnalyzeEntityExternalDependencies(entities []mendixparser.EntityInfo) []mendixparser.DependencySignal {
	var deps []mendixparser.DependencySignal

	for _, entity := range entities {
		if entity.IsExternal && entity.ExternalService != "" {
			kind := "external_entity"
			if strings.Contains(strings.ToLower(entity.ExternalService), "odata") {
				kind = "odata_entity"
			}

			deps = append(deps, mendixparser.DependencySignal{
				SourceName: entity.QualifiedName,
				TargetName: entity.ExternalService,
				TargetType: "external_service",
				Confidence: 0.90,
				Kind:       kind,
				Evidence:   fmt.Sprintf("External entity mapped from %s", entity.ExternalService),
				SourceFile: "", // Will be set by caller if needed
			})
		}
	}

	return deps
}
