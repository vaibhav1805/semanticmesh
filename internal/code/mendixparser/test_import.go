package mendixparser

import (
	"github.com/mendixlabs/mxcli/mdl/catalog"
	"github.com/mendixlabs/mxcli/sdk/mpr"
)

// testImport verifies mxcli imports compile correctly
func testImport() {
	// Just verify it compiles
	var _ *catalog.Catalog
	var _ mpr.Reader
}
