package mendixparser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AnalyzeProject is the main entry point for analyzing a Mendix project
func (p *Parser) AnalyzeProject(mprPath string) (*ProjectAnalysis, error) {
	// Create catalog manager (opens MPR and creates catalog)
	cm, err := NewCatalogManager(mprPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MPR: %w", err)
	}
	defer cm.Close()

	// Build catalog if configured
	if p.config.RefreshCatalog {
		if err := cm.BuildFull(); err != nil {
			return nil, fmt.Errorf("catalog build failed: %w", err)
		}
	}

	// Extract app name
	appName := ExtractAppName(mprPath)

	// Create app component
	appComponent := &ComponentSignal{
		Name:       appName,
		Type:       "service",
		Confidence: 0.95,
		SourceFile: mprPath,
		Evidence:   "mendix_mpr_file",
	}

	analysis := &ProjectAnalysis{
		AppName:      appName,
		MprPath:      mprPath,
		Components:   []ComponentSignal{*appComponent},
		Dependencies: []DependencySignal{},
		Modules:      []string{},
	}

	// Initialize metadata
	analysis.ExtractionProfile = string(p.config.ExtractionProfile)
	analysis.ExtractionTime = time.Now()
	analysis.TablesExtracted = []string{"modules"} // Start with modules

	// Extract modules
	modules, err := p.ExtractModules(cm)
	if err != nil {
		return nil, fmt.Errorf("module extraction failed: %w", err)
	}
	analysis.Modules = modules

	// Optionally create module components
	if p.config.DetectModulesAsComponents {
		for _, module := range modules {
			analysis.Components = append(analysis.Components, ComponentSignal{
				Name:       fmt.Sprintf("%s:%s", appName, module),
				Type:       "service",
				Confidence: 0.85,
				SourceFile: mprPath,
				Evidence:   fmt.Sprintf("Module in %s", appName),
			})
		}
	}

	// === Tier 1: Published APIs ===
	if p.config.ExtractPublishedAPIs {
		// Extract REST APIs
		if publishedAPIs, err := p.ExtractPublishedRESTAPIs(cm); err == nil && len(publishedAPIs) > 0 {
			analysis.PublishedAPIs = append(analysis.PublishedAPIs, publishedAPIs...)
			analysis.TablesExtracted = append(analysis.TablesExtracted, "published_rest_services", "published_rest_operations")

			// Convert to Components
			for _, api := range publishedAPIs {
				analysis.Components = append(analysis.Components, ComponentSignal{
					Name:       fmt.Sprintf("%s:%s", appName, api.Path),
					Type:       "rest-api",
					Confidence: 0.95,
					SourceFile: mprPath,
					Evidence:   fmt.Sprintf("Published REST API: %s (%d operations)", api.Name, len(api.Operations)),
				})
			}
		}

		// Extract OData APIs
		if odataAPIs, err := p.ExtractPublishedODataAPIs(cm); err == nil && len(odataAPIs) > 0 {
			analysis.PublishedAPIs = append(analysis.PublishedAPIs, odataAPIs...)
			analysis.TablesExtracted = append(analysis.TablesExtracted, "odata_services")

			// Convert to Components
			for _, api := range odataAPIs {
				analysis.Components = append(analysis.Components, ComponentSignal{
					Name:       fmt.Sprintf("%s:%s", appName, api.Path),
					Type:       "odata-api",
					Confidence: 0.95,
					SourceFile: mprPath,
					Evidence:   fmt.Sprintf("Published OData API: %s", api.Name),
				})
			}
		}
	}

	// === Tier 1: Domain Model ===
	if p.config.ExtractDomainModel {
		if entities, err := p.ExtractEntities(cm); err == nil {
			analysis.Entities = entities
			analysis.TablesExtracted = append(analysis.TablesExtracted, "entities")

			// Analyze external entity dependencies
			externalDeps := p.AnalyzeEntityExternalDependencies(entities)
			analysis.Dependencies = append(analysis.Dependencies, externalDeps...)
		}
	}

	// === Tier 2: Business Logic ===
	if p.config.ExtractBusinessLogic {
		if microflows, err := p.ExtractMicroflowsInfo(cm); err == nil {
			analysis.Microflows = microflows
			analysis.TablesExtracted = append(analysis.TablesExtracted, "microflows")
		}

		if javaActions, err := p.ExtractJavaActionsInfo(cm); err == nil {
			analysis.JavaActions = javaActions
			analysis.TablesExtracted = append(analysis.TablesExtracted, "java_actions")
		}
	}

	// === Tier 2: UI Structure ===
	if p.config.ExtractUIStructure {
		if pages, err := p.ExtractPagesInfo(cm); err == nil {
			analysis.Pages = pages
			analysis.TablesExtracted = append(analysis.TablesExtracted, "pages")
		}
	}

	// === Tier 2: Configuration ===
	if p.config.ExtractConfiguration {
		if constants, err := p.ExtractConstantsInfo(cm); err == nil {
			analysis.Constants = constants
			analysis.TablesExtracted = append(analysis.TablesExtracted, "constants")

			// Analyze constants for external dependencies
			constantDeps := p.AnalyzeConstantsForExternalDependencies(constants)
			analysis.Dependencies = append(analysis.Dependencies, constantDeps...)
		}
	}

	// Extract external dependencies (Phase 2)
	if restAPIs, err := p.ExtractRESTAPIs(cm, appName, mprPath); err == nil {
		analysis.Dependencies = append(analysis.Dependencies, restAPIs...)
	}

	if databases, err := p.ExtractDatabases(cm, appName, mprPath); err == nil {
		analysis.Dependencies = append(analysis.Dependencies, databases...)
	}

	if services, err := p.ExtractConsumedServices(cm, appName, mprPath); err == nil {
		analysis.Dependencies = append(analysis.Dependencies, services...)
	}

	// Extract internal dependencies if configured
	if p.config.IncludeInternalDeps {
		// Extract module dependencies
		if moduleDeps, err := p.ExtractModuleDependencies(cm, appName, mprPath); err == nil {
			analysis.Dependencies = append(analysis.Dependencies, moduleDeps...)
		}

		// Extract microflow dependencies
		if mfDeps, err := p.ExtractMicroflowDependencies(cm, appName, mprPath, true); err == nil {
			analysis.Dependencies = append(analysis.Dependencies, mfDeps...)
		}
	}

	return analysis, nil
}

// ExtractModules extracts the list of modules in the Mendix app
func (p *Parser) ExtractModules(cm *CatalogManager) ([]string, error) {
	results, err := cm.QueryAsMap("SELECT Name FROM modules ORDER BY Name")
	if err != nil {
		return nil, err
	}

	var modules []string
	for _, row := range results {
		if name, ok := row["Name"].(string); ok && name != "" {
			modules = append(modules, name)
		}
	}

	return modules, nil
}

// ExtractModuleDependencies extracts dependencies between modules
func (p *Parser) ExtractModuleDependencies(cm *CatalogManager, appName, mprPath string) ([]DependencySignal, error) {
	// Query all references
	results, err := cm.QueryAsMap("SELECT SourceName, TargetName FROM refs")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []DependencySignal{}, nil
		}
		return nil, err
	}

	// Build module-to-module edges
	moduleEdges := make(map[string]map[string]int) // [sourceModule][targetModule] = count

	for _, row := range results {
		sourceName, _ := row["SourceName"].(string)
		targetName, _ := row["TargetName"].(string)

		if sourceName == "" || targetName == "" {
			continue
		}

		// Extract module names (format: Module.Element)
		sourceParts := strings.Split(sourceName, ".")
		targetParts := strings.Split(targetName, ".")

		if len(sourceParts) < 2 || len(targetParts) < 2 {
			continue
		}

		sourceModule := sourceParts[0]
		targetModule := targetParts[0]

		// Skip self-references
		if sourceModule == targetModule {
			continue
		}

		if moduleEdges[sourceModule] == nil {
			moduleEdges[sourceModule] = make(map[string]int)
		}
		moduleEdges[sourceModule][targetModule]++
	}

	// Convert to signals
	var signals []DependencySignal
	for sourceModule, targets := range moduleEdges {
		for targetModule, count := range targets {
			signals = append(signals, DependencySignal{
				SourceName: fmt.Sprintf("%s:%s", appName, sourceModule),
				TargetName: fmt.Sprintf("%s:%s", appName, targetModule),
				TargetType: "service", // Modules are sub-components
				Confidence: 0.80,
				Kind:       "module_dependency",
				Evidence:   fmt.Sprintf("%d references from %s to %s", count, sourceModule, targetModule),
				SourceFile: mprPath,
			})
		}
	}

	return signals, nil
}

// ExtractMicroflowDependencies extracts microflow call graph (optional, for detailed analysis)
func (p *Parser) ExtractMicroflowDependencies(cm *CatalogManager, appName, mprPath string, includeInternal bool) ([]DependencySignal, error) {
	if !includeInternal {
		return []DependencySignal{}, nil
	}

	// Query microflow calls from refs table
	results, err := cm.QueryAsMap("SELECT SourceName, TargetName FROM refs WHERE RefKind = 'call' AND SourceType = 'Microflow' AND TargetType = 'Microflow'")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []DependencySignal{}, nil
		}
		return []DependencySignal{}, nil
	}

	var signals []DependencySignal
	for _, row := range results {
		sourceName, _ := row["SourceName"].(string)
		targetName, _ := row["TargetName"].(string)

		if sourceName == "" || targetName == "" {
			continue
		}

		signals = append(signals, DependencySignal{
			SourceName: fmt.Sprintf("%s:%s", appName, sourceName),
			TargetName: fmt.Sprintf("%s:%s", appName, targetName),
			TargetType: "service",
			Confidence: 0.75,
			Kind:       "microflow_call",
			Evidence:   fmt.Sprintf("Microflow %s calls %s", sourceName, targetName),
			SourceFile: mprPath,
		})
	}

	return signals, nil
}

// ExtractRESTAPIs extracts consumed REST API dependencies (Phase 2)
func (p *Parser) ExtractRESTAPIs(cm *CatalogManager, appName, mprPath string) ([]DependencySignal, error) {
	results, err := cm.QueryAsMap("SELECT Name, BaseUrl FROM rest_clients")
	if err != nil {
		// If table doesn't exist, return empty (not an error - catalog might not be built)
		if strings.Contains(err.Error(), "no such table") {
			return []DependencySignal{}, nil
		}
		return nil, err
	}

	var signals []DependencySignal
	for _, row := range results {
		serviceName, _ := row["Name"].(string)
		baseURL, _ := row["BaseUrl"].(string)

		if serviceName == "" {
			continue
		}

		// Extract target name from URL if possible
		targetName := serviceName
		if baseURL != "" {
			targetName = extractServiceNameFromURL(baseURL)
		}

		signals = append(signals, DependencySignal{
			SourceName: appName,
			TargetName: targetName,
			TargetType: "service",
			Confidence: 0.90,
			Kind:       "rest_api_call",
			Evidence:   fmt.Sprintf("REST client: %s -> %s", serviceName, baseURL),
			SourceFile: mprPath,
		})
	}

	return signals, nil
}

// ExtractDatabases extracts external database dependencies (Phase 2)
func (p *Parser) ExtractDatabases(cm *CatalogManager, appName, mprPath string) ([]DependencySignal, error) {
	results, err := cm.QueryAsMap("SELECT Name, ServiceName FROM external_entities")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []DependencySignal{}, nil
		}
		return nil, err
	}

	// Group by service (OData service)
	serviceMap := make(map[string]bool)
	var signals []DependencySignal

	for _, row := range results {
		entityName, _ := row["Name"].(string)
		serviceName, _ := row["ServiceName"].(string)

		if entityName == "" {
			continue
		}

		// Extract database/service identifier (avoid duplicates)
		dbName := serviceName
		if dbName == "" {
			dbName = "external-database"
		}
		if serviceMap[dbName] {
			continue
		}
		serviceMap[dbName] = true

		signals = append(signals, DependencySignal{
			SourceName: appName,
			TargetName: dbName,
			TargetType: "database",
			Confidence: 0.85,
			Kind:       "db_connection",
			Evidence:   fmt.Sprintf("External entity: %s (service: %s)", entityName, serviceName),
			SourceFile: mprPath,
		})
	}

	return signals, nil
}

// ExtractConsumedServices extracts consumed web service dependencies (Phase 2)
func (p *Parser) ExtractConsumedServices(cm *CatalogManager, appName, mprPath string) ([]DependencySignal, error) {
	// Query for external actions (consumed web services)
	results, err := cm.QueryAsMap("SELECT ActionName, ServiceName FROM external_actions")
	if err != nil {
		// External actions table might not exist in all versions
		if strings.Contains(err.Error(), "no such table") {
			return []DependencySignal{}, nil
		}
		return nil, err
	}

	var signals []DependencySignal
	serviceMap := make(map[string]bool)

	for _, row := range results {
		actionName, _ := row["ActionName"].(string)
		serviceName, _ := row["ServiceName"].(string)

		if serviceName == "" {
			continue
		}

		if serviceMap[serviceName] {
			continue
		}
		serviceMap[serviceName] = true

		signals = append(signals, DependencySignal{
			SourceName: appName,
			TargetName: serviceName,
			TargetType: "service",
			Confidence: 0.85,
			Kind:       "webservice_call",
			Evidence:   fmt.Sprintf("External action: %s from service %s", actionName, serviceName),
			SourceFile: mprPath,
		})
	}

	return signals, nil
}

// Helper functions for Phase 2 external dependency extraction

func extractServiceNameFromURL(url string) string {
	// Extract hostname from URL
	// Example: https://api.stripe.com/v1 -> stripe
	parts := strings.Split(url, "://")
	if len(parts) < 2 {
		return url
	}

	host := strings.Split(parts[1], "/")[0]
	host = strings.Split(host, ":")[0] // Remove port

	// Extract main domain
	domainParts := strings.Split(host, ".")
	if len(domainParts) >= 2 {
		return domainParts[len(domainParts)-2]
	}

	return host
}

func extractDatabaseName(entityName, dbType string) string {
	// Extract database identifier from entity name
	// Example: "PostgreSQL_Customer" -> "postgresql"
	if dbType != "" {
		return strings.ToLower(dbType)
	}

	parts := strings.Split(entityName, "_")
	if len(parts) > 0 {
		return strings.ToLower(parts[0])
	}

	return "external-database"
}

// ScanWorkspace scans a directory tree for all Mendix projects
func ScanWorkspace(rootPath string) ([]*ProjectInfo, error) {
	var projects []*ProjectInfo
	seen := make(map[string]bool) // Avoid duplicates

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if info.IsDir() {
			// Check if this directory contains a Mendix project
			isMendix, mprPath, err := DetectMendixProject(path)
			if err != nil || !isMendix {
				return nil
			}

			// Avoid duplicates
			absPath, _ := filepath.Abs(mprPath)
			if seen[absPath] {
				return filepath.SkipDir
			}
			seen[absPath] = true

			// Extract basic info
			appName := ExtractAppName(mprPath)
			projects = append(projects, &ProjectInfo{
				Name:    appName,
				MprPath: mprPath,
				RootDir: filepath.Dir(mprPath),
			})

			return filepath.SkipDir // Don't descend into Mendix project
		}

		return nil
	})

	return projects, err
}

// AnalyzeWorkspace analyzes all Mendix projects in a workspace
func (p *Parser) AnalyzeWorkspace(rootPath string) (*WorkspaceAnalysis, error) {
	// Discover all Mendix projects
	projects, err := ScanWorkspace(rootPath)
	if err != nil {
		return nil, fmt.Errorf("workspace scan failed: %w", err)
	}

	if len(projects) == 0 {
		return &WorkspaceAnalysis{Projects: []ProjectAnalysis{}}, nil
	}

	// Analyze each project
	var analyses []ProjectAnalysis
	for _, proj := range projects {
		analysis, err := p.AnalyzeProject(proj.MprPath)
		if err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: Failed to analyze %s: %v\n", proj.Name, err)
			continue
		}
		analyses = append(analyses, *analysis)
	}

	// Detect inter-app dependencies
	interAppDeps := p.detectInterAppDependencies(analyses)

	return &WorkspaceAnalysis{
		Projects:             analyses,
		InterAppDependencies: interAppDeps,
	}, nil
}

// detectInterAppDependencies finds dependencies between Mendix applications
func (p *Parser) detectInterAppDependencies(analyses []ProjectAnalysis) []DependencySignal {
	var interAppDeps []DependencySignal

	// Build map of app names
	appNames := make(map[string]bool)
	for _, analysis := range analyses {
		appNames[analysis.AppName] = true
	}

	// Check each app's dependencies to see if they reference other apps
	for _, analysis := range analyses {
		for _, dep := range analysis.Dependencies {
			// Check if target is another Mendix app
			if appNames[dep.TargetName] {
				interAppDeps = append(interAppDeps, DependencySignal{
					SourceName: analysis.AppName,
					TargetName: dep.TargetName,
					TargetType: "service",
					Confidence: 0.90,
					Kind:       "inter_app_dependency",
					Evidence:   fmt.Sprintf("%s depends on Mendix app %s", analysis.AppName, dep.TargetName),
					SourceFile: analysis.MprPath,
				})
			}
		}
	}

	// Detect shared database dependencies
	interAppDeps = append(interAppDeps, detectSharedDatabases(analyses)...)

	return interAppDeps
}

// detectSharedDatabases finds databases used by multiple Mendix apps
func detectSharedDatabases(analyses []ProjectAnalysis) []DependencySignal {
	// Map: database name -> list of apps using it
	dbUsage := make(map[string][]string)

	for _, analysis := range analyses {
		for _, dep := range analysis.Dependencies {
			if dep.TargetType == "database" {
				dbUsage[dep.TargetName] = append(dbUsage[dep.TargetName], analysis.AppName)
			}
		}
	}

	var signals []DependencySignal
	for dbName, apps := range dbUsage {
		if len(apps) > 1 {
			// This database is shared
			evidence := fmt.Sprintf("Shared database: used by %s", strings.Join(apps, ", "))
			for _, appName := range apps {
				signals = append(signals, DependencySignal{
					SourceName: appName,
					TargetName: dbName,
					TargetType: "database",
					Confidence: 0.85,
					Kind:       "shared_database",
					Evidence:   evidence,
					SourceFile: "",
				})
			}
		}
	}

	return signals
}

// ========== NEW TIER 1 & TIER 2 EXTRACTION METHODS ==========

// ExtractPublishedRESTAPIs extracts REST APIs this app exposes
func (p *Parser) ExtractPublishedRESTAPIs(cm *CatalogManager) ([]PublishedAPIInfo, error) {
	services, err := cm.QueryAsMap(`
		SELECT Id, Name, QualifiedName, Path, Version, ModuleName, ResourceCount, OperationCount
		FROM published_rest_services
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []PublishedAPIInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query published_rest_services: %w", err)
	}

	var apis []PublishedAPIInfo
	for _, svc := range services {
		serviceId := getString(svc, "Id")

		// Query operations for this service
		ops, err := cm.QueryAsMap(fmt.Sprintf(`
			SELECT ResourceName, HttpMethod, Path, Microflow, Summary
			FROM published_rest_operations
			WHERE ServiceId = '%s'
		`, serviceId))

		var operations []APIOperation
		if err == nil {
			for _, op := range ops {
				operations = append(operations, APIOperation{
					ResourceName: getString(op, "ResourceName"),
					HttpMethod:   getString(op, "HttpMethod"),
					Path:         getString(op, "Path"),
					Microflow:    getString(op, "Microflow"),
					Summary:      getString(op, "Summary"),
				})
			}
		}

		apis = append(apis, PublishedAPIInfo{
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
func (p *Parser) ExtractPublishedODataAPIs(cm *CatalogManager) ([]PublishedAPIInfo, error) {
	services, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, Path, ModuleName, EntitySetCount
		FROM odata_services
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []PublishedAPIInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query odata_services: %w", err)
	}

	var apis []PublishedAPIInfo
	for _, svc := range services {
		apis = append(apis, PublishedAPIInfo{
			Name:           getString(svc, "Name"),
			Type:           "odata",
			Path:           getString(svc, "Path"),
			ModuleName:     getString(svc, "ModuleName"),
			OperationCount: getInt(svc, "EntitySetCount"),
			Operations:     nil,
		})
	}

	return apis, nil
}

// ExtractEntities extracts domain model entities from the catalog
func (p *Parser) ExtractEntities(cm *CatalogManager) ([]EntityInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, EntityType,
		       IsExternal, ExternalService, AttributeCount
		FROM entities
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []EntityInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}

	var entities []EntityInfo
	for _, row := range results {
		entities = append(entities, EntityInfo{
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
func (p *Parser) AnalyzeEntityExternalDependencies(entities []EntityInfo) []DependencySignal {
	var deps []DependencySignal

	for _, entity := range entities {
		if entity.IsExternal && entity.ExternalService != "" {
			kind := "external_entity"
			if strings.Contains(strings.ToLower(entity.ExternalService), "odata") {
				kind = "odata_entity"
			}

			deps = append(deps, DependencySignal{
				SourceName: entity.QualifiedName,
				TargetName: entity.ExternalService,
				TargetType: "external_service",
				Confidence: 0.90,
				Kind:       kind,
				Evidence:   fmt.Sprintf("External entity mapped from %s", entity.ExternalService),
				SourceFile: "",
			})
		}
	}

	return deps
}

// ExtractMicroflowsInfo extracts microflows from the catalog
func (p *Parser) ExtractMicroflowsInfo(cm *CatalogManager) ([]MicroflowInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, MicroflowType,
		       ActivityCount, Complexity, Excluded
		FROM microflows
		WHERE Excluded = 0
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []MicroflowInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query microflows: %w", err)
	}

	var microflows []MicroflowInfo
	for _, row := range results {
		mfType := getString(row, "MicroflowType")
		microflows = append(microflows, MicroflowInfo{
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

// ExtractJavaActionsInfo extracts Java actions from the catalog
func (p *Parser) ExtractJavaActionsInfo(cm *CatalogManager) ([]JavaActionInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, ReturnType,
		       ParameterCount, ExportLevel
		FROM java_actions
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []JavaActionInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query java actions: %w", err)
	}

	var javaActions []JavaActionInfo
	for _, row := range results {
		javaActions = append(javaActions, JavaActionInfo{
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

// ExtractPagesInfo extracts pages from the catalog
func (p *Parser) ExtractPagesInfo(cm *CatalogManager) ([]PageInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, URL, WidgetCount, Excluded
		FROM pages
		WHERE Excluded = 0
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []PageInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query pages: %w", err)
	}

	var pages []PageInfo
	for _, row := range results {
		pages = append(pages, PageInfo{
			Name:          getString(row, "Name"),
			QualifiedName: getString(row, "QualifiedName"),
			ModuleName:    getString(row, "ModuleName"),
			URL:           getString(row, "URL"),
			WidgetCount:   getInt(row, "WidgetCount"),
		})
	}

	return pages, nil
}

// ExtractConstantsInfo extracts constants from the catalog
func (p *Parser) ExtractConstantsInfo(cm *CatalogManager) ([]ConstantInfo, error) {
	results, err := cm.QueryAsMap(`
		SELECT Name, QualifiedName, ModuleName, DataType,
		       DefaultValue, ExposedToClient
		FROM constants
		ORDER BY ModuleName, Name
	`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []ConstantInfo{}, nil
		}
		return nil, fmt.Errorf("failed to query constants: %w", err)
	}

	var constants []ConstantInfo
	for _, row := range results {
		constants = append(constants, ConstantInfo{
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
func (p *Parser) AnalyzeConstantsForExternalDependencies(constants []ConstantInfo) []DependencySignal {
	var deps []DependencySignal

	externalIndicators := []string{"url", "endpoint", "api", "service", "host"}

	for _, constant := range constants {
		constantNameLower := strings.ToLower(constant.Name)

		for _, indicator := range externalIndicators {
			if strings.Contains(constantNameLower, indicator) {
				serviceName := extractServiceNameFromConstant(constant)

				deps = append(deps, DependencySignal{
					SourceName: constant.QualifiedName,
					TargetName: serviceName,
					TargetType: "external_service",
					Confidence: 0.60,
					Kind:       "configuration_reference",
					Evidence:   fmt.Sprintf("Constant '%s' suggests external service configuration", constant.Name),
					SourceFile: "",
				})
				break
			}
		}
	}

	return deps
}

// ========== HELPER FUNCTIONS ==========

// getString safely extracts a string value from a map
func getString(row map[string]interface{}, key string) string {
	if v, ok := row[key]; ok && v != nil {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// getInt safely extracts an integer value from a map
func getInt(row map[string]interface{}, key string) int {
	if v, ok := row[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

// getBool safely extracts a boolean value from a map
func getBool(row map[string]interface{}, key string) bool {
	if v, ok := row[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
		if i, ok := v.(int); ok {
			return i != 0
		}
		if i, ok := v.(int64); ok {
			return i != 0
		}
	}
	return false
}

// extractServiceNameFromConstant attempts to extract a service name from a constant
func extractServiceNameFromConstant(constant ConstantInfo) string {
	if constant.DefaultValue != "" {
		if strings.HasPrefix(constant.DefaultValue, "http://") || strings.HasPrefix(constant.DefaultValue, "https://") {
			parts := strings.Split(constant.DefaultValue, "/")
			if len(parts) >= 3 {
				return parts[2]
			}
		}
		if !strings.Contains(constant.DefaultValue, " ") && len(constant.DefaultValue) < 100 {
			return constant.DefaultValue
		}
	}

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
