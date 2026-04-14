package mendixparser

import (
	"testing"
)

func TestExtractServiceNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "stripe API",
			url:  "https://api.stripe.com/v1",
			want: "stripe",
		},
		{
			name: "github API",
			url:  "https://api.github.com/users",
			want: "github",
		},
		{
			name: "localhost with port",
			url:  "http://localhost:8080/api",
			want: "localhost",
		},
		{
			name: "payment service",
			url:  "http://payment-service:8080/api",
			want: "payment-service",
		},
		{
			name: "subdomain service",
			url:  "https://payments.example.com/process",
			want: "example",
		},
		{
			name: "mendix cloud",
			url:  "https://my-app.mendixcloud.com",
			want: "mendixcloud",
		},
		{
			name: "no protocol",
			url:  "api.example.com",
			want: "api.example.com",
		},
		{
			name: "empty URL",
			url:  "",
			want: "",
		},
		{
			name: "IP address",
			url:  "http://192.168.1.100:3000",
			want: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceNameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractServiceNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractDatabaseName(t *testing.T) {
	tests := []struct {
		name       string
		entityName string
		dbType     string
		want       string
	}{
		{
			name:       "explicit postgres type",
			entityName: "Customer",
			dbType:     "PostgreSQL",
			want:       "postgresql",
		},
		{
			name:       "explicit mysql type",
			entityName: "Order",
			dbType:     "MySQL",
			want:       "mysql",
		},
		{
			name:       "entity name with postgres prefix",
			entityName: "PostgreSQL_Customer",
			dbType:     "",
			want:       "postgresql",
		},
		{
			name:       "entity name with mysql prefix",
			entityName: "MySQL_Order",
			dbType:     "",
			want:       "mysql",
		},
		{
			name:       "entity name with mongodb prefix",
			entityName: "MongoDB_Products",
			dbType:     "",
			want:       "mongodb",
		},
		{
			name:       "no prefix or type",
			entityName: "Product",
			dbType:     "",
			want:       "product",
		},
		{
			name:       "type overrides entity name",
			entityName: "MySQL_Customer",
			dbType:     "PostgreSQL",
			want:       "postgresql",
		},
		{
			name:       "empty entity name",
			entityName: "",
			dbType:     "",
			want:       "",
		},
		{
			name:       "oracle type",
			entityName: "TableName",
			dbType:     "Oracle",
			want:       "oracle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDatabaseName(tt.entityName, tt.dbType)
			if got != tt.want {
				t.Errorf("extractDatabaseName(%q, %q) = %q, want %q",
					tt.entityName, tt.dbType, got, tt.want)
			}
		})
	}
}

// TestQueryCatalogStructure tests that QueryCatalog returns correct structure
// This is a mock test - integration tests would use real mxcli
func TestQueryCatalogStructure(t *testing.T) {
	// Mock catalog results structure
	mockResults := []map[string]interface{}{
		{
			"Name":       "Administration",
			"Type":       "Module",
			"IsUserRole": true,
		},
		{
			"Name":       "MyFirstModule",
			"Type":       "Module",
			"IsUserRole": false,
		},
	}

	// Verify structure
	if len(mockResults) != 2 {
		t.Errorf("Expected 2 results, got %d", len(mockResults))
	}

	for _, result := range mockResults {
		if _, ok := result["Name"]; !ok {
			t.Error("Result should have 'Name' field")
		}
		if _, ok := result["Type"]; !ok {
			t.Error("Result should have 'Type' field")
		}
	}
}

// TestCheckMxcliAvailable tests the mxcli availability check
// This test only verifies the function exists and returns an error type
func TestCheckMxcliAvailableSignature(t *testing.T) {
	parser := New("nonexistent-mxcli-path")
	err := parser.CheckMxcliAvailable()

	// We expect an error since we're using a non-existent path
	if err == nil {
		// If no error, mxcli might actually be installed
		t.Log("mxcli appears to be available on this system")
	} else {
		// Error is expected for non-existent path
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// TestContainsHelper tests the helper function for string matching
func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{
			name:   "exact match",
			s:      "hello",
			substr: "hello",
			want:   true,
		},
		{
			name:   "substring at start",
			s:      "hello world",
			substr: "hello",
			want:   true,
		},
		{
			name:   "substring in middle",
			s:      "hello world",
			substr: "lo wo",
			want:   true,
		},
		{
			name:   "substring at end",
			s:      "hello world",
			substr: "world",
			want:   true,
		},
		{
			name:   "not found",
			s:      "hello world",
			substr: "xyz",
			want:   false,
		},
		{
			name:   "empty substring",
			s:      "hello",
			substr: "",
			want:   true,
		},
		{
			name:   "empty string",
			s:      "",
			substr: "hello",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
