package extractors

import (
	"fmt"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

// ExtractPublishedRESTAPIs extracts REST APIs this app exposes
func ExtractPublishedRESTAPIs(cm *mendixparser.CatalogManager) ([]mendixparser.PublishedAPIInfo, error) {
	// Query published REST services
	services, err := cm.QueryAsMap(`
		SELECT Id, Name, QualifiedName, Path, Version, ModuleName, ResourceCount, OperationCount
		FROM published_rest_services
	`)
	if err != nil {
		// Handle missing table gracefully (not all projects have published REST APIs)
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.PublishedAPIInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query published_rest_services: %w", err)
	}

	var apis []mendixparser.PublishedAPIInfo
	for _, svc := range services {
		serviceId := getString(svc, "Id")

		// Query operations for this service
		ops, err := cm.QueryAsMap(fmt.Sprintf(`
			SELECT ResourceName, HttpMethod, Path, Microflow, Summary
			FROM published_rest_operations
			WHERE ServiceId = '%s'
		`, serviceId))

		var operations []mendixparser.APIOperation
		if err == nil {
			for _, op := range ops {
				operations = append(operations, mendixparser.APIOperation{
					ResourceName: getString(op, "ResourceName"),
					HttpMethod:   getString(op, "HttpMethod"),
					Path:         getString(op, "Path"),
					Microflow:    getString(op, "Microflow"),
					Summary:      getString(op, "Summary"),
				})
			}
		}

		apis = append(apis, mendixparser.PublishedAPIInfo{
			Name:           getString(svc, "Name"),
			Type:           "rest",
			Path:           getString(svc, "Path"),
			Version:        getString(svc, "Version"),
			ModuleName:     getString(svc, "ModuleName"),
			OperationCount: getInt(svc, "OperationCount"),
			Operations:     operations,
		})
	}

	return apis, nil
}

// ExtractPublishedODataAPIs extracts OData APIs this app exposes
func ExtractPublishedODataAPIs(cm *mendixparser.CatalogManager) ([]mendixparser.PublishedAPIInfo, error) {
	// Query OData services
	services, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, Path, ModuleName, EntitySetCount
		FROM odata_services
	`)
	if err != nil {
		// Handle missing table gracefully (not all projects have OData services)
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.PublishedAPIInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query odata_services: %w", err)
	}

	var apis []mendixparser.PublishedAPIInfo
	for _, svc := range services {
		apis = append(apis, mendixparser.PublishedAPIInfo{
			Name:           getString(svc, "Name"),
			Type:           "odata",
			Path:           getString(svc, "Path"),
			ModuleName:     getString(svc, "ModuleName"),
			OperationCount: getInt(svc, "EntitySetCount"), // OData uses EntitySetCount instead of OperationCount
			Operations:     nil,                           // OData services don't have explicit operations
		})
	}

	return apis, nil
}
