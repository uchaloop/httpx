package httpx_test

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/uchaloop/httpx"

// TestCoreImportsOnlyStandardLibrary keeps httpx usable with any router and
// validator. Router, validator and other integrations belong to separate
// adapter modules.
func TestCoreImportsOnlyStandardLibrary(t *testing.T) {
	t.Parallel()

	output, err := exec.Command("go", "list", "-json", "./...").Output()
	if err != nil {
		t.Fatalf("list module packages: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var pkg struct {
			ImportPath   string
			Imports      []string
			TestImports  []string
			XTestImports []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}

		for _, imported := range slices.Concat(pkg.Imports, pkg.TestImports, pkg.XTestImports) {
			if !isStandardLibrary(imported) && imported != modulePath && !strings.HasPrefix(imported, modulePath+"/") {
				t.Errorf("core package %s imports %s outside the standard library", pkg.ImportPath, imported)
			}
		}
	}
}

// isStandardLibrary follows the go command's rule: the first element of a
// standard library import path contains no dot.
func isStandardLibrary(path string) bool {
	first, _, _ := strings.Cut(path, "/")

	return !strings.Contains(first, ".")
}
