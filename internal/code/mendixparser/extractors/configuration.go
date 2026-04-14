package extractors

import (
	"fmt"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

// ExtractConstants extracts constants from the catalog
func ExtractConstants(cm *mendixparser.CatalogManager) ([]mendixparser.ConstantInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, DataType,
		       DefaultValue, ExposedToClient
		FROM constants
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.ConstantInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query constants: %w", err)
	}

	var constants []mendixparser.ConstantInfo
	for _, row := range results {
		constants = append(constants, mendixparser.ConstantInfo{
			Name:            getString(row, "Name"),
			QualifiedName:   getString(row, "QualifiedName"),
			ModuleName:      getString(row, "ModuleName"),
			DataType:        getString(row, "DataType"),
			DefaultValue:    getString(row, "DefaultValue"),
			ExposedToClient: getBool(row, "ExposedToClient"),
		})
	}

	return constants, nil
}

// AnalyzeConstantsForExternalDependencies detects potential external services from constants
func AnalyzeConstantsForExternalDependencies(constants []mendixparser.ConstantInfo) []mendixparser.DependencySignal {
	var deps []mendixparser.DependencySignal

	// Keywords that suggest external service configuration
	externalIndicators := []string{"url", "endpoint", "api", "service", "host"}

	for _, constant := range constants {
		constantNameLower := strings.ToLower(constant.Name)

		// Check if constant name contains external service indicators
		for _, indicator := range externalIndicators {
			if strings.Contains(constantNameLower, indicator) {
				// Extract potential service name from constant
				serviceName := extractServiceNameFromConstant(constant)

				deps = append(deps, mendixparser.DependencySignal{
					SourceName: constant.QualifiedName,
					TargetName: serviceName,
					TargetType: "external_service",
					Confidence: 0.60, // Lower confidence as it's name-based heuristic
					Kind:       "configuration_reference",
					Evidence:   fmt.Sprintf("Constant '%s' suggests external service configuration", constant.Name),
					SourceFile: "", // Will be set by caller if needed
				})
				break // Only create one dependency per constant
			}
		}
	}

	return deps
}

// extractServiceNameFromConstant attempts to extract a service name from a constant
func extractServiceNameFromConstant(constant mendixparser.ConstantInfo) string {
	// If the constant has a value, try to parse it
	if constant.DefaultValue != "" {
		// Check if it's a URL
		if strings.HasPrefix(constant.DefaultValue, "http://") || strings.HasPrefix(constant.DefaultValue, "https://") {
			// Extract domain from URL
			parts := strings.Split(constant.DefaultValue, "/")
			if len(parts) >= 3 {
				return parts[2] // Returns domain like "api.example.com"
			}
		}
		// Return the value as-is if it looks like a hostname or service name
		if !strings.Contains(constant.DefaultValue, " ") && len(constant.DefaultValue) < 100 {
			return constant.DefaultValue
		}
	}

	// Fallback: create a service name from the constant name
	name := constant.Name
	name = strings.ReplaceAll(name, "URL", "")
	name = strings.ReplaceAll(name, "Endpoint", "")
	name = strings.ReplaceAll(name, "API", "")
	name = strings.ReplaceAll(name, "Service", "")
	name = strings.TrimSpace(name)

	if name == "" {
		return "UnknownExternalService"
	}

	return name
}
