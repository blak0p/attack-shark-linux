// Package architecture pins the dependency rules between generic code and
// per-model drivers.
package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/blak0p/attack-shark-linux"

// modelRoot holds every per-model driver. Only model packages themselves and
// the composition roots under cmd/ may import it.
const modelRoot = "internal/attackshark"

// knownViolations lists generic packages that still import a model driver.
// Entries may only be removed: the test fails on any new violation and on any
// entry that no longer matches real code.
var knownViolations = map[string][]string{
	"internal/configstore": {"internal/attackshark/x6"},
	"internal/desktop":     {"internal/attackshark/x6", "internal/attackshark/x6/macrocodec", "internal/attackshark/x6/protocol"},
	"internal/hidlinux":    {"internal/attackshark/x6", "internal/attackshark/x6/macrocodec", "internal/attackshark/x6/protocol"},
	"internal/mouse":       {"internal/attackshark/x6/macrocodec", "internal/attackshark/x6/protocol"},
}

// modelUSBIdentity lists the lowercase literals that identify one model's USB
// device. Generic code must read them from registered profiles instead.
var modelUSBIdentity = []string{"0x1d57", "0xfa60", "0xfa61", "1d57:fa6"}

// legacyPackages are the pre-separation homes of the X6 code.
var legacyPackages = []string{"internal/x6", "internal/protocol/x6"}

func TestGenericPackagesDoNotImportModelDrivers(t *testing.T) {
	root := repositoryRoot(t)
	actual := map[string]map[string]bool{}
	for dir, imports := range packageImports(t, root) {
		if isModelPackage(dir) || dir == "cmd" || strings.HasPrefix(dir, "cmd/") {
			continue
		}
		for _, imported := range imports {
			if imported == modelRoot || strings.HasPrefix(imported, modelRoot+"/") {
				if actual[dir] == nil {
					actual[dir] = map[string]bool{}
				}
				actual[dir][imported] = true
			}
		}
	}

	for dir, imports := range actual {
		for imported := range imports {
			if !contains(knownViolations[dir], imported) {
				t.Errorf("%s imports model driver %s; depend on internal/mouse seams instead", dir, imported)
			}
		}
	}
	for dir, imports := range knownViolations {
		for _, imported := range imports {
			if !actual[dir][imported] {
				t.Errorf("stale known violation %s -> %s; remove it from knownViolations", dir, imported)
			}
		}
	}
}

func TestGenericCodeHasNoModelUSBIdentity(t *testing.T) {
	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "testdata", "frontend", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || isModelPackage(filepath.ToSlash(filepath.Dir(rel))) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, literal := range modelUSBIdentity {
			if strings.Contains(lower, literal) {
				t.Errorf("%s contains model USB identity %s; read it from the registered profile", rel, literal)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan repository: %v", err)
	}
}

func TestLegacyX6PackagesAreGone(t *testing.T) {
	root := repositoryRoot(t)
	for _, legacy := range legacyPackages {
		if _, err := os.Stat(filepath.Join(root, legacy)); err == nil {
			t.Errorf("%s still exists; X6 code belongs under %s/x6", legacy, modelRoot)
		}
	}
	for dir, imports := range packageImports(t, root) {
		for _, imported := range imports {
			if contains(legacyPackages, imported) {
				t.Errorf("%s imports legacy package %s", dir, imported)
			}
		}
	}
}

func isModelPackage(dir string) bool {
	return dir == modelRoot || strings.HasPrefix(dir, modelRoot+"/")
}

// packageImports maps each repository-relative package directory to the
// sorted repository-relative imports of all its Go files, tests included.
func packageImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	seen := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "testdata", "frontend", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(rel)
		if seen[dir] == nil {
			seen[dir] = map[string]bool{}
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(imported, module+"/") {
				seen[dir][strings.TrimPrefix(imported, module+"/")] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan repository: %v", err)
	}
	result := map[string][]string{}
	for dir, imports := range seen {
		for imported := range imports {
			result[dir] = append(result[dir], imported)
		}
		sort.Strings(result[dir])
	}
	return result
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root %s has no go.mod: %v", root, err)
	}
	return root
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
