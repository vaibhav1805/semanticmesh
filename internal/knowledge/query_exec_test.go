package knowledge

import (
	"errors"
	"testing"
)

// --- QueryError interface tests ----------------------------------------------

func TestQueryError_Interface(t *testing.T) {
	qe := &QueryError{
		Message:     "test error",
		Code:        "TEST_CODE",
		Suggestions: []string{"try this"},
	}

	// Verify it satisfies the error interface.
	var err error = qe
	if err.Error() != "test error" {
		t.Fatalf("expected 'test error', got %q", err.Error())
	}

	// Verify type assertion works.
	var target *QueryError
	if !errors.As(err, &target) {
		t.Fatal("expected errors.As to succeed for *QueryError")
	}
	if target.Code != "TEST_CODE" {
		t.Fatalf("expected code TEST_CODE, got %q", target.Code)
	}
	if len(target.Suggestions) != 1 || target.Suggestions[0] != "try this" {
		t.Fatalf("unexpected suggestions: %v", target.Suggestions)
	}
}

// --- Impact query tests ------------------------------------------------------

func TestExecuteImpactQuery_MissingComponent(t *testing.T) {
	_, err := ExecuteImpactQuery(QueryImpactParams{})
	if err == nil {
		t.Fatal("expected error for missing component")
	}

	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QueryError, got %T: %v", err, err)
	}
	if qe.Code != "MISSING_ARG" {
		t.Fatalf("expected code MISSING_ARG, got %q", qe.Code)
	}
}

func TestExecuteImpactQuery_InvalidSourceType(t *testing.T) {
	_, err := ExecuteImpactQuery(QueryImpactParams{
		Component:  "test",
		SourceType: "invalid-type",
	})
	if err == nil {
		t.Fatal("expected error for invalid source type")
	}

	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QueryError, got %T: %v", err, err)
	}
	if qe.Code != "INVALID_ARG" {
		t.Fatalf("expected code INVALID_ARG, got %q", qe.Code)
	}
}

// --- Dependencies query tests ------------------------------------------------

func TestExecuteDependenciesQuery_MissingComponent(t *testing.T) {
	_, err := ExecuteDependenciesQuery(QueryDependenciesParams{})
	if err == nil {
		t.Fatal("expected error for missing component")
	}

	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QueryError, got %T: %v", err, err)
	}
	if qe.Code != "MISSING_ARG" {
		t.Fatalf("expected code MISSING_ARG, got %q", qe.Code)
	}
}

// --- Path query tests --------------------------------------------------------

func TestExecutePathQuery_MissingArgs(t *testing.T) {
	tests := []struct {
		name   string
		params QueryPathParams
	}{
		{"both empty", QueryPathParams{}},
		{"from empty", QueryPathParams{To: "b"}},
		{"to empty", QueryPathParams{From: "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecutePathQuery(tt.params)
			if err == nil {
				t.Fatal("expected error for missing args")
			}

			var qe *QueryError
			if !errors.As(err, &qe) {
				t.Fatalf("expected *QueryError, got %T: %v", err, err)
			}
			if qe.Code != "MISSING_ARG" {
				t.Fatalf("expected code MISSING_ARG, got %q", qe.Code)
			}
		})
	}
}

// --- List query tests --------------------------------------------------------

func TestExecuteListQuery_InvalidSourceType(t *testing.T) {
	_, err := ExecuteListQuery(QueryListParams{
		SourceType: "bad-value",
	})
	if err == nil {
		t.Fatal("expected error for invalid source type")
	}

	var qe *QueryError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QueryError, got %T: %v", err, err)
	}
	if qe.Code != "INVALID_ARG" {
		t.Fatalf("expected code INVALID_ARG, got %q", qe.Code)
	}
}

// --- GraphInfo tests ---------------------------------------------------------

func TestGetGraphInfo_NoGraph(t *testing.T) {
	// Set XDG to a temp dir with no graphs.
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, err := GetGraphInfo(GraphInfoParams{})
	if err == nil {
		t.Fatal("expected error when no graph imported")
	}
	// The error should mention "no graph imported".
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}
