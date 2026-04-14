package mendixparser

import "time"

// ComponentSignal represents a detected component in a Mendix app
type ComponentSignal struct {
	Name       string  // Component name (e.g., app name or module name)
	Type       string  // Component type (e.g., "mendix-app", "service")
	Confidence float64 // Detection confidence [0.4, 1.0]
	SourceFile string  // Path to .mpr file
	Evidence   string  // Description of how it was detected
}

// DependencySignal represents a dependency relationship
type DependencySignal struct {
	SourceName string  // Source component name
	TargetName string  // Target component name
	TargetType string  // Target type (service, database, etc.)
	Confidence float64 // Detection confidence [0.4, 1.0]
	Kind       string  // Dependency kind (e.g., "module_dependency", "http_call")
	Evidence   string  // Evidence for this dependency
	SourceFile string  // Source file path
}

// ProjectAnalysis holds the complete analysis result for a Mendix project
type ProjectAnalysis struct {
	// === Existing fields (unchanged) ===
	AppName      string
	MprPath      string
	Components   []ComponentSignal
	Dependencies []DependencySignal
	Modules      []string // List of modules in the app

	// === NEW: Tier 1 (Published Services) ===
	PublishedAPIs []PublishedAPIInfo `json:"published_apis,omitempty"`

	// === NEW: Tier 1 (Domain Model) ===
	Entities []EntityInfo `json:"entities,omitempty"`

	// === NEW: Tier 2 (Business Logic) ===
	Microflows  []MicroflowInfo  `json:"microflows,omitempty"`
	JavaActions []JavaActionInfo `json:"java_actions,omitempty"`

	// === NEW: Tier 2 (UI Structure) ===
	Pages []PageInfo `json:"pages,omitempty"`

	// === NEW: Tier 2 (Configuration) ===
	Constants []ConstantInfo `json:"constants,omitempty"`

	// === NEW: Metadata ===
	ExtractionProfile string    `json:"extraction_profile,omitempty"` // "minimal", "standard", "comprehensive"
	ExtractionTime    time.Time `json:"extraction_time,omitempty"`
	TablesExtracted   []string  `json:"tables_extracted,omitempty"`
}

// ProjectInfo holds basic information about a discovered Mendix project
type ProjectInfo struct {
	Name    string
	MprPath string
	RootDir string
}

// WorkspaceAnalysis contains results from analyzing multiple Mendix apps
type WorkspaceAnalysis struct {
	Projects             []ProjectAnalysis
	InterAppDependencies []DependencySignal
}

// EntityInfo represents a domain model entity
type EntityInfo struct {
	Name            string `json:"name"`
	QualifiedName   string `json:"qualified_name"`
	ModuleName      string `json:"module_name"`
	EntityType      string `json:"entity_type"`      // Values: "PERSISTENT", "NON_PERSISTENT", "EXTERNAL" (uppercase as stored in Mendix catalog)
	IsExternal      bool   `json:"is_external"`
	ExternalService string `json:"external_service,omitempty"`
	AttributeCount  int    `json:"attribute_count"`
}

// PublishedAPIInfo represents a REST/OData service this app exposes
type PublishedAPIInfo struct {
	Name           string         `json:"name"`
	Type           string         `json:"type"` // "rest", "odata"
	Path           string         `json:"path"`
	Version        string         `json:"version,omitempty"`
	ModuleName     string         `json:"module_name"`
	OperationCount int            `json:"operation_count"`
	Operations     []APIOperation `json:"operations,omitempty"`
}

// APIOperation represents a single REST/OData operation
type APIOperation struct {
	ResourceName string `json:"resource_name"`
	HttpMethod   string `json:"http_method"`
	Path         string `json:"path"`
	Microflow    string `json:"microflow"` // Implementing microflow
	Summary      string `json:"summary,omitempty"`
}

// MicroflowInfo represents business logic
type MicroflowInfo struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	ModuleName    string `json:"module_name"`
	Type          string `json:"type"` // "Microflow", "Nanoflow", "Rule"
	ActivityCount int    `json:"activity_count"`
	Complexity    int    `json:"complexity,omitempty"`
	IsScheduled   bool   `json:"is_scheduled"` // Background job?
}

// PageInfo represents UI entry point
type PageInfo struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	ModuleName    string `json:"module_name"`
	URL           string `json:"url,omitempty"`
	WidgetCount   int    `json:"widget_count"`
}

// JavaActionInfo represents custom Java code
type JavaActionInfo struct {
	Name           string `json:"name"`
	QualifiedName  string `json:"qualified_name"`
	ModuleName     string `json:"module_name"`
	ReturnType     string `json:"return_type,omitempty"`
	ParameterCount int    `json:"parameter_count"`
	ExportLevel    string `json:"export_level"` // "Public", "Private"
}

// ConstantInfo represents configuration constant
type ConstantInfo struct {
	Name            string `json:"name"`
	QualifiedName   string `json:"qualified_name"`
	ModuleName      string `json:"module_name"`
	DataType        string `json:"data_type"`
	DefaultValue    string `json:"default_value,omitempty"`
	ExposedToClient bool   `json:"exposed_to_client"`
}
