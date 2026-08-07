package hidlinux

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationBoundaryExcludesBackendAndOutgoingOperations(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), filepath.Join("..", "x6"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for filename, file := range packages["x6"].Files {
		for _, imported := range file.Imports {
			if strings.Trim(imported.Path.Value, "\"") == "github.com/sstallion/go-hid" {
				t.Fatalf("%s imports the HID backend", filename)
			}
		}
	}
}
