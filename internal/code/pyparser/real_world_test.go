package pyparser

import (
	"testing"
)

func TestSimpleRequestsGet(t *testing.T) {
	src := `
import requests

def test_func():
    response = requests.get("http://tvm-service.internal/v1/getcredentials")
    return response.json()
`

	parser := NewPythonParser()
	signals, err := parser.ParseFile("test.py", []byte(src))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	t.Logf("Found %d signals", len(signals))
	for _, sig := range signals {
		t.Logf("Signal: Line %d: %s -> %s (%s, confidence: %.2f)",
			sig.LineNumber, sig.DetectionKind, sig.TargetComponent, sig.TargetType, sig.Confidence)
	}

	// Should detect HTTP call
	if len(signals) == 0 {
		t.Fatal("Expected to find http_call signal")
	}

	foundHTTPCall := false
	for _, sig := range signals {
		if sig.DetectionKind == "http_call" && sig.TargetComponent == "tvm-service.internal" {
			foundHTTPCall = true
			break
		}
	}

	if !foundHTTPCall {
		t.Error("Expected to find http_call to tvm-service.internal")
	}
}

func TestRequestsGetComparison(t *testing.T) {
	// Compare different URL formats
	tests := []struct {
		name string
		src  string
		expectDetection bool
	}{
		{
			name: "simple_string",
			src: `import requests
resp = requests.get("http://api-01:8080/health")`,
			expectDetection: true,
		},
		{
			name: "fstring_inline",
			src: `import requests
endpoint = "http://api-02"
resp = requests.get(f"{endpoint}/health")`,
			expectDetection: false, // F-strings not parseable by regex
		},
		{
			name: "multiline_fstring",
			src: `import requests
endpoint = "http://api-03"
resp = requests.get(
    f"{endpoint}/v1/data",
    headers={},
    timeout=10
)`,
			expectDetection: false, // F-strings not parseable by regex
		},
	}

	parser := NewPythonParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals, err := parser.ParseFile("test.py", []byte(tt.src))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			t.Logf("Found %d signals", len(signals))
			for _, sig := range signals {
				t.Logf("  Line %d: %s -> %s", sig.LineNumber, sig.DetectionKind, sig.TargetComponent)
			}

			foundHTTPCall := false
			for _, sig := range signals {
				if sig.DetectionKind == "http_call" {
					foundHTTPCall = true
					break
				}
			}

			if foundHTTPCall != tt.expectDetection {
				if tt.expectDetection {
					t.Errorf("Expected to detect HTTP call but didn't")
				} else {
					t.Logf("Note: F-string URLs are not detected (expected limitation)")
				}
			}
		})
	}
}
