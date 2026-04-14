package extractors

import (
	"fmt"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

// ExtractMicroflows extracts microflows from the catalog
func ExtractMicroflows(cm *mendixparser.CatalogManager) ([]mendixparser.MicroflowInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, MicroflowType,
		       ActivityCount, Complexity, Excluded
		FROM microflows
		WHERE Excluded = 0
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.MicroflowInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query microflows: %w", err)
	}

	var microflows []mendixparser.MicroflowInfo
	for _, row := range results {
		mfType := getString(row, "MicroflowType")
		microflows = append(microflows, mendixparser.MicroflowInfo{
			Name:          getString(row, "Name"),
			QualifiedName: getString(row, "QualifiedName"),
			ModuleName:    getString(row, "ModuleName"),
			Type:          mfType,
			ActivityCount: getInt(row, "ActivityCount"),
			Complexity:    getInt(row, "Complexity"),
			IsScheduled:   strings.EqualFold(mfType, "SCHEDULED"),
		})
	}

	return microflows, nil
}

// ExtractJavaActions extracts Java actions from the catalog
func ExtractJavaActions(cm *mendixparser.CatalogManager) ([]mendixparser.JavaActionInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, ReturnType,
		       ParameterCount, ExportLevel
		FROM java_actions
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []mendixparser.JavaActionInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query java actions: %w", err)
	}

	var javaActions []mendixparser.JavaActionInfo
	for _, row := range results {
		javaActions = append(javaActions, mendixparser.JavaActionInfo{
			Name:           getString(row, "Name"),
			QualifiedName:  getString(row, "QualifiedName"),
			ModuleName:     getString(row, "ModuleName"),
			ReturnType:     getString(row, "ReturnType"),
			ParameterCount: getInt(row, "ParameterCount"),
			ExportLevel:    getString(row, "ExportLevel"),
		})
	}

	return javaActions, nil
}
